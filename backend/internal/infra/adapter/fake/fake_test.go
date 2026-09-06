package fake

import (
	"context"
	"reflect"
	"testing"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/registry"
)

// T13 — 게이트 3 단언: fake 등록 후 Registry 조회에 등장하고,
// BaseAdapter 구현체가 회수되며, capability는 0건(Phase 0 계약, 가정 A7)이다.
func TestFakeAdapterRegistersAndLooksUp(t *testing.T) {
	r := registry.New()
	if err := Register(r); err != nil {
		t.Fatalf("fake.Register: %v", err)
	}

	found := false
	for _, n := range r.ProviderTypeNames() {
		if n == "fake" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ProviderTypeNames() = %v, want to contain \"fake\"", r.ProviderTypeNames())
	}

	desc, adapter, ok := r.ProviderType("fake")
	if !ok {
		t.Fatal("ProviderType(\"fake\") not found after Register")
	}
	if _, isBase := adapter.(contract.BaseAdapter); !isBase {
		t.Fatalf("retrieved adapter %#v does not implement contract.BaseAdapter", adapter)
	}
	if _, isDiscoverer := adapter.(contract.Discoverer); isDiscoverer {
		t.Error("fake must not implement Discoverer in Phase 0 (A7)")
	}
	if _, isExecutor := adapter.(contract.OperationExecutor); isExecutor {
		t.Error("fake must not implement OperationExecutor in Phase 0 (A7)")
	}

	// descriptor 비교 — func 필드(ConfigSchema)는 reflect.DeepEqual 비교 불가라
	// 제로화 후 나머지 필드를 전수 비교한다.
	got := desc
	got.ConfigSchema = nil
	want := contract.ProviderTypeDescriptor{
		Type:            "fake",
		AdapterVersion:  "1",
		ProtocolVersion: "1",
		ContextKinds:    nil,
		BuiltIn:         true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fake descriptor mismatch:\n got: %+v\nwant: %+v", got, want)
	}
	if desc.ConfigSchema == nil {
		t.Error("fake descriptor ConfigSchema must be a non-nil typed builder")
	}
	if cs := desc.ConfigSchema(); len(cs.Fields) != 0 {
		t.Errorf("fake ConfigSchema() = %+v, want empty ConfigSpec", cs)
	}

	if caps := r.Capabilities("fake"); len(caps) != 0 {
		t.Errorf("Capabilities(\"fake\") = %+v, want empty (Phase 0: registration only, A7)", caps)
	}
}

// T13 보조 — fake BaseAdapter stub 동작(가정 A7: 기능 없음). 커버리지 게이트
// (§23.3, ≥80%) 충족을 위해 Phase 0 stub 표면을 직접 단언한다.
func TestFakeBaseAdapterStubs(t *testing.T) {
	var a contract.BaseAdapter = Adapter{}
	if err := a.Validate(context.Background(), contract.ConnectionView{ProviderType: "fake"}); err != nil {
		t.Errorf("fake Validate = %v, want nil", err)
	}
	h := a.Health(context.Background(), contract.ConnectionView{})
	if !h.Healthy {
		t.Errorf("fake Health = %+v, want healthy", h)
	}
	if err := a.Close(); err != nil {
		t.Errorf("fake Close = %v, want nil", err)
	}
}
