// k8s_create_face.go — k8s.resource.create(I10 §3.2.1)의 매니페스트 신원
// 매핑 표. apiVersion group × kind를 (§8.5 resource kind, namespaced, 컬렉션
// 경로 성분)으로 대응시키는 단일 원천이다 — def의 ResourceKinds는 공란
// (커넥션-스코프 부호화 — 등록 검증 no-op·자원 목록 자동 누출 방지)이고
// 실행기의 컬렉션 경로 후보도 본 표 소속 성분(Plural·APIVersions·
// Namespaced)에서 조립한다(이중 열거 금지 — 파생 계약 §3.2.1).
//
// v1 패리티 계약: 본 표는 v1 create face의 resourceType 15종 전수(k8s_path.go
// buildK8sCreateResourcePaths census — namespace·pod·service·ingress·configmap·
// secret·pvc·pv·workload 하위 5형·istio 4종·gatewayapi·httproute)를 덮는다.
// istio Gateway와 Gateway API Gateway는 apiVersion group으로 구분된다(동일
// kind 문자열·상이 group — 그룹 불일치는 사전 미매치, RI-4).
package contract

// K8sCreateFaceEntry — 매핑 표 1행.
type K8sCreateFaceEntry struct {
	APIGroup     string   // apiVersion group — ""는 core(apiVersion "v1")
	Kind         string   // 매니페스트 kind(예: "Deployment")
	ResourceKind string   // §8.5 resource kind(예: "orchestration.workload")
	Plural       string   // 컬렉션 경로 성분(예: "deployments") — 실행기 경로 조립 원료
	Namespaced   bool     // true면 manifest metadata.namespace 필수(§3.2.1 namespace 규칙)
	APIVersions  []string // 컬렉션 경로 apiVersion 후보(선호 순) — istio·gateway API는 2후보
}

// apiV1Only — 단일 버전 group(core·apps·batch·networking.k8s.io)의 후보.
var apiV1Only = []string{"v1"}

// apiV1Preferred — istio·gateway API의 선호 순 2후보(v1→v1beta1 — v1 face
// buildIstioResourcePathsWithPreferred·buildGatewayAPIResourcePathsWithPreferred
// 와 동일 순서).
var apiV1Preferred = []string{"v1", "v1beta1"}

// k8sCreateFaceTable — 서빙 면 전수(20행). v1 face 19면(15 resourceType) +
// apps/ReplicaSet(§3.2.1 apps 행 4형 — v1 face 밖 계획 표 성분).
var k8sCreateFaceTable = []K8sCreateFaceEntry{
	{APIGroup: "", Kind: "Namespace", ResourceKind: "orchestration.namespace", Plural: "namespaces", Namespaced: false, APIVersions: apiV1Only},
	{APIGroup: "", Kind: "PersistentVolume", ResourceKind: "storage.volume", Plural: "persistentvolumes", Namespaced: false, APIVersions: apiV1Only},
	{APIGroup: "", Kind: "Pod", ResourceKind: "orchestration.pod", Plural: "pods", Namespaced: true, APIVersions: apiV1Only},
	{APIGroup: "", Kind: "Service", ResourceKind: "network.load_balancer", Plural: "services", Namespaced: true, APIVersions: apiV1Only},
	{APIGroup: "", Kind: "ConfigMap", ResourceKind: "orchestration.configmap", Plural: "configmaps", Namespaced: true, APIVersions: apiV1Only},
	{APIGroup: "", Kind: "Secret", ResourceKind: "orchestration.secret", Plural: "secrets", Namespaced: true, APIVersions: apiV1Only},
	{APIGroup: "", Kind: "PersistentVolumeClaim", ResourceKind: "storage.volume", Plural: "persistentvolumeclaims", Namespaced: true, APIVersions: apiV1Only},
	{APIGroup: "networking.k8s.io", Kind: "Ingress", ResourceKind: "network.load_balancer", Plural: "ingresses", Namespaced: true, APIVersions: apiV1Only},
	{APIGroup: "apps", Kind: "Deployment", ResourceKind: "orchestration.workload", Plural: "deployments", Namespaced: true, APIVersions: apiV1Only},
	{APIGroup: "apps", Kind: "StatefulSet", ResourceKind: "orchestration.workload", Plural: "statefulsets", Namespaced: true, APIVersions: apiV1Only},
	{APIGroup: "apps", Kind: "DaemonSet", ResourceKind: "orchestration.workload", Plural: "daemonsets", Namespaced: true, APIVersions: apiV1Only},
	{APIGroup: "apps", Kind: "ReplicaSet", ResourceKind: "orchestration.workload", Plural: "replicasets", Namespaced: true, APIVersions: apiV1Only},
	{APIGroup: "batch", Kind: "Job", ResourceKind: "orchestration.workload", Plural: "jobs", Namespaced: true, APIVersions: apiV1Only},
	{APIGroup: "batch", Kind: "CronJob", ResourceKind: "orchestration.workload", Plural: "cronjobs", Namespaced: true, APIVersions: apiV1Only},
	{APIGroup: "networking.istio.io", Kind: "Gateway", ResourceKind: "network.gateway", Plural: "gateways", Namespaced: true, APIVersions: apiV1Preferred},
	{APIGroup: "networking.istio.io", Kind: "VirtualService", ResourceKind: "network.virtual_service", Plural: "virtualservices", Namespaced: true, APIVersions: apiV1Preferred},
	{APIGroup: "networking.istio.io", Kind: "DestinationRule", ResourceKind: "network.destination_rule", Plural: "destinationrules", Namespaced: true, APIVersions: apiV1Preferred},
	{APIGroup: "networking.istio.io", Kind: "ServiceEntry", ResourceKind: "network.service_entry", Plural: "serviceentries", Namespaced: true, APIVersions: apiV1Preferred},
	{APIGroup: "gateway.networking.k8s.io", Kind: "Gateway", ResourceKind: "network.gateway", Plural: "gateways", Namespaced: true, APIVersions: apiV1Preferred},
	{APIGroup: "gateway.networking.k8s.io", Kind: "HTTPRoute", ResourceKind: "network.http_route", Plural: "httproutes", Namespaced: true, APIVersions: apiV1Preferred},
}

// K8sAPIGroupOf — apiVersion 문자열("v1"·"apps/v1"·"networking.istio.io/v1beta1")
// 에서 group을 추출한다. core(group 없는 형태)는 "" — 매니페스트 apiVersion에서
// 표 키(APIGroup)를 얻는 유일 경로다.
func K8sAPIGroupOf(apiVersion string) string {
	for i := 0; i < len(apiVersion); i++ {
		if apiVersion[i] == '/' {
			return apiVersion[:i]
		}
	}
	return ""
}

// K8sCreateFace — apiVersion group × kind의 서빙 면 매핑을 조회한다. 소속이
// 아니면 false(그룹 불일치·알 수 없는 kind 모두 미매치 — 교차-kind 사전 거부).
func K8sCreateFace(apiGroup, kind string) (K8sCreateFaceEntry, bool) {
	for _, e := range k8sCreateFaceTable {
		if e.APIGroup == apiGroup && e.Kind == kind {
			return e, true
		}
	}
	return K8sCreateFaceEntry{}, false
}
