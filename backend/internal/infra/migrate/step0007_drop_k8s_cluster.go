package migrate

import (
	"fmt"

	"gorm.io/gorm"
)

// step0007DropK8sCluster — version 7: the §19.1 step 5 DROP (V2 Phase 6 I-b —
// phase6-plan §J10·§3.3). The legacy k8s_cluster table retires: since I-a the
// service layer resolves clusters through the provider_connection + SecretRef
// chain (S5) and the S6 equivalent lookups already read provider_connection,
// so the table has no reader left. Rollback = backup restore (C34) +
// register-k8s re-registration only — the runner is forward-only and no down
// migration is authored (R-Q; §11).
//
// Data closure (§J10): backfill-chain connection rows carry
// source_model='k8s_cluster' + a source_id pointing into the dropped table —
// they are marked stale_source=true (§5.4b — rows are never deleted), which
// closes the legacy id space for live reads. register-k8s chains carry no
// source pair and stay live.
//
// S3 columns (R-C): asset_service.k8s_cluster_id and
// ops_application_env_binding.k8s_cluster_id drop with the table — inbound FK
// constraints measured 0 (C35) and dev rows 0 (I-b gate), so no data
// migration arises. Every drop is guarded, so the step re-applies as a no-op
// (W-5).
//
// Postlude contract: the drop touches k8s_cluster + the 2 S3 columns +
// provider_connection.stale_source ONLY — no V2 core table (preservation
// constraint #5) and no generated-column guard (EnsureProviderTaskGuards
// duty) arises.
var step0007DropK8sCluster = Step{
	Version: 7,
	Name:    "drop_k8s_cluster",
	Run: func(db *gorm.DB) error {
		if err := dropTableIfPresent(db, "k8s_cluster"); err != nil {
			return err
		}
		// §J10 data closure — mark, don't delete (§5.4b). 0/1 literals work
		// on both dialects (MySQL tinyint, sqlite integer).
		if err := db.Exec(
			"UPDATE provider_connection SET stale_source = 1 WHERE provider_type = ? AND source_model = ? AND stale_source = 0",
			"kubernetes", "k8s_cluster",
		).Error; err != nil {
			return fmt.Errorf("migrate: step 0007 stale backfill chains: %w", err)
		}
		if err := dropColumnIfPresent(db, "asset_service", "k8s_cluster_id"); err != nil {
			return err
		}
		if err := dropColumnIfPresent(db, "ops_application_env_binding", "k8s_cluster_id"); err != nil {
			return err
		}
		return nil
	},
}

// dropTableIfPresent — W-5 idempotence guard for raw DROP: executes only when
// the table is present (tableExists — runner.go). Unlike CREATE, DROP is not
// naturally idempotent, so the guard is mandatory. The table name is a
// compile-time constant — no injection surface.
func dropTableIfPresent(db *gorm.DB, table string) error {
	exists, err := tableExists(db, table)
	if err != nil {
		return fmt.Errorf("migrate: step 0007 existence check for table %s: %w", table, err)
	}
	if !exists {
		return nil
	}
	if err := db.Exec("DROP TABLE " + table).Error; err != nil {
		return fmt.Errorf("migrate: step 0007 drop table %s: %w", table, err)
	}
	return nil
}

// dropColumnIfPresent — the column-level counterpart (columnExists —
// step0003_task_guards.go) for ALTER TABLE DROP COLUMN. On sqlite the S3
// columns never existed, so the guard makes the whole step a no-op there.
func dropColumnIfPresent(db *gorm.DB, table, column string) error {
	present, err := columnExists(db, table, column)
	if err != nil {
		return fmt.Errorf("migrate: step 0007 existence check for column %s.%s: %w", table, column, err)
	}
	if !present {
		return nil
	}
	if err := db.Exec("ALTER TABLE " + table + " DROP COLUMN " + column).Error; err != nil {
		return fmt.Errorf("migrate: step 0007 drop column %s.%s: %w", table, column, err)
	}
	return nil
}
