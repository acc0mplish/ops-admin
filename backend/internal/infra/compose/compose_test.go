// Compose wiring test — the assembly root registers the built-in adapters
// exactly once and hands every component the shared dependency set.
package compose_test

import (
	"context"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/adapter/kubernetes"
	"ops-admin/backend/internal/infra/compose"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/tasks"
	"ops-admin/backend/internal/testutil"
	"ops-admin/backend/util"
)

// TestBuildWiresStack — Build must yield a registry carrying both built-in
// adapters and a runner/broker wired onto the same db handle. A second Build
// on a fresh stack must not collide (registration is per-Registry — no
// global state, §11.1).
func TestBuildWiresStack(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)

	stack, err := compose.Build(db)
	if err != nil {
		t.Fatalf("compose.Build: %v", err)
	}
	names := stack.Registry.ProviderTypeNames()
	// 등록 수 단얫 갱신(보존 제약 6 명시적 예외 — 계획 r2 H3·§5 합류점): aliyun
	// (PR 27)·tencent(PR 28) 양 레인 합류 완료 — 4종이 최종 형상이다.
	if len(names) != 4 {
		t.Fatalf("registry provider types = %v, want [fake kubernetes aliyun tencent]", names)
	}
	got := map[string]bool{}
	for _, n := range names {
		got[n] = true
	}
	if !got["fake"] || !got["kubernetes"] || !got["aliyun"] || !got["tencent"] {
		t.Errorf("registry provider types = %v, want fake, kubernetes, aliyun and tencent", names)
	}
	if stack.Runner == nil || stack.Broker == nil || stack.Counters == nil {
		t.Fatal("stack left a component unwired")
	}
	// The shared counter set pre-registers the kubernetes provider row — the
	// gate ③ flat proof reads the {provider="kubernetes"} 0 line from the
	// report artifact's render.
	if want := `provider_rate_limit_total{provider="kubernetes"} 0`; !strings.Contains(stack.Counters.Render(), want) {
		t.Errorf("counters render missing %q:\n%s", want, stack.Counters.Render())
	}

	// A second assembly (fresh registry, same db) must succeed — explicit
	// composition, no global registration state.
	if _, err := compose.Build(db); err != nil {
		t.Errorf("second compose.Build: %v", err)
	}
}

// TestBuildRegistersTencentCapabilities — Phase 4 B(M2 tencent분): compose는
// tencent 어댑터(계획 §2 N6 — TC3 직구현, PR 28)와 그 읽기 capability 쌍을
// 등록한다(판정 J10 — inventory.full·compute.vm.read, ResourceKinds는
// compute.vm 1종). V5 실증: 두 capability 모두 §3.7 매핑 표가 Discoverer를
// 요구한다 — 등록이 받아졌다는 것은 *Adapter가 Discoverer 타입 단얫을
// 통과했다는 뜻이다(어댑터 자체의 4단얫 하네스는 adapter_test.go 소관).
func TestBuildRegistersTencentCapabilities(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)

	stack, err := compose.Build(db)
	if err != nil {
		t.Fatalf("compose.Build: %v", err)
	}

	caps := stack.Registry.Capabilities("tencent")
	got := map[string]contract.Capability{}
	for _, c := range caps {
		got[c.Name] = c
	}
	for _, want := range []string{"inventory.full", "compute.vm.read"} {
		c, ok := got[want]
		if !ok {
			t.Errorf("tencent capabilities = %v, missing %q", caps, want)
			continue
		}
		if !c.ReadOnly {
			t.Errorf("capability %q must be read-only (Phase 4는 읽기 전용 — §20)", want)
		}
		if len(c.ResourceKinds) != 1 || c.ResourceKinds[0] != "compute.vm" {
			t.Errorf("capability %q ResourceKinds = %v, want [compute.vm] (판정 J3 — vm family 한정)", want, c.ResourceKinds)
		}
	}
	if _, ok := got["orchestration.kubernetes.apply"]; ok {
		t.Error("tencent must not declare a mutating capability (읽기 전용 어댑터)")
	}

	// 공유 카운터에 tencent provider 행이 있다(게이트 ③ flat 증명 형식).
	if want := `provider_rate_limit_total{provider="tencent"} 0`; !strings.Contains(stack.Counters.Render(), want) {
		t.Errorf("counters render missing %q:\n%s", want, stack.Counters.Render())
	}
}

// TestBuildRegistersKubernetesRestart — Phase 3 B(M4/M5): compose는 kubernetes
// capability 3종(§10.1 어휘)과 restart opdef(§3.3)를 등록한다. V5 실증: apply
// capability 등록이 받아졌다는 것은 §3.7 매핑 표의 OperationExecutor 요구를
// *Adapter가 등록 시 타입 단얫으로 통과했다는 뜻이다.
func TestBuildRegistersKubernetesRestart(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)

	stack, err := compose.Build(db)
	if err != nil {
		t.Fatalf("compose.Build: %v", err)
	}

	def, ok := stack.Registry.Operation(kubernetes.RestartOperationName)
	if !ok {
		t.Fatalf("operation %q not registered", kubernetes.RestartOperationName)
	}
	// J4 — v1 권한 어휘 재사용(신규 권한 문자열 0 — 보존 제약 #2).
	if def.RequiredPermission != "assets:k8s:workload:restart" {
		t.Errorf("RequiredPermission = %q, want assets:k8s:workload:restart", def.RequiredPermission)
	}
	if def.RequiredCapability != "orchestration.kubernetes.apply" {
		t.Errorf("RequiredCapability = %q, want orchestration.kubernetes.apply", def.RequiredCapability)
	}
	if !def.Mutating {
		t.Error("restart operation must be mutating")
	}
	if !def.RequiresApproval { // J8 — V2 레인 신중 posture
		t.Error("restart operation must require approval")
	}
	if def.IdempotencyPolicy != "provider_frozen_annotation" { // J1
		t.Errorf("IdempotencyPolicy = %q, want provider_frozen_annotation", def.IdempotencyPolicy)
	}
	if def.Redaction == nil { // §10.2 typed redaction spec
		t.Error("restart operation carries no redaction spec")
	} else {
		// 독립 오라클: redaction 스펙의 허용 필드(JSON 태그)는 executor 성공
		// detail의 키 집합과 정확히 1:1이어야 한다(§3.2/§10.2 정합).
		spec := reflect.TypeOf(def.Redaction())
		if spec.Kind() != reflect.Struct {
			t.Fatalf("redaction spec is %s, want a struct", spec.Kind())
		}
		fields := make([]string, 0, spec.NumField())
		for i := 0; i < spec.NumField(); i++ {
			tag := strings.SplitN(spec.Field(i).Tag.Get("json"), ",", 2)[0]
			if tag == "" || tag == "-" {
				t.Errorf("redaction field %q lacks a json tag", spec.Field(i).Name)
				continue
			}
			fields = append(fields, tag)
		}
		sort.Strings(fields)
		want := []string{"generation", "readyReplicas", "restartedAt", "serverURL", "updated"}
		if !slices.Equal(fields, want) {
			t.Errorf("redaction allowed fields = %v, want %v (executor detail keys)", fields, want)
		}
	}

	caps := stack.Registry.Capabilities("kubernetes")
	got := map[string]bool{}
	for _, c := range caps {
		got[c.Name] = true
	}
	for _, want := range []string{"inventory.full", "orchestration.kubernetes.read", "orchestration.kubernetes.apply"} {
		if !got[want] {
			t.Errorf("kubernetes capabilities = %v, missing %q", caps, want)
		}
	}

	// V5 매핑 표의 실행 인터페이스 요구 — 등록된 어댑터가 실제로 구현한다.
	_, adapter, ok := stack.Registry.ProviderType("kubernetes")
	if !ok {
		t.Fatal("kubernetes provider type not registered")
	}
	if _, ok := adapter.(contract.OperationExecutor); !ok {
		t.Error("kubernetes adapter does not implement OperationExecutor (§3.7 apply row)")
	}
	if _, ok := adapter.(contract.TaskPoller); !ok {
		t.Error("kubernetes adapter does not implement TaskPoller (§14.3 dual mode)")
	}
}

// TestRegisterKubernetesOrderingContract — capability 선언은 provider type 등록의
// 후행 계약이다(V4). 순서 위반·중복 등록은 registration-time validation이 잡는다
// (§11 — 등록 시 판정이 권위).
func TestRegisterKubernetesOrderingContract(t *testing.T) {
	testutil.PinSecretKeys(t)

	// 1) provider type 없이 capability를 먼저 선언하면 V4가 거부한다.
	bare := registry.New()
	if err := compose.RegisterKubernetesForTest(bare); err == nil {
		t.Fatal("capability declaration before provider registration succeeded, want the V4 unregistered-type error")
	}

	// 2) 정상 순서는 성공, 재선언은 V4 중복으로 거부.
	reg := registry.New()
	k8s := kubernetes.NewAdapter()
	if err := reg.RegisterProviderType(k8s.Descriptor(), k8s); err != nil {
		t.Fatalf("RegisterProviderType: %v", err)
	}
	if err := compose.RegisterKubernetesForTest(reg); err != nil {
		t.Fatalf("RegisterKubernetesForTest: %v", err)
	}
	if err := compose.RegisterKubernetesForTest(reg); err == nil {
		t.Fatal("duplicate capability declaration succeeded, want the V4 duplicate error")
	}
}

// TestBuildRegistersAliyunInventory — Phase 4 A(M1/M2 aliyun분): compose는
// aliyun provider type + 읽기 capability 2종(판정 J10)을 등록한다. §3.7 매핑
// 표가 compute.vm.read·inventory.full에 Discoverer를 요구한다(V5) — 등록된
// 어댑터가 실제로 구현한다. opdef 없음 — Phase 4는 읽기 전용(§20).
func TestBuildRegistersAliyunInventory(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)

	stack, err := compose.Build(db)
	if err != nil {
		t.Fatalf("compose.Build: %v", err)
	}

	caps := stack.Registry.Capabilities("aliyun")
	got := map[string]contract.Capability{}
	for _, c := range caps {
		got[c.Name] = c
	}
	for _, name := range []string{"inventory.full", "compute.vm.read"} {
		c, ok := got[name]
		if !ok {
			t.Errorf("aliyun capabilities = %v, missing %q", caps, name)
			continue
		}
		if !c.ReadOnly {
			t.Errorf("capability %q must be read-only (§20)", name)
		}
		if !slices.Equal(c.ResourceKinds, []string{"compute.vm"}) {
			t.Errorf("capability %q ResourceKinds = %v, want [compute.vm] (E-3 vm family)", name, c.ResourceKinds)
		}
	}

	_, adapter, ok := stack.Registry.ProviderType("aliyun")
	if !ok {
		t.Fatal("aliyun provider type not registered")
	}
	if _, ok := adapter.(contract.Discoverer); !ok {
		t.Error("aliyun adapter does not implement Discoverer (§3.7 inventory row, V5)")
	}
	if _, ok := adapter.(contract.OperationExecutor); ok {
		t.Error("aliyun adapter must not implement OperationExecutor — Phase 4 is read-only (§20)")
	}
	// 공유 카운터 집합이 aliyun 0행을 미리 등록한다 — 리포트 렌더의 flat 증거.
	if want := `provider_rate_limit_total{provider="aliyun"} 0`; !strings.Contains(stack.Counters.Render(), want) {
		t.Errorf("counters render missing %q:\n%s", want, stack.Counters.Render())
	}

	// V4 순서 계약 — provider type 등록 없이 capability 선언은 거부된다.
	bare := registry.New()
	if err := compose.RegisterAliyunForTest(bare); err == nil {
		t.Fatal("capability declaration before provider registration succeeded, want the V4 unregistered-type error")
	}
}

// TestBuildEngineWiresTerminalHook — Phase 3 D2(M4/M9): BuildEngine assembles
// the durable engine over the stack's registry and wires the J6 OnTaskTerminal
// hook end to end. Driving a task to its terminal through the fake circuit
// must fire BOTH hook legs: the §18.1 terminal audit (the injected
// TerminalAuditFunc — production passes v2.RecordTaskTerminal; injected here
// so compose never imports the api layer) and the observation refresh
// (SyncRunner.RunSync on the task's connection — status independent, J6: a
// failed rollout's observation is still the truth of a partial rollout).
func TestBuildEngineWiresTerminalHook(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}

	// The chain the engine resolves AND the sync runner refreshes: fake
	// connection → context → resource, with a real inventory binding (a
	// sealed envelope under the pinned test key) so RunSync's broker resolve
	// succeeds. The resource urn matches the fake adapter's seeded workload —
	// the refresh re-observes the very resource the task mutated.
	now := time.Now()
	conn := model.ProviderConnection{UID: "conn-hook", ProviderType: "fake", Name: "hook cluster", Endpoint: "https://fake.invalid", Status: "active"}
	if err := db.Create(&conn).Error; err != nil {
		t.Fatal(err)
	}
	pctx := model.ProviderContext{UID: "ctx-hook", ConnectionID: conn.ID, Kind: "cluster", ExternalID: "1", Name: "hook cluster"}
	if err := db.Create(&pctx).Error; err != nil {
		t.Fatal(err)
	}
	envelope, err := util.EncryptSecretV2("test-kubeconfig-material")
	if err != nil {
		t.Fatalf("EncryptSecretV2: %v", err)
	}
	ref := model.SecretRef{UID: "ref-hook", Backend: "internal", Path: "test/hook", Ciphertext: envelope}
	if err := db.Create(&ref).Error; err != nil {
		t.Fatal(err)
	}
	binding := model.ProviderCredentialBinding{ProviderConnectionID: conn.ID, ProviderContextID: &pctx.ID, Purpose: "inventory", SecretRefID: ref.ID, Status: "active"}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatal(err)
	}
	res := model.InfraResource{
		UID: "res-hook", ContextID: pctx.ID, Kind: "orchestration.workload",
		ExternalURN: "urn:fake:workload:workload-1", DisplayName: "hook workload",
		LifecycleState: "running", HealthState: "healthy", ManagedState: "discovered",
		FirstSeenAt: now, LastSeenAt: now.Add(-time.Minute),
	}
	if err := db.Create(&res).Error; err != nil {
		t.Fatal(err)
	}
	baseline := model.ResourceObservation{
		ResourceID: res.ID, GenerationUID: res.UID, ObservationHash: "baseline", NormalizerVersion: "1",
		ObservedAt: now.Add(-time.Minute),
	}
	if err := db.Create(&baseline).Error; err != nil {
		t.Fatal(err)
	}

	stack, err := compose.Build(db)
	if err != nil {
		t.Fatalf("compose.Build: %v", err)
	}
	def := contract.OperationDefinition{
		Name: "fake.workload.restart", Version: "1",
		ResourceKinds: []string{"orchestration.workload"},
		// The fake adapter declares orchestration.kubernetes.apply (fake.go).
		RequiredCapability: "orchestration.kubernetes.apply",
		RequiredPermission: "assets:k8s:workload:restart",
		Mutating:           true, RiskLevel: "medium",
		TimeoutSeconds: 30, RetryPolicy: contract.RetryPolicy{MaxAttempts: 1},
	}
	if err := stack.Registry.RegisterOperation(def); err != nil {
		t.Fatalf("seed fake operation: %v", err)
	}

	var mu sync.Mutex
	var auditCalls []string
	audit := compose.TerminalAuditFunc(func(_ context.Context, _ *gorm.DB, task model.ProviderTask, status string, _ contract.JSONMap) error {
		mu.Lock()
		defer mu.Unlock()
		auditCalls = append(auditCalls, task.UID+"|"+status)
		return nil
	})

	engine := stack.BuildEngine(tasks.Config{
		WorkerID: "hook-test", PollInterval: time.Hour, LeaseSeconds: 30, ReaperGrace: time.Second,
	}, audit)
	if engine == nil {
		t.Fatal("BuildEngine returned nil")
	}

	ctx := context.Background()
	task, _, err := engine.Submit(ctx, tasks.SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-hook",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	processed, err := engine.RunOnce(ctx)
	if err != nil || !processed {
		t.Fatalf("RunOnce = (%v, %v), want (true, nil) — the task must reach its terminal", processed, err)
	}

	var final model.ProviderTask
	if err := db.Where("uid = ?", task.UID).First(&final).Error; err != nil {
		t.Fatal(err)
	}
	if final.Status != tasks.TaskStatusSucceeded {
		t.Fatalf("task status = %q, want succeeded (the null-handle synchronous leg)", final.Status)
	}

	// Audit leg: the §18.1 terminal writer was invoked exactly once with the
	// committed terminal task (url=task://<uid> convention is the writer's).
	mu.Lock()
	got := append([]string(nil), auditCalls...)
	mu.Unlock()
	if len(got) != 1 || got[0] != task.UID+"|"+tasks.TaskStatusSucceeded {
		t.Fatalf("terminal audit calls = %v, want exactly [%s|succeeded]", got, task.UID)
	}

	// Refresh leg: the hook ran a sync run on the task's connection and the
	// task's own resource gained a newer observation.
	var runs int64
	if err := db.Model(&model.InventorySyncRun{}).Where("context_id = ?", pctx.ID).Count(&runs).Error; err != nil {
		t.Fatalf("count sync runs: %v", err)
	}
	if runs == 0 {
		t.Fatal("no inventory sync run recorded — the observation-refresh leg never ran")
	}
	var observations int64
	if err := db.Model(&model.ResourceObservation{}).Where("resource_id = ?", res.ID).Count(&observations).Error; err != nil {
		t.Fatalf("count observations: %v", err)
	}
	if observations < 2 {
		t.Fatalf("observations for the task's resource = %d, want ≥2 (baseline + the terminal refresh)", observations)
	}
	var refreshed model.InfraResource
	if err := db.First(&refreshed, res.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !refreshed.LastSeenAt.After(res.LastSeenAt) {
		t.Error("resource last_seen_at not advanced by the terminal refresh")
	}
}

// TestBuildEngineHookSurvivesRefreshFailure — J6: a failed observation refresh
// must not skip the terminal audit row. Both legs run; the engine's
// fireTaskTerminal disposes the joined error (the terminal stands). Here the
// chain is unresolvable (no binding at all → RunSync cannot resolve the
// inventory credential) yet the audit writer still fired.
func TestBuildEngineHookSurvivesRefreshFailure(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}

	// Chain WITHOUT any binding: execution is fine (the fake runs on Config
	// alone — §3.5), but RunSync's broker resolve fails.
	conn := model.ProviderConnection{UID: "conn-norefresh", ProviderType: "fake", Name: "no refresh", Status: "active"}
	if err := db.Create(&conn).Error; err != nil {
		t.Fatal(err)
	}
	pctx := model.ProviderContext{UID: "ctx-norefresh", ConnectionID: conn.ID, Kind: "cluster", ExternalID: "1"}
	if err := db.Create(&pctx).Error; err != nil {
		t.Fatal(err)
	}
	res := model.InfraResource{
		UID: "res-norefresh", ContextID: pctx.ID, Kind: "orchestration.workload",
		ExternalURN: "urn:fake:workload:workload-1", FirstSeenAt: time.Now(), LastSeenAt: time.Now(),
	}
	if err := db.Create(&res).Error; err != nil {
		t.Fatal(err)
	}

	stack, err := compose.Build(db)
	if err != nil {
		t.Fatalf("compose.Build: %v", err)
	}
	def := contract.OperationDefinition{
		Name: "fake.workload.restart", Version: "1",
		ResourceKinds:      []string{"orchestration.workload"},
		RequiredCapability: "orchestration.kubernetes.apply",
		RequiredPermission: "assets:k8s:workload:restart",
		Mutating:           true, TimeoutSeconds: 30,
		RetryPolicy: contract.RetryPolicy{MaxAttempts: 1},
	}
	if err := stack.Registry.RegisterOperation(def); err != nil {
		t.Fatalf("seed fake operation: %v", err)
	}

	auditCalls := 0
	audit := compose.TerminalAuditFunc(func(context.Context, *gorm.DB, model.ProviderTask, string, contract.JSONMap) error {
		auditCalls++
		return nil
	})
	engine := stack.BuildEngine(tasks.Config{WorkerID: "hook-test2", PollInterval: time.Hour}, audit)

	ctx := context.Background()
	task, _, err := engine.Submit(ctx, tasks.SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "res-norefresh",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if _, err := engine.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v — a hook failure must not escape the engine (fireTaskTerminal disposes it)", err)
	}
	var final model.ProviderTask
	if err := db.Where("uid = ?", task.UID).First(&final).Error; err != nil {
		t.Fatal(err)
	}
	if final.Status != tasks.TaskStatusSucceeded {
		t.Fatalf("status = %q, want succeeded — the terminal stands despite the refresh failure (J6)", final.Status)
	}
	if auditCalls != 1 {
		t.Fatalf("terminal audit calls = %d, want 1 — the refresh failure must not skip the audit row", auditCalls)
	}
	// The hook failure is recorded, not swallowed: observation_refresh_failed.
	var mark int64
	if err := db.Model(&model.TaskEvent{}).
		Where("task_id = ? AND type = ?", final.ID, tasks.TaskEventObservationRefreshFailed).
		Count(&mark).Error; err != nil {
		t.Fatal(err)
	}
	if mark != 1 {
		t.Fatalf("observation_refresh_failed events = %d, want 1 — the failed refresh must be recorded", mark)
	}
}
