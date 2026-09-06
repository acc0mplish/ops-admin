// Backfill — the §5.4 propagation: v1 k8s_cluster rows propagate into the
// V2 provider_connection/provider_context/secret_ref/credential_binding
// tables. Read-only on k8s_cluster (R9 — v1 stays authoritative until the
// M2 cutover), idempotent across re-runs, incrementally keyed on the source
// updated-at checkpoint.
package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/util"
)

// v2EnvelopePrefix — the §4.2 envelope version tag (util's unexported
// constant, mirrored here as a read-only shape check). Values carrying the
// prefix are copied verbatim; anything else is treated as P-class plaintext
// (see sourceKubeSecret) — a plaintext detour into SecretRef is still
// forbidden (A6 posture, 보존 제약 #7).
const v2EnvelopePrefix = "v2:"

// BackfillReport is the propagation result — the report artifact's backfill
// section and the stale-source visibility surface (§13: Phase 2 exposes
// stale rows through MarkedStale counts only).
type BackfillReport struct {
	Created     int       `json:"created"`
	Updated     int       `json:"updated"`
	Unchanged   int       `json:"unchanged"`
	MarkedStale int       `json:"markedStale"`
	Skipped     int       `json:"skipped"`
	Checkpoint  time.Time `json:"checkpoint"`
}

// k8sClusterRow is the read-only projection of one v1 k8s_cluster row (the
// column subset the §3.4 mapping consumes — gorm default naming verified).
type k8sClusterRow struct {
	ID             uint
	Name           string
	APIServer      string
	Version        string
	NodeCount      int
	Env            string
	Tags           []byte
	ConnectionMode string
	GatewayID      *uint
	KubeConfig     string
	UpdatedAt      time.Time
}

// RunK8sBackfill propagates every v1 k8s_cluster row into the V2 tables:
//
//   - §5.4a incremental: a row is reprocessed only when its updated_at moved
//     past the stored source_updated_at checkpoint — re-runs touch changed
//     rows only.
//   - §5.4b stale marking: source rows that disappeared from k8s_cluster
//     mark their V2 connection stale_source=true (marked, never deleted).
//   - kubeconfig (P-class): envelope sources are copied verbatim; plaintext
//     sources are encrypted on copy (sourceKubeSecret — J2's verbatim-only
//     assumption did not hold and is recorded there).
func RunK8sBackfill(ctx context.Context, db *gorm.DB) (BackfillReport, error) {
	var rows []k8sClusterRow
	err := db.WithContext(ctx).Table("k8s_cluster").
		Select("id, name, api_server, version, node_count, env, tags, connection_mode, gateway_id, kube_config, updated_at").
		Order("id").
		Scan(&rows).Error
	if err != nil {
		return BackfillReport{}, fmt.Errorf("inventory: read k8s_cluster (read-only source): %w", err)
	}

	report := BackfillReport{}
	for i := range rows {
		row := &rows[i]
		if !report.Checkpoint.After(row.UpdatedAt) && report.Checkpoint.Before(row.UpdatedAt) {
			report.Checkpoint = row.UpdatedAt
		}
		keyID, secret, ok, err := sourceKubeSecret(row.KubeConfig)
		if err != nil {
			return report, fmt.Errorf("inventory: seal plaintext kubeconfig for k8s_cluster %d: %w", row.ID, err)
		}
		if !ok {
			// Empty (defensive — a registered cluster always has a
			// kubeconfig): skipped, never copied.
			report.Skipped++
			continue
		}
		created, updated, unchanged, err := propagateCluster(ctx, db, row, keyID, secret)
		if err != nil {
			return report, err
		}
		report.Created += created
		report.Updated += updated
		report.Unchanged += unchanged
	}

	stale, err := markStaleSources(ctx, db)
	if err != nil {
		return report, err
	}
	report.MarkedStale = stale
	return report, nil
}

// envelopeKeyID parses the v2 envelope shape "v2:<key_id>:<payload>" and
// returns its key id — the only part of the envelope the backfill reads.
func envelopeKeyID(value string) (string, bool) {
	if !strings.HasPrefix(value, v2EnvelopePrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(value, v2EnvelopePrefix)
	keyID, payload, ok := strings.Cut(rest, ":")
	if !ok || keyID == "" || payload == "" {
		return "", false
	}
	return keyID, true
}

// sourceKubeSecret normalizes the source kube_config into (keyID, secret)
// for the SecretRef copy, keeping k8s_cluster itself untouched (R9).
//
// J2 assumed the source kubeconfig always arrives as a v2 envelope and is
// copied VERBATIM. That assumption was wrong: spec §4.4 orders
// k8s_cluster.kube_config as P-class — plaintext is KEPT in v1 through the
// whole Phase 4 window because the v1 reader (service/k8s.go parseKubeConfig)
// consumes plain YAML and an envelope there breaks the legacy capture path.
// Spec §7.5 makes SecretRef v2-only, so a plaintext source is encrypted on
// copy into a fresh v2 envelope instead of being skipped — no plaintext ever
// lands in V2, and no envelope is ever written back into v1.
func sourceKubeSecret(value string) (keyID, secret string, ok bool, err error) {
	if strings.TrimSpace(value) == "" {
		return "", "", false, nil
	}
	if id, has := envelopeKeyID(value); has {
		// v2 source (post-cutover writer) — verbatim copy stands.
		return id, value, true, nil
	}
	sealed, err := util.EncryptSecretV2(value)
	if err != nil {
		return "", "", false, err
	}
	id, has := envelopeKeyID(sealed)
	if !has {
		return "", "", false, fmt.Errorf("sealed kubeconfig is not a v2 envelope")
	}
	return id, sealed, true, nil
}

// propagateCluster applies one source row to the V2 tables. secret is the
// SecretRef material — always a v2 envelope (verbatim or freshly sealed).
func propagateCluster(ctx context.Context, db *gorm.DB, row *k8sClusterRow, keyID, secret string) (created, updated, unchanged int, err error) {
	var existing model.ProviderConnection
	err = db.WithContext(ctx).Where("uid = ?", SourceKeyUID("k8s_cluster", row.ID)).First(&existing).Error
	switch {
	case isRecordNotFound(err):
		if err := createClusterChain(db, row, keyID, secret); err != nil {
			return 0, 0, 0, fmt.Errorf("inventory: backfill k8s_cluster %d: %w", row.ID, err)
		}
		return 1, 0, 0, nil
	case err != nil:
		return 0, 0, 0, fmt.Errorf("inventory: load backfilled connection for k8s_cluster %d: %w", row.ID, err)
	}

	// §5.4a — reprocess only when the source moved past the checkpoint.
	if existing.SourceUpdatedAt != nil && !row.UpdatedAt.After(*existing.SourceUpdatedAt) {
		return 0, 0, 1, nil
	}

	sourceUpdatedAt := row.UpdatedAt
	// Struct + Select — config_json's column serializer applies to
	// model-field writes only (map updates double-encode the JSON text).
	patch := model.ProviderConnection{
		Name: row.Name, Endpoint: row.APIServer, GatewayID: row.GatewayID,
		ConfigJSON: clusterConfig(row), Version: row.Version,
		SourceModel: "k8s_cluster", SourceID: row.ID, SourceUpdatedAt: &sourceUpdatedAt,
	}
	if err := db.Model(&model.ProviderConnection{}).Where("id = ?", existing.ID).
		Select("name", "endpoint", "gateway_id", "config_json", "version", "source_model", "source_id", "source_updated_at").
		Updates(patch).Error; err != nil {
		return 0, 0, 0, fmt.Errorf("inventory: update backfilled connection: %w", err)
	}
	if err := refreshClusterSatellites(db, existing.ID, row.Name, strconvID(row.ID), keyID, secret); err != nil {
		return 0, 0, 0, err
	}
	return 0, 1, 0, nil
}

// createClusterChain writes the connection + context + secret_ref +
// credential_binding chain for a first-time source row (§3.4 mapping).
func createClusterChain(db *gorm.DB, row *k8sClusterRow, keyID, secret string) error {
	connectionUID := SourceKeyUID("k8s_cluster", row.ID)
	sourceUpdatedAt := row.UpdatedAt
	conn := model.ProviderConnection{
		UID: connectionUID, ProviderType: "kubernetes",
		Name: row.Name, Endpoint: row.APIServer, GatewayID: row.GatewayID,
		Status: "active", Version: row.Version,
		ConfigJSON: clusterConfig(row),
		// §5.4 provenance — the incremental checkpoint lives here.
		SourceModel: "k8s_cluster", SourceID: row.ID, SourceUpdatedAt: &sourceUpdatedAt,
	}
	if err := db.Create(&conn).Error; err != nil {
		return fmt.Errorf("create provider_connection: %w", err)
	}

	pctx := model.ProviderContext{
		UID:          SourceKeyUIDSalted("k8s_cluster", row.ID, "context"),
		ConnectionID: conn.ID, Kind: "cluster",
		ExternalID: strconvID(row.ID), Name: row.Name,
		Status: "active",
	}
	if err := db.Create(&pctx).Error; err != nil {
		return fmt.Errorf("create provider_context: %w", err)
	}

	// P-class kubeconfig — the SecretRef material is always a v2 envelope
	// (verbatim envelope source, or freshly sealed plaintext, see
	// sourceKubeSecret). The v1 cell keeps its original value.
	ref := model.SecretRef{
		UID:        SourceKeyUIDSalted("k8s_cluster", row.ID, "secret"),
		Backend:    "internal",
		Path:       "backfill/k8s_cluster/" + strconvID(row.ID),
		KeyID:      keyID,
		Ciphertext: secret,
	}
	if err := db.Create(&ref).Error; err != nil {
		return fmt.Errorf("create secret_ref: %w", err)
	}

	binding := model.ProviderCredentialBinding{
		ProviderConnectionID: conn.ID,
		ProviderContextID:    &pctx.ID,
		Purpose:              "inventory",
		SecretRefID:          ref.ID,
		Status:               "active",
	}
	if err := db.Create(&binding).Error; err != nil {
		return fmt.Errorf("create provider_credential_binding: %w", err)
	}
	return nil
}

// refreshClusterSatellites re-points the context name and, when the source
// changed (re-key, rotation, or re-sealed plaintext — the incremental
// checkpoint already gates this to actual source updates), re-copies the
// SecretRef material.
func refreshClusterSatellites(db *gorm.DB, connectionID uint, name, externalID, keyID, kubeConfig string) error {
	if err := db.Model(&model.ProviderContext{}).Where("connection_id = ?", connectionID).
		Updates(map[string]any{"name": name, "external_id": externalID}).Error; err != nil {
		return fmt.Errorf("inventory: refresh backfilled context: %w", err)
	}
	var binding model.ProviderCredentialBinding
	if err := db.Where("provider_connection_id = ? AND purpose = ?", connectionID, "inventory").
		Order("id").First(&binding).Error; err != nil {
		return fmt.Errorf("inventory: load backfilled credential binding: %w", err)
	}
	var ref model.SecretRef
	if err := db.First(&ref, binding.SecretRefID).Error; err != nil {
		return fmt.Errorf("inventory: load backfilled secret_ref: %w", err)
	}
	if ref.Ciphertext == kubeConfig {
		return nil
	}
	if err := db.Model(&model.SecretRef{}).Where("id = ?", ref.ID).Updates(map[string]any{
		"key_id": keyID, "ciphertext": kubeConfig,
	}).Error; err != nil {
		return fmt.Errorf("inventory: refresh backfilled secret_ref: %w", err)
	}
	return nil
}

// markStaleSources — §5.4b: v1 rows that disappeared mark their V2
// connections stale_source=true. The read side (§5.4c) treats marked rows as
// absent; marking is idempotent (already-stale rows are not re-counted).
func markStaleSources(ctx context.Context, db *gorm.DB) (int, error) {
	res := db.WithContext(ctx).Model(&model.ProviderConnection{}).
		Where("source_model = ? AND stale_source = ? AND source_id NOT IN (?)",
			"k8s_cluster", false, db.Table("k8s_cluster").Select("id")).
		Update("stale_source", true)
	if res.Error != nil {
		return 0, fmt.Errorf("inventory: mark stale_source connections: %w", res.Error)
	}
	return int(res.RowsAffected), nil
}

// clusterConfig assembles the §3.4 ConfigJSON projection (env, tags,
// connection_mode, version, node_count — no credential material ever).
func clusterConfig(row *k8sClusterRow) contract.JSONMap {
	config := contract.JSONMap{
		"env":             row.Env,
		"connection_mode": row.ConnectionMode,
		"version":         row.Version,
		"node_count":      row.NodeCount,
	}
	var tags []any
	if len(row.Tags) > 0 {
		if err := json.Unmarshal(row.Tags, &tags); err == nil {
			config["tags"] = tags
		}
	}
	return config
}

func strconvID(id uint) string {
	return fmt.Sprintf("%d", id)
}
