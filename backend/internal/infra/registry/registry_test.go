// Package registry_test verifies registration, lookup and validation rules
// V1–V6 (spec §11 registry validation rules, Phase 0 scope) against the
// contract vocabulary. External test package — imports fake·contract.
package registry_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/adapter/fake"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/registry"
)

// discoverOnlyAdapter is a test double that implements BaseAdapter +
// Discoverer (§11) — used as the capability-accepting provider type in
// registry tests. Test-only: no production adapter code (§22 #17).
type discoverOnlyAdapter struct{}

func (discoverOnlyAdapter) Descriptor() contract.ProviderTypeDescriptor {
	return contract.ProviderTypeDescriptor{
		Type:            "kubernetes",
		AdapterVersion:  "1",
		ProtocolVersion: "1",
		ConfigSchema:    func() contract.ConfigSpec { return contract.ConfigSpec{} },
		ContextKinds:    []string{"cluster"},
		BuiltIn:         true,
	}
}

func (discoverOnlyAdapter) Validate(_ context.Context, _ contract.ConnectionView) error { return nil }

func (discoverOnlyAdapter) Health(_ context.Context, _ contract.ConnectionView) contract.HealthResult {
	return contract.HealthResult{Healthy: true}
}

func (discoverOnlyAdapter) Close() error { return nil }

func (discoverOnlyAdapter) Discover(_ context.Context, _ contract.DiscoverRequest) (contract.DiscoverPage, error) {
	return contract.DiscoverPage{}, nil
}

// bareAdapter implements BaseAdapter only — the "no capability-serving
// interface" double for the §3.7 mapping-table validation.
type bareAdapter struct{}

func (bareAdapter) Descriptor() contract.ProviderTypeDescriptor {
	return contract.ProviderTypeDescriptor{
		Type:            "tencent",
		AdapterVersion:  "1",
		ProtocolVersion: "1",
		ConfigSchema:    func() contract.ConfigSpec { return contract.ConfigSpec{} },
		BuiltIn:         true,
	}
}

func (bareAdapter) Validate(_ context.Context, _ contract.ConnectionView) error { return nil }
func (bareAdapter) Health(_ context.Context, _ contract.ConnectionView) contract.HealthResult {
	return contract.HealthResult{Healthy: true}
}
func (bareAdapter) Close() error { return nil }

// executorOnlyAdapter implements BaseAdapter + OperationExecutor — no
// Discoverer, deliberately no TaskPoller (optional interface, §3.7).
type executorOnlyAdapter struct{ bareAdapter }

func (executorOnlyAdapter) Descriptor() contract.ProviderTypeDescriptor {
	d := bareAdapter{}.Descriptor()
	d.Type = "aliyun"
	return d
}

func (executorOnlyAdapter) Execute(_ context.Context, _ contract.OperationRequest) (contract.OperationHandle, error) {
	return contract.OperationHandle{}, nil
}

// registerBare registers one of the mapping-test doubles under its own type
// name (M1 vocabulary names, one per double).
func registerDouble(t *testing.T, r *registry.Registry, a contract.BaseAdapter, typeName string) {
	t.Helper()
	if err := r.RegisterProviderType(contract.ProviderTypeDescriptor{
		Type:            typeName,
		AdapterVersion:  "1",
		ProtocolVersion: "1",
		ConfigSchema:    func() contract.ConfigSpec { return contract.ConfigSpec{} },
	}, a); err != nil {
		t.Fatalf("RegisterProviderType(%s): %v", typeName, err)
	}
}

// newRegistry returns an empty registry with fake registered.
func newRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	r := registry.New()
	if err := fake.Register(r); err != nil {
		t.Fatalf("fake.Register: %v", err)
	}
	return r
}

// newRegistryWithDiscoverer returns a registry with a registered provider
// type ("kubernetes") whose adapter implements Discoverer.
func newRegistryWithDiscoverer(t *testing.T) *registry.Registry {
	t.Helper()
	r := registry.New()
	d := discoverOnlyAdapter{}
	if err := r.RegisterProviderType(d.Descriptor(), d); err != nil {
		t.Fatalf("RegisterProviderType(kubernetes): %v", err)
	}
	return r
}

func validCapability() contract.Capability {
	return contract.Capability{Name: "inventory.full", Version: "1"}
}

func validOperation() contract.OperationDefinition {
	return contract.OperationDefinition{
		Name:               "kubernetes.workload.restart",
		Version:            "1",
		ResourceKinds:      []string{"orchestration.workload"},
		RequiredCapability: "orchestration.kubernetes.apply",
		RequiredPermission: "assets:k8s:workload:restart",
		Mutating:           true,
		RiskLevel:          "medium",
		TimeoutSeconds:     60,
		RetryPolicy:        contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
	}
}

// T5 — V1: 예약명과 M1 어휘 밖 이름의 등록 거부, 에러에 근거(milestone/unknown) 포함.
// V3(등록자 검증: ContextKinds ⊆ ProviderContextKinds)도 RegisterProviderType의
// 검증 경로로 같은 함수에서 커버한다.
func TestRegisterProviderRejectsReservedAndUnknown(t *testing.T) {
	r := registry.New()

	err := r.RegisterProviderType(contract.ProviderTypeDescriptor{Type: "proxmox"}, discoverOnlyAdapter{})
	if err == nil {
		t.Fatal("reserved provider type registered without error")
	}
	if !strings.Contains(err.Error(), "milestone") {
		t.Errorf("reserved-name error must cite milestone, got: %v", err)
	}

	err = r.RegisterProviderType(contract.ProviderTypeDescriptor{Type: "aws"}, discoverOnlyAdapter{})
	if err == nil {
		t.Fatal("unknown provider type registered without error")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("unknown-name error must cite unknown, got: %v", err)
	}
}

// T5 보조 — V3: descriptor의 ContextKinds가 어휘 밖이면 거부.
func TestRegisterProviderRejectsUnknownContextKind(t *testing.T) {
	r := registry.New()
	d := contract.ProviderTypeDescriptor{Type: "kubernetes", ContextKinds: []string{"tenant"}}
	if err := r.RegisterProviderType(d, discoverOnlyAdapter{}); err == nil {
		t.Fatal("descriptor with unknown context kind registered without error")
	}
}

// T6 — V2: provider 유형 중복 등록 거부.
func TestRegisterProviderDuplicateRejected(t *testing.T) {
	r := newRegistry(t)
	err := r.RegisterProviderType((&fake.Adapter{}).Descriptor(), &fake.Adapter{})
	if err == nil {
		t.Fatal("duplicate provider type registered without error")
	}
}

// T7 — V4: 미등록 유형·어휘 외 이름·유형 내 중복·알려지지 않은 ResourceKinds 전부 거부.
func TestRegisterCapabilityValidation(t *testing.T) {
	r := newRegistryWithDiscoverer(t)

	// 미등록 유형 (M1 어휘에는 있으나 레지스트리에 등록되지 않음).
	if err := r.RegisterCapabilities("tencent", validCapability()); err == nil {
		t.Error("capability on unregistered provider type accepted")
	}

	// 어휘 밖 이름.
	if err := r.RegisterCapabilities("kubernetes", contract.Capability{Name: "inventory.missing", Version: "1"}); err == nil {
		t.Error("capability outside vocabulary accepted")
	}

	// 유형 내 중복.
	if err := r.RegisterCapabilities("kubernetes", validCapability()); err != nil {
		t.Fatalf("valid capability rejected: %v", err)
	}
	if err := r.RegisterCapabilities("kubernetes", validCapability()); err == nil {
		t.Error("duplicate capability within type accepted")
	}

	// 알려지지 않은 ResourceKinds.
	r2 := newRegistryWithDiscoverer(t)
	bad := contract.Capability{Name: "inventory.incremental", Version: "1", ResourceKinds: []string{"compute.unknown"}}
	if err := r2.RegisterCapabilities("kubernetes", bad); err == nil {
		t.Error("capability with unknown resource kind accepted")
	}
}

// T8 — V5에서 매핑 표 기반 검증으로 강화 (§3.7, r2 D — phase0 "Phase 1 계획 소관"
// 주석 이행): 선언 capability의 필수 인터페이스를 어댑터가 타입 단언으로
// 구현했는지 검사한다. cost.read·console.web_terminal의 빈 요구는 검증 제외가
// 아니라 표의 명시적 값이다(§3.7).
func TestCapabilityInterfaceMappingRule(t *testing.T) {
	newWithDoubles := func(t *testing.T) (bare *registry.Registry, discoverer *registry.Registry, executor *registry.Registry) {
		t.Helper()
		rBare := registry.New()
		registerDouble(t, rBare, bareAdapter{}, "tencent")
		rDiscoverer := registry.New()
		registerDouble(t, rDiscoverer, discoverOnlyAdapter{}, "kubernetes")
		rExecutor := registry.New()
		registerDouble(t, rExecutor, executorOnlyAdapter{}, "aliyun")
		return rBare, rDiscoverer, rExecutor
	}

	cap := func(name string) contract.Capability { return contract.Capability{Name: name, Version: "1"} }

	t.Run("bare adapter cannot declare an interface-requiring capability", func(t *testing.T) {
		rBare, _, _ := newWithDoubles(t)
		for _, name := range []string{"inventory.full", "inventory.incremental", "orchestration.kubernetes.read", "compute.vm.read", "orchestration.kubernetes.apply"} {
			err := rBare.RegisterCapabilities("tencent", cap(name))
			if err == nil {
				t.Errorf("bare adapter declared %q without the required interface (§3.7 table)", name)
				continue
			}
			wantIface := map[string]string{
				"inventory.full":                 "Discoverer",
				"inventory.incremental":          "Discoverer",
				"orchestration.kubernetes.read":  "Discoverer",
				"compute.vm.read":                "Discoverer",
				"orchestration.kubernetes.apply": "OperationExecutor",
			}[name]
			if !strings.Contains(err.Error(), wantIface) {
				t.Errorf("%s rejection must name the required interface %s, got: %v", name, wantIface, err)
			}
		}
	})

	t.Run("empty requirement rows are explicit table values, not exclusions", func(t *testing.T) {
		rBare, _, _ := newWithDoubles(t)
		// cost.read — §3.2 row 18 (existing scheduler); console.web_terminal —
		// ConsoleBroker M2+. Both pass with NO interface check.
		if err := rBare.RegisterCapabilities("tencent", cap("cost.read")); err != nil {
			t.Errorf("cost.read on a bare adapter rejected: %v — the empty requirement is a table value (§3.7)", err)
		}
		if err := rBare.RegisterCapabilities("tencent", cap("console.web_terminal")); err != nil {
			t.Errorf("console.web_terminal on a bare adapter rejected: %v — deferred mapping row (§3.7)", err)
		}
	})

	t.Run("discoverer-only adapter cannot declare apply", func(t *testing.T) {
		_, rDiscoverer, _ := newWithDoubles(t)
		err := rDiscoverer.RegisterCapabilities("kubernetes", cap("orchestration.kubernetes.apply"))
		if err == nil {
			t.Fatal("orchestration.kubernetes.apply declared by a Discoverer-only adapter accepted — table requires OperationExecutor")
		}
	})

	t.Run("executor-only adapter cannot declare discovery capabilities", func(t *testing.T) {
		_, _, rExecutor := newWithDoubles(t)
		if err := rExecutor.RegisterCapabilities("aliyun", cap("inventory.full")); err == nil {
			t.Fatal("inventory.full declared by an executor-only adapter accepted — table requires Discoverer")
		}
	})

	t.Run("optional interfaces stay optional", func(t *testing.T) {
		// executorOnlyAdapter deliberately lacks TaskPoller/TaskCanceller —
		// apply must still be declarable (§3.7 선택 인터페이스).
		_, _, rExecutor := newWithDoubles(t)
		if err := rExecutor.RegisterCapabilities("aliyun", cap("orchestration.kubernetes.apply")); err != nil {
			t.Errorf("apply rejected without optional TaskPoller: %v", err)
		}
	})

	t.Run("fake declares inventory.full and apply through the table", func(t *testing.T) {
		r := newRegistry(t) // fake.Register declares both capabilities (§3.7)
		caps := r.Capabilities("fake")
		names := map[string]bool{}
		for _, c := range caps {
			names[c.Name] = true
		}
		if !names["inventory.full"] || !names["orchestration.kubernetes.apply"] {
			t.Errorf("fake capabilities = %v, want inventory.full + orchestration.kubernetes.apply declared (§3.7)", caps)
		}
	})
}

// T8 보조 — 매핑 표 완결성: 표는 M1 어휘 7종과 정확히 동치(누락·중복·오타 0)여야
// 하고, 인터페이스 이름은 검증기가 아는 어휘 안에 있어야 한다.
func TestCapabilityInterfaceTableCoversVocabulary(t *testing.T) {
	rowCount := map[string]int{}
	for _, row := range contract.CapabilityInterfaceMap {
		rowCount[row.Capability]++
		switch row.RequiredInterface {
		case "", "Discoverer", "OperationExecutor":
		default:
			t.Errorf("capability %q: unknown RequiredInterface %q — the validator cannot assert it", row.Capability, row.RequiredInterface)
		}
		for _, opt := range row.OptionalInterfaces {
			if opt != "TaskPoller" && opt != "TaskCanceller" {
				t.Errorf("capability %q: unknown optional interface %q", row.Capability, opt)
			}
		}
	}
	for _, entry := range contract.M1CapabilityVocabulary {
		if rowCount[entry.Name] != 1 {
			t.Errorf("capability %q has %d mapping rows, want exactly 1 (§3.7 table ↔ M1 vocabulary 동치)", entry.Name, rowCount[entry.Name])
		}
	}
	if len(contract.CapabilityInterfaceMap) != len(contract.M1CapabilityVocabulary) {
		t.Errorf("mapping table has %d rows, vocabulary has %d — no extras allowed (§3.7)",
			len(contract.CapabilityInterfaceMap), len(contract.M1CapabilityVocabulary))
	}
}

// T9 — V6: 연산 등록 검증. 중복 (name,version)·동일 이름 재등록(A13)·어휘 외
// RequiredCapability·알려지지 않은 종·파손된 권한 문자열(A14 완화 정규식)·
// Mutating↔ReadOnly 불일치 → 전부 에러. 4세그·하이픈 세그는 수용.
func TestRegisterOperationValidation(t *testing.T) {
	// §3.7 매핑 표: inventory.full은 Discoverer 유형에, apply는
	// OperationExecutor 유형에만 선언 가능 — 시드도 그에 맞게 분배한다.
	newWithDiscoverer := func(t *testing.T) *registry.Registry {
		r := newRegistryWithDiscoverer(t)
		if err := r.RegisterCapabilities("kubernetes",
			contract.Capability{Name: "inventory.full", Version: "1"},
		); err != nil {
			t.Fatalf("seed inventory.full: %v", err)
		}
		registerDouble(t, r, executorOnlyAdapter{}, "aliyun")
		if err := r.RegisterCapabilities("aliyun",
			contract.Capability{Name: "orchestration.kubernetes.apply", Version: "1"},
		); err != nil {
			t.Fatalf("seed orchestration.kubernetes.apply: %v", err)
		}
		return r
	}

	t.Run("duplicate name and version", func(t *testing.T) {
		r := newWithDiscoverer(t)
		if err := r.RegisterOperation(validOperation()); err != nil {
			t.Fatalf("seed operation: %v", err)
		}
		if err := r.RegisterOperation(validOperation()); err == nil {
			t.Error("duplicate (name, version) operation accepted")
		}
	})

	t.Run("same name re-registration rejected", func(t *testing.T) {
		r := newWithDiscoverer(t)
		if err := r.RegisterOperation(validOperation()); err != nil {
			t.Fatalf("seed operation: %v", err)
		}
		second := validOperation()
		second.Version = "2"
		if err := r.RegisterOperation(second); err == nil {
			t.Error("same-name re-registration (different version) accepted; Phase 0 is a single-version registry (A13)")
		}
	})

	t.Run("required capability outside vocabulary", func(t *testing.T) {
		r := newWithDiscoverer(t)
		op := validOperation()
		op.RequiredCapability = "cost.write"
		if err := r.RegisterOperation(op); err == nil {
			t.Error("operation with unknown required capability accepted")
		}
	})

	t.Run("unknown resource kind", func(t *testing.T) {
		r := newWithDiscoverer(t)
		op := validOperation()
		op.ResourceKinds = []string{"compute.unknown"}
		if err := r.RegisterOperation(op); err == nil {
			t.Error("operation with unknown resource kind accepted")
		}
	})

	t.Run("broken permission strings", func(t *testing.T) {
		cases := map[string]string{
			"one segment":  "monitor",
			"five segment": "a:b:c:d:e",
			"uppercase":    "Assets:k8s:workload:restart",
			"empty seg":    "assets::restart",
		}
		for label, perm := range cases {
			r := newWithDiscoverer(t)
			op := validOperation()
			op.RequiredPermission = perm
			if err := r.RegisterOperation(op); err == nil {
				t.Errorf("%s permission %q accepted", label, perm)
			}
		}
	})

	t.Run("mutating operation referencing readonly-only capability", func(t *testing.T) {
		r := newWithDiscoverer(t)
		op := validOperation()
		op.RequiredCapability = "inventory.full" // ReadOnly: true in vocabulary
		if err := r.RegisterOperation(op); err == nil {
			t.Error("mutating operation referencing a readonly capability accepted")
		}
	})

	t.Run("accepted: four-segment and hyphenated permission", func(t *testing.T) {
		r := newWithDiscoverer(t)
		hyphen := validOperation()
		hyphen.Name = "system.ldap.sync"
		hyphen.RequiredPermission = "system:admin:ldap-sync"
		if err := r.RegisterOperation(hyphen); err != nil {
			t.Errorf("hyphenated 3-segment permission rejected: %v", err)
		}
	})
}

// T10 — 정합 operation 등록 후 Operation(name) 조회로 동일 값 회수 (reflect.DeepEqual).
func TestOperationDefinitionRoundTripLookup(t *testing.T) {
	r := newRegistryWithDiscoverer(t)
	want := validOperation()
	if err := r.RegisterOperation(want); err != nil {
		t.Fatalf("RegisterOperation: %v", err)
	}
	got, ok := r.Operation("kubernetes.workload.restart")
	if !ok {
		t.Fatal("registered operation not found")
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("round-trip mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

// T12 — RegisterCapabilities 원자성: 유효 원소 뒤에 무효 원소가 오는 배치 호출은
// 전체가 거부되어야 하며, 루프 도중 기입된 유효 원소가 잔류해서는 안 된다
// ("Registration-time validation is authoritative" 계약). 무효 원소를 제거한
// 재시도는 성공해야 한다.
func TestRegisterCapabilitiesAtomicRejectsBatch(t *testing.T) {
	r := newRegistryWithDiscoverer(t)
	if err := r.RegisterCapabilities("kubernetes", validCapability()); err != nil {
		t.Fatalf("seed capability: %v", err)
	}

	err := r.RegisterCapabilities("kubernetes",
		contract.Capability{Name: "inventory.incremental", Version: "1"},
		contract.Capability{Name: "inventory.missing", Version: "1"},
	)
	if err == nil {
		t.Fatal("batch containing an invalid capability accepted")
	}

	for _, got := range r.Capabilities("kubernetes") {
		if got.Name == "inventory.incremental" {
			t.Error("rejected batch partially committed: inventory.incremental leaked into the registry")
		}
	}

	if err := r.RegisterCapabilities("kubernetes", contract.Capability{Name: "inventory.incremental", Version: "1"}); err != nil {
		t.Errorf("retry with the invalid element removed must succeed, got: %v", err)
	}
}

// T13 — ResourceKinds()는 전역 어휘 슬라이스 별칭이 아닌 방어 복사를 반환한다:
// 반환값 제자리 쓰기는 contract.M1ResourceKinds를 오염시켜서는 안 된다.
func TestResourceKindsDefensiveCopy(t *testing.T) {
	r := newRegistry(t)
	got := r.ResourceKinds()
	if len(got) == 0 {
		t.Fatal("ResourceKinds() returned empty slice; vocabulary changed?")
	}
	want := append([]string(nil), contract.M1ResourceKinds...)

	got[0] = "compute.tampered"

	if after := r.ResourceKinds(); !reflect.DeepEqual(after, want) {
		t.Errorf("global vocabulary mutated through returned slice: got %v, want %v", after, want)
	}
	if !reflect.DeepEqual(contract.M1ResourceKinds, want) {
		t.Error("contract.M1ResourceKinds was mutated in place")
	}
}

// T11 — 미등록 유형/연산 조회는 ok=false·패닉 없음, New() 직후 ProviderTypeNames()는 빈 슬라이스.
func TestRegistryLookupMissesAndEmptyState(t *testing.T) {
	empty := registry.New()
	if names := empty.ProviderTypeNames(); len(names) != 0 {
		t.Errorf("fresh registry ProviderTypeNames() = %v, want empty", names)
	}
	if _, _, ok := empty.ProviderType("fake"); ok {
		t.Error("lookup miss returned ok=true for provider type")
	}
	if _, ok := empty.Operation("nope"); ok {
		t.Error("lookup miss returned ok=true for operation")
	}
	if caps := empty.Capabilities("fake"); len(caps) != 0 {
		t.Errorf("Capabilities on unregistered type = %v, want empty", caps)
	}

	r := newRegistry(t)
	if _, ok := r.Operation("nope"); ok {
		t.Error("lookup miss returned ok=true for operation on populated registry")
	}
	if got := r.ResourceKinds(); !reflect.DeepEqual(got, contract.M1ResourceKinds) {
		t.Errorf("ResourceKinds() does not delegate to contract.M1ResourceKinds: %v", got)
	}
}
