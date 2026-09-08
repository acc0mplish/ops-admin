// T-2/T-4 (metrics-plan §4) — the §18.2 M1 engine·worker delta suite
// (F duration·G failure·H retry·I queue depth·J lease expired). Each test
// drives one engine action and asserts the render delta: 행위 1회 → 해당
// 패밀리 라인 +1(gauge는 값 설정). The nil-Metrics no-op contract (T-4) has
// its own test — 계기 부재가 실행·재큐·종단을 바꾸지 않는다.
package tasks

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"ops-admin/backend/internal/infra/metrics"
	"ops-admin/backend/internal/infra/model"
)

// metricsOperation — newEngineFixture가 시드하는 유일 연산(fake.workload.restart,
// MaxAttempts 2 — testOperation). 모든 라벨 단얫이 이 이름으로 고정된다.
const metricsOperation = "fake.workload.restart"

// fixtureWithMetrics — 계기가 주입된 픽스처. 기존 newEngineFixture 재사용
// (엔진 단독 경로 — 어댑터·자격 불요: 계기 지점은 종단 커밋·재큐·리프·폴 틱).
func fixtureWithMetrics(t *testing.T) *engineFixture {
	t.Helper()
	f := newEngineFixture(t, testConfig())
	f.eng.Metrics = metrics.New()
	return f
}

// metricLineValue parses the trailing number of one labeled sample line —
// 없으면 테스트 실패(존재 단얫 겸용). kubernetes latency_test의 renderLineValue
// 관례와 동일 형태.
func metricLineValue(t *testing.T, render, key string) float64 {
	t.Helper()
	for _, line := range strings.Split(render, "\n") {
		if strings.HasPrefix(line, key+" ") {
			fields := strings.Fields(line)
			v, err := strconv.ParseFloat(fields[len(fields)-1], 64)
			if err != nil {
				t.Fatalf("parse %q: %v", line, err)
			}
			return v
		}
	}
	t.Fatalf("render missing %q:\n%s", key, render)
	return 0
}

func submitOne(t *testing.T, f *engineFixture, ctx context.Context, resourceUID string) model.ProviderTask {
	t.Helper()
	task, replayed, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: metricsOperation,
		ResourceUID:   resourceUID,
	})
	if err != nil || replayed {
		t.Fatalf("Submit: (replayed=%v, err=%v)", replayed, err)
	}
	return task
}

func claimOrDie(t *testing.T, f *engineFixture, ctx context.Context) *ClaimedTask {
	t.Helper()
	claim, ok, err := f.eng.ClaimNext(ctx)
	if err != nil || !ok {
		t.Fatalf("ClaimNext: (ok=%v, err=%v)", ok, err)
	}
	return claim
}

// F(+G 부정) — Complete 1회 → duration{operation,status="succeeded"}의
// _count·+Inf bucket이 1이고 failure 패밀리는 미등장(성공 종단은 failure 가산
// 밖 — 가정 A4).
func TestCompleteObservesTaskDurationDelta(t *testing.T) {
	f := fixtureWithMetrics(t)
	ctx := context.Background()
	task := submitOne(t, f, ctx, "res-mx-1")

	if err := f.eng.Complete(ctx, claimOrDie(t, f, ctx), nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got := reloadTask(t, f.db, task.ID).Status; got != TaskStatusSucceeded {
		t.Fatalf("status = %q, want succeeded — 계기가 종단을 바꾸면 안 된다(T-4)", got)
	}
	render := f.eng.Metrics.Render()
	if !strings.Contains(render, "# TYPE provider_task_duration_seconds histogram") {
		t.Errorf("render missing the duration TYPE header:\n%s", render)
	}
	key := `provider_task_duration_seconds_count{operation="` + metricsOperation + `",status="succeeded"}`
	if got := metricLineValue(t, render, key); got != 1 {
		t.Errorf("duration count = %v, want 1 after one Complete", got)
	}
	bucket := `provider_task_duration_seconds_bucket{operation="` + metricsOperation + `",status="succeeded",le="+Inf"}`
	if got := metricLineValue(t, render, bucket); got != 1 {
		t.Errorf("duration +Inf bucket = %v, want 1", got)
	}
	if strings.Contains(render, "provider_task_failures_total") {
		t.Errorf("success terminal must not feed the failure family:\n%s", render)
	}
}

// H→G — 재큐 1회 → retries_total 1(failure·duration 미등장), 소진 종단 →
// failures_total{operation,code} 1 + duration{...,status="failed"} 1(retries
// 불변). MaxAttempts 2 — 동일 코드 executor_error로 두 번 Fail한다.
func TestFailRequeueThenExhaustFiresRetryAndFailure(t *testing.T) {
	f := fixtureWithMetrics(t)
	ctx := context.Background()
	task := submitOne(t, f, ctx, "res-mx-2")

	if err := f.eng.Fail(ctx, claimOrDie(t, f, ctx), ErrorCodeExecutorError, "boom"); err != nil {
		t.Fatalf("Fail (requeue leg): %v", err)
	}
	if got := reloadTask(t, f.db, task.ID).Status; got != TaskStatusQueued {
		t.Fatalf("status = %q, want queued (requeue, §13.5 failed→queued)", got)
	}
	render := f.eng.Metrics.Render()
	if got := metricLineValue(t, render, `provider_task_retries_total{operation="`+metricsOperation+`"}`); got != 1 {
		t.Errorf("retries = %v, want 1 after one requeue", got)
	}
	if strings.Contains(render, "provider_task_failures_total") || strings.Contains(render, "provider_task_duration_seconds") {
		t.Errorf("a requeue is not a terminal — no failure/duration families:\n%s", render)
	}

	// 백오프(BackoffSeconds 1) 대기 대신 클록을 조작해 두 번째 시도를 즉시 due로.
	if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", task.ID).
		Update("next_attempt_at", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatalf("force next_attempt_at: %v", err)
	}
	claim := claimOrDie(t, f, ctx)
	if claim.Task.AttemptCount != 2 {
		t.Fatalf("attempt_count = %d, want 2 (소진 전제)", claim.Task.AttemptCount)
	}
	if err := f.eng.Fail(ctx, claim, ErrorCodeExecutorError, "boom again"); err != nil {
		t.Fatalf("Fail (exhaustion leg): %v", err)
	}
	if got := reloadTask(t, f.db, task.ID).Status; got != TaskStatusFailed {
		t.Fatalf("status = %q, want failed (attempts exhausted)", got)
	}
	render = f.eng.Metrics.Render()
	fkey := `provider_task_failures_total{operation="` + metricsOperation + `",code="executor_error"}`
	if got := metricLineValue(t, render, fkey); got != 1 {
		t.Errorf("failures{executor_error} = %v, want 1 (소진 종단 1회)", got)
	}
	dkey := `provider_task_duration_seconds_count{operation="` + metricsOperation + `",status="failed"}`
	if got := metricLineValue(t, render, dkey); got != 1 {
		t.Errorf("duration{failed} count = %v, want 1", got)
	}
	if got := metricLineValue(t, render, `provider_task_retries_total{operation="`+metricsOperation+`"}`); got != 1 {
		t.Errorf("retries = %v, want still 1 — terminal path adds no retry", got)
	}
}

// F(취소) — 경계 취소 종단 1회 → duration{operation,status="cancelled"} 1.
// failure 가산 밖(가정 A4 — 취소는 failed/timed_out이 아니다).
func TestCancelAtClaimBoundaryObservesCancelledDuration(t *testing.T) {
	f := fixtureWithMetrics(t)
	ctx := context.Background()
	task := submitOne(t, f, ctx, "res-mx-3")
	if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", task.ID).
		Update("cancel_requested", true).Error; err != nil {
		t.Fatalf("flag cancel_requested: %v", err)
	}

	if _, ok, err := f.eng.ClaimNext(ctx); ok || err != nil {
		t.Fatalf("ClaimNext on cancel-flagged task: (ok=%v, err=%v), want (false, nil)", ok, err)
	}
	if got := reloadTask(t, f.db, task.ID).Status; got != TaskStatusCancelled {
		t.Fatalf("status = %q, want cancelled (collapsed claim-boundary cancel)", got)
	}
	render := f.eng.Metrics.Render()
	key := `provider_task_duration_seconds_count{operation="` + metricsOperation + `",status="cancelled"}`
	if got := metricLineValue(t, render, key); got != 1 {
		t.Errorf("duration{cancelled} count = %v, want 1", got)
	}
	if strings.Contains(render, "provider_task_failures_total") {
		t.Errorf("cancel terminal must not feed the failure family:\n%s", render)
	}
}

// J(+소진 F/G) — CAS 승윈 리프 1회(재큐 갈래) → lease_expired_total 1(failure·
// duration 미등장), 소진 갈래 1회 → lease_expired_total 2 + failures{lease_expired}
// 1 + duration{failed} 1.
func TestReaperObservesLeaseExpiredAndExhaustedFamilies(t *testing.T) {
	f := fixtureWithMetrics(t)
	ctx := context.Background()
	task := submitOne(t, f, ctx, "res-mx-4")

	claimOrDie(t, f, ctx) // crash: 소유자가 종단 없이 사라진다
	expireLeaseForcesReap(t, f, task.ID)
	if requeued, err := f.eng.ReapOnce(ctx); err != nil || requeued != 1 {
		t.Fatalf("ReapOnce: (requeued=%d, err=%v), want (1, nil)", requeued, err)
	}
	render := f.eng.Metrics.Render()
	if got := metricLineValue(t, render, "worker_lease_expired_total"); got != 1 {
		t.Errorf("lease_expired = %v, want 1 after one CAS-won reap", got)
	}
	if strings.Contains(render, "provider_task_failures_total") || strings.Contains(render, "provider_task_duration_seconds") {
		t.Errorf("reaper requeue is not a terminal — no failure/duration families:\n%s", render)
	}

	// 두 번째 크래시 — 시도 소진(MaxAttempts 2)으로 failed 종단에 착지한다.
	claimOrDie(t, f, ctx)
	expireLeaseForcesReap(t, f, task.ID)
	if requeued, err := f.eng.ReapOnce(ctx); err != nil || requeued != 0 {
		t.Fatalf("second ReapOnce: (requeued=%d, err=%v), want (0, nil)", requeued, err)
	}
	if got := reloadTask(t, f.db, task.ID).Status; got != TaskStatusFailed {
		t.Fatalf("status = %q, want failed (lease_expired exhaustion)", got)
	}
	render = f.eng.Metrics.Render()
	if got := metricLineValue(t, render, "worker_lease_expired_total"); got != 2 {
		t.Errorf("lease_expired = %v, want 2 (재큐·소진 양 갈래)", got)
	}
	if got := metricLineValue(t, render, `provider_task_failures_total{operation="`+metricsOperation+`",code="lease_expired"}`); got != 1 {
		t.Errorf("failures{lease_expired} = %v, want 1", got)
	}
	dkey := `provider_task_duration_seconds_count{operation="` + metricsOperation + `",status="failed"}`
	if got := metricLineValue(t, render, dkey); got != 1 {
		t.Errorf("duration{failed} count = %v, want 1 (소진 종단은 터미널 커밋)", got)
	}
}

// I — 폴 틱 1회 → worker_queue_depth gauge가 queued COUNT로 설정된다(클레임 1건
// 소모 후 재관측 시 1로 내려간다 — 1회/주기 델타).
func TestQueueDepthGaugeFollowsQueueState(t *testing.T) {
	f := fixtureWithMetrics(t)
	ctx := context.Background()

	depth, err := f.eng.QueueDepth(ctx)
	if err != nil || depth != 0 {
		t.Fatalf("QueueDepth on empty queue: (depth=%d, err=%v), want (0, nil)", depth, err)
	}
	submitOne(t, f, ctx, "res-mx-5")
	submitOne(t, f, ctx, "res-mx-6")

	f.eng.observeQueueDepth(ctx)
	if got := metricLineValue(t, f.eng.Metrics.Render(), "worker_queue_depth"); got != 2 {
		t.Errorf("queue_depth = %v, want 2 (queued COUNT)", got)
	}
	claimOrDie(t, f, ctx) // 한 건이 running으로 이동 — queued 면에서 뺀다

	f.eng.observeQueueDepth(ctx)
	if got := metricLineValue(t, f.eng.Metrics.Render(), "worker_queue_depth"); got != 1 {
		t.Errorf("queue_depth = %v, want 1 after one claim", got)
	}
}

// T-4 — Metrics nil: 계기 없이 동일 경로(종단·재큐·소진·리프)가 기존 결과를
// 그대로 낸다. 9개 기존 테스트 파일 전체가 nil 엔진으로 통과하는 것이 1차
// 증명이고, 여기서는 종단 상태만 명시적으로 단얫한다.
func TestMetricsNilKeepsEngineBehavior(t *testing.T) {
	f := newEngineFixture(t, testConfig()) // Metrics 미주입
	if f.eng.Metrics != nil {
		t.Fatalf("fixture without injection must carry nil Metrics")
	}
	ctx := context.Background()

	task := submitOne(t, f, ctx, "res-mx-nil")
	if err := f.eng.Fail(ctx, claimOrDie(t, f, ctx), ErrorCodeExecutorError, "boom"); err != nil {
		t.Fatalf("Fail with nil Metrics: %v", err)
	}
	if got := reloadTask(t, f.db, task.ID).Status; got != TaskStatusQueued {
		t.Fatalf("requeue status = %q, want queued — nil Metrics가 재큐를 바꾸면 안 된다", got)
	}
	if err := f.db.Model(&model.ProviderTask{}).Where("id = ?", task.ID).
		Update("next_attempt_at", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatalf("force next_attempt_at: %v", err)
	}
	if err := f.eng.Complete(ctx, claimOrDie(t, f, ctx), nil); err != nil {
		t.Fatalf("Complete with nil Metrics: %v", err)
	}
	if got := reloadTask(t, f.db, task.ID).Status; got != TaskStatusSucceeded {
		t.Fatalf("terminal status = %q, want succeeded — nil Metrics가 종단을 바꾸면 안 된다", got)
	}

	crashed := submitOne(t, f, ctx, "res-mx-nil-2")
	claimOrDie(t, f, ctx)
	expireLeaseForcesReap(t, f, crashed.ID)
	if requeued, err := f.eng.ReapOnce(ctx); err != nil || requeued != 1 {
		t.Fatalf("ReapOnce with nil Metrics: (requeued=%d, err=%v), want (1, nil)", requeued, err)
	}
	if got := reloadTask(t, f.db, crashed.ID).Status; got != TaskStatusQueued {
		t.Fatalf("reaped status = %q, want queued — nil Metrics가 리프를 바꾸면 안 된다", got)
	}
}
