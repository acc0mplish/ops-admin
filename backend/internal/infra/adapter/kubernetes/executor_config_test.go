// executor_config_test — P2-B: state-convergent mutation 2종(node
// labels_update·service update)의 Execute·Poll 계약 테스트. 왕복 하네스 소비는
// TestOperationContractRoundtrip(executor_workload_test.go — 6종으로 확장)이
// 담당하고, 본 파일은 발행+poll 1회 가족 고유의 판정면(상태 에코·재실행 수렴)을
// 단얫한다. 모의 서버·fixture는 executor_config_mock_test.go가 담당한다.
package kubernetes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/contract"
)

// --- Execute — node labels_update 계약. ---

// TestExecuteNodeLabelsUpdatePatchesLabels — LIST(이름 확정)→merge-patch 1회:
// 경로·Content-Type이 v1 UpdateK8sNodeLabels와 동일하고, 본문은 payload에 없는
// 기존 레이블을 null로 제거한다(v1 경험식). handle은 state 가족 형태로 기대
// 레이블을 자기서술한다. 재실행은 null 제거항이 소멸한 뒤 byte-identical이 되고
// generation 무증가다(provider_state_convergent).
func TestExecuteNodeLabelsUpdatePatchesLabels(t *testing.T) {
	f := newConfigFixtures(t)[NodeLabelsUpdateOperationName]

	handle, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if n := f.mock.patchCount(); n != 1 {
		t.Fatalf("patch count = %d, want 1", n)
	}
	p := f.mock.patches[0]
	if want := f.mock.nodeBasePath(); p.Path != want {
		t.Errorf("patch path = %q, want %q (node name resolved from the urn uid)", p.Path, want)
	}
	if want := "application/merge-patch+json"; p.ContentType != want {
		t.Errorf("patch Content-Type = %q, want %q", p.ContentType, want)
	}
	var body struct {
		Metadata struct {
			Labels map[string]any `json:"labels"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(p.Body, &body); err != nil {
		t.Fatalf("patch body decode: %v\nbody: %s", err, p.Body)
	}
	labels := body.Metadata.Labels
	if labels["env"] != "prod" || labels["region"] != "kr" {
		t.Errorf("patched labels = %v, want env=prod region=kr", labels)
	}
	if _, has := labels["kubernetes.io/hostname"]; !has {
		t.Errorf("patched labels = %v, want a null entry removing kubernetes.io/hostname (labels absent from the payload)", labels)
	} else if labels["kubernetes.io/hostname"] != nil {
		t.Errorf("kubernetes.io/hostname patch = %v, want null (v1 removal semantics)", labels["kubernetes.io/hostname"])
	}

	// handle — state 가족: 마커·connUID·apiPath·기대 레이블 자기서술.
	got, err := decodeStateRef(handle.ProviderRef)
	if err != nil {
		t.Fatalf("decodeStateRef(%q): %v", handle.ProviderRef, err)
	}
	if got.ConnectionUID != execConnUID || got.APIPath != f.mock.nodeBasePath() {
		t.Errorf("state handle = %+v, want connUID %q apiPath %q", got, execConnUID, f.mock.nodeBasePath())
	}
	if got.Expectation.Resource != "node" || len(got.Expectation.Labels) != 2 ||
		got.Expectation.Labels["env"] != "prod" || got.Expectation.Labels["region"] != "kr" {
		t.Errorf("state expectation = %+v, want node labels env=prod region=kr", got.Expectation)
	}

	// 재실행 — 첫 재실행만 null 제거항이 소멸하고, 그 뒤 재실행은 byte-identical.
	// 어느 재실행도 실변경이 없다(null·동일 값) → generation 무증가.
	first := canonical(f.mock.patches[0].Body)
	second, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("second Execute (state-convergent replay): %v", err)
	}
	if first == canonical(f.mock.patches[1].Body) {
		t.Error("replay patch equals the first — the null removal entries should have been retired by the applied state")
	}
	third, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("third Execute (stable replay): %v", err)
	}
	if canonical(f.mock.patches[1].Body) != canonical(f.mock.patches[2].Body) {
		t.Errorf("stable replay differs:\nsecond: %s\nthird:  %s", f.mock.patches[1].Body, f.mock.patches[2].Body)
	}
	if second.ProviderRef != handle.ProviderRef || third.ProviderRef != handle.ProviderRef {
		t.Errorf("replay handle changed: %q → %q → %q", handle.ProviderRef, second.ProviderRef, third.ProviderRef)
	}
	if n := f.mock.nodeGenerationOf(); n != 2 {
		t.Errorf("node generation after replays = %d, want 2 (identical label state must not bump)", n)
	}
}

// TestExecuteNodeLabelsValidationRejects — 검증 실패는 patch 0발행. 미지 uid는
// LIST 후 거부된다(v1의 GET 실패 거부와 동일 위치 — HTTP 호출 후 patch 전).
func TestExecuteNodeLabelsValidationRejects(t *testing.T) {
	f := newConfigFixtures(t)[NodeLabelsUpdateOperationName]
	for _, bad := range f.BadRequests() {
		_, err := NewAdapter().Execute(context.Background(), bad)
		if err == nil {
			t.Fatalf("node request %v succeeded, want error", bad.Payload)
		}
	}
	if n := f.mock.patchCount(); n != 0 {
		t.Errorf("validation failures issued %d patch(es), want 0", n)
	}
}

// --- Execute — service update 계약. ---

// TestExecuteServiceUpdatePatchesService — GET(현행)→wholesale-spec merge-patch:
// type·selector·ports·labels·annotations 재기록, clusterIP 보존, resourceVersion
// 현행값 보존(v1 와이어 동일), targetPort 문자열·숫자 폴백, protocol 기본값,
// nodePort는 NodePort에서만. 재실행 본문은 metadata.resourceVersion만 간다(모의가
// 실제 변경으로 올린 값 — v1의 현행값 보존 와이어) — 나머지는 byte-identical이고
// generation 무증가.
func TestExecuteServiceUpdatePatchesService(t *testing.T) {
	f := newConfigFixtures(t)[ServiceUpdateOperationName]

	handle, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if n := f.mock.patchCount(); n != 1 {
		t.Fatalf("patch count = %d, want 1", n)
	}
	p := f.mock.patches[0]
	if want := f.mock.serviceBasePath(); p.Path != want {
		t.Errorf("patch path = %q, want %q", p.Path, want)
	}
	if want := "application/merge-patch+json"; p.ContentType != want {
		t.Errorf("patch Content-Type = %q, want %q", p.ContentType, want)
	}
	var body map[string]any
	if err := json.Unmarshal(p.Body, &body); err != nil {
		t.Fatalf("patch body decode: %v\nbody: %s", err, p.Body)
	}
	if body["apiVersion"] != "v1" || body["kind"] != "Service" {
		t.Errorf("patch envelope = %v/%v, want v1/Service", body["apiVersion"], body["kind"])
	}
	metadata := body["metadata"].(map[string]any)
	if metadata["resourceVersion"] != "100" {
		t.Errorf("patched resourceVersion = %v, want the observed %q preserved (v1 wire)", metadata["resourceVersion"], "100")
	}
	if !sameJSON(metadata["labels"], map[string]any{"app": "web", "patched": "yes"}) {
		t.Errorf("patched labels = %v", metadata["labels"])
	}
	if !sameJSON(metadata["annotations"], map[string]any{"note": "hello"}) {
		t.Errorf("patched annotations = %v", metadata["annotations"])
	}
	spec := body["spec"].(map[string]any)
	if spec["type"] != "NodePort" {
		t.Errorf("patched type = %v, want NodePort", spec["type"])
	}
	if spec["clusterIP"] != "10.96.0.10" {
		t.Errorf("patched spec carries clusterIP = %v, want the observed value preserved by the spec copy", spec["clusterIP"])
	}
	if !sameJSON(spec["selector"], map[string]any{"app": "web", "tier": "api"}) {
		t.Errorf("patched selector = %v", spec["selector"])
	}
	ports, ok := spec["ports"].([]any)
	if !ok || len(ports) != 2 {
		t.Fatalf("patched ports = %v, want 2 entries", spec["ports"])
	}
	first := ports[0].(map[string]any)
	if !sameJSON(first, map[string]any{"port": 80.0, "protocol": "TCP", "targetPort": "web", "nodePort": 30080.0}) {
		t.Errorf("port entry 1 = %v, want 80/TCP/web/nodePort 30080", first)
	}
	second := ports[1].(map[string]any)
	if !sameJSON(second, map[string]any{"port": 8080.0, "protocol": "TCP", "targetPort": 8080.0}) {
		t.Errorf("port entry 2 = %v, want 8080/TCP/8080 (numeric fallback, no nodePort)", second)
	}
	if n := f.mock.generationOf(); n != 2 {
		t.Errorf("service generation after patch = %d, want 2 (real change bumps once)", n)
	}

	// 재실행 — 동일 값의 재 upsert: resourceVersion만 간다.
	if _, err := NewAdapter().Execute(context.Background(), f.Request()); err != nil {
		t.Fatalf("second Execute (state-convergent replay): %v", err)
	}
	var firstBody, secondBody map[string]any
	if err := json.Unmarshal(f.mock.patches[0].Body, &firstBody); err != nil {
		t.Fatalf("first patch decode: %v", err)
	}
	if err := json.Unmarshal(f.mock.patches[1].Body, &secondBody); err != nil {
		t.Fatalf("replay patch decode: %v", err)
	}
	firstMeta := firstBody["metadata"].(map[string]any)
	secondMeta := secondBody["metadata"].(map[string]any)
	if firstMeta["resourceVersion"] == secondMeta["resourceVersion"] {
		t.Error("replay reuses the execute-time resourceVersion — the mock state should have advanced")
	}
	delete(firstMeta, "resourceVersion")
	delete(secondMeta, "resourceVersion")
	if !sameJSON(firstBody, secondBody) {
		t.Errorf("replay differs beyond resourceVersion:\nfirst:  %s\nsecond: %s", f.mock.patches[0].Body, f.mock.patches[1].Body)
	}
	if n := f.mock.generationOf(); n != 2 {
		t.Errorf("service generation after replay = %d, want 2 (identical upsert must not bump)", n)
	}
	if exp, err := decodeStateRef(handle.ProviderRef); err != nil || exp.Expectation.Type != "NodePort" {
		t.Errorf("state expectation = %+v (err %v), want service type NodePort", exp, err)
	}
}

// TestExecuteServiceUpdateValidationRejects — v1 검증 문언 그대로(k8s 접두)·검증은
// HTTP 전이라 patch 0발행.
func TestExecuteServiceUpdateValidationRejects(t *testing.T) {
	f := newConfigFixtures(t)[ServiceUpdateOperationName]
	want := []string{
		"at least one service port is required",
		"unsupported service type",
		"headless service must use ClusterIP",
		"external name is required",
		"service port must be between 1 and 65535",
		"service port must be between 1 and 65535",
	}
	for i, bad := range f.BadRequests() {
		_, err := NewAdapter().Execute(context.Background(), bad)
		if err == nil {
			t.Fatalf("service bad request #%d %v succeeded, want error", i, bad.Payload)
		}
		if !strings.Contains(err.Error(), want[i]) {
			t.Errorf("service bad request #%d error = %v, want it to contain %q (v1 wording)", i, err, want[i])
		}
	}
	if n := f.mock.patchCount(); n != 0 {
		t.Errorf("validation failures issued %d patch(es), want 0", n)
	}
}

// --- Poll — state handle 가족의 판정면. ---

// TestPollStateHandleRunningUntilConverged — 상태 에코 판정의 반증 가능성: 관측이
// 기대에서 벗어나면 Running(detail 없음)으로 돌아가고, 상태가 회복되면
// Succeeded(stateResultRedaction 2키)로 수렴한다. J12 정합 가드도 state 경로에서
// 동일하게 작동한다.
func TestPollStateHandleRunningUntilConverged(t *testing.T) {
	adapter := NewAdapter()

	nodeFixtures := newConfigFixtures(t)
	nodeHandle, err := adapter.Execute(context.Background(), nodeFixtures[NodeLabelsUpdateOperationName].Request())
	if err != nil {
		t.Fatalf("node Execute: %v", err)
	}
	nodeMock := nodeFixtures[NodeLabelsUpdateOperationName].mock
	nodeMock.removeNodeLabel("region")
	status, err := adapter.Poll(context.Background(), contract.PollRequest{Handle: nodeHandle, Connection: nodeFixtures[NodeLabelsUpdateOperationName].Connection()})
	if err != nil {
		t.Fatalf("node Poll: %v", err)
	}
	if status.State != contract.OperationStateRunning || status.Detail != nil {
		t.Errorf("node poll with a missing label = {%v %v}, want running with no detail", status.State, status.Detail)
	}
	nodeMock.setNodeLabel("region", "kr")
	status, err = adapter.Poll(context.Background(), contract.PollRequest{Handle: nodeHandle, Connection: nodeFixtures[NodeLabelsUpdateOperationName].Connection()})
	if err != nil {
		t.Fatalf("node Poll (recovered): %v", err)
	}
	assertStateSucceeded(t, status)

	serviceFixtures := newConfigFixtures(t)
	serviceHandle, err := adapter.Execute(context.Background(), serviceFixtures[ServiceUpdateOperationName].Request())
	if err != nil {
		t.Fatalf("service Execute: %v", err)
	}
	serviceMock := serviceFixtures[ServiceUpdateOperationName].mock
	serviceMock.setServiceType("ClusterIP")
	status, err = adapter.Poll(context.Background(), contract.PollRequest{Handle: serviceHandle, Connection: serviceFixtures[ServiceUpdateOperationName].Connection()})
	if err != nil {
		t.Fatalf("service Poll: %v", err)
	}
	if status.State != contract.OperationStateRunning || status.Detail != nil {
		t.Errorf("service poll with a regressed type = {%v %v}, want running with no detail", status.State, status.Detail)
	}
	serviceMock.setServiceType("NodePort")
	status, err = adapter.Poll(context.Background(), contract.PollRequest{Handle: serviceHandle, Connection: serviceFixtures[ServiceUpdateOperationName].Connection()})
	if err != nil {
		t.Fatalf("service Poll (recovered): %v", err)
	}
	assertStateSucceeded(t, status)

	// J12 — 다른 커넥션의 폴은 조립 버그로 즉시 거부된다.
	_, err = adapter.Poll(context.Background(), contract.PollRequest{
		Handle: serviceHandle,
		Connection: contract.ConnectionView{
			UID:          "conn-uid-other",
			ProviderType: ProviderName,
			Material:     serviceFixtures[ServiceUpdateOperationName].Connection().Material,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "assembly bug") {
		t.Errorf("cross-connection poll error = %v, want the J12 assembly-bug guard", err)
	}
}

func assertStateSucceeded(t *testing.T, status contract.OperationStatus) {
	t.Helper()
	if status.State != contract.OperationStateSucceeded {
		t.Fatalf("poll state = %v, want succeeded", status.State)
	}
	// state 가족의 detail은 stateResultRedaction(compose.go)과 1:1인 2키다 —
	// rollout 5키 단얫(assertDetailKeys)과 다른 허용 집합이다.
	assertKeySet(t, "state poll detail", status.Detail, "generation", "serverURL")
	assertServerURL(t, status.Detail)
}

// TestPollStateHandleRejectsGarbage — 손상된 state handle은 폴 전에 거부된다
// (J2 — handle은 task_attempt.handle_ref에 지속되므로 변형 입력에 닫혀 있다).
func TestPollStateHandleRejectsGarbage(t *testing.T) {
	adapter := NewAdapter()
	f := newConfigFixtures(t)[NodeLabelsUpdateOperationName]
	conn := f.Connection()
	for _, garbage := range []string{
		"state|conn|/api/v1/nodes/x",      // 세그먼트 3
		"state|conn|notaapi|AAAA",         // apiPath 접두 위반
		"state|conn|/api/v1/nodes/x|!!!!", // base64url 위반
		"state|conn|/api/v1/nodes/x|e30",  // expectation resource 부재({})
		"state||/api/v1/nodes/x|e30",      // connUID 부재
	} {
		if _, err := adapter.Poll(context.Background(), contract.PollRequest{Handle: contract.OperationHandle{ProviderRef: garbage}, Connection: conn}); err == nil {
			t.Errorf("garbage state handle %q polled without error, want rejection", garbage)
		}
	}
}
