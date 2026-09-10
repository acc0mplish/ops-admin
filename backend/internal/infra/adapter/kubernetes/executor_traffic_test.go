// executor_traffic_test — P2-D: traffic mutation 2종(k8s.istio.traffic_update·
// k8s.httproute.traffic_update)의 Execute·Poll 계약 테스트. 왕복 하네스 소비는
// TestOperationContractRoundtrip(executor_workload_test.go — 10종으로 확정)이
// 담당하고, 본 파일은 traffic 가족 고유의 판정면(v1beta1 폴백 경로·첫 조정 가능
// 엔트리 선정·가중치 splice 본문·동결 가중치 에코 폴)을 단얫한다. 모의 서버·
// fixture는 executor_traffic_mock_test.go가 담당한다.
package kubernetes

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/contract"
)

// --- Execute — k8s.istio.traffic_update 계약. ---

// TestExecuteIstioTrafficPutsWeightedRoutes — v1 후보 404 → v1beta1 GET → 가중치
// splice PUT 1회: 경로·Content-Type·검증 문언이 v1 UpdateK8sIstioTraffic과 동일
// 와이어고, 첫 조정 가능 http 항목(인덱스 1 — match 전용 선행 항목 스킵)에만
// 가중치가 반영된다. handle은 동결 엔트리 위치·가중치를 자기서술한다. 재실행은
// resourceVersion만 fresh 관측을 따라가고 본문은 byte-identical 수렴, generation
// 무증가다(provider_frozen_payload — apply 왕복 단얫과 동일 3왕복 형태).
func TestExecuteIstioTrafficPutsWeightedRoutes(t *testing.T) {
	f := newTrafficFixtures(t)[IstioTrafficUpdateOperationName]

	handle, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	puts := f.mock.recordsOf(http.MethodPut)
	if len(puts) != 1 {
		t.Fatalf("put count = %d, want 1", len(puts))
	}
	if want := f.mock.vsBasePath(); puts[0].Path != want {
		t.Errorf("put path = %q, want the v1beta1 candidate %q (v1 candidate 404 → fallback)", puts[0].Path, want)
	}
	if want := "application/json"; puts[0].ContentType != want {
		t.Errorf("put Content-Type = %q, want %q (v1 wire)", puts[0].ContentType, want)
	}
	var body map[string]any
	if err := json.Unmarshal(puts[0].Body, &body); err != nil {
		t.Fatalf("put body decode: %v\nbody: %s", err, puts[0].Body)
	}
	if body["apiVersion"] != "networking.istio.io/v1beta1" || body["kind"] != "VirtualService" {
		t.Errorf("put envelope = %v/%v, want networking.istio.io/v1beta1/VirtualService", body["apiVersion"], body["kind"])
	}
	// 편차 2 단얫 — 관측 resourceVersion이 그대로 실린다(낙관 동시성).
	metadata := body["metadata"].(map[string]any)
	if metadata["resourceVersion"] != "300" {
		t.Errorf("put resourceVersion = %v, want the observed %q carried (optimistic concurrency)", metadata["resourceVersion"], "300")
	}
	spec := body["spec"].(map[string]any)
	if hosts := spec["hosts"]; !sameJSON(hosts, []any{"seed.example.com"}) {
		t.Errorf("put hosts = %v, want the observed hosts preserved (full-fidelity splice)", hosts)
	}
	httpEntries := spec["http"].([]any)
	if len(httpEntries) != 2 {
		t.Fatalf("put http entries = %d, want 2", len(httpEntries))
	}
	if _, hasMatch := httpEntries[0].(map[string]any)["match"]; !hasMatch {
		t.Errorf("non-routed leading entry = %v, want the match-only entry preserved untouched", httpEntries[0])
	}
	routes := httpEntries[1].(map[string]any)["route"].([]any)
	if len(routes) != 2 {
		t.Fatalf("target entry routes = %d, want 2", len(routes))
	}
	first := routes[0].(map[string]any)
	if first["weight"] != float64(70) {
		t.Errorf("route[0] weight = %v, want 70", first["weight"])
	}
	if got := first["destination"].(map[string]any)["host"]; got != "a.seed.svc.cluster.local" {
		t.Errorf("route[0] destination.host = %v, want preserved", got)
	}
	second := routes[1].(map[string]any)
	if second["weight"] != float64(30) {
		t.Errorf("route[1] weight = %v, want 30", second["weight"])
	}
	if got := second["destination"].(map[string]any)["subset"]; got != "v2" {
		t.Errorf("route[1] destination.subset = %v, want preserved", got)
	}

	// handle — state 가족: 동결 엔트리 위치(1 — 첫 조정 가능 항목)·가중치.
	got := mustDecodeStateHandle(t, handle.ProviderRef)
	if got.ConnectionUID != execConnUID || got.APIPath != f.mock.vsBasePath() {
		t.Errorf("state handle = %+v, want connUID %q apiPath %q", got, execConnUID, f.mock.vsBasePath())
	}
	if got.Expectation.Resource != "traffic_istio" || got.Expectation.EntryIndex != 1 ||
		!sameJSON(got.Expectation.Weights, []int{70, 30}) {
		t.Errorf("expectation = %q entry %d weights %v, want traffic_istio entry 1 weights [70 30]",
			got.Expectation.Resource, got.Expectation.EntryIndex, got.Expectation.Weights)
	}

	// 재실행 — RV만 fresh 관측을 따라가고(1차 재실행 PUT 후 mock 무증가), 그 뒤
	// 본문은 byte-identical이다. 어느 재실행도 generation을 올리지 않는다.
	secondReq := f.Request()
	secondHandle, err := NewAdapter().Execute(context.Background(), secondReq)
	if err != nil {
		t.Fatalf("second Execute (frozen payload replay): %v", err)
	}
	thirdHandle, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("third Execute (stable replay): %v", err)
	}
	puts = f.mock.recordsOf(http.MethodPut)
	if len(puts) != 3 {
		t.Fatalf("put count after replays = %d, want 3", len(puts))
	}
	var firstBody, secondBody map[string]any
	if err := json.Unmarshal(puts[0].Body, &firstBody); err != nil {
		t.Fatalf("first put decode: %v", err)
	}
	if err := json.Unmarshal(puts[1].Body, &secondBody); err != nil {
		t.Fatalf("replay put decode: %v", err)
	}
	firstMeta := firstBody["metadata"].(map[string]any)
	secondMeta := secondBody["metadata"].(map[string]any)
	if firstMeta["resourceVersion"] == secondMeta["resourceVersion"] {
		t.Error("replay reuses the execute-time resourceVersion — the mock state should have advanced to 301")
	}
	delete(firstMeta, "resourceVersion")
	delete(secondMeta, "resourceVersion")
	if !sameJSON(firstBody, secondBody) {
		t.Errorf("replay differs beyond resourceVersion:\nfirst:  %s\nsecond: %s", puts[0].Body, puts[1].Body)
	}
	if string(puts[1].Body) != string(puts[2].Body) {
		t.Errorf("stable replay is not byte-identical:\nsecond: %s\nthird:  %s", puts[1].Body, puts[2].Body)
	}
	if secondHandle.ProviderRef != handle.ProviderRef || thirdHandle.ProviderRef != handle.ProviderRef {
		t.Errorf("replay handle changed: %q → %q → %q", handle.ProviderRef, secondHandle.ProviderRef, thirdHandle.ProviderRef)
	}
	if got := f.mock.vsGeneration(); got != 2 {
		t.Errorf("virtualservice generation after replays = %d, want 2 (identical weights must not bump)", got)
	}
}

// --- Execute — k8s.httproute.traffic_update 계약. ---

// TestExecuteHTTPRouteTrafficPutsBackendWeights — GatewayAPI 폴백 경로에서 첫 조정
// 가능 rule(인덱스 1 — matches 전용 선행 rule 스킵)의 backendRefs에 가중치가
// 반영되고 선행 rule·hostnames는 보존된다. handle은 traffic_httproute 판별자로
// 동결한다.
func TestExecuteHTTPRouteTrafficPutsBackendWeights(t *testing.T) {
	f := newTrafficFixtures(t)[HTTPRouteTrafficUpdateOperationName]

	handle, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	puts := f.mock.recordsOf(http.MethodPut)
	if len(puts) != 1 {
		t.Fatalf("put count = %d, want 1", len(puts))
	}
	if want := f.mock.routeBasePath(); puts[0].Path != want {
		t.Errorf("put path = %q, want the v1beta1 candidate %q (v1 candidate 404 → fallback)", puts[0].Path, want)
	}
	var body map[string]any
	if err := json.Unmarshal(puts[0].Body, &body); err != nil {
		t.Fatalf("put body decode: %v\nbody: %s", err, puts[0].Body)
	}
	if body["kind"] != "HTTPRoute" {
		t.Errorf("put kind = %v, want HTTPRoute", body["kind"])
	}
	metadata := body["metadata"].(map[string]any)
	if metadata["resourceVersion"] != "400" {
		t.Errorf("put resourceVersion = %v, want the observed %q carried", metadata["resourceVersion"], "400")
	}
	spec := body["spec"].(map[string]any)
	if hostnames := spec["hostnames"]; !sameJSON(hostnames, []any{"seed.example.com"}) {
		t.Errorf("put hostnames = %v, want preserved", hostnames)
	}
	rules := spec["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("put rules = %d, want 2", len(rules))
	}
	if _, hasMatches := rules[0].(map[string]any)["matches"]; !hasMatches {
		t.Errorf("leading rule = %v, want the matches-only rule preserved untouched", rules[0])
	}
	refs := rules[1].(map[string]any)["backendRefs"].([]any)
	if len(refs) != 2 {
		t.Fatalf("target rule backendRefs = %d, want 2", len(refs))
	}
	if refs[0].(map[string]any)["weight"] != float64(70) {
		t.Errorf("backendRefs[0] weight = %v, want 70", refs[0].(map[string]any)["weight"])
	}
	if refs[1].(map[string]any)["weight"] != float64(30) {
		t.Errorf("backendRefs[1] weight = %v, want 30", refs[1].(map[string]any)["weight"])
	}
	got := mustDecodeStateHandle(t, handle.ProviderRef)
	if got.Expectation.Resource != "traffic_httproute" || got.Expectation.EntryIndex != 1 ||
		!sameJSON(got.Expectation.Weights, []int{70, 30}) {
		t.Errorf("expectation = %q entry %d weights %v, want traffic_httproute entry 1 weights [70 30]",
			got.Expectation.Resource, got.Expectation.EntryIndex, got.Expectation.Weights)
	}
}

// TestExecuteTrafficValidationRejects — 검증 실패는 HTTP write 0회. v1 문언
// 순서(invalid payload → no adjustable → count changed → weight ≥ 0 → total 100)
// 과 URN 면 밖 거부를 2종 모두에서 단얫한다.
func TestExecuteTrafficValidationRejects(t *testing.T) {
	cases := []struct {
		op     string
		fixt   *trafficFixture
		phrase string
	}{
		{IstioTrafficUpdateOperationName, newTrafficFixtures(t)[IstioTrafficUpdateOperationName], "invalid istio traffic payload"},
		{HTTPRouteTrafficUpdateOperationName, newTrafficFixtures(t)[HTTPRouteTrafficUpdateOperationName], "invalid http route traffic payload"},
	}
	for _, c := range cases {
		t.Run(c.op, func(t *testing.T) {
			for i, bad := range c.fixt.BadRequests() {
				_, err := NewAdapter().Execute(context.Background(), bad)
				if err == nil {
					t.Fatalf("bad request #%d (%s) succeeded, want error", i, bad.ResourceURN)
				}
			}
			if n := c.fixt.mock.mutationCount(); n != 0 {
				t.Errorf("validation failures issued %d mutation(s), want 0", n)
			}
			// 개별 문언 — v1 순서의 중간 단계 3건.
			urn := c.fixt.urn
			for _, tc := range []struct {
				payload contract.JSONMap
				want    string
			}{
				{trafficWeightsPayload(34, 33, 33), "changed, please refresh and try again"},
				{trafficWeightsPayload(-1, 101), "traffic weight must be greater than or equal to 0"},
				{trafficWeightsPayload(50, 49), "traffic weights must total 100"},
			} {
				_, err := NewAdapter().Execute(context.Background(), contract.OperationRequest{
					OperationName: c.op, ResourceURN: urn, Payload: tc.payload, Connection: c.fixt.Connection(),
				})
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Errorf("payload %v error = %v, want %q", tc.payload, err, tc.want)
				}
			}
			// 조정 가능 엔트리 부재는 없는 mock 형태다 — ghost 목표의 후보 404
			// 순회 거부를 보충 단얫한다(J12 가드가 먼저 뜨지 않도록 Connection을
			// 심는다).
			ghost := c.fixt.BadRequests()[len(c.fixt.BadRequests())-1]
			ghost.Connection = c.fixt.Connection()
			_, err := NewAdapter().Execute(context.Background(), ghost)
			if err == nil || !strings.Contains(err.Error(), "was not found on any served api version") {
				t.Errorf("ghost target error = %v, want the candidate-exhaustion wording", err)
			}
		})
	}
}

// --- Poll — traffic 가족의 판정면. ---

// TestPollTrafficEchoRunningUntilConverged — 동결 가중치 에코 판정의 반증
// 가능성: 관측 가중치가 동결값에서 어긋나면 Running(detail 없음), 회복되면
// Succeeded(stateResultRedaction 2키)로 수렴한다. J12 — 다른 커넥션의 폴은 조립
// 버그로 즉시 거부된다.
func TestPollTrafficEchoRunningUntilConverged(t *testing.T) {
	adapter := NewAdapter()
	f := newTrafficFixtures(t)[IstioTrafficUpdateOperationName]

	handle, err := adapter.Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	f.mock.setWeights(&f.mock.vs, "http", "route", 1, []int{10, 90})
	status, err := adapter.Poll(context.Background(), contract.PollRequest{Handle: handle, Connection: f.Connection()})
	if err != nil {
		t.Fatalf("Poll (tampered): %v", err)
	}
	if status.State != contract.OperationStateRunning || status.Detail != nil {
		t.Errorf("poll with tampered weights = {%v %v}, want running with no detail", status.State, status.Detail)
	}
	f.mock.setWeights(&f.mock.vs, "http", "route", 1, []int{70, 30})
	status, err = adapter.Poll(context.Background(), contract.PollRequest{Handle: handle, Connection: f.Connection()})
	if err != nil {
		t.Fatalf("Poll (recovered): %v", err)
	}
	if status.State != contract.OperationStateSucceeded {
		t.Fatalf("poll state = %v, want succeeded", status.State)
	}
	assertKeySet(t, "traffic poll detail", status.Detail, "generation", "serverURL")
	assertServerURL(t, status.Detail)
	if got, _ := status.Detail["serverURL"].(string); got != f.mock.srv.URL {
		t.Errorf("detail serverURL = %q, want %q", got, f.mock.srv.URL)
	}

	_, err = adapter.Poll(context.Background(), contract.PollRequest{
		Handle: handle,
		Connection: contract.ConnectionView{
			UID:          "conn-uid-other",
			ProviderType: ProviderName,
			Material:     f.Connection().Material,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "assembly bug") {
		t.Errorf("cross-connection poll error = %v, want the J12 assembly-bug guard", err)
	}
}

// TestPollTrafficHandleRejectsGarbage — 손상된 traffic handle은 폴 전에 거부된다:
// 무가중치 expectation은 decode에서(encodeStateRef가 만들지 않는 형태 — P2-D
// 판단 기록), apiPath 접두 위반은 기존 가드에서 거부된다.
func TestPollTrafficHandleRejectsGarbage(t *testing.T) {
	adapter := NewAdapter()
	f := newTrafficFixtures(t)[IstioTrafficUpdateOperationName]
	garbage := []string{
		"state|conn|/apis/networking.istio.io/v1beta1/namespaces/x/virtualservices/x|" + encodeGarbageExpectation(`{"resource":"traffic_istio","entryIndex":1}`),
		"state|conn|/apis/networking.istio.io/v1beta1/namespaces/x/virtualservices/x|" + encodeGarbageExpectation(`{"resource":"traffic_httproute"}`),
		"state|conn|notaapi|" + encodeGarbageExpectation(`{"resource":"traffic_istio","entryIndex":0,"weights":[70,30]}`),
	}
	for _, ref := range garbage {
		if _, err := adapter.Poll(context.Background(), contract.PollRequest{Handle: contract.OperationHandle{ProviderRef: ref}, Connection: f.Connection()}); err == nil {
			t.Errorf("garbage traffic handle %q polled without error, want rejection", ref)
		}
	}
}
