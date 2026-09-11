// executor_create_test.go — I10 §3.4: 커넥션-스코프 create leg(k8s.resource.create)
// 의 Execute 계약 테스트. 모의 k8s API 서버는 POST 컬렉션 경로의 create 시맨틱
// (201 생성·존재 시 409)과 GET 항목 경로만 제공한다 — executor_resource 가족의
// mock·fixture 분리 형상을 승계하되 산출물 계약이 단일 테스트 파일이라 본 파일 안에
// 둔다(판단 기록). 단얫면: 신원 가드(불소속 kind·namespace 규칙)·컬렉션 경로 20행
// 전수(v1 face 15 resourceType + ReplicaSet)·POST 201/409-에코일치/409-불일치
// 3분기·state| 핸들 부호화·매핑 표 APIVersions 무변형(J0 리뷰 LOW-2)·
// CreateOperationName ↔ registry def 교차 잠금(J0 리뷰 LOW-3).
package kubernetes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/registry"
)

// --- 모의 create API 서버 — POST 컬렉션 경로 + GET 항목 경로. ---

// createMock — 컬렉션 경로별 강제 상태(3분기 반증 재료)와 객체 저장소를 가진
// 모의 서버. POST는 본문 metadata.name으로 항목 경로를 유도하고, 존재 시 409를
// 내는 k8s create 시맨틱이다. 저장 시 서버측 필드(uid·resourceVersion)를
// 주입해 폴·수렴 판정의 부분집합 에코(관측 ⊇ 동결) 재료로 쓴다.
type createMock struct {
	t   *testing.T
	srv *httptest.Server

	mu               sync.Mutex
	objects          map[string]map[string]any // 항목 경로 → 관측 객체
	forcedPostStatus map[string]int            // 컬렉션 경로 → 강제 POST 상태(0 = 기본 시맨틱)
	posts            []string                  // POST된 컬렉션 경로 기록
	gets             []string                  // GET된 항목 경로 기록
}

func newCreateMock(t *testing.T) *createMock {
	m := &createMock{
		t:                t,
		objects:          map[string]map[string]any{},
		forcedPostStatus: map[string]int{},
	}
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.srv.Close)
	return m
}

func (m *createMock) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch r.Method {
	case http.MethodPost:
		var obj map[string]any
		if err := json.NewDecoder(r.Body).Decode(&obj); err != nil {
			m.t.Fatalf("createMock: decode post body: %v", err)
		}
		m.posts = append(m.posts, r.URL.Path)
		if status := m.forcedPostStatus[r.URL.Path]; status != 0 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"kind":"Status","reason":"AlreadyExists"}`))
			return
		}
		metadata, _ := obj["metadata"].(map[string]any)
		name, _ := metadata["name"].(string)
		itemPath := r.URL.Path + "/" + name
		if _, exists := m.objects[itemPath]; exists {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"kind":"Status","reason":"AlreadyExists","message":` +
				`"` + itemPath + ` already exists"}`))
			return
		}
		m.objects[itemPath] = enrichObserved(obj)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(m.objects[itemPath])
	case http.MethodGet:
		m.gets = append(m.gets, r.URL.Path)
		obj, ok := m.objects[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(obj)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// enrichObserved — 서버가 생성 객체에 붙이는 필드(uid·resourceVersion·
// metadata.resourceVersion) 주입. 동결 manifest ⊊ 관측 — 부분집합 에코의
// 정방향 재현이다. 입력(동결 manifest)은 변형하지 않는다(immuetable 복사).
func enrichObserved(obj map[string]any) map[string]any {
	out := make(map[string]any, len(obj)+1)
	for key, value := range obj {
		out[key] = value
	}
	out["uid"] = "mock-uid-1"
	metadata, _ := out["metadata"].(map[string]any)
	metaCopy := make(map[string]any, len(metadata)+2)
	for key, value := range metadata {
		metaCopy[key] = value
	}
	metaCopy["uid"] = "mock-uid-1"
	metaCopy["resourceVersion"] = "1001"
	out["metadata"] = metaCopy
	return out
}

func (m *createMock) postPaths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.posts)
}

func (m *createMock) getPaths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.gets)
}

func (m *createMock) seed(itemPath string, obj map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[itemPath] = obj
}

func (m *createMock) forcePost(collection string, status int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.forcedPostStatus[collection] = status
}

// --- 요청 조립 — 커넥션-스코프(ResourceURN 공백) 요청. ---

func createConnection(mock *createMock) contract.ConnectionView {
	return contract.ConnectionView{
		UID:          execConnUID,
		ProviderType: ProviderName,
		Material:     map[string]string{contract.CredentialPurposeOperations: kubeconfigFor(mock.srv.URL)},
	}
}

func createRequest(mock *createMock, yaml string) contract.OperationRequest {
	return contract.OperationRequest{
		OperationName: CreateOperationName,
		Payload:       contract.JSONMap{"yaml": yaml},
		Connection:    createConnection(mock),
	}
}

// --- fixture 매니페스트. ---

// createConfigMapYAML — core kind. metadata에 namespace가 있다(namespaced 종은
// 필수 — §3.2.1 namespace 규칙).
const createConfigMapYAML = "apiVersion: v1\n" +
	"kind: ConfigMap\n" +
	"metadata:\n" +
	"  name: created-cm\n" +
	"  namespace: " + execNamespace + "\n" +
	"data:\n" +
	"  level: info\n"

// createVirtualServiceYAML — istio v1beta1 선언. 매핑 표 기본 순서(v1→v1beta1)를
// 선언 버전 선순위로 재정렬해야 하는 LOW-1 재료다.
const createVirtualServiceYAML = "apiVersion: networking.istio.io/v1beta1\n" +
	"kind: VirtualService\n" +
	"metadata:\n" +
	"  name: created-vs\n" +
	"  namespace: " + execNamespace + "\n" +
	"spec:\n" +
	"  hosts:\n" +
	"    - created.example.com\n"

const createNamespaceYAML = "apiVersion: v1\n" +
	"kind: Namespace\n" +
	"metadata:\n" +
	"  name: created-ns\n"

func cmCollectionPath() string {
	return "/api/v1/namespaces/" + execNamespace + "/configmaps"
}

func vsCollectionPath(version string) string {
	return "/apis/networking.istio.io/" + version + "/namespaces/" + execNamespace + "/virtualservices"
}

// --- Execute — 201 행복 경로 + state| 핸들. ---

// TestExecuteResourceCreatePostsFrozenManifest — POST 1회(동결 매니페스트 본문,
// 컬렉션 경로) → state| 핸들(항목 경로·manifest 기대상태). 핸들은 kubeconfig 물질을
// 운반하지 않는다(VK-10). 재실행은 POST 409 → GET 부분집합 에코 → 동일 핸들
// 수렴(RI-5 — crash-retry가 이미 도달한 목표를 실패로 오보하지 않는다).
func TestExecuteResourceCreatePostsFrozenManifest(t *testing.T) {
	mock := newCreateMock(t)

	handle, err := NewAdapter().Execute(context.Background(), createRequest(mock, createConfigMapYAML))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	posts := mock.postPaths()
	if len(posts) != 1 || posts[0] != cmCollectionPath() {
		t.Fatalf("post paths = %v, want exactly [%s]", posts, cmCollectionPath())
	}

	// 핸들 — state 가족 인코딩(executor_state.go 무편집 재사용): connUID·항목
	// 경로·manifest 기대상태. kubeconfig 물질 부재 단얫.
	got := mustDecodeStateHandle(t, handle.ProviderRef)
	itemPath := cmCollectionPath() + "/created-cm"
	if got.ConnectionUID != execConnUID || got.APIPath != itemPath {
		t.Errorf("state handle = %+v, want connUID %q apiPath %q", got, execConnUID, itemPath)
	}
	if got.Expectation.Resource != "manifest" {
		t.Errorf("expectation resource = %q, want manifest (기존 화이트리스트 재사용 — executor_state.go 무편집)", got.Expectation.Resource)
	}
	if manifestSubsumes(got.Expectation.Manifest, map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{"name": "created-cm", "namespace": execNamespace},
		"data":     map[string]any{"level": "info"},
	}) {
		// 동결 manifest가 원본 구조를 보존하는지(부분집합 판정의 양방향 재료).
	} else {
		t.Errorf("frozen manifest lost the submitted structure: %v", got.Expectation.Manifest)
	}
	if strings.Contains(handle.ProviderRef, kubeconfigTokenStr) {
		t.Error("state handle carries kubeconfig material — 보존 제약 #5 위반")
	}

	// 재실행(크래시 → 리퍼 재큐 시뮬레이션) — 객체가 이제 존재하므로 POST 409 →
	// GET 에코 → 수렴 핸들. ProviderRef는 동일하다(동일 payload → 동일 동결).
	retry, err := NewAdapter().Execute(context.Background(), createRequest(mock, createConfigMapYAML))
	if err != nil {
		t.Fatalf("retry Execute (crash-retry convergence): %v", err)
	}
	if retry.ProviderRef != handle.ProviderRef {
		t.Errorf("retry handle = %q, want the identical %q", retry.ProviderRef, handle.ProviderRef)
	}
	if posts = mock.postPaths(); len(posts) != 2 {
		t.Errorf("post count after retry = %d, want 2", len(posts))
	}
	if gets := mock.getPaths(); len(gets) != 1 || gets[0] != itemPath {
		t.Errorf("get paths after retry = %v, want exactly [%s] (409 수렴 판정 — GET 항목 경로 1회)", gets, itemPath)
	}

	// 별도 클러스터(신규 mock) — 동일 payload는 동일 ProviderRef다(결정적 부호화).
	fresh := newCreateMock(t)
	other, err := NewAdapter().Execute(context.Background(), createRequest(fresh, createConfigMapYAML))
	if err != nil {
		t.Fatalf("fresh-cluster Execute: %v", err)
	}
	if other.ProviderRef != handle.ProviderRef {
		t.Errorf("fresh-cluster handle = %q, want the deterministic %q", other.ProviderRef, handle.ProviderRef)
	}
}

// --- Execute — POST 결과 3분기(§3.4). ---

// TestExecuteResourceCreateConflictTernary — 409는 유일하게 GET 판정으로 진입하는
// 상태다: 에코 일치 → 수렴 핸들 / 불일치 → 즉시 충돌 실패 / GET 404(경쟁 소실) →
// 원 POST 에러 재시도. 그 외 상태(500 등)는 GET 없이 executor error다.
func TestExecuteResourceCreateConflictTernary(t *testing.T) {
	t.Run("409 with matching echo converges", func(t *testing.T) {
		mock := newCreateMock(t)
		mock.forcePost(cmCollectionPath(), http.StatusConflict)
		// 타 실행 주체가 같은 내용을 이미 만든 상태 — 관측은 동결 manifest를
		// 포함한다(부분집합).
		mock.seed(cmCollectionPath()+"/created-cm", enrichObserved(map[string]any{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": map[string]any{"name": "created-cm", "namespace": execNamespace},
			"data":     map[string]any{"level": "info"},
		}))

		handle, err := NewAdapter().Execute(context.Background(), createRequest(mock, createConfigMapYAML))
		if err != nil {
			t.Fatalf("Execute (409+echo): %v", err)
		}
		got := mustDecodeStateHandle(t, handle.ProviderRef)
		if got.Expectation.Resource != "manifest" || got.APIPath != cmCollectionPath()+"/created-cm" {
			t.Errorf("converged handle = %+v, want manifest expectation at the item path", got)
		}
		if gets := mock.getPaths(); len(gets) != 1 {
			t.Errorf("get count = %d, want 1 (409 수렴 판정은 GET 1회)", len(gets))
		}
	})

	t.Run("409 with different content fails immediately", func(t *testing.T) {
		mock := newCreateMock(t)
		mock.forcePost(cmCollectionPath(), http.StatusConflict)
		mock.seed(cmCollectionPath()+"/created-cm", map[string]any{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": map[string]any{"name": "created-cm", "namespace": execNamespace},
			"data":     map[string]any{"level": "debug"}, // 동결 내용과 다른 값
		})

		handle, err := NewAdapter().Execute(context.Background(), createRequest(mock, createConfigMapYAML))
		if err == nil {
			t.Fatalf("Execute (409+mismatch) = handle %q, want a conflict error", handle.ProviderRef)
		}
		if !strings.Contains(err.Error(), "already exists with different content") {
			t.Errorf("error = %v, want the v1-parity conflict wording", err)
		}
		if handle.ProviderRef != "" {
			t.Errorf("handle = %q, want empty on conflict failure", handle.ProviderRef)
		}
	})

	t.Run("409 with vanished object returns the post error for retry", func(t *testing.T) {
		mock := newCreateMock(t)
		mock.forcePost(cmCollectionPath(), http.StatusConflict)

		handle, err := NewAdapter().Execute(context.Background(), createRequest(mock, createConfigMapYAML))
		if err == nil {
			t.Fatalf("Execute (409+race-gone) = handle %q, want the post error", handle.ProviderRef)
		}
		if !strings.Contains(err.Error(), "unexpected status: 409") {
			t.Errorf("error = %v, want the original 409 post error (재시도 가능 executor error)", err)
		}
	})

	t.Run("non-409 status skips the convergence GET", func(t *testing.T) {
		mock := newCreateMock(t)
		mock.forcePost(cmCollectionPath(), http.StatusInternalServerError)

		handle, err := NewAdapter().Execute(context.Background(), createRequest(mock, createConfigMapYAML))
		if err == nil {
			t.Fatalf("Execute (500) = handle %q, want an executor error", handle.ProviderRef)
		}
		if !strings.Contains(err.Error(), "unexpected status: 500") {
			t.Errorf("error = %v, want the original 500 post error", err)
		}
		if gets := mock.getPaths(); len(gets) != 0 {
			t.Errorf("get count = %d, want 0 (409만이 수렴 판정에 진입한다)", len(gets))
		}
	})
}

// --- Execute — 신원 가드(§3.2.1 — 불소속 kind·name·namespace 정합). ---

// TestExecuteResourceCreateValidationRejects — 검증 실패는 HTTP write 0회.
func TestExecuteResourceCreateValidationRejects(t *testing.T) {
	mock := newCreateMock(t)
	base := func() contract.OperationRequest { return createRequest(mock, createConfigMapYAML) }

	cases := []struct {
		name string
		req  contract.OperationRequest
	}{
		{"payload 부재", func() contract.OperationRequest {
			req := base()
			req.Payload = nil
			return req
		}()},
		{"공백 yaml", createRequest(mock, "   ")},
		{"파싱 불가 yaml", createRequest(mock, "!!!: [")},
		{"객체 아닌 yaml", createRequest(mock, "- a")},
		{"apiVersion 부재", createRequest(mock, "kind: ConfigMap\nmetadata:\n  name: created-cm\n  namespace: "+execNamespace)},
		{"kind 부재", createRequest(mock, "apiVersion: v1\nmetadata:\n  name: created-cm\n  namespace: "+execNamespace)},
		{"매핑 표 불소속 kind", createRequest(mock, "apiVersion: v1\nkind: StorageClass\nmetadata:\n  name: created-sc")},
		{"group 불일치(istio kind를 core로)", createRequest(mock, "apiVersion: v1\nkind: VirtualService\nmetadata:\n  name: created-vs\n  namespace: "+execNamespace)},
		{"group 불일치(Gateway를 apps로)", createRequest(mock, "apiVersion: apps/v1\nkind: Gateway\nmetadata:\n  name: created-gw\n  namespace: "+execNamespace)},
		{"name 부재", createRequest(mock, "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  namespace: "+execNamespace)},
		{"namespaced 종 namespace 부재", createRequest(mock, "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: created-cm")},
		{"클러스터 스코프 종 namespace 존재", createRequest(mock, "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: created-ns\n  namespace: other")},
		{"커넥션-스코프에 URN 존재", func() contract.OperationRequest {
			req := base()
			req.ResourceURN = "urn:k8s:3:configmap:" + execNamespace + "/created-cm"
			return req
		}()},
		{"connection UID 부재", func() contract.OperationRequest {
			req := base()
			req.Connection.UID = ""
			return req
		}()},
		{"자격 물질 부재", func() contract.OperationRequest {
			req := base()
			req.Connection.Material = nil
			return req
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handle, err := NewAdapter().Execute(context.Background(), tc.req)
			if err == nil {
				t.Fatalf("Execute = handle %q, want a validation error", handle.ProviderRef)
			}
			if handle.ProviderRef != "" {
				t.Errorf("handle = %q, want empty on validation failure", handle.ProviderRef)
			}
		})
	}
	if posts := mock.postPaths(); len(posts) != 0 {
		t.Errorf("post count after all rejections = %d, want 0 (검증 실패는 HTTP write 0회)", len(posts))
	}
}

// --- 컬렉션 경로 — 매핑 표 파생(§3.2.1 파생 계약). ---

// TestCreateCollectionPathsRealigned — 컬렉션 경로 후보는 표 소유 성분(Plural·
// APIVersions·Namespaced·APIGroup)에서 조립되고, 매니페스트 선언 버전이 후보
// 안에 있으면 맨 앞으로 재정렬된다(J0 리뷰 LOW-1 — v1
// buildIstioResourcePathsWithPreferred 패리티). 출력은 항상 신규 슬라이스다
// (LOW-2 — 표 공유 백킹 배열 보존).
func TestCreateCollectionPathsRealigned(t *testing.T) {
	ns := execNamespace
	cases := []struct {
		name     string
		entry    contract.K8sCreateFaceEntry
		declared string
		want     []string
	}{
		{"core 클러스터 스코프", contract.K8sCreateFaceEntry{APIGroup: "", Plural: "namespaces", APIVersions: []string{"v1"}}, "v1",
			[]string{"/api/v1/namespaces"}},
		{"core namespaced", contract.K8sCreateFaceEntry{APIGroup: "", Plural: "configmaps", Namespaced: true, APIVersions: []string{"v1"}}, "v1",
			[]string{"/api/v1/namespaces/" + ns + "/configmaps"}},
		{"group 단일 버전", contract.K8sCreateFaceEntry{APIGroup: "apps", Plural: "deployments", Namespaced: true, APIVersions: []string{"v1"}}, "v1",
			[]string{"/apis/apps/v1/namespaces/" + ns + "/deployments"}},
		{"istio 기본 순서(선언 v1)", contract.K8sCreateFaceEntry{APIGroup: "networking.istio.io", Plural: "gateways", Namespaced: true, APIVersions: []string{"v1", "v1beta1"}}, "v1",
			[]string{"/apis/networking.istio.io/v1/namespaces/" + ns + "/gateways", "/apis/networking.istio.io/v1beta1/namespaces/" + ns + "/gateways"}},
		{"istio 선언 v1beta1 선순위", contract.K8sCreateFaceEntry{APIGroup: "networking.istio.io", Plural: "virtualservices", Namespaced: true, APIVersions: []string{"v1", "v1beta1"}}, "v1beta1",
			[]string{"/apis/networking.istio.io/v1beta1/namespaces/" + ns + "/virtualservices", "/apis/networking.istio.io/v1/namespaces/" + ns + "/virtualservices"}},
		{"gatewayapi 선언 v1beta1 선순위", contract.K8sCreateFaceEntry{APIGroup: "gateway.networking.k8s.io", Plural: "httproutes", Namespaced: true, APIVersions: []string{"v1", "v1beta1"}}, "v1beta1",
			[]string{"/apis/gateway.networking.k8s.io/v1beta1/namespaces/" + ns + "/httproutes", "/apis/gateway.networking.k8s.io/v1/namespaces/" + ns + "/httproutes"}},
		{"미지 선언 버전은 기본 순서", contract.K8sCreateFaceEntry{APIGroup: "networking.istio.io", Plural: "gateways", Namespaced: true, APIVersions: []string{"v1", "v1beta1"}}, "v2",
			[]string{"/apis/networking.istio.io/v1/namespaces/" + ns + "/gateways", "/apis/networking.istio.io/v1beta1/namespaces/" + ns + "/gateways"}},
		{"빈 선언은 기본 순서", contract.K8sCreateFaceEntry{APIGroup: "networking.istio.io", Plural: "gateways", Namespaced: true, APIVersions: []string{"v1", "v1beta1"}}, "",
			[]string{"/apis/networking.istio.io/v1/namespaces/" + ns + "/gateways", "/apis/networking.istio.io/v1beta1/namespaces/" + ns + "/gateways"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidates := tc.entry.APIVersions
			got := createCollectionPaths(tc.entry, tc.declared, ns)
			if !slices.Equal(got, tc.want) {
				t.Errorf("createCollectionPaths = %v, want %v", got, tc.want)
			}
			if !slices.Equal(candidates, tc.entry.APIVersions) {
				t.Errorf("input candidates mutated: %v, want %v", candidates, tc.entry.APIVersions)
			}
		})
	}
}

// TestExecuteResourceCreateFaceExhaustive — 서빙 면 전수(20행 = v1 face 15
// resourceType + apps/ReplicaSet)를 와이어로 잠근다: 각 apiVersion group × kind의
// 첫 후보 컬렉션 경로로 POST 1회, 핸들 항목 경로는 컬렉션 + "/" + name.
func TestExecuteResourceCreateFaceExhaustive(t *testing.T) {
	ns := execNamespace
	cases := []struct {
		apiVersion string
		kind       string
		cluster    bool // 클러스터 스코프(namespace 불요)
		collection string
	}{
		{"v1", "Namespace", true, "/api/v1/namespaces"},
		{"v1", "PersistentVolume", true, "/api/v1/persistentvolumes"},
		{"v1", "Pod", false, "/api/v1/namespaces/" + ns + "/pods"},
		{"v1", "Service", false, "/api/v1/namespaces/" + ns + "/services"},
		{"v1", "ConfigMap", false, "/api/v1/namespaces/" + ns + "/configmaps"},
		{"v1", "Secret", false, "/api/v1/namespaces/" + ns + "/secrets"},
		{"v1", "PersistentVolumeClaim", false, "/api/v1/namespaces/" + ns + "/persistentvolumeclaims"},
		{"networking.k8s.io/v1", "Ingress", false, "/apis/networking.k8s.io/v1/namespaces/" + ns + "/ingresses"},
		{"apps/v1", "Deployment", false, "/apis/apps/v1/namespaces/" + ns + "/deployments"},
		{"apps/v1", "StatefulSet", false, "/apis/apps/v1/namespaces/" + ns + "/statefulsets"},
		{"apps/v1", "DaemonSet", false, "/apis/apps/v1/namespaces/" + ns + "/daemonsets"},
		{"apps/v1", "ReplicaSet", false, "/apis/apps/v1/namespaces/" + ns + "/replicasets"},
		{"batch/v1", "Job", false, "/apis/batch/v1/namespaces/" + ns + "/jobs"},
		{"batch/v1", "CronJob", false, "/apis/batch/v1/namespaces/" + ns + "/cronjobs"},
		{"networking.istio.io/v1", "Gateway", false, "/apis/networking.istio.io/v1/namespaces/" + ns + "/gateways"},
		{"networking.istio.io/v1beta1", "VirtualService", false, "/apis/networking.istio.io/v1beta1/namespaces/" + ns + "/virtualservices"},
		{"networking.istio.io/v1beta1", "DestinationRule", false, "/apis/networking.istio.io/v1beta1/namespaces/" + ns + "/destinationrules"},
		{"networking.istio.io/v1beta1", "ServiceEntry", false, "/apis/networking.istio.io/v1beta1/namespaces/" + ns + "/serviceentries"},
		{"gateway.networking.k8s.io/v1", "Gateway", false, "/apis/gateway.networking.k8s.io/v1/namespaces/" + ns + "/gateways"},
		{"gateway.networking.k8s.io/v1beta1", "HTTPRoute", false, "/apis/gateway.networking.k8s.io/v1beta1/namespaces/" + ns + "/httproutes"},
	}
	mock := newCreateMock(t)
	for _, tc := range cases {
		t.Run(tc.kind+"@"+tc.apiVersion, func(t *testing.T) {
			name := strings.ToLower(tc.kind) + "-1"
			manifest := "apiVersion: " + tc.apiVersion + "\nkind: " + tc.kind + "\nmetadata:\n  name: " + name
			if !tc.cluster {
				manifest += "\n  namespace: " + ns
			}
			handle, err := NewAdapter().Execute(context.Background(), createRequest(mock, manifest))
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			posts := mock.postPaths()
			if len(posts) == 0 || posts[len(posts)-1] != tc.collection {
				t.Errorf("post path = %v, want last %q", posts, tc.collection)
			}
			got := mustDecodeStateHandle(t, handle.ProviderRef)
			if want := tc.collection + "/" + name; got.APIPath != want {
				t.Errorf("handle apiPath = %q, want %q", got.APIPath, want)
			}
		})
	}
	if posts := mock.postPaths(); len(posts) != len(cases) {
		t.Errorf("post count = %d, want %d", len(posts), len(cases))
	}
}

// --- 매핑 표 보존(J0 리뷰 LOW-2). ---

// TestExecuteResourceCreateTableVersionsUnmutated — 선언 버선 선순위 재정렬이
// 매핑 표의 공유 백킹 배열(apiV1Preferred)을 변형하지 않는다. 실행 전후
// contract.K8sCreateFace가 같은 순서를 반환해야 한다.
func TestExecuteResourceCreateTableVersionsUnmutated(t *testing.T) {
	const istioVirtualService = "networking.istio.io"
	const kind = "VirtualService"
	entry, ok := contract.K8sCreateFace(istioVirtualService, kind)
	if !ok {
		t.Fatalf("K8sCreateFace(%q, %q) missing — J0 매핑 표 회귀", istioVirtualService, kind)
	}
	before := slices.Clone(entry.APIVersions)
	if !slices.Equal(before, []string{"v1", "v1beta1"}) {
		t.Fatalf("table order changed upstream: %v, want [v1 v1beta1]", before)
	}

	mock := newCreateMock(t)
	if _, err := NewAdapter().Execute(context.Background(), createRequest(mock, createVirtualServiceYAML)); err != nil {
		t.Fatalf("Execute (v1beta1-declared istio manifest): %v", err)
	}
	if posts := mock.postPaths(); len(posts) != 1 || posts[0] != vsCollectionPath("v1beta1") {
		t.Errorf("post path = %v, want the realigned %q", posts, vsCollectionPath("v1beta1"))
	}

	after, ok := contract.K8sCreateFace(istioVirtualService, kind)
	if !ok {
		t.Fatalf("K8sCreateFace(%q, %q) missing after execute", istioVirtualService, kind)
	}
	if !slices.Equal(before, after.APIVersions) {
		t.Errorf("table APIVersions mutated by execution: %v, want the original %v", after.APIVersions, before)
	}
}

// --- CreateOperationName ↔ registry def 교차 잠금(J0 리뷰 LOW-3). ---

// TestCreateOperationNameRegistryCrossLock — 어댑터 상수와 compose def 리터럴
// (compose_k8s_ops.go "k8s.resource.create")의 일치를 레지스트리 경로로 잠근다.
// compose가 본 어댑터를 import하므로(사이클) 패키지 kubernetes 테스트에서 compose
// 정의를 직접 읽을 수 없다 — opdef_table_test(J0)가 compose def name == 리터럴을
// 잠그고, 본 테스트가 상수 == 리터럴 + 등록 가능성(레지스트리 검증 체인 통과) +
// reg.Operation(상수) 해석을 잠가 전이적으로 전체 사슬을 고정한다(판단 기록).
func TestCreateOperationNameRegistryCrossLock(t *testing.T) {
	if CreateOperationName != "k8s.resource.create" {
		t.Fatalf("CreateOperationName = %q, want the compose def literal %q", CreateOperationName, "k8s.resource.create")
	}
	reg := registry.New()
	def := contract.OperationDefinition{
		Name:               "k8s.resource.create", // compose_k8s_ops.go:328 리터럴과 동일
		Version:            "1",
		RequiredPermission: "assets:k8s:workload:yaml",
		RequiredCapability: "orchestration.kubernetes.apply",
		ResourceKinds:      nil, // 커넥션-스코프 — 빈 슬라이스(§3.2 부호화)
		Mutating:           true,
		RiskLevel:          "high",
		RequiresApproval:   true,
	}
	if err := reg.RegisterOperation(def); err != nil {
		t.Fatalf("RegisterOperation(create def): %v", err)
	}
	got, ok := reg.Operation(CreateOperationName)
	if !ok {
		t.Fatalf("reg.Operation(%q) not found — CreateOperationName과 등록 def 이름이 갈라졌다", CreateOperationName)
	}
	if got.RequiredCapability != def.RequiredCapability || got.RequiredPermission != def.RequiredPermission {
		t.Errorf("resolved def = %+v, want the registered create def", got)
	}
}
