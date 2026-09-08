package tasks

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/model"
)

// ReapOnce is one deterministic reaper cycle (§13.2):
//
//	find status='running' AND lease_expires_at < NOW() - grace
//	-> re-queue (status='queued', next_attempt_at=NOW) when attempts remain
//	-> mark 'failed' with ErrorCode='lease_expired' when attempts exhausted
//	-> append task_event('reaper_requeued')
//
// Comparison times are bound Go-side (T-10). This is the crash-recovery
// path: a task stuck in running after its worker died is recovered within
// lease + grace + poll interval — the measurable fix for spec §2.8's
// orphaned-running defect. The crashed worker's open attempt is closed with
// 'lease_expired' in the same transaction (one row per execution attempt —
// the crash IS an attempt outcome). Returns the number of re-queued tasks.
func (e *Engine) ReapOnce(ctx context.Context) (int, error) {
	now := time.Now()
	cutoff := now.Add(-e.cfg.ReaperGrace)

	var stale []model.ProviderTask
	err := e.db.WithContext(ctx).
		Where("status = ? AND lease_expires_at IS NOT NULL AND lease_expires_at < ?", TaskStatusRunning, cutoff).
		Order("id").
		Find(&stale).Error
	if err != nil {
		return 0, fmt.Errorf("tasks: reaper select stale leases: %w", err)
	}

	requeued := 0
	for i := range stale {
		task := stale[i]
		attemptsRemain := task.AttemptCount < task.MaxAttempts
		won := false           // CAS 승윈 리프 처리만 계기 대상(§18.2 J — 경합 패배 제외)
		claimAt := time.Time{} // 소진 종단 duration의 원점 — 마지막 열린 시도의 클레임 시각

		var next map[string]any
		var eventType string
		eventData := map[string]any{"attempt_no": task.AttemptCount}
		if attemptsRemain {
			next = map[string]any{
				"status":           TaskStatusQueued,
				"next_attempt_at":  now, // §13.2 "next_attempt_at=NOW"
				"lease_expires_at": nil,
				"version":          task.Version + 1,
			}
			eventType = TaskEventReaperRequeued
			eventData["exhausted"] = false
		} else {
			next = map[string]any{
				"status":           TaskStatusFailed,
				"error_code":       ErrorCodeLeaseExpired,
				"error_message":    "lease expired before a terminal commit — attempts exhausted",
				"finished_at":      now,
				"lease_expires_at": nil,
				"version":          task.Version + 1,
			}
			eventType = TaskEventFailed
			eventData["exhausted"] = true
			eventData["error_code"] = ErrorCodeLeaseExpired
		}

		err := e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&model.ProviderTask{}).
				Where("id = ? AND status = ? AND version = ? AND lease_expires_at < ?",
					task.ID, TaskStatusRunning, task.Version, cutoff).
				Updates(next)
			if res.Error != nil {
				return fmt.Errorf("tasks: reap task %d: %w", task.ID, res.Error)
			}
			if res.RowsAffected == 0 {
				return nil // raced (terminal commit or another reaper) — not ours
			}
			won = true
			// 소진 종단의 duration 원점(가정 A4 — claim→terminal). 닫히는 열린 시도의
			// 시작이 클레임 시각이다; 종결 커밋과 같은 트랜잭션에서 읽는다. 질의 실패는
			// duration 미가산일 뿐 리프를 바꾸지 않는다.
			if !attemptsRemain {
				var open model.TaskAttempt
				if err := tx.Where("task_id = ? AND finished_at IS NULL", task.ID).
					Order("id DESC").First(&open).Error; err == nil {
					claimAt = open.StartedAt
				}
			}
			// Close the crashed worker's open attempt(s) in the same commit.
			if err := tx.Model(&model.TaskAttempt{}).
				Where("task_id = ? AND finished_at IS NULL", task.ID).
				Updates(map[string]any{"finished_at": now, "error_code": ErrorCodeLeaseExpired}).Error; err != nil {
				return fmt.Errorf("tasks: close crashed attempts of task %d: %w", task.ID, err)
			}
			return appendEvent(tx, task.ID, task.AttemptCount, eventType, actorReaper, eventData)
		})
		if err != nil {
			return requeued, err
		}
		if won {
			e.observeReapTerminal(&task, !attemptsRemain, claimAt)
		}
		if attemptsRemain {
			requeued++
		}
	}
	return requeued, nil
}

// observeReapTerminal feeds the §18.2 J family — plus F/G on the exhausted
// branch — for one CAS-won reap. The requeue branch is not a terminal: it
// adds only worker_lease_expired_total (duration·failure·retry 가산 밖 —
// metrics 패키지 가정 A4). The exhausted branch commits the failed terminal,
// so it feeds the same F/G families finishAttempt does: duration
// {operation, failed} from the crashed attempt's claim time and failure
// {operation, lease_expired}. Metrics nil = no-op (T-4).
func (e *Engine) observeReapTerminal(task *model.ProviderTask, exhausted bool, claimAt time.Time) {
	if e.Metrics == nil {
		return
	}
	e.Metrics.IncLeaseExpired()
	if exhausted {
		if !claimAt.IsZero() {
			e.Metrics.ObserveTaskDuration(task.OperationName, TaskStatusFailed, time.Since(claimAt))
		}
		e.Metrics.IncTaskFailure(task.OperationName, ErrorCodeLeaseExpired)
	}
}

const actorReaper = "reaper"
