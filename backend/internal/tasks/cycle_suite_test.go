// T37/T38/T39 (plan §6, PR 19) — the full-cycle suite. Gate ④: the fake
// adapter drives plan→approve→execute→audit end to end over the REAL
// migration schema with the three-level execution chain (connection→context→
// infra_resource, r2 G/T-7) seeded, and the task_event sequence is asserted
// EXACT — zero extra events. T38 persists the §13.4 cancel flag to the next
// claim boundary; T39 proves Start/Stop leave no goroutine behind and that
// the deterministic entry points run concurrently with Stop without
// deadlocking.
package tasks

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/adapter/fake"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
)

// newGuardedFakeFixture is the gate-④ stage: the fake adapter wired into an
// engine over the REAL production schema (migrate.Run, steps 0000–0003 — the
// step0003 guards in force) with the T-7 three-level chain seeded. Shared by
// the idempotency suite.
func newGuardedFakeFixture(t *testing.T, requiresApproval bool) *fakeFixture {
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
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run (steps 0000–0003): %v", err)
	}
	reg := registry.New()
	if err := fake.Register(reg); err != nil {
		t.Fatalf("fake.Register: %v", err)
	}
	def := testOperation("fake.workload.restart")
	def.RequiresApproval = requiresApproval
	if err := reg.RegisterOperation(def); err != nil {
		t.Fatalf("seed operation: %v", err)
	}
	base := &engineFixture{db: db, reg: reg, eng: NewEngine(db, reg, testConfig())}
	_, adapter, _ := reg.ProviderType("fake")
	fa, _ := adapter.(*fake.Adapter)
	return &fakeFixture{
		engineFixture: base,
		fake:          fa,
		urn:           seedChain(t, db, "fake", "res-uid-1"),
	}
}

// T37 — TestFakeFullCyclePlanApproveExecuteAudit (gate ④): Submit lands at
// awaiting_approval (§13.3 snapshot), Approve queues it, RunOnce executes it
// through the fake chain (null handle → synchronous terminal), and the
// task_event log is EXACTLY [created approved claimed succeeded] — a complete
// audit with zero extra events. The reject leg terminal-cancels without any
// execution.
func TestFakeFullCyclePlanApproveExecuteAudit(t *testing.T) {
	t.Run("approve leg: exact four-event cycle", func(t *testing.T) {
		f := newGuardedFakeFixture(t, true)
		ctx := context.Background()

		task, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-uid-1",
			Payload:       contract.JSONMap{"reason": "gate-iv"},
		})
		if err != nil {
			t.Fatalf("Submit: %v", err)
		}
		if task.Status != TaskStatusAwaitingApproval {
			t.Fatalf("submitted status = %q, want awaiting_approval (§13.3)", task.Status)
		}
		// The gate path never executes while awaiting approval.
		if processed, err := f.eng.RunOnce(ctx); err != nil || processed {
			t.Fatalf("RunOnce while awaiting approval = (processed=%v, err=%v), want (false, nil)", processed, err)
		}
		if got := f.fake.ExecCount(); got != 0 {
			t.Fatalf("ExecCount while awaiting approval = %d, want 0 — the gate holds execution", got)
		}

		if err := f.eng.Approve(ctx, task.UID, "alice"); err != nil {
			t.Fatalf("Approve: %v", err)
		}
		approved := reloadTask(t, f.db, task.ID)
		if approved.Status != TaskStatusQueued {
			t.Fatalf("post-Approve status = %q, want queued", approved.Status)
		}
		if approved.ApprovalStatus != ApprovalStatusApproved || approved.Approver != "alice" || approved.ApprovalAt == nil {
			t.Fatalf("approval columns = (%q, %q, %v), want (approved, alice, set)", approved.ApprovalStatus, approved.Approver, approved.ApprovalAt)
		}

		processed, err := f.eng.RunOnce(ctx)
		if err != nil || !processed {
			t.Fatalf("RunOnce after approval = (processed=%v, err=%v), want (true, nil)", processed, err)
		}
		final := reloadTask(t, f.db, task.ID)
		if final.Status != TaskStatusSucceeded || final.FinishedAt == nil {
			t.Fatalf("final = (%q, finished=%v), want (succeeded, set)", final.Status, final.FinishedAt)
		}

		// 게이트 ④ — the audit sequence is EXACT: zero extra events.
		events := eventsOf(t, f.db, task.ID)
		want := []string{TaskEventCreated, TaskEventApproved, TaskEventClaimed, TaskEventSucceeded}
		types := eventTypes(events)
		if len(types) != len(want) {
			t.Fatalf("full-cycle event sequence = %v, want EXACTLY %v (extra events: %d)", types, want, len(types)-len(want))
		}
		for i := range want {
			if types[i] != want[i] {
				t.Fatalf("full-cycle event sequence = %v, want EXACTLY %v", types, want)
			}
		}
		// The approval audit carries its actor; the execution audit carries
		// the attempt it belongs to.
		if events[1].Actor != "alice" {
			t.Errorf("approved event actor = %q, want alice", events[1].Actor)
		}
		if events[3].AttemptNo != 1 {
			t.Errorf("succeeded event attempt_no = %d, want 1", events[3].AttemptNo)
		}
		// Execution went through the seeded three-level chain (T-7): the
		// engine assembled the URN from infra_resource.external_urn.
		if got := f.fake.LastRequest().ResourceURN; got != f.urn {
			t.Errorf("OperationRequest.ResourceURN = %q, want %q (engine-owned UID→URN)", got, f.urn)
		}
		if got := f.fake.ExecCount(); got != 1 {
			t.Errorf("ExecCount = %d, want exactly 1 over the full cycle", got)
		}
	})

	t.Run("reject leg: terminal cancel, no execution, resource freed", func(t *testing.T) {
		f := newGuardedFakeFixture(t, true)
		ctx := context.Background()

		task, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-uid-1",
		})
		if err != nil {
			t.Fatalf("Submit: %v", err)
		}
		if err := f.eng.Reject(ctx, task.UID, "bob"); err != nil {
			t.Fatalf("Reject: %v", err)
		}
		if processed, err := f.eng.RunOnce(ctx); err != nil || processed {
			t.Fatalf("RunOnce after rejection = (processed=%v, err=%v), want (false, nil)", processed, err)
		}
		if got := f.fake.ExecCount(); got != 0 {
			t.Fatalf("ExecCount after rejection = %d, want 0 — rejection never executes", got)
		}
		final := reloadTask(t, f.db, task.ID)
		if final.Status != TaskStatusCancelled || !IsTerminalTaskStatus(final.Status) {
			t.Fatalf("rejected status = %q, want cancelled (terminal — never a 'rejected' Status, r2 A)", final.Status)
		}
		types := eventTypes(eventsOf(t, f.db, task.ID))
		want := []string{TaskEventCreated, TaskEventRejected}
		if len(types) != len(want) || types[0] != want[0] || types[1] != want[1] {
			t.Fatalf("reject-leg events = %v, want EXACTLY %v", types, want)
		}
		// The terminal freed the resource: a fresh mutation is accepted.
		if _, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-uid-1",
		}); err != nil {
			t.Fatalf("resubmit after rejection: %v — the terminal must free the resource (§13.4)", err)
		}
	})
}

// T38 — TestCancelRequestedPersists: the §13.4 cancel flag is a durable
// column — set on a queued task it survives until the next claim boundary,
// where the task terminal-cancels WITHOUT ever being handed to an executor.
// (Phase 1 has no public RequestCancel verb yet — the flag is set directly on
// the row; the boundary contract under test is the engine's.)
func TestCancelRequestedPersists(t *testing.T) {
	f := newGuardedFakeFixture(t, false)
	ctx := context.Background()

	task, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// The flag lands on the row and persists across reads — §13.4 "requested
	// via status flag", acted on at the NEXT claim boundary, not before.
	if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", task.ID).
		Update("cancel_requested", true).Error; err != nil {
		t.Fatalf("set cancel_requested: %v", err)
	}
	flagged := reloadTask(t, f.db, task.ID)
	if !flagged.CancelRequested {
		t.Fatal("cancel_requested did not persist on the row")
	}
	if flagged.Status != TaskStatusQueued {
		t.Fatalf("flagged task status = %q, want still queued — the flag itself does not transition", flagged.Status)
	}
	// The reaper is not the cancel path either: a flagged queued task with no
	// lease is not the reaper's business.
	if requeued, err := f.eng.ReapOnce(ctx); err != nil || requeued != 0 {
		t.Fatalf("ReapOnce on a flagged queued task = (%d, %v), want (0, nil)", requeued, err)
	}

	// The next boundary reflects it: RunOnce claims, sees the flag, and
	// terminal-cancels without execution.
	processed, err := f.eng.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce at the cancel boundary: %v", err)
	}
	if processed {
		t.Fatal("RunOnce reported a processed claim — a cancel-flagged task must not be handed to an executor")
	}
	if got := f.fake.ExecCount(); got != 0 {
		t.Fatalf("ExecCount at the cancel boundary = %d, want 0 — cancelled before execution", got)
	}
	final := reloadTask(t, f.db, task.ID)
	if final.Status != TaskStatusCancelled || final.FinishedAt == nil {
		t.Fatalf("final = (%q, finished=%v), want (cancelled, set) at the boundary (§13.4)", final.Status, final.FinishedAt)
	}
	if !IsTerminalTaskStatus(final.Status) {
		t.Error("boundary cancel is not terminal")
	}
	types := eventTypes(eventsOf(t, f.db, task.ID))
	want := []string{TaskEventCreated, TaskEventClaimed, TaskEventCancelled}
	if len(types) != len(want) {
		t.Fatalf("cancel-boundary events = %v, want EXACTLY %v", types, want)
	} else {
		for i := range want {
			if types[i] != want[i] {
				t.Fatalf("cancel-boundary events = %v, want EXACTLY %v", types, want)
			}
		}
	}
	// The terminal cancel freed the resource for a fresh mutation.
	if _, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
	}); err != nil {
		t.Fatalf("resubmit after boundary cancel: %v", err)
	}
}

// T39 — TestStartStopNoGoroutineLeak (r2 H): after Start drives real cycles
// and Stop returns, the goroutine count returns to the pre-Start baseline
// (polled with a timeout — never a blind sleep), and Stop is safe to call
// while deterministic entry points (RunOnce/ReapOnce) are still in flight.
func TestStartStopNoGoroutineLeak(t *testing.T) {
	t.Run("loops return the goroutine count to baseline", func(t *testing.T) {
		f := newFakeFixture(t, Config{
			WorkerID:     "leak-worker",
			PollInterval: 5 * time.Millisecond,
			LeaseSeconds: 60,
			ReaperGrace:  time.Second,
		})
		task, _, err := f.eng.Submit(context.Background(), SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-uid-1",
		})
		if err != nil {
			t.Fatalf("Submit: %v", err)
		}

		baseline := runtime.NumGoroutine()
		ctx, cancel := context.WithCancel(context.Background())
		f.eng.Start(ctx)
		// Real work flows through the loops before stopping.
		deadline := time.Now().Add(5 * time.Second)
		for {
			if got := reloadTask(t, f.db, task.ID); got.Status == TaskStatusSucceeded {
				break
			} else if time.Now().After(deadline) {
				t.Fatalf("loops never completed the task — status %q", got.Status)
			}
			time.Sleep(2 * time.Millisecond)
		}
		f.eng.Stop()
		f.eng.Stop() // idempotent
		cancel()

		// Poll with a timeout: transient goroutines may take a moment to exit.
		settleDeadline := time.Now().Add(5 * time.Second)
		slack := 2
		for {
			now := runtime.NumGoroutine()
			if now <= baseline+slack {
				break
			}
			if time.Now().After(settleDeadline) {
				t.Fatalf("goroutines after Stop = %d, want <= baseline %d + %d (leaked loop goroutines)", now, baseline, slack)
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	t.Run("Stop concurrent with in-flight RunOnce/ReapOnce: no deadlock", func(t *testing.T) {
		f := newFakeFixture(t, Config{
			WorkerID:     "concurrent-worker",
			PollInterval: 3 * time.Millisecond,
			LeaseSeconds: 60,
			ReaperGrace:  time.Second,
		})
		hammerCtx, hammerCancel := context.WithCancel(context.Background())
		f.eng.Start(hammerCtx)

		// Deterministic entry points hammer alongside the loops.
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for hammerCtx.Err() == nil {
					_, _ = f.eng.RunOnce(hammerCtx)
					_, _ = f.eng.ReapOnce(hammerCtx)
				}
			}()
		}

		time.Sleep(50 * time.Millisecond) // let the concurrent cycles interleave
		hammerCancel()
		wg.Wait()

		stopped := make(chan struct{})
		go func() {
			f.eng.Stop() // must return even with everything winding down
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(10 * time.Second):
			t.Fatal("Stop deadlocked while deterministic entry points were winding down")
		}
	})
}
