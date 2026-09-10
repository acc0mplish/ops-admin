// executor_config_mock_test — P2-B: state-convergent mutation 2종(node
// labels_update·service update) 테스트의 모의 k8s API 서버와 왕복 하네스 fixture
// (P2-A executor_workload_mock_test.go의 650라인 분할 형상 승계 — mock/fixture와
// 계약 테스트를 분리). 모의 서버는 merge-patch를 k8s 시맨틱으로 반영한다:
// 레이블 맵은 병합(null → 제거), 배열(ports)은 통째로 대체, 실제 변경만
// generation·resourceVersion을 증가시킨다 — 재실행 patch의 수렴
// (provider_state_convergent)을 실클러스터 없이 실증하는 수다.
package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"ops-admin/backend/internal/infra/contract"
)

// --- 모의 config API 서버 — node(list/object) + service object. ---

type configMock struct {
	t   *testing.T
	srv *httptest.Server

	mu                 sync.Mutex
	nodeName           string
	nodeUID            string
	nodeLabels         map[string]any
	nodeGeneration     int64
	svcNamespace       string
	svcName            string
	svcType            string
	svcClusterIP       string
	svcSelector        map[string]any
	svcPorts           []any
	svcExternalName    string
	svcLabels          map[string]any
	svcAnnotations     map[string]any
	svcResourceVersion string
	svcGeneration      int64
	patches            []rolloutPatchRecord
}

func newConfigMock(t *testing.T) *configMock {
	m := &configMock{
		t:                  t,
		nodeName:           "seed-control",
		nodeUID:            "node-uid-seed-1",
		nodeLabels:         map[string]any{"env": "dev", "kubernetes.io/hostname": "seed-control"},
		nodeGeneration:     1,
		svcNamespace:       execNamespace,
		svcName:            "seed-svc",
		svcType:            "ClusterIP",
		svcClusterIP:       "10.96.0.10",
		svcSelector:        map[string]any{"app": "web"},
		svcPorts:           []any{map[string]any{"port": 80, "protocol": "TCP", "targetPort": 80}},
		svcLabels:          map[string]any{"app": "web"},
		svcAnnotations:     map[string]any{},
		svcResourceVersion: "100",
		svcGeneration:      1,
	}
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.srv.Close)
	return m
}

func (m *configMock) nodeBasePath() string {
	return "/api/v1/nodes/" + m.nodeName
}

func (m *configMock) serviceBasePath() string {
	return "/api/v1/namespaces/" + m.svcNamespace + "/services/" + m.svcName
}

func (m *configMock) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := r.URL.Path
	switch {
	case path == "/api/v1/nodes":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{m.nodeObject()}})
	case path == m.nodeBasePath():
		m.serveObject(w, r, m.nodeObject, m.applyNodeLabelsPatch)
	case path == m.serviceBasePath():
		m.serveObject(w, r, m.serviceObject, m.applyServicePatch)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// serveObject — GET/PATCH 공통 뼈대. patch 적용은 호출자가 k8s 시맨틱을 담당한다.
func (m *configMock) serveObject(w http.ResponseWriter, r *http.Request, object func() map[string]any, apply func([]byte)) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(object())
	case http.MethodPatch:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			m.t.Fatalf("configMock: read patch body: %v", err)
		}
		m.patches = append(m.patches, rolloutPatchRecord{Path: r.URL.Path, ContentType: r.Header.Get("Content-Type"), Body: body})
		apply(body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(object())
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (m *configMock) nodeObject() map[string]any {
	labels := make(map[string]any, len(m.nodeLabels))
	for k, v := range m.nodeLabels {
		labels[k] = v
	}
	return map[string]any{
		"metadata": map[string]any{
			"name":       m.nodeName,
			"uid":        m.nodeUID,
			"labels":     labels,
			"generation": m.nodeGeneration,
		},
	}
}

func (m *configMock) serviceObject() map[string]any {
	spec := map[string]any{
		"type":      m.svcType,
		"clusterIP": m.svcClusterIP,
	}
	if m.svcSelector != nil {
		selector := make(map[string]any, len(m.svcSelector))
		for k, v := range m.svcSelector {
			selector[k] = v
		}
		spec["selector"] = selector
	}
	if m.svcPorts != nil {
		spec["ports"] = append([]any(nil), m.svcPorts...)
	}
	if m.svcExternalName != "" {
		spec["externalName"] = m.svcExternalName
	}
	return map[string]any{
		"metadata": map[string]any{
			"name":            m.svcName,
			"namespace":       m.svcNamespace,
			"resourceVersion": m.svcResourceVersion,
			"labels":          cloneMap(m.svcLabels),
			"annotations":     cloneMap(m.svcAnnotations),
			"generation":      m.svcGeneration,
		},
		"spec": spec,
	}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// applyNodeLabelsPatch — metadata.labels merge-patch의 k8s 시맨틱: null → 제거,
// 값 → 설정/치환. 실제 변경만 generation을 증가시킨다(A1 선례).
func (m *configMock) applyNodeLabelsPatch(body []byte) {
	var patch struct {
		Metadata struct {
			Labels map[string]any `json:"labels"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(body, &patch); err != nil {
		m.t.Fatalf("configMock: decode node labels patch: %v", err)
	}
	before := canonical(m.nodeLabels)
	for key, value := range patch.Metadata.Labels {
		if value == nil {
			delete(m.nodeLabels, key)
			continue
		}
		m.nodeLabels[key] = value
	}
	if before != canonical(m.nodeLabels) {
		m.nodeGeneration++
	}
}

// applyServicePatch — service wholesale-spec merge-patch의 k8s 시맨틱: metadata
// 레이블·어노테이션과 spec.selector는 병합(merge-patch는 부재 키를 지우지 않는다 —
// v1의 로컬 delete 무효와 동일 결과), spec.ports는 배열 통째 대체, 스칼라
// (type·externalName·clusterIP)는 설정. 실제 변경만 generation·resourceVersion을
// 증가시킨다.
func (m *configMock) applyServicePatch(body []byte) {
	var patch map[string]any
	if err := json.Unmarshal(body, &patch); err != nil {
		m.t.Fatalf("configMock: decode service patch: %v", err)
	}
	before := canonical(m.serviceObject())
	if metadata, ok := patch["metadata"].(map[string]any); ok {
		mergeMap(m.svcLabels, metadata["labels"])
		mergeMap(m.svcAnnotations, metadata["annotations"])
	}
	if spec, ok := patch["spec"].(map[string]any); ok {
		if value, ok := spec["type"].(string); ok {
			m.svcType = value
		}
		if value, ok := spec["externalName"].(string); ok {
			m.svcExternalName = value
		}
		if value, ok := spec["clusterIP"].(string); ok {
			m.svcClusterIP = value
		}
		mergeMap(m.svcSelector, spec["selector"])
		if ports, ok := spec["ports"].([]any); ok {
			m.svcPorts = append([]any(nil), ports...)
		}
	}
	if before != canonical(m.serviceObject()) {
		m.svcGeneration++
		version, _ := strconv.Atoi(m.svcResourceVersion)
		m.svcResourceVersion = fmt.Sprintf("%d", version+1)
	}
}

func mergeMap(target, patchRaw any) {
	patch, ok := patchRaw.(map[string]any)
	if !ok {
		return
	}
	dest, ok := target.(map[string]any)
	if !ok {
		return
	}
	for key, value := range patch {
		if value == nil {
			delete(dest, key)
			continue
		}
		dest[key] = value
	}
}

func canonical(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<unmarshalable %T>", v)
	}
	return string(raw)
}

// --- 테스트 훅 — poll 판정면의 반증(Running 경로)을 만드는 상태 교란. ---

// removeNodeLabel — 관측에서 레이블을 들어낸다(기대 초집합 위반 → Running).
func (m *configMock) removeNodeLabel(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.nodeLabels, key)
}

// setNodeLabel — 관측에 레이블을 다시 심는다(Running → 수렴 회복 경로).
func (m *configMock) setNodeLabel(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodeLabels[key] = value
}

// setServiceType — 관측 type을 바꾼다(기대 type 불일치 → Running).
func (m *configMock) setServiceType(typ string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.svcType = typ
}

func (m *configMock) nodeGenerationOf() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nodeGeneration
}

func (m *configMock) patchCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.patches)
}

func (m *configMock) generationOf() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.svcGeneration
}

// --- Fixture — contracttest.OperationFixture의 state-convergent 구현. ---

type configFixture struct {
	t       *testing.T
	mock    *configMock
	op      string
	payload contract.JSONMap
	bad     []contract.OperationRequest
}

func (f *configFixture) Request() contract.OperationRequest {
	urn := ""
	switch f.op {
	case NodeLabelsUpdateOperationName:
		urn = "urn:k8s:3:node:" + f.mock.nodeUID
	case ServiceUpdateOperationName:
		urn = "urn:k8s:3:service:" + f.mock.svcNamespace + "/" + f.mock.svcName
	}
	return contract.OperationRequest{
		OperationName: f.op,
		ResourceURN:   urn,
		Payload:       f.payload,
		Connection:    f.Connection(),
	}
}

func (f *configFixture) BadRequests() []contract.OperationRequest { return f.bad }

func (f *configFixture) Mutations() int { return f.mock.patchCount() }

// AllowedDetailKeys — state 가족의 Poll detail 2키(§10.2 — stateResultRedaction과
// 1:1).
func (f *configFixture) AllowedDetailKeys() []string {
	return []string{"generation", "serverURL"}
}

func (f *configFixture) SecretMarker() string { return kubeconfigTokenStr }

func (f *configFixture) Connection() contract.ConnectionView {
	return contract.ConnectionView{
		UID:          execConnUID,
		ProviderType: ProviderName,
		Material:     map[string]string{contract.CredentialPurposeOperations: kubeconfigFor(f.mock.srv.URL)},
	}
}

// newConfigFixtures — 왕복 하네스에 공급할 2종 fixture. node 초기 레이블에는
// kubelet 계열 키(kubernetes.io/hostname)를 심어 제거항(null)이 본문에 나타나는
// 것과 재부착 초집합 판정의 전제를 함께 검증한다. service payload는 NodePort 전환
// — nodePort 유·무, targetPort 문자열·숫자 폴백, protocol 기본값을 한 번에 건넨다.
func newConfigFixtures(t *testing.T) map[string]*configFixture {
	mock := newConfigMock(t)
	return map[string]*configFixture{
		NodeLabelsUpdateOperationName: {
			t: t, op: NodeLabelsUpdateOperationName, mock: mock,
			payload: contract.JSONMap{"labels": map[string]any{"env": "prod", "region": "kr"}},
			bad: []contract.OperationRequest{
				{OperationName: NodeLabelsUpdateOperationName, ResourceURN: "urn:k8s:3:node:" + mock.nodeUID, Payload: contract.JSONMap{}},
				{OperationName: NodeLabelsUpdateOperationName, ResourceURN: "urn:k8s:3:node:" + mock.nodeUID, Payload: contract.JSONMap{"labels": map[string]any{"": "x"}}},
				{OperationName: NodeLabelsUpdateOperationName, ResourceURN: "urn:k8s:3:node:ghost-uid", Payload: contract.JSONMap{"labels": map[string]any{"env": "prod"}}},
			},
		},
		ServiceUpdateOperationName: {
			t: t, op: ServiceUpdateOperationName, mock: mock,
			payload: contract.JSONMap{
				"type":        "NodePort",
				"selector":    map[string]any{"app": "web", "tier": "api"},
				"labels":      map[string]any{"app": "web", "patched": "yes"},
				"annotations": map[string]any{"note": "hello"},
				"ports": []any{
					map[string]any{"port": 80, "targetPort": "web", "nodePort": 30080},
					map[string]any{"port": 8080, "targetPort": "8080"},
				},
			},
			bad: []contract.OperationRequest{
				{OperationName: ServiceUpdateOperationName, ResourceURN: "urn:k8s:3:service:" + mock.svcNamespace + "/" + mock.svcName, Payload: contract.JSONMap{}},
				{OperationName: ServiceUpdateOperationName, ResourceURN: "urn:k8s:3:service:" + mock.svcNamespace + "/" + mock.svcName, Payload: contract.JSONMap{"type": "MetalLB"}},
				{OperationName: ServiceUpdateOperationName, ResourceURN: "urn:k8s:3:service:" + mock.svcNamespace + "/" + mock.svcName, Payload: contract.JSONMap{"type": "NodePort", "headless": true, "ports": []any{map[string]any{"port": 80}}}},
				{OperationName: ServiceUpdateOperationName, ResourceURN: "urn:k8s:3:service:" + mock.svcNamespace + "/" + mock.svcName, Payload: contract.JSONMap{"type": "ExternalName"}},
				{OperationName: ServiceUpdateOperationName, ResourceURN: "urn:k8s:3:service:" + mock.svcNamespace + "/" + mock.svcName, Payload: contract.JSONMap{"type": "ClusterIP", "ports": []any{map[string]any{"port": 0}}}},
				{OperationName: ServiceUpdateOperationName, ResourceURN: "urn:k8s:3:service:" + mock.svcNamespace + "/" + mock.svcName, Payload: contract.JSONMap{"type": "ClusterIP", "ports": []any{map[string]any{"port": 70000}}}},
			},
		},
	}
}
