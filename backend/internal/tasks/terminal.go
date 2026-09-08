package tasks

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
)

// cancelAtClaimBoundary claims the flagged task and terminal-cancels it in
// one commit — Queued→Running→Cancelling→Collapsed-Cancelled keeps every edge
// inside the §13.5 diagram. The attempt created by the claim is closed
// unexecuted.
func (e *Engine) cancelAtClaimBoundary(ctx context.Context, task *model.ProviderTask, now time.Time) error {
	return e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.ProviderTask{}).
			Where("id = ? AND status = ? AND next_attempt_at <= ? AND version = ?",
				task.ID, TaskStatusQueued, now, task.Version).
			Updates(map[string]any{
				"status":           TaskStatusCancelled, // running→cancelling→cancelled collapsed (§13.5 edges)
				"finished_at":      now,
				"lease_expires_at": nil,
				"attempt_count":    gorm.Expr("attempt_count + 1"),
				"version":          task.Version + 1,
			})
		if res.Error != nil {
			return fmt.Errorf("tasks: cancel-at-boundary CAS on task %d: %w", task.ID, res.Error)
		}
		if res.RowsAffected == 0 {
			return nil // lost to a concurrent claimer — it owns the task now
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
			return fmt.Errorf("tasks: create boundary-cancel attempt for task %d: %w", fresh.ID, err)
		}
		if err := appendEvent(tx, fresh.ID, fresh.AttemptCount, TaskEventClaimed, e.cfg.WorkerID, nil); err != nil {
			return err
		}
		return appendEvent(tx, fresh.ID, fresh.AttemptCount, TaskEventCancelled, e.cfg.WorkerID,
			contract.JSONMap{"reason": "cancel_requested at claim boundary (§13.4)"})
	})
}

// Complete terminally succeeds the claimed task — transition, attempt closure
// and the 'succeeded' event commit in one transaction (D5).
func (e *Engine) Complete(ctx context.Context, claim *ClaimedTask, detail contract.JSONMap) error {
	return e.finishAttempt(ctx, claim, finishParams{
		status:    TaskStatusSucceeded,
		eventType: TaskEventSucceeded,
		eventData: detail,
		errorCode: "",
		errorMsg:  "",
	})
}

// nonRetryableCodes — configuration/identity faults where a retry is
// meaningless by contract: the resource is unknown (T-7), the definition or
// adapter vanished, the capability is not declared, or the bound credential
// will not resolve. These terminal-fail immediately regardless of remaining
// attempts; transient execution errors (executor_error/operation_failed)
// keep the §13.5 retry path.
var nonRetryableCodes = map[string]bool{
	ErrorCodeUnknownResource:     true, // T-7: "조회 누락 = unknown resource 하드 에러, 재시도 없음"
	ErrorCodeUnknownOperation:    true, // "정의 미등록 = 하드 에러, 재시도 없음"
	ErrorCodeNoExecutor:          true,
	ErrorCodeCapabilityNotServed: true,
	ErrorCodeCredentialError:     true,
}

// Fail closes the claimed attempt. timed_out is an engine-derived terminal
// (§3.9 — the adapter never reports it). Non-retryable codes terminal-fail
// immediately. Otherwise, with attempts remaining, the task returns to
// queued — NOT failed — with the retry decision, backoff, attempt closure
// and 'attempt_failed' event in a single transaction (T-9); exhaustion lands
// on the failed terminal.
func (e *Engine) Fail(ctx context.Context, claim *ClaimedTask, code, msg string) error {
	if code == ErrorCodeTimedOut {
		return e.finishAttempt(ctx, claim, finishParams{
			status: TaskStatusTimedOut, eventType: TaskEventTimedOut,
			errorCode: code, errorMsg: msg,
		})
	}
	if !nonRetryableCodes[code] && claim.Task.AttemptCount < claim.Task.MaxAttempts {
		return e.requeueForRetry(ctx, claim, code, msg)
	}
	return e.finishAttempt(ctx, claim, finishParams{
		status: TaskStatusFailed, eventType: TaskEventFailed,
		errorCode: code, errorMsg: msg,
	})
}

type finishParams struct {
	status    string
	eventType string
	eventData contract.JSONMap
	errorCode string
	errorMsg  string
}

// finishAttempt commits a terminal outcome — status transition (version-CAS'd,
// running only), attempt closure, and event in one transaction (D5).
func (e *Engine) finishAttempt(ctx context.Context, claim *ClaimedTask, p finishParams) error {
	now := time.Now()
	err := e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.ProviderTask{}).
			Where("id = ? AND status = ? AND version = ?", claim.Task.ID, TaskStatusRunning, claim.Task.Version).
			Updates(map[string]any{
				"status":        p.status,
				"error_code":    p.errorCode,
				"error_message": p.errorMsg,
				"finished_at":   now,
				"version":       claim.Task.Version + 1,
			})
		if res.Error != nil {
			return fmt.Errorf("tasks: terminal %s on task %d: %w", p.status, claim.Task.ID, res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrStaleWrite
		}
		if err := closeAttempt(tx, claim.Attempt.ID, now, p.errorCode, p.errorMsg); err != nil {
			return err
		}
		// Defensive copy — the caller's detail map (e.g. the adapter's
		// OperationStatus.Detail) is never mutated by the engine.
		data := make(contract.JSONMap, len(p.eventData)+1)
		for k, v := range p.eventData {
			data[k] = v
		}
		if p.errorCode != "" {
			data["error_code"] = p.errorCode
		}
		return appendEvent(tx, claim.Task.ID, claim.Attempt.AttemptNo, p.eventType, e.cfg.WorkerID, data)
	})
	if err != nil {
		return fmt.Errorf("tasks: complete attempt (task %d, attempt %d): %w", claim.Task.ID, claim.Attempt.AttemptNo, err)
	}
	// J6 — 종단 커밋 이후 훅 발화(종단 불변). 훅 실패는 여기서 처분되고
	// 호출자에게 새어나가지 않는다(Complete/Fail의 반환은 종단 커밋의 진위만
	// 말한다).
	return e.fireTaskTerminal(ctx, claim, p)
}

// fireTaskTerminal invokes OnTaskTerminal after the terminal commit (J6).
// The hook receives the COMMITTED terminal row — a fresh reload, not the
// caller's pre-commit snapshot — plus the terminal status and the event-side
// detail (Complete: the adapter's OperationStatus.Detail; Fail: nil — the
// error code rides on the task row). A hook failure never rolls the terminal
// back: it is logged in full and recorded as an observation_refresh_failed
// task_event (status only — event data carries no free-text error, per the
// engine's closed-vocabulary event convention and 보존 제약 #7's task_event
// scan surface). Only infrastructure failures (reload/event write) propagate.
func (e *Engine) fireTaskTerminal(ctx context.Context, claim *ClaimedTask, p finishParams) error {
	if e.OnTaskTerminal == nil {
		return nil
	}
	var fresh model.ProviderTask
	if err := e.db.WithContext(ctx).First(&fresh, claim.Task.ID).Error; err != nil {
		return fmt.Errorf("tasks: reload terminal task %d for OnTaskTerminal: %w", claim.Task.ID, err)
	}
	if err := e.OnTaskTerminal(ctx, fresh, p.status, p.eventData); err != nil {
		log.Printf("tasks: OnTaskTerminal hook failed on task %s (%s) — the terminal stands: %v", fresh.UID, p.status, err)
		if err := appendEvent(e.db.WithContext(ctx), claim.Task.ID, claim.Attempt.AttemptNo,
			TaskEventObservationRefreshFailed, e.cfg.WorkerID, contract.JSONMap{"status": p.status}); err != nil {
			return fmt.Errorf("tasks: append %s event for task %d: %w", TaskEventObservationRefreshFailed, claim.Task.ID, err)
		}
	}
	return nil
}

// requeueForRetry — T-9 single transaction: Failed→Queued (§13.5), backoff
// into next_attempt_at (def.RetryPolicy.BackoffSeconds; a vanished definition
// degrades to an immediate retry), attempt closure, 'attempt_failed' event.
func (e *Engine) requeueForRetry(ctx context.Context, claim *ClaimedTask, code, msg string) error {
	now := time.Now()
	backoff := time.Duration(0)
	if def, ok := e.reg.Operation(claim.Task.OperationName); ok {
		backoff = time.Duration(def.RetryPolicy.BackoffSeconds) * time.Second
	}
	nextAttempt := now.Add(backoff)

	err := e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.ProviderTask{}).
			Where("id = ? AND status = ? AND version = ?", claim.Task.ID, TaskStatusRunning, claim.Task.Version).
			Updates(map[string]any{
				"status":           TaskStatusQueued,
				"next_attempt_at":  nextAttempt,
				"lease_expires_at": nil,
				"error_code":       code,
				"error_message":    msg,
				"version":          claim.Task.Version + 1,
			})
		if res.Error != nil {
			return fmt.Errorf("tasks: retry requeue on task %d: %w", claim.Task.ID, res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrStaleWrite
		}
		if err := closeAttempt(tx, claim.Attempt.ID, now, code, msg); err != nil {
			return err
		}
		return appendEvent(tx, claim.Task.ID, claim.Attempt.AttemptNo, TaskEventAttemptFailed, e.cfg.WorkerID,
			contract.JSONMap{"error_code": code, "next_attempt_at": nextAttempt})
	})
	if err != nil {
		return fmt.Errorf("tasks: requeue attempt (task %d, attempt %d): %w", claim.Task.ID, claim.Attempt.AttemptNo, err)
	}
	return nil
}

func closeAttempt(tx *gorm.DB, attemptID uint, now time.Time, code, msg string) error {
	res := tx.Model(&model.TaskAttempt{}).Where("id = ?", attemptID).
		Updates(map[string]any{"finished_at": now, "error_code": code, "error_message": msg})
	if res.Error != nil {
		return fmt.Errorf("tasks: close attempt %d: %w", attemptID, res.Error)
	}
	return nil
}

func appendEvent(tx *gorm.DB, taskID uint, attemptNo int, eventType, actor string, data contract.JSONMap) error {
	event := model.TaskEvent{
		TaskID:    taskID,
		AttemptNo: attemptNo,
		Type:      eventType,
		Actor:     actor,
		DataJSON:  data,
		At:        time.Now(),
	}
	if err := tx.Create(&event).Error; err != nil {
		return fmt.Errorf("tasks: append %s event for task %d: %w", eventType, taskID, err)
	}
	return nil
}
