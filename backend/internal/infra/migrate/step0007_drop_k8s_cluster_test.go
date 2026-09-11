package migrate

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/testutil"
)

// newStep0007DB boots an in-memory sqlite with steps 0000–0006 applied — the
// pre-drop surface. The v1 tables the step drops (k8s_cluster·asset_service·
// ops_application_env_binding) are recreated by hand in sqlite DDL, because
// the v1 AutoMigrate list is outside this package's scope.
func newStep0007DB(t *testing.T) *gorm.DB {
	t.Helper()
	testutil.PinSecretKeys(t)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := applyPending(db, steps[:7]); err != nil {
		t.Fatalf("apply steps 0000-0006: %v", err)
	}
	return db
}

// TestStep0007DropK8sCluster pins the §19.1 step 5 contract: the legacy table
// drops, the S3 columns drop, and the backfill chains close as
// stale_source=true (§5.4b — mark, never delete) while register-k8s chains
// stay live. The v1 tables are absent on sqlite by default, so they are
// recreated here to exercise the guarded drops for real. A second
// application of the step body must be a no-op (W-5).
func TestStep0007DropK8sCluster(t *testing.T) {
	db := newStep0007DB(t)

	for _, ddl := range []string{
		"CREATE TABLE k8s_cluster (id INTEGER PRIMARY KEY, name TEXT NOT NULL)",
		"CREATE TABLE asset_service (id INTEGER PRIMARY KEY, k8s_cluster_id INTEGER)",
		"CREATE TABLE ops_application_env_binding (id INTEGER PRIMARY KEY, k8s_cluster_id INTEGER)",
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatalf("recreate v1 surface: %v", err)
		}
	}
	backfill := model.ProviderConnection{
		UID: "conn-backfill", ProviderType: "kubernetes", Name: "kind-legacy",
		Endpoint: "https://legacy:6443", SourceModel: "k8s_cluster", SourceID: 1,
	}
	registered := model.ProviderConnection{
		UID: "conn-register", ProviderType: "kubernetes", Name: "kind-registered",
		Endpoint: "https://registered:6443",
	}
	if err := db.Create(&backfill).Error; err != nil {
		t.Fatalf("seed backfill chain: %v", err)
	}
	if err := db.Create(&registered).Error; err != nil {
		t.Fatalf("seed register chain: %v", err)
	}

	if err := applyPending(db, steps[7:]); err != nil {
		t.Fatalf("apply step 0007: %v", err)
	}

	if exists, err := tableExists(db, "k8s_cluster"); err != nil || exists {
		t.Fatalf("k8s_cluster must be gone: exists=%v err=%v", exists, err)
	}
	for _, pair := range [][2]string{
		{"asset_service", "k8s_cluster_id"},
		{"ops_application_env_binding", "k8s_cluster_id"},
	} {
		present, err := columnExists(db, pair[0], pair[1])
		if err != nil || present {
			t.Fatalf("S3 column %s.%s must be gone: present=%v err=%v", pair[0], pair[1], present, err)
		}
	}

	var stale int64
	if err := db.Model(&model.ProviderConnection{}).
		Where("provider_type = ? AND source_model = ? AND stale_source = 1", "kubernetes", "k8s_cluster").
		Count(&stale).Error; err != nil {
		t.Fatal(err)
	}
	if stale != 1 {
		t.Fatalf("stale backfill chains = %d, want 1 (§5.4b — marked, not deleted)", stale)
	}
	var reloaded model.ProviderConnection
	if err := db.Where("uid = ?", "conn-register").First(&reloaded).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.StaleSource {
		t.Fatal("register-k8s chain must stay live after the drop")
	}

	// W-5: re-applying the step body is a no-op (all guards hold).
	if err := step0007DropK8sCluster.Run(db); err != nil {
		t.Fatalf("re-apply step 0007: %v", err)
	}
}

// TestStep0007ListedLast pins the W-4 append-only step list contract: the
// list grew by exactly one line per PR and the pin advances with each new
// step (0008/resource_uid_widen — see step0008_resource_uid_widen.go).
func TestStep0007ListedLast(t *testing.T) {
	if len(steps) == 0 {
		t.Fatal("step list is empty")
	}
	last := steps[len(steps)-1]
	if last.Version != 8 || last.Name != "resource_uid_widen" {
		t.Fatalf("last step is %d/%q, want 8/resource_uid_widen", last.Version, last.Name)
	}
}
