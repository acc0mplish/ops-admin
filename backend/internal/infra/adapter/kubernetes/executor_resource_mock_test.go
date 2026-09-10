// executor_resource_mock_test — P2-C: resource mutation 2종(k8s.resource.apply
// update 한정·k8s.resource.delete) 테스트의 모의 k8s API 서버와 왕복 하네스 fixture
// (P2-A/B의 mock·fixture와 계약 테스트 분리 형상 승계). 모의 서버는 PUT을 replace
// 시맨틱으로 반영한다: 내용이 실제로 변한 것만 resourceVersion·generation을
// 증가시킨다 — 재실행 PUT의 수렴(provider_frozen_manifest)과 재 DELETE의 404 종단
// (provider_terminal_404)을 실클러스터 없이 실증하는 수다.
package kubernetes

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"ops-admin/backend/internal/infra/contract"
)

// --- 모의 resource API 서버 — configmap + httproute(v1beta1 만) 객체. ---

// resourceMutationRecord — PUT·DELETE 1회 기록(GET은 부작용이 아니라 미기록 —
// contracttest.OperationFixture.Mutations 계약).
type resourceMutationRecord struct {
	Method      string
	Path        string
	ContentType string
	Body        []byte
}

type resourceMock struct {
	t   *testing.T
	srv *httptest.Server

	mu          sync.Mutex
	cm          mockObject
	cmNamespace string
	cmName      string
	route       mockObject
	records     []resourceMutationRecord
}

// mockObject — 모의 단일 자원(present=false가 404다). PUT은 replace다.
type mockObject struct {
	present    bool
	obj        map[string]any
	generation int64
	rv         string
}

func newResourceMock(t *testing.T) *resourceMock {
	m := &resourceMock{
		t:           t,
		cmNamespace: execNamespace,
		cmName:      "seed-cm",
	}
	m.cm = newMockObject(map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":            "seed-cm",
			"namespace":       execNamespace,
			"labels":          map[string]any{"app": "seed"},
			"resourceVersion": "100",
		},
		"data": map[string]any{"level": "debug"},
	}, "100")
	m.route = newMockObject(map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1beta1",
		"kind":       "HTTPRoute",
		"metadata": map[string]any{
			"name":            "seed-route",
			"namespace":       execNamespace,
			"resourceVersion": "200",
		},
		"spec": map[string]any{"parentRefs": []any{map[string]any{"name": "seed-gw"}}},
	}, "200")
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.srv.Close)
	return m
}

func newMockObject(obj map[string]any, rv string) mockObject {
	return mockObject{present: true, obj: obj, generation: 1, rv: rv}
}

func (m *resourceMock) cmBasePath() string {
	return "/api/v1/namespaces/" + m.cmNamespace + "/configmaps/" + m.cmName
}

func (m *resourceMock) routeBasePath() string {
	// v1beta1만 존재하는 클러스터 — v1 후보는 404다(J-P1-1 폴백의 역방향 재현).
	return "/apis/gateway.networking.k8s.io/v1beta1/namespaces/" + execNamespace + "/httproutes/seed-route"
}

func (m *resourceMock) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch path := r.URL.Path; path {
	case m.cmBasePath():
		m.serveObject(w, r, &m.cm)
	case m.routeBasePath():
		m.serveObject(w, r, &m.route)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// serveObject — GET/PUT/DELETE 공통 뼈대. PUT은 replace(내용 무변화면 RV·generation
// 무증가), DELETE는 즉시 부재다.
func (m *resourceMock) serveObject(w http.ResponseWriter, r *http.Request, obj *mockObject) {
	switch r.Method {
	case http.MethodGet:
		if !obj.present {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(obj.obj)
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			m.t.Fatalf("resourceMock: read put body: %v", err)
		}
		m.records = append(m.records, resourceMutationRecord{Method: r.Method, Path: r.URL.Path, ContentType: r.Header.Get("Content-Type"), Body: body})
		m.applyReplace(obj, body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(obj.obj)
	case http.MethodDelete:
		m.records = append(m.records, resourceMutationRecord{Method: r.Method, Path: r.URL.Path})
		obj.present = false
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// applyReplace — PUT replace의 k8s 시맨틱: submitted를 그대로 저장하고 내용이
// 실제로 변했을 때만 resourceVersion·generation을 증가시킨다(비교는 버전 무관 —
// 재실행 PUT의 무증가 수렴 실증).
func (m *resourceMock) applyReplace(obj *mockObject, body []byte) {
	applyMockObjectReplace(m.t, obj, body)
}

// applyMockObjectReplace — applyReplace의 자유 함수형(P2-D traffic mock이 같은
// replace 시맨틱을 재사용한다 — 판단 기록: state 가족 mock의 수렴 실증은 한 벌).
func applyMockObjectReplace(t *testing.T, obj *mockObject, body []byte) {
	var submitted map[string]any
	if err := json.Unmarshal(body, &submitted); err != nil {
		t.Fatalf("mock: decode put body: %v", err)
	}
	if canonical(stripResourceVersion(submitted)) == canonical(stripResourceVersion(obj.obj)) {
		return
	}
	version, _ := strconv.Atoi(obj.rv)
	obj.rv = strconv.Itoa(version + 1)
	obj.generation++
	if metadata, ok := submitted["metadata"].(map[string]any); ok {
		metadata["resourceVersion"] = obj.rv
	}
	obj.obj = submitted
}

// stripResourceVersion — 비교·단얫용 복사(원본 무변경 — metadata.resourceVersion 제외).
func stripResourceVersion(obj map[string]any) map[string]any {
	out := make(map[string]any, len(obj))
	for key, value := range obj {
		out[key] = value
	}
	metadata, ok := out["metadata"].(map[string]any)
	if !ok {
		return out
	}
	metaCopy := make(map[string]any, len(metadata))
	for key, value := range metadata {
		metaCopy[key] = value
	}
	delete(metaCopy, "resourceVersion")
	out["metadata"] = metaCopy
	return out
}

// --- 테스트 훅 — 폴 판정면의 반증(Running 경로)과 재실행 경로를 만든다. ---

func (m *resourceMock) dropCMData() {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cm.obj, "data")
}

func (m *resourceMock) setCMData(value map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cm.obj["data"] = value
}

// restoreCM — DELETE가 만든 부재를 되돌린다(poll Running 경로 재현).
func (m *resourceMock) restoreCM() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cm.present = true
}

func (m *resourceMock) removeCM() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cm.present = false
}

func (m *resourceMock) mutationCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.records)
}

func (m *resourceMock) recordsOf(method string) []resourceMutationRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []resourceMutationRecord
	for _, record := range m.records {
		if record.Method == method {
			out = append(out, record)
		}
	}
	return out
}

func (m *resourceMock) cmGeneration() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cm.generation
}

// --- Fixture — contracttest.OperationFixture의 resource 2종 구현. ---

type resourceFixture struct {
	t       *testing.T
	mock    *resourceMock
	op      string
	payload contract.JSONMap
	bad     []contract.OperationRequest
}

func (f *resourceFixture) Request() contract.OperationRequest {
	urn := "urn:k8s:3:configmap:" + f.mock.cmNamespace + "/" + f.mock.cmName
	return contract.OperationRequest{
		OperationName: f.op,
		ResourceURN:   urn,
		Payload:       f.payload,
		Connection:    f.Connection(),
	}
}

func (f *resourceFixture) BadRequests() []contract.OperationRequest { return f.bad }

func (f *resourceFixture) Mutations() int { return f.mock.mutationCount() }

// AllowedDetailKeys — apply는 state 가족의 2키(generation·serverURL —
// stateResultRedaction), delete는 1키(serverURL — deleteResultRedaction)다.
func (f *resourceFixture) AllowedDetailKeys() []string {
	if f.op == DeleteOperationName {
		return []string{"serverURL"}
	}
	return []string{"generation", "serverURL"}
}

func (f *resourceFixture) SecretMarker() string { return kubeconfigTokenStr }

func (f *resourceFixture) Connection() contract.ConnectionView {
	return contract.ConnectionView{
		UID:          execConnUID,
		ProviderType: ProviderName,
		Material:     map[string]string{contract.CredentialPurposeOperations: kubeconfigFor(f.mock.srv.URL)},
	}
}

// resourceFixtureYAML — fixture manifest(ConfigMap). 의도적 특성: metadata에는
// namespace가 없다(서버가 경로에서 채우는 v1 폼 와이어)·data가 있다(폴 부분집합
// 판정의 반증 재료 — mock이 data를 들어내면 Running이 된다).
const resourceFixtureYAML = "apiVersion: v1\n" +
	"kind: ConfigMap\n" +
	"metadata:\n" +
	"  name: seed-cm\n" +
	"  labels:\n" +
	"    app: seed\n" +
	"data:\n" +
	"  level: info\n"

// resourceFixtureRouteYAML — GatewayAPI 폴백용 manifest(v1beta1 클러스터).
const resourceFixtureRouteYAML = "apiVersion: gateway.networking.k8s.io/v1beta1\n" +
	"kind: HTTPRoute\n" +
	"metadata:\n" +
	"  name: seed-route\n" +
	"spec:\n" +
	"  hostnames:\n" +
	"    - seed.example.com\n"

func applyBadRequests(mock *resourceMock) []contract.OperationRequest {
	yamlPayload := func(yaml string) contract.JSONMap { return contract.JSONMap{"yaml": yaml} }
	at := func(urn string) contract.OperationRequest {
		return contract.OperationRequest{OperationName: ApplyOperationName, ResourceURN: urn}
	}
	cmURN := "urn:k8s:3:configmap:" + mock.cmNamespace + "/" + mock.cmName
	bad := []contract.OperationRequest{
		at(cmURN), // payload 부재
		{OperationName: ApplyOperationName, ResourceURN: cmURN, Payload: yamlPayload("   ")},
		{OperationName: ApplyOperationName, ResourceURN: cmURN, Payload: yamlPayload("!!!: [")},
		{OperationName: ApplyOperationName, ResourceURN: cmURN, Payload: yamlPayload("- a")},
		{OperationName: ApplyOperationName, ResourceURN: cmURN, Payload: yamlPayload("apiVersion: v1\nmetadata:\n  name: seed-cm")},
		{OperationName: ApplyOperationName, ResourceURN: cmURN, Payload: yamlPayload("apiVersion: v1\nkind: Secret\nmetadata:\n  name: seed-cm")},
		{OperationName: ApplyOperationName, ResourceURN: cmURN, Payload: yamlPayload("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: ghost")},
		{OperationName: ApplyOperationName, ResourceURN: cmURN, Payload: yamlPayload("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: seed-cm\n  namespace: other")},
		// 클러스터 스코프 목표에 namespaced manifest — cluster-scoped 가드.
		{OperationName: ApplyOperationName, ResourceURN: "urn:k8s:3:pv:seed-pv", Payload: yamlPayload("apiVersion: v1\nkind: PersistentVolume\nmetadata:\n  name: seed-pv\n  namespace: default")},
		// 미서빙 singular·uid 신원 종·불량 subtype — parse 거부.
		at("urn:k8s:3:storageclass:fast"),
		at("urn:k8s:3:node:node-uid-x"),
		at("urn:k8s:3:workload:" + mock.cmNamespace + "/wat/nm"),
		// 존재하지 않는 목표 — 후보 GET 404 순회 후 거부(부재는 update 대상이 아니다).
		at("urn:k8s:3:configmap:" + mock.cmNamespace + "/ghost-cm"),
	}
	for i := range bad {
		if bad[i].Payload == nil {
			bad[i].Payload = yamlPayload(resourceFixtureYAML)
		}
	}
	return bad
}

func deleteBadRequests() []contract.OperationRequest {
	return []contract.OperationRequest{
		{OperationName: DeleteOperationName, ResourceURN: "urn:k8s:3:storageclass:fast"},
		{OperationName: DeleteOperationName, ResourceURN: "urn:k8s:3:configmap:onlyname"},
		{OperationName: DeleteOperationName, ResourceURN: "not-a-urn"},
		{OperationName: DeleteOperationName, ResourceURN: "urn:k8s:x:configmap:ns/nm"},
		{OperationName: DeleteOperationName, ResourceURN: "urn:k8s:3:workload:ns/wat/nm"},
	}
}

// newResourceFixtures — 왕복 하네스에 공급할 resource 2종 fixture. apply와 delete가
// 같은 mock을 공유하면 실행 순서에 따라 delete가 apply의 목표를 지워버리므로(하네스는
// 맵 순회가 아닌 고정 순서지만 단얫의 독립을 위해) mock을 분리한다(판단 기록).
func newResourceFixtures(t *testing.T) map[string]*resourceFixture {
	applyMock := newResourceMock(t)
	deleteMock := newResourceMock(t)
	return map[string]*resourceFixture{
		ApplyOperationName: {
			t: t, op: ApplyOperationName, mock: applyMock,
			payload: contract.JSONMap{"yaml": resourceFixtureYAML},
			bad:     applyBadRequests(applyMock),
		},
		DeleteOperationName: {
			t: t, op: DeleteOperationName, mock: deleteMock,
			bad: deleteBadRequests(),
		},
	}
}

// encodeGarbageExpectation — 손상 handle 조립용(base64url of expectation JSON).
func encodeGarbageExpectation(json string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(json))
}

// mustDecodeStateHandle — 테스트에서 state handle을 푸는 헬퍼.
func mustDecodeStateHandle(t *testing.T, ref string) stateHandle {
	t.Helper()
	handle, err := decodeStateRef(ref)
	if err != nil {
		t.Fatalf("decodeStateRef(%q): %v", ref, err)
	}
	return handle
}

// stateHandleGarbageCases — resource 가족이 추가한 손상 handle 사례(미지 resource·
// manifest 형태 위반). 기존 state 가족 사례는 executor_config_test.go가 담당한다.
func stateHandleGarbageCases() []string {
	return []string{
		"state|conn|/api/v1/namespaces/x|" + encodeGarbageExpectation(`{"resource":"storageclass"}`), // 미지 resource
		"state|conn|/api/v1/namespaces/x|" + encodeGarbageExpectation(`{"resource":"manifest"}`),     // manifest 부재
		"state|conn|notaapi|" + encodeGarbageExpectation(`{"resource":"delete"}`),                    // apiPath 접두 위반
		"state||/api/v1/namespaces/x|" + encodeGarbageExpectation(`{"resource":"delete"}`),           // connUID 부재
	}
}
