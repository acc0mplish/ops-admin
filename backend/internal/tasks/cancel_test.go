package tasks

import (
	"context"
	"errors"
	"testing"
	"time"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
)

// N6 — Engine.RequestCancel (§3.6, §13.4): the public cancel verb.
//
//	queued            → 즉시 종단 cancelled (cancelAtClaimBoundary와 동일 형태의
//	                     collapsed claim+cancel 단일 커밋 — queued→running→
//	                     cancelling→cancelled 변이 전부 §13.5 다이어그램 내부)
//	awaiting_approval → 즉시 종단 cancelled (awaiting→cancelled 직접 에지 —
//	                     Reject와 동일 형태, 시도 없음)
//	running/cancelling → cancel_requested=true 플래그 CAS + cancel_requested
//	                     이벤트(N-4 식별자) — 소비는 "next claim boundary"(§13.4)
//	나머지(planned·종단 4종) → ErrNotCancellable 거부
//
// 모든 쓰기는 version CAS(D4)이고 플래그+이벤트는 단일 트랜잭션(D5 정신).

// TestRequestCancelQueuedLeg — queued 태스크의 즉시 종단: collapsed 경계
// 커밋이 [cancel_requested, claimed, cancelled] 이벤트와 닫힌 attempt를 남기고,
// 실행은 단 한 번도 일어나지 않으며, 종단이 자원을 해제한다(§13.4).
func TestRequestCancelQueuedLeg(t *testing.T) {
	f := newGuardedFakeFixture(t, false)
	ctx := context.Background()

	task, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := f.eng.RequestCancel(ctx, task.UID, "carol"); err != nil {
		t.Fatalf("RequestCancel on queued: %v", err)
	}

	final := reloadTask(t, f.db, task.ID)
	if final.Status != TaskStatusCancelled || !IsTerminalTaskStatus(final.Status) {
		t.Fatalf("status = %q, want cancelled (terminal)", final.Status)
	}
	if !final.CancelRequested {
		t.Error("cancel_requested flag not persisted on the terminal row (§13.4 persisted either way)")
	}
	if final.FinishedAt == nil {
		t.Error("finished_at not set on the immediate cancel")
	}
	if got := f.fake.ExecCount(); got != 0 {
		t.Fatalf("ExecCount = %d, want 0 — a cancelled queued task never executes", got)
	}
	// Collapsed claim: one attempt opened and closed unexecuted (same shape as
	// the loop-side cancelAtClaimBoundary).
	if final.AttemptCount != 1 {
		t.Fatalf("attempt_count = %d, want 1 — the collapsed claim consumes one attempt slot", final.AttemptCount)
	}
	var attempt model.TaskAttempt
	if err := f.db.Where("task_id = ?", final.ID).First(&attempt).Error; err != nil {
		t.Fatalf("boundary attempt row missing: %v", err)
	}
	if attempt.FinishedAt == nil {
		t.Error("boundary attempt not closed in the cancel commit")
	}
	// Event order: the operator's request precedes the boundary records.
	types := eventTypes(eventsOf(t, f.db, task.ID))
	want := []string{TaskEventCreated, TaskEventCancelRequested, TaskEventClaimed, TaskEventCancelled}
	if len(types) != len(want) {
		t.Fatalf("queued-cancel events = %v, want EXACTLY %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("queued-cancel events = %v, want EXACTLY %v", types, want)
		}
	}
	events := eventsOf(t, f.db, task.ID)
	if events[1].Actor != "carol" {
		t.Errorf("cancel_requested actor = %q, want carol", events[1].Actor)
	}
	// The terminal frees the resource for a fresh mutation (§13.4).
	if _, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
	}); err != nil {
		t.Fatalf("resubmit after cancel: %v — the terminal must free the resource", err)
	}
}

// TestRequestCancelQueuedBackoffCancelsImmediately — 재시도 백오프 중
// (next_attempt_at 미래) queued 태스크도 "즉시" 종단된다. RequestCancel 자체가
// 운영자가 연 경계다 — 다음 폴 틱까지 기다리지 않는다.
func TestRequestCancelQueuedBackoffCancelsImmediately(t *testing.T) {
	f := newGuardedFakeFixture(t, false)
	ctx := context.Background()

	task, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	future := time.Now().Add(10 * time.Minute)
	if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", task.ID).
		Update("next_attempt_at", future).Error; err != nil {
		t.Fatalf("push next_attempt_at into backoff: %v", err)
	}
	if err := f.eng.RequestCancel(ctx, task.UID, "carol"); err != nil {
		t.Fatalf("RequestCancel on backoff-queued: %v", err)
	}
	final := reloadTask(t, f.db, task.ID)
	if final.Status != TaskStatusCancelled || final.FinishedAt == nil {
		t.Fatalf("status = (%q, finished=%v), want immediate (cancelled, set) despite future next_attempt_at", final.Status, final.FinishedAt)
	}
}

// TestRequestCancelAwaitingLeg — awaiting_approval 태스크의 즉시 종단:
// awaiting→cancelled 직접 에지(§13.5, Reject와 동일 형태). 시도·클레임 없음.
func TestRequestCancelAwaitingLeg(t *testing.T) {
	f := newGuardedFakeFixture(t, true)
	ctx := context.Background()

	task, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if task.Status != TaskStatusAwaitingApproval {
		t.Fatalf("fixture posture broken: status = %q, want awaiting_approval", task.Status)
	}
	if err := f.eng.RequestCancel(ctx, task.UID, "carol"); err != nil {
		t.Fatalf("RequestCancel on awaiting_approval: %v", err)
	}

	final := reloadTask(t, f.db, task.ID)
	if final.Status != TaskStatusCancelled || !IsTerminalTaskStatus(final.Status) {
		t.Fatalf("status = %q, want cancelled (terminal)", final.Status)
	}
	if !final.CancelRequested {
		t.Error("cancel_requested flag not persisted (§13.4)")
	}
	if final.AttemptCount != 0 {
		t.Fatalf("attempt_count = %d, want 0 — an unclaimed task never gains an attempt", final.AttemptCount)
	}
	var attempts int64
	if err := f.db.Model(&model.TaskAttempt{}).Where("task_id = ?", final.ID).Count(&attempts).Error; err != nil {
		t.Fatalf("count attempts: %v", err)
	}
	if attempts != 0 {
		t.Fatalf("attempt rows = %d, want 0", attempts)
	}
	types := eventTypes(eventsOf(t, f.db, task.ID))
	want := []string{TaskEventCreated, TaskEventCancelRequested, TaskEventCancelled}
	if len(types) != len(want) {
		t.Fatalf("awaiting-cancel events = %v, want EXACTLY %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("awaiting-cancel events = %v, want EXACTLY %v", types, want)
		}
	}
	// The terminal frees the resource.
	if _, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
	}); err != nil {
		t.Fatalf("resubmit after awaiting-cancel: %v", err)
	}
}

// TestRequestCancelRunningLeg — running 태스크: 플래그+이벤트만. 상태는
// running 유지(§13.4 "acted on at the next claim boundary" — 폴 사이클이
// 소비하는 별도 경로는 만들지 않는다, r2 F-5). 이후 리퍼 재큐 → 다음 클레임
// 경계에서 종단 소비되고 재실행은 없다.
func TestRequestCancelRunningLeg(t *testing.T) {
	f := newGuardedFakeFixture(t, false)
	ctx := context.Background()

	task, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
		Payload:       contract.JSONMap{"async": true, "polls": 5},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if _, err := f.eng.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce (claim + first poll): %v", err)
	}
	running := reloadTask(t, f.db, task.ID)
	if running.Status != TaskStatusRunning {
		t.Fatalf("fixture posture broken: status = %q, want running (async poll 1 of 5)", running.Status)
	}
	if got := f.fake.ExecCount(); got != 1 {
		t.Fatalf("ExecCount after cycle 1 = %d, want 1", got)
	}

	if err := f.eng.RequestCancel(ctx, task.UID, "carol"); err != nil {
		t.Fatalf("RequestCancel on running: %v", err)
	}
	flagged := reloadTask(t, f.db, task.ID)
	if flagged.Status != TaskStatusRunning {
		t.Fatalf("status after flag = %q, want still running — the flag alone does not transition (§13.4)", flagged.Status)
	}
	if !flagged.CancelRequested {
		t.Fatal("cancel_requested flag not set on the running row")
	}
	events := eventsOf(t, f.db, task.ID)
	flagIdx := -1
	for i, ev := range events {
		if ev.Type == TaskEventCancelRequested {
			flagIdx = i
		}
	}
	if flagIdx < 0 {
		t.Fatalf("cancel_requested event not emitted (N-4): %v", eventTypes(events))
	}
	if events[flagIdx].Actor != "carol" {
		t.Errorf("cancel_requested actor = %q, want carol", events[flagIdx].Actor)
	}
	if events[flagIdx].AttemptNo != flagged.AttemptCount {
		t.Errorf("cancel_requested attempt_no = %d, want the in-flight attempt %d", events[flagIdx].AttemptNo, flagged.AttemptCount)
	}

	// Consumption at the NEXT claim boundary: lease expiry → reaper requeue →
	// the poller's ClaimNext sees the flag and terminal-cancels without
	// re-executing.
	past := time.Now().Add(-2 * time.Hour)
	if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", flagged.ID).
		Update("lease_expires_at", past).Error; err != nil {
		t.Fatalf("force lease expiry: %v", err)
	}
	if requeued, err := f.eng.ReapOnce(ctx); err != nil || requeued != 1 {
		t.Fatalf("ReapOnce = (%d, %v), want (1, nil) — attempts remain, flag rides the requeue", requeued, err)
	}
	if _, err := f.eng.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce at the cancel boundary: %v", err)
	}
	final := reloadTask(t, f.db, task.ID)
	if final.Status != TaskStatusCancelled || final.FinishedAt == nil {
		t.Fatalf("final = (%q, finished=%v), want (cancelled, set) at the boundary", final.Status, final.FinishedAt)
	}
	if got := f.fake.ExecCount(); got != 1 {
		t.Fatalf("ExecCount after boundary consumption = %d, want 1 — the flagged requeue never re-executes", got)
	}
}

// TestRequestCancelRefusals — 어휘 밖 상태 거부: 종단 4종·planned·미지 UID.
// (HTTP 계층의 종단 사전 409 게이트가 우선이지만 — 엔진 동사는 상태머신
// 정직성을 스스로 지킨다.)
func TestRequestCancelRefusals(t *testing.T) {
	f := newGuardedFakeFixture(t, false)
	ctx := context.Background()

	if err := f.eng.RequestCancel(ctx, "no-such-uid", "carol"); err == nil {
		t.Fatal("RequestCancel on unknown uid = nil error, want a not-found error")
	}

	succeeded := seedTask(t, f.db, func(task *model.ProviderTask) {
		task.Status = TaskStatusSucceeded
	})
	if err := f.eng.RequestCancel(ctx, succeeded.UID, "carol"); !errors.Is(err, ErrNotCancellable) {
		t.Fatalf("RequestCancel on succeeded = %v, want ErrNotCancellable", err)
	}
	planned := seedTask(t, f.db, func(task *model.ProviderTask) {
		task.Status = TaskStatusPlanned
	})
	if err := f.eng.RequestCancel(ctx, planned.UID, "carol"); !errors.Is(err, ErrNotCancellable) {
		t.Fatalf("RequestCancel on planned = %v, want ErrNotCancellable (planned never persists past Submit — the guard is state-machine honesty)", err)
	}
	// Refusals write nothing.
	for _, task := range []model.ProviderTask{succeeded, planned} {
		after := reloadTask(t, f.db, task.ID)
		if after.CancelRequested || after.Status != task.Status {
			t.Fatalf("refused cancel mutated the row: status %q flag %v", after.Status, after.CancelRequested)
		}
	}
}

// TestRequestCancelIsIdempotentAcrossLegs — §3.5(L1): cancel 재호출은 동일
// 종단으로 수렴한다. 즉시 종단 후 재호출은 거부(ErrNotCancellable)되고,
// 플래그 경로(running) 재호출은 플래그를 재확인한다(멱등 — CAS는 이미 참인
// 플래그에 다시 true를 쓴다).
func TestRequestCancelIsIdempotentAcrossLegs(t *testing.T) {
	f := newGuardedFakeFixture(t, false)
	ctx := context.Background()

	task, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := f.eng.RequestCancel(ctx, task.UID, "carol"); err != nil {
		t.Fatalf("first cancel: %v", err)
	}
	if err := f.eng.RequestCancel(ctx, task.UID, "carol"); !errors.Is(err, ErrNotCancellable) {
		t.Fatalf("second cancel = %v, want ErrNotCancellable — the task already reached the cancelled terminal", err)
	}
}
