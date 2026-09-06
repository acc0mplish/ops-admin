// N11 (plan §3.1/§3.6/§6 — 판단 J12/J6, Phase A 임계경로 ①) — 자격 전달
// 회로. 엔진의 어댑터 조립 지점 2곳(executeClaimed의 Execute+첫 폴,
// pollAsyncAttempts의 재폴)이 엔진이 조립한 ConnectionView를
// OperationRequest.Connection / PollRequest.Connection으로 전달하는지를
// 기계 판정한다. 재폴 경로가 J12의 자격 공백이었다: 크래시 후 attempt 2는
// attempt.HandleRef만으로 handle을 재조립해 폴했으므로 자격을 스스로 확보할
// 수 없었다.
//
// handle-UID 정합(§3.1 "PollRequest.Connection.UID == Handle.ProviderRef에
// 인코딩된 connUID")의 ProviderRef 측 단얫은 connUID를 인코딩하는 k8s
// executor(N1, Phase B)와 함께 착지한다 — fake의 ProviderRef
// ("fake:upid:N")는 connUID를 담지 않는다. Phase A는 조립 단일성(Execute와
// Poll이 동일 연결 자격을 받는다·재폴이 첫 폴과 동일한 UID를 받는다)을
// 단얫한다.
package tasks

import (
	"context"
	"testing"
	"time"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/testutil"
	"ops-admin/backend/util"
)

// seedOperationsBinding arms conn-fake with an active operations-purpose
// binding (the J12(1) shape the backfill now creates) so connectionView
// resolves real broker material into Material["operations"].
func seedOperationsBinding(t *testing.T, f *fakeFixture, material string) {
	t.Helper()
	testutil.PinSecretKeys(t)
	sealed, err := util.EncryptSecretV2(material)
	if err != nil {
		t.Fatalf("EncryptSecretV2: %v", err)
	}
	ref := model.SecretRef{UID: "sec-ops", Ciphertext: sealed}
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
		Status:               "active",
	}
	if err := f.db.Create(&binding).Error; err != nil {
		t.Fatalf("seed operations binding: %v", err)
	}
}

// N-1 이행 — executeClaimed는 브로커 Resolve(operations)로 조립한 뷰를
// OperationRequest.Connection으로 전달한다(폐기하지 않는다). 바인딩이 없는
// 연결(§3.5 fake 경로)은 Material nil로 흐른다.
func TestCircuitExecuteReceivesConnection(t *testing.T) {
	t.Run("no binding: view delivered with nil Material", func(t *testing.T) {
		f := newFakeFixture(t, testConfig())
		ctx := context.Background()
		if _, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-uid-1",
		}); err != nil {
			t.Fatalf("Submit: %v", err)
		}
		if _, err := f.eng.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		conn := f.fake.LastRequest().Connection
		if conn.UID != "conn-fake" {
			t.Errorf("OperationRequest.Connection.UID = %q, want the execution chain's connection uid", conn.UID)
		}
		if conn.ProviderType != "fake" || conn.Endpoint != "https://fake.invalid" {
			t.Errorf("Connection = (%q, %q), want the joined connection fields", conn.ProviderType, conn.Endpoint)
		}
		if conn.Material != nil {
			t.Errorf("Connection.Material = %v without a binding, want nil (§3.5 fake path)", conn.Material)
		}
	})

	t.Run("operations binding resolves into Material", func(t *testing.T) {
		f := newFakeFixture(t, testConfig())
		seedOperationsBinding(t, f, "kubernetes-rw-token")
		ctx := context.Background()
		if _, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-uid-1",
		}); err != nil {
			t.Fatalf("Submit: %v", err)
		}
		if _, err := f.eng.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		conn := f.fake.LastRequest().Connection
		if conn.UID != "conn-fake" {
			t.Errorf("Connection.UID = %q, want conn-fake", conn.UID)
		}
		if got := conn.Material[contract.CredentialPurposeOperations]; got != "kubernetes-rw-token" {
			t.Errorf("Material[operations] = %q, want the broker-resolved material", got)
		}
	})
}

// 첫 폴(executeClaimed 동일 사이클 폴) — PollRequest{Handle, Connection}이
// 전달되고, Execute가 받은 것과 동일한 연결 자격이다(조립 단일성).
func TestCircuitFirstPollCredentials(t *testing.T) {
	f := newFakeFixture(t, testConfig())
	seedOperationsBinding(t, f, "kubernetes-rw-token")
	ctx := context.Background()
	task, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-uid-1",
		Payload:       contract.JSONMap{"async": true, "polls": 1},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if _, err := f.eng.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if got := reloadTask(t, f.db, task.ID).Status; got != TaskStatusSucceeded {
		t.Fatalf("status = %q, want succeeded on the first poll (polls=1)", got)
	}

	poll := f.fake.LastPoll()
	if poll.Handle.ProviderRef == "" {
		t.Fatal("first poll handle is empty — the engine must pass Execute's handle")
	}
	if poll.Connection.UID != "conn-fake" {
		t.Errorf("first poll Connection.UID = %q, want conn-fake (J12 — every poll carries credentials)", poll.Connection.UID)
	}
	if got := poll.Connection.Material[contract.CredentialPurposeOperations]; got != "kubernetes-rw-token" {
		t.Errorf("first poll Material[operations] = %q, want the broker-resolved material", got)
	}
	if uid := f.fake.LastRequest().Connection.UID; uid != poll.Connection.UID {
		t.Errorf("Execute saw connection %q but Poll saw %q — the two assembly sites diverged", uid, poll.Connection.UID)
	}
}

// 재폴(pollAsyncAttempts) — J12 자격 공백의 폐쇄: attempt.HandleRef만으로
// handle을 재조립하던 경로가 connectionView도 재조립해 전달한다. 크래시 후
// attempt 2의 재폴이 자격을 스스로 확보할 수 없던 구멍이 여기서 막힌다.
func TestCircuitRepollCredentials(t *testing.T) {
	f := newFakeFixture(t, testConfig())
	seedOperationsBinding(t, f, "kubernetes-rw-token")
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
	if got := reloadTask(t, f.db, task.ID).Status; got != TaskStatusRunning {
		t.Fatalf("after cycle 1 status = %q, want running (poll 1 of 2)", got)
	}
	first := f.fake.LastPoll()

	if _, err := f.eng.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce 2: %v", err)
	}
	if got := reloadTask(t, f.db, task.ID).Status; got != TaskStatusSucceeded {
		t.Fatalf("after cycle 2 status = %q, want succeeded (re-poll 2 of 2)", got)
	}
	repolled := f.fake.LastPoll()

	if repolled.Handle.ProviderRef != first.Handle.ProviderRef {
		t.Errorf("re-poll handle = %q, want the persisted attempt handle %q", repolled.Handle.ProviderRef, first.Handle.ProviderRef)
	}
	var attempt model.TaskAttempt
	if err := f.db.Where("task_id = ?", task.ID).Order("id DESC").First(&attempt).Error; err != nil {
		t.Fatalf("load attempt: %v", err)
	}
	if attempt.HandleRef != repolled.Handle.ProviderRef {
		t.Errorf("attempt.handle_ref = %q, want the polled handle %q (§3.6 — the handle persists on the attempt)", attempt.HandleRef, repolled.Handle.ProviderRef)
	}
	if repolled.Connection.UID != "conn-fake" {
		t.Errorf("re-poll Connection.UID = %q, want conn-fake — the re-poll path re-assembles credentials (J12)", repolled.Connection.UID)
	}
	if got := repolled.Connection.Material[contract.CredentialPurposeOperations]; got != "kubernetes-rw-token" {
		t.Errorf("re-poll Material[operations] = %q, want the broker-resolved material", got)
	}
}

// J6 — OnTaskTerminal 훅: Complete/Fail 종단 커밋 이후 공통 발화. 성공·실패
// 양계열, 재큐(비종단) 미발화, 훅 실패가 종단을 되돌리지 않음을 단얫한다.
func TestCircuitOnTaskTerminal(t *testing.T) {
	t.Run("succeeded series carries committed task and detail", func(t *testing.T) {
		f := newFakeFixture(t, testConfig())
		type call struct {
			uid    string
			status string
			detail contract.JSONMap
		}
		var calls []call
		f.eng.OnTaskTerminal = func(_ context.Context, task model.ProviderTask, status string, detail contract.JSONMap) error {
			calls = append(calls, call{uid: task.UID, status: status, detail: detail})
			if task.Status != status {
				t.Errorf("hook task.Status = %q, want the committed terminal %q", task.Status, status)
			}
			return nil
		}
		ctx := context.Background()
		task, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-uid-1",
			Payload:       contract.JSONMap{"async": true, "polls": 1},
		})
		if err != nil {
			t.Fatalf("Submit: %v", err)
		}
		if _, err := f.eng.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if len(calls) != 1 {
			t.Fatalf("hook calls = %d, want exactly 1", len(calls))
		}
		if calls[0].uid != task.UID {
			t.Errorf("hook uid = %q, want %q", calls[0].uid, task.UID)
		}
		if calls[0].status != TaskStatusSucceeded {
			t.Errorf("hook status = %q, want succeeded", calls[0].status)
		}
		if calls[0].detail == nil {
			t.Error("hook detail = nil, want the adapter's terminal OperationStatus.Detail (감사 종단행 인터페이스)")
		}
	})

	t.Run("failed series carries the terminal error code", func(t *testing.T) {
		f := newFakeFixture(t, testConfig())
		def := testOperation("fake.workload.once")
		def.RetryPolicy = contract.RetryPolicy{MaxAttempts: 1, BackoffSeconds: 1}
		if err := f.reg.RegisterOperation(def); err != nil {
			t.Fatalf("register single-attempt operation: %v", err)
		}
		var codes []string
		f.eng.OnTaskTerminal = func(_ context.Context, task model.ProviderTask, status string, _ contract.JSONMap) error {
			if status == TaskStatusFailed {
				codes = append(codes, task.ErrorCode)
			}
			return nil
		}
		ctx := context.Background()
		if _, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.once",
			ResourceUID:   "res-uid-1",
			Payload:       contract.JSONMap{"failAttempt": 1},
		}); err != nil {
			t.Fatalf("Submit: %v", err)
		}
		if _, err := f.eng.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if len(codes) != 1 || codes[0] != ErrorCodeExecutorError {
			t.Errorf("failed-series hook error codes = %v, want exactly [%s]", codes, ErrorCodeExecutorError)
		}
	})

	t.Run("retry requeue is not terminal — no hook", func(t *testing.T) {
		f := newFakeFixture(t, testConfig())
		hookCalls := 0
		f.eng.OnTaskTerminal = func(context.Context, model.ProviderTask, string, contract.JSONMap) error {
			hookCalls++
			return nil
		}
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
		if got := reloadTask(t, f.db, task.ID).Status; got != TaskStatusQueued {
			t.Fatalf("after cycle 1 status = %q, want queued (retry requeue, not terminal)", got)
		}
		if got := hookCalls; got != 0 {
			t.Fatalf("hook calls after a non-terminal requeue = %d, want 0 (the hook is terminal-only)", got)
		}
		// The requeue set next_attempt_at = now + BackoffSeconds(1s) — force the
		// retry due immediately (the crash suite's clock-manipulation stance:
		// no real sleeps in engine tests).
		due := time.Now().Add(-time.Second)
		if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", task.ID).
			Update("next_attempt_at", due).Error; err != nil {
			t.Fatalf("force retry due: %v", err)
		}
		if _, err := f.eng.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce 2: %v", err)
		}
		if got := hookCalls; got != 1 {
			t.Fatalf("hook calls after the retry succeeds = %d, want 1", got)
		}
	})

	t.Run("hook failure never rolls the terminal back", func(t *testing.T) {
		f := newFakeFixture(t, testConfig())
		f.eng.OnTaskTerminal = func(context.Context, model.ProviderTask, string, contract.JSONMap) error {
			return errHookBoom
		}
		ctx := context.Background()
		task, _, err := f.eng.Submit(ctx, SubmitInput{
			OperationName: "fake.workload.restart",
			ResourceUID:   "res-uid-1",
		})
		if err != nil {
			t.Fatalf("Submit: %v", err)
		}
		if _, err := f.eng.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce: %v — a hook failure must not surface as an engine error", err)
		}
		got := reloadTask(t, f.db, task.ID)
		if got.Status != TaskStatusSucceeded {
			t.Fatalf("status = %q, want succeeded — the terminal commit stands (J6 종단 불변)", got.Status)
		}
		found := false
		for _, ev := range eventsOf(t, f.db, task.ID) {
			if ev.Type == TaskEventObservationRefreshFailed {
				found = true
			}
		}
		if !found {
			t.Errorf("no %s event after a hook failure — the failure must be recorded (J6)", TaskEventObservationRefreshFailed)
		}
	})
}

// errHookBoom — the injected hook failure (deterministic, message-free).
var errHookBoom = errHookBoomError{}

type errHookBoomError struct{}

func (errHookBoomError) Error() string { return "hook: injected failure" }
