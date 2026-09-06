package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	mysqlconfig "github.com/go-sql-driver/mysql"
	gormmysqldriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// openMemoryRunDB opens a single-connection in-memory sqlite database: the
// sqlite leg of the runner assumes no concurrent-runner contention, which a
// one-connection database guarantees by construction (A15).
func openMemoryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access pooled handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	return db
}

// markerStep builds a step that records its own invocations. The marker DDL
// is itself existence-guarded, so a second application of the same step list
// would be a no-op even without the applied-version bookkeeping — the counter
// is what proves the runner skips applied steps instead of re-running them.
func markerStep(version int64, name string, runs *int) Step {
	table := fmt.Sprintf("migrate_marker_%d", version)
	return Step{
		Version: version,
		Name:    name,
		Run: func(db *gorm.DB) error {
			*runs++
			return execIfTableAbsent(db, table, fmt.Sprintf(
				"CREATE TABLE IF NOT EXISTS %s (marker TEXT NOT NULL)", table))
		},
	}
}

func countApplied(t *testing.T, db *gorm.DB, version int64) (int, SchemaMigration) {
	t.Helper()
	var rows []SchemaMigration
	if err := db.Where("version = ?", version).Find(&rows).Error; err != nil {
		t.Fatalf("query schema_migration for version %d: %v", version, err)
	}
	if len(rows) == 0 {
		return 0, SchemaMigration{}
	}
	return len(rows), rows[0]
}

// T14 — TestRunnerAppliesPendingInOrder: three steps apply in registered
// order, every version is recorded in schema_migration (dirty=false), and a
// re-run is a no-op (no re-execution, no duplicate version rows).
func TestRunnerAppliesPendingInOrder(t *testing.T) {
	db := openMemoryRunDB(t)

	var runsA, runsB, runsC int
	list := []Step{
		markerStep(1, "marker_a", &runsA),
		markerStep(2, "marker_b", &runsB),
		markerStep(3, "marker_c", &runsC),
	}

	if err := applyPending(db, list); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if runsA != 1 || runsB != 1 || runsC != 1 {
		t.Fatalf("first run executed each step once, got a=%d b=%d c=%d", runsA, runsB, runsC)
	}

	applied, err := loadApplied(db)
	if err != nil {
		t.Fatalf("load applied versions: %v", err)
	}
	if len(applied) != 3 {
		t.Fatalf("schema_migration rows = %d, want 3", len(applied))
	}
	for _, step := range list {
		row, ok := applied[step.Version]
		if !ok {
			t.Fatalf("version %d not recorded in schema_migration", step.Version)
		}
		if row.Name != step.Name {
			t.Errorf("version %d recorded as %q, want %q", step.Version, row.Name, step.Name)
		}
		if row.Dirty {
			t.Errorf("version %d recorded dirty, want clean", step.Version)
		}
		if row.AppliedAt.IsZero() {
			t.Errorf("version %d recorded without applied_at", step.Version)
		}
	}

	if err := applyPending(db, list); err != nil {
		t.Fatalf("re-run: %v", err)
	}
	if runsA != 1 || runsB != 1 || runsC != 1 {
		t.Fatalf("re-run must be a no-op, got a=%d b=%d c=%d", runsA, runsB, runsC)
	}
	for _, step := range list {
		if n, _ := countApplied(t, db, step.Version); n != 1 {
			t.Errorf("version %d recorded %d times after re-run, want exactly 1", step.Version, n)
		}
	}
}

// T15 — TestRunnerDirtyBlocksSubsequentRuns: an injected step failure marks
// the version dirty, and every subsequent run refuses with an error that
// names the dirty state and the manual resolution procedure (R1 — automatic
// recovery of a torn step is forbidden).
func TestRunnerDirtyBlocksSubsequentRuns(t *testing.T) {
	db := openMemoryRunDB(t)

	var runsOK int
	list := []Step{
		markerStep(1, "marker_ok", &runsOK),
		{
			Version: 2,
			Name:    "marker_boom",
			Run: func(db *gorm.DB) error {
				return fmt.Errorf("injected step failure")
			},
		},
	}

	err := applyPending(db, list)
	if err == nil {
		t.Fatal("run with a failing step must error")
	}
	if !strings.Contains(err.Error(), "marker_boom") {
		t.Errorf("error must name the failing step, got: %v", err)
	}
	if runsOK != 1 {
		t.Errorf("clean step ran %d times, want 1", runsOK)
	}

	n, row := countApplied(t, db, 2)
	if n != 1 {
		t.Fatalf("failed version recorded %d times, want 1", n)
	}
	if !row.Dirty {
		t.Error("failed version must be recorded with dirty=true")
	}

	retry := applyPending(db, list)
	if retry == nil {
		t.Fatal("run over a dirty version must be refused")
	}
	if !strings.Contains(retry.Error(), "dirty") {
		t.Errorf("refusal must cite the dirty flag, got: %v", retry)
	}
	if !strings.Contains(retry.Error(), "schema_migration") {
		t.Errorf("refusal must name the manual resolution table, got: %v", retry)
	}
	if runsOK != 1 {
		t.Errorf("refused run re-executed a clean step (%d times), want no execution", runsOK)
	}
}

// T17 — TestBootstrapIsIdempotent: step0000 applied twice is a no-op (raw
// bootstrap DDL under the W-5 existence guard) and a full double run records
// version 0 exactly once.
func TestBootstrapIsIdempotent(t *testing.T) {
	db := openMemoryRunDB(t)

	for i := 0; i < 2; i++ {
		if err := step0000Bootstrap.Run(db); err != nil {
			t.Fatalf("bootstrap application %d: %v", i+1, err)
		}
	}
	exists, err := tableExists(db, "schema_migration")
	if err != nil {
		t.Fatalf("existence check: %v", err)
	}
	if !exists {
		t.Fatal("bootstrap left no schema_migration table")
	}

	for i := 0; i < 2; i++ {
		if err := applyPending(db, steps); err != nil {
			t.Fatalf("full run %d: %v", i+1, err)
		}
	}
	if n, _ := countApplied(t, db, 0); n != 1 {
		t.Fatalf("bootstrap version recorded %d times over double run, want 1", n)
	}
}

// lockProbePool is the hand-made gorm.ConnPool wrapper demanded by §3.1
// (T16 — sqlmock and friends are new dependencies and forbidden, preservation
// constraint #4). It wraps exactly one *sql.Conn (an in-memory sqlite
// connection), records every advisory-lock statement, and rewrites the lock
// statements — plus the mysql dialector's version probe — into SQL the
// pinned conn can answer.
type lockProbePool struct {
	conn           gorm.ConnPool
	lockCalls      []string
	acquireReturns string // value the rewritten GET_LOCK probe yields
}

func (p *lockProbePool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.conn.PrepareContext(ctx, query)
}

func (p *lockProbePool) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return p.conn.ExecContext(ctx, p.rewrite(query), args...)
}

func (p *lockProbePool) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return p.conn.QueryContext(ctx, p.rewrite(query), args...)
}

func (p *lockProbePool) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return p.conn.QueryRowContext(ctx, p.rewrite(query), args...)
}

func (p *lockProbePool) rewrite(query string) string {
	returns := p.acquireReturns
	if returns == "" {
		returns = "1"
	}
	switch {
	case strings.Contains(query, "GET_LOCK"):
		p.lockCalls = append(p.lockCalls, query)
		return fmt.Sprintf("SELECT %s", returns)
	case strings.Contains(query, "RELEASE_LOCK"):
		p.lockCalls = append(p.lockCalls, query)
		return "SELECT 1"
	case strings.Contains(query, "VERSION()"):
		return "SELECT '8.0.36'"
	}
	return query
}

// T16 — TestRunnerLockIsAdvisory: on the MySQL path the lock is acquired and
// released exactly once through the pinned ConnPool, the step callback
// receives a session bound to that same single pinned pool (T-4/W-8: the
// whole run on one sql.Conn), a failed acquisition aborts before any step
// runs (A15 fail-fast), and sqlite bypasses the lock without error.
func TestRunnerLockIsAdvisory(t *testing.T) {
	// sqlite bypass (A15): the full run completes with no lock statements.
	if err := Run(context.Background(), openMemoryRunDB(t)); err != nil {
		t.Fatalf("sqlite bypass run: %v", err)
	}

	// MySQL path through the hand-made wrapper over ONE pinned *sql.Conn.
	memory := openMemoryRunDB(t)
	sqlDB, err := memory.DB()
	if err != nil {
		t.Fatalf("access pooled handle: %v", err)
	}
	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		t.Fatalf("pin a raw conn: %v", err)
	}
	defer conn.Close()
	probe := &lockProbePool{conn: conn}

	ran := false
	err = runPinned(context.Background(), probe, func(pinned *gorm.DB) error {
		ran = true
		var one int64
		if err := pinned.Raw("SELECT 1").Scan(&one).Error; err != nil {
			return err
		}
		// The session handed to the step executor must be built on the very
		// pool the runner pinned — every query lands on that one conn.
		if pinned.Statement == nil || pinned.Statement.ConnPool != gorm.ConnPool(probe) {
			t.Errorf("step session is not bound to the pinned pool")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("runPinned: %v", err)
	}
	if !ran {
		t.Fatal("lock run never reached the step callback")
	}
	if len(probe.lockCalls) != 2 {
		t.Fatalf("lock statements = %d (%q), want exactly one acquire and one release", len(probe.lockCalls), probe.lockCalls)
	}
	if !strings.Contains(probe.lockCalls[0], "GET_LOCK") {
		t.Errorf("first lock statement = %q, want GET_LOCK acquisition", probe.lockCalls[0])
	}
	if !strings.Contains(probe.lockCalls[1], "RELEASE_LOCK") {
		t.Errorf("second lock statement = %q, want RELEASE_LOCK", probe.lockCalls[1])
	}

	// Failed acquisition (another runner holds the lock): abort before any
	// step runs — waiting is not the single-instance posture (A15).
	held := &lockProbePool{conn: conn, acquireReturns: "0"}
	refused := runPinned(context.Background(), held, func(*gorm.DB) error {
		t.Error("step callback must not run when the lock is held elsewhere")
		return nil
	})
	if refused == nil {
		t.Fatal("run with a held advisory lock must abort")
	}
	if !strings.Contains(refused.Error(), "advisory lock") {
		t.Errorf("refusal must cite the advisory lock, got: %v", refused)
	}
}

// TestMySQLMigrationSuite is the tier-2 env-gated real-MySQL suite (J5,
// r2 B): it runs only when OPS_TASKENGINE_MYSQL_DSN is set and otherwise
// skips with an explicit message (plan §10). The suite manages its own
// scratch database — created and dropped by the test itself, never touching
// the operational schema (A16) — and proves the clean-install equivalence of
// the registered steps on MySQL 8.0. It grows alongside the step list.
func TestMySQLMigrationSuite(t *testing.T) {
	dsn := os.Getenv("OPS_TASKENGINE_MYSQL_DSN")
	if dsn == "" {
		t.Skip("OPS_TASKENGINE_MYSQL_DSN not set — tier-2 MySQL migration suite skipped (set it to run the real-MySQL clean-install suite)")
	}

	cfg, err := mysqlconfig.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse OPS_TASKENGINE_MYSQL_DSN: %v", err)
	}
	if cfg.DBName == "" {
		cfg.DBName = "ops_v2gate"
	}
	scratch := cfg.DBName
	cfg.ParseTime = true
	cfg.Loc = time.Local

	// Scratch schema lifecycle (A16): drop, create, run, drop.
	noDB := *cfg
	noDB.DBName = ""
	admin, err := sql.Open("mysql", noDB.FormatDSN())
	if err != nil {
		t.Fatalf("dial mysql without schema: %v", err)
	}
	defer admin.Close()
	if _, err := admin.Exec("DROP DATABASE IF EXISTS " + scratch); err != nil {
		t.Fatalf("drop scratch schema: %v", err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + scratch + " CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("create scratch schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS " + scratch)
	})

	db, err := gorm.Open(gormmysqldriver.Open(cfg.FormatDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open scratch schema: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := Run(ctx, db); err != nil {
		t.Fatalf("migration run over clean MySQL schema: %v", err)
	}

	var row SchemaMigration
	if err := db.Where("version = ?", int64(0)).First(&row).Error; err != nil {
		t.Fatalf("bootstrap version not recorded on MySQL: %v", err)
	}
	if row.Dirty {
		t.Error("bootstrap version recorded dirty on MySQL")
	}

	before, err := loadApplied(db)
	if err != nil {
		t.Fatalf("load applied: %v", err)
	}
	if err := Run(ctx, db); err != nil {
		t.Fatalf("re-run over migrated MySQL schema: %v", err)
	}
	after, err := loadApplied(db)
	if err != nil {
		t.Fatalf("reload applied: %v", err)
	}
	if len(before) != len(after) {
		t.Errorf("re-run changed the recorded versions %d -> %d, want a no-op", len(before), len(after))
	}
}
