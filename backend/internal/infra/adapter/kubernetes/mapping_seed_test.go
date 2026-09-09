package kubernetes_test

// mapping_mock_seed_test.go — mapping_test.go의 목 K8s API 서버 시드 인프라(650행
// 계약 분할 — PC-L·§9-7). 소유: discovery 섹션 시드 본문(mappingSeedBodies), 섹션
// 경로를 서빙하는 httptest 목(mappingMock), 최소 kubeconfig, 커서 워크 헬퍼.
// 단언 논리(coverageTable·normalizedKeyFor·Test* 함수)는 mapping_test.go에 둔다.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"ops-admin/backend/internal/infra/adapter/kubernetes"
	"ops-admin/backend/internal/infra/contract"
)

// mappingSeedVersion is the /version GitVersion the mock serves — the
// cluster.version landing spot (§4.1) asserts against it.
const mappingSeedVersion = "v1.29.4"

// mappingSeedBodies maps a discovery section path to ONE representative object
// with every mapped field populated. Values are literal JSON — the envelope
// wrapping lives in mappingMock's handler. configmap·secret은 data "값"을
// 싣지 않는다(보존 제약 #7 — 어댑터 경계 폐기 검증은 internal 테스트 소관).
var mappingSeedBodies = map[string]string{
	"/version":                   `{"gitVersion":"` + mappingSeedVersion + `","major":"1","minor":"29"}`,
	"/api/v1/nodes":              `{"metadata":{"name":"node-1","uid":"node-uid-1","creationTimestamp":"2026-09-01T00:00:00Z","labels":{"node-role.kubernetes.io/control-plane":""}},"spec":{"taints":[{"key":"node-role.kubernetes.io/master","effect":"NoSchedule"}],"podCIDRs":["10.244.0.0/24"]},"status":{"conditions":[{"type":"Ready","status":"True"}],"capacity":{"cpu":"8","memory":"31457280Ki","pods":"110"},"allocatable":{"cpu":"7500m","memory":"31457280Ki","pods":"110"},"nodeInfo":{"kubeletVersion":"v1.29.4","osImage":"Ubuntu 22.04"},"addresses":[{"type":"InternalIP","address":"192.168.10.2"}]}}`,
	"/api/v1/namespaces":         `{"metadata":{"name":"default","uid":"ns-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"status":{"phase":"Active"}}`,
	"/api/v1/pods":               `{"metadata":{"name":"web-1","namespace":"default","uid":"pod-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"containers":[{"name":"app","resources":{"requests":{"cpu":"500m","memory":"256Mi"},"limits":{"cpu":"1","memory":"1Gi"}}}]},"status":{"phase":"Running","hostIP":"192.168.10.2","podIP":"10.244.0.5","containerStatuses":[{"name":"app","restartCount":2,"ready":true}]}}`,
	"/apis/apps/v1/deployments":  `{"metadata":{"name":"web","namespace":"default","uid":"dep-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"replicas":3,"template":{"spec":{"containers":[{"name":"app","image":"nginx:1","resources":{"requests":{"cpu":"250m","memory":"512Mi"},"limits":{"cpu":"2","memory":"2Gi"}}}]}}},"status":{"readyReplicas":3,"updatedReplicas":3,"availableReplicas":3}}`,
	"/apis/apps/v1/statefulsets": `{"metadata":{"name":"db","namespace":"default","uid":"sts-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"replicas":1,"template":{"spec":{"containers":[{"name":"pg","image":"postgres:16"}]}}},"status":{"readyReplicas":1,"updatedReplicas":1,"availableReplicas":1}}`,
	"/apis/apps/v1/daemonsets":   `{"metadata":{"name":"agent","namespace":"default","uid":"ds-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"template":{"spec":{"containers":[{"name":"agent","image":"agent:2"}]}}},"status":{"desiredNumberScheduled":2,"numberReady":2}}`,
	// P1-A 확장 종 — Discover 워크가 통과하는 모든 섹션 경로를 서빙해야 한다
	// (P1-C1 이후 404는 섹션 스킵 신호지만, 시드된 경로는 §4.6 mapped 행의
	// 랜딩 스폿 증명을 위해 유지한다).
	"/apis/apps/v1/replicasets":            `{"metadata":{"name":"rs-1","namespace":"default","uid":"rs-uid-1","creationTimestamp":"2026-09-01T00:00:00Z","ownerReferences":[{"uid":"dep-uid-1","kind":"Deployment","name":"web"}]},"spec":{"replicas":3,"template":{"spec":{"containers":[{"name":"app","image":"nginx:1"}]}}},"status":{"readyReplicas":3}}`,
	"/apis/batch/v1/jobs":                  `{"metadata":{"name":"job-1","namespace":"default","uid":"job-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"template":{"spec":{"containers":[{"name":"app","image":"busybox:1"}]}}},"status":{"succeeded":1}}`,
	"/apis/batch/v1/cronjobs":              `{"metadata":{"name":"cron-1","namespace":"default","uid":"cron-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"schedule":"*/5 * * * *"}}`,
	"/api/v1/endpoints":                    `{"metadata":{"name":"web-svc","namespace":"default","uid":"ep-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"subsets":[{"addresses":[{"ip":"10.244.0.5"},{"ip":"10.244.0.6"}]}]}`,
	"/api/v1/services":                     `{"metadata":{"name":"web-svc","namespace":"default","uid":"svc-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"type":"LoadBalancer","clusterIP":"10.96.0.10","ports":[{"name":"http","port":80,"protocol":"TCP"}],"externalIPs":["198.51.100.4"]},"status":{"loadBalancer":{"ingress":[{"ip":"203.0.113.7"}]}}}`,
	"/apis/networking.k8s.io/v1/ingresses": `{"metadata":{"name":"web-ing","namespace":"default","uid":"ing-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"rules":[{"host":"app.example.com"}],"tls":[{"hosts":["app.example.com"]}]},"status":{"loadBalancer":{"ingress":[{"ip":"203.0.113.9"}]}}}`,
	// P1-C1 확장 종 — §4.8 mapped 전환의 랜딩 스폿(network.gateway hosts·
	// network.http_route parents)이 실제 수집 체인에서 산출됨을 증명하는 시드.
	// v1 경로를 서빙한다(선호 버전 — 폴백 검증은 normalizer_gw_test.go 소관).
	"/apis/gateway.networking.k8s.io/v1/gateways":   `{"metadata":{"name":"gw-seed","namespace":"default","uid":"gw-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"gatewayClassName":"istio","listeners":[{"name":"http","hostname":"app.example.com","port":80,"protocol":"HTTP"}]},"status":{"addresses":[{"type":"IPAddress","value":"203.0.113.7"}]}}`,
	"/apis/gateway.networking.k8s.io/v1/httproutes": `{"metadata":{"name":"route-seed","namespace":"default","uid":"rt-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"parentRefs":[{"name":"gw-seed"}],"rules":[{"backendRefs":[{"name":"web-svc","port":80,"weight":100}]}]}}`,
	"/api/v1/configmaps":                            `{"metadata":{"name":"cm-1","namespace":"default","uid":"cm-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"data":{"k1":"v1","k2":"v2"}}`,
	"/api/v1/secrets":                               `{"metadata":{"name":"sec-1","namespace":"default","uid":"sec-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"type":"Opaque","data":{"tk":"dg=="}}`,
	"/api/v1/persistentvolumes":                     `{"metadata":{"name":"pv-1","uid":"pv-uid-1","creationTimestamp":"2026-09-01T00:00:00Z","annotations":{"ops-admin.io/namespace-scope":"team-a"}},"spec":{"storageClassName":"nfs","accessModes":["ReadWriteMany"],"capacity":{"storage":"1Gi"},"nfs":{"server":"10.0.0.9","path":"/exports/team-a"},"persistentVolumeReclaimPolicy":"Retain"},"status":{"phase":"Bound"}},{"metadata":{"name":"pv-host","uid":"pv-uid-2","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"storageClassName":"standard","accessModes":["ReadWriteOnce"],"capacity":{"storage":"1Gi"},"hostPath":{"path":"/data"},"persistentVolumeReclaimPolicy":"Delete"},"status":{"phase":"Available"}}`,
	"/api/v1/persistentvolumeclaims":                `{"metadata":{"name":"pvc-1","namespace":"default","uid":"pvc-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"storageClassName":"standard","accessModes":["ReadWriteOnce"]},"status":{"phase":"Bound","capacity":{"storage":"1Gi"}}}`,
	"/apis/storage.k8s.io/v1/storageclasses":        `{"metadata":{"name":"standard","uid":"sc-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"provisioner":"kubernetes.io/aws-ebs"}`,
}

// mappingMock serves the discovery section paths (single page, continue "") —
// the §14.1 raw REST envelope the adapter's client decodes. P1-C1 이후 404는
// 섹션 스킵 신호(J-P1-1)라 GatewayAPI CRD 부재 클러스터 변형도 통과하지만, §4.8
// mapped 전환의 랜딩 스폿 증명을 위해 gateway 2종은 v1 경로를 서빙한다(나머지
// 17개 섹션 경로 전부 서빙 — storageclass는 mapped 필드가 없지만 Discover
// 워크가 통과한다).
type mappingMock struct {
	srv *httptest.Server
}

func newMappingMock(t *testing.T) *mappingMock {
	m := &mappingMock{}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := mappingSeedBodies[r.URL.Path]
		if !ok {
			http.Error(w, "mapping mock: no seed for "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/version" {
			_, _ = w.Write([]byte(body))
			return
		}
		_, _ = w.Write([]byte(`{"metadata":{"continue":"","resourceVersion":"1"},"items":[` + body + `]}`))
	}))
	t.Cleanup(m.srv.Close)
	return m
}

// mappingKubeconfig — the minimal kubeconfig the parser resolves (내부 테스트의
// kubeconfigFor와 무의존 재현 — external test package라 내부 mock에 접근 불가).
func mappingKubeconfig(server string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Config
current-context: mapping-test
clusters:
  - name: test
    cluster:
      server: %s
contexts:
  - name: mapping-test
    context:
      cluster: test
      user: test
users:
  - name: test
    user:
      token: test-token
`, server)
}

// discoverForNormalization walks the adapter's full cursor chain against the
// mock and returns every discovered resource — the normalization output G-P1b
// asserts against.
func discoverForNormalization(t *testing.T, adapter *kubernetes.Adapter, conn contract.ConnectionView) []contract.DiscoveredResource {
	t.Helper()
	resources := make([]contract.DiscoveredResource, 0, 16)
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 64 { // 17 섹션 상한의 방어 여유 — 커서 루프 비정상 반복 조기 적색
			t.Fatalf("discoverForNormalization: cursor chain exceeded 64 pages — non-terminating walk")
		}
		page, err := adapter.Discover(context.Background(), contract.DiscoverRequest{
			ContextID:  1,
			Cursor:     cursor,
			Connection: conn,
		})
		if err != nil {
			t.Fatalf("Discover(cursor=%q): %v", cursor, err)
		}
		resources = append(resources, page.Resources...)
		if page.NextCursor == "" {
			return resources
		}
		cursor = page.NextCursor
	}
}
