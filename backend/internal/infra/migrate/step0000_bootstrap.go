package migrate

import "gorm.io/gorm"

// step0000Bootstrap creates the schema_migration version table itself. The
// bootstrap must be raw DDL: the version table cannot depend on the very
// bookkeeping it exists to record (plan §2 PR 15, file 3).
var step0000Bootstrap = Step{
	Version: 0,
	Name:    "bootstrap_schema_migration",
	Run: func(db *gorm.DB) error {
		// CREATE TABLE IF NOT EXISTS under the W-5 existence guard — the
		// guarded bootstrap stays explicit about its idempotence (T17).
		return execIfTableAbsent(db, "schema_migration", `CREATE TABLE IF NOT EXISTS schema_migration (
	version BIGINT PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	dirty BOOLEAN NOT NULL DEFAULT FALSE,
	applied_at TIMESTAMP NOT NULL
)`)
	},
}
