// T35 (plan §6, PR 19) — the idempotency suite, N6 full cycle (§13.4): the
// same key submitted twice is ONE task and ONE provider execution — the
// assertion instrument is the fake adapter's atomic execution counter
// (§23.4 "execution count asserted at the provider double"). T28 (guards
// suite) proved the submit-level replay; this suite closes the loop through
// execution. Gate ② (plan §7 claim 6). The tier-2 MySQL leg proves the 1062
// dialect mapping on real MySQL (T-6, J5).
package tasks

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
)

// T35 — TestIdempotencySingleExecutionAtProvider: submit(key) → RunOnce →
// succeeded; a second submit with the same key replays the SAME task; the
// next cycle has nothing to claim; the provider double executed EXACTLY once.
func TestIdempotencySingleExecutionAtProvider(t *testing.T) {
	t.Run("replay after success: provider executes exactly once", func(t *testing.T) {
		f := newGuardedFakeFixture(t, false)
		ctx := context.Background()

		first, replayed, err := f.eng.Submit(ctx, SubmitInput{
			OperationName:  "fake.workload.restart",
			ResourceUID:    "res-uid-1", // the fixture's seeded three-level chain (T-7)
			Payload:        contract.JSONMap{"n": 1},
			IdempotencyKey: "idem-exec-1",
		})
		if err != nil {
			t.Fatalf("first submit: %v", err)
		}
		if _, err := f.eng.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce 1: %v", err)
		}
		if got := reloadTask(t, f.db, first.ID); got.Status != TaskStatusSucceeded {
			t.Fatalf("after RunOnce status = %q, want succeeded", got.Status)
		}

		// The replay: same key, same resource — replay wins over resource_busy
		// (N6 answers "has this already run?", not "is the resource free?").
		second, replayed, err := f.eng.Submit(ctx, SubmitInput{
			OperationName:  "fake.workload.restart",
			ResourceUID:    "res-uid-1",
			Payload:        contract.JSONMap{"n": 1},
			IdempotencyKey: "idem-exec-1",
		})
		if err != nil {
			t.Fatalf("replay submit: %v", err)
		}
		if !replayed {
			t.Error("replay submit replayed = false, want true (§13.4)")
		}
		if second.UID != first.UID || second.ID != first.ID {
			t.Fatalf("replay returned task %s (id %d), want the original %s (id %d)", second.UID, second.ID, first.UID, first.ID)
		}

		// Nothing left to claim — the replay must NOT enqueue a second run.
		processed, err := f.eng.RunOnce(ctx)
		if err != nil {
			t.Fatalf("RunOnce after replay: %v", err)
		}
		if processed {
			t.Fatal("RunOnce after replay processed a task — the replayed task was re-enqueued")
		}

		// 게이트 ② instrument: the provider double ran EXACTLY once.
		if got := f.fake.ExecCount(); got != 1 {
			t.Fatalf("fake ExecCount = %d, want exactly 1 (N6 provider-double assertion)", got)
		}
		// And the audit trail stays exactly the happy-path sequence — the
		// replay appended nothing.
		want := []string{TaskEventCreated, TaskEventClaimed, TaskEventSucceeded}
		if types := eventTypes(eventsOf(t, f.db, first.ID)); len(types) != len(want) {
			t.Fatalf("event sequence after replay = %v, want exactly %v", types, want)
		} else {
			for i := range want {
				if types[i] != want[i] {
					t.Fatalf("event sequence after replay = %v, want exactly %v", types, want)
				}
			}
		}
		var rows int64
		f.db.Model(&model.ProviderTask{}).Where("idempotency_key = ?", "idem-exec-1").Count(&rows)
		if rows != 1 {
			t.Errorf("provider_task rows holding the key = %d, want 1", rows)
		}
	})

	t.Run("concurrent same-key submits: one task, one row, identical UID", func(t *testing.T) {
		const callers = 8
		f := newGuardedFakeFixture(t, false)
		ctx := context.Background()

		ready := &sync.WaitGroup{}
		ready.Add(callers)
		start := make(chan struct{})
		type outcome struct {
			uid      string
			replayed bool
			err      error
		}
		outcomes := make(chan outcome, callers)
		for i := 0; i < callers; i++ {
			go func() {
				ready.Done()
				<-start
				task, replayed, err := f.eng.Submit(ctx, SubmitInput{
					OperationName:  "fake.workload.restart",
					ResourceUID:    "res-idem-race",
					Payload:        contract.JSONMap{"n": 1},
					IdempotencyKey: "idem-race-1",
				})
				outcomes <- outcome{uid: task.UID, replayed: replayed, err: err}
			}()
		}
		ready.Wait()
		close(start)

		uids := map[string]bool{}
		replays := 0
		for i := 0; i < callers; i++ {
			o := <-outcomes
			if o.err != nil {
				t.Fatalf("concurrent submit errored: %v (single-conn serialization makes every caller a winner-or-replay)", o.err)
			}
			if o.uid == "" {
				t.Fatal("concurrent submit returned an empty UID")
			}
			uids[o.uid] = true
			if o.replayed {
				replays++
			}
		}
		if len(uids) != 1 {
			t.Fatalf("distinct task UIDs across %d same-key submits = %d, want 1 — %v", callers, len(uids), uids)
		}
		if replays != callers-1 {
			t.Errorf("replayed callers = %d, want exactly %d (one creator, the rest replays)", replays, callers-1)
		}
		var rows int64
		f.db.Model(&model.ProviderTask{}).Count(&rows)
		if rows != 1 {
			t.Fatalf("provider_task rows = %d, want 1 (the race still created exactly one task)", rows)
		}
	})
}

// TestMySQLIdempotencyReplay — tier 2 (J5 의무): the §13.4 replay against real
// MySQL, where the duplicate key surfaces as MySQL error 1062 and
// classifyUniqueViolation must map it by index name to the replay path
// (T-6 MySQL 방언). Same key twice → same task, replayed=true, one row.
func TestMySQLIdempotencyReplay(t *testing.T) {
	f := openMySQLEngineFixture(t, "idem")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	first, replayed, err := f.eng.Submit(ctx, SubmitInput{
		OperationName:  "fake.workload.restart",
		ResourceUID:    "res-mysql-idem-1",
		Payload:        contract.JSONMap{"n": 1},
		IdempotencyKey: "mysql-idem-1",
	})
	if err != nil {
		t.Fatalf("first submit on MySQL: %v", err)
	}
	if replayed {
		t.Fatal("first submit replayed = true, want false")
	}

	second, replayed, err := f.eng.Submit(ctx, SubmitInput{
		OperationName:  "fake.workload.restart",
		ResourceUID:    "res-mysql-idem-1",
		Payload:        contract.JSONMap{"n": 1},
		IdempotencyKey: "mysql-idem-1",
	})
	if err != nil {
		t.Fatalf("replay submit on MySQL (1062 mapping leg): %v", err)
	}
	if !replayed {
		t.Error("MySQL replay submit replayed = false, want true — the 1062 path did not resolve to the replay")
	}
	if second.UID != first.UID {
		t.Fatalf("MySQL replay returned %s, want the original %s", second.UID, first.UID)
	}
	var rows int64
	if err := f.db.Model(&model.ProviderTask{}).Where("idempotency_key = ?", "mysql-idem-1").
		Count(&rows).Error; err != nil {
		t.Fatalf("count keyed rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("rows holding the key = %d, want 1", rows)
	}

	// A resource_busy miss WITHOUT a key stays a hard error on MySQL too —
	// the 1062 mapping must not swallow the uniqueness guard.
	_, _, err = f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-mysql-idem-1",
	})
	if !errors.Is(err, ErrResourceBusy) {
		t.Fatalf("keyless second submit on MySQL err = %v, want ErrResourceBusy (the guard is not swallowed by the replay path)", err)
	}
}
