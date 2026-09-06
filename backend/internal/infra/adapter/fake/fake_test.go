package fake

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/contracttest"
	"ops-admin/backend/internal/infra/registry"
)

// T13 — 게이트 3 단언: fake 등록 후 Registry 조회에 등장하고,
// BaseAdapter 구현체가 회수되며, Phase 1 실행 형상(§3.7/§3.8)의 capability
// inventory.full + orchestration.kubernetes.apply를 선언한다 — 매핑 표 검증이
// 실제로 발화하는 경로(가정 A7 해소).
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
	// Phase 1 실행 인터페이스 — phase0 가정 A7 해소.
	if _, isDiscoverer := adapter.(contract.Discoverer); !isDiscoverer {
		t.Error("fake must implement Discoverer in Phase 1 (§3.8)")
	}
	if _, isExecutor := adapter.(contract.OperationExecutor); !isExecutor {
		t.Error("fake must implement OperationExecutor in Phase 1 (§3.8)")
	}
	if _, isPoller := adapter.(contract.TaskPoller); !isPoller {
		t.Error("fake must implement TaskPoller in Phase 1 (§3.8 async path)")
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

	caps := r.Capabilities("fake")
	names := map[string]bool{}
	for _, c := range caps {
		names[c.Name] = true
	}
	if !names["inventory.full"] || !names["orchestration.kubernetes.apply"] {
		t.Errorf("Capabilities(\"fake\") = %+v, want inventory.full + orchestration.kubernetes.apply (§3.7)", caps)
	}
	if len(caps) != 2 {
		t.Errorf("Capabilities(\"fake\") has %d entries, want exactly the two §3.7 declarations", len(caps))
	}
}

// fake BaseAdapter stub 동작. 커버리지 게이트(§23.3, ≥80%) 충족을 위해 stub
// 표면을 직접 단언한다.
func TestFakeBaseAdapterStubs(t *testing.T) {
	var a contract.BaseAdapter = &Adapter{}
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

// §3.8 — Discover: 고정 시드 인메모리 리소스 페이지(orchestration.* 종 3종),
// 단일 페이지 NextCursor "" 로 종결.
func TestFakeDiscoverPages(t *testing.T) {
	a := &Adapter{}
	page, err := a.Discover(context.Background(), contract.DiscoverRequest{ContextID: 1})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(page.Resources) != 3 {
		t.Fatalf("first page resources = %d, want 3 (orchestration.* 3종, §3.8)", len(page.Resources))
	}
	if page.NextCursor != "" {
		t.Errorf("NextCursor = %q, want terminal (single page)", page.NextCursor)
	}
	kinds := map[string]bool{}
	for _, res := range page.Resources {
		if !contract.IsKnownResourceKind(res.Kind) {
			t.Errorf("discovered kind %q outside the M1 vocabulary", res.Kind)
		}
		kinds[res.Kind] = true
	}
	for _, want := range []string{"orchestration.cluster", "orchestration.node", "orchestration.workload"} {
		if !kinds[want] {
			t.Errorf("first page missing orchestration kind %q", want)
		}
	}

	// A non-empty cursor ends discovery — paging terminates deterministically.
	next, err := a.Discover(context.Background(), contract.DiscoverRequest{ContextID: 1, Cursor: "stale"})
	if err != nil {
		t.Fatalf("Discover(cursor): %v", err)
	}
	if len(next.Resources) != 0 || next.NextCursor != "" {
		t.Errorf("Discover(non-empty cursor) = (%d resources, cursor %q), want terminal empty page", len(next.Resources), next.NextCursor)
	}
}

// §3.8 — Execute: 호출 카운트 기록(N6 "실행 1회" 단얿의 근거), Payload
// 제어: failAttempt==n → n회째 오류, async==true → ProviderRef 반환(§14.3
// UPID 이중 모드), 기본 → 널 핸들 동기 완료. Poll은 Execute가 남긴 핸들
// 상태를 succeeded/failed/running으로 반환한다.
func TestFakeExecuteAndPoll(t *testing.T) {
	t.Run("synchronous null handle", func(t *testing.T) {
		a := &Adapter{}
		handle, err := a.Execute(context.Background(), contract.OperationRequest{
			OperationName: "fake.workload.restart",
			ResourceURN:   "urn:fake:workload:workload-1",
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if handle.ProviderRef != "" {
			t.Errorf("sync Execute handle = %q, want empty (null handle = synchronous completion, §14.3)", handle.ProviderRef)
		}
		if a.ExecCount() != 1 {
			t.Errorf("ExecCount = %d, want 1", a.ExecCount())
		}
		if got := a.LastRequest().ResourceURN; got != "urn:fake:workload:workload-1" {
			t.Errorf("LastRequest ResourceURN = %q, want the engine-assembled URN echo", got)
		}
	})

	t.Run("failAttempt injects a failure on the nth attempt", func(t *testing.T) {
		a := &Adapter{}
		req := contract.OperationRequest{OperationName: "op", Payload: contract.JSONMap{"failAttempt": 2}}
		if _, err := a.Execute(context.Background(), req); err != nil {
			t.Fatalf("attempt 1 must succeed: %v", err)
		}
		if _, err := a.Execute(context.Background(), req); err == nil {
			t.Fatal("attempt 2 must fail (failAttempt=2)")
		}
		if _, err := a.Execute(context.Background(), req); err != nil {
			t.Fatalf("attempt 3 must succeed again: %v", err)
		}
		if a.ExecCount() != 3 {
			t.Errorf("ExecCount = %d, want 3 — every attempt counts, failed or not", a.ExecCount())
		}
	})

	t.Run("async handle polls to success without a new attempt", func(t *testing.T) {
		a := &Adapter{}
		handle, err := a.Execute(context.Background(), contract.OperationRequest{
			Payload: contract.JSONMap{"async": true, "polls": 2},
		})
		if err != nil {
			t.Fatalf("Execute(async): %v", err)
		}
		if handle.ProviderRef == "" {
			t.Fatal("async Execute returned a null handle — Poll path not engaged")
		}

		status, err := a.Poll(context.Background(), contract.PollRequest{Handle: handle})
		if err != nil {
			t.Fatalf("Poll 1: %v", err)
		}
		if status.State != contract.OperationStateRunning {
			t.Errorf("Poll 1 state = %q, want running (polls=2)", status.State)
		}
		status, err = a.Poll(context.Background(), contract.PollRequest{Handle: handle})
		if err != nil {
			t.Fatalf("Poll 2: %v", err)
		}
		if status.State != contract.OperationStateSucceeded {
			t.Errorf("Poll 2 state = %q, want succeeded", status.State)
		}
		if a.ExecCount() != 1 {
			t.Errorf("ExecCount = %d after async completion, want 1 — polling never re-executes (§3.6)", a.ExecCount())
		}
	})

	t.Run("async handle can fail", func(t *testing.T) {
		a := &Adapter{}
		handle, err := a.Execute(context.Background(), contract.OperationRequest{
			Payload: contract.JSONMap{"async": true, "asyncFail": true},
		})
		if err != nil {
			t.Fatalf("Execute(async, fail): %v", err)
		}
		status, err := a.Poll(context.Background(), contract.PollRequest{Handle: handle})
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if status.State != contract.OperationStateFailed {
			t.Errorf("Poll state = %q, want failed", status.State)
		}
	})

	t.Run("unknown handle is an error", func(t *testing.T) {
		a := &Adapter{}
		if _, err := a.Poll(context.Background(), contract.PollRequest{Handle: contract.OperationHandle{ProviderRef: "nope"}}); err == nil {
			t.Fatal("Poll with an unknown handle must error")
		} else if !errors.Is(err, errUnknownHandle) {
			t.Errorf("Poll unknown-handle error = %v, want errUnknownHandle", err)
		}
	})
}

// M19/J12 — PollRequest 전파: Poll은 PollRequest{Handle, Connection}를 받고
// LastPoll이 그 요청을 그대로 노출한다 — 엔진 회로 테스트(N11)가 두 폴 조립
// 지점의 자격 전달을 이 기구로 단얫한다.
func TestFakePollRequestPropagation(t *testing.T) {
	a := &Adapter{}
	handle, err := a.Execute(context.Background(), contract.OperationRequest{
		Payload: contract.JSONMap{"async": true, "polls": 1},
	})
	if err != nil {
		t.Fatalf("Execute(async): %v", err)
	}
	req := contract.PollRequest{
		Handle: handle,
		Connection: contract.ConnectionView{
			UID:          "conn-circuit",
			ProviderType: "fake",
			Material:     map[string]string{contract.CredentialPurposeOperations: "material"},
		},
	}
	status, err := a.Poll(context.Background(), req)
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if status.State != contract.OperationStateSucceeded {
		t.Fatalf("Poll state = %q, want succeeded (polls=1)", status.State)
	}

	got := a.LastPoll()
	if got.Handle.ProviderRef != handle.ProviderRef {
		t.Errorf("LastPoll handle = %q, want %q", got.Handle.ProviderRef, handle.ProviderRef)
	}
	if got.Connection.UID != "conn-circuit" {
		t.Errorf("LastPoll connection UID = %q, want conn-circuit", got.Connection.UID)
	}
	if m := got.Connection.Material[contract.CredentialPurposeOperations]; m != "material" {
		t.Errorf("LastPoll Material[operations] = %q, want the passed-through material", m)
	}
}

// T44 — fake passes the contracttest harness: the §23.1 four assertion
// families (discovery paging, error taxonomy, rate-limit signaling,
// redaction) against the seeded single page. fake is the harness's own
// measurement-instrument canary (계획 J5 — 하네스 자체의 측정 기구 검증).
func TestFakePassesContractHarness(t *testing.T) {
	a := &Adapter{}
	contracttest.RunContractSuite(t, a, fakeFixture{a: a})
}

// fakeFixture adapts *Adapter to the harness Fixture seam. Seed/PageLimit are
// test-side knowledge; scenario arming delegates to the adapter.
type fakeFixture struct{ a *Adapter }

func (f fakeFixture) Seed() []contract.DiscoveredResource { return seededResources }

func (f fakeFixture) PageLimit() int { return len(seededResources) }

func (f fakeFixture) Scenario(name string) error { return f.a.Scenario(name) }

// Connection — the fake needs no credential material (J2 path is exercised
// for real by the kubernetes adapter's harness run).
func (fakeFixture) Connection() contract.ConnectionView { return contract.ConnectionView{} }
