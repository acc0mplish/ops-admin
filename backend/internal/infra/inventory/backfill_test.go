// Backfill tests (plan §6 T49 — §5.4 propagation): the k8s_cluster → V2
// propagation is read-only on the v1 table and idempotent across re-runs.
package inventory_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/util"
)

// k8sClusterDDL is the minimal v1 k8s_cluster shape the backfill reads
// (columns mirror the gorm default naming of model.K8sCluster — verified:
// api_server/node_count/kube_config/connection_mode).
const k8sClusterDDL = `CREATE TABLE k8s_cluster (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	api_server TEXT NOT NULL,
	version TEXT NOT NULL DEFAULT '',
	node_count INTEGER DEFAULT 0,
	env TEXT DEFAULT '',
	tags TEXT DEFAULT '[]',
	connection_mode TEXT DEFAULT 'direct',
	gateway_id INTEGER,
	kube_config TEXT DEFAULT '',
	updated_at DATETIME
)`

type k8sClusterSeed struct {
	name, apiServer, kubeConfig string
	gatewayID                   *uint
}

// seedClusters inserts v1 rows and returns their ids in insertion order.
func seedClusters(t *testing.T, db *gorm.DB, seeds []k8sClusterSeed) []uint {
	t.Helper()
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	ids := make([]uint, 0, len(seeds))
	for i, s := range seeds {
		row := map[string]any{
			"name": s.name, "api_server": s.apiServer, "version": "v1.30.0",
			"node_count": 3, "env": "dev", "tags": `["a","b"]`,
			"connection_mode": "direct", "kube_config": s.kubeConfig,
			"updated_at": base.Add(time.Duration(i) * time.Hour),
		}
		if s.gatewayID != nil {
			row["gateway_id"] = *s.gatewayID
		}
		if err := db.Table("k8s_cluster").Create(row).Error; err != nil {
			t.Fatalf("seed k8s_cluster %s: %v", s.name, err)
		}
		var id uint
		if err := db.Table("k8s_cluster").Where("name = ?", s.name).Select("id").Scan(&id).Error; err != nil {
			t.Fatalf("reload k8s_cluster id: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

// connectionBySource loads the backfilled connection for a v1 row id.
func connectionBySource(t *testing.T, db *gorm.DB, id uint) (model.ProviderConnection, bool) {
	t.Helper()
	var conn model.ProviderConnection
	err := db.Where("source_model = ? AND source_id = ?", "k8s_cluster", id).First(&conn).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return model.ProviderConnection{}, false
		}
		t.Fatalf("load backfilled connection: %v", err)
	}
	return conn, true
}

// T49 — TestBackfillIncrementalAndStaleMarking (§5.4): full propagation on
// the first run, changed-rows-only on re-runs, v1 deletions marked
// stale_source, and the kubeconfig envelope copied verbatim (J2 — no
// re-encryption, no plaintext detour).
func TestBackfillIncrementalAndStaleMarking(t *testing.T) {
	db := newInventoryDB(t)
	if err := db.Exec(k8sClusterDDL).Error; err != nil {
		t.Fatalf("create k8s_cluster: %v", err)
	}
	envelopeA, err := util.EncryptSecretV2("kubeconfig-A")
	if err != nil {
		t.Fatalf("encrypt A: %v", err)
	}
	envelopeB, err := util.EncryptSecretV2("kubeconfig-B")
	if err != nil {
		t.Fatalf("encrypt B: %v", err)
	}
	gw := uint(7)
	ids := seedClusters(t, db, []k8sClusterSeed{
		{name: "kind-a", apiServer: "https://a:6443", kubeConfig: envelopeA, gatewayID: &gw},
		{name: "kind-b", apiServer: "https://b:6443", kubeConfig: envelopeB},
		{name: "kind-empty", apiServer: "https://c:6443", kubeConfig: ""},                           // defensive skip
		{name: "kind-legacy", apiServer: "https://d:6443", kubeConfig: "plaintext-not-an-envelope"}, // A6 UNKNOWN halt
	})

	// Run 1 — full propagation.
	report, err := inventory.RunK8sBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("backfill run 1: %v", err)
	}
	if report.Created != 2 {
		t.Errorf("run 1 created = %d, want 2", report.Created)
	}
	if report.Skipped != 2 {
		t.Errorf("run 1 skipped = %d, want 2 (empty kubeconfig + non-envelope A6 halt)", report.Skipped)
	}

	// Mapping assertions for the propagated row.
	connA, ok := connectionBySource(t, db, ids[0])
	if !ok {
		t.Fatal("run 1 produced no connection for kind-a")
	}
	if connA.ProviderType != "kubernetes" || connA.Endpoint != "https://a:6443" || connA.Name != "kind-a" {
		t.Errorf("connection mapping wrong: %+v", connA)
	}
	if connA.GatewayID == nil || *connA.GatewayID != gw {
		t.Errorf("gateway id not inherited: %+v", connA.GatewayID)
	}
	if connA.SourceModel != "k8s_cluster" || connA.SourceID != ids[0] {
		t.Errorf("source provenance missing: model=%q id=%d", connA.SourceModel, connA.SourceID)
	}
	if connA.SourceUpdatedAt == nil {
		t.Error("source_updated_at not checkpointed")
	}
	if mode, _ := connA.ConfigJSON["connection_mode"].(string); mode != "direct" {
		t.Errorf("config connection_mode = %v, want direct", connA.ConfigJSON["connection_mode"])
	}

	// Context: one cluster context per connection (k8s 1:1 — A5).
	var pctx model.ProviderContext
	if err := db.Where("connection_id = ?", connA.ID).First(&pctx).Error; err != nil {
		t.Fatalf("backfilled context: %v", err)
	}
	if pctx.Kind != "cluster" || pctx.ExternalID != itoaUint(ids[0]) || pctx.Name != "kind-a" {
		t.Errorf("context mapping wrong: %+v", pctx)
	}

	// SecretRef: envelope verbatim (J2), key id parsed from the envelope.
	binding := model.ProviderCredentialBinding{}
	if err := db.Where("provider_connection_id = ?", connA.ID).First(&binding).Error; err != nil {
		t.Fatalf("backfilled credential binding: %v", err)
	}
	if binding.Purpose != "inventory" {
		t.Errorf("binding purpose = %q, want inventory", binding.Purpose)
	}
	ref := model.SecretRef{}
	if err := db.First(&ref, binding.SecretRefID).Error; err != nil {
		t.Fatalf("backfilled secret ref: %v", err)
	}
	if ref.Ciphertext != envelopeA {
		t.Error("SecretRef.Ciphertext is not the kubeconfig envelope verbatim — re-encryption or plaintext detour")
	}
	if ref.KeyID == "" || !strings.HasPrefix(envelopeA, "v2:"+ref.KeyID+":") {
		t.Errorf("SecretRef.KeyID = %q not parsed from the envelope", ref.KeyID)
	}

	// Run 2 — immediate re-run: nothing changed → unchanged everywhere.
	report2, err := inventory.RunK8sBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("backfill run 2: %v", err)
	}
	if report2.Created != 0 || report2.Updated != 0 || report2.Unchanged != 2 {
		t.Errorf("run 2 report = %+v, want created 0 updated 0 unchanged 2", report2)
	}
	// Deterministic UID — the same source row maps to the same connection.
	connA2, _ := connectionBySource(t, db, ids[0])
	if connA2.UID != connA.UID {
		t.Errorf("connection UID changed across re-runs: %q → %q (must be source-key derived)", connA.UID, connA2.UID)
	}

	// Run 3 — touch kind-a: only the changed row is reprocessed.
	if err := db.Exec(`UPDATE k8s_cluster SET name = 'kind-a-renamed', updated_at = ? WHERE id = ?`,
		time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC), ids[0]).Error; err != nil {
		t.Fatalf("touch kind-a: %v", err)
	}
	report3, err := inventory.RunK8sBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("backfill run 3: %v", err)
	}
	if report3.Updated != 1 || report3.Unchanged != 1 || report3.Created != 0 {
		t.Errorf("run 3 report = %+v, want updated 1 unchanged 1 created 0", report3)
	}
	connA3, _ := connectionBySource(t, db, ids[0])
	if connA3.Name != "kind-a-renamed" {
		t.Errorf("run 3 did not propagate the rename: name = %q", connA3.Name)
	}

	// Run 4 — delete kind-b in v1: its V2 connection is marked stale_source.
	if err := db.Exec(`DELETE FROM k8s_cluster WHERE id = ?`, ids[1]).Error; err != nil {
		t.Fatalf("delete kind-b: %v", err)
	}
	report4, err := inventory.RunK8sBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("backfill run 4: %v", err)
	}
	if report4.MarkedStale != 1 {
		t.Errorf("run 4 markedStale = %d, want 1", report4.MarkedStale)
	}
	connB, ok := connectionBySource(t, db, ids[1])
	if !ok {
		t.Fatal("stale marking removed the connection row — §5.4b marks, not deletes")
	}
	if !connB.StaleSource {
		t.Error("kind-b connection not marked stale_source")
	}
	// The live row must stay unstale.
	connAlive, _ := connectionBySource(t, db, ids[0])
	if connAlive.StaleSource {
		t.Error("live kind-a connection wrongly marked stale_source")
	}

	// Run 5 — re-run after stale marking is a no-op (idempotent marking).
	report5, err := inventory.RunK8sBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("backfill run 5: %v", err)
	}
	if report5.MarkedStale != 0 {
		t.Errorf("run 5 markedStale = %d, want 0 (idempotent marking)", report5.MarkedStale)
	}
}

// TestBackfillDoesNotTouchV1Table — R9: the backfill is read-only on
// k8s_cluster; a run must leave row count and content identical.
func TestBackfillDoesNotTouchV1Table(t *testing.T) {
	db := newInventoryDB(t)
	if err := db.Exec(k8sClusterDDL).Error; err != nil {
		t.Fatalf("create k8s_cluster: %v", err)
	}
	envelope, err := util.EncryptSecretV2("kubeconfig")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	ids := seedClusters(t, db, []k8sClusterSeed{{name: "kind-a", apiServer: "https://a:6443", kubeConfig: envelope}})

	before := map[string]any{}
	if err := db.Table("k8s_cluster").Where("id = ?", ids[0]).Take(&before).Error; err != nil {
		t.Fatalf("read v1 row: %v", err)
	}

	if _, err := inventory.RunK8sBackfill(context.Background(), db); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	after := map[string]any{}
	if err := db.Table("k8s_cluster").Where("id = ?", ids[0]).Take(&after).Error; err != nil {
		t.Fatalf("re-read v1 row: %v", err)
	}
	for key, want := range before {
		if got, ok := after[key]; !ok || got != want {
			t.Errorf("v1 row column %q changed: %v → %v (backfill must be read-only on k8s_cluster)", key, want, got)
		}
	}
}

// TestBackfillCheckpointReflectsSource — the report checkpoint carries the
// newest processed v1 updated_at (the incremental watermark).
func TestBackfillCheckpointReflectsSource(t *testing.T) {
	db := newInventoryDB(t)
	if err := db.Exec(k8sClusterDDL).Error; err != nil {
		t.Fatalf("create k8s_cluster: %v", err)
	}
	envelope, _ := util.EncryptSecretV2("kubeconfig")
	ids := seedClusters(t, db, []k8sClusterSeed{
		{name: "kind-a", apiServer: "https://a:6443", kubeConfig: envelope},
		{name: "kind-b", apiServer: "https://b:6443", kubeConfig: envelope},
	})
	report, err := inventory.RunK8sBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	var want time.Time
	if err := db.Table("k8s_cluster").Where("id = ?", ids[1]).Select("updated_at").Scan(&want).Error; err != nil {
		t.Fatalf("read checkpoint source: %v", err)
	}
	if !report.Checkpoint.Equal(want) {
		t.Errorf("checkpoint = %v, want newest v1 updated_at %v", report.Checkpoint, want)
	}
	// An empty v1 table yields the zero checkpoint without an error.
	empty := newInventoryDB(t)
	if err := empty.Exec(k8sClusterDDL).Error; err != nil {
		t.Fatalf("create empty k8s_cluster: %v", err)
	}
	emptyReport, err := inventory.RunK8sBackfill(context.Background(), empty)
	if err != nil {
		t.Fatalf("empty backfill: %v", err)
	}
	if !emptyReport.Checkpoint.IsZero() {
		t.Errorf("empty-table checkpoint = %v, want zero", emptyReport.Checkpoint)
	}
}

func itoaUint(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}
