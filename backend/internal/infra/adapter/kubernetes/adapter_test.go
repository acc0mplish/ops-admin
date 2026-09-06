package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/contracttest"
	"ops-admin/backend/internal/infra/metrics"
)

// --- 모의 K8s API 서버 (§23.2 정신 — 실클러스터 의존 없는 httptest) ---
//
// legacy fetchK8sData가 조회하는 raw REST 응답 구조를 재현한다: 리스트 엔드포인트
// {metadata:{continue},items:[…]} + /version {gitVersion}. limit/continue 페이징을
// 존중해 커서 위임 경로를 실제로 검증한다.

type mockK8s struct {
	t          *testing.T
	srv        *httptest.Server
	mu         sync.Mutex
	gitVersion string
	mode       string // "" normal | rate_limited | permission_denied
	lists      map[string][]map[string]any
	calls      int
}

func newMockK8s(t *testing.T, gitVersion string) *mockK8s {
	m := &mockK8s{t: t, gitVersion: gitVersion, lists: map[string][]map[string]any{}}
	m.seedLists()
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.srv.Close)
	return m
}

func (m *mockK8s) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++

	switch m.mode {
	case contract.SignalRateLimited:
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		return
	case contract.SignalPermissionDenied:
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if r.URL.Path == "/version" {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"gitVersion":%q,"major":"1","minor":"29"}`, m.gitVersion)
		return
	}

	items, ok := m.lists[r.URL.Path]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	start := 0
	if tok := r.URL.Query().Get("continue"); tok != "" {
		n, err := strconv.Atoi(tok)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		start = n
	}
	limit := len(items)
	if q := r.URL.Query().Get("limit"); q != "" {
		n, err := strconv.Atoi(q)
		if err == nil && n >= 0 && n < limit {
			limit = n
		}
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	next := ""
	if end < len(items) {
		next = strconv.Itoa(end)
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"metadata":{"continue":%q,"resourceVersion":"1"},"items":`, next)
	_ = json.NewEncoder(w).Encode(items[start:end])
	fmt.Fprint(w, "}")
}

func (m *mockK8s) obj(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		m.t.Fatalf("marshal mock object: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		m.t.Fatalf("unmarshal mock object: %v", err)
	}
	return out
}

func (m *mockK8s) seedLists() {
	meta := func(name, ns, uid string) map[string]any {
		mm := map[string]any{"name": name, "uid": uid, "creationTimestamp": "2026-09-01T00:00:00Z"}
		if ns != "" {
			mm["namespace"] = ns
		}
		return mm
	}
	m.lists["/api/v1/nodes"] = []map[string]any{m.obj(map[string]any{
		"metadata": map[string]any{
			"name": "kind-control-plane", "uid": "node-uid-1", "creationTimestamp": "2026-09-01T00:00:00Z",
			"labels": map[string]string{"node-role.kubernetes.io/control-plane": ""},
		},
		"spec": map[string]any{"unschedulable": false, "taints": []map[string]string{
			{"key": "node-role.kubernetes.io/master", "effect": "NoSchedule"},
		}},
		"status": map[string]any{
			"conditions": []map[string]string{{"type": "Ready", "status": "True"}},
			"capacity":   map[string]string{"cpu": "8", "memory": "31457280Ki", "pods": "110"},
			"allocatable": map[string]string{
				"cpu": "7500m", "memory": "31457280Ki",
			},
		},
	})}
	m.lists["/api/v1/namespaces"] = []map[string]any{
		m.obj(map[string]any{"metadata": meta("default", "", "ns-uid-1"), "status": map[string]any{"phase": "Active"}}),
		m.obj(map[string]any{"metadata": meta("kube-system", "", "ns-uid-2"), "status": map[string]any{"phase": "Active"}}),
	}
	m.lists["/api/v1/pods"] = []map[string]any{
		m.obj(map[string]any{
			"metadata": meta("web-abc123", "default", "pod-uid-1"),
			"spec":     map[string]any{"nodeName": "kind-control-plane"},
			"status": map[string]any{
				"phase": "Running",
				"containerStatuses": []map[string]any{
					{"name": "app", "restartCount": 1, "ready": true},
					{"name": "sidecar", "restartCount": 1, "ready": true},
				},
			},
		}),
		m.obj(map[string]any{
			"metadata": meta("job-runner", "default", "pod-uid-2"),
			"spec":     map[string]any{"nodeName": "kind-control-plane"},
			"status":   map[string]any{"phase": "Pending", "containerStatuses": []map[string]any{}},
		}),
		m.obj(map[string]any{
			"metadata": meta("batch-done", "default", "pod-uid-3"),
			"spec":     map[string]any{"nodeName": "kind-control-plane"},
			"status":   map[string]any{"phase": "Succeeded", "containerStatuses": []map[string]any{}},
		}),
	}
	workloadContainers := []map[string]any{{"name": "app", "image": "nginx:1.25"}}
	m.lists["/apis/apps/v1/deployments"] = []map[string]any{m.obj(map[string]any{
		"metadata": meta("web", "default", "wl-uid-1"),
		"spec": map[string]any{
			"replicas": 3,
			"template": map[string]any{"spec": map[string]any{"containers": workloadContainers}},
		},
		"status": map[string]any{"readyReplicas": 3, "availableReplicas": 3},
	})}
	m.lists["/apis/apps/v1/statefulsets"] = []map[string]any{m.obj(map[string]any{
		"metadata": meta("db", "default", "wl-uid-2"),
		"spec": map[string]any{
			"replicas": 2,
			"template": map[string]any{"spec": map[string]any{"containers": workloadContainers}},
		},
		"status": map[string]any{"readyReplicas": 2},
	})}
	m.lists["/apis/apps/v1/daemonsets"] = []map[string]any{m.obj(map[string]any{
		"metadata": meta("agent", "default", "wl-uid-3"),
		"spec": map[string]any{
			"template": map[string]any{"spec": map[string]any{"containers": workloadContainers}},
		},
		"status": map[string]any{"desiredNumberScheduled": 1, "numberReady": 1},
	})}
	m.lists["/api/v1/services"] = []map[string]any{
		m.obj(map[string]any{
			"metadata": meta("kubernetes", "default", "svc-uid-1"),
			"spec": map[string]any{"type": "ClusterIP", "clusterIP": "10.96.0.1", "ports": []map[string]any{
				{"name": "https", "port": 443, "protocol": "TCP"},
			}},
		}),
		m.obj(map[string]any{
			"metadata": meta("web-svc", "default", "svc-uid-2"),
			"spec": map[string]any{"type": "NodePort", "clusterIP": "10.96.0.42", "ports": []map[string]any{
				{"name": "http", "port": 80, "protocol": "TCP", "nodePort": 30080},
			}},
		}),
	}
	m.lists["/apis/networking.k8s.io/v1/ingresses"] = []map[string]any{m.obj(map[string]any{
		"metadata": meta("web-ing", "default", "ing-uid-1"),
		"spec": map[string]any{"rules": []map[string]any{
			{"host": "web.example.com"},
		}},
	})}
	m.lists["/api/v1/configmaps"] = []map[string]any{
		m.obj(map[string]any{"metadata": meta("cm-1", "default", "cm-uid-1"), "data": map[string]string{"a": "1", "b": "2"}}),
		m.obj(map[string]any{"metadata": meta("cm-2", "default", "cm-uid-2"), "data": map[string]string{"only": "1"}}),
	}
	// The harness redaction canary lives in a secret data VALUE — normalizers
	// must surface metadata (type + data key NAMES) only.
	m.lists["/api/v1/secrets"] = []map[string]any{m.obj(map[string]any{
		"metadata": meta("secret-1", "default", "sec-uid-1"),
		"type":     "Opaque",
		"data":     map[string]string{"password": contracttest.MarkerValue},
	})}
	m.lists["/api/v1/persistentvolumes"] = []map[string]any{m.obj(map[string]any{
		"metadata": meta("pv-1", "", "pv-uid-1"),
		"spec": map[string]any{
			"storageClassName": "standard", "accessModes": []string{"ReadWriteOnce"},
			"capacity": map[string]string{"storage": "10Gi"},
		},
		"status": map[string]any{"phase": "Available"},
	})}
	m.lists["/api/v1/persistentvolumeclaims"] = []map[string]any{m.obj(map[string]any{
		"metadata": meta("pvc-1", "default", "pvc-uid-1"),
		"spec": map[string]any{
			"storageClassName": "standard", "accessModes": []string{"ReadWriteOnce"},
			"resources": map[string]any{"requests": map[string]string{"storage": "5Gi"}},
		},
		"status": map[string]any{"phase": "Bound", "capacity": map[string]string{"storage": "5Gi"}},
	})}
	m.lists["/apis/storage.k8s.io/v1/storageclasses"] = []map[string]any{m.obj(map[string]any{
		"metadata":      meta("standard", "", "sc-uid-1"),
		"provisioner":   "rancher.io/local-path",
		"reclaimPolicy": "Delete",
	})}
}

// --- 하네스 Fixture — 모의 서버를 시나리오·자격 주입면으로 감싼다. ---

const testPageSize = 4

type k8sFixture struct {
	t          *testing.T
	mock       *mockK8s
	adapter    *Adapter
	kubeconfig string
}

func newK8sFixture(t *testing.T, gitVersion string) *k8sFixture {
	mock := newMockK8s(t, gitVersion)
	adapter := NewAdapter(WithPageSize(testPageSize))
	f := &k8sFixture{t: t, mock: mock, adapter: adapter, kubeconfig: kubeconfigFor(mock.srv.URL)}
	return f
}

func kubeconfigFor(server string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Config
current-context: kind-test
clusters:
  - name: test
    cluster:
      server: %s
contexts:
  - name: kind-test
    context:
      cluster: test
      user: test
users:
  - name: test
    user:
      token: test-token
`, server)
}

func (f *k8sFixture) Seed() []contract.DiscoveredResource {
	// Independent oracle: hand-written expected URNs/kinds — not produced by
	// the normalizer (self-fulfilling 방지).
	return []contract.DiscoveredResource{
		{ExternalURN: "urn:k8s:1:node:node-uid-1", Kind: "orchestration.node"},
		{ExternalURN: "urn:k8s:1:namespace:default", Kind: "orchestration.namespace"},
		{ExternalURN: "urn:k8s:1:namespace:kube-system", Kind: "orchestration.namespace"},
		{ExternalURN: "urn:k8s:1:pod:pod-uid-1", Kind: "orchestration.pod"},
		{ExternalURN: "urn:k8s:1:pod:pod-uid-2", Kind: "orchestration.pod"},
		{ExternalURN: "urn:k8s:1:pod:pod-uid-3", Kind: "orchestration.pod"},
		{ExternalURN: "urn:k8s:1:workload:default/deployment/web", Kind: "orchestration.workload"},
		{ExternalURN: "urn:k8s:1:workload:default/statefulset/db", Kind: "orchestration.workload"},
		{ExternalURN: "urn:k8s:1:workload:default/daemonset/agent", Kind: "orchestration.workload"},
		{ExternalURN: "urn:k8s:1:service:default/kubernetes", Kind: "network.load_balancer"},
		{ExternalURN: "urn:k8s:1:service:default/web-svc", Kind: "network.load_balancer"},
		{ExternalURN: "urn:k8s:1:ingress:default/web-ing", Kind: "network.load_balancer"},
		{ExternalURN: "urn:k8s:1:configmap:default/cm-1", Kind: "orchestration.configmap"},
		{ExternalURN: "urn:k8s:1:configmap:default/cm-2", Kind: "orchestration.configmap"},
		{ExternalURN: "urn:k8s:1:secret:default/secret-1", Kind: "orchestration.secret"},
		{ExternalURN: "urn:k8s:1:pv:pv-1", Kind: "storage.volume"},
		{ExternalURN: "urn:k8s:1:pvc:default/pvc-1", Kind: "storage.volume"},
		{ExternalURN: "urn:k8s:1:storageclass:standard", Kind: "storage.pool"},
	}
}

func (*k8sFixture) PageLimit() int { return testPageSize }

func (f *k8sFixture) Scenario(name string) error {
	f.mock.mu.Lock()
	switch name {
	case "":
		f.mock.mode = ""
		f.kubeconfig = kubeconfigFor(f.mock.srv.URL)
	case contract.SignalRateLimited, contract.SignalPermissionDenied:
		f.mock.mode = name
	case contract.SignalUnreachable:
		f.mock.mode = ""
		f.kubeconfig = kubeconfigFor("http://127.0.0.1:1") // connection refused
	default:
		f.mock.mu.Unlock()
		return fmt.Errorf("k8sFixture: unknown scenario %q", name)
	}
	f.mock.mu.Unlock()
	return nil
}

func (f *k8sFixture) Connection() contract.ConnectionView {
	return contract.ConnectionView{
		ProviderType: "kubernetes",
		Material:     map[string]string{"inventory": f.kubeconfig},
	}
}

// T45 — kubernetes 어댑터가 fake와 동일한 contract harness를 통과한다
// (§23.4 Q1 2행). 게이트웨이 변형 케이스는 별도 테스트.
func TestKubernetesAdapterPassesContractHarness(t *testing.T) {
	f := newK8sFixture(t, "v1.29.4")
	contracttest.RunContractSuite(t, f.adapter, f)
}

// T43 — distribution detection: GitVersion `+k3s` 접미사 판정 (A3 — k3d 실클러스터
// 없이 GitVersion 문자열로 증명).
func TestDistributionDetection(t *testing.T) {
	cases := []struct {
		gitVersion string
		want       string
	}{
		{"v1.29.4+k3s1", "k3s"},
		{"v1.31.2+k3s1", "k3s"},
		{"v1.29.4", "kubernetes"},
		{"v1.30.0-rc.1", "kubernetes"},
		{"", "kubernetes"},
	}
	for _, tc := range cases {
		if got := detectDistribution(tc.gitVersion); got != tc.want {
			t.Errorf("detectDistribution(%q) = %q, want %q", tc.gitVersion, got, tc.want)
		}
	}

	// Health 경로로도 판정이 노출된다(모의 서버 GitVersion 기준).
	f := newK8sFixture(t, "v1.29.4+k3s1")
	h := f.adapter.Health(context.Background(), f.Connection())
	if !h.Healthy {
		t.Fatalf("Health = %+v, want healthy", h)
	}
	if !strings.Contains(h.Message, "k3s") {
		t.Errorf("Health message %q does not report k3s distribution", h.Message)
	}
}

// T42 — 정규화: 종 11종의 URN·kind/subtype·단위 정규화(Ki→GB)·secret metadata
// 전용을 고정 JSON 픽스처로 단언한다.
func TestNormalizerBuildsURNAndUnits(t *testing.T) {
	ctxID := uint(7)

	t.Run("node", func(t *testing.T) {
		raw := []byte(`{
			"metadata": {"name": "n1", "uid": "u-node", "labels": {"node-role.kubernetes.io/control-plane": ""}},
			"spec": {"taints": [{"key": "k", "effect": "NoSchedule"}]},
			"status": {
				"conditions": [{"type": "Ready", "status": "True"}],
				"capacity": {"cpu": "8", "memory": "31457280Ki"},
				"allocatable": {"cpu": "7500m", "memory": "20971520Ki"}
			}
		}`)
		res, err := normalizeSection(ctxID, "nodes", raw)
		if err != nil {
			t.Fatalf("normalizeSection(nodes): %v", err)
		}
		if res.ExternalURN != "urn:k8s:7:node:u-node" {
			t.Errorf("URN = %q, want urn:k8s:7:node:u-node", res.ExternalURN)
		}
		if res.Kind != "orchestration.node" {
			t.Errorf("Kind = %q", res.Kind)
		}
		if res.Normalized["capacityCoresGB"] != 8.0 {
			t.Errorf("capacityCoresGB = %v, want 8", res.Normalized["capacityCoresGB"])
		}
		if res.Normalized["capacityMemoryGB"] != 30.0 {
			t.Errorf("capacityMemoryGB = %v, want 30 (31457280Ki → 30 GiB-scale GB)", res.Normalized["capacityMemoryGB"])
		}
		if res.Normalized["allocatableCoresGB"] != 7.5 {
			t.Errorf("allocatableCoresGB = %v, want 7.5 (7500m)", res.Normalized["allocatableCoresGB"])
		}
		if res.Normalized["allocatableMemoryGB"] != 20.0 {
			t.Errorf("allocatableMemoryGB = %v, want 20", res.Normalized["allocatableMemoryGB"])
		}
		if res.Normalized["healthState"] != "healthy" {
			t.Errorf("healthState = %v, want healthy", res.Normalized["healthState"])
		}
		if roles, ok := res.Normalized["roles"].([]string); !ok || len(roles) != 1 || roles[0] != "control-plane" {
			t.Errorf("roles = %v, want [control-plane]", res.Normalized["roles"])
		}
	})

	t.Run("pod", func(t *testing.T) {
		raw := []byte(`{
			"metadata": {"name": "p1", "namespace": "default", "uid": "u-pod"},
			"status": {"phase": "Running", "containerStatuses": [{"restartCount": 2}, {"restartCount": 3}]}
		}`)
		res, err := normalizeSection(ctxID, "pods", raw)
		if err != nil {
			t.Fatalf("normalizeSection(pods): %v", err)
		}
		if res.ExternalURN != "urn:k8s:7:pod:u-pod" {
			t.Errorf("URN = %q (pod identity is uid — 재생성 pod은 name 정합, mapping.md 규칙)", res.ExternalURN)
		}
		if res.Kind != "orchestration.pod" {
			t.Errorf("Kind = %q", res.Kind)
		}
		if res.Normalized["restartCount"] != 5 {
			t.Errorf("restartCount = %v, want 5", res.Normalized["restartCount"])
		}
		if res.Normalized["phase"] != "Running" {
			t.Errorf("phase = %v", res.Normalized["phase"])
		}
	})

	t.Run("workload", func(t *testing.T) {
		raw := []byte(`{
			"metadata": {"name": "web", "namespace": "default", "uid": "u-wl"},
			"spec": {"replicas": 3, "template": {"spec": {"containers": [{"name": "app", "image": "nginx:1.25"}]}}},
			"status": {"readyReplicas": 2}
		}`)
		res, err := normalizeSection(ctxID, "deployments", raw)
		if err != nil {
			t.Fatalf("normalizeSection(deployments): %v", err)
		}
		if res.ExternalURN != "urn:k8s:7:workload:default/deployment/web" {
			t.Errorf("URN = %q", res.ExternalURN)
		}
		if res.Kind != "orchestration.workload" || res.Subtype != "deployment" {
			t.Errorf("Kind/Subtype = %q/%q, want orchestration.workload/deployment", res.Kind, res.Subtype)
		}
		if res.Normalized["replicas"] != 3 || res.Normalized["readyReplicas"] != 2 {
			t.Errorf("replicas/readyReplicas = %v/%v, want 3/2", res.Normalized["replicas"], res.Normalized["readyReplicas"])
		}
		if res.Normalized["image"] != "nginx:1.25" {
			t.Errorf("image = %v, want nginx:1.25 (V2-only field — 비교 집합 외)", res.Normalized["image"])
		}
	})

	t.Run("daemonset", func(t *testing.T) {
		raw := []byte(`{
			"metadata": {"name": "agent", "namespace": "default", "uid": "u-ds"},
			"status": {"desiredNumberScheduled": 4, "numberReady": 3}
		}`)
		res, err := normalizeSection(ctxID, "daemonsets", raw)
		if err != nil {
			t.Fatalf("normalizeSection(daemonsets): %v", err)
		}
		if res.Normalized["replicas"] != 4 || res.Normalized["readyReplicas"] != 3 {
			t.Errorf("replicas/readyReplicas = %v/%v, want 4/3 (daemonset desired/numberReady)", res.Normalized["replicas"], res.Normalized["readyReplicas"])
		}
	})

	t.Run("secret_metadata_only", func(t *testing.T) {
		raw := []byte(`{
			"metadata": {"name": "s1", "namespace": "default", "uid": "u-sec"},
			"type": "kubernetes.io/tls",
			"data": {"tls.crt": "LS0t", "tls.key": "LS0t"}
		}`)
		res, err := normalizeSection(ctxID, "secrets", raw)
		if err != nil {
			t.Fatalf("normalizeSection(secrets): %v", err)
		}
		if res.Kind != "orchestration.secret" {
			t.Errorf("Kind = %q (J9 어휘 확장 2종)", res.Kind)
		}
		if res.Normalized["type"] != "kubernetes.io/tls" {
			t.Errorf("type = %v", res.Normalized["type"])
		}
		keys, ok := res.Normalized["dataKeys"].([]string)
		if !ok || len(keys) != 2 {
			t.Fatalf("dataKeys = %v, want 2 key names", res.Normalized["dataKeys"])
		}
		// 데이터 값은 어디에도 존재하지 않는다 — Raw·Normalized 전수 스캔.
		for _, tree := range []string{marshalForScan(t, res.Raw), marshalForScan(t, res.Normalized)} {
			if strings.Contains(tree, "LS0t") {
				t.Errorf("secret data value leaked into payload: %s", tree)
			}
		}
	})

	t.Run("configmap", func(t *testing.T) {
		raw := []byte(`{
			"metadata": {"name": "c1", "namespace": "default", "uid": "u-cm"},
			"data": {"b": "1", "a": "2"}
		}`)
		res, err := normalizeSection(ctxID, "configmaps", raw)
		if err != nil {
			t.Fatalf("normalizeSection(configmaps): %v", err)
		}
		if res.Kind != "orchestration.configmap" {
			t.Errorf("Kind = %q (J9 어휘 확장 2종)", res.Kind)
		}
		keys, _ := res.Normalized["dataKeys"].([]string)
		if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
			t.Errorf("dataKeys = %v, want sorted [a b]", keys)
		}
	})

	t.Run("network_and_storage", func(t *testing.T) {
		svc, err := normalizeSection(ctxID, "services", []byte(`{
			"metadata": {"name": "svc", "namespace": "default", "uid": "u-svc"},
			"spec": {"type": "NodePort", "clusterIP": "10.96.0.9", "ports": [{"port": 80, "protocol": "TCP"}]}
		}`))
		if err != nil {
			t.Fatalf("normalizeSection(services): %v", err)
		}
		if svc.ExternalURN != "urn:k8s:7:service:default/svc" || svc.Kind != "network.load_balancer" || svc.Subtype != "service" {
			t.Errorf("service = %s/%s/%s", svc.ExternalURN, svc.Kind, svc.Subtype)
		}
		ing, err := normalizeSection(ctxID, "ingresses", []byte(`{
			"metadata": {"name": "ing", "namespace": "default", "uid": "u-ing"},
			"spec": {"rules": [{"host": "a.example.com"}, {"host": "b.example.com"}]}
		}`))
		if err != nil {
			t.Fatalf("normalizeSection(ingresses): %v", err)
		}
		if ing.Subtype != "ingress" {
			t.Errorf("ingress Subtype = %q", ing.Subtype)
		}
		ns := ctxID
		_ = ns
		pv, err := normalizeSection(ctxID, "persistentvolumes", []byte(`{
			"metadata": {"name": "pv1", "uid": "u-pv"},
			"spec": {"storageClassName": "std", "accessModes": ["ReadWriteOnce"], "capacity": {"storage": "10Gi"}}
		}`))
		if err != nil {
			t.Fatalf("normalizeSection(persistentvolumes): %v", err)
		}
		if pv.ExternalURN != "urn:k8s:7:pv:pv1" || pv.Kind != "storage.volume" || pv.Subtype != "persistent_volume" {
			t.Errorf("pv = %s/%s/%s", pv.ExternalURN, pv.Kind, pv.Subtype)
		}
		if pv.Normalized["capacityGB"] != 10.0 {
			t.Errorf("pv capacityGB = %v, want 10", pv.Normalized["capacityGB"])
		}
		pvc, err := normalizeSection(ctxID, "persistentvolumeclaims", []byte(`{
			"metadata": {"name": "pvc1", "namespace": "default", "uid": "u-pvc"},
			"spec": {"storageClassName": "std", "accessModes": ["ReadWriteOnce"]},
			"status": {"phase": "Bound", "capacity": {"storage": "5Gi"}}
		}`))
		if err != nil {
			t.Fatalf("normalizeSection(persistentvolumeclaims): %v", err)
		}
		if pvc.Subtype != "pvc" || pvc.Normalized["capacityGB"] != 5.0 {
			t.Errorf("pvc Subtype/capacityGB = %q/%v", pvc.Subtype, pvc.Normalized["capacityGB"])
		}
		sc, err := normalizeSection(ctxID, "storageclasses", []byte(`{
			"metadata": {"name": "standard", "uid": "u-sc"},
			"provisioner": "rancher.io/local-path"
		}`))
		if err != nil {
			t.Fatalf("normalizeSection(storageclasses): %v", err)
		}
		if sc.Kind != "storage.pool" || sc.Subtype != "storage_class" {
			t.Errorf("storageclass Kind/Subtype = %s/%s", sc.Kind, sc.Subtype)
		}
		if !contract.IsKnownResourceKind("orchestration.configmap") || !contract.IsKnownResourceKind("orchestration.secret") {
			t.Errorf("vocabulary extension not recognized by IsKnownResourceKind")
		}
	})
}

func marshalForScan(t *testing.T, m contract.JSONMap) string {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal scan tree: %v", err)
	}
	return string(b)
}

// Raw 크기 상한(§8.2 — MaxRawBytes 64KiB는 계획 도입값 A12): 초과분 절단 +
// normalized.truncated 마커.
func TestRawSizeLimitTruncates(t *testing.T) {
	big := strings.Repeat("x", 100<<10)
	raw := []byte(fmt.Sprintf(`{
		"metadata": {"name": "big", "namespace": "default", "uid": "u-big", "labels": {"big": %q}},
		"status": {"phase": "Active"}
	}`, big))
	res, err := normalizeSection(1, "namespaces", raw)
	if err != nil {
		t.Fatalf("normalizeSection: %v", err)
	}
	rawJSON := marshalForScan(t, res.Raw)
	if len(rawJSON) > MaxRawBytes {
		t.Errorf("Raw = %d bytes, exceeds MaxRawBytes %d", len(rawJSON), MaxRawBytes)
	}
	if res.Normalized["truncated"] != true {
		t.Errorf("normalized.truncated = %v, want true marker", res.Normalized["truncated"])
	}
}

// 게이트웨이 모드(J1/A4): 자격 입력(Connection.Config)으로 홉 구성이 판정되고,
// 주입된 게이트웨이 다이얼러 경로로 디스커버리가 성립한다. 다이얼러 부재 시에는
// 명시적 오류(실측은 M2).
func TestKubernetesGatewayHop(t *testing.T) {
	f := newK8sFixture(t, "v1.29.4")

	t.Run("no_dialer_errors", func(t *testing.T) {
		conn := f.Connection()
		conn.Config = contract.JSONMap{"connection_mode": "gateway", "gateway_id": "7"}
		_, err := f.adapter.Discover(context.Background(), contract.DiscoverRequest{ContextID: 1, Connection: conn})
		if err == nil || !strings.Contains(err.Error(), "gateway") {
			t.Fatalf("gateway without dialer: err = %v, want gateway configuration error", err)
		}
	})

	t.Run("injected_dialer_drives_the_hop", func(t *testing.T) {
		hops := 0
		adapter := NewAdapter(
			WithPageSize(testPageSize),
			WithGatewayDialer(func(ctx context.Context, network, addr string) (net.Conn, error) {
				hops++
				// 모의 게이트웨이: 목적지로의 TCP 홉을 대신 수행한다.
				return net.Dial(network, addr)
			}),
		)
		conn := f.Connection()
		conn.Config = contract.JSONMap{"connection_mode": "gateway", "gateway_id": "7"}
		page, err := adapter.Discover(context.Background(), contract.DiscoverRequest{ContextID: 1, Connection: conn})
		if err != nil {
			t.Fatalf("Discover via gateway hop: %v", err)
		}
		if len(page.Resources) == 0 {
			t.Fatalf("gateway hop discovery returned no resources")
		}
		if hops == 0 {
			t.Fatalf("gateway dialer was never invoked — hop path not exercised")
		}
	})

	t.Run("missing_gateway_id_errors", func(t *testing.T) {
		conn := f.Connection()
		conn.Config = contract.JSONMap{"connection_mode": "gateway"}
		_, err := f.adapter.Discover(context.Background(), contract.DiscoverRequest{ContextID: 1, Connection: conn})
		if err == nil || !strings.Contains(err.Error(), "gateway_id") {
			t.Fatalf("gateway without gateway_id: err = %v, want gateway_id error", err)
		}
	})
}

// Descriptor 계약(§3.1) — Type/ContextKinds/BuiltIn 고정값.
func TestKubernetesDescriptor(t *testing.T) {
	d := NewAdapter().Descriptor()
	if d.Type != "kubernetes" || d.AdapterVersion != "1" || d.ProtocolVersion != "1" {
		t.Errorf("Descriptor = %+v", d)
	}
	if len(d.ContextKinds) != 1 || d.ContextKinds[0] != "cluster" || !d.BuiltIn {
		t.Errorf("Descriptor ContextKinds/BuiltIn = %+v/%v", d.ContextKinds, d.BuiltIn)
	}
}

// 자격 물질 부재·잘못된 kubeconfig의 에러 경로(J2 — 어댑터는 req.Connection만
// 본다) + Validate의 신호 에러 매핑(403 → permission_denied).
func TestKubernetesCredentialAndSignalPaths(t *testing.T) {
	f := newK8sFixture(t, "v1.29.4")
	ctx := context.Background()

	t.Run("missing_material", func(t *testing.T) {
		_, err := f.adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1})
		if err == nil || !strings.Contains(err.Error(), "inventory") {
			t.Fatalf("Discover without Material[inventory]: err = %v", err)
		}
	})

	t.Run("invalid_kubeconfig", func(t *testing.T) {
		conn := contract.ConnectionView{Material: map[string]string{"inventory": "not: a: kubeconfig: ["}}
		if _, err := f.adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Connection: conn}); err == nil {
			t.Fatalf("Discover with malformed kubeconfig succeeded")
		}
	})

	t.Run("forbidden_signals_permission_denied", func(t *testing.T) {
		if err := f.Scenario(contract.SignalPermissionDenied); err != nil {
			t.Fatalf("Scenario: %v", err)
		}
		defer func() { _ = f.Scenario("") }()
		_, err := f.adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Connection: f.Connection()})
		var sig *contract.ProviderSignalError
		if !errors.As(err, &sig) || sig.Kind != contract.SignalPermissionDenied {
			t.Fatalf("403 err = %v, want permission_denied signal", err)
		}
	})
}

// Validate 경로 — kubeconfig 파싱 + /version 조회·신호 매핑(계획 §3.1).
func TestKubernetesValidate(t *testing.T) {
	f := newK8sFixture(t, "v1.31.2+k3s1")
	ctx := context.Background()

	if err := f.adapter.Validate(ctx, f.Connection()); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if err := f.Scenario(contract.SignalRateLimited); err != nil {
		t.Fatalf("Scenario: %v", err)
	}
	defer func() { _ = f.Scenario("") }()
	err := f.adapter.Validate(ctx, f.Connection())
	var sig *contract.ProviderSignalError
	if !errors.As(err, &sig) || sig.Kind != contract.SignalRateLimited {
		t.Fatalf("Validate(429) err = %v, want rate_limited signal", err)
	}
}

// Close·옵션 — 컴파일 계약(§11)과 카운터 공유면(WithCounters) 확인.
func TestKubernetesCloseAndOptions(t *testing.T) {
	shared := metrics.New()
	shared.RegisterProviders(ProviderName)
	a := NewAdapter(WithCounters(shared), WithPageSize(0))
	if a.RateCounter() != shared {
		t.Fatalf("WithCounters did not share the counter set")
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
