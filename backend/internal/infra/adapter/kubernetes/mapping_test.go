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
	"nodes.version": "mapped", "nodes.internalIP": "mapped",
	"nodes.os": "mapped", "nodes.cpu": "mapped", "nodes.memory": "mapped",
	"nodes.pods": "dropped(v2-derived)",
	// §4.4 namespaces
	"namespaces.name": "mapped", "namespaces.status": "mapped", "namespaces.createdAt": "mapped",
	"namespaces.pods": "dropped(v2-derived)", "namespaces.services": "dropped(v2-derived)",
	"namespaces.workloads": "dropped(v2-derived)",
	// §4.5 pods
	"pods.name": "mapped", "pods.namespace": "mapped", "pods.status": "mapped",
	"pods.node": "mapped(relationship)", "pods.restarts": "mapped",
	"pods.workloadName": "dropped(v2-derived)", "pods.workloadType": "dropped(v2-derived)",
	"pods.nodeIP": "mapped", "pods.ip": "mapped",
	"pods.age": "dropped(volatile)",
	// §4.6 workloads
	"workloads.name": "mapped", "workloads.type": "mapped", "workloads.namespace": "mapped",
	"workloads.ready": "mapped", "workloads.image(v2-only)": "v2-only",
	"workloads.updated": "mapped", "workloads.available": "mapped",
	"workloads.requests": "mapped", "workloads.limits": "mapped",
	"workloads.age": "dropped(volatile)",
	// §4.7 network.services
	"network.services.name": "mapped", "network.services.namespace": "mapped",
	"network.services.type": "mapped", "network.services.clusterIP": "mapped",
	"network.services.ports": "mapped", "network.services.externalIP": "mapped",
	"network.services.endpoints": "dropped(v2-derived)", "network.services.age": "dropped(volatile)",
	// §4.7 network.ingresses
	"network.ingresses.name": "mapped", "network.ingresses.namespace": "mapped",
	"network.ingresses.host": "mapped", "network.ingresses.address": "mapped",
	"network.ingresses.tls": "mapped", "network.ingresses.age": "dropped(volatile)",
	// §4.8 advancedNetwork — P1-C1 수집 착지로 mapped 전환(P1-C2 단일 착지점에서
	// doc+표 동시 전환). 비교 집합 불참은 §3.2 스코프 표(I-P5 이월)가 담당한다.
	"advancedNetwork.gatewayApiGateways": "mapped",
	"advancedNetwork.httpRoutes":         "mapped",
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
	"configStorage.storage.namespaceScope": "mapped",
	"configStorage.storage.status":         "mapped",
	"configStorage.storage.sourceType":     "mapped",
	"configStorage.storage.path":           "mapped",
	"configStorage.storage.nfsServer":      "mapped",
	"configStorage.storage.reclaimPolicy":  "mapped",
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
// dispositions: 14 comparable kinds (P1-C2로 job·cronjob 합류), 2 single-side
// kinds.
func TestComparisonScopeTableMatchesEngine(t *testing.T) {
	doc := mappingDoc(t)

	comparable := []string{
		"node", "namespace", "pod", "deployment", "statefulset", "daemonset",
		"job", "cronjob", // P1-C2 비교 집합 합류(J-P1-2)
		"service", "ingress", "configmap", "secret", "pv", "pvc",
	}
	for _, section := range comparable {
		if got := inventory.ScopeDisposition(section); got != "compare" {
			t.Errorf("section %q disposition = %q, want compare", section, got)
		}
	}
	legacyOnly := map[string]string{
		"replicaset": "dropped(v2-not-collected)",
		// storageclass는 v2-only이지만 legacy 표기의 처분 어휘도 §3.2 표에 있다.
		"storageclass": "v2-only",
	}
	for section, want := range legacyOnly {
		if got := inventory.ScopeDisposition(section); got != want {
			t.Errorf("section %q disposition = %q, want %q", section, got, want)
		}
	}

	// mapping.md documents all single-side rows.
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
	case "nodes.version":
		return "normalized:kubeletVersion"
	case "nodes.internalIP":
		return "normalized:internalIP"
	case "nodes.os":
		return "normalized:osImage"
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
	case "pods.nodeIP":
		return "normalized:hostIP"
	case "pods.ip":
		return "normalized:podIP"
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
	case "workloads.updated":
		return "normalized:updatedReplicas"
	case "workloads.available":
		return "normalized:availableReplicas"
	case "workloads.requests", "workloads.limits":
		// 원시량은 normalized.containers(milli/bytes 정수)로 저장 — legacy
		// "500m / 1.0Gi" 포맷은 조립 P1-D의 포맷터 오라클이 소유한다.
		return "normalized:containers"
	// §4.7 network.services
	case "network.services.name":
		return "display"
	case "network.services.namespace":
		return "raw:namespace"
	case "network.services.type":
		return "normalized:type"
	case "network.services.clusterIP":
		return "normalized:clusterIP"
	case "network.services.externalIP":
		return "normalized:externalIP"
	case "network.services.ports":
		return "normalized:ports"
	// §4.7 network.ingresses
	case "network.ingresses.name":
		return "display"
	case "network.ingresses.namespace":
		return "raw:namespace"
	case "network.ingresses.address":
		return "normalized:address"
	case "network.ingresses.tls":
		return "normalized:tls"
	case "network.ingresses.host":
		return "normalized:hosts"
	// §4.8 advancedNetwork — P1-C1 수집 종의 J-P1-3 normalized 키. 섹션 배열의
	// 대표 랜딩 스폿은 gateway hosts·httproute parents다(전필드 대응은 mapping.md
	// §4.8 표가 소유 — 기계 단얫은 at-least-one).
	case "advancedNetwork.gatewayApiGateways":
		return "normalized:hosts"
	case "advancedNetwork.httpRoutes":
		return "normalized:parents"
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
	case "configStorage.storage.namespaceScope":
		return "normalized:namespaceScope"
	case "configStorage.storage.status":
		return "normalized:phase"
	case "configStorage.storage.capacity":
		return "normalized:capacityGB"
	case "configStorage.storage.sourceType":
		return "normalized:sourceType"
	case "configStorage.storage.path":
		return "normalized:sourcePath"
	case "configStorage.storage.nfsServer":
		return "normalized:nfsServer"
	case "configStorage.storage.reclaimPolicy":
		return "normalized:reclaimPolicy"
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
	case strings.HasPrefix(coverageKey, "advancedNetwork.gatewayApiGateways"):
		return "network.gateway", "" // P1-C1 — GatewayAPI Gateway 수집 종
	case strings.HasPrefix(coverageKey, "advancedNetwork.httpRoutes"):
		return "network.http_route", "" // P1-C1 — GatewayAPI HTTPRoute 수집 종
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
