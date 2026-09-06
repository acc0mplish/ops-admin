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
	err := r.RegisterProviderType(fake.Adapter{}.Descriptor(), fake.Adapter{})
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

// T8 — V5 최소 가드: capability-수용 인터페이스(Discoverer·OperationExecutor)를
// 하나도 구현하지 않는 어댑터(fake)의 어떤 capability 선언도 거부된다.
// capability별 인터페이스 매핑 단언은 Phase 1 계획 소관(계획 §13 인계 4).
func TestCapabilityInterfaceMappingRule(t *testing.T) {
	r := newRegistry(t)
	err := r.RegisterCapabilities("fake", validCapability())
	if err == nil {
		t.Fatal("capability declared by adapter implementing no capability-serving interface was accepted")
	}
}

// T9 — V6: 연산 등록 검증. 중복 (name,version)·동일 이름 재등록(A13)·어휘 외
// RequiredCapability·알려지지 않은 종·파손된 권한 문자열(A14 완화 정규식)·
// Mutating↔ReadOnly 불일치 → 전부 에러. 4세그·하이픈 세그는 수용.
func TestRegisterOperationValidation(t *testing.T) {
	newWithDiscoverer := func(t *testing.T) *registry.Registry {
		r := newRegistryWithDiscoverer(t)
		if err := r.RegisterCapabilities("kubernetes",
			contract.Capability{Name: "orchestration.kubernetes.apply", Version: "1"},
			contract.Capability{Name: "inventory.full", Version: "1"},
		); err != nil {
			t.Fatalf("seed capabilities: %v", err)
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
