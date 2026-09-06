// Package tasks is the durable task engine of spec §13: claim-by-CAS
// execution, a §13.5 state machine, a stale-lease reaper, and the registry-
// driven execution path. Phase 1 starts it only from tests — main.go wiring
// is Phase 3's (plan A6).
package tasks

// Task statuses — spec §13.5 state model verbatim (9 states).
const (
	TaskStatusPlanned          = "planned"
	TaskStatusAwaitingApproval = "awaiting_approval"
	TaskStatusQueued           = "queued"
	TaskStatusRunning          = "running"
	TaskStatusSucceeded        = "succeeded"
	TaskStatusFailed           = "failed"
	TaskStatusTimedOut         = "timed_out"
	TaskStatusCancelling       = "cancelling"
	TaskStatusCancelled        = "cancelled"
)

// TaskStatuses is the closed 9-state vocabulary (§13.5 diagram nodes).
var TaskStatuses = []string{
	TaskStatusPlanned,
	TaskStatusAwaitingApproval,
	TaskStatusQueued,
	TaskStatusRunning,
	TaskStatusSucceeded,
	TaskStatusFailed,
	TaskStatusTimedOut,
	TaskStatusCancelling,
	TaskStatusCancelled,
}

// IsTaskStatus reports membership in the closed §13.5 vocabulary.
func IsTaskStatus(status string) bool {
	for _, s := range TaskStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// terminalTaskStatuses — the terminal set is exactly four (r2 A): succeeded,
// failed, timed_out, cancelled. It is the same list as the step0003
// active_flag CASE (plan §3.4) — 'rejected' is NOT a task status; it exists
// only as an ApprovalStatus value (PR 18 column) and as a task_event type.
var terminalTaskStatuses = map[string]bool{
	TaskStatusSucceeded: true,
	TaskStatusFailed:    true,
	TaskStatusTimedOut:  true,
	TaskStatusCancelled: true,
}

// IsTerminalTaskStatus reports whether status is one of the four terminal
// task statuses (r2 A — §3.4 DDL CASE 목록과 동치).
func IsTerminalTaskStatus(status string) bool {
	return terminalTaskStatuses[status]
}

// taskTransitions is the §13.5 stateDiagram-v2 edge set verbatim — 11 named
// transitions. Polling collapses into Running (§13.5); the [*]→Planned edge
// is creation, not a transition.
var taskTransitions = map[string]map[string]bool{
	TaskStatusPlanned: {
		TaskStatusAwaitingApproval: true, // §13.3 RequiresApproval snapshot
		TaskStatusQueued:           true,
	},
	TaskStatusAwaitingApproval: {
		TaskStatusQueued:    true, // §13.3 approve (PR 18 API)
		TaskStatusCancelled: true, // §13.3 rejection is terminal
	},
	TaskStatusQueued: {
		TaskStatusRunning: true, // §13.2 claim
	},
	TaskStatusRunning: {
		TaskStatusSucceeded:  true,
		TaskStatusFailed:     true,
		TaskStatusTimedOut:   true, // engine-derived from CallTimeout/Deadline (§3.9)
		TaskStatusCancelling: true,
	},
	TaskStatusCancelling: {
		TaskStatusCancelled: true,
	},
	TaskStatusFailed: {
		TaskStatusQueued: true, // §13.2/§13.5 retry when attempts remain
	},
}

// CanTransition reports whether from→to is one of the 11 §13.5 edges.
func CanTransition(from, to string) bool {
	return taskTransitions[from][to]
}

// taskTransitionsEdges flattens the table (test helper for the 11-edge count).
func taskTransitionsEdges() [][2]string {
	var edges [][2]string
	for from, targets := range taskTransitions {
		for to := range targets {
			edges = append(edges, [2]string{from, to})
		}
	}
	return edges
}

// Task event types — A4 vocabulary. The only spec-named type is
// 'reaper_requeued' (§13.2 verbatim); the rest are derived from the §13.5
// state machine. 'rejected' appears HERE and as an ApprovalStatus value —
// never as a provider_task.Status (r2 A).
const (
	TaskEventCreated         = "created"
	TaskEventApproved        = "approved"
	TaskEventRejected        = "rejected"
	TaskEventClaimed         = "claimed"
	TaskEventAttemptFailed   = "attempt_failed"
	TaskEventReaperRequeued  = "reaper_requeued" // §13.2 verbatim
	TaskEventSucceeded       = "succeeded"
	TaskEventFailed          = "failed"
	TaskEventTimedOut        = "timed_out"
	TaskEventCancelRequested = "cancel_requested"
	TaskEventCancelled       = "cancelled"
)

// TaskEventTypes is the closed event vocabulary (A4).
var TaskEventTypes = []string{
	TaskEventCreated,
	TaskEventApproved,
	TaskEventRejected,
	TaskEventClaimed,
	TaskEventAttemptFailed,
	TaskEventReaperRequeued,
	TaskEventSucceeded,
	TaskEventFailed,
	TaskEventTimedOut,
	TaskEventCancelRequested,
	TaskEventCancelled,
}

// IsTaskEventType reports membership in the closed event vocabulary.
func IsTaskEventType(typ string) bool {
	for _, t := range TaskEventTypes {
		if t == typ {
			return true
		}
	}
	return false
}

// Engine error codes recorded on provider_task.error_code. Spec-named:
// lease_expired (§13.2), resource_busy (§13.4). Engine-defined (A4 인접):
// unknown_resource/unknown_operation/no_executor/capability_not_served/
// credential_error/executor_error/operation_failed/timed_out.
const (
	ErrorCodeLeaseExpired        = "lease_expired"     // §13.2
	ErrorCodeResourceBusy        = "resource_busy"     // §13.4 (fires from PR 18's unique index)
	ErrorCodeUnknownResource     = "unknown_resource"  // T-7 chain join miss — no retry
	ErrorCodeUnknownOperation    = "unknown_operation" // def vanished from the registry
	ErrorCodeNoExecutor          = "no_executor"       // adapter lacks the required interface
	ErrorCodeCapabilityNotServed = "capability_not_served"
	ErrorCodeCredentialError     = "credential_error" // binding present but resolution failed
	ErrorCodeExecutorError       = "executor_error"   // Go-level Execute/Poll error (retryable)
	ErrorCodeOperationFailed     = "operation_failed" // adapter-reported failure
	ErrorCodeTimedOut            = "timed_out"        // engine-derived terminal (§3.9)
)

// Sentinel errors surfaced to engine callers. Task-level execution failures
// are recorded on the task row, not returned as engine errors.
var (
	// ErrEmptyResourceUID — T-5: '' would bypass the step0003 active_flag
	// unique; Submit hard-rejects before any row exists.
	ErrEmptyResourceUID = errEmptyResourceUID{}
	// ErrOperationNotRegistered — the registry is the canonical operation
	// source (T-10); an unknown name is a submit-time hard error.
	ErrOperationNotRegistered = errOperationNotRegistered{}
	// ErrResourceBusy — §13.4: a second active mutation of one resource.
	ErrResourceBusy = errResourceBusy{}
	// ErrStaleWrite — D4: the writer's version snapshot no longer matches.
	ErrStaleWrite = errStaleWrite{}
)

type errEmptyResourceUID struct{}

func (errEmptyResourceUID) Error() string {
	return "tasks: SubmitInput.ResourceUID must not be empty — it would bypass the per-resource active unique (T-5)"
}

type errOperationNotRegistered struct{}

func (errOperationNotRegistered) Error() string {
	return "tasks: operation is not registered in the registry (canonical source, T-10)"
}

type errResourceBusy struct{}

func (errResourceBusy) Error() string {
	return "tasks: resource already has an active mutation (resource_busy, §13.4)"
}

type errStaleWrite struct{}

func (errStaleWrite) Error() string {
	return "tasks: stale write rejected — the task version moved on (optimistic CAS, §13.4)"
}
