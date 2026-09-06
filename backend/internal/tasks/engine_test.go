// Package tasks_test is the in-package test suite for the durable task
// engine (spec §13; plan §3.6). Internal package: it asserts unexported
// contracts too (classifyUniqueViolation, leaseLowerBound).
package tasks

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	gormMysql "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/adapter/fake"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/testutil"
	"ops-admin/backend/util"
)

// engineFixture is one engine over one in-memory database with the task and
// execution-chain tables present and a default operation registered.
type engineFixture struct {
	db  *gorm.DB
	reg *registry.Registry
	eng *Engine
}

func testConfig() Config {
	return Config{
		WorkerID:     "test-worker",
		PollInterval: 2 * time.Second,
		LeaseSeconds: 60,
		ReaperGrace:  5 * time.Second,
	}
}

// newTestDB opens an in-memory database with every table the engine touches.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&model.ProviderTask{}, &model.TaskAttempt{}, &model.TaskEvent{},
		&model.ProviderConnection{}, &model.ProviderContext{},
		&model.ProviderCredentialBinding{}, &model.SecretRef{}, &model.InfraResource{},
	); err != nil {
		t.Fatalf("AutoMigrate engine tables: %v", err)
	}
	return db
}

// testOperation is a valid OperationDefinition for the fake execution path:
// mutating, apply-capability, retryable.
func testOperation(name string) contract.OperationDefinition {
	return contract.OperationDefinition{
		Name:               name,
		Version:            "1",
		ResourceKinds:      []string{"orchestration.workload"},
		RequiredCapability: "orchestration.kubernetes.apply",
		RequiredPermission: "assets:k8s:workload:restart",
		Mutating:           true,
		RiskLevel:          "medium",
		TimeoutSeconds:     30,
		RetryPolicy:        contract.RetryPolicy{MaxAttempts: 2, BackoffSeconds: 1},
	}
}

// newEngineFixture registers the default operation and returns a ready engine.
func newEngineFixture(t *testing.T, cfg Config) *engineFixture {
	t.Helper()
	db := newTestDB(t)
	reg := registry.New()
	if err := reg.RegisterOperation(testOperation("fake.workload.restart")); err != nil {
		t.Fatalf("seed operation: %v", err)
	}
	return &engineFixture{db: db, reg: reg, eng: NewEngine(db, reg, cfg)}
}

// seedTask inserts a task row directly (bypassing Submit) for CAS tests.
func seedTask(t *testing.T, db *gorm.DB, mutate func(*model.ProviderTask)) model.ProviderTask {
	t.Helper()
	now := time.Now()
	task := model.ProviderTask{
		UID:                fmt.Sprintf("task-%d", time.Now().UnixNano()),
		OperationName:      "fake.workload.restart",
		OperationVersion:   "1",
		ResourceUID:        "res-uid-1",
		Status:             TaskStatusQueued,
		MaxAttempts:        2,
		NextAttemptAt:      &now,
		CallTimeoutSeconds: 30,
	}
	if mutate != nil {
		mutate(&task)
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	return task
}

func reloadTask(t *testing.T, db *gorm.DB, id uint) model.ProviderTask {
	t.Helper()
	var task model.ProviderTask
	if err := db.First(&task, id).Error; err != nil {
		t.Fatalf("reload task %d: %v", id, err)
	}
	return task
}

func eventsOf(t *testing.T, db *gorm.DB, taskID uint) []model.TaskEvent {
	t.Helper()
	var events []model.TaskEvent
	if err := db.Where("task_id = ?", taskID).Order("id").Find(&events).Error; err != nil {
		t.Fatalf("load events: %v", err)
	}
	return events
}

// T24 — TestStateMachineMatchesSpec: the §13.5 transition table verbatim —
// exactly 11 allowed transitions, every other pair rejected, and the terminal
// set is exactly the four {succeeded, failed, timed_out, cancelled}. 'rejected'
// is NOT a task status (r2 A): it exists only as a future ApprovalStatus value
// and as a task_event type.
func TestStateMachineMatchesSpec(t *testing.T) {
	allowed := [][2]string{
		{TaskStatusPlanned, TaskStatusAwaitingApproval},
		{TaskStatusPlanned, TaskStatusQueued},
		{TaskStatusAwaitingApproval, TaskStatusQueued},
		{TaskStatusAwaitingApproval, TaskStatusCancelled},
		{TaskStatusQueued, TaskStatusRunning},
		{TaskStatusRunning, TaskStatusSucceeded},
		{TaskStatusRunning, TaskStatusFailed},
		{TaskStatusRunning, TaskStatusTimedOut},
		{TaskStatusRunning, TaskStatusCancelling},
		{TaskStatusCancelling, TaskStatusCancelled},
		{TaskStatusFailed, TaskStatusQueued},
	}
	if got := len(taskTransitionsEdges()); got != 11 {
		t.Fatalf("transition table has %d edges, want exactly 11 (§13.5)", got)
	}
	allowedSet := map[[2]string]bool{}
	for _, e := range allowed {
		allowedSet[e] = true
		if !CanTransition(e[0], e[1]) {
			t.Errorf("CanTransition(%s, %s) = false, want true (§13.5)", e[0], e[1])
		}
	}
	for _, from := range TaskStatuses {
		for _, to := range TaskStatuses {
			if allowedSet[[2]string{from, to}] {
				continue
			}
			if CanTransition(from, to) {
				t.Errorf("CanTransition(%s, %s) = true, want false (not in §13.5)", from, to)
			}
		}
	}

	// Closed 9-state vocabulary (§13.5 diagram nodes).
	if len(TaskStatuses) != 9 {
		t.Fatalf("TaskStatuses has %d entries, want 9", len(TaskStatuses))
	}
	// Terminal set is exactly the four — the same list as the step0003
	// active_flag CASE (r2 A 동치).
	wantTerminal := map[string]bool{
		TaskStatusSucceeded: true, TaskStatusFailed: true,
		TaskStatusTimedOut: true, TaskStatusCancelled: true,
	}
	for _, s := range TaskStatuses {
		if IsTerminalTaskStatus(s) != wantTerminal[s] {
			t.Errorf("IsTerminalTaskStatus(%s) = %v, want %v", s, IsTerminalTaskStatus(s), wantTerminal[s])
		}
	}
	if IsTerminalTaskStatus("rejected") {
		t.Error("'rejected' must not be a terminal task status — it is an ApprovalStatus value/task_event type only (r2 A)")
	}
	if IsTaskStatus("rejected") {
		t.Error("'rejected' must not be a task status at all (r2 A)")
	}
	if !IsTaskEventType(TaskEventRejected) {
		t.Error("'rejected' must be a task_event type (A4)")
	}
	if !IsTaskEventType(TaskEventReaperRequeued) {
		t.Error("'reaper_requeued' must be a task_event type (§13.2 verbatim)")
	}
}

// T-6 — TestClassifyUniqueViolation: dialect unique-violation errors map to
// the constraint index names the submit path branches on.
func TestClassifyUniqueViolation(t *testing.T) {
	cases := []struct {
		label string
		err   error
		index string
		ok    bool
	}{
		{
			label: "mysql 1062 resource active",
			err:   &gormMysql.MySQLError{Number: 1062, Message: "Duplicate entry 'res-1-1' for key 'uq_provider_task_resource_active'"},
			index: "uq_provider_task_resource_active",
			ok:    true,
		},
		{
			// MySQL 8 (verified 8.0.46 via the tier-2 suite) qualifies the
			// key as '<table>.<index>' — the bare name must still come out.
			label: "mysql 1062 table-qualified resource active",
			err:   &gormMysql.MySQLError{Number: 1062, Message: "Duplicate entry 'res-1-1' for key 'provider_task.uq_provider_task_resource_active'"},
			index: "uq_provider_task_resource_active",
			ok:    true,
		},
		{
			label: "mysql 1062 idempotency",
			err:   &gormMysql.MySQLError{Number: 1062, Message: "Duplicate entry 'idem-1' for key 'uq_provider_task_idempotency'"},
			index: "uq_provider_task_idempotency",
			ok:    true,
		},
		{
			label: "mysql 1062 identity",
			err:   &gormMysql.MySQLError{Number: 1062, Message: "Duplicate entry '7-urn' for key 'uq_infra_resource_identity'"},
			index: "uq_infra_resource_identity",
			ok:    true,
		},
		{
			label: "mysql other error number",
			err:   &gormMysql.MySQLError{Number: 1048, Message: "Column cannot be null"},
			ok:    false,
		},
		{
			label: "sqlite single column",
			err:   fmt.Errorf("constraint failed: UNIQUE constraint failed: provider_task.idempotency_key (2067)"),
			index: "uq_provider_task_idempotency",
			ok:    true,
		},
		{
			label: "sqlite composite column",
			err:   fmt.Errorf("constraint failed: UNIQUE constraint failed: provider_task.resource_uid, provider_task.active_flag (2067)"),
			index: "uq_provider_task_resource_active",
			ok:    true,
		},
		{
			label: "sqlite identity composite",
			err:   fmt.Errorf("UNIQUE constraint failed: infra_resource.context_id, infra_resource.kind, infra_resource.external_urn"),
			index: "uq_infra_resource_identity",
			ok:    true,
		},
		{
			label: "wrapped unique error",
			err:   fmt.Errorf("create provider_task: %w", &gormMysql.MySQLError{Number: 1062, Message: "Duplicate entry 'x' for key 'uq_provider_task_resource_active'"}),
			index: "uq_provider_task_resource_active",
			ok:    true,
		},
		{
			label: "not a unique violation",
			err:   errors.New("connection refused"),
			ok:    false,
		},
		{
			label: "nil error",
			ok:    false,
		},
	}
	for _, tc := range cases {
		index, ok := classifyUniqueViolation(tc.err)
		if ok != tc.ok || index != tc.index {
			t.Errorf("%s: classifyUniqueViolation = (%q, %v), want (%q, %v)", tc.label, index, ok, tc.index, tc.ok)
		}
	}
}

// T40 — TestSubmitRejectsEmptyResourceUID: ResourceUID=” is a hard rejection
// with no row created (T-5 — it would bypass the step0003 active_flag unique).
func TestSubmitRejectsEmptyResourceUID(t *testing.T) {
	f := newEngineFixture(t, testConfig())

	_, _, err := f.eng.Submit(context.Background(), SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "",
		Payload:       contract.JSONMap{"k": "v"},
	})
	if !errors.Is(err, ErrEmptyResourceUID) {
		t.Fatalf("Submit with empty ResourceUID err = %v, want ErrEmptyResourceUID", err)
	}
	var count int64
	f.db.Model(&model.ProviderTask{}).Count(&count)
	if count != 0 {
		t.Fatalf("provider_task rows = %d after rejected submit, want 0", count)
	}
}

// Submit 기본 계약 — registry 정준원천 스냅샷(T-10: SubmitInput에 승인 플래그 없음 —
// OperationDefinition이 정준원천), 전이·이벤트·UID 생성.
func TestSubmitSnapshotsRegistryDefinition(t *testing.T) {
	f := newEngineFixture(t, testConfig())
	ctx := context.Background()

	task, replayed, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
		Payload:       contract.JSONMap{"k": "v"},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if replayed {
		t.Error("replayed = true on first submit, want false")
	}
	if task.Status != TaskStatusQueued {
		t.Errorf("status = %q, want queued (planned→queued, §13.5; def.RequiresApproval=false)", task.Status)
	}
	if task.MaxAttempts != 2 {
		t.Errorf("MaxAttempts = %d, want 2 (def.RetryPolicy snapshot)", task.MaxAttempts)
	}
	if task.CallTimeoutSeconds != 30 {
		t.Errorf("CallTimeoutSeconds = %d, want 30 (def.TimeoutSeconds snapshot)", task.CallTimeoutSeconds)
	}
	if task.OperationVersion != "1" {
		t.Errorf("OperationVersion = %q, want def version 1", task.OperationVersion)
	}
	if len(task.UID) != 32 {
		t.Errorf("UID = %q, want 32-char crypto/rand hex (A12)", task.UID)
	}
	if task.NextAttemptAt == nil {
		t.Error("NextAttemptAt is nil — §13.2 claim condition would never match")
	}
	if task.Version != 1 {
		t.Errorf("Version = %d, want 1 (create + planned→queued transition)", task.Version)
	}
	events := eventsOf(t, f.db, task.ID)
	if len(events) != 1 || events[0].Type != TaskEventCreated {
		t.Fatalf("events after submit = %v, want exactly [created]", eventTypes(events))
	}

	// Unknown operation is a hard error (정의 미등록 = 재시도 없음).
	if _, _, err := f.eng.Submit(ctx, SubmitInput{OperationName: "nope.op", ResourceUID: "r"}); !errors.Is(err, ErrOperationNotRegistered) {
		t.Errorf("Submit with unknown operation err = %v, want ErrOperationNotRegistered", err)
	}
	// Version mismatch against the registry is a hard error (single-version registry, A13).
	if _, _, err := f.eng.Submit(ctx, SubmitInput{OperationName: "fake.workload.restart", OperationVersion: "9", ResourceUID: "r"}); err == nil {
		t.Error("Submit with mismatched OperationVersion accepted")
	}
	// RequiresApproval=true lands in awaiting_approval (§13.3) — the flag is
	// read from the registry definition, not from SubmitInput (r2 T-10).
	if err := f.reg.RegisterOperation(approvalOperation("fake.workload.approved.restart")); err != nil {
		t.Fatalf("seed approval operation: %v", err)
	}
	approvalTask, _, err := f.eng.Submit(ctx, SubmitInput{OperationName: "fake.workload.approved.restart", ResourceUID: "res-uid-2"})
	if err != nil {
		t.Fatalf("Submit(approval): %v", err)
	}
	if approvalTask.Status != TaskStatusAwaitingApproval {
		t.Errorf("approval task status = %q, want awaiting_approval (§13.3)", approvalTask.Status)
	}
}

func approvalOperation(name string) contract.OperationDefinition {
	def := testOperation(name)
	def.RequiresApproval = true
	return def
}

func eventTypes(events []model.TaskEvent) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.Type)
	}
	return out
}

// T25 — TestClaimIsCAS: the §13.2 single-statement claim — every non-claimable
// shape yields RowsAffected 0 (no claim), the happy path flips the row exactly
// once and creates the attempt inside the same transaction.
func TestClaimIsCAS(t *testing.T) {
	t.Run("not queued is not claimable", func(t *testing.T) {
		f := newEngineFixture(t, testConfig())
		seedTask(t, f.db, func(task *model.ProviderTask) { task.Status = TaskStatusPlanned })
		claimed, ok, err := f.eng.ClaimNext(context.Background())
		if err != nil || ok || claimed != nil {
			t.Fatalf("ClaimNext(planned) = (%v, %v, %v), want (nil, false, nil)", claimed, ok, err)
		}
	})

	t.Run("future next_attempt_at is not claimable", func(t *testing.T) {
		f := newEngineFixture(t, testConfig())
		future := time.Now().Add(10 * time.Minute)
		seedTask(t, f.db, func(task *model.ProviderTask) { task.NextAttemptAt = &future })
		claimed, ok, err := f.eng.ClaimNext(context.Background())
		if err != nil || ok || claimed != nil {
			t.Fatalf("ClaimNext(future) = (%v, %v, %v), want (nil, false, nil)", claimed, ok, err)
		}
	})

	t.Run("already running is not claimable", func(t *testing.T) {
		f := newEngineFixture(t, testConfig())
		seedTask(t, f.db, func(task *model.ProviderTask) { task.Status = TaskStatusRunning })
		claimed, ok, err := f.eng.ClaimNext(context.Background())
		if err != nil || ok || claimed != nil {
			t.Fatalf("ClaimNext(running) = (%v, %v, %v), want (nil, false, nil)", claimed, ok, err)
		}
	})

	t.Run("happy path claims once and creates the attempt in the claim tx", func(t *testing.T) {
		f := newEngineFixture(t, testConfig())
		seeded := seedTask(t, f.db, nil)

		claim, ok, err := f.eng.ClaimNext(context.Background())
		if err != nil || !ok || claim == nil {
			t.Fatalf("ClaimNext = (%v, %v, %v), want a claim", claim, ok, err)
		}
		if claim.Task.ID != seeded.ID {
			t.Fatalf("claimed task %d, want %d", claim.Task.ID, seeded.ID)
		}
		got := reloadTask(t, f.db, seeded.ID)
		if got.Status != TaskStatusRunning {
			t.Errorf("status = %q, want running", got.Status)
		}
		if got.AttemptCount != 1 {
			t.Errorf("attempt_count = %d, want 1", got.AttemptCount)
		}
		if got.Version != seeded.Version+1 {
			t.Errorf("version = %d, want %d (CAS increments)", got.Version, seeded.Version+1)
		}
		if got.LeaseExpiresAt == nil {
			t.Fatal("lease_expires_at is nil after claim")
		}
		if got.StartedAt == nil {
			t.Error("started_at is nil after first claim (§18.2 claim→terminal measurement)")
		}
		// Attempt row — same transaction (T-10), attempt_no = incremented count.
		if claim.Attempt.TaskID != seeded.ID || claim.Attempt.AttemptNo != 1 {
			t.Errorf("attempt = (task %d, no %d), want (task %d, no 1)", claim.Attempt.TaskID, claim.Attempt.AttemptNo, seeded.ID)
		}
		if claim.Attempt.WorkerID != "test-worker" {
			t.Errorf("attempt worker = %q, want cfg.WorkerID", claim.Attempt.WorkerID)
		}
		var attempts []model.TaskAttempt
		f.db.Where("task_id = ?", seeded.ID).Find(&attempts)
		if len(attempts) != 1 {
			t.Fatalf("attempt rows = %d, want 1 (created in the claim transaction)", len(attempts))
		}
		types := eventTypes(eventsOf(t, f.db, seeded.ID))
		if len(types) != 1 || types[0] != TaskEventClaimed {
			t.Errorf("events after claim = %v, want [claimed]", types)
		}

		// A second claim on the now-running row finds nothing.
		if _, ok, err := f.eng.ClaimNext(context.Background()); err != nil || ok {
			t.Errorf("second ClaimNext = (%v, %v), want no claim — the row is running", ok, err)
		}
	})
}

// T-8 — TestLeaseLowerBound: the lease must cover the attempt —
// lease = now + max(LeaseSeconds, CallTimeoutSeconds) + ReaperGrace. An
// earlier expiry lets the reaper requeue while execution is still in flight,
// opening double execution.
func TestLeaseLowerBound(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	cfg := Config{LeaseSeconds: 60, ReaperGrace: 5 * time.Second}

	if got := leaseLowerBound(cfg, 30, now); !got.Equal(now.Add(65 * time.Second)) {
		t.Errorf("lease(60, timeout 30) = %v, want now+65s", got)
	}
	if got := leaseLowerBound(cfg, 300, now); !got.Equal(now.Add(305 * time.Second)) {
		t.Errorf("lease(60, timeout 300) = %v, want now+305s — the call timeout raises the floor", got)
	}
	if got := leaseLowerBound(Config{LeaseSeconds: 60}, 0, now); !got.Equal(now.Add(60 * time.Second)) {
		t.Errorf("lease(60, no grace) = %v, want now+60s", got)
	}
}

// T26 — TestCompleteCommitsAttemptAndEventAtomically: state transition,
// attempt closure, and event append commit in ONE transaction (D5). With the
// event table dropped mid-flight, both Complete and Fail must roll back
// completely — no state change survives without its event.
func TestCompleteCommitsAttemptAndEventAtomically(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		f := newEngineFixture(t, testConfig())
		seedTask(t, f.db, nil)
		claim, ok, err := f.eng.ClaimNext(context.Background())
		if err != nil || !ok {
			t.Fatalf("claim: (%v, %v)", ok, err)
		}
		detail := contract.JSONMap{"exit": "0"}
		if err := f.eng.Complete(context.Background(), claim, detail); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		got := reloadTask(t, f.db, claim.Task.ID)
		if got.Status != TaskStatusSucceeded {
			t.Errorf("status = %q, want succeeded", got.Status)
		}
		if got.FinishedAt == nil {
			t.Error("finished_at is nil")
		}
		if got.Version != claim.Task.Version+1 {
			t.Errorf("version = %d, want %d", got.Version, claim.Task.Version+1)
		}
		var attempt model.TaskAttempt
		if err := f.db.First(&attempt, claim.Attempt.ID).Error; err != nil {
			t.Fatalf("load attempt: %v", err)
		}
		if attempt.FinishedAt == nil {
			t.Error("attempt not closed on Complete")
		}
		types := eventTypes(eventsOf(t, f.db, claim.Task.ID))
		want := []string{TaskEventClaimed, TaskEventSucceeded}
		if len(types) != len(want) {
			t.Fatalf("events = %v, want %v", types, want)
		}
		for i := range want {
			if types[i] != want[i] {
				t.Fatalf("events = %v, want %v", types, want)
			}
		}
		last := eventsOf(t, f.db, claim.Task.ID)[1]
		if last.AttemptNo != claim.Attempt.AttemptNo {
			t.Errorf("succeeded event attempt_no = %d, want %d", last.AttemptNo, claim.Attempt.AttemptNo)
		}
	})

	t.Run("event failure rolls the whole terminal commit back", func(t *testing.T) {
		f := newEngineFixture(t, testConfig())
		seedTask(t, f.db, nil)
		claim, ok, err := f.eng.ClaimNext(context.Background())
		if err != nil || !ok {
			t.Fatalf("claim: (%v, %v)", ok, err)
		}
		if err := f.db.Migrator().DropTable(&model.TaskEvent{}); err != nil {
			t.Fatalf("drop task_event: %v", err)
		}

		if err := f.eng.Complete(context.Background(), claim, nil); err == nil {
			t.Fatal("Complete succeeded with the event table missing — atomicity not exercised")
		}
		got := reloadTask(t, f.db, claim.Task.ID)
		if got.Status != TaskStatusRunning || got.FinishedAt != nil {
			t.Fatalf("after failed Complete: status=%q finished=%v — the state change must roll back with the event (D5)", got.Status, got.FinishedAt)
		}
		var attempt model.TaskAttempt
		f.db.First(&attempt, claim.Attempt.ID)
		if attempt.FinishedAt != nil {
			t.Error("attempt closure must roll back with the event too")
		}

		// Fail hits the same all-or-nothing contract.
		if err := f.eng.Fail(context.Background(), claim, ErrorCodeExecutorError, "boom"); err == nil {
			t.Fatal("Fail succeeded with the event table missing")
		}
		got = reloadTask(t, f.db, claim.Task.ID)
		if got.Status != TaskStatusRunning {
			t.Fatalf("after failed Fail: status=%q, want still running (rolled back)", got.Status)
		}
	})
}

// T27 — TestVersionCASRejectsStaleWrite: writes carry WHERE version=? — a
// claim holder whose snapshot went stale (reaper requeue, another writer)
// cannot complete, fail, or requeue the task (D4).
func TestVersionCASRejectsStaleWrite(t *testing.T) {
	bumpVersion := func(t *testing.T, f *engineFixture, id uint) {
		t.Helper()
		if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", id).
			Update("version", gorm.Expr("version + 5")).Error; err != nil {
			t.Fatalf("bump version: %v", err)
		}
	}

	t.Run("stale Complete", func(t *testing.T) {
		f := newEngineFixture(t, testConfig())
		seedTask(t, f.db, nil)
		claim, ok, err := f.eng.ClaimNext(context.Background())
		if err != nil || !ok {
			t.Fatalf("claim: (%v, %v)", ok, err)
		}
		bumpVersion(t, f, claim.Task.ID)
		if err := f.eng.Complete(context.Background(), claim, nil); !errors.Is(err, ErrStaleWrite) {
			t.Fatalf("stale Complete err = %v, want ErrStaleWrite", err)
		}
		if got := reloadTask(t, f.db, claim.Task.ID); got.Status != TaskStatusRunning {
			t.Errorf("status = %q after stale Complete, want still running", got.Status)
		}
	})

	t.Run("stale terminal Fail", func(t *testing.T) {
		f := newEngineFixture(t, testConfig())
		seedTask(t, f.db, func(task *model.ProviderTask) { task.MaxAttempts = 1 })
		claim, ok, err := f.eng.ClaimNext(context.Background())
		if err != nil || !ok {
			t.Fatalf("claim: (%v, %v)", ok, err)
		}
		bumpVersion(t, f, claim.Task.ID)
		if err := f.eng.Fail(context.Background(), claim, ErrorCodeExecutorError, "boom"); !errors.Is(err, ErrStaleWrite) {
			t.Fatalf("stale Fail err = %v, want ErrStaleWrite", err)
		}
		if got := reloadTask(t, f.db, claim.Task.ID); got.Status != TaskStatusRunning {
			t.Errorf("status = %q after stale Fail, want still running", got.Status)
		}
	})

	t.Run("stale retry Fail", func(t *testing.T) {
		f := newEngineFixture(t, testConfig())
		seedTask(t, f.db, nil)
		claim, ok, err := f.eng.ClaimNext(context.Background())
		if err != nil || !ok {
			t.Fatalf("claim: (%v, %v)", ok, err)
		}
		bumpVersion(t, f, claim.Task.ID)
		if err := f.eng.Fail(context.Background(), claim, ErrorCodeExecutorError, "boom"); !errors.Is(err, ErrStaleWrite) {
			t.Fatalf("stale retry Fail err = %v, want ErrStaleWrite", err)
		}
		if got := reloadTask(t, f.db, claim.Task.ID); got.Status != TaskStatusRunning {
			t.Errorf("status = %q after stale retry Fail, want still running — no requeue on a stale snapshot", got.Status)
		}
	})
}

// T-9 — TestFailRequeuesInSingleTransaction: with attempts remaining, Fail
// returns the task to queued (NOT failed) — retry decision, backoff, attempt
// closure and the attempt_failed event commit in ONE transaction. Exhaustion
// lands on the failed terminal.
func TestFailRequeuesInSingleTransaction(t *testing.T) {
	t.Run("retry remains", func(t *testing.T) {
		f := newEngineFixture(t, testConfig())
		seedTask(t, f.db, nil)
		claim, ok, err := f.eng.ClaimNext(context.Background())
		if err != nil || !ok {
			t.Fatalf("claim: (%v, %v)", ok, err)
		}
		if err := f.eng.Fail(context.Background(), claim, ErrorCodeExecutorError, "attempt exploded"); err != nil {
			t.Fatalf("Fail: %v", err)
		}
		got := reloadTask(t, f.db, claim.Task.ID)
		if got.Status != TaskStatusQueued {
			t.Fatalf("status = %q, want queued — §13.5 Failed→Queued when attempts remain", got.Status)
		}
		if got.NextAttemptAt == nil || !got.NextAttemptAt.After(time.Now()) {
			t.Errorf("next_attempt_at = %v, want a future backoff (BackoffSeconds=1)", got.NextAttemptAt)
		}
		if got.ErrorCode != ErrorCodeExecutorError {
			t.Errorf("error_code = %q, want %q kept on the row", got.ErrorCode, ErrorCodeExecutorError)
		}
		var attempt model.TaskAttempt
		f.db.First(&attempt, claim.Attempt.ID)
		if attempt.FinishedAt == nil || attempt.ErrorCode != ErrorCodeExecutorError {
			t.Errorf("attempt not closed with its error: %+v", attempt)
		}
		types := eventTypes(eventsOf(t, f.db, claim.Task.ID))
		want := []string{TaskEventClaimed, TaskEventAttemptFailed}
		if len(types) != len(want) || types[0] != want[0] || types[1] != want[1] {
			t.Fatalf("events = %v, want %v", types, want)
		}

		// The retry claims again as attempt 2 (MaxAttempts=2 → last attempt).
		// The 1s backoff is simulated as elapsed — deterministically, no sleep.
		due := time.Now().Add(-time.Second)
		if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", claim.Task.ID).
			Update("next_attempt_at", due).Error; err != nil {
			t.Fatalf("age the backoff: %v", err)
		}
		second, ok, err := f.eng.ClaimNext(context.Background())
		if err != nil || !ok {
			t.Fatalf("second claim: (%v, %v)", ok, err)
		}
		if second.Task.ID != claim.Task.ID || second.Attempt.AttemptNo != 2 {
			t.Fatalf("second claim = (task %d, attempt %d), want (same task, attempt 2)", second.Task.ID, second.Attempt.AttemptNo)
		}

		// Attempts exhausted now — Fail lands on the failed terminal.
		if err := f.eng.Fail(context.Background(), second, ErrorCodeExecutorError, "boom again"); err != nil {
			t.Fatalf("exhausted Fail: %v", err)
		}
		final := reloadTask(t, f.db, claim.Task.ID)
		if final.Status != TaskStatusFailed || final.FinishedAt == nil {
			t.Fatalf("final = (%q, finished=%v), want (failed, finished)", final.Status, final.FinishedAt)
		}
		types = eventTypes(eventsOf(t, f.db, claim.Task.ID))
		if types[len(types)-1] != TaskEventFailed {
			t.Errorf("last event = %v, want failed", types[len(types)-1])
		}
	})

	t.Run("timed_out is terminal without retry", func(t *testing.T) {
		f := newEngineFixture(t, testConfig())
		seedTask(t, f.db, nil)
		claim, ok, err := f.eng.ClaimNext(context.Background())
		if err != nil || !ok {
			t.Fatalf("claim: (%v, %v)", ok, err)
		}
		if err := f.eng.Fail(context.Background(), claim, ErrorCodeTimedOut, "call exceeded CallTimeoutSeconds"); err != nil {
			t.Fatalf("Fail(timed_out): %v", err)
		}
		got := reloadTask(t, f.db, claim.Task.ID)
		if got.Status != TaskStatusTimedOut {
			t.Fatalf("status = %q, want timed_out (engine-derived terminal, §3.9)", got.Status)
		}
		types := eventTypes(eventsOf(t, f.db, claim.Task.ID))
		if types[len(types)-1] != TaskEventTimedOut {
			t.Errorf("last event = %v, want timed_out", types[len(types)-1])
		}
	})
}

// ---------------------------------------------------------------------------
// R4 — 실행 회로(T-7/T-8)·비동기 폴·리퍼(§13.2)·루프: fake 경유 종단.
// ---------------------------------------------------------------------------

// executorOnlyDouble is a local executor-without-poller double for the
// capability_not_served path (the fake itself implements everything).
type executorOnlyDouble struct{}

func (executorOnlyDouble) Descriptor() contract.ProviderTypeDescriptor {
	return contract.ProviderTypeDescriptor{
		Type:            "aliyun",
		AdapterVersion:  "1",
		ProtocolVersion: "1",
		ConfigSchema:    func() contract.ConfigSpec { return contract.ConfigSpec{} },
	}
}
func (executorOnlyDouble) Validate(_ context.Context, _ contract.ConnectionView) error { return nil }
func (executorOnlyDouble) Health(_ context.Context, _ contract.ConnectionView) contract.HealthResult {
	return contract.HealthResult{Healthy: true}
}
func (executorOnlyDouble) Close() error { return nil }
func (executorOnlyDouble) Execute(_ context.Context, _ contract.OperationRequest) (contract.OperationHandle, error) {
	return contract.OperationHandle{}, nil
}

// seedChain creates the T-7 execution chain: connection(providerType) →
// context → infra_resource(uid→urn). Returns the URN the engine must assemble.
func seedChain(t *testing.T, db *gorm.DB, providerType, resourceUID string) string {
	t.Helper()
	conn := model.ProviderConnection{
		UID:          "conn-" + providerType,
		ProviderType: providerType,
		Name:         "conn-" + providerType,
		Endpoint:     "https://" + providerType + ".invalid",
		ConfigJSON:   contract.JSONMap{"region": "test"},
	}
	if err := db.Create(&conn).Error; err != nil {
		t.Fatalf("seed connection: %v", err)
	}
	ctx := model.ProviderContext{
		UID:          "ctx-" + providerType,
		ConnectionID: conn.ID,
		Kind:         "cluster",
	}
	if err := db.Create(&ctx).Error; err != nil {
		t.Fatalf("seed context: %v", err)
	}
	urn := "urn:fake:workload:" + resourceUID
	res := model.InfraResource{
		UID:         resourceUID,
		ContextID:   ctx.ID,
		Kind:        "orchestration.workload",
		ExternalURN: urn,
	}
	if err := db.Create(&res).Error; err != nil {
		t.Fatalf("seed resource: %v", err)
	}
	return urn
}

// fakeFixture wires the engine against the fake adapter (§3.8) — the Phase 1
// execution circuit — and seeds one resource chain.
type fakeFixture struct {
	*engineFixture
	fake *fake.Adapter
	urn  string
}

func newFakeFixture(t *testing.T, cfg Config) *fakeFixture {
	t.Helper()
	db := newTestDB(t)
	reg := registry.New()
	if err := fake.Register(reg); err != nil {
		t.Fatalf("fake.Register: %v", err)
	}
	if err := reg.RegisterOperation(testOperation("fake.workload.restart")); err != nil {
		t.Fatalf("seed operation: %v", err)
	}
	base := &engineFixture{db: db, reg: reg, eng: NewEngine(db, reg, cfg)}
	_, adapter, _ := reg.ProviderType("fake")
	fa, _ := adapter.(*fake.Adapter)
	return &fakeFixture{
		engineFixture: base,
		fake:          fa,
		urn:           seedChain(t, db, "fake", "res-uid-1"),
	}
}

// T-7 — RunOnce의 동기 경로 종단: 클레임 → 실행 체인 조인 → UID→URN 조립 →
// fake Execute(널 핸들) → succeeded 종단. 이벤트 시퀀스 created→claimed→succeeded.
func TestRunOnceExecutesThroughFakeChain(t *testing.T) {
	f := newFakeFixture(t, testConfig())
	ctx := context.Background()

	task, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	processed, err := f.eng.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !processed {
		t.Fatal("RunOnce processed = false, want true")
	}
	got := reloadTask(t, f.db, task.ID)
	if got.Status != TaskStatusSucceeded {
		t.Fatalf("status = %q, want succeeded", got.Status)
	}
	if f.fake.ExecCount() != 1 {
		t.Errorf("fake ExecCount = %d, want exactly 1 (single execution)", f.fake.ExecCount())
	}
	if got := f.fake.LastRequest().ResourceURN; got != f.urn {
		t.Errorf("OperationRequest.ResourceURN = %q, want %q — the engine owns UID→URN (T-7)", got, f.urn)
	}
	if got := f.fake.LastRequest().OperationName; got != "fake.workload.restart" {
		t.Errorf("OperationRequest.OperationName = %q, want the task's operation", got)
	}
	types := eventTypes(eventsOf(t, f.db, task.ID))
	want := []string{TaskEventCreated, TaskEventClaimed, TaskEventSucceeded}
	if len(types) != len(want) {
		t.Fatalf("event sequence = %v, want %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("event sequence = %v, want %v", types, want)
		}
	}
}

// §3.6/T-8 비동기 이중 모드(§14.3 UPID): Execute가 핸들을 반환하면 같은 사이클에
// Poll 1회, Running이면 태스크 running 유지 — 이후 매 폴 사이클에서 Poll하며
// Poll은 새 attempt를 만들지 않는다.
func TestRunOnceAsyncPollPath(t *testing.T) {
	f := newFakeFixture(t, testConfig())
	ctx := context.Background()

	task, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
		Payload:       contract.JSONMap{"async": true, "polls": 2},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if _, err := f.eng.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce 1: %v", err)
	}
	got := reloadTask(t, f.db, task.ID)
	if got.Status != TaskStatusRunning {
		t.Fatalf("after cycle 1 status = %q, want running (poll 1 of 2 → running)", got.Status)
	}
	if got.AttemptCount != 1 {
		t.Fatalf("attempt_count after cycle 1 = %d, want 1 — polling never creates attempts", got.AttemptCount)
	}

	if _, err := f.eng.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce 2: %v", err)
	}
	got = reloadTask(t, f.db, task.ID)
	if got.Status != TaskStatusSucceeded {
		t.Fatalf("after cycle 2 status = %q, want succeeded (poll 2 of 2)", got.Status)
	}
	if got.AttemptCount != 1 {
		t.Fatalf("attempt_count after async completion = %d, want 1 — one execution, polls only (§3.6)", got.AttemptCount)
	}
	if f.fake.ExecCount() != 1 {
		t.Errorf("fake ExecCount = %d, want 1 — the async path executes once", f.fake.ExecCount())
	}
}

// fake failAttempt 재시도 회로: 1회 실패 → queued 복귀(attempt_failed) →
// 백오프 경과 후 2회째 성공.
func TestRunOnceRetriesThroughFake(t *testing.T) {
	f := newFakeFixture(t, testConfig())
	ctx := context.Background()

	task, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
		Payload:       contract.JSONMap{"failAttempt": 1},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if _, err := f.eng.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce 1: %v", err)
	}
	got := reloadTask(t, f.db, task.ID)
	if got.Status != TaskStatusQueued {
		t.Fatalf("after failed attempt status = %q, want queued (retry)", got.Status)
	}
	types := eventTypes(eventsOf(t, f.db, task.ID))
	if types[len(types)-1] != TaskEventAttemptFailed {
		t.Fatalf("last event = %v, want attempt_failed", types[len(types)-1])
	}

	// 백오프(1s) 경과 시뮬레이션 — 결정적.
	due := time.Now().Add(-time.Second)
	if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", task.ID).
		Update("next_attempt_at", due).Error; err != nil {
		t.Fatalf("age backoff: %v", err)
	}
	if _, err := f.eng.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce 2: %v", err)
	}
	got = reloadTask(t, f.db, task.ID)
	if got.Status != TaskStatusSucceeded {
		t.Fatalf("after retry status = %q, want succeeded", got.Status)
	}
	if f.fake.ExecCount() != 2 {
		t.Errorf("fake ExecCount = %d, want 2 (one failure + one success)", f.fake.ExecCount())
	}
}

// T-7 하드 에러: 체인 조인 누락(리소스 없음·soft-delete 제외 포함)은
// unknown_resource 종단 — 재시도 없음(정의상 무의미).
func TestRunOnceUnknownResourceIsTerminalNoRetry(t *testing.T) {
	f := newFakeFixture(t, testConfig())
	ctx := context.Background()

	task, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-never-seeded",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if _, err := f.eng.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	got := reloadTask(t, f.db, task.ID)
	if got.Status != TaskStatusFailed {
		t.Fatalf("status = %q, want failed (unknown resource is a hard terminal)", got.Status)
	}
	if got.ErrorCode != ErrorCodeUnknownResource {
		t.Errorf("error_code = %q, want %q", got.ErrorCode, ErrorCodeUnknownResource)
	}
	if got.AttemptCount != 1 {
		t.Errorf("attempt_count = %d, want 1 — no retry on an unknown resource (T-7)", got.AttemptCount)
	}

	// soft-delete 리소스도 기본 스코프에서 제외되어 누락과 동일하다.
	var res model.InfraResource
	f.db.Where("uid = ?", "res-uid-1").First(&res)
	deleted := time.Now()
	if err := f.db.Model(&res).Update("deleted_at", deleted).Error; err != nil {
		t.Fatalf("soft-delete resource: %v", err)
	}
	task2, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
	})
	if err != nil {
		t.Fatalf("Submit 2: %v", err)
	}
	if _, err := f.eng.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce 2: %v", err)
	}
	got2 := reloadTask(t, f.db, task2.ID)
	if got2.Status != TaskStatusFailed || got2.ErrorCode != ErrorCodeUnknownResource {
		t.Fatalf("soft-deleted resource: (%q, %q), want (failed, unknown_resource)", got2.Status, got2.ErrorCode)
	}
}

// 레지스트리 계약 위배 경로: 어댑터가 실행 인터페이스는 갖추고도 필요
// capability를 선언하지 않으면 capability_not_served 종단.
func TestRunOnceCapabilityNotServedIsTerminal(t *testing.T) {
	f := newFakeFixture(t, testConfig())
	if err := f.reg.RegisterProviderType(executorOnlyDouble{}.Descriptor(), executorOnlyDouble{}); err != nil {
		t.Fatalf("register executor double: %v", err)
	}
	seedChain(t, f.db, "aliyun", "res-aliyun-1")
	ctx := context.Background()

	task, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-aliyun-1",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if _, err := f.eng.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	got := reloadTask(t, f.db, task.ID)
	if got.Status != TaskStatusFailed || got.ErrorCode != ErrorCodeCapabilityNotServed {
		t.Fatalf("capability mismatch: (%q, %q), want (failed, capability_not_served)", got.Status, got.ErrorCode)
	}
}

// §13.4 — 클레임 경계 취소: CancelRequested 플래그가 지속되고 다음 클레임
// 경계에서 종단 cancelled로 반영된다(T38 전주).
func TestClaimBoundaryCancelsFlaggedTask(t *testing.T) {
	f := newEngineFixture(t, testConfig())
	seeded := seedTask(t, f.db, func(task *model.ProviderTask) { task.CancelRequested = true })

	claim, ok, err := f.eng.ClaimNext(context.Background())
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if ok || claim != nil {
		t.Fatal("a cancel-requested task must not be handed to an executor")
	}
	got := reloadTask(t, f.db, seeded.ID)
	if got.Status != TaskStatusCancelled {
		t.Fatalf("status = %q, want cancelled at the claim boundary (§13.4)", got.Status)
	}
	if got.FinishedAt == nil {
		t.Error("cancelled task has no finished_at")
	}
	types := eventTypes(eventsOf(t, f.db, seeded.ID))
	if len(types) != 2 || types[0] != TaskEventClaimed || types[1] != TaskEventCancelled {
		t.Errorf("events = %v, want [claimed cancelled]", types)
	}
}

// §13.2 리퍼 — ReapOnce: 만료 리스 재큐(시도 잔여)·재큐 이벤트·미완 attempt
// 종결, 소진 시 failed 'lease_expired' 종단. 크래시의 스키마적 본질(클레임 후
// 커밋 없는 running 행)을 리스 만료 조작으로 결정적으로 재현한다.
func TestReapOnceRequeuesAndExhausts(t *testing.T) {
	t.Run("requeue when attempts remain", func(t *testing.T) {
		f := newEngineFixture(t, testConfig())
		seeded := seedTask(t, f.db, nil)
		if _, ok, err := f.eng.ClaimNext(context.Background()); err != nil || !ok {
			t.Fatalf("claim: (%v, %v)", ok, err)
		}
		// 크래시: 클레임 이후 아무 커밋 없이 소유자 소실 — 리스를 이미 만료+grace 경과로 조작.
		expired := time.Now().Add(-2 * time.Hour)
		if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", seeded.ID).
			Update("lease_expires_at", expired).Error; err != nil {
			t.Fatalf("expire lease: %v", err)
		}

		requeued, err := f.eng.ReapOnce(context.Background())
		if err != nil {
			t.Fatalf("ReapOnce: %v", err)
		}
		if requeued != 1 {
			t.Fatalf("requeued = %d, want 1", requeued)
		}
		got := reloadTask(t, f.db, seeded.ID)
		if got.Status != TaskStatusQueued {
			t.Fatalf("status = %q, want queued (reaper re-queue, §13.2)", got.Status)
		}
		if got.NextAttemptAt == nil || got.NextAttemptAt.After(time.Now()) {
			t.Errorf("next_attempt_at = %v, want due immediately (reaper sets NOW())", got.NextAttemptAt)
		}
		types := eventTypes(eventsOf(t, f.db, seeded.ID))
		if types[len(types)-1] != TaskEventReaperRequeued {
			t.Fatalf("last event = %v, want reaper_requeued (§13.2 verbatim)", types[len(types)-1])
		}
		// 유실 태스크 0 — 재큐된 태스크는 다시 클레임 가능하고 attempt_no는 이어진다.
		due := time.Now().Add(-time.Second)
		f.db.Model(&model.ProviderTask{}).Where("id = ?", seeded.ID).Update("next_attempt_at", due)
		second, ok, err := f.eng.ClaimNext(context.Background())
		if err != nil || !ok {
			t.Fatalf("post-reap claim: (%v, %v)", ok, err)
		}
		if second.Attempt.AttemptNo != 2 {
			t.Errorf("post-reap attempt_no = %d, want 2 — attempts continue after recovery", second.Attempt.AttemptNo)
		}
		var openAttempts int64
		f.db.Model(&model.TaskAttempt{}).Where("task_id = ? AND finished_at IS NULL", seeded.ID).Count(&openAttempts)
		if openAttempts != 1 {
			t.Errorf("open attempts after re-claim = %d, want exactly the new one (reaper closed the crashed attempt)", openAttempts)
		}
	})

	t.Run("exhausted attempts fail with lease_expired", func(t *testing.T) {
		f := newEngineFixture(t, testConfig())
		seeded := seedTask(t, f.db, func(task *model.ProviderTask) { task.MaxAttempts = 1 })
		if _, ok, err := f.eng.ClaimNext(context.Background()); err != nil || !ok {
			t.Fatalf("claim: (%v, %v)", ok, err)
		}
		expired := time.Now().Add(-2 * time.Hour)
		if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", seeded.ID).
			Update("lease_expires_at", expired).Error; err != nil {
			t.Fatalf("expire lease: %v", err)
		}

		requeued, err := f.eng.ReapOnce(context.Background())
		if err != nil {
			t.Fatalf("ReapOnce: %v", err)
		}
		if requeued != 0 {
			t.Fatalf("requeued = %d, want 0 — attempts exhausted", requeued)
		}
		got := reloadTask(t, f.db, seeded.ID)
		if got.Status != TaskStatusFailed {
			t.Fatalf("status = %q, want failed", got.Status)
		}
		if got.ErrorCode != ErrorCodeLeaseExpired {
			t.Errorf("error_code = %q, want lease_expired (§13.2)", got.ErrorCode)
		}
		if got.FinishedAt == nil {
			t.Error("exhausted reap has no finished_at")
		}
	})

	t.Run("live leases are not reaped", func(t *testing.T) {
		f := newEngineFixture(t, testConfig())
		seedTask(t, f.db, nil)
		if _, ok, _ := f.eng.ClaimNext(context.Background()); !ok {
			t.Fatal("claim failed")
		}
		requeued, err := f.eng.ReapOnce(context.Background())
		if err != nil {
			t.Fatalf("ReapOnce: %v", err)
		}
		if requeued != 0 {
			t.Fatalf("requeued = %d on a live lease, want 0", requeued)
		}
	})
}

// §3.5 — ConnectionView.Material: 바인딩 없는 연결(fake/Phase 1 경로)은
// Material이 nil로 흐르고, 바인딩이 있으면 브로커가 해석한 operations
// 자재가 채워진다.
func TestConnectionViewMaterial(t *testing.T) {
	t.Run("no binding leaves Material nil", func(t *testing.T) {
		f := newFakeFixture(t, testConfig())
		chain, err := f.eng.resolveExecutionChain(context.Background(), "res-uid-1")
		if err != nil {
			t.Fatalf("resolveExecutionChain: %v", err)
		}
		view, err := f.eng.connectionView(context.Background(), chain)
		if err != nil {
			t.Fatalf("connectionView: %v", err)
		}
		if view.Material != nil {
			t.Errorf("Material = %v without any binding, want nil (§3.5 fake path)", view.Material)
		}
		if view.ProviderType != "fake" || view.Endpoint != "https://fake.invalid" {
			t.Errorf("view = (%q, %q), want the joined connection fields", view.ProviderType, view.Endpoint)
		}
		if view.Config["region"] != "test" {
			t.Errorf("Config = %v, want the deserialized connection config", view.Config)
		}
	})

	t.Run("binding resolves operations material", func(t *testing.T) {
		testutil.PinSecretKeys(t)
		f := newFakeFixture(t, testConfig())
		chain, err := f.eng.resolveExecutionChain(context.Background(), "res-uid-1")
		if err != nil {
			t.Fatalf("resolveExecutionChain: %v", err)
		}
		sealed, err := util.EncryptSecretV2("kubernetes-rw-token")
		if err != nil {
			t.Fatalf("EncryptSecretV2: %v", err)
		}
		ref := model.SecretRef{UID: "sec-1", Ciphertext: sealed}
		if err := f.db.Create(&ref).Error; err != nil {
			t.Fatalf("seed secret_ref: %v", err)
		}
		var conn model.ProviderConnection
		if err := f.db.Where("uid = ?", "conn-fake").First(&conn).Error; err != nil {
			t.Fatalf("load connection: %v", err)
		}
		binding := model.ProviderCredentialBinding{
			ProviderConnectionID: conn.ID,
			Purpose:              contract.CredentialPurposeOperations,
			SecretRefID:          ref.ID,
		}
		if err := f.db.Create(&binding).Error; err != nil {
			t.Fatalf("seed binding: %v", err)
		}

		view, err := f.eng.connectionView(context.Background(), chain)
		if err != nil {
			t.Fatalf("connectionView: %v", err)
		}
		if view.Material[contract.CredentialPurposeOperations] != "kubernetes-rw-token" {
			t.Errorf("Material[operations] = %q, want the decrypted one-time view", view.Material[contract.CredentialPurposeOperations])
		}
	})
}

// Start/Stop 스모크 — 루프 배선이 실제로 사이클을 돈다(종단 도달 대기)·
// Stop이 반환한다(T39의 누수 계측은 PR 19이 소유).
func TestStartStopRunsCycles(t *testing.T) {
	f := newFakeFixture(t, Config{
		WorkerID:     "loop-worker",
		PollInterval: 5 * time.Millisecond,
		LeaseSeconds: 60,
		ReaperGrace:  time.Second,
	})
	task, _, err := f.eng.Submit(context.Background(), SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.eng.Start(ctx)

	deadline := time.Now().Add(5 * time.Second)
	for {
		got := reloadTask(t, f.db, task.ID)
		if got.Status == TaskStatusSucceeded {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("poller loop never completed the task — status %q", got.Status)
		}
		time.Sleep(2 * time.Millisecond)
	}

	f.eng.Stop() // must return — no deadlock, no panic
	f.eng.Stop() // idempotent
}

// 어댑터 보고 실패(§3.9 OperationStateFailed)의 재시도 경로: asyncFail 폴이
// 실패를 보고하면 operation_failed로 Fail → 재시도 → 소진 시 failed 종단.
func TestRunOnceAsyncPollFailureRetries(t *testing.T) {
	f := newFakeFixture(t, testConfig())
	ctx := context.Background()

	task, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
		Payload:       contract.JSONMap{"async": true, "asyncFail": true},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if _, err := f.eng.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce 1: %v", err)
	}
	got := reloadTask(t, f.db, task.ID)
	if got.Status != TaskStatusQueued {
		t.Fatalf("after failed async poll status = %q, want queued (operation_failed retry)", got.Status)
	}
	if got.ErrorCode != ErrorCodeOperationFailed {
		t.Errorf("error_code = %q, want operation_failed (adapter-reported)", got.ErrorCode)
	}
	types := eventTypes(eventsOf(t, f.db, task.ID))
	if types[len(types)-1] != TaskEventAttemptFailed {
		t.Fatalf("last event = %v, want attempt_failed", types[len(types)-1])
	}

	// 백오프 경과 후 재시도 — 다시 실패하고 이번엔 소진(MaxAttempts=2).
	due := time.Now().Add(-time.Second)
	if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", task.ID).
		Update("next_attempt_at", due).Error; err != nil {
		t.Fatalf("age backoff: %v", err)
	}
	if _, err := f.eng.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce 2: %v", err)
	}
	got = reloadTask(t, f.db, task.ID)
	if got.Status != TaskStatusFailed || got.ErrorCode != ErrorCodeOperationFailed {
		t.Fatalf("exhausted async failure: (%q, %q), want (failed, operation_failed)", got.Status, got.ErrorCode)
	}
}
