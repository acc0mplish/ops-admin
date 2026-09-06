// executor_test — Phase 3 B(N2): k8s workload restart executor의 httptest 대조군
// (계획 §5 Phase B·§6 adapter 계약). 모의 API 서버는 strategic-merge-patch를 실제
// k8s 시맨킹(A1 — 동일 어노테이션 값의 재적용은 spec 무변화 → generation 무증가)으로
// 반영해, 재실행 patch byte 동일(J1 멱등 근거)과 교정 판정식 3종(J2 r2)을
// 실클러스터 없이 실증한다.
package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"ops-admin/backend/internal/infra/contract"
)

// --- 모의 롤아웃 API 서버 — 단일 워크로드를 보유한 상태 저장 대조군. ---

const (
	execConnUID            = "conn-uid-exec-1"
	execNamespace          = "v2-seed"
	execWorkloadName       = "seed-web"
	frozenRestartedAtValue = "2026-09-07T01:02:03Z" // J1 — 서버 동결값(재실행에도 불변)
	kubeconfigTokenStr     = "test-token"           // kubeconfigFor가 심는 bearer — 누출 스캔 표적(VK-10)
)

type rolloutPatchRecord struct {
	Path        string
	ContentType string
	Body        []byte
}

type rolloutMock struct {
	t   *testing.T
	srv *httptest.Server

	mu      sync.Mutex
	mode    string // "" | 신호 | not_found
	kind    string
	obj     map[string]any
	meta    map[string]any
	ann     map[string]any
	status  map[string]any
	patches []rolloutPatchRecord
}

func newRolloutMock(t *testing.T, kind string, replicas *int) *rolloutMock {
	m := &rolloutMock{t: t, kind: kind}
	m.ann = map[string]any{}
	m.status = map[string]any{}
	m.meta = map[string]any{
		"name":       execWorkloadName,
		"namespace":  execNamespace,
		"generation": 1, // §0.1 기선 — 시드는 generation 1·어노테이션 없음
	}
	spec := map[string]any{
		"template": map[string]any{
			"metadata": map[string]any{"annotations": m.ann},
		},
	}
	if replicas != nil {
		spec["replicas"] = *replicas
	}
	m.obj = map[string]any{"metadata": m.meta, "spec": spec, "status": m.status}
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.srv.Close)
	return m
}

func (m *rolloutMock) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch m.mode {
	case contract.SignalRateLimited:
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		return
	case contract.SignalPermissionDenied:
		w.WriteHeader(http.StatusForbidden)
		return
	case "not_found":
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if want := "/apis/apps/v1/namespaces/" + execNamespace + "/" + m.kind + "s/" + execWorkloadName; r.URL.Path != want {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodPatch:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			m.t.Fatalf("rolloutMock: read patch body: %v", err)
		}
		m.patches = append(m.patches, rolloutPatchRecord{
			Path:        r.URL.Path,
			ContentType: r.Header.Get("Content-Type"),
			Body:        body,
		})
		var patch struct {
			Spec struct {
				Template struct {
					Metadata struct {
						Annotations map[string]string `json:"annotations"`
					} `json:"metadata"`
				} `json:"template"`
			} `json:"spec"`
		}
		if err := json.Unmarshal(body, &patch); err != nil {
			m.t.Fatalf("rolloutMock: decode patch body: %v", err)
		}
		// A1 시맨틱: 값이 실제로 바뀌는 어노테이션만 spec 변경 → generation 증가.
		changed := false
		for k, v := range patch.Spec.Template.Metadata.Annotations {
			if m.ann[k] != v {
				m.ann[k] = v
				changed = true
			}
		}
		if changed {
			m.meta["generation"] = m.meta["generation"].(int) + 1
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(m.obj)
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(m.obj)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (m *rolloutMock) setMode(mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mode = mode
}

// setStatus replaces the workload status map — 판정식 경계 시나리오 주입면.
func (m *rolloutMock) setStatus(fields map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.status {
		delete(m.status, k)
	}
	for k, v := range fields {
		m.status[k] = v
	}
}

func (m *rolloutMock) generation() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int64(m.meta["generation"].(int))
}

func (m *rolloutMock) patchCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.patches)
}

// --- Fixture — adapter·모의 서버·실행 자격(Material["operations"])의 결합. ---

type executorFixture struct {
	t       *testing.T
	mock    *rolloutMock
	adapter *Adapter
}

func newExecutorFixture(t *testing.T, kind string, replicas *int) *executorFixture {
	return &executorFixture{t: t, mock: newRolloutMock(t, kind, replicas), adapter: NewAdapter()}
}

func (f *executorFixture) Connection() contract.ConnectionView {
	return contract.ConnectionView{
		UID:          execConnUID,
		ProviderType: ProviderName,
		Material:     map[string]string{contract.CredentialPurposeOperations: kubeconfigFor(f.mock.srv.URL)},
	}
}

func (f *executorFixture) request() contract.OperationRequest {
	return contract.OperationRequest{
		OperationName: RestartOperationName,
		ResourceURN:   "urn:k8s:3:workload:" + execNamespace + "/" + f.mock.kind + "/" + execWorkloadName,
		Payload:       contract.JSONMap{"restartedAt": frozenRestartedAtValue},
		Connection:    f.Connection(),
	}
}

func (f *executorFixture) poll(handle contract.OperationHandle) (contract.OperationStatus, error) {
	return f.adapter.Poll(context.Background(), contract.PollRequest{Handle: handle, Connection: f.Connection()})
}

// --- Execute — PATCH 계약 (§3.2). ---

// TestExecuteSendsFrozenAnnotationPatch — PATCH 1회: 경로·Content-Type·본문이
// 정확히 pod template 어노테이션 1개의 strategic-merge-patch이고, handle이 patch
// 응답의 metadata.generation을 expectGeneration로 인코딩한다(J2).
func TestExecuteSendsFrozenAnnotationPatch(t *testing.T) {
	replicas := 3
	f := newExecutorFixture(t, "deployment", &replicas)

	handle, err := f.adapter.Execute(context.Background(), f.request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if n := f.mock.patchCount(); n != 1 {
		t.Fatalf("patch count = %d, want 1", n)
	}
	p := f.mock.patches[0]
	if want := "/apis/apps/v1/namespaces/" + execNamespace + "/deployments/" + execWorkloadName; p.Path != want {
		t.Errorf("patch path = %q, want %q", p.Path, want)
	}
	if want := "application/strategic-merge-patch+json"; p.ContentType != want {
		t.Errorf("patch Content-Type = %q, want %q", p.ContentType, want)
	}

	// 독립 오라클: 본문을 전개해 정확히 어노테이션 1개·불필요 필드 0건임을 단얫
	// (self-fulfilling 방지 — 구현의 직렬화기를 재활용하지 않는다).
	var body map[string]any
	if err := json.Unmarshal(p.Body, &body); err != nil {
		t.Fatalf("patch body decode: %v\nbody: %s", err, p.Body)
	}
	assertKeySet(t, "patch top", body, "spec")
	spec := body["spec"].(map[string]any)
	assertKeySet(t, "patch spec", spec, "template")
	tmpl := spec["template"].(map[string]any)
	assertKeySet(t, "patch template", tmpl, "metadata")
	tmeta := tmpl["metadata"].(map[string]any)
	assertKeySet(t, "patch template.metadata", tmeta, "annotations")
	ann := tmeta["annotations"].(map[string]any)
	assertKeySet(t, "patch annotations", ann, RestartAnnotationKey)
	if got := ann[RestartAnnotationKey]; got != frozenRestartedAtValue {
		t.Errorf("patch annotation %s = %v, want frozen %q", RestartAnnotationKey, got, frozenRestartedAtValue)
	}
	if !strings.Contains(string(p.Body), RestartAnnotationKey) {
		t.Errorf("raw patch body lacks the annotation key: %s", p.Body)
	}

	// §0.1 시맨틱: 시드 generation 1 + spec 변경 1회 → 2. expectGeneration=2.
	if got, want := f.mock.generation(), int64(2); got != want {
		t.Errorf("cluster generation after patch = %d, want %d", got, want)
	}
	wantRef := strings.Join([]string{"rollout", execConnUID, execNamespace, "deployment", execWorkloadName, "2"}, "|")
	if handle.ProviderRef != wantRef {
		t.Errorf("handle ProviderRef = %q, want %q", handle.ProviderRef, wantRef)
	}
}

func assertKeySet(t *testing.T, where string, m map[string]any, want ...string) {
	t.Helper()
	if len(m) != len(want) {
		t.Errorf("%s key set = %v, want exactly %v", where, keysOf(m), want)
		return
	}
	for _, k := range want {
		if _, ok := m[k]; !ok {
			t.Errorf("%s missing key %q (got %v)", where, k, keysOf(m))
		}
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestExecuteRepatchIsByteIdentical — J1 멱등 근거: 동일 동결값 재실행(크래시 →
// 리퍼 재큐 → attempt 2)은 byte-identical patch를 내고 generation을 증가시키지
// 않는다(A1 모의 시맨틱) — 재롤아웃(이중 restart) 없음.
func TestExecuteRepatchIsByteIdentical(t *testing.T) {
	replicas := 2
	f := newExecutorFixture(t, "deployment", &replicas)

	first, err := f.adapter.Execute(context.Background(), f.request())
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	second, err := f.adapter.Execute(context.Background(), f.request())
	if err != nil {
		t.Fatalf("second Execute (requeue replay): %v", err)
	}

	if n := f.mock.patchCount(); n != 2 {
		t.Fatalf("patch count = %d, want 2", n)
	}
	if !bytes.Equal(f.mock.patches[0].Body, f.mock.patches[1].Body) {
		t.Errorf("repatch body differs:\nfirst:  %s\nsecond: %s", f.mock.patches[0].Body, f.mock.patches[1].Body)
	}
	if first.ProviderRef != second.ProviderRef {
		t.Errorf("repatch handle changed: %q → %q (expectGeneration must be identical)", first.ProviderRef, second.ProviderRef)
	}
	if got, want := f.mock.generation(), int64(2); got != want {
		t.Errorf("cluster generation after repatch = %d, want %d (identical-value patch must not bump generation — A1)", got, want)
	}
}

// TestExecuteValidationRejectsWithoutSideEffects — 검증 실패는 HTTP 왕복 전에
// 즉시 에러(패치 0발행). Material["operations"] 부재 메시지는 자격 purpose를
// 이름질 뿐 평문을 노출하지 않는다.
func TestExecuteValidationRejectsWithoutSideEffects(t *testing.T) {
	replicas := 1
	f := newExecutorFixture(t, "deployment", &replicas)
	conn := f.Connection()

	cases := []struct {
		name string
		req  contract.OperationRequest
		want string // 에러 메시지 부분 문자열
	}{
		{
			name: "unknown operation",
			req:  contract.OperationRequest{OperationName: "k8s.workload.delete", ResourceURN: "urn:k8s:3:workload:v2-seed/deployment/seed-web", Payload: contract.JSONMap{"restartedAt": frozenRestartedAtValue}, Connection: conn},
			want: "not served",
		},
		{
			name: "empty URN",
			req:  contract.OperationRequest{OperationName: RestartOperationName, ResourceURN: "", Payload: contract.JSONMap{"restartedAt": frozenRestartedAtValue}, Connection: conn},
			want: "not a k8s URN",
		},
		{
			name: "non-workload URN",
			req:  contract.OperationRequest{OperationName: RestartOperationName, ResourceURN: "urn:k8s:3:node:node-uid-1", Payload: contract.JSONMap{"restartedAt": frozenRestartedAtValue}, Connection: conn},
			want: "workload URN",
		},
		{
			name: "unsupported kind",
			req:  contract.OperationRequest{OperationName: RestartOperationName, ResourceURN: "urn:k8s:3:workload:v2-seed/job/seed-web", Payload: contract.JSONMap{"restartedAt": frozenRestartedAtValue}, Connection: conn},
			want: "unsupported workload kind",
		},
		{
			name: "short URN tail",
			req:  contract.OperationRequest{OperationName: RestartOperationName, ResourceURN: "urn:k8s:3:workload/v2-seed/deployment", Payload: contract.JSONMap{"restartedAt": frozenRestartedAtValue}, Connection: conn},
			want: "not a workload URN",
		},
		{
			name: "non-numeric context id",
			req:  contract.OperationRequest{OperationName: RestartOperationName, ResourceURN: "urn:k8s:prod:workload:v2-seed/deployment/seed-web", Payload: contract.JSONMap{"restartedAt": frozenRestartedAtValue}, Connection: conn},
			want: "non-numeric context id",
		},
		{
			name: "cloud URN prefix",
			req:  contract.OperationRequest{OperationName: RestartOperationName, ResourceURN: "urn:aws:3:workload:v2-seed/deployment/seed-web", Payload: contract.JSONMap{"restartedAt": frozenRestartedAtValue}, Connection: conn},
			want: "not a k8s URN",
		},
		{
			name: "missing restartedAt",
			req:  contract.OperationRequest{OperationName: RestartOperationName, ResourceURN: "urn:k8s:3:workload:v2-seed/deployment/seed-web", Payload: contract.JSONMap{}, Connection: conn},
			want: "restartedAt",
		},
		{
			name: "restartedAt not a string",
			req:  contract.OperationRequest{OperationName: RestartOperationName, ResourceURN: "urn:k8s:3:workload:v2-seed/deployment/seed-web", Payload: contract.JSONMap{"restartedAt": 1759207323}, Connection: conn},
			want: "RFC3339",
		},
		{
			name: "restartedAt not RFC3339",
			req:  contract.OperationRequest{OperationName: RestartOperationName, ResourceURN: "urn:k8s:3:workload:v2-seed/deployment/seed-web", Payload: contract.JSONMap{"restartedAt": "2026-09-07 01:02:03"}, Connection: conn},
			want: "RFC3339",
		},
		{
			name: "missing operations material",
			req:  contract.OperationRequest{OperationName: RestartOperationName, ResourceURN: "urn:k8s:3:workload:v2-seed/deployment/seed-web", Payload: contract.JSONMap{"restartedAt": frozenRestartedAtValue}, Connection: contract.ConnectionView{UID: execConnUID, ProviderType: ProviderName}},
			want: "operations",
		},
		{
			name: "blank operations material",
			req:  contract.OperationRequest{OperationName: RestartOperationName, ResourceURN: "urn:k8s:3:workload:v2-seed/deployment/seed-web", Payload: contract.JSONMap{"restartedAt": frozenRestartedAtValue}, Connection: contract.ConnectionView{UID: execConnUID, ProviderType: ProviderName, Material: map[string]string{contract.CredentialPurposeOperations: "  "}}},
			want: "operations",
		},
		{
			name: "connection without UID",
			req:  contract.OperationRequest{OperationName: RestartOperationName, ResourceURN: "urn:k8s:3:workload:v2-seed/deployment/seed-web", Payload: contract.JSONMap{"restartedAt": frozenRestartedAtValue}, Connection: contract.ConnectionView{ProviderType: ProviderName, Material: conn.Material}},
			want: "UID",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.adapter.Execute(context.Background(), tc.req)
			if err == nil {
				t.Fatalf("Execute succeeded, want error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Execute error = %v, want substring %q", err, tc.want)
			}
			if n := f.mock.patchCount(); n != 0 {
				t.Errorf("validation failure issued %d HTTP patch(es), want 0", n)
			}
		})
	}
}

// TestExecuteToleratesResourceRevision — resourceRevision은 감사용 선택 키(§3.2):
// 존재해도 거부하지 않는다.
func TestExecuteToleratesResourceRevision(t *testing.T) {
	replicas := 1
	f := newExecutorFixture(t, "deployment", &replicas)
	req := f.request()
	req.Payload["resourceRevision"] = "1"

	if _, err := f.adapter.Execute(context.Background(), req); err != nil {
		t.Fatalf("Execute with resourceRevision: %v", err)
	}
}

// --- Poll — 교정 판정식 3종 (J2 r2). ---

func TestPollDeploymentConvergence(t *testing.T) {
	replicas := 3
	f := newExecutorFixture(t, "deployment", &replicas)

	handle, err := f.adapter.Execute(context.Background(), f.request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	t.Run("old_generation_cohort_coexisting_is_running", func(t *testing.T) {
		// 렌즈2 반증 시나리오: available은 spec을 만족하지만 구 RS pod가 병존
		// (updatedReplicas < spec) — available/ready만의 판정식은 여기서 오탐한다.
		f.mock.setStatus(map[string]any{
			"observedGeneration": 2, "replicas": 4, "readyReplicas": 3,
			"availableReplicas": 3, "updatedReplicas": 1,
		})
		status, err := f.poll(handle)
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if status.State != contract.OperationStateRunning {
			t.Errorf("state = %q, want running (updatedReplicas 1 < spec 3 — rollout still in flight)", status.State)
		}
		if status.Detail != nil {
			t.Errorf("running status carried detail %v, want nil (detail is terminal-only)", status.Detail)
		}
	})

	t.Run("observed_generation_lagging_is_running", func(t *testing.T) {
		f.mock.setStatus(map[string]any{
			"observedGeneration": 1, "replicas": 3, "readyReplicas": 3,
			"availableReplicas": 3, "updatedReplicas": 3,
		})
		status, err := f.poll(handle)
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if status.State != contract.OperationStateRunning {
			t.Errorf("state = %q, want running (observedGeneration 1 < expect 2)", status.State)
		}
	})

	t.Run("converged_succeeds_with_detail", func(t *testing.T) {
		f.mock.setStatus(map[string]any{
			"observedGeneration": 2, "replicas": 3, "readyReplicas": 3,
			"availableReplicas": 3, "updatedReplicas": 3,
		})
		status, err := f.poll(handle)
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if status.State != contract.OperationStateSucceeded {
			t.Fatalf("state = %q, want succeeded (detail: %v)", status.State, status.Detail)
		}
		d := status.Detail
		if d == nil {
			t.Fatal("succeeded status carried no detail")
		}
		assertDetailKeys(t, d)
		if got := d["generation"]; fmt.Sprint(got) != "2" {
			t.Errorf("detail generation = %v, want 2", got)
		}
		if got := d["restartedAt"]; got != frozenRestartedAtValue {
			t.Errorf("detail restartedAt = %v, want frozen %q (gate ② assertion 3 evidence)", got, frozenRestartedAtValue)
		}
		if got := d["readyReplicas"]; fmt.Sprint(got) != "3" {
			t.Errorf("detail readyReplicas = %v, want 3", got)
		}
		if got := d["updated"]; fmt.Sprint(got) != "3" {
			t.Errorf("detail updated = %v, want 3", got)
		}
		assertServerURL(t, d)
	})

	t.Run("nil_replicas_defaults_to_one", func(t *testing.T) {
		// k8s 시맨틱: spec.replicas 생략은 기본값 1.
		oneless := newExecutorFixture(t, "deployment", nil)
		h, err := oneless.adapter.Execute(context.Background(), oneless.request())
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		oneless.mock.setStatus(map[string]any{
			"observedGeneration": 2, "readyReplicas": 1,
			"availableReplicas": 1, "updatedReplicas": 1,
		})
		status, err := oneless.poll(h)
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if status.State != contract.OperationStateSucceeded {
			t.Errorf("state = %q, want succeeded with the replicas default of 1", status.State)
		}
	})
}

func TestPollStatefulSetConvergence(t *testing.T) {
	replicas := 2
	f := newExecutorFixture(t, "statefulset", &replicas)

	handle, err := f.adapter.Execute(context.Background(), f.request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	t.Run("revision_divergence_is_running", func(t *testing.T) {
		// sts는 구 revision pod가 ready 카운트에 포함되므로 revision 수렴이
		// 롤아웃 종결 신호다(r2 교정).
		f.mock.setStatus(map[string]any{
			"replicas": 2, "readyReplicas": 2, "updatedReplicas": 1,
			"currentRevision": "seed-web-1", "updateRevision": "seed-web-2",
		})
		status, err := f.poll(handle)
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if status.State != contract.OperationStateRunning {
			t.Errorf("state = %q, want running (updateRevision != currentRevision)", status.State)
		}
	})

	t.Run("revision_converged_but_not_ready_is_running", func(t *testing.T) {
		f.mock.setStatus(map[string]any{
			"replicas": 2, "readyReplicas": 1, "updatedReplicas": 1,
			"currentRevision": "seed-web-2", "updateRevision": "seed-web-2",
		})
		status, err := f.poll(handle)
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if status.State != contract.OperationStateRunning {
			t.Errorf("state = %q, want running (readyReplicas 1 < spec 2)", status.State)
		}
	})

	t.Run("converged_succeeds", func(t *testing.T) {
		f.mock.setStatus(map[string]any{
			"replicas": 2, "readyReplicas": 2, "updatedReplicas": 2,
			"currentRevision": "seed-web-2", "updateRevision": "seed-web-2",
		})
		status, err := f.poll(handle)
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if status.State != contract.OperationStateSucceeded {
			t.Fatalf("state = %q, want succeeded", status.State)
		}
		assertDetailKeys(t, status.Detail)
		if got := status.Detail["readyReplicas"]; fmt.Sprint(got) != "2" {
			t.Errorf("detail readyReplicas = %v, want 2", got)
		}
		if got := status.Detail["updated"]; fmt.Sprint(got) != "2" {
			t.Errorf("detail updated = %v, want 2 (sts status.updatedReplicas)", got)
		}
		assertServerURL(t, status.Detail)
	})
}

func TestPollDaemonSetConvergence(t *testing.T) {
	f := newExecutorFixture(t, "daemonset", nil) // ds는 spec.replicas를 쓰지 않는다

	handle, err := f.adapter.Execute(context.Background(), f.request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	t.Run("update_in_flight_is_running", func(t *testing.T) {
		f.mock.setStatus(map[string]any{
			"desiredNumberScheduled": 3, "updatedNumberScheduled": 1, "numberReady": 3,
		})
		status, err := f.poll(handle)
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if status.State != contract.OperationStateRunning {
			t.Errorf("state = %q, want running (numberReady 3 == desired지만 구 세대 포함 — updated 1 < 3)", status.State)
		}
	})

	t.Run("converged_succeeds", func(t *testing.T) {
		f.mock.setStatus(map[string]any{
			"desiredNumberScheduled": 3, "updatedNumberScheduled": 3, "numberReady": 3,
		})
		status, err := f.poll(handle)
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if status.State != contract.OperationStateSucceeded {
			t.Fatalf("state = %q, want succeeded", status.State)
		}
		assertDetailKeys(t, status.Detail)
		if got := status.Detail["readyReplicas"]; fmt.Sprint(got) != "3" {
			t.Errorf("detail readyReplicas = %v, want 3 (ds numberReady)", got)
		}
		if got := status.Detail["updated"]; fmt.Sprint(got) != "3" {
			t.Errorf("detail updated = %v, want 3 (ds updatedNumberScheduled)", got)
		}
	})
}

// assertDetailKeys — 성공 detail은 restart 결과 redaction 허용 필드와 1:1(§10.2).
func assertDetailKeys(t *testing.T, d contract.JSONMap) {
	t.Helper()
	if d == nil {
		t.Fatal("nil detail")
	}
	assertKeySet(t, "poll detail", d, "generation", "restartedAt", "readyReplicas", "updated", "serverURL")
}

// assertServerURL — VK-3: serverURL은 Connection에서 파싱한 API 서버 주소.
// httptest는 loopback — kind 게이트의 loopback 단얫과 동일 형상.
func assertServerURL(t *testing.T, d contract.JSONMap) {
	t.Helper()
	url, _ := d["serverURL"].(string)
	if url == "" {
		t.Fatal("detail serverURL missing")
	}
	host := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
	if !strings.HasPrefix(host, "127.0.0.1:") {
		t.Errorf("detail serverURL = %q, want a loopback httptest address", url)
	}
}

// --- Poll — 신호·핸들 계약. ---

func TestPollProviderSignals(t *testing.T) {
	replicas := 1
	f := newExecutorFixture(t, "deployment", &replicas)
	handle, err := f.adapter.Execute(context.Background(), f.request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	t.Run("forbidden_maps_to_permission_denied", func(t *testing.T) {
		f.mock.setMode(contract.SignalPermissionDenied)
		_, err := f.poll(handle)
		assertSignal(t, err, contract.SignalPermissionDenied)
	})

	t.Run("rate_limited_maps_to_rate_limited", func(t *testing.T) {
		f.mock.setMode(contract.SignalRateLimited)
		_, err := f.poll(handle)
		assertSignal(t, err, contract.SignalRateLimited)
	})

	t.Run("missing_workload_is_plain_error", func(t *testing.T) {
		f.mock.setMode("not_found")
		_, err := f.poll(handle)
		if err == nil {
			t.Fatal("Poll on a deleted workload succeeded, want error")
		}
		var sig *contract.ProviderSignalError
		if errors.As(err, &sig) {
			t.Errorf("404 surfaced as signal %q, want a plain error (그 외 일반 error — §3.2)", sig.Kind)
		}
		if !strings.Contains(err.Error(), "404") {
			t.Errorf("error should carry the status: %v", err)
		}
	})
}

func assertSignal(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want ProviderSignalError %q, got nil", want)
	}
	var sig *contract.ProviderSignalError
	if !errors.As(err, &sig) {
		t.Fatalf("error %v is not a ProviderSignalError", err)
	}
	if sig.Kind != want {
		t.Errorf("signal kind = %q, want %q", sig.Kind, want)
	}
}

// TestExecuteProviderSignals — PATCH 경로의 신호 매핑(patchJSON이 getJSON과
// 동일한 §9.3 매핑을 유지하는지).
func TestExecuteProviderSignals(t *testing.T) {
	replicas := 1
	f := newExecutorFixture(t, "deployment", &replicas)
	req := f.request()

	f.mock.setMode(contract.SignalPermissionDenied)
	_, err := f.adapter.Execute(context.Background(), req)
	assertSignal(t, err, contract.SignalPermissionDenied)

	f.mock.setMode(contract.SignalRateLimited)
	_, err = f.adapter.Execute(context.Background(), req)
	assertSignal(t, err, contract.SignalRateLimited)
}

func TestPollHandleContract(t *testing.T) {
	replicas := 1
	f := newExecutorFixture(t, "deployment", &replicas)

	cases := []struct {
		name string
		ref  string
		want string
	}{
		{"empty ref", "", "malformed rollout handle"},
		{"wrong marker", "deploy|u|ns|deployment|n|2", "not a rollout handle"},
		{"too few segments", "rollout|u|ns|deployment|n", "malformed rollout handle"},
		{"too many segments", "rollout|u|ns|deployment|n|2|extra", "malformed rollout handle"},
		{"non-numeric generation", "rollout|u|ns|deployment|n|two", "invalid expectGeneration"},
		{"zero generation", "rollout|u|ns|deployment|n|0", "invalid expectGeneration"},
		{"empty connection uid", "rollout||ns|deployment|n|2", "connection"},
		{"unsupported kind", "rollout|u|ns|cronjob|n|2", "unsupported workload kind"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.poll(contract.OperationHandle{ProviderRef: tc.ref})
			if err == nil {
				t.Fatalf("Poll succeeded, want error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Poll error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

// TestPollConnectionMustMatchHandle — J12 정합 가드: 엔진이 조립한 Connection의
// UID가 handle이 자기서술하는 connUID와 다르면 조립 버그 — 다른 클러스터를
// 폴하는 오발사를 즉시 잡는다.
func TestPollConnectionMustMatchHandle(t *testing.T) {
	replicas := 1
	f := newExecutorFixture(t, "deployment", &replicas)
	handle, err := f.adapter.Execute(context.Background(), f.request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	other := f.Connection()
	other.UID = "conn-uid-other-cluster"
	_, err = f.adapter.Poll(context.Background(), contract.PollRequest{Handle: handle, Connection: other})
	if err == nil {
		t.Fatal("Poll with a mismatched connection succeeded, want error")
	}
	if !strings.Contains(err.Error(), execConnUID) || !strings.Contains(err.Error(), "conn-uid-other-cluster") {
		t.Errorf("mismatch error should name both UIDs: %v", err)
	}
}

// TestPollMissingOperationsMaterial — 재폴 자격 부재도 즉시 에러(Execute의
// Material 대칭 — J12).
func TestPollMissingOperationsMaterial(t *testing.T) {
	replicas := 1
	f := newExecutorFixture(t, "deployment", &replicas)
	handle, err := f.adapter.Execute(context.Background(), f.request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	_, err = f.adapter.Poll(context.Background(), contract.PollRequest{
		Handle:     handle,
		Connection: contract.ConnectionView{UID: execConnUID, ProviderType: ProviderName},
	})
	if err == nil {
		t.Fatal("Poll without Material[operations] succeeded, want error")
	}
	if !strings.Contains(err.Error(), contract.CredentialPurposeOperations) {
		t.Errorf("error should name the credential purpose: %v", err)
	}
}

// TestExecutorEmitsNoCredentialMaterial — VK-10/보존 제약 #7: 검증 에러·신호
// 에러·성공 detail·patch 본문 어디에도 kubeconfig 물질(bearer)이 출현하지
// 않는다.
func TestExecutorEmitsNoCredentialMaterial(t *testing.T) {
	replicas := 2
	f := newExecutorFixture(t, "statefulset", &replicas)

	handle, err := f.adapter.Execute(context.Background(), f.request())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	f.mock.setStatus(map[string]any{
		"replicas": 2, "readyReplicas": 2, "updatedReplicas": 2,
		"currentRevision": "seed-web-2", "updateRevision": "seed-web-2",
	})
	status, err := f.poll(handle)
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}

	f.mock.setMode(contract.SignalPermissionDenied)
	_, pollErr := f.poll(handle)
	sigErr := fmt.Sprint(pollErr)

	badReq := f.request()
	badReq.Payload = contract.JSONMap{}
	_, validateErr := f.adapter.Execute(context.Background(), badReq)

	detailJSON, _ := json.Marshal(status.Detail)
	scans := map[string]string{
		"success detail":  string(detailJSON),
		"poll signal err": sigErr,
		"validation err":  fmt.Sprint(validateErr),
	}
	for i, p := range f.mock.patches {
		scans[fmt.Sprintf("patch body %d", i)] = string(p.Body)
	}
	for where, s := range scans {
		if strings.Contains(s, kubeconfigTokenStr) {
			t.Errorf("%s leaks credential material:\n%s", where, s)
		}
	}
}
