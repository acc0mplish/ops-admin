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
		if attemptsRemain {
			requeued++
		}
	}
	return requeued, nil
}

const actorReaper = "reaper"
