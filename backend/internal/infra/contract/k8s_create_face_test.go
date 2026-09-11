// k8s_create_face_test.go — 매니페스트 신원 매핑 표(k8s_create_face.go) 잠금
// (I10 §3.2.1·CIm). 단얫 3축: ① v1 create face resourceType 15종 전수 대응
// (k8s_path.go buildK8sCreateResourcePaths census와 1:1 — workload 하위 5형·
// istio 4·gatewayapi·httproute 포함) ② istio Gateway와 Gateway API Gateway의
// 그룹 구분 이중 대응(동일 kind 문자열·상이 apiVersion group — RI-4) ③
// namespaced 플래그(Namespace·PersistentVolume만 클러스터 스코프).
package contract

import (
	"slices"
	"strings"
	"testing"
)

// v1FaceRows — v1 create face의 resourceType 15종(workload 1종이 하위 5형으로
// 전개되어 19 group×kind 면)과 그 서빙 면 대응. plural은 v1 컬렉션 경로
// 성분(k8s_path.go:90-176)과 동일 어휘다 — 경로 조립 파생 계약(§3.2.1: 실행기
// 컬렉션 경로는 본 표 소속 성분에서 조립한다)의 원료 정합.
var v1FaceRows = []struct {
	v1Type string
	group  string
	kind   string
	plural string
}{
	{"namespace", "", "Namespace", "namespaces"},
	{"pod", "", "Pod", "pods"},
	{"service", "", "Service", "services"},
	{"ingress", "networking.k8s.io", "Ingress", "ingresses"},
	{"configmap", "", "ConfigMap", "configmaps"},
	{"secret", "", "Secret", "secrets"},
	{"pvc", "", "PersistentVolumeClaim", "persistentvolumeclaims"},
	{"pv", "", "PersistentVolume", "persistentvolumes"},
	{"workload:deployment", "apps", "Deployment", "deployments"},
	{"workload:statefulset", "apps", "StatefulSet", "statefulsets"},
	{"workload:daemonset", "apps", "DaemonSet", "daemonsets"},
	{"workload:job", "batch", "Job", "jobs"},
	{"workload:cronjob", "batch", "CronJob", "cronjobs"},
	{"gateway", "networking.istio.io", "Gateway", "gateways"},
	{"virtualservice", "networking.istio.io", "VirtualService", "virtualservices"},
	{"destinationrule", "networking.istio.io", "DestinationRule", "destinationrules"},
	{"serviceentry", "networking.istio.io", "ServiceEntry", "serviceentries"},
	{"gatewayapi", "gateway.networking.k8s.io", "Gateway", "gateways"},
	{"httproute", "gateway.networking.k8s.io", "HTTPRoute", "httproutes"},
}

func TestK8sCreateFace(t *testing.T) {
	// --- v1 face 전수 대응: 19 group×kind 조회 전부 성공 + plural·어휘 소속.
	// 신설 어휘 2종(destinationrule→network.destination_rule·serviceentry→
	// network.service_entry)은 I10ResourceKindExtensions 소속까지 단얫. ---
	for _, row := range v1FaceRows {
		entry, ok := K8sCreateFace(row.group, row.kind)
		if !ok {
			t.Errorf("v1 face %q (%s/%s): not in the create face table", row.v1Type, row.group, row.kind)
			continue
		}
		if entry.Plural != row.plural {
			t.Errorf("v1 face %q: Plural = %q, want %q (v1 컬렉션 경로 어휘)", row.v1Type, entry.Plural, row.plural)
		}
		if !IsKnownResourceKind(entry.ResourceKind) {
			t.Errorf("v1 face %q: ResourceKind %q not in the §8.5 vocabulary", row.v1Type, entry.ResourceKind)
		}
	}
	if e, _ := K8sCreateFace("networking.istio.io", "DestinationRule"); e.ResourceKind != "network.destination_rule" {
		t.Errorf("DestinationRule ResourceKind = %q, want network.destination_rule (I10 신설)", e.ResourceKind)
	}
	if e, _ := K8sCreateFace("networking.istio.io", "ServiceEntry"); e.ResourceKind != "network.service_entry" {
		t.Errorf("ServiceEntry ResourceKind = %q, want network.service_entry (I10 신설)", e.ResourceKind)
	}
	if !IsKnownResourceKind("network.destination_rule") || !IsKnownResourceKind("network.service_entry") {
		t.Errorf("I10 extension kinds not recognized by IsKnownResourceKind")
	}

	// --- ReplicaSet — v1 face 밖 계획 표 성분(§3.2.1 apps 행이 4형을 못 박는다:
	// v1 workload 5형에 없는 orchestration.workload 대응의 4번째 apps kind). ---
	if e, ok := K8sCreateFace("apps", "ReplicaSet"); !ok || e.ResourceKind != "orchestration.workload" || !e.Namespaced {
		t.Errorf("apps/ReplicaSet = %+v (ok=%v), want orchestration.workload namespaced row", e, ok)
	}

	// --- 그룹 구분 Gateway 이중(RI-4): 동일 kind "Gateway"가 istio·gateway API
	// 양 group에 별행으로 존재하고 둘 다 network.gateway로 대응한다. ---
	istioGW, okIstio := K8sCreateFace("networking.istio.io", "Gateway")
	gwapiGW, okGwapi := K8sCreateFace("gateway.networking.k8s.io", "Gateway")
	if !okIstio || !okGwapi {
		t.Fatalf("dual Gateway: istio=%v gwapi=%v — both groups must carry a Gateway row", okIstio, okGwapi)
	}
	if istioGW.ResourceKind != "network.gateway" || gwapiGW.ResourceKind != "network.gateway" {
		t.Errorf("dual Gateway kinds: istio=%q gwapi=%q, want both network.gateway", istioGW.ResourceKind, gwapiGW.ResourceKind)
	}
	gatewayRows := 0
	for _, e := range k8sCreateFaceTable {
		if e.Kind == "Gateway" {
			gatewayRows++
		}
	}
	if gatewayRows != 2 {
		t.Errorf("table holds %d Gateway rows, want exactly 2 (istio + gateway API — 그룹 구분)", gatewayRows)
	}

	// --- namespaced 플래그: 클러스터 스코프는 Namespace·PersistentVolume
	// 뿐이고 나머지 전종이 namespaced다 (§3.2.1 namespace 규칙의 전제). ---
	clusterScoped := map[string]bool{"Namespace": true, "PersistentVolume": true}
	for _, e := range k8sCreateFaceTable {
		if want := !clusterScoped[e.Kind]; e.Namespaced != want {
			t.Errorf("%s/%s: Namespaced = %v, want %v", e.APIGroup, e.Kind, e.Namespaced, want)
		}
	}

	// --- 표 무결성: (group, kind) 키 유일 · ResourceKind 전종 어휘 소속 ·
	// plural 비공백 소문자 · apiVersion 후보(istio·gateway API만 v1→v1beta1
	// 2후보, 나머지 group은 v1 단일). ---
	seen := make(map[string]bool, len(k8sCreateFaceTable))
	dualVersion := map[string]bool{"networking.istio.io": true, "gateway.networking.k8s.io": true}
	for _, e := range k8sCreateFaceTable {
		key := e.APIGroup + "|" + e.Kind
		if seen[key] {
			t.Errorf("duplicate (group, kind) key %q", key)
		}
		seen[key] = true
		if !IsKnownResourceKind(e.ResourceKind) {
			t.Errorf("%s/%s: ResourceKind %q not in the §8.5 vocabulary", e.APIGroup, e.Kind, e.ResourceKind)
		}
		if e.Plural == "" || e.Plural != strings.ToLower(e.Plural) {
			t.Errorf("%s/%s: Plural %q must be non-empty lowercase", e.APIGroup, e.Kind, e.Plural)
		}
		wantVersions := []string{"v1"}
		if dualVersion[e.APIGroup] {
			wantVersions = []string{"v1", "v1beta1"}
		}
		if !slices.Equal(e.APIVersions, wantVersions) {
			t.Errorf("%s/%s: APIVersions = %v, want %v", e.APIGroup, e.Kind, e.APIVersions, wantVersions)
		}
	}

	// --- 교차-kind 가드(RI-4 사전 차단): 소속 group이 아니면 거짓 — core
	// Deployment·istio 밖 HTTPRoute·gateway API 밖 VirtualService 전부 미매치. ---
	for _, miss := range [][2]string{
		{"", "Gateway"},
		{"", "Deployment"},
		{"", "DestinationRule"},
		{"apps", "Gateway"},
		{"networking.istio.io", "HTTPRoute"},
		{"gateway.networking.k8s.io", "VirtualService"},
		{"networking.k8s.io", "Namespace"},
		{"", "Ingress"},
	} {
		if _, ok := K8sCreateFace(miss[0], miss[1]); ok {
			t.Errorf("K8sCreateFace(%q, %q) = true, want false (그룹 불일치 사전 거부)", miss[0], miss[1])
		}
	}
}

// TestK8sAPIGroupOf — apiVersion 문자열의 group 추출. core("v1" 등 group 없는
// 형태)는 "" — 매니페스트 apiVersion에서 표 키(APIGroup)를 얻는 유일 경로.
func TestK8sAPIGroupOf(t *testing.T) {
	cases := map[string]string{
		"v1":                           "",
		"v1beta1":                      "",
		"":                             "",
		"apps/v1":                      "apps",
		"batch/v1":                     "batch",
		"networking.k8s.io/v1":         "networking.k8s.io",
		"networking.istio.io/v1":       "networking.istio.io",
		"networking.istio.io/v1beta1":  "networking.istio.io",
		"gateway.networking.k8s.io/v1": "gateway.networking.k8s.io",
	}
	for apiVersion, want := range cases {
		if got := K8sAPIGroupOf(apiVersion); got != want {
			t.Errorf("K8sAPIGroupOf(%q) = %q, want %q", apiVersion, got, want)
		}
	}
}
