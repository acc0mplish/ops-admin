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
	"os"
	"reflect"
	"strings"
	"testing"

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
