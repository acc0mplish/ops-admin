// executor_traffic_mock_test — P2-D: traffic mutation 2종(k8s.istio.traffic_update·
// k8s.httproute.traffic_update) 테스트의 모의 k8s API 서버와 왕복 하네스 fixture
// (P2-A/B/C의 mock·fixture와 계약 테스트 분리 형상 승계). 모의 서버는 v1beta1 만
// 존재하는 클러스터를 재현한다 — v1 후보가 404로 폴백하는 경로(J-P1-1 선호의
// 역방향)를 istio·GatewayAPI 양쪽에서 통과시키고, PUT은 resourceMock과 같은
// replace 시맨틱(실변경만 resourceVersion·generation 증가 — applyMockObjectReplace
// 재사용)으로 provider_frozen_payload 재실행의 무증가 수렴을 실클러스터 없이
// 실증한다.
package kubernetes

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"ops-admin/backend/internal/infra/contract"
)

// --- 모의 traffic API 서버 — virtualservice(v1beta1 만) + httproute(v1beta1 만). ---

type trafficMock struct {
	t   *testing.T
	srv *httptest.Server

	mu      sync.Mutex
	vs      mockObject
	route   mockObject
	records []resourceMutationRecord
}

func newTrafficMock(t *testing.T) *trafficMock {
	m := &trafficMock{}
	// 조정 불가 선행 엔트리(match 전용 http·matches 전용 rule)로 first-entry
	// 선정(v1 firstVirtualServiceHTTPRouteIndex·firstHTTPRouteRuleIndex)이
	// 인덱스 1을 고르는 경로를 함께 단얫한다.
	m.vs = newMockObject(map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":            "seed-vs",
			"namespace":       execNamespace,
			"resourceVersion": "300",
		},
		"spec": map[string]any{
			"hosts": []any{"seed.example.com"},
			"http": []any{
				map[string]any{"match": []any{map[string]any{"uri": map[string]any{"prefix": "/api"}}}},
				map[string]any{"route": []any{
					map[string]any{"destination": map[string]any{"host": "a.seed.svc.cluster.local"}, "weight": 50},
					map[string]any{"destination": map[string]any{"host": "b.seed.svc.cluster.local", "subset": "v2"}, "weight": 50},
				}},
			},
		},
	}, "300")
	m.route = newMockObject(map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1beta1",
		"kind":       "HTTPRoute",
		"metadata": map[string]any{
			"name":            "seed-route",
			"namespace":       execNamespace,
			"resourceVersion": "400",
		},
		"spec": map[string]any{
			"hostnames": []any{"seed.example.com"},
			"rules": []any{
				map[string]any{"matches": []any{map[string]any{"path": map[string]any{"value": "/api"}}}},
				map[string]any{"backendRefs": []any{
					map[string]any{"name": "a", "port": 80, "weight": 50},
					map[string]any{"name": "b", "port": 80, "weight": 50},
				}},
			},
		},
	}, "400")
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.srv.Close)
	return m
}

// vsBasePath — v1beta1만 존재하는 istio 클러스터 — v1 후보는 404다(v1
// buildIstioResourcePathsWithPreferred 기본 순서의 폴백 역방향 재현,
// resourceMock.routeBasePath와 동일 구상).
func (m *trafficMock) vsBasePath() string {
	return "/apis/networking.istio.io/v1beta1/namespaces/" + execNamespace + "/virtualservices/seed-vs"
}

func (m *trafficMock) routeBasePath() string {
	return "/apis/gateway.networking.k8s.io/v1beta1/namespaces/" + execNamespace + "/httproutes/seed-route"
}

func (m *trafficMock) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch path := r.URL.Path; path {
	case m.vsBasePath():
		m.serveObject(w, r, &m.vs)
	case m.routeBasePath():
		m.serveObject(w, r, &m.route)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// serveObject — GET/PUT 공통 뼈대. PUT은 applyMockObjectReplace(resourceMock과
// 같은 replace 시맨틱 — 실변경만 버전 증가)다.
func (m *trafficMock) serveObject(w http.ResponseWriter, r *http.Request, obj *mockObject) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(obj.obj)
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			m.t.Fatalf("trafficMock: read put body: %v", err)
		}
		m.records = append(m.records, resourceMutationRecord{Method: r.Method, Path: r.URL.Path, ContentType: r.Header.Get("Content-Type"), Body: body})
		applyMockObjectReplace(m.t, obj, body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(obj.obj)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// --- 테스트 훅 — 폴 판정면의 반증(Running 경로) 재료. ---

// weightsOf — 관측 목표 엔트리의 현재 가중치 배열 복사(단얫·변조용). PUT 뒤 mock
// 객체는 JSON 디코드 산물로 대체되므로 수치는 int·float64 양형을 받는다.
func (m *trafficMock) weightsOf(obj *mockObject, entryField, routeField string, entryIndex int) []int {
	m.mu.Lock()
	defer m.mu.Unlock()
	spec, _ := obj.obj["spec"].(map[string]any)
	entries, _ := spec[entryField].([]any)
	entry, _ := entries[entryIndex].(map[string]any)
	routes, _ := entry[routeField].([]any)
	out := make([]int, len(routes))
	for i, rawRoute := range routes {
		route, _ := rawRoute.(map[string]any)
		switch weight := route["weight"].(type) {
		case int:
			out[i] = weight
		case float64:
			out[i] = int(weight)
		}
	}
	return out
}

// setWeights — 목표 엔트리의 가중치를 바꿔치기한다(폴 반증 — 에코 판정이 어긋나면
// Running이어야 한다). 복사 후 교체 — 원본 맵의 다른 성분은 건드리지 않는다.
func (m *trafficMock) setWeights(obj *mockObject, entryField, routeField string, entryIndex int, weights []int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	specCopy := mapsClone(obj.obj["spec"].(map[string]any))
	entries, _ := specCopy[entryField].([]any)
	entriesCopy := make([]any, len(entries))
	copy(entriesCopy, entries)
	entry := mapsClone(entries[entryIndex].(map[string]any))
	routes, _ := entry[routeField].([]any)
	routesCopy := make([]any, len(routes))
	for i, rawRoute := range routes {
		routeCopy := mapsClone(rawRoute.(map[string]any))
		routeCopy["weight"] = weights[i]
		routesCopy[i] = routeCopy
	}
	entry[routeField] = routesCopy
	entriesCopy[entryIndex] = entry
	specCopy[entryField] = entriesCopy
	obj.obj["spec"] = specCopy
}

func (m *trafficMock) mutationCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.records)
}

func (m *trafficMock) recordsOf(method string) []resourceMutationRecord {
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

func (m *trafficMock) vsGeneration() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.vs.generation
}

// --- Fixture — contracttest.OperationFixture의 traffic 2종 구현. ---

type trafficFixture struct {
	t       *testing.T
	mock    *trafficMock
	op      string
	urn     string
	payload contract.JSONMap
	bad     []contract.OperationRequest
}

func (f *trafficFixture) Request() contract.OperationRequest {
	return contract.OperationRequest{
		OperationName: f.op,
		ResourceURN:   f.urn,
		Payload:       f.payload,
		Connection:    f.Connection(),
	}
}

func (f *trafficFixture) BadRequests() []contract.OperationRequest { return f.bad }

func (f *trafficFixture) Mutations() int { return f.mock.mutationCount() }

// AllowedDetailKeys — state 가족의 2키(generation·serverURL —
// stateResultRedaction)다.
func (f *trafficFixture) AllowedDetailKeys() []string {
	return []string{"generation", "serverURL"}
}

func (f *trafficFixture) SecretMarker() string { return kubeconfigTokenStr }

func (f *trafficFixture) Connection() contract.ConnectionView {
	return contract.ConnectionView{
		UID:          execConnUID,
		ProviderType: ProviderName,
		Material:     map[string]string{contract.CredentialPurposeOperations: kubeconfigFor(f.mock.srv.URL)},
	}
}

// trafficWeightsPayload — 가중치 배열 payload(v1 Routes 배열의 weight 성분만 —
// index·host 등 표시 성분은 실행기가 소비하지 않는다).
func trafficWeightsPayload(weights ...int) contract.JSONMap {
	routes := make([]any, len(weights))
	for i, weight := range weights {
		routes[i] = map[string]any{"weight": weight}
	}
	return contract.JSONMap{"routes": routes}
}

// trafficBadRequests — 검증 실패 전수(HTTP write 0회 — GET은 부작용이 아니다).
// 문언·순서는 v1 1:1(executor_traffic.go 파일 헤더).
func trafficBadRequests(op string, singular string, urn string, ghostURN string) []contract.OperationRequest {
	at := func(u string, payload contract.JSONMap) contract.OperationRequest {
		return contract.OperationRequest{OperationName: op, ResourceURN: u, Payload: payload}
	}
	weights := trafficWeightsPayload(70, 30)
	return []contract.OperationRequest{
		{OperationName: op, ResourceURN: urn}, // payload 부재
		at(urn, contract.JSONMap{"routes": []any{}}),
		// 경로 수 불일치 — 관측 2경로에 3가중치(GET 후 거부, write 0회).
		at(urn, trafficWeightsPayload(34, 33, 33)),
		// 음수 가중치 — 수 일치 뒤 v1 순서로 거부.
		at(urn, trafficWeightsPayload(-1, 101)),
		// 합계 99.
		at(urn, trafficWeightsPayload(50, 49)),
		// URN 면 밖: 미서빙 singular·불량 tail·비URN·ghost 목표(전 후보 404).
		at("urn:k8s:3:service:"+execNamespace+"/seed-svc", weights),
		at("urn:k8s:3:"+singular+":onlyname", weights),
		{OperationName: op, ResourceURN: "not-a-urn", Payload: weights},
		at(ghostURN, weights),
	}
}

// newTrafficFixtures — 왕복 하네스에 공급할 traffic 2종 fixture. apply·delete의
// mock 분리 판단(newResourceFixtures)과 같은 이유로 op별 mock을 분리한다 —
// 하네스 단얫의 독립.
func newTrafficFixtures(t *testing.T) map[string]*trafficFixture {
	istioMock := newTrafficMock(t)
	routeMock := newTrafficMock(t)
	istioURN := "urn:k8s:3:virtualservice:" + execNamespace + "/seed-vs"
	routeURN := "urn:k8s:3:httproute:" + execNamespace + "/seed-route"
	return map[string]*trafficFixture{
		IstioTrafficUpdateOperationName: {
			t: t, op: IstioTrafficUpdateOperationName, mock: istioMock, urn: istioURN,
			payload: trafficWeightsPayload(70, 30),
			bad:     trafficBadRequests(IstioTrafficUpdateOperationName, "virtualservice", istioURN, "urn:k8s:3:virtualservice:"+execNamespace+"/ghost-vs"),
		},
		HTTPRouteTrafficUpdateOperationName: {
			t: t, op: HTTPRouteTrafficUpdateOperationName, mock: routeMock, urn: routeURN,
			payload: trafficWeightsPayload(70, 30),
			bad:     trafficBadRequests(HTTPRouteTrafficUpdateOperationName, "httproute", routeURN, "urn:k8s:3:httproute:"+execNamespace+"/ghost-route"),
		},
	}
}
