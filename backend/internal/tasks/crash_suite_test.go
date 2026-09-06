// T32/T33 (plan §6, PR 19) — the crash-recovery suite (spec §13.2, N7, plan
// §1 J5 layer 1). The schematic essence of a crash — a running row whose owner
// vanished between claim and terminal commit (expired lease + open attempt) —
// is reproduced DETERMINISTICALLY by expiring the lease; no goroutine timing,
// no OS signal. ReapOnce must recover every such row with zero lost tasks,
// and exhausted tasks must land on the failed 'lease_expired' terminal.
// Gate ① runs this under -race (plan §7 claim 5).
package tasks

import (
	"context"
	"testing"
	"time"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
)

// expireLeaseForcesReap manipulates the crash clock (J5 layer 1): the lease is
// set beyond ReaperGrace into the past so the very next ReapOnce sees it —
// the deterministic stand-in for "worker died mid-task".
func expireLeaseForcesReap(t *testing.T, f *engineFixture, id uint) {
	t.Helper()
	expired := time.Now().Add(-(f.eng.cfg.ReaperGrace + time.Hour))
	if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", id).
		Update("lease_expires_at", expired).Error; err != nil {
		t.Fatalf("force lease expiry on task %d: %v", id, err)
	}
}

// claimOrFatal claims one task or fails the test — the crash suite always
// starts from a claimed (running) row.
func claimOrFatal(t *testing.T, f *engineFixture) *ClaimedTask {
	t.Helper()
	claim, ok, err := f.eng.ClaimNext(context.Background())
	if err != nil || !ok {
		t.Fatalf("ClaimNext: (ok=%v, err=%v) — the crash precondition is a claim", ok, err)
	}
	return claim
}

// T32 — TestCrashRecoveryReaperRequeues: claim → owner vanishes without a
// terminal commit → lease forced past the grace → ReapOnce requeues
// (status='queued', next_attempt_at=NOW), appends 'reaper_requeued', closes
// the crashed attempt, and the task continues at attempt_no+1 (유실 0).
func TestCrashRecoveryReaperRequeues(t *testing.T) {
	t.Run("owner vanishes after claim: requeue, event, attempt continuity", func(t *testing.T) {
		f := newGuardedFixture(t, false)
		ctx := context.Background()
		task, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-crash-1",
		})
		if err != nil {
			t.Fatalf("Submit: %v", err)
		}
		claim := claimOrFatal(t, f)
		if claim.Task.ID != task.ID {
			t.Fatalf("claimed task %d, want %d", claim.Task.ID, task.ID)
		}
		// The crash: no Complete/Fail ever arrives — the owner simply stops.
		expireLeaseForcesReap(t, f, task.ID)

		requeued, err := f.eng.ReapOnce(ctx)
		if err != nil {
			t.Fatalf("ReapOnce: %v", err)
		}
		if requeued != 1 {
			t.Fatalf("requeued = %d, want 1 (§13.2 re-queue when attempts remain)", requeued)
		}
		got := reloadTask(t, f.db, task.ID)
		if got.Status != TaskStatusQueued {
			t.Fatalf("status = %q, want queued (reaper re-queue, §13.2)", got.Status)
		}
		if got.LeaseExpiresAt != nil {
			t.Errorf("lease_expires_at = %v after requeue, want nil — the expired lease is cleared", got.LeaseExpiresAt)
		}
		if got.NextAttemptAt == nil || got.NextAttemptAt.After(time.Now()) {
			t.Errorf("next_attempt_at = %v, want due immediately (§13.2 next_attempt_at=NOW)", got.NextAttemptAt)
		}
		events := eventsOf(t, f.db, task.ID)
		if types := eventTypes(events); types[len(types)-1] != TaskEventReaperRequeued {
			t.Fatalf("last event = %v, want reaper_requeued (§13.2 verbatim)", types[len(types)-1])
		}
		if actor := events[len(events)-1].Actor; actor != actorReaper {
			t.Errorf("reaper event actor = %q, want %q", actor, actorReaper)
		}
		// The crashed worker's attempt is closed as an attempt outcome in the
		// same reap commit — the crash IS an attempt outcome (§13.2).
		var crashed model.TaskAttempt
		if err := f.db.First(&crashed, claim.Attempt.ID).Error; err != nil {
			t.Fatalf("load crashed attempt: %v", err)
		}
		if crashed.FinishedAt == nil || crashed.ErrorCode != ErrorCodeLeaseExpired {
			t.Fatalf("crashed attempt = (finished=%v, code=%q), want (set, %q)",
				crashed.FinishedAt, crashed.ErrorCode, ErrorCodeLeaseExpired)
		}

		// Recovery continues the task — attempt numbering picks up, and the
		// recovered claim reaches a terminal the original owner never wrote.
		second := claimOrFatal(t, f)
		if second.Task.ID != task.ID || second.Attempt.AttemptNo != 2 {
			t.Fatalf("post-reap claim = (task %d, attempt %d), want (same task, attempt 2)", second.Task.ID, second.Attempt.AttemptNo)
		}
		if err := f.eng.Complete(ctx, second, contract.JSONMap{"recovered": true}); err != nil {
			t.Fatalf("Complete after recovery: %v", err)
		}
		final := reloadTask(t, f.db, task.ID)
		if final.Status != TaskStatusSucceeded || final.FinishedAt == nil {
			t.Fatalf("recovered task = (%q, finished=%v), want (succeeded, finished)", final.Status, final.FinishedAt)
		}
		want := []string{TaskEventCreated, TaskEventClaimed, TaskEventReaperRequeued, TaskEventClaimed, TaskEventSucceeded}
		if types := eventTypes(eventsOf(t, f.db, task.ID)); len(types) != len(want) {
			t.Fatalf("event sequence = %v, want exactly %v", types, want)
		} else {
			for i := range want {
				if types[i] != want[i] {
					t.Fatalf("event sequence = %v, want exactly %v", types, want)
				}
			}
		}
	})

	t.Run("population reap: zero lost tasks, zero orphaned attempts", func(t *testing.T) {
		f := newGuardedFixture(t, false)
		ctx := context.Background()

		// Population: 3 crashed (expired leases), 1 healthy running, 1 queued,
		// 1 terminal. Only the crashed three are the reaper's business.
		var crashedIDs []uint
		for i := 0; i < 3; i++ {
			task, _, err := f.eng.Submit(ctx, SubmitInput{
				OperationName: "fake.workload.restart",
				ResourceUID:   "res-crash-pop-" + string(rune('a'+i)),
			})
			if err != nil {
				t.Fatalf("Submit crashed[%d]: %v", i, err)
			}
			claimOrFatal(t, f)
			expireLeaseForcesReap(t, f, task.ID)
			crashedIDs = append(crashedIDs, task.ID)
		}
		healthy, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-crash-healthy",
		})
		if err != nil {
			t.Fatalf("Submit healthy: %v", err)
		}
		healthyClaim := claimOrFatal(t, f) // live lease — must NOT be reaped
		// ClaimNext orders by next_attempt_at — create/claim in strict order so
		// each claimOrFatal targets exactly the task just submitted.
		terminal, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-crash-terminal",
		})
		if err != nil {
			t.Fatalf("Submit terminal: %v", err)
		}
		terminalClaim := claimOrFatal(t, f)
		if terminalClaim.Task.ID != terminal.ID {
			t.Fatalf("terminal claim picked task %d, want %d (claim-order drift)", terminalClaim.Task.ID, terminal.ID)
		}
		if err := f.eng.Complete(ctx, terminalClaim, nil); err != nil {
			t.Fatalf("Complete terminal: %v", err)
		}
		queued, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-crash-queued",
		})
		if err != nil {
			t.Fatalf("Submit queued: %v", err)
		}
		_ = queued

		requeued, err := f.eng.ReapOnce(ctx)
		if err != nil {
			t.Fatalf("ReapOnce over the population: %v", err)
		}
		if requeued != len(crashedIDs) {
			t.Fatalf("requeued = %d, want exactly the %d crashed tasks", requeued, len(crashedIDs))
		}

		// 유실 0 (N7): after one reap cycle every row is terminal, queued
		// (claimable again), or running under a LIVE lease — nothing sits in
		// running with a dead lease, the §2.8 orphaned-running defect.
		var orphans int64
		cutoff := time.Now().Add(-f.eng.cfg.ReaperGrace)
		if err := f.db.Model(&model.ProviderTask{}).
			Where("status = ? AND lease_expires_at IS NOT NULL AND lease_expires_at < ?",
				TaskStatusRunning, cutoff).
			Count(&orphans).Error; err != nil {
			t.Fatalf("count orphans: %v", err)
		}
		if orphans != 0 {
			t.Fatalf("orphaned running rows after ReapOnce = %d, want 0 (zero lost tasks, N7)", orphans)
		}
		for _, id := range crashedIDs {
			got := reloadTask(t, f.db, id)
			if got.Status != TaskStatusQueued {
				t.Errorf("crashed task %d status = %q, want queued (recovered)", id, got.Status)
			}
		}
		// The healthy worker's row and its open attempt are untouched.
		if got := reloadTask(t, f.db, healthy.ID); got.Status != TaskStatusRunning {
			t.Errorf("healthy running task status = %q, want still running (live lease is not reaped)", got.Status)
		}
		var openAttempts int64
		if err := f.db.Model(&model.TaskAttempt{}).
			Where("finished_at IS NULL").Count(&openAttempts).Error; err != nil {
			t.Fatalf("count open attempts: %v", err)
		}
		if openAttempts != 1 {
			t.Fatalf("open attempts after population reap = %d, want exactly 1 (the healthy claim %d)", openAttempts, healthyClaim.Attempt.ID)
		}
		// Queued and terminal rows are not the reaper's business either.
		if got := reloadTask(t, f.db, queued.ID); got.Status != TaskStatusQueued {
			t.Errorf("queued task status = %q, want queued (untouched by the reaper)", got.Status)
		}
		if got := reloadTask(t, f.db, terminal.ID); got.Status != TaskStatusSucceeded {
			t.Errorf("terminal task status = %q, want succeeded (untouched by the reaper)", got.Status)
		}
		// Every recovered task is claimable again in the same instant — the
		// queued pool is the 3 recovered plus the never-claimed row.
		var queuedCount int64
		if err := f.db.Model(&model.ProviderTask{}).
			Where("status = ?", TaskStatusQueued).Count(&queuedCount).Error; err != nil {
			t.Fatalf("count queued: %v", err)
		}
		if queuedCount != int64(len(crashedIDs))+1 {
			t.Fatalf("queued rows after reap = %d, want %d (3 recovered + 1 untouched)", queuedCount, len(crashedIDs)+1)
		}
		for i := 0; i < int(queuedCount); i++ {
			if _, ok, err := f.eng.ClaimNext(ctx); err != nil || !ok {
				t.Fatalf("post-reap claim %d: (ok=%v, err=%v) — recovered rows must be claimable", i, ok, err)
			}
		}
	})
}

// T33 — TestCrashRecoveryAttemptsExhausted: a task whose owner keeps crashing
// exhausts its attempts and lands on the failed 'lease_expired' terminal with
// the event recorded — the bounded-loss guarantee (§13.2).
func TestCrashRecoveryAttemptsExhausted(t *testing.T) {
	t.Run("single attempt fails terminally with lease_expired", func(t *testing.T) {
		f := newGuardedFixture(t, false)
		ctx := context.Background()
		seeded := seedTask(t, f.db, func(task *model.ProviderTask) {
			task.MaxAttempts = 1
			task.ResourceUID = "res-exhaust-1"
		})
		claim := claimOrFatal(t, f)
		expireLeaseForcesReap(t, f, seeded.ID)

		requeued, err := f.eng.ReapOnce(ctx)
		if err != nil {
			t.Fatalf("ReapOnce: %v", err)
		}
		if requeued != 0 {
			t.Fatalf("requeued = %d, want 0 — attempts exhausted", requeued)
		}
		got := reloadTask(t, f.db, seeded.ID)
		if got.Status != TaskStatusFailed {
			t.Fatalf("status = %q, want failed (exhausted terminal, §13.2)", got.Status)
		}
		if got.ErrorCode != ErrorCodeLeaseExpired {
			t.Errorf("error_code = %q, want %q (§13.2 verbatim)", got.ErrorCode, ErrorCodeLeaseExpired)
		}
		if got.FinishedAt == nil || got.LeaseExpiresAt != nil {
			t.Errorf("terminal shape = (finished=%v, lease=%v), want (set, nil)", got.FinishedAt, got.LeaseExpiresAt)
		}
		if !IsTerminalTaskStatus(got.Status) {
			t.Error("exhausted crash status is not in the 4-terminal set")
		}
		types := eventTypes(eventsOf(t, f.db, seeded.ID))
		if len(types) != 2 || types[0] != TaskEventClaimed || types[1] != TaskEventFailed {
			t.Fatalf("events = %v, want [claimed failed]", types)
		}
		// The one crashed attempt is closed with the lease_expired outcome.
		var attempt model.TaskAttempt
		if err := f.db.First(&attempt, claim.Attempt.ID).Error; err != nil {
			t.Fatalf("load attempt: %v", err)
		}
		if attempt.FinishedAt == nil || attempt.ErrorCode != ErrorCodeLeaseExpired {
			t.Errorf("attempt = (finished=%v, code=%q), want (set, lease_expired)", attempt.FinishedAt, attempt.ErrorCode)
		}
	})

	t.Run("repeated crashes exhaust attempts into the failed terminal", func(t *testing.T) {
		f := newGuardedFixture(t, false)
		ctx := context.Background()
		// testOperation snapshots MaxAttempts=2 — two crash cycles exhaust it.
		task, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-exhaust-loop",
		})
		if err != nil {
			t.Fatalf("Submit: %v", err)
		}

		// Crash cycle 1: claim, vanish, reap → requeued.
		claimOrFatal(t, f)
		expireLeaseForcesReap(t, f, task.ID)
		requeued, err := f.eng.ReapOnce(ctx)
		if err != nil || requeued != 1 {
			t.Fatalf("reap cycle 1 = (%d, %v), want (1, nil)", requeued, err)
		}

		// Crash cycle 2: the recovered claim also vanishes — now exhausted.
		second := claimOrFatal(t, f)
		if second.Attempt.AttemptNo != 2 {
			t.Fatalf("recovered attempt_no = %d, want 2", second.Attempt.AttemptNo)
		}
		expireLeaseForcesReap(t, f, task.ID)
		requeued, err = f.eng.ReapOnce(ctx)
		if err != nil {
			t.Fatalf("ReapOnce cycle 2: %v", err)
		}
		if requeued != 0 {
			t.Fatalf("requeued cycle 2 = %d, want 0 — MaxAttempts reached", requeued)
		}

		final := reloadTask(t, f.db, task.ID)
		if final.Status != TaskStatusFailed || final.ErrorCode != ErrorCodeLeaseExpired {
			t.Fatalf("final = (%q, %q), want (failed, lease_expired)", final.Status, final.ErrorCode)
		}
		// Every attempt row is closed — repeated crashes leave no open work.
		var open int64
		f.db.Model(&model.TaskAttempt{}).Where("task_id = ? AND finished_at IS NULL", task.ID).Count(&open)
		if open != 0 {
			t.Errorf("open attempts after exhaustion = %d, want 0", open)
		}
		var attempts int64
		f.db.Model(&model.TaskAttempt{}).Where("task_id = ?", task.ID).Count(&attempts)
		if attempts != 2 {
			t.Errorf("attempt rows = %d, want 2 (one per crash cycle)", attempts)
		}
	})
}
