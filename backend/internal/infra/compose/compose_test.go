// Compose wiring test — the assembly root registers the built-in adapters
// exactly once and hands every component the shared dependency set.
package compose_test

import (
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/adapter/kubernetes"
	"ops-admin/backend/internal/infra/compose"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/testutil"
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
	if len(names) != 2 {
		t.Fatalf("registry provider types = %v, want [fake kubernetes]", names)
	}
	got := map[string]bool{}
	for _, n := range names {
		got[n] = true
	}
	if !got["fake"] || !got["kubernetes"] {
		t.Errorf("registry provider types = %v, want fake and kubernetes", names)
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
