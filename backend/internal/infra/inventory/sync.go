// Package inventory owns the V2 shadow-sync engine (§9): the SyncRunner
// turns Discoverer pages into infra_resource rows and generation-stamped
// observations, reconciles outcomes, and publishes a generation only when
// every page completed (§9.2). PR 21 scope — plan §2, §3.3 (r2).
package inventory

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/metrics"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/infra/secrets"
)

// SyncRunner drives one shadow sync per invocation (A9 — the CLI is the only
// Phase 2 trigger). It resolves the credential binding through the secrets
// broker (J2), pages the registry-selected Discoverer, and owns the
// InventorySyncRun lifecycle. The runner never branches on a provider type —
// the registry lookup is the only adapter access path (arch rule 1).
type SyncRunner struct {
	db       *gorm.DB
	registry *registry.Registry
	broker   *secrets.Broker
	counters *metrics.Counters
}

// NewSyncRunner wires the runner onto shared dependencies (compose).
func NewSyncRunner(db *gorm.DB, reg *registry.Registry, broker *secrets.Broker, counters *metrics.Counters) *SyncRunner {
	return &SyncRunner{db: db, registry: reg, broker: broker, counters: counters}
}

// SyncInput — plan §3.3. Mode is recorded on the run; the Phase 2 execution
// path is full-mode only — cursor resume and incremental consumption are the
// Phase 4 handoff (§3.3 r2, §13).
type SyncInput struct {
	ConnectionUID string
	Mode          string // full | incremental | targeted (§9.1)
}

// SyncReport is the runner's machine-readable result — the CLI artifact
// shape (claims 5/6 evidence) minus anything credential-shaped.
type SyncReport struct {
	RunUID      string
	Status      string
	Outcomes    map[string]int // §9.3 verbatim 10종
	StartedAt   time.Time
	CommittedAt time.Time // zero on a partial/failed run
	FinishedAt  time.Time
	MetricsText string // Prometheus render — gate ③ evidence (J6)
}

// RunSync executes one sync run for the connection.
func (r *SyncRunner) RunSync(ctx context.Context, in SyncInput) (SyncReport, error) {
	mode := in.Mode
	if mode == "" {
		mode = "full"
	}

	conn, err := r.loadConnection(ctx, in.ConnectionUID)
	if err != nil {
		return SyncReport{}, err
	}
	discoverer, err := r.resolveDiscoverer(conn.ProviderType)
	if err != nil {
		return SyncReport{}, err
	}
	pctx, err := r.resolveContext(ctx, conn.ID)
	if err != nil {
		return SyncReport{}, err
	}
	view, err := r.resolveConnectionView(ctx, conn)
	if err != nil {
		return SyncReport{}, err
	}

	started := time.Now()
	run := model.InventorySyncRun{
		UID: newSyncUID(), ConnectionID: conn.ID, ContextID: pctx.ID,
		Mode: mode, Status: RunStatusRunning, StartedAt: started,
	}
	if err := r.db.Create(&run).Error; err != nil {
		return SyncReport{}, fmt.Errorf("inventory: create sync run: %w", err)
	}

	report := SyncReport{RunUID: run.UID, Status: RunStatusRunning, StartedAt: started, Outcomes: newOutcomeMap()}
	seen, status, errCode := r.consumePages(ctx, &run, pctx.ID, discoverer, view, report.Outcomes)
	finished := time.Now()

	// §9.2 — a full generation becomes authoritative only after all pages
	// complete: absence reconciliation and the commit are publication-side
	// effects that fire on the succeeded path ONLY (N8/N9).
	if status == RunStatusSucceeded {
		if err := r.reconcileAbsences(pctx.ID, &run, seen, report.Outcomes, finished); err != nil {
			return report, fmt.Errorf("inventory: reconcile absences: %w", err)
		}
		report.CommittedAt = finished
	} else if len(seen) > 0 || run.SeenCount > 0 {
		// 부분 완료 병기 (§3.3 r2) — the partial count accompanies every run
		// that observed resources but did not complete its pages.
		report.Outcomes[OutcomePartial]++
	}

	// Terminal cursor bookkeeping (지속 형상 — §3.3 r2): a succeeded run ends
	// with the terminal cursor; partial/failed runs keep their last cursor.
	terminalCursor := run.Cursor
	if status == RunStatusSucceeded {
		terminalCursor = ""
	}
	updates := map[string]any{
		"status":        status,
		"cursor":        terminalCursor,
		"error_code":    errCode,
		"seen_count":    run.SeenCount,
		"created_count": report.Outcomes[OutcomeCreated],
		"updated_count": report.Outcomes[OutcomeUpdated],
		"missing_count": run.MissingCount,
		"finished_at":   finished,
	}
	if !report.CommittedAt.IsZero() {
		updates["committed_at"] = report.CommittedAt
	}
	if err := r.db.Model(&model.InventorySyncRun{}).Where("id = ?", run.ID).Updates(updates).Error; err != nil {
		return report, fmt.Errorf("inventory: finalize sync run: %w", err)
	}

	report.Status = status
	report.FinishedAt = finished
	if r.counters != nil {
		report.MetricsText = r.counters.Render()
	}
	return report, nil
}

// loadConnection loads the connection row by UID.
func (r *SyncRunner) loadConnection(ctx context.Context, uid string) (model.ProviderConnection, error) {
	var conn model.ProviderConnection
	err := r.db.WithContext(ctx).Where("uid = ?", uid).First(&conn).Error
	if err != nil {
		if isRecordNotFound(err) {
			return model.ProviderConnection{}, fmt.Errorf("inventory: connection %q not found", uid)
		}
		return model.ProviderConnection{}, fmt.Errorf("inventory: load connection: %w", err)
	}
	return conn, nil
}

// resolveDiscoverer is the only adapter access path — registry lookup, then
// the Discoverer capability assertion (arch rule 1).
func (r *SyncRunner) resolveDiscoverer(providerType string) (contract.Discoverer, error) {
	_, adapter, ok := r.registry.ProviderType(providerType)
	if !ok {
		return nil, fmt.Errorf("inventory: provider type %q is not registered", providerType)
	}
	discoverer, ok := adapter.(contract.Discoverer)
	if !ok {
		return nil, fmt.Errorf("inventory: provider type %q does not implement Discoverer", providerType)
	}
	return discoverer, nil
}

// resolveContext finds the cluster context of the connection (k8s contexts
// are 1:1 with the connection — A5; the first context is the sync scope).
func (r *SyncRunner) resolveContext(ctx context.Context, connectionID uint) (model.ProviderContext, error) {
	var pctx model.ProviderContext
	err := r.db.WithContext(ctx).Where("connection_id = ?", connectionID).Order("id").First(&pctx).Error
	if err != nil {
		if isRecordNotFound(err) {
			return model.ProviderContext{}, fmt.Errorf("inventory: connection %d has no provider context", connectionID)
		}
		return model.ProviderContext{}, fmt.Errorf("inventory: load provider context: %w", err)
	}
	return pctx, nil
}

// resolveConnectionView assembles the DiscoverRequest.Connection view (J2):
// the broker resolves the "inventory" purpose material, which lands in
// Material["inventory"] — the adapter reads req.Connection only. The
// resolved value never reaches the report, logs, or errors.
func (r *SyncRunner) resolveConnectionView(ctx context.Context, conn model.ProviderConnection) (contract.ConnectionView, error) {
	resolved, err := r.broker.Resolve(ctx, conn.UID, "inventory")
	if err != nil {
		return contract.ConnectionView{}, fmt.Errorf("inventory: resolve inventory credential: %w", err)
	}
	return contract.ConnectionView{
		ProviderType: conn.ProviderType,
		Endpoint:     conn.Endpoint,
		Config:       conn.ConfigJSON,
		Material:     map[string]string{"inventory": resolved.Value},
	}, nil
}

// consumePages walks the Discoverer cursor chain, upserting every resource.
// Returns the run-scoped seen set (for the publication-side passes), the
// terminal run status, and the error code.
func (r *SyncRunner) consumePages(
	ctx context.Context,
	run *model.InventorySyncRun,
	contextID uint,
	discoverer contract.Discoverer,
	view contract.ConnectionView,
	outcomes map[string]int,
) (seen []resourceSeen, status, errCode string) {
	seen = make([]resourceSeen, 0, 64)
	cursor := ""
	for {
		page, err := discoverer.Discover(ctx, contract.DiscoverRequest{
			ContextID:  contextID,
			Cursor:     cursor,
			Connection: view,
		})
		if err != nil {
			// ProviderSignalError → outcome/상태 매핑 (§3.3 r2 — 어휘 밖
			// unreachable은 outcome 미가산, run 상태로 처분).
			var sig *contract.ProviderSignalError
			switch {
			case errors.As(err, &sig) && sig.Kind == contract.SignalRateLimited:
				outcomes[OutcomeRateLimited]++
				if run.SeenCount == 0 {
					return seen, RunStatusFailed, "provider_rate_limited"
				}
				return seen, RunStatusPartial, ""
			case errors.As(err, &sig) && sig.Kind == contract.SignalPermissionDenied:
				outcomes[OutcomePermissionDenied]++
				return seen, RunStatusFailed, "provider_permission_denied"
			case errors.As(err, &sig) && sig.Kind == contract.SignalUnreachable:
				// N9 — provider outage is not resource deletion: no outcome
				// count, no reconciliation (never reached on this path).
				return seen, RunStatusFailed, "provider_unreachable"
			default:
				return seen, RunStatusFailed, "discover_failed"
			}
		}

		for _, res := range page.Resources {
			if record, _ := r.upsertResource(run, contextID, res, outcomes); record != nil {
				seen = append(seen, *record)
			}
			run.SeenCount++
		}

		cursor = page.NextCursor
		// Cursor persistence after every page (§9.2 cursors are persisted —
		// resume consumption is the Phase 4 handoff, §3.3 r2).
		run.Cursor = cursor
		if err := r.db.Model(&model.InventorySyncRun{}).Where("id = ?", run.ID).
			Update("cursor", cursor).Error; err != nil {
			return seen, RunStatusFailed, "cursor_persist_failed"
		}
		if cursor == "" {
			break
		}
	}

	// Linkage pass — complete pages collected, so relationships derive now.
	if err := r.deriveRelationships(r.db, run.UID, seen, time.Now()); err != nil {
		return seen, RunStatusFailed, "relationship_failed"
	}
	return seen, RunStatusSucceeded, ""
}

// upsertResource applies one discovered resource to the identity-keyed row:
// created / updated / unchanged outcomes, the identity_conflict guard
// (I3 — reported, not merged), and the normalization guard (§8.5 kind
// vocabulary + URN presence). A nil record means the resource never became
// part of the run's seen set.
func (r *SyncRunner) upsertResource(run *model.InventorySyncRun, contextID uint, res contract.DiscoveredResource, outcomes map[string]int) (*resourceSeen, string) {
	now := time.Now()

	if res.ExternalURN == "" || !contract.IsKnownResourceKind(res.Kind) {
		outcomes[OutcomeNormalizationFail]++
		return nil, OutcomeNormalizationFail
	}

	var existing model.InfraResource
	err := r.db.Where("context_id = ? AND kind = ? AND external_urn = ?", contextID, res.Kind, res.ExternalURN).First(&existing).Error
	switch {
	case isRecordNotFound(err):
		return r.createResource(run, contextID, res, now, outcomes)
	case err != nil:
		outcomes[OutcomeNormalizationFail]++
		return nil, OutcomeNormalizationFail
	}

	// Identity conflict — a same-URN row whose ExternalID disagrees. Report,
	// never merge (I3); the row still counts as seen for absence purposes.
	if existing.ExternalID != "" && existing.ExternalID != res.ExternalID {
		outcomes[OutcomeIdentityConflict]++
		recoverSighting(&existing, now)
		if err := r.saveSightingState(&existing); err != nil {
			outcomes[OutcomeNormalizationFail]++
			return nil, OutcomeNormalizationFail
		}
		return &resourceSeen{id: existing.ID, kind: existing.Kind, subtype: existing.Subtype, name: existing.DisplayName, raw: res.Raw}, OutcomeIdentityConflict
	}

	// Unchanged vs updated — the §8.2 payload hash decides; only a changed
	// payload earns a new generation-stamped observation.
	priorHash, hasPrior, hashErr := latestObservationHash(r.db, existing.ID)
	if hashErr != nil {
		outcomes[OutcomeUnchanged]++
		return &resourceSeen{id: existing.ID, kind: existing.Kind, subtype: existing.Subtype, name: existing.DisplayName, raw: res.Raw}, OutcomeUnchanged
	}
	if hasPrior && priorHash == observationHash(res) {
		outcomes[OutcomeUnchanged]++
		recoverSighting(&existing, now)
		if err := r.saveSightingState(&existing); err != nil {
			outcomes[OutcomeNormalizationFail]++
			return nil, OutcomeNormalizationFail
		}
		return &resourceSeen{id: existing.ID, kind: existing.Kind, subtype: existing.Subtype, name: existing.DisplayName, raw: res.Raw}, OutcomeUnchanged
	}

	if err := r.updateResource(run, &existing, res, now, outcomes); err != nil {
		outcomes[OutcomeNormalizationFail]++
		return nil, OutcomeNormalizationFail
	}
	outcomes[OutcomeUpdated]++
	return &resourceSeen{id: existing.ID, kind: res.Kind, subtype: res.Subtype, name: res.DisplayName, raw: res.Raw}, OutcomeUpdated
}

// createResource inserts the identity row plus its first generation-stamped
// observation.
func (r *SyncRunner) createResource(run *model.InventorySyncRun, contextID uint, res contract.DiscoveredResource, now time.Time, outcomes map[string]int) (*resourceSeen, string) {
	lifecycle, health := resourceStates(res)
	created := model.InfraResource{
		UID: newSyncUID(), ContextID: contextID,
		Kind: res.Kind, Subtype: res.Subtype,
		ExternalID: res.ExternalID, ExternalURN: res.ExternalURN,
		Name: res.ExternalID, DisplayName: res.DisplayName,
		LifecycleState: lifecycle, HealthState: health,
		ManagedState: "discovered",
		LabelsJSON:   adapterLabels(res),
		FirstSeenAt:  now, LastSeenAt: now,
	}
	if err := r.db.Create(&created).Error; err != nil {
		outcomes[OutcomeNormalizationFail]++
		return nil, OutcomeNormalizationFail
	}
	if err := storeObservation(r.db, created.ID, run.UID, res, now); err != nil {
		outcomes[OutcomeNormalizationFail]++
		return nil, OutcomeNormalizationFail
	}
	outcomes[OutcomeCreated]++
	return &resourceSeen{id: created.ID, kind: res.Kind, subtype: res.Subtype, name: res.DisplayName, raw: res.Raw}, OutcomeCreated
}

// updateResource persists the changed payload and its new observation.
func (r *SyncRunner) updateResource(run *model.InventorySyncRun, existing *model.InfraResource, res contract.DiscoveredResource, now time.Time, outcomes map[string]int) error {
	lifecycle, health := resourceStates(res)
	existing.Subtype = res.Subtype
	existing.DisplayName = res.DisplayName
	existing.LifecycleState = lifecycle
	existing.HealthState = health
	existing.LabelsJSON = adapterLabels(res)
	recoverSighting(existing, now)
	if err := r.db.Model(&model.InfraResource{}).Where("id = ?", existing.ID).
		Select("subtype", "display_name", "lifecycle_state", "health_state", "labels_json", "managed_state", "last_seen_at").
		Updates(*existing).Error; err != nil {
		return err
	}
	return storeObservation(r.db, existing.ID, run.UID, res, now)
}

// saveSightingState persists the sighting-side columns (absence reset +
// last_seen_at). Struct + Select — the labels column serializer applies to
// model-field writes only (map updates double-encode the JSON text).
func (r *SyncRunner) saveSightingState(res *model.InfraResource) error {
	return r.db.Model(&model.InfraResource{}).Where("id = ?", res.ID).
		Select("labels_json", "managed_state", "last_seen_at").
		Updates(model.InfraResource{
			LabelsJSON: res.LabelsJSON, ManagedState: res.ManagedState, LastSeenAt: res.LastSeenAt,
		}).Error
}

// reconcileAbsences runs the §9.3 absence ladder over the completed
// generation: every live resource of the context not seen this run advances
// the ladder (2nd consecutive absence → stale_candidate/orphaned, 3rd →
// tombstone). Fires on the succeeded path only.
func (r *SyncRunner) reconcileAbsences(contextID uint, run *model.InventorySyncRun, seen []resourceSeen, outcomes map[string]int, at time.Time) error {
	seenIDs := make(map[uint]bool, len(seen))
	for _, s := range seen {
		seenIDs[s.id] = true
	}

	var resources []model.InfraResource
	if err := r.db.Where("context_id = ? AND deleted_at IS NULL", contextID).Find(&resources).Error; err != nil {
		return fmt.Errorf("inventory: load resources for absence pass: %w", err)
	}

	missing := 0
	for i := range resources {
		res := &resources[i]
		if seenIDs[res.ID] {
			continue
		}
		missing++
		outcome := noteAbsence(r.db, res, at)
		columns := []string{"labels_json"}
		if outcome == OutcomeTombstoned {
			columns = append(columns, "managed_state", "deleted_at")
		} else if outcome == OutcomeStaleCandidate {
			columns = append(columns, "managed_state")
		}
		if err := r.db.Model(&model.InfraResource{}).Where("id = ?", res.ID).
			Select(columns).Updates(model.InfraResource{
			LabelsJSON: res.LabelsJSON, ManagedState: res.ManagedState, DeletedAt: res.DeletedAt,
		}).Error; err != nil {
			return err
		}
		if outcome != "" {
			outcomes[outcome]++
		}
	}
	run.MissingCount = missing
	return nil
}

// recoverSighting resets the absence ladder on a re-sighting (shared by the
// updated/unchanged/conflict paths).
func recoverSighting(res *model.InfraResource, now time.Time) {
	clearAbsence(res)
	res.LastSeenAt = now
}

// adapterLabels copies the adapter's Raw payload keys into the resource
// label map, skipping the reserved internal absence key (it is owned by the
// absence pass, never by adapter data).
func adapterLabels(res contract.DiscoveredResource) contract.JSONMap {
	labels := make(contract.JSONMap, len(res.Raw))
	for k, v := range res.Raw {
		if k == absenceLabelKey {
			continue
		}
		labels[k] = v
	}
	return labels
}

// newSyncUID — crypto/rand 16-byte hex (32 chars), the engine's A12 UID
// convention (no uuid dependency — go.mod is frozen). A crypto/rand failure
// is a dead process; panicking is the honest posture.
func newSyncUID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic("inventory: crypto/rand failed for UID generation: " + err.Error())
	}
	return hex.EncodeToString(buf[:])
}

// SourceKeyUID — the deterministic UID the backfill derives from the v1
// source key (plan §3.4): re-runs must map the same source row to the same
// connection/context/secret rows.
func SourceKeyUID(kind string, id uint) string {
	sum := sha256.Sum256([]byte("backfill|" + kind + "|" + strconv.FormatUint(uint64(id), 10)))
	return hex.EncodeToString(sum[:])[:32]
}

// SourceKeyUIDSalted — the deterministic UID for the satellite rows
// (context, secret_ref) of one backfilled source.
func SourceKeyUIDSalted(kind string, id uint, salt string) string {
	sum := sha256.Sum256([]byte("backfill|" + kind + "|" + strconv.FormatUint(uint64(id), 10) + "|" + salt))
	return hex.EncodeToString(sum[:])[:32]
}
