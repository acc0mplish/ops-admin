package migrate

import (
	"fmt"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/model"
)

// step0003TaskGuards — version 3: the provider_task guard Expand (plan §2 PR
// 18, §3.4). Two halves:
//
//   - approval columns + the idempotency unique (uq_provider_task_idempotency)
//     arrive through AutoMigrate — they are plain model columns/tags
//     (model/task.go).
//   - the §13.4 resource-uniqueness guard CANNOT be a model tag (r2 C): a
//     generated column is not a model field, and a plain unique tag on
//     ResourceUID would wrongly make resource_uid globally unique. It is raw
//     DDL, owned by EnsureProviderTaskGuards below.
//
// Frozen at PR 18 merge (W-4); later schema changes land as step 0004+.
var step0003TaskGuards = Step{
	Version: 3,
	Name:    "task_guards",
	Run: func(db *gorm.DB) error {
		if err := db.AutoMigrate(&model.ProviderTask{}); err != nil {
			return fmt.Errorf("migrate: step 0003 AutoMigrate provider_task: %w", err)
		}
		if err := EnsureProviderTaskGuards(db); err != nil {
			return fmt.Errorf("migrate: step 0003 guard postlude: %w", err)
		}
		return nil
	},
}

// EnsureProviderTaskGuards is the W-5 postlude — the existence-checked raw DDL
// for the §13.4 per-resource active uniqueness guard:
//
//	active_flag TINYINT GENERATED ALWAYS AS
//	  (CASE WHEN status IN ('succeeded','failed','timed_out','cancelled') THEN NULL ELSE 1 END) STORED
//	UNIQUE KEY uq_provider_task_resource_active (resource_uid, active_flag)
//
// Terminal rows compute to NULL and drop out of the unique — the standard
// workaround for MySQL's lack of partial indexes (A5; the full invariant —
// single active, multiple terminals, terminal-transition-then-reactivate —
// verified on glebarez sqlite, plan R7). The CASE list of four is exactly
// the terminal Status set (r2 A — 동치 with state.go IsTerminalTaskStatus/T24).
//
// Contract (r2 C/W-5): step0003 is the ORIGIN of this DDL — every later step
// that touches provider_task MUST re-run this postlude after its own DDL.
// Every statement is existence-checked, so re-application is a no-op when
// the guards survive. Exported so the contract is assertable (T31).
func EnsureProviderTaskGuards(db *gorm.DB) error {
	// ① generated column — checked through pragma_table_xinfo /
	// information_schema.columns: generated columns are invisible to
	// pragma_table_info ([실측 E-3h]).
	exists, err := columnExists(db, "provider_task", "active_flag")
	if err != nil {
		return fmt.Errorf("migrate: active_flag existence check: %w", err)
	}
	if !exists {
		if res := db.Exec(activeFlagColumnDDL); res.Error != nil {
			return fmt.Errorf("migrate: add active_flag generated column: %w", res.Error)
		}
	}
	// ② composite unique — checked through sqlite_master /
	// information_schema.statistics.
	exists, err = indexExists(db, "provider_task", "uq_provider_task_resource_active")
	if err != nil {
		return fmt.Errorf("migrate: uq_provider_task_resource_active existence check: %w", err)
	}
	if !exists {
		ddl := "ALTER TABLE provider_task ADD UNIQUE KEY uq_provider_task_resource_active (resource_uid, active_flag)"
		if db.Dialector.Name() != dialectMySQL {
			// sqlite cannot ALTER a table constraint — the guard is a named
			// unique index (both dialects surface the same index name).
			ddl = "CREATE UNIQUE INDEX IF NOT EXISTS uq_provider_task_resource_active ON provider_task (resource_uid, active_flag)"
		}
		if res := db.Exec(ddl); res.Error != nil {
			return fmt.Errorf("migrate: add uq_provider_task_resource_active: %w", res.Error)
		}
	}
	return nil
}

// activeFlagColumnDDL — the generated column, identical for both dialects
// (verified on glebarez sqlite, plan R7; MySQL 8 standard syntax). The CASE
// list is the four-terminal set — edit it only with state.go (r2 A 동치).
const activeFlagColumnDDL = "ALTER TABLE provider_task ADD COLUMN active_flag TINYINT GENERATED ALWAYS AS (CASE WHEN status IN ('succeeded','failed','timed_out','cancelled') THEN NULL ELSE 1 END) STORED"

// columnExists — sqlite: pragma_table_xinfo (xinfo, not table_info: generated
// columns are hidden from the latter, [실측 E-3h]); MySQL:
// information_schema.columns scoped to the current database.
func columnExists(db *gorm.DB, table, column string) (bool, error) {
	var count int64
	if db.Dialector.Name() == dialectMySQL {
		err := db.Raw(
			"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?",
			table, column).Scan(&count).Error
		return count > 0, err
	}
	// The pragma table-valued function takes its argument in call position —
	// inline the table name (a package-constant identifier, never user input)
	// rather than relying on bind support inside pragma calls across builds.
	err := db.Raw(
		fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_xinfo('%s') WHERE name = ?", table),
		column).Scan(&count).Error
	return count > 0, err
}

// indexExists — sqlite: sqlite_master (type='index'); MySQL:
// information_schema.statistics (one row per index column — COUNT of DISTINCT
// INDEX_NAME).
func indexExists(db *gorm.DB, table, index string) (bool, error) {
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND name = ?"
	if db.Dialector.Name() == dialectMySQL {
		query = "SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?"
	}
	var count int64
	if err := db.Raw(query, table, index).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
