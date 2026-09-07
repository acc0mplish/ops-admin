package contract

import (
	"reflect"
	"testing"
)

// T1 — 리소스 종 어휘가 스펙 §8.5 "Initial common kinds" 19종과
// 순서·철자까지 정확히 일치하고 중복이 없음을 단언한다.
func TestM1ResourceKindVocabularyMatchesSpec(t *testing.T) {
	want := []string{
		"machine.baremetal",
		"compute.hypervisor_node",
		"compute.vm",
		"compute.system_container",
		"compute.image",
		"compute.template",
		"orchestration.cluster",
		"orchestration.node",
		"orchestration.namespace",
		"orchestration.workload",
		"orchestration.pod",
		"network.segment",
		"network.interface",
		"network.ip",
		"network.load_balancer",
		"storage.pool",
		"storage.volume",
		"storage.snapshot",
		"identity.account",
	}
	if !reflect.DeepEqual(M1ResourceKinds, want) {
		t.Fatalf("M1ResourceKinds does not match spec §8.5 order/spelling:\n got: %v\nwant: %v", M1ResourceKinds, want)
	}
	seen := make(map[string]bool, len(M1ResourceKinds))
	for _, k := range M1ResourceKinds {
		if k == "" {
			t.Fatalf("empty resource kind in vocabulary")
		}
		if seen[k] {
			t.Errorf("duplicate resource kind %q", k)
		}
		seen[k] = true
	}
	if len(M1ResourceKinds) != 19 {
		t.Errorf("M1ResourceKinds length = %d, want 19", len(M1ResourceKinds))
	}
	if !IsKnownResourceKind("compute.vm") {
		t.Errorf("IsKnownResourceKind(\"compute.vm\") = false, want true")
	}
	if IsKnownResourceKind("compute.unknown") {
		t.Errorf("IsKnownResourceKind(\"compute.unknown\") = true, want false")
	}
	if IsKnownResourceKind("") {
		t.Errorf("IsKnownResourceKind(\"\") = true, want false")
	}
}

// T2 — provider 유형 어휘: M1 5종(proxmox 승격, phase5 §5 Phase A) + 예약 3종
// + 컨텍스트 종 4종, M1 집합과 예약 집합의 교집합 0.
func TestProviderTypeVocabulary(t *testing.T) {
	wantM1 := []string{"kubernetes", "aliyun", "tencent", "proxmox", "fake"}
	if !reflect.DeepEqual(M1ProviderTypeNames, wantM1) {
		t.Fatalf("M1ProviderTypeNames = %v, want %v", M1ProviderTypeNames, wantM1)
	}
	wantReserved := []string{"vcenter", "cloudstack", "openstack"}
	if !reflect.DeepEqual(ReservedProviderTypeNames, wantReserved) {
		t.Fatalf("ReservedProviderTypeNames = %v, want %v", ReservedProviderTypeNames, wantReserved)
	}
	wantContext := []string{"cluster", "account", "project", "subscription"}
	if !reflect.DeepEqual(ProviderContextKinds, wantContext) {
		t.Fatalf("ProviderContextKinds = %v, want %v", ProviderContextKinds, wantContext)
	}
	reservedSet := make(map[string]bool, len(ReservedProviderTypeNames))
	for _, n := range ReservedProviderTypeNames {
		reservedSet[n] = true
	}
	for _, n := range M1ProviderTypeNames {
		if reservedSet[n] {
			t.Errorf("provider type %q is both M1 and reserved", n)
		}
	}
}

// T3 — 역량 어휘가 스펙 §10.1 "Initial names (M1)" 7종과 정합:
// 이름·ReadOnly(apply만 false)·중복 0.
func TestCapabilityVocabularyMatchesSpec(t *testing.T) {
	want := []CapabilityVocabularyEntry{
		{Name: "inventory.full", ReadOnly: true, OwnerPhase: "M1"},
		{Name: "inventory.incremental", ReadOnly: true, OwnerPhase: "M1"},
		{Name: "orchestration.kubernetes.read", ReadOnly: true, OwnerPhase: "M1"},
		{Name: "orchestration.kubernetes.apply", ReadOnly: false, OwnerPhase: "M1/Phase3"},
		{Name: "compute.vm.read", ReadOnly: true, OwnerPhase: "M1"},
		{Name: "cost.read", ReadOnly: true, OwnerPhase: "M1"},
		{Name: "console.web_terminal", ReadOnly: true, OwnerPhase: "M1"},
	}
	if !reflect.DeepEqual(M1CapabilityVocabulary, want) {
		t.Fatalf("M1CapabilityVocabulary = %+v\nwant %+v", M1CapabilityVocabulary, want)
	}
	if len(M1CapabilityVocabulary) != 7 {
		t.Errorf("M1CapabilityVocabulary length = %d, want 7", len(M1CapabilityVocabulary))
	}
	seen := make(map[string]bool, len(M1CapabilityVocabulary))
	for _, e := range M1CapabilityVocabulary {
		if e.Name == "" {
			t.Fatalf("empty capability name in vocabulary")
		}
		if seen[e.Name] {
			t.Errorf("duplicate capability name %q", e.Name)
		}
		seen[e.Name] = true
		if e.Name != "orchestration.kubernetes.apply" && !e.ReadOnly {
			t.Errorf("capability %q ReadOnly = false, want true (only apply is non-readonly)", e.Name)
		}
	}
}

// T4 — 최소 형상 계약: 어휘 슬라이스가 전부 비어 있지 않은 원소만 갖고,
// ProviderTypeDescriptor의 §7.1 6필드가 컴파일 시점 존재·값 단언 가능함을 확인.
func TestContractSupportTypesAreMinimal(t *testing.T) {
	for _, n := range M1ProviderTypeNames {
		if n == "" {
			t.Errorf("empty name in M1ProviderTypeNames")
		}
	}
	for _, n := range ReservedProviderTypeNames {
		if n == "" {
			t.Errorf("empty name in ReservedProviderTypeNames")
		}
	}
	for _, k := range ProviderContextKinds {
		if k == "" {
			t.Errorf("empty kind in ProviderContextKinds")
		}
	}
	for _, e := range M1CapabilityVocabulary {
		if e.OwnerPhase == "" {
			t.Errorf("capability %q has empty OwnerPhase", e.Name)
		}
	}

	// §7.1 descriptor 6필드 전수 — 컴파일 시점 필드 존재 + 값 단언.
	d := ProviderTypeDescriptor{
		Type:            "fake",
		AdapterVersion:  "1",
		ProtocolVersion: "1",
		ConfigSchema:    func() ConfigSpec { return ConfigSpec{} },
		ContextKinds:    []string{"cluster"},
		BuiltIn:         true,
	}
	if d.Type != "fake" || d.AdapterVersion != "1" || d.ProtocolVersion != "1" || !d.BuiltIn {
		t.Errorf("ProviderTypeDescriptor field round-trip broken: %+v", d)
	}
	if got := d.ConfigSchema(); len(got.Fields) != 0 {
		t.Errorf("ConfigSchema() = %+v, want empty ConfigSpec", got)
	}
	if !reflect.DeepEqual(d.ContextKinds, []string{"cluster"}) {
		t.Errorf("ContextKinds round-trip broken: %v", d.ContextKinds)
	}

	// 지원 타입 최소 형상 — 필드 존재·값 단언(가정 A6).
	f := ConfigFieldSpec{Name: "region", Type: "string", Required: true, Secret: false}
	if f.Name != "region" || f.Type != "string" || !f.Required || f.Secret {
		t.Errorf("ConfigFieldSpec round-trip broken: %+v", f)
	}
	m := JSONMap{"k": "v"}
	if m["k"] != "v" {
		t.Errorf("JSONMap round-trip broken: %v", m)
	}
	rp := RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5}
	if rp.MaxAttempts != 3 || rp.BackoffSeconds != 5 {
		t.Errorf("RetryPolicy round-trip broken: %+v", rp)
	}
	op := OperationDefinition{
		Name:               "op",
		Version:            "1",
		ResourceKinds:      []string{"compute.vm"},
		RequiredCapability: "compute.vm.read",
		RequiredPermission: "a:b:c",
		Mutating:           false,
		RiskLevel:          "low",
		RequiresApproval:   false,
		IdempotencyPolicy:  "natural",
		TimeoutSeconds:     30,
		RetryPolicy:        rp,
		Redaction:          func() any { return nil },
	}
	if op.Name != "op" || op.Redaction == nil {
		t.Errorf("OperationDefinition field round-trip broken: %+v", op)
	}
}
