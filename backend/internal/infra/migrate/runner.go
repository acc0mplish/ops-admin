// Package migrate implements the v2 versioned migration runner (spec §19 —
// "schema version table; one migration runner at a time; database advisory
// lock"; plan §2 PR 15): ordered, immutable steps recorded in a
// schema_migration table, guarded by a database advisory lock held for the
// whole run.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	dialectMySQL = "mysql"

	// lockName is the advisory lock key (A15). A zero timeout on GET_LOCK:
	// in the single-instance deployment there is no contention worth waiting
	// out, so failing startup immediately is the honest posture — a second
	// runner must not queue behind a possibly-torn run.
	lockName = "ops-admin:v2-migrate"
)

// SchemaMigration is §19's schema version table (plan §3.1). One row per
// applied step; dirty=true marks a partially-failed step requiring manual
// resolution (MySQL DDL is non-transactional — auto-retry of a torn step is
// riskier than stopping).
type SchemaMigration struct {
	// Version is a plain (non-auto-increment) key: version 0 is the
	// bootstrap step and must be insertable as a literal zero.
	Version   int64  `gorm:"primaryKey;autoIncrement:false"`
	Name      string `gorm:"size:255;not null"`
	Dirty     bool   `gorm:"not null;default:false"`
	AppliedAt time.Time
}

// TableName pins the table name that the raw bootstrap DDL in
// step0000_bootstrap.go creates.
func (SchemaMigration) TableName() string { return "schema_migration" }

// Run applies every pending migration step, in registered order, under the
// advisory lock. A runner failure blocks v1 startup fail-closed (W-1/R12) —
// main.go wires this before any serving path.
func Run(ctx context.Context, db *gorm.DB) error {
	if db.Dialector.Name() == dialectMySQL {
		sqlDB, err := db.DB()
		if err != nil {
			return fmt.Errorf("migrate: access pooled database handle: %w", err)
		}
		return withAdvisoryLock(ctx, sqlDB, func(pinned *gorm.DB) error {
			return applyPending(pinned, steps)
		})
	}
	// sqlite (tests): a single-connection in-memory database has no
	// concurrent-runner contention to guard, so the lock is bypassed (A15).
	return applyPending(db, steps)
}

// withAdvisoryLock pins one sql.Conn for the entire run and executes fn
// against a session bound to it (T-4/W-8).
func withAdvisoryLock(ctx context.Context, sqlDB *sql.DB, fn func(*gorm.DB) error) error {
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migrate: pin advisory-lock connection: %w", err)
	}
	defer conn.Close()
	return runPinned(ctx, conn, fn)
}

// runPinned acquires the advisory lock on pool, hands fn a gorm session bound
// to that very pool, and releases the lock after fn returns. The lock is
// session-scoped (MySQL GET_LOCK): pinning one ConnPool for acquisition,
// every step, and the release is what keeps it alive for the whole run —
// executing steps on a separate pool would silently drop it (§3.1 T-4/W-8).
func runPinned(ctx context.Context, pool gorm.ConnPool, fn func(*gorm.DB) error) error {
	if err := acquireAdvisoryLock(ctx, pool); err != nil {
		return err
	}
	defer func() { _ = releaseAdvisoryLock(ctx, pool) }()

	pinned, err := openLockedSession(pool)
	if err != nil {
		return fmt.Errorf("migrate: open pinned session: %w", err)
	}
	return fn(pinned)
}

// openLockedSession binds a gorm session to the pinned pool so that every
// query — steps included — runs on the advisory-lock connection.
func openLockedSession(pool gorm.ConnPool) (*gorm.DB, error) {
	return gorm.Open(mysql.New(mysql.Config{Conn: pool}), &gorm.Config{})
}

// acquireAdvisoryLock executes GET_LOCK on the pinned pool. Any outcome but
// "acquired" aborts the run (A15).
func acquireAdvisoryLock(ctx context.Context, pool gorm.ConnPool) error {
	var acquired int64
	if err := pool.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", lockName).Scan(&acquired); err != nil {
		return fmt.Errorf("migrate: acquire advisory lock %q: %w", lockName, err)
	}
	if acquired != 1 {
		return fmt.Errorf("migrate: advisory lock %q is held by another runner (GET_LOCK returned %d) — aborting startup", lockName, acquired)
	}
	return nil
}

// releaseAdvisoryLock executes RELEASE_LOCK on the pinned pool. It runs in a
// deferred call and cannot un-fail the run; the lock also dies with the
// connection, so a release error is recorded but not escalated.
func releaseAdvisoryLock(ctx context.Context, pool gorm.ConnPool) error {
	if _, err := pool.ExecContext(ctx, "SELECT RELEASE_LOCK(?)", lockName); err != nil {
		return fmt.Errorf("migrate: release advisory lock %q: %w", lockName, err)
	}
	return nil
}

// applyPending executes every registered step whose version is not yet
// recorded, refusing to start while a dirty row exists.
//
// W-4 (불변 스텝): the list handed in here is append-only history — applied
// steps are skipped by version, never re-interpreted. A schema change lands
// as a new step (0004+), never as an edit to an applied one (preservation
// constraint #11, §1 J6).
func applyPending(db *gorm.DB, list []Step) error {
	if err := bootstrapSchemaMigration(db); err != nil {
		return err
	}
	applied, err := loadApplied(db)
	if err != nil {
		return err
	}
	if version, name, dirty := findDirty(applied); dirty {
		return fmt.Errorf(
			"migrate: step %d (%s) is marked dirty — manual resolution required: repair or drop the torn DDL, delete its dirty schema_migration row (version %d), then re-run (plan §11); automatic recovery is forbidden (R1)",
			version, name, version)
	}

	for _, step := range list {
		if _, done := applied[step.Version]; done {
			continue
		}
		if err := executeStep(db, step); err != nil {
			return err
		}
	}
	return nil
}

// bootstrapSchemaMigration guarantees the version table exists before any
// bookkeeping reads it — the chicken-and-egg half of the bootstrap step.
func bootstrapSchemaMigration(db *gorm.DB) error {
	return step0000Bootstrap.Run(db)
}

// loadApplied indexes the recorded versions by version number.
func loadApplied(db *gorm.DB) (map[int64]SchemaMigration, error) {
	var rows []SchemaMigration
	if err := db.Order("version").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("migrate: read schema_migration: %w", err)
	}
	applied := make(map[int64]SchemaMigration, len(rows))
	for _, row := range rows {
		applied[row.Version] = row
	}
	return applied, nil
}

// findDirty reports a recorded dirty row, if any.
func findDirty(applied map[int64]SchemaMigration) (int64, string, bool) {
	for version, row := range applied {
		if row.Dirty {
			return version, row.Name, true
		}
	}
	return 0, "", false
}

// executeStep runs one pending step and records its version. A failed step
// is marked dirty — a torn non-transactional DDL must never be retried
// blindly (R1) — and the failure propagates to block startup (W-1).
func executeStep(db *gorm.DB, step Step) error {
	if err := step.Run(db); err != nil {
		if markErr := markDirty(db, step); markErr != nil {
			return fmt.Errorf("migrate: step %d (%s) failed: %w (and marking it dirty failed: %v)", step.Version, step.Name, err, markErr)
		}
		return fmt.Errorf("migrate: step %d (%s) failed and was marked dirty in schema_migration: %w", step.Version, step.Name, err)
	}
	record := SchemaMigration{Version: step.Version, Name: step.Name, AppliedAt: time.Now()}
	if err := db.Create(&record).Error; err != nil {
		return fmt.Errorf("migrate: record version %d (%s): %w", step.Version, step.Name, err)
	}
	return nil
}

// markDirty records the failed step as dirty, creating the row when the
// failure happened before any record existed.
func markDirty(db *gorm.DB, step Step) error {
	var row SchemaMigration
	err := db.Where("version = ?", step.Version).First(&row).Error
	switch {
	case err == nil:
		return db.Model(&row).Update("dirty", true).Error
	case err == gorm.ErrRecordNotFound:
		torn := SchemaMigration{Version: step.Version, Name: step.Name, Dirty: true, AppliedAt: time.Now()}
		return db.Create(&torn).Error
	default:
		return fmt.Errorf("migrate: read version %d to mark dirty: %w", step.Version, err)
	}
}

// tableExists reports whether the table exists — sqlite_master on sqlite,
// information_schema.tables scoped to the current database on MySQL.
func tableExists(db *gorm.DB, table string) (bool, error) {
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?"
	if db.Dialector.Name() == dialectMySQL {
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?"
	}
	var count int64
	if err := db.Raw(query, table).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// execIfTableAbsent runs ddl only when the table does not exist yet — the
// W-5 idempotence guard for raw DDL: every raw statement passes an existence
// check first, so re-application is a no-op. Column-level guards (generation
// columns, [E-3h]: sqlite pragma_table_xinfo / MySQL information_schema.columns)
// arrive with the PR 18 postlude and must follow this same shape.
func execIfTableAbsent(db *gorm.DB, table, ddl string) error {
	exists, err := tableExists(db, table)
	if err != nil {
		return fmt.Errorf("migrate: existence check for table %s: %w", table, err)
	}
	if exists {
		return nil
	}
	if err := db.Exec(ddl).Error; err != nil {
		return fmt.Errorf("migrate: raw DDL on table %s: %w", table, err)
	}
	return nil
}
