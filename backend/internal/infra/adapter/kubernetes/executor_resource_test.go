// executor_resource_test — P2-C: resource mutation 2종(k8s.resource.apply update
// 한정·k8s.resource.delete)의 Execute·Poll 계약 테스트. 왕복 하네스 소비는
// TestOperationContractRoundtrip(executor_workload_test.go — 8종으로 확장)이
// 담당하고, 본 파일은 resource 가족 고유의 판정면(동결 manifest 와이어·부분집합
// 상태 에코·404 종단·버전 폴백)을 단얫한다. 모의 서버·fixture는
// executor_resource_mock_test.go가 담당한다.
package kubernetes

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/contract"
)

// --- Execute — k8s.resource.apply 계약. ---

// TestExecuteApplyPutsFrozenManifest — GET(resourceVersion 동결)→PUT 1회: 경로·
// Content-Type이 v1 UpdateK8sResourceYAML 와이어와 동일하고, 본문 metadata에는
// 관측 resourceVersion이 주입된다. handle은 state 가족 형태로 주입 전 동결
// manifest를 자기서술한다(RV 무관 — provider_frozen_manifest). 재실행은 동일
// 목표 manifest의 재 PUT이고 실변경이 없으므로 2·3차 본문은 byte-identical,
// generation 무증가다.
func TestExecuteApplyPutsFrozenManifest(t *testing.T) {
	f := newResourceFixtures(t)[ApplyOperationName]

	handle, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	puts := f.mock.recordsOf(http.MethodPut)
	if len(puts) != 1 {
		t.Fatalf("put count = %d, want 1", len(puts))
	}
	if want := f.mock.cmBasePath(); puts[0].Path != want {
		t.Errorf("put path = %q, want %q", puts[0].Path, want)
	}
	if want := "application/json"; puts[0].ContentType != want {
		t.Errorf("put Content-Type = %q, want %q (v1 wire)", puts[0].ContentType, want)
	}
	var body map[string]any
	if err := json.Unmarshal(puts[0].Body, &body); err != nil {
		t.Fatalf("put body decode: %v\nbody: %s", err, puts[0].Body)
	}
	if body["apiVersion"] != "v1" || body["kind"] != "ConfigMap" {
		t.Errorf("put envelope = %v/%v, want v1/ConfigMap", body["apiVersion"], body["kind"])
	}
	metadata := body["metadata"].(map[string]any)
	if metadata["resourceVersion"] != "100" {
		t.Errorf("put resourceVersion = %v, want the observed %q injected (v1 wire)", metadata["resourceVersion"], "100")
	}
	if !sameJSON(metadata["labels"], map[string]any{"app": "seed"}) {
		t.Errorf("put labels = %v", metadata["labels"])
	}
	if !sameJSON(body["data"], map[string]any{"level": "info"}) {
		t.Errorf("put data = %v, want the frozen manifest's data (level: info)", body["data"])
	}

	// handle — state 가족: 주입 전 동결 manifest(RV 키 부재)를 자기서술한다.
	got := mustDecodeStateHandle(t, handle.ProviderRef)
	if got.ConnectionUID != execConnUID || got.APIPath != f.mock.cmBasePath() {
		t.Errorf("state handle = %+v, want connUID %q apiPath %q", got, execConnUID, f.mock.cmBasePath())
	}
	if got.Expectation.Resource != "manifest" {
		t.Errorf("expectation resource = %q, want manifest", got.Expectation.Resource)
	}
	if manifestMetadata, ok := got.Expectation.Manifest["metadata"].(map[string]any); ok {
		if _, has := manifestMetadata["resourceVersion"]; has {
			t.Error("frozen manifest carries resourceVersion — the expectation must freeze the manifest pre-injection (version-agnostic replay)")
		}
	} else {
		t.Errorf("frozen manifest metadata = %v, want a map", got.Expectation.Manifest["metadata"])
	}

	// 재실행 — RV만 현행 관측을 따라가고, 실변경이 없으면(2차 이후) 본문이
	// byte-identical로 수렴한다. 어느 재실행도 generation을 올리지 않는다.
	second, err := NewAdapter().Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("second Execute (frozen-manifest replay): %v", err)
	}
	third, err := NewAdapter().Execute(context.Background(), f.Request())
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
		t.Error("replay reuses the execute-time resourceVersion — the mock state should have advanced to 101")
	}
	delete(firstMeta, "resourceVersion")
	delete(secondMeta, "resourceVersion")
	if !sameJSON(firstBody, secondBody) {
		t.Errorf("replay differs beyond resourceVersion:\nfirst:  %s\nsecond: %s", puts[0].Body, puts[1].Body)
	}
	if string(puts[1].Body) != string(puts[2].Body) {
		t.Errorf("stable replay is not byte-identical:\nsecond: %s\nthird:  %s", puts[1].Body, puts[2].Body)
	}
	if second.ProviderRef != handle.ProviderRef || third.ProviderRef != handle.ProviderRef {
		t.Errorf("replay handle changed: %q → %q → %q", handle.ProviderRef, second.ProviderRef, third.ProviderRef)
	}
	if n := f.mock.cmGeneration(); n != 2 {
		t.Errorf("configmap generation after replays = %d, want 2 (identical manifest must not bump)", n)
	}
}

// TestExecuteApplyValidationRejects — 검증 실패는 HTTP write 0회. 신원 가드(kind·
// name·namespace·cluster-scoped)와 URN 면(미서빙 singular·uid 신원 종·불량
// subtype·부재 목표)이 전부 PUT 전에 거부된다.
func TestExecuteApplyValidationRejects(t *testing.T) {
	f := newResourceFixtures(t)[ApplyOperationName]
	for i, bad := range f.BadRequests() {
		if _, err := NewAdapter().Execute(context.Background(), bad); err == nil {
			t.Fatalf("apply bad request #%d (%v) succeeded, want error", i, bad.ResourceURN)
		}
	}
	if n := f.mock.mutationCount(); n != 0 {
		t.Errorf("validation failures issued %d mutation(s), want 0", n)
	}
}

// TestExecuteApplyGatewayVersionFallback — GatewayAPI 2종의 버전 폴백: v1 후보가
// 404면 v1beta1 후보에 PUT하고, handle은 발견된 v1beta1 경로를 자기서술한다(수집기
// sectionFallbacks — J-P1-1 — 와 동일 선호의 쓰기 면).
func TestExecuteApplyGatewayVersionFallback(t *testing.T) {
	fixtures := newResourceFixtures(t)
	f := fixtures[ApplyOperationName]
	f.payload = contract.JSONMap{"yaml": resourceFixtureRouteYAML}
	request := f.Request()
	request.ResourceURN = "urn:k8s:3:httproute:" + execNamespace + "/seed-route"

	handle, err := NewAdapter().Execute(context.Background(), request)
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
	if got := mustDecodeStateHandle(t, handle.ProviderRef); got.APIPath != f.mock.routeBasePath() {
		t.Errorf("handle apiPath = %q, want the resolved v1beta1 path %q", got.APIPath, f.mock.routeBasePath())
	}
	status, err := NewAdapter().Poll(context.Background(), contract.PollRequest{Handle: handle, Connection: f.Connection()})
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	assertManifestSucceeded(t, status, f.mock.srv.URL)
}

// --- Execute — k8s.resource.delete 계약. ---

// TestExecuteDeleteThenPollTerminal404 — DELETE 1회 → 폴 404가 성공 종단
// (provider_terminal_404 — client.go errNotFound 재사용, P1-C 판단 6). 재실행
// DELETE의 404는 실패가 아니라 이미 도달한 목표 상태의 확인이다(동일 handle).
// 부재가 아니면 Running(다음 폴)이다.
func TestExecuteDeleteThenPollTerminal404(t *testing.T) {
	adapter := NewAdapter()
	f := newResourceFixtures(t)[DeleteOperationName]

	handle, err := adapter.Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	deletes := f.mock.recordsOf(http.MethodDelete)
	if len(deletes) != 1 {
		t.Fatalf("delete count = %d, want 1", len(deletes))
	}
	if want := f.mock.cmBasePath(); deletes[0].Path != want {
		t.Errorf("delete path = %q, want %q", deletes[0].Path, want)
	}
	if got := mustDecodeStateHandle(t, handle.ProviderRef); got.Expectation.Resource != "delete" || got.APIPath != f.mock.cmBasePath() {
		t.Errorf("state handle = %+v, want delete expectation on %q", got, f.mock.cmBasePath())
	}

	// 폴 — 부재면 즉시 성공 종단(detail은 deleteResultRedaction 1키).
	status, err := adapter.Poll(context.Background(), contract.PollRequest{Handle: handle, Connection: f.Connection()})
	assertDeleteSucceeded(t, status, f.mock.srv.URL, err)

	// 재실행(크래시 → 리퍼 재큐 경로) — DELETE 404여도 handle은 동일하다
	// (provider_terminal_404 멱등).
	replay, err := adapter.Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("replay Execute after deletion: %v", err)
	}
	if replay.ProviderRef != handle.ProviderRef {
		t.Errorf("replay handle = %q, want the original %q (terminal 404 is the goal state)", replay.ProviderRef, handle.ProviderRef)
	}

	// Running 경로 — 부재가 아니면 다음 폴을 기다린다(detail은 terminal-only).
	running := newResourceFixtures(t)[DeleteOperationName]
	runningHandle, err := adapter.Execute(context.Background(), running.Request())
	if err != nil {
		t.Fatalf("running-path Execute: %v", err)
	}
	running.mock.restoreCM()
	status, err = adapter.Poll(context.Background(), contract.PollRequest{Handle: runningHandle, Connection: running.Connection()})
	if err != nil {
		t.Fatalf("running-path Poll: %v", err)
	}
	if status.State != contract.OperationStateRunning || status.Detail != nil {
		t.Errorf("poll with the object restored = {%v %v}, want running with no detail", status.State, status.Detail)
	}
	running.mock.removeCM()
	status, err = adapter.Poll(context.Background(), contract.PollRequest{Handle: runningHandle, Connection: running.Connection()})
	assertDeleteSucceeded(t, status, running.mock.srv.URL, err)

	// J12 — 다른 커넥션의 폴은 조립 버그로 즉시 거부된다.
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

// TestExecuteDeleteValidationRejects — URN 면 밖 목표는 DELETE 0회로 거부된다.
func TestExecuteDeleteValidationRejects(t *testing.T) {
	f := newResourceFixtures(t)[DeleteOperationName]
	for i, bad := range f.BadRequests() {
		if _, err := NewAdapter().Execute(context.Background(), bad); err == nil {
			t.Fatalf("delete bad request #%d (%s) succeeded, want error", i, bad.ResourceURN)
		}
	}
	if n := f.mock.mutationCount(); n != 0 {
		t.Errorf("validation failures issued %d mutation(s), want 0", n)
	}
}

// --- Poll — resource 가족의 판정면. ---

// TestPollManifestEchoRunningUntilConverged — 동결 manifest 부분집합 판정의
// 반증 가능성: 관측에서 manifest 성분이 사라지면 Running(detail 없음), 회복되면
// Succeeded(stateResultRedaction 2키)로 수렴한다.
func TestPollManifestEchoRunningUntilConverged(t *testing.T) {
	adapter := NewAdapter()
	f := newResourceFixtures(t)[ApplyOperationName]

	handle, err := adapter.Execute(context.Background(), f.Request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	f.mock.dropCMData()
	status, err := adapter.Poll(context.Background(), contract.PollRequest{Handle: handle, Connection: f.Connection()})
	if err != nil {
		t.Fatalf("Poll (tampered): %v", err)
	}
	if status.State != contract.OperationStateRunning || status.Detail != nil {
		t.Errorf("poll with the manifest data removed = {%v %v}, want running with no detail", status.State, status.Detail)
	}
	f.mock.setCMData(map[string]any{"level": "info"})
	status, err = adapter.Poll(context.Background(), contract.PollRequest{Handle: handle, Connection: f.Connection()})
	if err != nil {
		t.Fatalf("Poll (recovered): %v", err)
	}
	assertManifestSucceeded(t, status, f.mock.srv.URL)

	// J12 — 다른 커넥션의 폴은 조립 버그로 즉시 거부된다.
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

// TestPollResourceHandleRejectsGarbage — 손상된 resource handle은 폴 전에 거부된다
// (J2 — handle은 task_attempt.handle_ref에 지속되므로 변형 입력에 닫혀 있다).
func TestPollResourceHandleRejectsGarbage(t *testing.T) {
	adapter := NewAdapter()
	f := newResourceFixtures(t)[ApplyOperationName]
	for _, garbage := range stateHandleGarbageCases() {
		if _, err := adapter.Poll(context.Background(), contract.PollRequest{Handle: contract.OperationHandle{ProviderRef: garbage}, Connection: f.Connection()}); err == nil {
			t.Errorf("garbage resource handle %q polled without error, want rejection", garbage)
		}
	}
}

// TestResourceCandidatePaths — 종별 경로 후보 표(v1 buildK8sYAMLResourcePath 면 +
// GatewayAPI 2후보). 클러스터 스코프 2종·batch 2종·apps 4종·core 4종·GatewayAPI
// 2종의 경로 형태를 고정한다.
func TestResourceCandidatePaths(t *testing.T) {
	cases := []struct {
		target resourceTarget
		want   []string
	}{
		{resourceTarget{Singular: "namespace", Name: "seed"}, []string{"/api/v1/namespaces/seed"}},
		{resourceTarget{Singular: "pv", Name: "seed-pv"}, []string{"/api/v1/persistentvolumes/seed-pv"}},
		{resourceTarget{Singular: "workload", Subtype: "deployment", Namespace: "ns", Name: "web"}, []string{"/apis/apps/v1/namespaces/ns/deployments/web"}},
		{resourceTarget{Singular: "workload", Subtype: "replicaset", Namespace: "ns", Name: "web"}, []string{"/apis/apps/v1/namespaces/ns/replicasets/web"}},
		{resourceTarget{Singular: "workload", Subtype: "job", Namespace: "ns", Name: "seed-job"}, []string{"/apis/batch/v1/namespaces/ns/jobs/seed-job"}},
		{resourceTarget{Singular: "workload", Subtype: "cronjob", Namespace: "ns", Name: "seed-cron"}, []string{"/apis/batch/v1/namespaces/ns/cronjobs/seed-cron"}},
		{resourceTarget{Singular: "service", Namespace: "ns", Name: "svc"}, []string{"/api/v1/namespaces/ns/services/svc"}},
		{resourceTarget{Singular: "ingress", Namespace: "ns", Name: "ing"}, []string{"/apis/networking.k8s.io/v1/namespaces/ns/ingresses/ing"}},
		{resourceTarget{Singular: "configmap", Namespace: "ns", Name: "cm"}, []string{"/api/v1/namespaces/ns/configmaps/cm"}},
		{resourceTarget{Singular: "secret", Namespace: "ns", Name: "sec"}, []string{"/api/v1/namespaces/ns/secrets/sec"}},
		{resourceTarget{Singular: "pvc", Namespace: "ns", Name: "claim"}, []string{"/api/v1/namespaces/ns/persistentvolumeclaims/claim"}},
		{resourceTarget{Singular: "gateway", Namespace: "ns", Name: "gw"}, []string{
			"/apis/gateway.networking.k8s.io/v1/namespaces/ns/gateways/gw",
			"/apis/gateway.networking.k8s.io/v1beta1/namespaces/ns/gateways/gw",
		}},
		{resourceTarget{Singular: "httproute", Namespace: "ns", Name: "route"}, []string{
			"/apis/gateway.networking.k8s.io/v1/namespaces/ns/httproutes/route",
			"/apis/gateway.networking.k8s.io/v1beta1/namespaces/ns/httproutes/route",
		}},
	}
	for _, c := range cases {
		got := resourceCandidatePaths(c.target)
		if !sameJSON(got, c.want) {
			t.Errorf("resourceCandidatePaths(%+v) = %v, want %v", c.target, got, c.want)
		}
	}
}

// --- 단얫 헬퍼. ---

func assertManifestSucceeded(t *testing.T, status contract.OperationStatus, serverURL string) {
	t.Helper()
	if status.State != contract.OperationStateSucceeded {
		t.Fatalf("poll state = %v, want succeeded", status.State)
	}
	// apply의 detail은 stateResultRedaction과 1:1인 2키다(generation이 없는 종은
	// 부재 관측값 0 — rollout 가족 restartedAt 공백 선례).
	assertKeySet(t, "manifest poll detail", status.Detail, "generation", "serverURL")
	assertServerURL(t, status.Detail)
	if got, _ := status.Detail["serverURL"].(string); got != serverURL {
		t.Errorf("detail serverURL = %q, want %q", got, serverURL)
	}
}

func assertDeleteSucceeded(t *testing.T, status contract.OperationStatus, serverURL string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if status.State != contract.OperationStateSucceeded {
		t.Fatalf("poll state = %v, want succeeded (404 is the provider_terminal_404 terminal)", status.State)
	}
	// delete의 detail은 deleteResultRedaction과 1:1인 1키다 — 부재 판정에는 관측
	// 성분이 없다.
	assertKeySet(t, "delete poll detail", status.Detail, "serverURL")
	if got, _ := status.Detail["serverURL"].(string); got != serverURL {
		t.Errorf("detail serverURL = %q, want %q", got, serverURL)
	}
}
