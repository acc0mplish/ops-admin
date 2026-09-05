package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/model"
)

// fixturePath is the committed baseline fixture: today's schema plus today's
// seed output frozen as bytes. From the first post-freeze schema change on,
// reopening it and running AutoMigrate + Seed proves the "old data → new
// schema" upgrade path (in-process reruns only prove idempotency — the
// distinction D-4 draws).
var fixturePath = filepath.Join("testdata", "migration-fixture", "baseline.sqlite")

// TestMigrationFixtureUpgrade reopens the committed baseline fixture from
// disk and proves a migration pass over it is safe: AutoMigrate + Seed leave
// every pre-existing row in place, never double-seed, and keep the one-shot
// route-permission migration marker.
//
// Set OPS_MIGRATION_FIXTURE_UPDATE=1 to regenerate the fixture from a clean
// database. Regeneration commits must stay separate from the schema change
// they follow and must name the code state they were built from (M1 guard,
// documented in docs/security/ci-baseline.md) — otherwise the upgrade proof
// degenerates into the idempotency proof the fixture exists to replace.
func TestMigrationFixtureUpgrade(t *testing.T) {
	t.Setenv("OPS_ADMIN_INITIAL_PASSWORD", "fixture-test-password")
	if os.Getenv("OPS_MIGRATION_FIXTURE_UPDATE") == "1" {
		regenerateFixture(t)
	}

	if _, err := os.Stat(fixturePath); err != nil {
		t.Fatalf("baseline fixture missing: %v (regenerate with OPS_MIGRATION_FIXTURE_UPDATE=1 go test ./store/ -run TestMigrationFixture)", err)
	}

	// Open a copy: sqlite may write to the file it opens (journal, page
	// churn), and the committed fixture must stay frozen.
	work := filepath.Join(t.TempDir(), "baseline.sqlite")
	copyFixtureFile(t, fixturePath, work)
	db := openFixtureDB(t, work)

	before := snapshotFixtureCounts(t, db)
	if before.roles == 0 || before.menus == 0 {
		t.Fatalf("fixture is not seeded: roles=%d menus=%d", before.roles, before.menus)
	}
	if !fixtureMarkerPresent(t, db) {
		t.Fatal("fixture lacks the route-permissions granted marker; it must freeze a fully seeded state")
	}

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate over fixture: %v", err)
	}
	if err := Seed(db); err != nil {
		t.Fatalf("Seed over fixture: %v", err)
	}

	after := snapshotFixtureCounts(t, db)
	if after.roles != before.roles {
		t.Errorf("roles changed %d -> %d over reopen + AutoMigrate + Seed: Seed must not recreate roles on an already seeded database", before.roles, after.roles)
	}
	if after.admins < before.admins {
		t.Errorf("admins shrank %d -> %d over reopen + migration: data loss", before.admins, after.admins)
	}
	if after.menus < before.menus {
		t.Errorf("menus shrank %d -> %d over reopen + migration: data loss", before.menus, after.menus)
	}
	if !fixtureMarkerPresent(t, db) {
		t.Error("route-permissions granted marker lost over reopen + migration")
	}
}

// regenerateFixture builds the fixture from a clean database via the real
// AutoMigrate + Seed path, then writes it where the committed fixture lives.
// The bytes are not reproducible across runs by design: the seeded admin
// password is a bcrypt hash with a random salt and rows carry creation
// timestamps. Stability is therefore asserted on the semantic fingerprint
// (per-table counts plus marker), not on sha256.
func regenerateFixture(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(fixturePath), 0o755); err != nil {
		t.Fatal(err)
	}
	built := filepath.Join(t.TempDir(), "built.sqlite")
	db := openFixtureDB(t, built)
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate for fixture build: %v", err)
	}
	if err := Seed(db); err != nil {
		t.Fatalf("Seed for fixture build: %v", err)
	}
	// Compact the file: AutoMigrate churns pages across the 90 migrated
	// tables; VACUUM rewrites it to the minimal page set before the freeze.
	if err := db.Exec("VACUUM").Error; err != nil {
		t.Fatalf("VACUUM for fixture build: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	copyFixtureFile(t, built, fixturePath)
	t.Logf("regenerated %s", fixturePath)
}

func openFixtureDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite file %s: %v", path, err)
	}
	return db
}

type fixtureCounts struct {
	admins int64
	roles  int64
	menus  int64
}

func snapshotFixtureCounts(t *testing.T, db *gorm.DB) fixtureCounts {
	t.Helper()
	var counts fixtureCounts
	if err := db.Model(&model.Admin{}).Count(&counts.admins).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Role{}).Count(&counts.roles).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Menu{}).Count(&counts.menus).Error; err != nil {
		t.Fatal(err)
	}
	return counts
}

func fixtureMarkerPresent(t *testing.T, db *gorm.DB) bool {
	t.Helper()
	var count int64
	if err := db.Model(&model.Menu{}).Where("value = ?", routePermissionsMarkerValue).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count > 0
}

func copyFixtureFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
