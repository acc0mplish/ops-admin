package tasks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
)

// ErrNotCancellable — RequestCancel refused because the task sits outside the
// cancellable live states {queued, awaiting_approval, running, cancelling}:
// one of the four terminals, or `planned` (a transition state Submit's
// transaction never persists — encountering one is an anomaly, and the verb
// refuses rather than inventing a §13.5-illegal edge). The HTTP layer
// pre-checks terminals to 409; this sentinel is the engine's own
// state-machine honesty.
var ErrNotCancellable = errNotCancellable{}

type errNotCancellable struct{}

func (errNotCancellable) Error() string {
	return "tasks: cancel verb refused — the task is not in a cancellable state (§13.4)"
}

// RequestCancel is the §13.4 cancel verb (plan N6 — the Phase 1 N-4
// cancel_requested emission path completed). One verb, three live-state legs,
// every write version-CAS'd (D4) with flag/transition + event in a single
// transaction (D5 spirit):
//
//	queued             → 즉시 종단 cancelled — cancelQueuedNow: the same
//	                     collapsed Queued→Running→Cancelling→Cancelled commit
//	                     shape the loop-side boundary uses (claim edges stay
//	                     inside the §13.5 diagram), attempt opened+closed
//	                     unexecuted. next_attempt_at is NOT a guard here:
//	                     RequestCancel IS the operator-opened boundary — a
//	                     backoff wait must not delay it ("즉시").
//	awaiting_approval  → 즉시 종단 cancelled — cancelAwaitingNow: the direct
//	                     awaiting→cancelled edge (§13.5, the Reject shape);
//	                     no attempt (never claimed).
//	running/cancelling → cancel_requested=true + the cancel_requested event
//	                     (N-4 identifier). Consumption is the NEXT claim
//	                     boundary (§13.4 verbatim): the existing ClaimNext
//	                     flag check terminal-cancels the task once the
//	                     lease/retry path requeues it. There is deliberately
//	                     NO poll-cycle consumption path — a second terminal
//	                     producer would split the D5 verification space, and
//	                     the k8s restart adapter offers no partial-rollback
//	                     cancel (plan r2 F-5).
//
// The verb is naturally converging (§3.5 L1): a repeat against a task this
// verb already terminal-cancelled refuses with ErrNotCancellable; a repeat
// against a still-running task re-asserts the flag.
func (e *Engine) RequestCancel(ctx context.Context, taskUID, actor string) error {
	var task model.ProviderTask
	err := e.db.WithContext(ctx).Where("uid = ?", taskUID).First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("tasks: cancel on unknown task uid %q: %w", taskUID, err)
		}
		return fmt.Errorf("tasks: load task %q for cancel: %w", taskUID, err)
	}

	switch task.Status {
	case TaskStatusQueued:
		return e.cancelQueuedNow(ctx, &task, actor)
	case TaskStatusAwaitingApproval:
		return e.cancelAwaitingNow(ctx, &task, actor)
	case TaskStatusRunning, TaskStatusCancelling:
		return e.flagCancelRequested(ctx, &task, actor)
	default:
		return fmt.Errorf("%w: uid %s is %q", ErrNotCancellable, taskUID, task.Status)
	}
}

// cancelQueuedNow terminally cancels a queued task in one commit — the
// collapsed claim+cancel of the loop-side cancelAtClaimBoundary (engine.go),
// with the operator's request recorded first: flag + terminal transition,
// one attempt opened and closed unexecuted, events
// [cancel_requested, claimed, cancelled]. The §13.5 edges stay legal
// (queued→running→cancelling→collapsed-cancelled). A lost CAS (concurrent
// claimer or terminal) is ErrStaleWrite — the caller re-issues the verb and
// it converges.
func (e *Engine) cancelQueuedNow(ctx context.Context, task *model.ProviderTask, actor string) error {
	now := time.Now()
	err := e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.ProviderTask{}).
			Where("id = ? AND status = ? AND version = ?",
				task.ID, TaskStatusQueued, task.Version).
			Updates(map[string]any{
				"status":           TaskStatusCancelled, // running→cancelling→cancelled collapsed (§13.5 edges)
				"cancel_requested": true,                // §13.4 "persisted either way" — the row self-describes the request
				"finished_at":      now,
				"lease_expires_at": nil,
				"attempt_count":    gorm.Expr("attempt_count + 1"),
				"version":          task.Version + 1,
			})
		if res.Error != nil {
			return fmt.Errorf("tasks: immediate cancel CAS on task %d: %w", task.ID, res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrStaleWrite
		}
		var fresh model.ProviderTask
		if err := tx.First(&fresh, task.ID).Error; err != nil {
			return fmt.Errorf("tasks: reload cancelled task %d: %w", task.ID, err)
		}
		attempt := model.TaskAttempt{
			TaskID:     fresh.ID,
			AttemptNo:  fresh.AttemptCount,
			WorkerID:   e.cfg.WorkerID,
			StartedAt:  now,
			FinishedAt: &now,
		}
		if err := tx.Create(&attempt).Error; err != nil {
			return fmt.Errorf("tasks: create immediate-cancel attempt for task %d: %w", fresh.ID, err)
		}
		if err := appendEvent(tx, fresh.ID, 0, TaskEventCancelRequested, actor, nil); err != nil {
			return err
		}
		if err := appendEvent(tx, fresh.ID, fresh.AttemptCount, TaskEventClaimed, e.cfg.WorkerID, nil); err != nil {
			return err
		}
		return appendEvent(tx, fresh.ID, fresh.AttemptCount, TaskEventCancelled, e.cfg.WorkerID,
			contract.JSONMap{"reason": "cancel_requested via RequestCancel (§13.4 immediate leg)"})
	})
	if err != nil {
		return fmt.Errorf("tasks: cancel queued (task %s): %w", task.UID, err)
	}
	return nil
}

// cancelAwaitingNow terminally cancels an awaiting_approval task over the
// direct §13.5 edge (the Reject shape — approval.go's terminal leg): flag +
// terminal transition + events [cancel_requested, cancelled]; no attempt —
// the task was never claimed.
func (e *Engine) cancelAwaitingNow(ctx context.Context, task *model.ProviderTask, actor string) error {
	now := time.Now()
	err := e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.ProviderTask{}).
			Where("id = ? AND status = ? AND version = ?",
				task.ID, TaskStatusAwaitingApproval, task.Version).
			Updates(map[string]any{
				"status":           TaskStatusCancelled, // §13.5 awaiting_approval→cancelled
				"cancel_requested": true,
				"finished_at":      now,
				"lease_expires_at": nil,
				"version":          task.Version + 1,
			})
		if res.Error != nil {
			return fmt.Errorf("tasks: awaiting cancel CAS on task %d: %w", task.ID, res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrStaleWrite
		}
		if err := appendEvent(tx, task.ID, 0, TaskEventCancelRequested, actor, nil); err != nil {
			return err
		}
		return appendEvent(tx, task.ID, 0, TaskEventCancelled, e.cfg.WorkerID,
			contract.JSONMap{"reason": "cancel_requested via RequestCancel (§13.4 awaiting leg)"})
	})
	if err != nil {
		return fmt.Errorf("tasks: cancel awaiting (task %s): %w", task.UID, err)
	}
	return nil
}

// flagCancelRequested raises the §13.4 flag on a live running (or cancelling)
// task and appends the cancel_requested event (N-4) in the same transaction.
// The task's status does NOT change: consumption is the next claim boundary —
// ClaimNext's existing flag check terminal-cancels the task after the
// lease/retry machinery requeues it. attempt_no is the in-flight attempt, so
// the event timeline reads against the attempt that was live when the
// operator asked.
func (e *Engine) flagCancelRequested(ctx context.Context, task *model.ProviderTask, actor string) error {
	err := e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.ProviderTask{}).
			Where("id = ? AND status IN ? AND version = ?",
				task.ID, []string{TaskStatusRunning, TaskStatusCancelling}, task.Version).
			Updates(map[string]any{
				"cancel_requested": true,
				"version":          task.Version + 1,
			})
		if res.Error != nil {
			return fmt.Errorf("tasks: cancel flag CAS on task %d: %w", task.ID, res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrStaleWrite
		}
		return appendEvent(tx, task.ID, task.AttemptCount, TaskEventCancelRequested, actor, nil)
	})
	if err != nil {
		return fmt.Errorf("tasks: flag cancel_requested (task %s): %w", task.UID, err)
	}
	return nil
}
