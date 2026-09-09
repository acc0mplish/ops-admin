// T51 (plan §6, PR 22 single ownership — §12 r2): the mapping document and
// the Go compare engine stay in mechanical equivalence — every field the
// legacy API response serializes is classified (mapped / dropped(reason) /
// v2-only) both in mapping.md §4 and in the coverage table below, the pairing
// key table (§3.1) and the comparison scope table (§3.2) agree with the
// engine's scope dispositions.
//
// Package note: the file lives in the adapter package directory (mapping.md
// lives next to the normalizer — §15.2), but it is an external test package
// that exercises the inventory compare engine — it owns no adapter code.
package kubernetes_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/adapter/kubernetes"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/model"
)

func mappingDoc(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("mapping.md")
	if err != nil {
		t.Fatalf("read mapping.md: %v", err)
	}
	return string(b)
}

// legacyFieldNames extracts the JSON-serialized field names of one section
// item struct (the wire form mapping.md's coverage tables use).
func legacyFieldNames(t *testing.T, item any) []string {
	t.Helper()
	typ := reflect.TypeOf(item)
	names := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "" || name == "-" {
			t.Fatalf("section item %s field %s has no json name", typ.Name(), typ.Field(i).Name)
		}
		names = append(names, name)
	}
	return names
}

// coverageTable — the Go side of the §15.2 coverage rule: every legacy
// serialized field with its disposition (mapped / dropped(reason) / v2-only).
// Keys are "<section-path>.<json-field>"; values are the disposition class
// recorded in mapping.md §4.
var coverageTable = map[string]string{
	// §4.1 cluster
	"cluster.id": "dropped(v2-source-key)", "cluster.name": "dropped(v2-source-key)",
	"cluster.status": "dropped(v2-source-key)", "cluster.statusText": "dropped(v2-source-key)",
	"cluster.apiServer": "dropped(v2-source-key)", "cluster.version": "mapped",
	"cluster.nodeCount": "dropped(v2-derived)", "cluster.env": "dropped(v2-source-key)",
	"cluster.tags": "dropped(v2-source-key)", "cluster.connectionMode": "dropped(v2-source-key)",
	"cluster.gatewayId": "dropped(v2-source-key)", "cluster.gatewayName": "dropped(v2-source-key)",
	"cluster.monitorDatasourceId": "dropped(monitoring-domain)", "cluster.monitorDatasourceName": "dropped(monitoring-domain)",
	"cluster.description": "dropped(v2-source-key)", "cluster.lastSyncAt": "dropped(volatile)",
	"cluster.createTime": "dropped(volatile)", "cluster.updateTime": "dropped(volatile)",
	// §4.2 overview
	"overview.healthScore": "dropped(monitoring-derived)", "overview.cpuUsage": "dropped(monitoring-derived)",
	"overview.memoryUsage": "dropped(monitoring-derived)", "overview.podUsage": "dropped(monitoring-derived)",
	"overview.requestRate": "dropped(monitoring-derived)", "overview.alertCount": "dropped(monitoring-derived)",
	"overview.distribution": "dropped(v2-schema-absent)", "overview.certificates": "dropped(v2-schema-absent)",
	// §4.3 nodes
	"nodes.name": "mapped", "nodes.role": "mapped", "nodes.status": "mapped",
	"nodes.version": "dropped(v2-schema-absent)", "nodes.internalIP": "dropped(v2-schema-absent)",
	"nodes.os": "dropped(v2-schema-absent)", "nodes.cpu": "mapped", "nodes.memory": "mapped",
	"nodes.pods": "dropped(v2-derived)",
	// §4.4 namespaces
	"namespaces.name": "mapped", "namespaces.status": "mapped", "namespaces.createdAt": "mapped",
	"namespaces.pods": "dropped(v2-derived)", "namespaces.services": "dropped(v2-derived)",
	"namespaces.workloads": "dropped(v2-derived)",
	// §4.5 pods
	"pods.name": "mapped", "pods.namespace": "mapped", "pods.status": "mapped",
	"pods.node": "mapped(relationship)", "pods.restarts": "mapped",
	"pods.workloadName": "dropped(v2-derived)", "pods.workloadType": "dropped(v2-derived)",
	"pods.nodeIP": "dropped(v2-schema-absent)", "pods.ip": "dropped(v2-schema-absent)",
	"pods.age": "dropped(volatile)",
	// §4.6 workloads
	"workloads.name": "mapped", "workloads.type": "mapped", "workloads.namespace": "mapped",
	"workloads.ready": "mapped", "workloads.image(v2-only)": "v2-only",
	"workloads.updated": "dropped(v2-schema-absent)", "workloads.available": "dropped(v2-schema-absent)",
	"workloads.requests": "dropped(v2-schema-absent)", "workloads.limits": "dropped(v2-schema-absent)",
	"workloads.age": "dropped(volatile)",
	// §4.7 network.services
	"network.services.name": "mapped", "network.services.namespace": "mapped",
	"network.services.type": "mapped", "network.services.clusterIP": "mapped",
	"network.services.ports": "mapped", "network.services.externalIP": "dropped(v2-schema-absent)",
	"network.services.endpoints": "dropped(v2-derived)", "network.services.age": "dropped(volatile)",
	// §4.7 network.ingresses
	"network.ingresses.name": "mapped", "network.ingresses.namespace": "mapped",
	"network.ingresses.host": "mapped", "network.ingresses.address": "dropped(v2-schema-absent)",
	"network.ingresses.tls": "dropped(v2-schema-absent)", "network.ingresses.age": "dropped(volatile)",
	// §4.8 advancedNetwork
	"advancedNetwork.gatewayApiGateways": "dropped(v2-not-collected)",
	"advancedNetwork.httpRoutes":         "dropped(v2-not-collected)",
	// §4.9 configStorage.configMaps
	"configStorage.configMaps.name": "mapped", "configStorage.configMaps.namespace": "mapped",
	"configStorage.configMaps.keys": "mapped", "configStorage.configMaps.age": "dropped(volatile)",
	// §4.9 configStorage.secrets — the legacy LIST item serializes
	// name/namespace/type/age only; dataKeys는 v2-only (리스트에 keys 필드 부재 —
	// 실측 정정).
	"configStorage.secrets.name": "mapped", "configStorage.secrets.namespace": "mapped",
	"configStorage.secrets.type": "mapped", "configStorage.secrets.keys": "v2-only",
	"configStorage.secrets.age": "dropped(volatile)",
	// §4.9 configStorage.storage
	"configStorage.storage.name": "mapped", "configStorage.storage.kind": "mapped",
	"configStorage.storage.namespace": "mapped", "configStorage.storage.capacity": "mapped",
	"configStorage.storage.storageClass": "mapped", "configStorage.storage.accessModes": "mapped",
	"configStorage.storage.namespaceScope": "dropped(v2-derived)",
	"configStorage.storage.status":         "dropped(v2-schema-absent)",
	"configStorage.storage.sourceType":     "dropped(v2-schema-absent)",
	"configStorage.storage.path":           "dropped(v2-schema-absent)",
	"configStorage.storage.nfsServer":      "dropped(v2-schema-absent)",
	"configStorage.storage.reclaimPolicy":  "dropped(v2-schema-absent)",
}

// T51 — coverage rule: every field the legacy API response serializes is
// classified in the Go table AND recorded in mapping.md.
func TestMappingTableCoversLegacyFields(t *testing.T) {
	doc := mappingDoc(t)

	sections := []struct {
		path string
		item any
	}{
		{"cluster.", model.K8sClusterView{}},
		{"overview.", model.K8sOverview{}},
		{"nodes.", model.K8sNodeItem{}},
		{"namespaces.", model.K8sNamespaceItem{}},
		{"pods.", model.K8sPodItem{}},
		{"workloads.", model.K8sWorkloadItem{}},
		{"network.services.", model.K8sServiceItem{}},
		{"network.ingresses.", model.K8sIngressItem{}},
		{"advancedNetwork.", model.K8sAdvancedNetworkSection{}},
		{"configStorage.configMaps.", model.K8sConfigMapItem{}},
		{"configStorage.secrets.", model.K8sSecretItem{}},
		{"configStorage.storage.", model.K8sStorageItem{}},
	}

	for _, section := range sections {
		for _, field := range legacyFieldNames(t, section.item) {
			key := section.path + field
			disposition, ok := coverageTable[key]
			if !ok {
				t.Errorf("legacy field %s is not classified in the coverage table", key)
				continue
			}
			if !strings.Contains(doc, field) {
				t.Errorf("field %s (%s) is absent from mapping.md", field, key)
			}
			if !strings.Contains(doc, disposition) {
				t.Errorf("disposition %q of %s is absent from mapping.md", disposition, key)
			}
		}
	}
}

// T51 — comparison scope table (mapping.md §3.2) matches the engine's
// dispositions: 12 comparable kinds, 4 single-side kinds.
func TestComparisonScopeTableMatchesEngine(t *testing.T) {
	doc := mappingDoc(t)

	comparable := []string{
		"node", "namespace", "pod", "deployment", "statefulset", "daemonset",
		"service", "ingress", "configmap", "secret", "pv", "pvc",
	}
	for _, section := range comparable {
		if got := inventory.ScopeDisposition(section); got != "compare" {
			t.Errorf("section %q disposition = %q, want compare", section, got)
		}
	}
	legacyOnly := map[string]string{
		"replicaset": "dropped(v2-not-collected)",
		"job":        "dropped(v2-not-collected)",
		"cronjob":    "dropped(v2-not-collected)",
		// storageclass는 v2-only이지만 legacy 표기의 처분 어휘도 §3.2 표에 있다.
		"storageclass": "v2-only",
	}
	for section, want := range legacyOnly {
		if got := inventory.ScopeDisposition(section); got != want {
			t.Errorf("section %q disposition = %q, want %q", section, got, want)
		}
	}

	// mapping.md documents all four single-side rows.
	for _, token := range []string{"ReplicaSet", "Job", "CronJob", "storageclass", "dropped(v2-not-collected)", "v2-only"} {
		if !strings.Contains(doc, token) {
			t.Errorf("scope token %q absent from mapping.md §3.2", token)
		}
	}
}

// T51 — pairing key table (mapping.md §3.1): the namespace/name rule and the
// name rule for cluster-scoped kinds are documented and enforced.
func TestPairingKeyTableMatchesEngine(t *testing.T) {
	doc := mappingDoc(t)
	for _, token := range []string{"namespace/name", "페어링 키"} {
		if !strings.Contains(doc, token) {
			t.Errorf("pairing key token %q absent from mapping.md §3.1", token)
		}
	}

	sections := []string{
		"node", "namespace", "pod", "deployment", "statefulset", "daemonset",
		"service", "ingress", "configmap", "secret", "pv", "pvc",
	}
	for _, section := range sections {
		if inventory.PairingKeyShape(section) == "" {
			t.Errorf("section %q has no pairing key shape", section)
		}
		if !strings.Contains(doc, inventory.PairingKeyShape(section)) {
			t.Errorf("pairing shape %q of %q absent from mapping.md", inventory.PairingKeyShape(section), section)
		}
	}
}

// --- G-P1b (V-4 상환) — mapped 필드의 정규화 산출 검증 ---
//
// handover §3 G-P1b: the doc-substring checks above verify Go-table ↔
// mapping.md equivalence, but "Go coverageTable 값과 .md 텍스트를 함께 고치면
// 수집기 0개로도 녹색". TestMappedFieldsProduceNormalizedKeys 봉쇄 그 구멍:
// disposition "mapped" 필드마다 실제 수집기 체인(Adapter.Discover 전체 커서
// 워크 over a mock K8s API server)의 정규화 산출물에 랜딩 스폿이 실재함을
// 단얫한다 — Go 표·문서를 함께 고쳐도 정규화기가 키를 안 내면 적색.

// normalizedKeyFor returns the normalization-output landing spot mapping.md
// §4 records for a "mapped" coverage key, as a target descriptor:
//
//	"display"          → DiscoveredResource.DisplayName (신원·페어링 키 성분)
//	"subtype"          → DiscoveredResource.Subtype
//	"raw:<key>"        → Raw[key] (namespace·creationTimestamp 성분)
//	"normalized:<key>" → Normalized[key]
//	"@version-probe"   → cluster.version(§4.1) — connection 속성: Health/
//	                     Validate /version GitVersion 착지, 리소스 행 아님
//
// Unlisted keys return "" — TestMappedFieldsProduceNormalizedKeys iterates
// coverageTable, so a future mapped row without a recorded landing spot fails
// loudly instead of silently skipping (테스트 자체의 공허 통과 봉쇄).
func normalizedKeyFor(coverageKey string) string {
	switch coverageKey {
	// §4.1 cluster — connection 속성, Health/Validate /version 착지.
	case "cluster.version":
		return "@version-probe"
	// §4.3 nodes
	case "nodes.name":
		return "display"
	case "nodes.role":
		return "normalized:roles"
	case "nodes.status":
		return "normalized:healthState"
	case "nodes.cpu":
		return "normalized:capacityCoresGB"
	case "nodes.memory":
		return "normalized:capacityMemoryGB"
	// §4.4 namespaces
	case "namespaces.name":
		return "display"
	case "namespaces.status":
		return "normalized:phase"
	case "namespaces.createdAt":
		return "raw:creationTimestamp"
	// §4.5 pods — pods.node는 mapped(relationship)이라 이 검증 밖(관계 데이터).
	case "pods.name":
		return "display"
	case "pods.namespace":
		return "raw:namespace"
	case "pods.status":
		return "normalized:phase"
	case "pods.restarts":
		return "normalized:restartCount"
	// §4.6 workloads
	case "workloads.name":
		return "display"
	case "workloads.namespace":
		return "raw:namespace"
	case "workloads.type":
		return "subtype"
	case "workloads.ready":
		return "normalized:readyReplicas"
	// §4.7 network.services
	case "network.services.name":
		return "display"
	case "network.services.namespace":
		return "raw:namespace"
	case "network.services.type":
		return "normalized:type"
	case "network.services.clusterIP":
		return "normalized:clusterIP"
	case "network.services.ports":
		return "normalized:ports"
	// §4.7 network.ingresses
	case "network.ingresses.name":
		return "display"
	case "network.ingresses.namespace":
		return "raw:namespace"
	case "network.ingresses.host":
		return "normalized:hosts"
	// §4.9 configStorage.configMaps
	case "configStorage.configMaps.name":
		return "display"
	case "configStorage.configMaps.namespace":
		return "raw:namespace"
	case "configStorage.configMaps.keys":
		return "normalized:dataKeys"
	// §4.9 configStorage.secrets
	case "configStorage.secrets.name":
		return "display"
	case "configStorage.secrets.namespace":
		return "raw:namespace"
	case "configStorage.secrets.type":
		return "normalized:type"
	case "configStorage.secrets.keys":
		return "normalized:dataKeys"
	// §4.9 configStorage.storage — pv+pvc 2종(namespace는 pvc만 보유, 정합).
	case "configStorage.storage.name":
		return "display"
	case "configStorage.storage.kind":
		return "subtype"
	case "configStorage.storage.namespace":
		return "raw:namespace"
	case "configStorage.storage.capacity":
		return "normalized:capacityGB"
	case "configStorage.storage.storageClass":
		return "normalized:storageClassName"
	case "configStorage.storage.accessModes":
		return "normalized:accessModes"
	default:
		return ""
	}
}

// normalizedKeyFamily routes a mapped coverage key to the discovered resource
// family (kind, subtype) whose section produces the landing spot. The empty
// kind is the @version-probe exception — connection 속성에는 리소스 계열이 없다.
func normalizedKeyFamily(coverageKey string) (kind, subtype string) {
	switch {
	case strings.HasPrefix(coverageKey, "cluster."):
		return "", ""
	case strings.HasPrefix(coverageKey, "nodes."):
		return "orchestration.node", ""
	case strings.HasPrefix(coverageKey, "namespaces."):
		return "orchestration.namespace", ""
	case strings.HasPrefix(coverageKey, "pods."):
		return "orchestration.pod", ""
	case strings.HasPrefix(coverageKey, "workloads."):
		return "orchestration.workload", ""
	case strings.HasPrefix(coverageKey, "network.services."):
		return "network.load_balancer", "service"
	case strings.HasPrefix(coverageKey, "network.ingresses."):
		return "network.load_balancer", "ingress"
	case strings.HasPrefix(coverageKey, "configStorage.configMaps."):
		return "orchestration.configmap", ""
	case strings.HasPrefix(coverageKey, "configStorage.secrets."):
		return "orchestration.secret", ""
	case strings.HasPrefix(coverageKey, "configStorage.storage."):
		return "storage.volume", ""
	default:
		return "", ""
	}
}

// landingSpotProduced reports whether one discovered resource actually
// produces the target descriptor — the mechanical leg of the V-4 remedy.
func landingSpotProduced(target string, res contract.DiscoveredResource) bool {
	switch target {
	case "display":
		return res.DisplayName != ""
	case "subtype":
		return res.Subtype != ""
	}
	comp, key, ok := strings.Cut(target, ":")
	if !ok {
		return false // "@version-probe" 등 리소스 성분이 아닌 랜딩 스폿
	}
	switch comp {
	case "raw":
		_, present := res.Raw[key]
		return present
	case "normalized":
		_, present := res.Normalized[key]
		return present
	}
	return false
}

// familyOf returns the discovered resources of one mapped field's section —
// kind exact, subtype ""은 와일드카드(workload 3종·volume 2종은 subtype별로
// 적재되므로 계열 조회는 와일드카드로 합친다).
func familyOf(families map[string][]contract.DiscoveredResource, kind, subtype string) []contract.DiscoveredResource {
	family := []contract.DiscoveredResource{}
	for key, res := range families {
		gk, gs, _ := strings.Cut(key, "/")
		if gk == kind && (subtype == "" || gs == subtype) {
			family = append(family, res...)
		}
	}
	return family
}

// assertNormalizedKey asserts the landing spot normalizedKeyFor records for
// coverageKey is actually produced by the normalization output of the mapped
// field's resource family — at least one family member must produce it (pv는
// 클러스터 스코프라 namespace가 없는 정합 케이스를 at-least-one이 흡수).
func assertNormalizedKey(t *testing.T, coverageKey string, family []contract.DiscoveredResource) {
	t.Helper()
	target := normalizedKeyFor(coverageKey)
	if target == "" {
		t.Errorf("coverage key %q is \"mapped\" in coverageTable but normalizedKeyFor records no landing spot — add the mapping.md §4 target", coverageKey)
		return
	}
	kind, subtype := normalizedKeyFamily(coverageKey)
	if kind == "" {
		t.Errorf("coverage key %q: landing spot %q has no resource family — extend normalizedKeyFamily", coverageKey, target)
		return
	}
	if len(family) == 0 {
		t.Errorf("coverage key %q: resource family %s/%s produced no resources — collection or seed broke (G-P1b)", coverageKey, kind, subtype)
		return
	}
	for _, res := range family {
		if landingSpotProduced(target, res) {
			return
		}
	}
	t.Errorf("coverage key %q: landing spot %q produced by none of %d %s resources — collect first, classify second (G-P1b)", coverageKey, target, len(family), kind)
}

// mappingSeedVersion is the /version GitVersion the mock serves — the
// cluster.version landing spot (§4.1) asserts against it.
const mappingSeedVersion = "v1.29.4"

// mappingSeedBodies maps a discovery section path to ONE representative object
// with every mapped field populated. Values are literal JSON — the envelope
// wrapping lives in mappingMock's handler. configmap·secret은 data "값"을
// 싣지 않는다(보존 제약 #7 — 어댑터 경계 폐기 검증은 internal 테스트 소관).
var mappingSeedBodies = map[string]string{
	"/version":                   `{"gitVersion":"` + mappingSeedVersion + `","major":"1","minor":"29"}`,
	"/api/v1/nodes":              `{"metadata":{"name":"node-1","uid":"node-uid-1","creationTimestamp":"2026-09-01T00:00:00Z","labels":{"node-role.kubernetes.io/control-plane":""}},"spec":{"taints":[{"key":"node-role.kubernetes.io/master","effect":"NoSchedule"}]},"status":{"conditions":[{"type":"Ready","status":"True"}],"capacity":{"cpu":"8","memory":"31457280Ki","pods":"110"},"allocatable":{"cpu":"7500m","memory":"31457280Ki"}}}`,
	"/api/v1/namespaces":         `{"metadata":{"name":"default","uid":"ns-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"status":{"phase":"Active"}}`,
	"/api/v1/pods":               `{"metadata":{"name":"web-1","namespace":"default","uid":"pod-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"status":{"phase":"Running","containerStatuses":[{"name":"app","restartCount":2,"ready":true}]}}`,
	"/apis/apps/v1/deployments":  `{"metadata":{"name":"web","namespace":"default","uid":"dep-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"replicas":3,"template":{"spec":{"containers":[{"name":"app","image":"nginx:1"}]}}},"status":{"readyReplicas":3}}`,
	"/apis/apps/v1/statefulsets": `{"metadata":{"name":"db","namespace":"default","uid":"sts-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"replicas":1,"template":{"spec":{"containers":[{"name":"pg","image":"postgres:16"}]}}},"status":{"readyReplicas":1}}`,
	"/apis/apps/v1/daemonsets":   `{"metadata":{"name":"agent","namespace":"default","uid":"ds-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"template":{"spec":{"containers":[{"name":"agent","image":"agent:2"}]}}},"status":{"desiredNumberScheduled":2,"numberReady":2}}`,
	// P1-A 확장 종 — Discover 워크가 통과하는 모든 섹션 경로를 서빙해야 한다
	// (404는 일반 에러 신호 — J-P1-1 404-스킵은 P1-C1 client.go 소관).
	"/apis/apps/v1/replicasets":              `{"metadata":{"name":"rs-1","namespace":"default","uid":"rs-uid-1","creationTimestamp":"2026-09-01T00:00:00Z","ownerReferences":[{"uid":"dep-uid-1","kind":"Deployment","name":"web"}]},"spec":{"replicas":3,"template":{"spec":{"containers":[{"name":"app","image":"nginx:1"}]}}},"status":{"readyReplicas":3}}`,
	"/apis/batch/v1/jobs":                    `{"metadata":{"name":"job-1","namespace":"default","uid":"job-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"template":{"spec":{"containers":[{"name":"app","image":"busybox:1"}]}}},"status":{"succeeded":1}}`,
	"/apis/batch/v1/cronjobs":                `{"metadata":{"name":"cron-1","namespace":"default","uid":"cron-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"schedule":"*/5 * * * *"}}`,
	"/api/v1/endpoints":                      `{"metadata":{"name":"web-svc","namespace":"default","uid":"ep-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"subsets":[{"addresses":[{"ip":"10.244.0.5"},{"ip":"10.244.0.6"}]}]}`,
	"/api/v1/services":                       `{"metadata":{"name":"web-svc","namespace":"default","uid":"svc-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"type":"ClusterIP","clusterIP":"10.96.0.10","ports":[{"name":"http","port":80,"protocol":"TCP"}]}}`,
	"/apis/networking.k8s.io/v1/ingresses":   `{"metadata":{"name":"web-ing","namespace":"default","uid":"ing-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"rules":[{"host":"app.example.com"}]}}`,
	"/api/v1/configmaps":                     `{"metadata":{"name":"cm-1","namespace":"default","uid":"cm-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"data":{"k1":"v1","k2":"v2"}}`,
	"/api/v1/secrets":                        `{"metadata":{"name":"sec-1","namespace":"default","uid":"sec-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"type":"Opaque","data":{"tk":"dg=="}}`,
	"/api/v1/persistentvolumes":              `{"metadata":{"name":"pv-1","uid":"pv-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"storageClassName":"standard","accessModes":["ReadWriteOnce"],"capacity":{"storage":"1Gi"}}}`,
	"/api/v1/persistentvolumeclaims":         `{"metadata":{"name":"pvc-1","namespace":"default","uid":"pvc-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"spec":{"storageClassName":"standard","accessModes":["ReadWriteOnce"]},"status":{"phase":"Bound","capacity":{"storage":"1Gi"}}}`,
	"/apis/storage.k8s.io/v1/storageclasses": `{"metadata":{"name":"standard","uid":"sc-uid-1","creationTimestamp":"2026-09-01T00:00:00Z"},"provisioner":"kubernetes.io/aws-ebs"}`,
}

// mappingMock serves the discovery section paths (single page, continue "") —
// the §14.1 raw REST envelope the adapter's client decodes. 404는 일반 에러
// 신호라 17개 섹션 경로를 전부 서빙해야 한다(storageclasses·P1-A 확장 4종
// 포함 — storageclass는 mapped 필드가 없지만 Discover 워크가 통과한다).
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

// TestMappedFieldsProduceNormalizedKeys — G-P1b (V-4 상환, 명명 고정): every
// disposition-"mapped" row in coverageTable must be PRODUCED by the real
// collection chain, not merely named in mapping.md and the Go table.
func TestMappedFieldsProduceNormalizedKeys(t *testing.T) {
	mock := newMappingMock(t)
	adapter := kubernetes.NewAdapter()
	conn := contract.ConnectionView{
		Material: map[string]string{"inventory": mappingKubeconfig(mock.srv.URL)},
	}
	resources := discoverForNormalization(t, adapter, conn)
	if len(resources) == 0 {
		t.Fatalf("discover produced no resources — the normalization output is empty (G-P1b)")
	}

	// Family grouping: "<kind>/<subtype>" → the section's resources.
	families := map[string][]contract.DiscoveredResource{}
	for _, res := range resources {
		key := res.Kind + "/" + res.Subtype
		families[key] = append(families[key], res)
	}

	// Iterate coverageTable — a future mapped row without a recorded landing
	// spot fails instead of silently skipping.
	for key, disposition := range coverageTable {
		if disposition != "mapped" {
			continue // dropped·v2-only·mapped(relationship) 행은 이 검증 밖
		}
		if normalizedKeyFor(key) == "@version-probe" {
			continue // cluster.version — Health 프로브로 아래에서 검증
		}
		kind, subtype := normalizedKeyFamily(key)
		assertNormalizedKey(t, key, familyOf(families, kind, subtype))
	}

	// cluster.version (§4.1) — connection 속성: the /version GitVersion the
	// Health probe surfaces IS the V2 landing spot, not a resource field.
	health := adapter.Health(context.Background(), conn)
	if !health.Healthy {
		t.Errorf("cluster.version: Health probe unhealthy: %s", health.Message)
	}
	if !strings.Contains(health.Message, mappingSeedVersion) {
		t.Errorf("cluster.version: Health message %q does not surface the seeded GitVersion %q — /version landing spot broke", health.Message, mappingSeedVersion)
	}

	// Subtype 완전성 — the seed's workload kinds (P1-A 확장: replicaset·job·
	// cronjob 포함) + 2 volume kinds anchor the §4.6 type row and §4.9 kind
	// row (at-least-one 단얫의 근거가 되는 기수).
	wantSubtypes := map[string][]string{
		"orchestration.workload": {"deployment", "statefulset", "daemonset", "replicaset", "job", "cronjob"},
		"storage.volume":         {"persistent_volume", "pvc"},
	}
	for kind, wants := range wantSubtypes {
		got := map[string]bool{}
		for _, res := range resources {
			if res.Kind == kind {
				got[res.Subtype] = true
			}
		}
		for _, w := range wants {
			if !got[w] {
				t.Errorf("kind %s: subtype %q absent from the discovered set (§4.6/§4.9 mapped rows anchor on it)", kind, w)
			}
		}
	}
}
