package migrate

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/internal/testutil"
	"ops-admin/backend/model"
)

// newFinopsLinkDB boots an in-memory sqlite through the full step list
// (including step 0006) — the J4 v1-EXTEND surface.
func newFinopsLinkDB(t *testing.T) *gorm.DB {
	t.Helper()
	testutil.PinSecretKeys(t)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}
	return db
}

// TestStep0006FinopsCloudLinkNullableColumn pins the J4 schema contract: the
// link column exists on integration_finops_account, a v1-style row written
// without it lands with NULL (no backfill, no default — existing rows stay
// unlinked by design), and a backfilled v2 row carries the uid back.
func TestStep0006FinopsCloudLinkNullableColumn(t *testing.T) {
	db := newFinopsLinkDB(t)

	v1Row := model.IntegrationFinOpsAccount{Name: "legacy", Provider: "alicloud", Currency: "CNY"}
	if err := db.Create(&v1Row).Error; err != nil {
		t.Fatalf("create v1-style finops row: %v", err)
	}
	var reloaded model.IntegrationFinOpsAccount
	if err := db.First(&reloaded, v1Row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.ProviderConnectionUID != "" {
		t.Fatalf("v1 row must keep the link column NULL, got %q", reloaded.ProviderConnectionUID)
	}

	backfilled := model.IntegrationFinOpsAccount{Name: "linked", Provider: "alicloud", Currency: "CNY", ProviderConnectionUID: "abc123def456"}
	if err := db.Create(&backfilled).Error; err != nil {
		t.Fatalf("create backfilled finops row: %v", err)
	}
	var joined model.IntegrationFinOpsAccount
	if err := db.Where("provider_connection_uid = ?", "abc123def456").First(&joined).Error; err != nil {
		t.Fatalf("link join (the rewire's access path) failed: %v", err)
	}
}

// TestStep0006ListedLast pins the W-4 append-only step list contract: the
// list grew by exactly one line per PR and the pin advances with each new
// step (0007/drop_k8s_cluster — see step0007_drop_k8s_cluster_test.go).
func TestStep0006ListedLast(t *testing.T) {
	if len(steps) == 0 {
		t.Fatal("step list is empty")
	}
	last := steps[len(steps)-1]
	if last.Version != 7 || last.Name != "drop_k8s_cluster" {
		t.Fatalf("last step is %d/%q, want 7/drop_k8s_cluster", last.Version, last.Name)
	}
}
