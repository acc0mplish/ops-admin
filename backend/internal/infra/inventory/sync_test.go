// Package inventory tests (plan §6 T46–T48 + §3.3): the sync runner is the
// adapter-agnostic engine layer — every discoverer here is a scripted stub
// registered in a fresh registry under the "kubernetes" name, so the runner
// is exercised through the same registry-lookup path compose uses (arch rule
// 1 — no provider-type branching in the runner).
package inventory_test

import (
	"context"
	"strings"
	"testing"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/internal/infra/metrics"
	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/infra/secrets"
	"ops-admin/backend/internal/testutil"
	"ops-admin/backend/util"
)

// stubAdapter is the scripted discoverer double: pages are returned in order,
// an entry in errs diverts the matching call to a provider signal or plain
// error, and exhaustion terminates paging.
type stubAdapter struct {
	pages []contract.DiscoverPage
	errs  []error
	calls int
}

func (s *stubAdapter) Descriptor() contract.ProviderTypeDescriptor {
	return contract.ProviderTypeDescriptor{
		Type: "kubernetes", AdapterVersion: "1", ProtocolVersion: "1",
		ContextKinds: []string{"cluster"}, BuiltIn: true,
	}
}

func (*stubAdapter) Validate(_ context.Context, _ contract.ConnectionView) error { return nil }

func (*stubAdapter) Health(_ context.Context, _ contract.ConnectionView) contract.HealthResult {
	return contract.HealthResult{Healthy: true}
}

func (*stubAdapter) Close() error { return nil }

func (s *stubAdapter) Discover(_ context.Context, _ contract.DiscoverRequest) (contract.DiscoverPage, error) {
	i := s.calls
	s.calls++
	if i < len(s.errs) && s.errs[i] != nil {
		return contract.DiscoverPage{}, s.errs[i]
	}
	if i < len(s.pages) {
		return s.pages[i], nil
	}
	return contract.DiscoverPage{}, nil
}

// res builds a minimal well-formed discovered resource.
func res(urn, externalID, kind, subtype string, normalized contract.JSONMap, raw contract.JSONMap) contract.DiscoveredResource {
	return contract.DiscoveredResource{
		ExternalID: externalID, ExternalURN: urn, Kind: kind, Subtype: subtype,
		DisplayName: externalID, Normalized: normalized, Raw: raw,
	}
}

// newInventoryDB opens the in-memory fixture DB with the full v2 schema.
func newInventoryDB(t *testing.T) *gorm.DB {
	t.Helper()
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}
	return db
}

// seedConnection creates the connection → context → secret_ref → binding
// chain a sync resolves through the broker (J2 path), with the credential
// material sealed as a real v2 envelope under the pinned test key.
func seedConnection(t *testing.T, db *gorm.DB, uid string) (model.ProviderConnection, model.ProviderContext) {
	t.Helper()
	envelope, err := util.EncryptSecretV2("test-kubeconfig-material")
	if err != nil {
		t.Fatalf("EncryptSecretV2: %v", err)
	}
	conn := model.ProviderConnection{UID: uid, ProviderType: "kubernetes", Name: "seed cluster", Endpoint: "https://seed:6443", Status: "active"}
	if err := db.Create(&conn).Error; err != nil {
		t.Fatalf("seed connection: %v", err)
	}
	pctx := model.ProviderContext{UID: uid + "-ctx", ConnectionID: conn.ID, Kind: "cluster", ExternalID: "1", Name: "seed cluster"}
	if err := db.Create(&pctx).Error; err != nil {
		t.Fatalf("seed context: %v", err)
	}
	ref := model.SecretRef{UID: uid + "-ref", Backend: "internal", Path: "test/" + uid, Ciphertext: envelope}
	if err := db.Create(&ref).Error; err != nil {
		t.Fatalf("seed secret ref: %v", err)
	}
	binding := model.ProviderCredentialBinding{ProviderConnectionID: conn.ID, ProviderContextID: &pctx.ID, Purpose: "inventory", SecretRefID: ref.ID, Status: "active"}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatalf("seed credential binding: %v", err)
	}
	return conn, pctx
}

// newRunner registers the stub discoverer in a fresh registry and wires the
// runner exactly the way compose.Build does.
func newRunner(t *testing.T, db *gorm.DB, d *stubAdapter) *inventory.SyncRunner {
	t.Helper()
	reg := registry.New()
	if err := reg.RegisterProviderType(d.Descriptor(), d); err != nil {
		t.Fatalf("register stub provider type: %v", err)
	}
	counters := metrics.New()
	counters.RegisterProviders("stub")
	return inventory.NewSyncRunner(db, reg, secrets.NewBroker(db), counters)
}

// runRow reloads the run row by UID.
func runRow(t *testing.T, db *gorm.DB, uid string) model.InventorySyncRun {
	t.Helper()
	var run model.InventorySyncRun
	if err := db.Where("uid = ?", uid).First(&run).Error; err != nil {
		t.Fatalf("load run %q: %v", uid, err)
	}
	return run
}

// resourceRow reloads a resource by URN within a context.
func resourceRow(t *testing.T, db *gorm.DB, contextID uint, urn string) model.InfraResource {
	t.Helper()
	var out model.InfraResource
	if err := db.Where("context_id = ? AND external_urn = ?", contextID, urn).First(&out).Error; err != nil {
		t.Fatalf("load resource %q: %v", urn, err)
	}
	return out
}

// observationCount counts the observations of one resource.
func observationCount(t *testing.T, db *gorm.DB, resourceID uint) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&model.ResourceObservation{}).Where("resource_id = ?", resourceID).Count(&n).Error; err != nil {
		t.Fatalf("count observations: %v", err)
	}
	return n
}

var twoPages = []contract.DiscoverPage{
	{Resources: []contract.DiscoveredResource{
		res("urn:t:node-1", "node-1", "orchestration.node", "", contract.JSONMap{"healthState": "healthy"}, nil),
		res("urn:t:node-2", "node-2", "orchestration.node", "", contract.JSONMap{"healthState": "degraded"}, nil),
	}, NextCursor: "page-2"},
	{Resources: []contract.DiscoveredResource{
		res("urn:t:workload-1", "ns1/workload-1", "orchestration.workload", "", contract.JSONMap{"replicas": 2}, nil),
	}},
}

// T46 — TestSyncPublishesOnlyCompleteGenerations (N8·I1): a run that cannot
// finish every page stays Status=partial with no CommittedAt, the previous
// complete generation stays authoritative, and absence reconciliation (the
// publication-side effect) never fires for an incomplete run.
func TestSyncPublishesOnlyCompleteGenerations(t *testing.T) {
	db := newInventoryDB(t)
	conn, pctx := seedConnection(t, db, "conn-1")

	// Run 1 — both pages complete: a full committed generation.
	runner := newRunner(t, db, &stubAdapter{pages: twoPages})
	report, err := runner.RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"})
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	run1 := runRow(t, db, report.RunUID)
	if run1.Status != inventory.RunStatusSucceeded {
		t.Errorf("run 1 status = %q, want succeeded", run1.Status)
	}
	if run1.CommittedAt == nil {
		t.Error("run 1 CommittedAt is nil — complete generation was not committed")
	}
	if got := report.Outcomes["created"]; got != 3 {
		t.Errorf("run 1 outcomes.created = %d, want 3", got)
	}
	// Every observation of run 1 carries the run generation (§8.2).
	for _, urn := range []string{"urn:t:node-1", "urn:t:node-2", "urn:t:workload-1"} {
		r := resourceRow(t, db, pctx.ID, urn)
		var obs model.ResourceObservation
		if err := db.Where("resource_id = ?", r.ID).First(&obs).Error; err != nil {
			t.Fatalf("observation for %s: %v", urn, err)
		}
		if obs.GenerationUID != run1.UID {
			t.Errorf("observation for %s carries generation %q, want run uid %q", urn, obs.GenerationUID, run1.UID)
		}
	}
	// A succeeded run ends with a terminal cursor (지속 형상 — 계획 §3.3).
	if run1.Cursor != "" {
		t.Errorf("succeeded run cursor = %q, want empty terminal cursor", run1.Cursor)
	}

	// Run 2 — page 1 completes, page 2 signals rate_limited: partial, no commit.
	runner2 := newRunner(t, db, &stubAdapter{
		pages: []contract.DiscoverPage{twoPages[0]},
		errs:  []error{nil, &contract.ProviderSignalError{Kind: contract.SignalRateLimited, Message: "429"}},
	})
	report2, err := runner2.RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"})
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	run2 := runRow(t, db, report2.RunUID)
	if run2.Status != inventory.RunStatusPartial {
		t.Errorf("run 2 status = %q, want partial", run2.Status)
	}
	if run2.CommittedAt != nil {
		t.Error("partial run committed — §9.2 partial generation must not become authoritative")
	}
	if got := report2.Outcomes["rate_limited"]; got != 1 {
		t.Errorf("run 2 outcomes.rate_limited = %d, want 1", got)
	}
	if got := report2.Outcomes["partial"]; got != 1 {
		t.Errorf("run 2 outcomes.partial = %d, want 1", got)
	}
	// The interrupted run keeps its last cursor persisted (지속 형상).
	if run2.Cursor == "" {
		t.Error("partial run lost its persisted cursor")
	}
	// N8/I1: the prior complete generation is untouched — run 1 observations
	// remain the latest, and the not-re-seen workload-1 was NOT absence-marked.
	workload := resourceRow(t, db, pctx.ID, "urn:t:workload-1")
	if got := observationCount(t, db, workload.ID); got != 1 {
		t.Errorf("workload-1 observations = %d, want 1 (no new generation published)", got)
	}
	if workload.ManagedState != "discovered" {
		t.Errorf("workload-1 managed_state = %q after partial run, want discovered (absence reconciliation is publication-side)", workload.ManagedState)
	}
	if workload.DeletedAt != nil {
		t.Error("workload-1 soft-deleted by a partial run")
	}
	// The succeeded run 1 remains the only committed generation.
	var committed int64
	if err := db.Model(&model.InventorySyncRun{}).Where("context_id = ? AND committed_at IS NOT NULL AND status = ?", pctx.ID, inventory.RunStatusSucceeded).Count(&committed).Error; err != nil {
		t.Fatalf("count committed runs: %v", err)
	}
	if committed != 1 {
		t.Errorf("committed succeeded runs = %d, want 1 (the complete generation)", committed)
	}
}

// T47 — TestProviderOutageCreatesNoTombstones (N9·I2): an unreachable signal
// is outside the §9.3 outcome vocabulary — it maps to run Status=failed with
// ErrorCode=provider_unreachable, counts zero tombstones, and leaves every
// discovered resource in place.
func TestProviderOutageCreatesNoTombstones(t *testing.T) {
	db := newInventoryDB(t)
	conn, pctx := seedConnection(t, db, "conn-1")

	runner := newRunner(t, db, &stubAdapter{pages: twoPages})
	if _, err := runner.RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"}); err != nil {
		t.Fatalf("run 1: %v", err)
	}

	outage := newRunner(t, db, &stubAdapter{errs: []error{
		&contract.ProviderSignalError{Kind: contract.SignalUnreachable, Message: "connection refused"},
	}})
	report, err := outage.RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"})
	if err != nil {
		t.Fatalf("outage run: %v", err)
	}
	run := runRow(t, db, report.RunUID)
	if run.Status != inventory.RunStatusFailed {
		t.Errorf("outage run status = %q, want failed", run.Status)
	}
	if run.ErrorCode != "provider_unreachable" {
		t.Errorf("outage run error_code = %q, want provider_unreachable", run.ErrorCode)
	}
	for _, outcome := range []string{"tombstoned", "stale_candidate", "created", "updated"} {
		if got := report.Outcomes[outcome]; got != 0 {
			t.Errorf("outage outcomes[%s] = %d, want 0", outcome, got)
		}
	}
	for _, urn := range []string{"urn:t:node-1", "urn:t:node-2", "urn:t:workload-1"} {
		r := resourceRow(t, db, pctx.ID, urn)
		if r.DeletedAt != nil {
			t.Errorf("%s soft-deleted by provider outage (N9)", urn)
		}
		if r.ManagedState == "tombstoned" || r.ManagedState == "orphaned" {
			t.Errorf("%s managed_state = %q after outage, want untouched", urn, r.ManagedState)
		}
	}
}

// T48 — TestIdentityConflictReportedNotMerged (I3): a provider that returns
// the same identity URN with a different ExternalID is reported through the
// identity_conflict outcome and never merged into the existing row.
func TestIdentityConflictReportedNotMerged(t *testing.T) {
	db := newInventoryDB(t)
	conn, pctx := seedConnection(t, db, "conn-1")

	runner := newRunner(t, db, &stubAdapter{pages: []contract.DiscoverPage{{
		Resources: []contract.DiscoveredResource{
			res("urn:t:node-1", "node-1", "orchestration.node", "", contract.JSONMap{"healthState": "healthy"}, nil),
		},
	}}})
	if _, err := runner.RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"}); err != nil {
		t.Fatalf("run 1: %v", err)
	}

	conflict := newRunner(t, db, &stubAdapter{pages: []contract.DiscoverPage{{
		Resources: []contract.DiscoveredResource{
			res("urn:t:node-1", "node-1-RENAMED", "orchestration.node", "", contract.JSONMap{"healthState": "healthy"}, nil),
		},
	}}})
	report, err := conflict.RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"})
	if err != nil {
		t.Fatalf("conflict run: %v", err)
	}
	if got := report.Outcomes["identity_conflict"]; got != 1 {
		t.Errorf("identity_conflict outcome = %d, want 1 (reported, not merged)", got)
	}
	if got := report.Outcomes["updated"]; got != 0 {
		t.Errorf("updated outcome = %d after identity conflict, want 0 (no merge)", got)
	}
	r := resourceRow(t, db, pctx.ID, "urn:t:node-1")
	if r.ExternalID != "node-1" {
		t.Errorf("existing row ExternalID was merged to %q — identity conflicts must be reported, not merged", r.ExternalID)
	}
	if got := observationCount(t, db, r.ID); got != 1 {
		t.Errorf("observations = %d after identity conflict, want 1 (conflicting payload not stored)", got)
	}
}

// TestSyncCountsNormalizationFailures — resources outside the §8.5 kind
// vocabulary (or without an identity URN) count normalization_failed and are
// never stored.
func TestSyncCountsNormalizationFailures(t *testing.T) {
	db := newInventoryDB(t)
	conn, _ := seedConnection(t, db, "conn-1")

	runner := newRunner(t, db, &stubAdapter{pages: []contract.DiscoverPage{{
		Resources: []contract.DiscoveredResource{
			res("urn:t:bad", "bad", "made.up.kind", "", nil, nil),
			{ExternalID: "no-urn", Kind: "orchestration.node", DisplayName: "no-urn"},
			res("urn:t:ok", "ok", "orchestration.node", "", nil, nil),
		},
	}}})
	report, err := runner.RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := report.Outcomes["normalization_failed"]; got != 2 {
		t.Errorf("normalization_failed = %d, want 2", got)
	}
	if got := report.Outcomes["created"]; got != 1 {
		t.Errorf("created = %d, want 1", got)
	}
	var n int64
	if err := db.Model(&model.InfraResource{}).Count(&n).Error; err != nil {
		t.Fatalf("count resources: %v", err)
	}
	if n != 1 {
		t.Errorf("stored resources = %d, want 1 — failed normalizations must not persist", n)
	}
}

// TestSyncDerivesRelationships — the run-scoped linkage pass turns adapter
// linkage metadata (pod Raw nodeName, pvc Raw volumeName) into §8.4
// discovery relationships stamped with the run generation.
func TestSyncDerivesRelationships(t *testing.T) {
	db := newInventoryDB(t)
	conn, _ := seedConnection(t, db, "conn-1")

	pod := res("urn:t:pod-1", "default/pod-1", "orchestration.pod", "", contract.JSONMap{"phase": "Running"},
		contract.JSONMap{"nodeName": "node-1"})
	pvc := res("urn:t:pvc-1", "default/pvc-1", "storage.volume", "pvc", contract.JSONMap{},
		contract.JSONMap{"volumeName": "pv-1"})
	// The node and the pv must precede the referring resources in the pages —
	// the linkage pass indexes the whole collected run.
	pages := []contract.DiscoverPage{
		{Resources: []contract.DiscoveredResource{
			res("urn:t:node-1", "node-1", "orchestration.node", "", nil, nil),
			res("urn:t:pv-1", "pv-1", "storage.volume", "persistent_volume", nil, nil),
		}, NextCursor: "page-2"},
		{Resources: []contract.DiscoveredResource{pod, pvc}},
	}
	runner := newRunner(t, db, &stubAdapter{pages: pages})
	report, err := runner.RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	var rels []model.InfraRelationship
	if err := db.Order("id").Find(&rels).Error; err != nil {
		t.Fatalf("load relationships: %v", err)
	}
	if len(rels) != 2 {
		t.Fatalf("relationships = %d, want 2 (pod→node runs_on, pvc→pv backed_by)", len(rels))
	}
	urnOf := func(id uint) string {
		var r model.InfraResource
		if err := db.First(&r, id).Error; err != nil {
			t.Fatalf("load resource %d: %v", id, err)
		}
		return r.ExternalURN
	}
	type relKey struct{ from, to, typ string }
	got := map[relKey]model.InfraRelationship{}
	for _, rel := range rels {
		got[relKey{urnOf(rel.FromResourceID), urnOf(rel.ToResourceID), rel.Type}] = rel
	}
	runsOn, ok := got[relKey{"urn:t:pod-1", "urn:t:node-1", "runs_on"}]
	if !ok {
		t.Fatalf("pod→node runs_on relationship missing: %+v", got)
	}
	backedBy, ok := got[relKey{"urn:t:pvc-1", "urn:t:pv-1", "backed_by"}]
	if !ok {
		t.Fatalf("pvc→pv backed_by relationship missing: %+v", got)
	}
	for _, rel := range []model.InfraRelationship{runsOn, backedBy} {
		if rel.GenerationUID != report.RunUID {
			t.Errorf("relationship generation = %q, want run uid %q", rel.GenerationUID, report.RunUID)
		}
		if rel.Source != "discovery" {
			t.Errorf("relationship source = %q, want discovery", rel.Source)
		}
	}
}

// TestSyncPermissionDeniedFailsRun — permission_denied counts its §9.3
// outcome and fails the run (nothing partial about an authorization refusal).
func TestSyncPermissionDeniedFailsRun(t *testing.T) {
	db := newInventoryDB(t)
	conn, _ := seedConnection(t, db, "conn-1")

	runner := newRunner(t, db, &stubAdapter{errs: []error{
		&contract.ProviderSignalError{Kind: contract.SignalPermissionDenied, Message: "forbidden"},
	}})
	report, err := runner.RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	run := runRow(t, db, report.RunUID)
	if run.Status != inventory.RunStatusFailed {
		t.Errorf("status = %q, want failed", run.Status)
	}
	if got := report.Outcomes["permission_denied"]; got != 1 {
		t.Errorf("permission_denied = %d, want 1", got)
	}
}

// TestSyncMarksStaleCandidatesAndTombstones — the §9.3 absence ladder over
// complete generations only: 2 consecutive absences → stale_candidate
// (ManagedState=orphaned), a third → tombstoned; a re-sighting recovers.
func TestSyncMarksStaleCandidatesAndTombstones(t *testing.T) {
	db := newInventoryDB(t)
	conn, pctx := seedConnection(t, db, "conn-1")
	withdrawn := []contract.DiscoverPage{{
		Resources: []contract.DiscoveredResource{
			res("urn:t:node-1", "node-1", "orchestration.node", "", nil, nil),
			res("urn:t:workload-1", "ns1/workload-1", "orchestration.workload", "", contract.JSONMap{"replicas": 2}, nil),
		},
	}}

	// Generation 1: two resources.
	runner := newRunner(t, db, &stubAdapter{pages: twoPages})
	if _, err := runner.RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"}); err != nil {
		t.Fatalf("gen 1: %v", err)
	}
	// Generations 2–4: node-2 absent. The ladder counts completed-generation
	// absences: 1st absence (gen 2) records the counter only, the 2nd (gen 3)
	// fires stale_candidate + orphaned, the 3rd (gen 4) fires tombstone.
	for i := 2; i <= 4; i++ {
		absent := newRunner(t, db, &stubAdapter{pages: withdrawn})
		report, err := absent.RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"})
		if err != nil {
			t.Fatalf("gen %d: %v", i, err)
		}
		run := runRow(t, db, report.RunUID)
		if run.Status != inventory.RunStatusSucceeded {
			t.Fatalf("gen %d status = %q, want succeeded", i, run.Status)
		}
		if i == 2 {
			if got := report.Outcomes["stale_candidate"]; got != 0 {
				t.Errorf("gen 2 stale_candidate = %d, want 0 (1st absence is only the counter)", got)
			}
		}
		if i == 3 {
			if got := report.Outcomes["stale_candidate"]; got != 1 {
				t.Errorf("gen 3 stale_candidate = %d, want 1 (2nd consecutive absence)", got)
			}
			node2 := resourceRow(t, db, pctx.ID, "urn:t:node-2")
			if node2.ManagedState != "orphaned" {
				t.Errorf("gen 3 node-2 managed_state = %q, want orphaned", node2.ManagedState)
			}
		}
		if i == 4 {
			if got := report.Outcomes["tombstoned"]; got != 1 {
				t.Errorf("gen 4 tombstoned = %d, want 1 (3rd consecutive absence)", got)
			}
			node2 := resourceRow(t, db, pctx.ID, "urn:t:node-2")
			if node2.DeletedAt == nil {
				t.Error("gen 4 node-2 not soft-deleted — tombstone missing")
			}
		}
	}
	// Re-sighting recovers the still-present resource.
	back := newRunner(t, db, &stubAdapter{pages: twoPages})
	if _, err := back.RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"}); err != nil {
		t.Fatalf("gen 5: %v", err)
	}
	node1 := resourceRow(t, db, pctx.ID, "urn:t:node-1")
	if node1.ManagedState == "orphaned" || node1.ManagedState == "tombstoned" {
		t.Errorf("node-1 managed_state = %q after re-sighting, want recovered", node1.ManagedState)
	}
}

// TestSyncReportCarriesMetricsRender — the report embeds the shared counter
// render (gate ③ evidence shape — J6).
func TestSyncReportCarriesMetricsRender(t *testing.T) {
	db := newInventoryDB(t)
	conn, _ := seedConnection(t, db, "conn-1")

	reg := registry.New()
	d := &stubAdapter{pages: twoPages}
	if err := reg.RegisterProviderType(d.Descriptor(), d); err != nil {
		t.Fatalf("register: %v", err)
	}
	counters := metrics.New()
	counters.RegisterProviders("stub")
	counters.IncRateLimit("stub")
	runner := inventory.NewSyncRunner(db, reg, secrets.NewBroker(db), counters)

	report, err := runner.RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.MetricsText == "" {
		t.Fatal("report MetricsText empty — gate ③ evidence shape missing")
	}
	if !strings.Contains(report.MetricsText, `provider_rate_limit_total{provider="stub"} 1`) {
		t.Errorf("MetricsText = %q, want the rate-limit line with value 1", report.MetricsText)
	}
	if report.CommittedAt.IsZero() || report.FinishedAt.IsZero() {
		t.Errorf("report timestamps zero: committed=%v finished=%v", report.CommittedAt, report.FinishedAt)
	}
}

// TestSyncRequiresCredentialBinding — a connection without the inventory
// binding fails closed before any provider call (J2 path).
func TestSyncRequiresCredentialBinding(t *testing.T) {
	db := newInventoryDB(t)
	conn := model.ProviderConnection{UID: "conn-no-binding", ProviderType: "kubernetes", Name: "n", Endpoint: "https://x", Status: "active"}
	if err := db.Create(&conn).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	runner := newRunner(t, db, &stubAdapter{})
	if _, err := runner.RunSync(context.Background(), inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"}); err == nil {
		t.Fatal("sync without an inventory credential binding succeeded — must fail closed")
	}
	var runs int64
	if err := db.Model(&model.InventorySyncRun{}).Count(&runs).Error; err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if runs != 0 {
		t.Errorf("runs = %d, want 0 — resolution failure precedes run creation", runs)
	}
}
