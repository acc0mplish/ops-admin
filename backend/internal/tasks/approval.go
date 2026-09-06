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

// ApprovalStatus vocabulary (§13.3) — the v1 OpsJob convention (model/ops.go:538
// 관례) reused verbatim: the column defaults to 'not_required'; 'approved' and
// 'rejected' are the two decision values Approve/Reject write. There is
// deliberately NO 'pending' value — the awaiting posture is expressed by
// provider_task.Status='awaiting_approval', and 'rejected' exists only here
// and as a task_event type, never as a Status (r2 A: the terminal set stays
// the four {succeeded, failed, timed_out, cancelled}).
const (
	ApprovalStatusNotRequired = "not_required"
	ApprovalStatusApproved    = "approved"
	ApprovalStatusRejected    = "rejected"
)

// ErrNotAwaitingApproval — Approve/Reject refused because the task is not in
// awaiting_approval (already decided, running, terminal, …). The §13.5 edge
// set allows both verbs only from awaiting_approval.
var ErrNotAwaitingApproval = errNotAwaitingApproval{}

type errNotAwaitingApproval struct{}

func (errNotAwaitingApproval) Error() string {
	return "tasks: approval verb refused — the task is not in awaiting_approval (§13.3)"
}

// Approve moves an awaiting_approval task to queued (§13.3) — status
// transition (version-CAS'd), approval columns and the 'approved' event
// commit in one transaction (D5).
func (e *Engine) Approve(ctx context.Context, taskUID, approver string) error {
	return e.decideApproval(ctx, taskUID, approver, approvalDecision{
		nextStatus:     TaskStatusQueued,
		approvalStatus: ApprovalStatusApproved,
		eventType:      TaskEventApproved,
		terminal:       false,
	})
}

// Reject terminally cancels an awaiting_approval task (§13.3 "rejection is
// terminal"): Status lands on cancelled — one of the four terminal statuses,
// never a 'rejected' Status (r2 A) — ApprovalStatus='rejected', and the
// 'rejected' event records the approver. The terminal transition also NULLs
// active_flag, freeing the resource for a new mutation (§13.4).
func (e *Engine) Reject(ctx context.Context, taskUID, approver string) error {
	return e.decideApproval(ctx, taskUID, approver, approvalDecision{
		nextStatus:     TaskStatusCancelled,
		approvalStatus: ApprovalStatusRejected,
		eventType:      TaskEventRejected,
		terminal:       true,
	})
}

// approvalDecision parameterizes the two §13.3 verbs over one transition body.
type approvalDecision struct {
	nextStatus     string
	approvalStatus string
	eventType      string
	terminal       bool
}

// decideApproval loads the task by UID, CAS-transitions it from
// awaiting_approval, and appends the decision event — all in one transaction.
// Failure disambiguation: unknown UID → not-found error (at load); a status
// other than awaiting_approval → ErrNotAwaitingApproval (at load); a version
// that moved between the load and the CAS → ErrStaleWrite (RowsAffected 0).
func (e *Engine) decideApproval(ctx context.Context, taskUID, approver string, d approvalDecision) error {
	now := time.Now()
	err := e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.ProviderTask
		if err := tx.Where("uid = ?", taskUID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("tasks: %s on unknown task uid %q: %w", d.eventType, taskUID, err)
			}
			return fmt.Errorf("tasks: load task %q for %s: %w", taskUID, d.eventType, err)
		}
		if task.Status != TaskStatusAwaitingApproval {
			return fmt.Errorf("%w: uid %s is %q, verb %s", ErrNotAwaitingApproval, taskUID, task.Status, d.eventType)
		}

		updates := map[string]any{
			"status":          d.nextStatus,
			"approval_status": d.approvalStatus,
			"approver":        approver,
			"approval_at":     now,
			"version":         task.Version + 1,
		}
		if d.terminal {
			updates["finished_at"] = now // §13.3 rejection is terminal
		}
		res := tx.Model(&model.ProviderTask{}).
			Where("id = ? AND status = ? AND version = ?", task.ID, TaskStatusAwaitingApproval, task.Version).
			Updates(updates)
		if res.Error != nil {
			return fmt.Errorf("tasks: %s CAS on task %d: %w", d.eventType, task.ID, res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrStaleWrite
		}
		return appendEvent(tx, task.ID, 0, d.eventType, approver,
			contract.JSONMap{"approver": approver, "approval_status": d.approvalStatus})
	})
	if err != nil {
		return fmt.Errorf("tasks: %s (task %s): %w", d.eventType, taskUID, err)
	}
	return nil
}
