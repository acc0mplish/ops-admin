package migrate

import "gorm.io/gorm"

// Step is one versioned migration (plan §2 PR 15, file 2).
type Step struct {
	// Version is the monotonic schema version recorded in schema_migration.
	Version int64
	// Name is the stable human-readable identifier recorded beside the
	// version.
	Name string
	// Run applies the schema change against the database session pinned for
	// the migration run (single advisory-lock connection on MySQL — §3.1).
	// Steps must be idempotent (W-5): raw DDL goes through the
	// existence-checked executors in runner.go, model steps through
	// AutoMigrate.
	Run func(db *gorm.DB) error
}

// steps is the migration list in application order (§3.2). Later PRs extend
// it by exactly one line each — this list is the sequential serialization
// point between PRs.
//
// W-4 (불변 스텝): a step that has been applied once is immutable — editing
// or retracting it is forbidden (preservation constraint #11, §1 J6); every
// schema change lands as a new step (0004+) appended below.
var steps = []Step{
	step0000Bootstrap,
	// PR 16 appends step0001InfraFoundation here (one line).
}
