// T36 (plan §6, PR 19) — the per-resource uniqueness suite under concurrency
// (§13.4/N11): when N submissions for the SAME resource race, the step0003
// generated-column guard (active_flag + uq_provider_task_resource_active)
// admits exactly one active mutation and returns resource_busy to the rest —
// the DB-level invariant the engine surfaces, complementary to T30's
// sequential leg and T31's row-surgery shape. Gate ③ (plan §7 claim 7). The
// tier-2 MySQL leg proves the generated column and composite unique behave on
// real MySQL (J5).
package tasks

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"ops-admin/backend/internal/infra/model"
)

// T36 — TestResourceBusyUnderConcurrency: concurrent identical-resource
// submits produce exactly one task; every other caller gets an immediate
// ErrResourceBusy; and once the winner goes terminal the resource frees.
func TestResourceBusyUnderConcurrency(t *testing.T) {
	t.Run("concurrent submits: exactly one task, the rest resource_busy", func(t *testing.T) {
		const callers = 8
		f := newGuardedFixture(t, false)
		ctx := context.Background()

		ready := &sync.WaitGroup{}
		ready.Add(callers)
		start := make(chan struct{})
		type outcome struct {
			uid string
			err error
		}
		outcomes := make(chan outcome, callers)
		for i := 0; i < callers; i++ {
			go func() {
				ready.Done()
				<-start
				task, _, err := f.eng.Submit(ctx, SubmitInput{
					OperationName: "fake.workload.restart",
					ResourceUID:   "res-busy-race",
				})
				outcomes <- outcome{uid: task.UID, err: err}
			}()
		}
		ready.Wait()
		close(start)

		created := 0
		busy := 0
		uids := map[string]bool{}
		for i := 0; i < callers; i++ {
			o := <-outcomes
			switch {
			case o.err == nil:
				created++
				uids[o.uid] = true
			case errors.Is(o.err, ErrResourceBusy):
				busy++
			default:
				t.Fatalf("concurrent submit returned neither a task nor resource_busy: %v", o.err)
			}
		}
		if created != 1 {
			t.Fatalf("created tasks = %d, want exactly 1 under a %d-way race (N11)", created, callers)
		}
		if busy != callers-1 {
			t.Fatalf("resource_busy rejections = %d, want exactly %d", busy, callers-1)
		}
		if len(uids) != 1 {
			t.Fatalf("distinct created UIDs = %d, want 1 — %v", len(uids), uids)
		}
		var rows int64
		f.db.Model(&model.ProviderTask{}).Where("resource_uid = ?", "res-busy-race").Count(&rows)
		if rows != 1 {
			t.Fatalf("provider_task rows on the resource = %d, want 1 — the guard is the last line of defense", rows)
		}
	})

	t.Run("terminal commit frees the resource for the next mutation", func(t *testing.T) {
		f := newGuardedFixture(t, false)
		ctx := context.Background()

		first, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-busy-free",
		})
		if err != nil {
			t.Fatalf("first submit: %v", err)
		}
		if _, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-busy-free",
		}); !errors.Is(err, ErrResourceBusy) {
			t.Fatalf("second active submit err = %v, want ErrResourceBusy", err)
		}

		// The winner runs to a terminal through the engine path (not row
		// surgery — T31 owns that leg): the STORED column recomputes on the
		// terminal UPDATE and the unique releases the resource.
		claim, ok, err := f.eng.ClaimNext(ctx)
		if err != nil || !ok || claim.Task.ID != first.ID {
			t.Fatalf("claim the winner: (ok=%v, task=%v, err=%v)", ok, claim, err)
		}
		if err := f.eng.Complete(ctx, claim, nil); err != nil {
			t.Fatalf("Complete the winner: %v", err)
		}

		second, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-busy-free",
		})
		if err != nil {
			t.Fatalf("submit after terminal: %v — the terminal must free the resource (§13.4)", err)
		}
		if second.UID == first.UID {
			t.Fatal("post-terminal submit returned the OLD task — a new mutation needs a new task")
		}
		// The terminal row and the new active row coexist on one resource.
		var rows int64
		f.db.Model(&model.ProviderTask{}).Where("resource_uid = ?", "res-busy-free").Count(&rows)
		if rows != 2 {
			t.Fatalf("rows on the resource after terminal+resubmit = %d, want 2 (NULL-excluded terminal)", rows)
		}
	})
}

// TestMySQLResourceUniqueness — tier 2 (J5 의무): the §23.1
// "resource-uniqueness rejection" behavior on real MySQL — the step0003
// generated column exists (information_schema generation_expression), the
// composite unique fires as MySQL 1062 and maps to resource_busy, and
// terminal rows NULL-exclude so the resource reactivates.
func TestMySQLResourceUniqueness(t *testing.T) {
	f := openMySQLEngineFixture(t, "uniq")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// (a) active_flag is a STORED generated column on MySQL — the guard shape.
	var generated int64
	if err := f.db.Raw(
		"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'provider_task' AND column_name = 'active_flag' AND generation_expression <> ''",
	).Scan(&generated).Error; err != nil {
		t.Fatalf("probe active_flag on information_schema: %v", err)
	}
	if generated != 1 {
		t.Fatalf("active_flag generated-column rows = %d, want 1 (step0003 shape on MySQL)", generated)
	}
	var uniqueIndex int64
	if err := f.db.Raw(
		"SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'provider_task' AND index_name = 'uq_provider_task_resource_active'",
	).Scan(&uniqueIndex).Error; err != nil {
		t.Fatalf("probe uq_provider_task_resource_active on information_schema: %v", err)
	}
	if uniqueIndex != 1 {
		t.Fatalf("uq_provider_task_resource_active rows = %d, want 1", uniqueIndex)
	}

	// (b) The second active mutation rejects through the MySQL 1062 → index
	// name → resource_busy mapping (T-6 dialect leg).
	first, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-mysql-uniq-1",
	})
	if err != nil {
		t.Fatalf("first submit on MySQL: %v", err)
	}
	if _, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-mysql-uniq-1",
	}); !errors.Is(err, ErrResourceBusy) {
		t.Fatalf("second active submit on MySQL err = %v, want ErrResourceBusy (1062 → uq_provider_task_resource_active)", err)
	}

	// (c) Terminal rows compute to NULL and drop out of the unique (A5) —
	// the resource accepts a new active mutation on real MySQL too.
	if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", first.ID).
		Update("status", TaskStatusSucceeded).Error; err != nil {
		t.Fatalf("terminalize first: %v", err)
	}
	second, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-mysql-uniq-1",
	})
	if err != nil {
		t.Fatalf("resubmit after terminal on MySQL: %v — NULL rows must be unique-excluded", err)
	}
	if second.UID == first.UID {
		t.Fatal("post-terminal resubmit returned the old task on MySQL")
	}
	// A second active row alongside the new active one is still busy.
	if _, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-mysql-uniq-1",
	}); !errors.Is(err, ErrResourceBusy) {
		t.Fatalf("second active submit after reactivation err = %v, want ErrResourceBusy", err)
	}
}
