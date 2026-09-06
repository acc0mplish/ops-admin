// Package testutil holds the shared test harness of the secret-migration
// suite. Go keeps a package's internal tests beside its code (unexported
// identifiers are only visible in-package), so the _test.go files themselves
// cannot leave their package directory; what can live here is everything the
// growing test files would otherwise duplicate: environment pinning and the
// in-memory fixture database.
package testutil

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/util"
)

// PinSecretKeys pins the secret key state for one test: an empty
// OPS_SECRET_MASTER_KEYS env (so first-use parsing cannot pick up the outer
// environment), the implicit master key set, and a fixed credential seed.
// The previous state is restored on cleanup so tests cannot pollute each
// other's key material.
func PinSecretKeys(t *testing.T) {
	t.Helper()
	t.Setenv("OPS_SECRET_MASTER_KEYS", "")
	if err := util.ConfigureSecretMasterKeys(""); err != nil {
		t.Fatal(err)
	}
	util.ConfigureCredentialKey("testutil-pinned-credential-seed")
	t.Cleanup(func() {
		util.ConfigureCredentialKey("")
		_ = util.ConfigureSecretMasterKeys("")
	})
}

// OpenMemoryDB opens a single-connection in-memory sqlite database and runs
// the given DDL statements. The single connection is required: ":memory:"
// gives every new connection its own empty database, so batched queries must
// not be able to race onto a second one.
func OpenMemoryDB(t *testing.T, statements ...string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}
