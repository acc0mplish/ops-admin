package store

import (
	"testing"

	"ops-admin/backend/model"
)

// TestV2ComputeInventoryMenuSeed is the PR 30 D2 verification (plan J9/M15 —
// spec §3.5 menu row): the "Compute Inventory" row exists exactly once under
// the Infrastructure group, and reseeding stays a no-op — the row rides the
// same idempotent ensureMenu upsert as the rest of the boot seed.
func TestV2ComputeInventoryMenuSeed(t *testing.T) {
	db := newSeedTestDB(t)
	if err := Seed(db); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	var group model.Menu
	if err := db.Where("value = ?", "infra").First(&group).Error; err != nil {
		t.Fatalf("Infrastructure group missing: %v", err)
	}
	var compute []model.Menu
	if err := db.Where("value = ? AND parent_id = ?", "infra:compute", group.ID).Find(&compute).Error; err != nil {
		t.Fatal(err)
	}
	if len(compute) != 1 {
		t.Fatalf("Compute Inventory menu must seed exactly one row, found %d", len(compute))
	}
	if compute[0].MenuName != "Compute Inventory" || compute[0].URL != "/infra/compute" || compute[0].MenuStatus != 1 {
		t.Fatalf("unexpected Compute Inventory row: %+v", compute[0])
	}

	// Reseed: idempotent — the row count over the whole tree must not move.
	var before int64
	db.Model(&model.Menu{}).Count(&before)
	if err := Seed(db); err != nil {
		t.Fatalf("re-Seed: %v", err)
	}
	var after int64
	db.Model(&model.Menu{}).Count(&after)
	if before != after {
		t.Fatalf("reseed changed the menu row count: %d -> %d", before, after)
	}

	// The pre-existing infra rows are untouched by the additive row.
	for _, value := range []string{"infra:overview", "infra:providers", "infra:resources", "infra:tasks"} {
		var count int64
		db.Model(&model.Menu{}).Where("value = ?", value).Count(&count)
		if count != 1 {
			t.Fatalf("existing infra menu %q must stay a single row, found %d", value, count)
		}
	}
}
