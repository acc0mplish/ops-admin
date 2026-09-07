package v2

import (
	"errors"
	"net/http"
	"time"

	"ops-admin/backend/httpx"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/tasks"

	"github.com/gin-gonic/gin"
)

// V2 task reads and the approval chain (plan §3.4, N8). The approval verbs
// gate on ops:job:approve (J4 — router-level middleware); this file owns the
// engine delegation and the error mapping: unknown task 404, a verb refused
// off awaiting_approval 409 (ErrNotAwaitingApproval), the not-yet-landed
// cancel capability 503 (the CancelStarter seam), everything else 500.

// taskView is the §16.2 task representation. Payload is included: the frozen
// execution payload (restartedAt, resourceRevision — J1/J7) carries no
// credential material (보존 제약 #7), and it is exactly what the UI and the
// trace assertions read back.
type taskView struct {
	UID              string           `json:"uid"`
	Operation        string           `json:"operation"`
	OperationVersion string           `json:"operationVersion"`
	ResourceUID      string           `json:"resourceUid"`
	Status           string           `json:"status"`
	ApprovalStatus   string           `json:"approvalStatus"`
	Approver         string           `json:"approver"`
	AttemptCount     int              `json:"attemptCount"`
	MaxAttempts      int              `json:"maxAttempts"`
	CancelRequested  bool             `json:"cancelRequested"`
	RequiresApproval bool             `json:"requiresApproval"`
	ErrorCode        string           `json:"errorCode"`
	ErrorMessage     string           `json:"errorMessage"`
	Payload          contract.JSONMap `json:"payload"`
	CreatedAt        time.Time        `json:"createdAt"`
	StartedAt        *time.Time       `json:"startedAt"`
	FinishedAt       *time.Time       `json:"finishedAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
}

func newTaskView(task model.ProviderTask) taskView {
	return taskView{
		UID:              task.UID,
		Operation:        task.OperationName,
		OperationVersion: task.OperationVersion,
		ResourceUID:      task.ResourceUID,
		Status:           task.Status,
		ApprovalStatus:   task.ApprovalStatus,
		Approver:         task.Approver,
		AttemptCount:     task.AttemptCount,
		MaxAttempts:      task.MaxAttempts,
		CancelRequested:  task.CancelRequested,
		RequiresApproval: task.RequiresApproval,
		ErrorCode:        task.ErrorCode,
		ErrorMessage:     task.ErrorMessage,
		Payload:          task.PayloadJSON,
		CreatedAt:        task.CreatedAt,
		StartedAt:        task.StartedAt,
		FinishedAt:       task.FinishedAt,
		UpdatedAt:        task.UpdatedAt,
	}
}

type eventView struct {
	AttemptNo int              `json:"attemptNo"`
	Type      string           `json:"type"`
	Actor     string           `json:"actor"`
	Data      contract.JSONMap `json:"data"`
	At        time.Time        `json:"at"`
}

// loadTask loads one task by UID, mapping unknown UIDs to 404 and stashing
// the §18.1 context the audit middleware needs for the denied/failed paths.
func (a *InfraAPI) loadTask(c *gin.Context) (model.ProviderTask, *auditInfo, bool) {
	var task model.ProviderTask
	if err := a.db.Where("uid = ?", c.Param("uid")).First(&task).Error; err != nil {
		stashAudit(c, &auditInfo{TaskUID: c.Param("uid"), ErrorCode: "TASK_NOT_FOUND"})
		httpx.FailedCode(c, http.StatusNotFound, "TASK_NOT_FOUND", nil)
		return model.ProviderTask{}, nil, false
	}
	info := &auditInfo{
		TaskUID:          task.UID,
		Operation:        task.OperationName,
		OperationVersion: task.OperationVersion,
		ResourceUID:      task.ResourceUID,
	}
	stashAudit(c, info)
	return task, info, true
}

// GetTask — GET /tasks/:uid.
func (a *InfraAPI) GetTask(c *gin.Context) {
	task, _, ok := a.loadTask(c)
	if !ok {
		return
	}
	httpx.Success(c, gin.H{"task": newTaskView(task)})
}

// GetTaskEvents — GET /tasks/:uid/events: the append-only event log in
// write order.
func (a *InfraAPI) GetTaskEvents(c *gin.Context) {
	task, _, ok := a.loadTask(c)
	if !ok {
		return
	}
	var rows []model.TaskEvent
	if err := a.db.Where("task_id = ?", task.ID).Order("at ASC, id ASC").Find(&rows).Error; err != nil {
		httpx.Failed(c, http.StatusInternalServerError, err.Error())
		return
	}
	events := make([]eventView, 0, len(rows))
	for _, row := range rows {
		events = append(events, eventView{AttemptNo: row.AttemptNo, Type: row.Type, Actor: row.Actor, Data: row.DataJSON, At: row.At})
	}
	httpx.Success(c, gin.H{"events": events})
}

// ApproveTask — POST /tasks/:uid/approve (§13.3): awaiting_approval → queued.
func (a *InfraAPI) ApproveTask(c *gin.Context) {
	a.decideTask(c, "approve")
}

// RejectTask — POST /tasks/:uid/reject (§13.3): rejection is terminal.
func (a *InfraAPI) RejectTask(c *gin.Context) {
	a.decideTask(c, "reject")
}

// decideTask is the shared approve/reject body: load (404), delegate to the
// engine verb, map ErrNotAwaitingApproval to 409, reload and return the task.
func (a *InfraAPI) decideTask(c *gin.Context, verb string) {
	if a.engine == nil {
		stashAudit(c, &auditInfo{ErrorCode: "ENGINE_UNAVAILABLE"})
		httpx.FailedCode(c, http.StatusServiceUnavailable, "ENGINE_UNAVAILABLE", nil)
		return
	}
	task, _, ok := a.loadTask(c)
	if !ok {
		return
	}
	actor := username(c)
	var err error
	if verb == "approve" {
		err = a.engine.Approve(c.Request.Context(), task.UID, actor)
	} else {
		err = a.engine.Reject(c.Request.Context(), task.UID, actor)
	}
	if err != nil {
		if errors.Is(err, tasks.ErrNotAwaitingApproval) {
			stashAudit(c, &auditInfo{TaskUID: task.UID, ErrorCode: "TASK_NOT_AWAITING_APPROVAL"})
			httpx.FailedCode(c, http.StatusConflict, "TASK_NOT_AWAITING_APPROVAL", nil)
			return
		}
		stashAudit(c, &auditInfo{TaskUID: task.UID, ErrorCode: "TASK_DECISION_FAILED"})
		httpx.Failed(c, http.StatusInternalServerError, err.Error())
		return
	}
	var updated model.ProviderTask
	if err := a.db.Where("uid = ?", task.UID).First(&updated).Error; err != nil {
		httpx.Failed(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Success(c, gin.H{"task": newTaskView(updated)})
}

// CancelTask — POST /tasks/:uid/cancel (§3.6): terminal tasks answer 409,
// then the delegation rides the CancelStarter seam (queued/awaiting cancel
// immediately, running raises the cancel_requested flag at the engine —
// N6). The seam is nil until the engine lane's Engine.RequestCancel lands,
// and the route degrades 503 in the meantime.
func (a *InfraAPI) CancelTask(c *gin.Context) {
	task, _, ok := a.loadTask(c)
	if !ok {
		return
	}
	if tasks.IsTerminalTaskStatus(task.Status) {
		stashAudit(c, &auditInfo{TaskUID: task.UID, ErrorCode: "TASK_NOT_CANCELLABLE"})
		httpx.FailedCode(c, http.StatusConflict, "TASK_NOT_CANCELLABLE", nil)
		return
	}
	if a.cancelStarter == nil {
		stashAudit(c, &auditInfo{TaskUID: task.UID, ErrorCode: "ENGINE_CANCEL_UNAVAILABLE"})
		httpx.FailedCode(c, http.StatusServiceUnavailable, "ENGINE_CANCEL_UNAVAILABLE", nil)
		return
	}
	if err := a.cancelStarter.RequestCancel(c.Request.Context(), task.UID, username(c)); err != nil {
		if errors.Is(err, tasks.ErrNotAwaitingApproval) {
			stashAudit(c, &auditInfo{TaskUID: task.UID, ErrorCode: "TASK_NOT_CANCELLABLE"})
			httpx.FailedCode(c, http.StatusConflict, "TASK_NOT_CANCELLABLE", nil)
			return
		}
		stashAudit(c, &auditInfo{TaskUID: task.UID, ErrorCode: "TASK_CANCEL_FAILED"})
		httpx.Failed(c, http.StatusInternalServerError, err.Error())
		return
	}
	var updated model.ProviderTask
	if err := a.db.Where("uid = ?", task.UID).First(&updated).Error; err != nil {
		httpx.Failed(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Success(c, gin.H{"task": newTaskView(updated)})
}
