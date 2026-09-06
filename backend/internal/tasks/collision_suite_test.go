// T34 (plan §6, PR 19) — the claim-collision suite (r2 H): barrier-started
// N-claimer×M-round races prove the single-winner invariant under -race, and
// a DETERMINISTIC-INTERLEAVE CONTROL GROUP proves the instrument itself can
// detect non-atomicity — with the read→decide→write boundary synchronized by
// force, an unguarded SELECT→UPDATE double-claims while the engine's exact CAS
// predicate still yields exactly one winner. Falsifiability, not tautology.
//
// This file also anchors the tier-2 env-gated MySQL suites (J5, r2 B): the
// openMySQLEngineFixture helper here is shared by the idempotency and
// uniqueness suites.
package tasks

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	mysqlconfig "github.com/go-sql-driver/mysql"

	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
)

// claimerResult is one claimer's outcome in a race round.
type claimerResult struct {
	worker  int
	won     bool
	err     error
	attempt int
}

// runClaimRace runs one barrier-synchronized race round: N engines (one per
// worker id, all over the same db/registry) call ClaimNext simultaneously;
// exactly one must win. Returns the per-worker results.
func runClaimRace(t *testing.T, f *engineFixture, workers int) []claimerResult {
	t.Helper()
	engines := make([]*Engine, workers)
	for i := range engines {
		cfg := testConfig()
		cfg.WorkerID = fmt.Sprintf("racer-%d", i)
		engines[i] = NewEngine(f.db, f.reg, cfg)
	}

	ready := &sync.WaitGroup{}
	ready.Add(workers)
	start := make(chan struct{})
	results := make(chan claimerResult, workers)
	ctx := context.Background()
	for i := 0; i < workers; i++ {
		go func(i int) {
			ready.Done()
			<-start
			claim, ok, err := engines[i].ClaimNext(ctx)
			res := claimerResult{worker: i, won: ok, err: err}
			if ok {
				res.attempt = claim.Attempt.AttemptNo
			}
			results <- res // exactly one send per claimer — collection waits for all
		}(i)
	}
	ready.Wait()
	close(start)

	out := make([]claimerResult, 0, workers)
	for i := 0; i < workers; i++ {
		out = append(out, <-results)
	}
	return out
}

// assertSingleWinner checks the per-round invariant: exactly one winner with
// attempt_no 1, every other claimer a clean no-task (ok=false, err=nil).
func assertSingleWinner(t *testing.T, round int, results []claimerResult) {
	t.Helper()
	wins := 0
	for _, r := range results {
		if r.err != nil {
			t.Fatalf("round %d: claimer %d errored: %v", round, r.worker, r.err)
		}
		if !r.won {
			continue
		}
		wins++
		if r.attempt != 1 {
			t.Errorf("round %d: winner claimant %d attempt_no = %d, want 1", round, r.worker, r.attempt)
		}
	}
	if wins != 1 {
		t.Fatalf("round %d: winners = %d, want exactly 1 (single-winner invariant, r2 H)", round, wins)
	}
}

// T34 — TestParallelClaimsSingleWinner: M rounds; each round seeds one due
// task and releases N claimers simultaneously; every round has exactly one
// winner, the loser set is clean no-task, and the winner's terminal commit
// frees the next round. Runs under -race as gate ①' sibling — the CAS (§13.2
// single-statement claim + §13.4 version predicate) is the whole defense.
func TestParallelClaimsSingleWinner(t *testing.T) {
	t.Run("barrier-started racers: exactly one winner per round", func(t *testing.T) {
		const workers, rounds = 8, 4
		f := newGuardedFixture(t, false)
		ctx := context.Background()

		var totalAttempts int64
		for round := 0; round < rounds; round++ {
			seedTask(t, f.db, func(task *model.ProviderTask) {
				task.ResourceUID = fmt.Sprintf("res-race-%d", round)
			})
			results := runClaimRace(t, f, workers)
			assertSingleWinner(t, round, results)

			// The single winner terminal-commits so the next round starts clean.
			var claimed model.ProviderTask
			if err := f.db.Where("resource_uid = ? AND status = ?", fmt.Sprintf("res-race-%d", round), TaskStatusRunning).
				First(&claimed).Error; err != nil {
				t.Fatalf("round %d: no running row after the race: %v", round, err)
			}
			claim := &ClaimedTask{Task: claimed}
			if err := f.db.Model(&model.TaskAttempt{}).Where("task_id = ?", claimed.ID).
				First(&claim.Attempt).Error; err != nil {
				t.Fatalf("round %d: load winner attempt: %v", round, err)
			}
			if err := f.eng.Complete(ctx, claim, nil); err != nil {
				t.Fatalf("round %d: winner Complete: %v", round, err)
			}
		}

		// Exactly one attempt per round ever existed — no double claims.
		f.db.Model(&model.TaskAttempt{}).Count(&totalAttempts)
		if totalAttempts != rounds {
			t.Fatalf("attempt rows across %d rounds = %d, want exactly %d (one claim per round)", rounds, totalAttempts, rounds)
		}
	})

	// Deterministic interleave control group (r2 H-3 falsifiability): force
	// EVERY claimer past its read before ANY write happens. The unguarded
	// SELECT→UPDATE shape must double-claim under that schedule — proving the
	// harness can catch non-atomicity. The engine's exact CAS predicate under
	// the same schedule still elects exactly one winner.
	t.Run("control: forced interleave double-claims without the CAS guard", func(t *testing.T) {
		const workers = 4
		f := newGuardedFixture(t, false)
		seeded := seedTask(t, f.db, func(task *model.ProviderTask) {
			task.ResourceUID = "res-control-unguarded"
		})

		readThrough := &sync.WaitGroup{} // symmetric barrier: read→(all)→write
		readThrough.Add(workers)
		wins := make(chan int, workers)
		for i := 0; i < workers; i++ {
			go func() {
				// READ: the same unguarded candidate every claimer sees.
				var task model.ProviderTask
				if err := f.db.Where("status = ? AND next_attempt_at <= ?", TaskStatusQueued, time.Now()).
					Order("next_attempt_at ASC, id ASC").First(&task).Error; err != nil {
					wins <- 0
					return
				}
				readThrough.Done()
				readThrough.Wait() // interleave boundary: all reads done, zero writes yet
				// WRITE: no status guard, no version predicate — the naive shape.
				res := f.db.Model(&model.ProviderTask{}).Where("id = ?", task.ID).
					Updates(map[string]any{"status": TaskStatusRunning, "attempt_count": gorm.Expr("attempt_count + 1")})
				if res.Error != nil {
					wins <- 0
					return
				}
				wins <- int(res.RowsAffected)
			}()
		}
		winners := 0
		for i := 0; i < workers; i++ {
			if n := <-wins; n > 0 {
				winners++
			}
		}
		if winners != workers {
			t.Fatalf("unguarded interleave winners = %d, want %d — the double-claim signature the instrument must catch", winners, workers)
		}
		got := reloadTask(t, f.db, seeded.ID)
		if got.AttemptCount != workers {
			t.Errorf("attempt_count = %d after the unguarded race, want %d (every claimer bumped it)", got.AttemptCount, workers)
		}
	})

	t.Run("control: the engine CAS predicate stays single-winner under the same interleave", func(t *testing.T) {
		const workers = 4
		f := newGuardedFixture(t, false)
		seeded := seedTask(t, f.db, func(task *model.ProviderTask) {
			task.ResourceUID = "res-control-cas"
		})
		now := time.Now()

		readThrough := &sync.WaitGroup{}
		readThrough.Add(workers)
		wins := make(chan int, workers)
		for i := 0; i < workers; i++ {
			go func() {
				var task model.ProviderTask
				if err := f.db.Where("status = ? AND next_attempt_at <= ?", TaskStatusQueued, now).
					Order("next_attempt_at ASC, id ASC").First(&task).Error; err != nil {
					wins <- 0
					return
				}
				readThrough.Done()
				readThrough.Wait()
				// The engine's exact claimTask predicate (engine.go): the
				// §13.2 CAS + §13.4 version guard — the row flips at most once.
				res := f.db.Model(&model.ProviderTask{}).
					Where("id = ? AND status = ? AND next_attempt_at <= ? AND version = ?",
						task.ID, TaskStatusQueued, now, task.Version).
					Updates(map[string]any{"status": TaskStatusRunning, "version": task.Version + 1})
				if res.Error != nil {
					wins <- 0
					return
				}
				wins <- int(res.RowsAffected)
			}()
		}
		winners := 0
		for i := 0; i < workers; i++ {
			if n := <-wins; n > 0 {
				winners++
			}
		}
		if winners != 1 {
			t.Fatalf("CAS interleave winners = %d, want exactly 1 — the guard is what closes the double claim", winners)
		}
		got := reloadTask(t, f.db, seeded.ID)
		if got.AttemptCount != 0 {
			t.Errorf("attempt_count = %d, want 0 (the engine bumps it inside the claim tx)", got.AttemptCount)
		}
		if got.Status != TaskStatusRunning {
			t.Errorf("status = %q, want running (the one winner flipped it)", got.Status)
		}
	})
}

// ---------------------------------------------------------------------------
// Tier 2 — env-gated real-MySQL fixtures (J5, r2 B). Shared by the
// collision/idempotency/uniqueness suites; skipped without
// OPS_TASKENGINE_MYSQL_DSN. The scratch schema is created and dropped by the
// test itself (A16) — the operational schema is never touched.
// ---------------------------------------------------------------------------

// openMySQLEngineFixture opens the MySQL scratch schema named
// "<DSN-database>-<suffix>", applies the full guarded migration (steps
// 0000–0003), registers the default operation, and returns a ready fixture.
// Each suffix gets its own schema so package-parallel test binaries and
// sibling tier-2 tests never collide.
func openMySQLEngineFixture(t *testing.T, suffix string) *engineFixture {
	t.Helper()
	dsn := mysqlDSNOrSkip(t)
	cfg, err := mysqlconfig.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse OPS_TASKENGINE_MYSQL_DSN: %v", err)
	}
	base := cfg.DBName
	if base == "" {
		base = "ops_v2gate"
	}
	scratch := base + "_" + suffix
	cfg.DBName = scratch
	cfg.ParseTime = true
	cfg.Loc = time.Local

	noDB := *cfg
	noDB.DBName = ""
	admin, err := sql.Open("mysql", noDB.FormatDSN())
	if err != nil {
		t.Fatalf("dial mysql without schema: %v", err)
	}
	if _, err := admin.Exec("DROP DATABASE IF EXISTS " + scratch); err != nil {
		t.Fatalf("drop scratch schema %s: %v", scratch, err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + scratch + " CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("create scratch schema %s: %v", scratch, err)
	}
	// LIFO with the gorm close below: pool closes first, then the schema
	// drops, then the admin handle — never a drop through a closed handle.
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS " + scratch)
		_ = admin.Close()
	})

	db, err := gorm.Open(gormmysql.Open(cfg.FormatDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open scratch schema %s: %v", scratch, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access pooled handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := runMigrateWithLockRetry(ctx, t, db); err != nil {
		t.Fatalf("migrate.Run over clean MySQL schema %s: %v", scratch, err)
	}
	reg := registry.New()
	if err := reg.RegisterOperation(testOperation("fake.workload.restart")); err != nil {
		t.Fatalf("seed operation: %v", err)
	}
	return &engineFixture{db: db, reg: reg, eng: NewEngine(db, reg, testConfig())}
}

// mysqlDSNOrSkip returns the tier-2 DSN or skips the test (env-gated, §10).
func mysqlDSNOrSkip(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("OPS_TASKENGINE_MYSQL_DSN")
	if dsn == "" {
		t.Skip("OPS_TASKENGINE_MYSQL_DSN not set — tier-2 MySQL suite skipped (set it to run the real-MySQL engine suites)")
	}
	return dsn
}

// runMigrateWithLockRetry — GET_LOCK is SERVER-scoped: the advisory lock name
// is shared by every runner against this MySQL server (single-instance
// production posture, A15). When `go test` launches the migrate and tasks
// test BINARIES in parallel, the sibling binary's run legitimately holds the
// lock; the runner's zero-timeout fail-fast is per-run by design, so the
// TEST retries the whole run until the sibling releases (bounded — a run
// holds the lock for seconds only). Any non-lock error returns immediately.
func runMigrateWithLockRetry(ctx context.Context, t *testing.T, db *gorm.DB) error {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for {
		err := migrate.Run(ctx, db)
		if err == nil || !strings.Contains(err.Error(), "advisory lock") {
			return err
		}
		if time.Now().After(deadline) {
			return err
		}
		t.Logf("advisory lock held by the sibling test binary — retrying the migration run")
		time.Sleep(500 * time.Millisecond)
	}
}

// TestMySQLParallelClaims — tier 2 (J5 의무): the claim race on real MySQL
// DML. N claimers over real connections fight for one due row per round
// (REPEATABLE READ, locking UPDATE … WHERE) — every round still elects
// exactly one winner and one attempt row.
func TestMySQLParallelClaims(t *testing.T) {
	const workers, rounds = 8, 3
	f := openMySQLEngineFixture(t, "claims")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	engines := make([]*Engine, workers)
	for i := range engines {
		cfg := testConfig()
		cfg.WorkerID = fmt.Sprintf("mysql-racer-%d", i)
		engines[i] = NewEngine(f.db, f.reg, cfg)
	}

	for round := 0; round < rounds; round++ {
		uid := fmt.Sprintf("res-mysql-race-%d", round)
		if _, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   uid,
		}); err != nil {
			t.Fatalf("round %d submit: %v", round, err)
		}

		ready := &sync.WaitGroup{}
		ready.Add(workers)
		start := make(chan struct{})
		results := make(chan claimerResult, workers)
		for i := 0; i < workers; i++ {
			go func(i int) {
				ready.Done()
				<-start
				claim, ok, err := engines[i].ClaimNext(ctx)
				res := claimerResult{worker: i, won: ok, err: err}
				if ok {
					res.attempt = claim.Attempt.AttemptNo
				}
				results <- res
			}(i)
		}
		ready.Wait()
		close(start)
		roundResults := make([]claimerResult, 0, workers)
		for i := 0; i < workers; i++ {
			roundResults = append(roundResults, <-results)
		}
		assertSingleWinner(t, round, roundResults)

		// The winner's terminal commit frees the resource for the next round.
		var claimed model.ProviderTask
		if err := f.db.Where("resource_uid = ? AND status = ?", uid, TaskStatusRunning).
			First(&claimed).Error; err != nil {
			t.Fatalf("round %d: no running row after the MySQL race: %v", round, err)
		}
		var attempt model.TaskAttempt
		if err := f.db.Where("task_id = ?", claimed.ID).First(&attempt).Error; err != nil {
			t.Fatalf("round %d: load winner attempt: %v", round, err)
		}
		if err := f.eng.Complete(ctx, &ClaimedTask{Task: claimed, Attempt: attempt}, nil); err != nil {
			t.Fatalf("round %d: winner Complete: %v", round, err)
		}
		var attempts int64
		f.db.Model(&model.TaskAttempt{}).Where("task_id = ?", claimed.ID).Count(&attempts)
		if attempts != 1 {
			t.Fatalf("round %d: attempt rows = %d, want 1 (single claim on real MySQL)", round, attempts)
		}
	}
}
