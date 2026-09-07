package store

import (
	"testing"

	"ops-admin/backend/model"
)

// TestV2TasksApprovalsMenuSeed is the E2 verification (plan M20 — A10): the
// "Tasks & Approvals" menu row exists exactly once under the Infrastructure
// group after Seed, reseeding is a no-op (the boot seed's idempotent upsert
// is the "(one migration)" semantics — A10), and the pre-existing menu rows
// are untouched by the additive row.
func TestV2TasksApprovalsMenuSeed(t *testing.T) {
	db := newSeedTestDB(t)
	if err := Seed(db); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	var group model.Menu
	if err := db.Where("value = ?", "infra").First(&group).Error; err != nil {
		t.Fatalf("Infrastructure group missing: %v", err)
	}
	var tasks []model.Menu
	if err := db.Where("value = ? AND parent_id = ?", "infra:tasks", group.ID).Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("Tasks & Approvals menu must seed exactly one row (claim 15), found %d", len(tasks))
	}
	if tasks[0].MenuName != "Tasks & Approvals" || tasks[0].URL != "/infra/tasks" || tasks[0].MenuStatus != 1 {
		t.Fatalf("unexpected Tasks & Approvals row: %+v", tasks[0])
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

	// The pre-existing PR 23 infra rows are untouched.
	for _, value := range []string{"infra:overview", "infra:providers", "infra:resources"} {
		var count int64
		db.Model(&model.Menu{}).Where("value = ?", value).Count(&count)
		if count != 1 {
			t.Fatalf("existing infra menu %q must stay a single row, found %d", value, count)
		}
	}
}
