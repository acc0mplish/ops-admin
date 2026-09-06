package tasks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	gormMysql "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/infra/secrets"
)

// Config is the engine configuration (plan §3.6).
type Config struct {
	WorkerID     string        // 식별자(태스크 시도 기록)
	PollInterval time.Duration // 기본 2s (§13.1 "default interval 2s, configurable")
	LeaseSeconds int           // 클레임 리스
	ReaperGrace  time.Duration // §13.2 "lease_expires_at < NOW() - grace"
}

// Engine is the durable task engine (§13): submit snapshots the registry
// definition, claims run through the §13.2 CAS, terminal commits carry their
// attempt and event in one transaction (D5), and every write is version-CAS'd
// (D4). The engine never branches on provider names — adapters come from the
// registry alone (arch rule 1).
type Engine struct {
	db     *gorm.DB
	reg    *registry.Registry
	cfg    Config
	broker *secrets.Broker

	// Loop lifecycle (loops.go): Start/Stop own these; RunOnce/ReapOnce
	// stay callable directly for deterministic tests.
	loopMu   sync.Mutex
	loopCtx  context.Context
	loopStop context.CancelFunc
	loopWG   sync.WaitGroup
	loopRun  bool
}

// NewEngine returns an engine over db. Phase 1 starts it from tests only —
// main.go wiring is Phase 3's (A6).
func NewEngine(db *gorm.DB, reg *registry.Registry, cfg Config) *Engine {
	return &Engine{
		db:     db,
		reg:    reg,
		cfg:    cfg,
		broker: secrets.NewBroker(db),
	}
}

// SubmitInput — r2 T-10: there is deliberately NO RequiresApproval flag
// here; the canonical source is the registry OperationDefinition the engine
// snapshots at submit time.
type SubmitInput struct {
	OperationName    string
	OperationVersion string
	ResourceUID      string // '' 는 하드 거부(T-5) — active_flag 유니크 우회 봉쇄
	Payload          contract.JSONMap
	IdempotencyKey   string // "" 이면 미설정(단일 실행) — 충돌 회수 경로는 PR 18
}

// Submit ① looks the OperationDefinition up in the registry (hard error when
// unregistered — no retry) and snapshots MaxAttempts/CallTimeoutSeconds plus
// the approval posture (RequiresApproval, §13.3), ② rejects an empty
// ResourceUID (T-5), ③ creates the task planned and transitions it in the
// same transaction — awaiting_approval when the definition requires approval,
// queued otherwise (§13.3), ④ resolves unique violations (T-6): with an
// IdempotencyKey set, a failed insert first attempts the replay recovery —
// the existing task returns with replayed=true (§13.4/N6, idempotency.go) —
// and everything else classifies by index name: resource-active →
// resource_busy hard error (N11), idempotency-miss → retryable conflict
// error (R4). No fallbacks.
func (e *Engine) Submit(ctx context.Context, in SubmitInput) (model.ProviderTask, bool, error) {
	if in.ResourceUID == "" {
		return model.ProviderTask{}, false, ErrEmptyResourceUID
	}
	def, ok := e.reg.Operation(in.OperationName)
	if !ok {
		return model.ProviderTask{}, false, fmt.Errorf("%w: %q", ErrOperationNotRegistered, in.OperationName)
	}
	version := def.Version
	if in.OperationVersion != "" && in.OperationVersion != def.Version {
		return model.ProviderTask{}, false, fmt.Errorf(
			"tasks: operation %q submitted at version %q but the registry serves %q (single-version registry, A13)",
			in.OperationName, in.OperationVersion, def.Version)
	}

	now := time.Now()
	maxAttempts := def.RetryPolicy.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	task := model.ProviderTask{
		UID:                newTaskUID(),
		OperationName:      def.Name,
		OperationVersion:   version,
		ResourceUID:        in.ResourceUID,
		PayloadJSON:        in.Payload,
		Status:             TaskStatusPlanned,
		MaxAttempts:        maxAttempts,
		NextAttemptAt:      &now,
		CallTimeoutSeconds: def.TimeoutSeconds,
		RequiresApproval:   def.RequiresApproval,      // §13.3 전이 판단 스냅샷 (r2 T-10: 정준원천은 registry)
		ApprovalStatus:     ApprovalStatusNotRequired, // 대기 표현은 Status=awaiting_approval이 담당 (approval.go)
	}
	if in.IdempotencyKey != "" { // "" 이면 미설정(단일 실행) — 컬럼은 NULL로 유니크에서 제외
		key := in.IdempotencyKey
		task.IdempotencyKey = &key
	}

	var out model.ProviderTask
	err := e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&task).Error; err != nil {
			return classifySubmitViolation(err)
		}
		if err := appendEvent(tx, task.ID, 0, TaskEventCreated, actorSubmitter,
			contract.JSONMap{"uid": task.UID, "operation": task.OperationName}); err != nil {
			return err
		}
		next := TaskStatusQueued
		if def.RequiresApproval {
			next = TaskStatusAwaitingApproval
		}
		if err := tx.Model(&model.ProviderTask{}).
			Where("id = ? AND version = ?", task.ID, task.Version).
			Updates(map[string]any{"status": next, "version": task.Version + 1}).Error; err != nil {
			return fmt.Errorf("tasks: initial transition to %q: %w", next, err)
		}
		if err := tx.First(&out, task.ID).Error; err != nil {
			return fmt.Errorf("tasks: reload submitted task: %w", err)
		}
		return nil
	})
	if err != nil {
		// Replay-first (idempotency.go): with a key set, a hit proves the
		// insert duplicated that key — return the existing task even when the
		// resource-active unique fired on the same insert (N6 semantics).
		if in.IdempotencyKey != "" {
			if existing, found, replayErr := e.replayByIdempotencyKey(ctx, in.IdempotencyKey); replayErr == nil && found {
				return existing, true, nil
			}
		}
		return model.ProviderTask{}, false, err
	}
	return out, false, nil
}

// classifySubmitViolation maps a submit-time unique violation to its engine
// semantics (§3.6 ④): resource-active → resource_busy hard error. The
// idempotency case is the replay recovery's fallback — it fires only when the
// key collided but no committed task holds it (a concurrent submit's
// transaction is still in flight — the caller retries, R4); the happy replay
// path returns from Submit before this classification is consulted
// (idempotency.go).
func classifySubmitViolation(err error) error {
	index, ok := classifyUniqueViolation(err)
	if !ok {
		return fmt.Errorf("tasks: create provider_task: %w", err)
	}
	switch index {
	case uniqueIndexResourceActive:
		return ErrResourceBusy
	case uniqueIndexIdempotency:
		return fmt.Errorf("tasks: idempotency conflict on %s (retry the submit — R4): %w", index, err)
	default:
		return fmt.Errorf("tasks: unique constraint %s rejected the submit: %w", index, err)
	}
}

// Constraint index names the engine branches on (T-6). uq_provider_task_* are
// created by the PR 18 step 0003 Expand; uq_infra_resource_identity exists
// from PR 16 (a collision there is Phase 2 discovery's business — the engine
// never fires it).
const (
	uniqueIndexIdempotency      = "uq_provider_task_idempotency"
	uniqueIndexResourceActive   = "uq_provider_task_resource_active"
	uniqueIndexResourceIdentity = "uq_infra_resource_identity"
)

// classifyUniqueViolation maps dialect unique-violation errors to the
// constraint's index name (T-6): MySQL 1062 "Duplicate entry … for key
// 'uq_…'" by index name; sqlite "UNIQUE constraint failed: <table>.<col,…>"
// by column set. Non-unique errors return ok=false.
func classifyUniqueViolation(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	var myErr *gormMysql.MySQLError
	if errors.As(err, &myErr) && myErr.Number == 1062 {
		msg := myErr.Message
		const marker = "for key '"
		if i := strings.LastIndex(msg, marker); i >= 0 {
			rest := msg[i+len(marker):]
			if j := strings.IndexByte(rest, '\''); j >= 0 {
				return rest[:j], true
			}
		}
		return "", false
	}
	// sqlite (glebarez): "constraint failed: UNIQUE constraint failed:
	// provider_task.resource_uid, provider_task.active_flag (2067)".
	const marker = "UNIQUE constraint failed:"
	msg := err.Error()
	i := strings.Index(msg, marker)
	if i < 0 {
		return "", false
	}
	rest := msg[i+len(marker):]
	if j := strings.IndexByte(rest, '('); j >= 0 { // strip the sqlite "(2067)" suffix
		rest = rest[:j]
	}
	var cols []string
	for _, part := range strings.Split(rest, ",") {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "provider_task.")
		part = strings.TrimPrefix(part, "infra_resource.")
		if part == "" {
			return "", false
		}
		cols = append(cols, part)
	}
	if len(cols) == 0 {
		return "", false
	}
	// Order-insensitive column-set key.
	sorted := append([]string(nil), cols...)
	for a := 1; a < len(sorted); a++ {
		for b := a; b > 0 && sorted[b] < sorted[b-1]; b-- {
			sorted[b], sorted[b-1] = sorted[b-1], sorted[b]
		}
	}
	switch strings.Join(sorted, ",") {
	case "idempotency_key":
		return uniqueIndexIdempotency, true
	case "active_flag,resource_uid":
		return uniqueIndexResourceActive, true
	case "context_id,external_urn,kind":
		return uniqueIndexResourceIdentity, true
	default:
		return "", false
	}
}

const actorSubmitter = "submitter"

// ClaimedTask pairs the claimed task snapshot (post-CAS version) with the
// attempt row created in the claim transaction.
type ClaimedTask struct {
	Task    model.ProviderTask
	Attempt model.TaskAttempt
}

// errClaimCASLost aborts the claim transaction when the §13.2 UPDATE hit 0
// rows — the caller scans for the next candidate instead of erroring.
var errClaimCASLost = errors.New("tasks: claim CAS lost")

// maxClaimScan bounds the candidate scan per ClaimNext call: contended rows
// (claimed by another worker between select and CAS) are skipped; beyond this
// bound the engine yields to the next cycle rather than spinning.
const maxClaimScan = 128

// ClaimNext claims the next due queued task via the §13.2 single-statement
// CAS (plus the §13.4 version predicate), creating the TaskAttempt row inside
// the same transaction (T-10). A cancel-requested task is terminal-cancelled
// at this boundary (§13.4 — "acted on at the next claim boundary") through
// the diagram-legal running→cancelling→cancelled edges, collapsed in one
// commit.
func (e *Engine) ClaimNext(ctx context.Context) (*ClaimedTask, bool, error) {
	for scan := 0; scan < maxClaimScan; scan++ {
		now := time.Now()
		var task model.ProviderTask
		err := e.db.WithContext(ctx).
			Where("status = ? AND next_attempt_at <= ?", TaskStatusQueued, now).
			Order("next_attempt_at ASC, id ASC").First(&task).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, fmt.Errorf("tasks: select claimable task: %w", err)
		}

		if task.CancelRequested {
			if err := e.cancelAtClaimBoundary(ctx, &task, now); err != nil {
				return nil, false, err
			}
			continue
		}

		claim, ok, err := e.claimTask(ctx, &task, now)
		if err != nil {
			return nil, false, err
		}
		if ok {
			return claim, true, nil
		}
		// CAS lost to a concurrent worker — scan the next candidate.
	}
	return nil, false, nil
}

// claimTask runs the §13.2 claim as one transaction:
//
//	UPDATE provider_task SET status='running', lease_expires_at=?,
//	    attempt_count=attempt_count+1 WHERE id=? AND status='queued'
//	    AND next_attempt_at<=?  (+ version CAS, §13.4)
//
// Comparison times are bound Go-side (sqlite has no now(), T-10). The
// TaskAttempt row (attempt_no = the incremented attempt_count) is created in
// this same transaction (T-10). The lease honors the T-8 lower bound.
func (e *Engine) claimTask(ctx context.Context, task *model.ProviderTask, now time.Time) (*ClaimedTask, bool, error) {
	startedAt := task.StartedAt
	if startedAt == nil {
		startedAt = &now
	}
	leaseUntil := leaseLowerBound(e.cfg, task.CallTimeoutSeconds, now)

	var claimed *ClaimedTask
	err := e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.ProviderTask{}).
			Where("id = ? AND status = ? AND next_attempt_at <= ? AND version = ?",
				task.ID, TaskStatusQueued, now, task.Version).
			Updates(map[string]any{
				"status":           TaskStatusRunning,
				"lease_expires_at": leaseUntil,
				"attempt_count":    gorm.Expr("attempt_count + 1"),
				"version":          task.Version + 1,
				"started_at":       startedAt,
			})
		if res.Error != nil {
			return fmt.Errorf("tasks: claim CAS on task %d: %w", task.ID, res.Error)
		}
		if res.RowsAffected == 0 {
			return errClaimCASLost
		}

		var fresh model.ProviderTask
		if err := tx.First(&fresh, task.ID).Error; err != nil {
			return fmt.Errorf("tasks: reload claimed task %d: %w", task.ID, err)
		}
		attempt := model.TaskAttempt{
			TaskID:    fresh.ID,
			AttemptNo: fresh.AttemptCount,
			WorkerID:  e.cfg.WorkerID,
			StartedAt: now,
		}
		if err := tx.Create(&attempt).Error; err != nil {
			return fmt.Errorf("tasks: create attempt for task %d: %w", fresh.ID, err)
		}
		if err := appendEvent(tx, fresh.ID, fresh.AttemptCount, TaskEventClaimed, e.cfg.WorkerID,
			contract.JSONMap{"worker": e.cfg.WorkerID, "lease_expires_at": leaseUntil}); err != nil {
			return err
		}
		claimed = &ClaimedTask{Task: fresh, Attempt: attempt}
		return nil
	})
	if err != nil {
		if errors.Is(err, errClaimCASLost) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return claimed, true, nil
}

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

// leaseLowerBound — T-8: the lease must outlive the attempt. If it can expire
// before synchronous completion (execute + async polls), the reaper requeues a
// task whose execution is still in flight — double execution. Contract:
//
//	lease = now + max(LeaseSeconds, CallTimeoutSeconds) + ReaperGrace
func leaseLowerBound(cfg Config, callTimeoutSeconds int, now time.Time) time.Time {
	seconds := cfg.LeaseSeconds
	if callTimeoutSeconds > seconds {
		seconds = callTimeoutSeconds
	}
	return now.Add(time.Duration(seconds) * time.Second).Add(cfg.ReaperGrace)
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

// newTaskUID — A12: crypto/rand 16-byte hex (32 chars), generated in-repo (no
// uuid dependency — go.mod is frozen). A crypto/rand failure is a dead
// process; panicking is the honest posture.
func newTaskUID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic("tasks: crypto/rand failed for UID generation: " + err.Error())
	}
	return hex.EncodeToString(buf[:])
}

// ---------------------------------------------------------------------------
// 실행 회로 (T-7/T-8) — 체인 조인·UID→URN·어댑터 결합·비동기 폴.
// ---------------------------------------------------------------------------

// executionChain is the T-7 join result: resource_uid → infra_resource →
// provider_context → provider_connection → ProviderType.
type executionChain struct {
	ConnectionUID string
	ProviderType  string
	ExternalURN   string
}

// resolveExecutionChain walks the three-level join. Soft-deleted resources
// are outside the default scope (deleted_at IS NULL) — a miss (unknown or
// deleted resource, or a broken chain) is an `unknown resource` hard error
// the caller terminal-fails without retry (T-7).
func (e *Engine) resolveExecutionChain(ctx context.Context, resourceUID string) (executionChain, error) {
	var row executionChain
	res := e.db.WithContext(ctx).
		Table("infra_resource").
		Select("provider_connection.uid AS connection_uid, provider_connection.provider_type AS provider_type, infra_resource.external_urn AS external_urn").
		Joins("JOIN provider_context ON provider_context.id = infra_resource.context_id").
		Joins("JOIN provider_connection ON provider_connection.id = provider_context.connection_id").
		Where("infra_resource.uid = ? AND infra_resource.deleted_at IS NULL", resourceUID).
		Limit(1).
		Scan(&row)
	if res.Error != nil {
		return executionChain{}, fmt.Errorf("tasks: execution chain join for %q: %w", resourceUID, res.Error)
	}
	if row.ConnectionUID == "" {
		return executionChain{}, fmt.Errorf("tasks: unknown resource %q — no live resource→context→connection chain (T-7)", resourceUID)
	}
	return row, nil
}

// connectionView assembles the §7.2-derived view the execution path hands to
// adapters, filling Material through the §7.4/§3.5 broker. A connection with
// NO operations binding (the fake / Phase 1 gate path) runs with nil Material
// — "fake는 Config만으로 동작" (§3.5). A binding that exists but fails to
// resolve is a hard error: executing without the configured credential would
// be the silent hole §3.5 closes.
func (e *Engine) connectionView(ctx context.Context, chain executionChain) (contract.ConnectionView, error) {
	var conn model.ProviderConnection
	if err := e.db.WithContext(ctx).Where("uid = ?", chain.ConnectionUID).First(&conn).Error; err != nil {
		return contract.ConnectionView{}, fmt.Errorf("tasks: load connection %q: %w", chain.ConnectionUID, err)
	}
	view := contract.ConnectionView{
		ProviderType: conn.ProviderType,
		Endpoint:     conn.Endpoint,
		Config:       conn.ConfigJSON,
	}
	var bindings int64
	if err := e.db.WithContext(ctx).Model(&model.ProviderCredentialBinding{}).
		Where("provider_connection_id = ? AND purpose = ?", conn.ID, contract.CredentialPurposeOperations).
		Count(&bindings).Error; err != nil {
		return contract.ConnectionView{}, fmt.Errorf("tasks: probe operations bindings for %q: %w", chain.ConnectionUID, err)
	}
	if bindings == 0 {
		return view, nil
	}
	resolved, err := e.broker.Resolve(ctx, conn.UID, contract.CredentialPurposeOperations)
	if err != nil {
		return contract.ConnectionView{}, fmt.Errorf("tasks: %s: %w", ErrorCodeCredentialError, err)
	}
	view.Material = map[string]string{contract.CredentialPurposeOperations: resolved.Value}
	return view, nil
}

// capabilityServed — the adapter choice is a registry lookup only (arch
// rule 1): the connection's provider type must have declared the operation's
// required capability.
func capabilityServed(reg *registry.Registry, providerType, capability string) bool {
	for _, c := range reg.Capabilities(providerType) {
		if c.Name == capability {
			return true
		}
	}
	return false
}

// executeClaimed runs one claimed attempt end to end: chain join → registry
// lookups → broker material → Execute → (null handle: synchronous terminal) |
// (handle: persist on the attempt, Poll once this same cycle — §14.3 dual
// mode). Task-level failures terminal-fail or requeue through Fail; only
// infrastructure errors are returned.
func (e *Engine) executeClaimed(ctx context.Context, claim *ClaimedTask) error {
	chain, err := e.resolveExecutionChain(ctx, claim.Task.ResourceUID)
	if err != nil {
		return e.Fail(ctx, claim, ErrorCodeUnknownResource, err.Error()) // T-7: no retry
	}
	def, ok := e.reg.Operation(claim.Task.OperationName)
	if !ok {
		return e.Fail(ctx, claim, ErrorCodeUnknownOperation, "operation definition vanished from the registry")
	}
	_, adapter, ok := e.reg.ProviderType(chain.ProviderType)
	if !ok {
		return e.Fail(ctx, claim, ErrorCodeNoExecutor, fmt.Sprintf("provider type %q is not registered", chain.ProviderType))
	}
	executor, isExecutor := adapter.(contract.OperationExecutor)
	if !isExecutor {
		return e.Fail(ctx, claim, ErrorCodeNoExecutor, fmt.Sprintf("adapter for %q implements no OperationExecutor (§3.7)", chain.ProviderType))
	}
	if !capabilityServed(e.reg, chain.ProviderType, def.RequiredCapability) {
		return e.Fail(ctx, claim, ErrorCodeCapabilityNotServed,
			fmt.Sprintf("provider type %q does not declare capability %q required by %q", chain.ProviderType, def.RequiredCapability, def.Name))
	}
	if _, err := e.connectionView(ctx, chain); err != nil {
		return e.Fail(ctx, claim, ErrorCodeCredentialError, err.Error())
	}

	execCtx := ctx
	cancel := func() {}
	if claim.Task.CallTimeoutSeconds > 0 {
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(claim.Task.CallTimeoutSeconds)*time.Second)
	}
	defer cancel()

	// UID→URN 변환은 엔진의 책임 (T-7): OperationRequest.ResourceURN은
	// infra_resource.external_urn으로 조립된다.
	handle, err := executor.Execute(execCtx, contract.OperationRequest{
		OperationName: claim.Task.OperationName,
		ResourceURN:   chain.ExternalURN,
		Payload:       claim.Task.PayloadJSON,
	})
	if err != nil {
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return e.Fail(ctx, claim, ErrorCodeTimedOut, fmt.Sprintf("provider call exceeded CallTimeoutSeconds=%d", claim.Task.CallTimeoutSeconds))
		}
		return e.Fail(ctx, claim, ErrorCodeExecutorError, err.Error())
	}

	if handle.ProviderRef == "" {
		// §14.3 PVE shape: null-with-exit-status → single-attempt success.
		return e.Complete(ctx, claim, nil)
	}

	// Async: persist the provider-native handle on THIS attempt (no new
	// attempt — §3.6), then Poll once in the same cycle.
	if err := e.db.WithContext(ctx).Model(&model.TaskAttempt{}).
		Where("id = ?", claim.Attempt.ID).
		Update("handle_ref", handle.ProviderRef).Error; err != nil {
		return fmt.Errorf("tasks: persist handle for attempt %d: %w", claim.Attempt.ID, err)
	}
	claim.Attempt.HandleRef = handle.ProviderRef

	poller, isPoller := adapter.(contract.TaskPoller)
	if !isPoller {
		return e.Fail(ctx, claim, ErrorCodeNoExecutor,
			fmt.Sprintf("adapter for %q returned a provider handle but implements no TaskPoller (§14.3 dual mode)", chain.ProviderType))
	}
	status, err := poller.Poll(execCtx, handle)
	if err != nil {
		// A transient poll error leaves the attempt open: the next cycle polls
		// again without re-executing; a genuinely dead worker is the reaper's
		// business (bounded by the lease).
		return nil
	}
	return e.applyOperationStatus(ctx, claim, status)
}

// applyOperationStatus maps the §3.9 adapter state onto the task:
// Succeeded→succeeded, Failed→failed (retry policy applies), Running→keep
// running (next poll cycle; §13.5 "Polling collapses into Running").
func (e *Engine) applyOperationStatus(ctx context.Context, claim *ClaimedTask, status contract.OperationStatus) error {
	switch status.State {
	case contract.OperationStateSucceeded:
		return e.Complete(ctx, claim, status.Detail)
	case contract.OperationStateFailed:
		msg := "provider reported failure"
		if detail := status.Detail; detail != nil {
			msg = fmt.Sprintf("provider reported failure: %v", detail)
		}
		return e.Fail(ctx, claim, ErrorCodeOperationFailed, msg)
	default:
		return nil // running — poll again next cycle
	}
}

// pollAsyncAttempts advances every running task with an open async attempt
// (§3.6): Poll once per cycle, no new attempts. Tasks whose status is not
// running anymore (reaped, cancelled) are skipped — their attempts close via
// those paths.
func (e *Engine) pollAsyncAttempts(ctx context.Context) error {
	var tasks []model.ProviderTask
	err := e.db.WithContext(ctx).
		Joins("JOIN task_attempt ON task_attempt.task_id = provider_task.id AND task_attempt.finished_at IS NULL AND task_attempt.handle_ref <> ''").
		Where("provider_task.status = ?", TaskStatusRunning).
		Distinct().
		Find(&tasks).Error
	if err != nil {
		return fmt.Errorf("tasks: select async in-flight tasks: %w", err)
	}
	for i := range tasks {
		task := tasks[i]
		var attempt model.TaskAttempt
		err := e.db.WithContext(ctx).
			Where("task_id = ? AND finished_at IS NULL AND handle_ref <> ''", task.ID).
			Order("id DESC").First(&attempt).Error
		if err != nil {
			continue // raced to terminal — nothing to poll
		}
		chain, err := e.resolveExecutionChain(ctx, task.ResourceUID)
		if err != nil {
			// The resource vanished mid-flight (deleted between claim and poll)
			// — the attempt cannot advance; let the lease/reaper close it.
			continue
		}
		_, adapter, ok := e.reg.ProviderType(chain.ProviderType)
		if !ok {
			continue
		}
		poller, isPoller := adapter.(contract.TaskPoller)
		if !isPoller {
			continue
		}
		claim := &ClaimedTask{Task: task, Attempt: attempt}
		status, err := poller.Poll(ctx, contract.OperationHandle{ProviderRef: attempt.HandleRef})
		if err != nil {
			continue // transient — next cycle
		}
		if err := e.applyOperationStatus(ctx, claim, status); err != nil {
			return err
		}
	}
	return nil
}
