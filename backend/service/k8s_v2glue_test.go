// k8s_v2glue_test.go — P1-E Z 오라클 승계 글루 (계획 J-P1-5·P1-E1/E2).
// k8s_chartest_helper_test.go에서 분할했다(§9-7 800 하드캡 — 글루 추가로 805행이
// 되어 같은 작업 내 분할). 소관은 legacy kube 구조체 → V2 관측·조립 승계 호출
// 뿐이다. 같은 패키지 _test 파일이라 심볼 공유는 유지되고, service 프로덕션
// 심볼 추가는 0이다. PC-V의 승계 테스트 파일 경계 안에 있다.
package service

import (
	"encoding/json"

	"ops-admin/backend/internal/infra/adapter/kubernetes"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/model"
)

// --- P1-E Z 오라클 승계 글루 (계획 J-P1-5) ---------------------------------
//
// legacy kube 구조체 → json.Marshal → kubernetes.NormalizeSection(프로덕션
// 디코드) → JSON 왕복(DB 관측 형상) → inventory 조립. 각 v2X는 대응 legacy
// 1행의 V2 승계 호출이다. 디코드 실패는 픽스처 버그라 패닉한다.
// v2 미대응 5종(buildJobDetail·buildCronJobDetail·buildHTTPRouteTrafficItems·
// flattenGatewayHosts·flattenGatewayPorts)은 사유 처분 — 각 마커 행 주석 참조.

// v2OracleCtxID는 글루 고정 ctxID다 — URN 성분뿐이라 동치 판정과 무관하다.
const v2OracleCtxID = uint(1)

// v2Row는 legacy kube 구조체 1건을 V2 관측 행으로 승계한다. 마셜 → 프로덕션
// 디코드 → JSON 왕복의 2차 마셜은 DB에서 관측을 읽는 ProjectResources와 같은
// 형상 정규화([]ownerReference→[]any·map[string]string→map[string]any)를
// 재현한다 — 오라클이 프로덕션 소비 형상을 검토하게 하기 위함이다.
func v2Row(section string, item any) inventory.ProjectedResource {
	body, err := json.Marshal(item)
	if err != nil {
		panic("v2glue marshal: " + err.Error())
	}
	res, err := kubernetes.NormalizeSection(v2OracleCtxID, section, body)
	if err != nil {
		panic("v2glue normalize " + section + ": " + err.Error())
	}
	row := inventory.ProjectedResource{
		UID: res.ExternalID, Kind: res.Kind, Subtype: res.Subtype,
		ExternalID: res.ExternalID, DisplayName: res.DisplayName,
		Normalized: res.Normalized, Raw: res.Raw,
	}
	blob, err := json.Marshal(row)
	if err != nil {
		panic("v2glue row marshal: " + err.Error())
	}
	if err := json.Unmarshal(blob, &row); err != nil {
		panic("v2glue row unmarshal: " + err.Error())
	}
	return row
}

// v2Rows는 슬라이스형 v2Row다.
func v2Rows[S any](section string, items []S) []inventory.ProjectedResource {
	rows := make([]inventory.ProjectedResource, 0, len(items))
	for i := range items {
		rows = append(rows, v2Row(section, items[i]))
	}
	return rows
}

// v2Str·v2StrList는 normalized 키 리더다(jsonString·jsonStrList 동치).
func v2Str(row inventory.ProjectedResource, key string) string {
	s, _ := row.Normalized[key].(string)
	return s
}

func v2StrList(row inventory.ProjectedResource, key string) []string {
	switch list := row.Normalized[key].(type) {
	case []string:
		return list
	case []any:
		out := make([]string, 0, len(list))
		for _, item := range list {
			s, _ := item.(string)
			out = append(out, s)
		}
		return out
	}
	return nil
}

// v2Containers는 normalized.containers([]any of map)를 조립 시그니처
// ([]contract.JSONMap)로 변환한다.
func v2Containers(row inventory.ProjectedResource) []contract.JSONMap {
	items, _ := row.Normalized["containers"].([]any)
	out := make([]contract.JSONMap, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// --- 파이프라인형 승계 호출 (테스트 본문 1행 교체 대상) ---------------------

// v2NodeReadyStatus — nodeReadyStatus 승계: 조건→readyCondition 어휘 변환은
// 프로덕션 노드 디코드 소관이고 표시 어휘 결합은 NodeReadyStatus다.
func v2NodeReadyStatus(node kubeNode) string {
	return inventory.NodeReadyStatus(v2Str(v2Row("nodes", node), "readyCondition"))
}

// v2JoinNodeRoles — joinNodeRoles 승계: 라벨→roles[] 수집은 프로덕션 노드
// 디코드(빈 접미→worker·정렬)가 담당하고 결합만 JoinNodeRoles다.
func v2JoinNodeRoles(labels map[string]string) string {
	node := kubeNode{}
	node.Metadata.Labels = labels
	return inventory.JoinNodeRoles(v2StrList(v2Row("nodes", node), "roles"))
}

// v2FormatWorkloadResourceSummary — formatWorkloadResourceSummary 승계:
// 원시량(milli·bytes) 변환은 수집측 containerQuantities가, 합산·포맷은
// 조립 FormatWorkloadResourceSummary가 담당한다.
func v2FormatWorkloadResourceSummary(containers []kubeContainer, requests bool) string {
	workload := struct {
		Spec struct {
			Template struct {
				Spec struct {
					Containers []kubeContainer `json:"containers"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
	}{}
	workload.Spec.Template.Spec.Containers = containers
	return inventory.FormatWorkloadResourceSummary(
		v2Containers(v2Row("deployments", workload)), requests)
}

// v2BuildOverviewDistribution — buildOverviewDistribution 승계.
func v2BuildOverviewDistribution(cluster model.K8sClusterView, nodes []kubeNode, cms []kubeConfigMap) []model.K8sKVTextItem {
	return inventory.BuildOverviewDistribution(cluster, v2Rows("nodes", nodes), v2Rows("configmaps", cms))
}

// v2ResolveK8sNetworkCIDRs — resolveK8sNetworkCIDRs 승계.
func v2ResolveK8sNetworkCIDRs(nodes []kubeNode, cms []kubeConfigMap) (string, string) {
	return inventory.ResolveK8sNetworkCIDRs(v2Rows("nodes", nodes), v2Rows("configmaps", cms))
}

// v2CalculateK8sAggregateMetrics — calculateK8sAggregateMetrics 승계.
func v2CalculateK8sAggregateMetrics(nodes []kubeNode, pods []kubePod) k8sAggregateMetrics {
	agg := inventory.CalculateK8sAggregateMetrics(v2Rows("nodes", nodes), v2Rows("pods", pods))
	return k8sAggregateMetrics{
		TotalAllocCPUMilli:    agg.TotalAllocCPUMilli,
		TotalAllocMemoryBytes: agg.TotalAllocMemoryBytes,
		TotalReqCPUMilli:      agg.TotalReqCPUMilli,
		TotalReqMemoryBytes:   agg.TotalReqMemoryBytes,
		AlertCount:            agg.AlertCount,
	}
}

// v2BuildNamespaceCounts — buildNamespaceCounts 승계. 계수 종(pod·service·
// workload 5 subtype)만 행으로 싣는다.
func v2BuildNamespaceCounts(data k8sFetchedData) map[string]struct {
	pods      int
	services  int
	workloads int
} {
	rows := make([]inventory.ProjectedResource, 0)
	rows = append(rows, v2Rows("pods", data.Pods)...)
	rows = append(rows, v2Rows("services", data.Services)...)
	rows = append(rows, v2Rows("deployments", data.Deployments)...)
	rows = append(rows, v2Rows("statefulsets", data.StatefulSet)...)
	rows = append(rows, v2Rows("daemonsets", data.DaemonSets)...)
	rows = append(rows, v2Rows("jobs", data.Jobs)...)
	rows = append(rows, v2Rows("cronjobs", data.CronJobs)...)
	counted := inventory.BuildNamespaceCounts(rows)
	counts := make(map[string]struct {
		pods      int
		services  int
		workloads int
	}, len(counted))
	for namespace, stat := range counted {
		counts[namespace] = struct {
			pods      int
			services  int
			workloads int
		}{pods: stat.Pods, services: stat.Services, workloads: stat.Workloads}
	}
	return counts
}

// v2BuildPodItemsWithWorkloads — buildPodItemsWithWorkloads 승계.
func v2BuildPodItemsWithWorkloads(data k8sFetchedData) []model.K8sPodItem {
	rows := make([]inventory.ProjectedResource, 0)
	rows = append(rows, v2Rows("pods", data.Pods)...)
	rows = append(rows, v2Rows("deployments", data.Deployments)...)
	rows = append(rows, v2Rows("statefulsets", data.StatefulSet)...)
	rows = append(rows, v2Rows("daemonsets", data.DaemonSets)...)
	rows = append(rows, v2Rows("replicasets", data.ReplicaSets)...)
	rows = append(rows, v2Rows("jobs", data.Jobs)...)
	rows = append(rows, v2Rows("cronjobs", data.CronJobs)...)
	return inventory.BuildPodItemsWithWorkloads(rows)
}

// v2BuildEndpointCounts — buildEndpointCounts 승계.
func v2BuildEndpointCounts(endpoints []kubeEndpoints) map[string]int {
	return inventory.BuildEndpointCounts(v2Rows("endpoints", endpoints))
}

// v2ServiceExternalIP — serviceExternalIP 승계. V2는 관측 부재 시 키를
// 생략하므로 legacy "<none>" 센티널은 글루 경계(조립 포맷)가 채운다.
func v2ServiceExternalIP(service kubeService) string {
	if ip := v2Str(v2Row("services", service), "externalIP"); ip != "" {
		return ip
	}
	return "<none>"
}

// v2PersistentVolumeSource — persistentVolumeSource 승계. legacy 3중항은
// sourceType·sourcePath·nfsServer 키의 "-" 폴백으로 닫는다.
func v2PersistentVolumeSource(pv kubePersistentVolume) (string, string, string) {
	row := v2Row("persistentvolumes", pv)
	return inventory.FallbackText(v2Str(row, "sourceType")),
		inventory.FallbackText(v2Str(row, "sourcePath")),
		inventory.FallbackText(v2Str(row, "nfsServer"))
}

// v2StorageNamespaceScope — storageNamespaceScope 승계. 어노테이션 단독 입력을
// PV 디코드로 통과시킨다(namespaceScope 키는 디코드가 기본값 "Cluster-scoped"
// 까지 채운다).
func v2StorageNamespaceScope(annotations map[string]string) string {
	pv := struct {
		Metadata struct {
			Name        string            `json:"name"`
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
	}{}
	pv.Metadata.Name = "scope-probe"
	pv.Metadata.Annotations = annotations
	return v2Str(v2Row("persistentvolumes", pv), "namespaceScope")
}

// v2CollectGatewayAPIHosts·Ports·Addresses — collectGatewayAPI* 승계:
// 수집 의미론은 gateway 디코드(hosts·ports·addresses 키)가 소유다.
func v2CollectGatewayAPIHosts(gateway kubeGatewayAPI) []string {
	return v2StrList(v2Row("gateways", gateway), "hosts")
}

func v2CollectGatewayAPIPorts(gateway kubeGatewayAPI) []string {
	return v2StrList(v2Row("gateways", gateway), "ports")
}

func v2CollectGatewayAPIAddresses(gateway kubeGatewayAPI) []string {
	return v2StrList(v2Row("gateways", gateway), "addresses")
}

// v2ResolveGatewayAPIAddress — resolveGatewayAPIAddress 승계: 행 1건 조립의
// Address 필드(resolveGatewayAddress — status 주소 → 서비스 폴백 → "-")로
// 통과한다.
func v2ResolveGatewayAPIAddress(item kubeGatewayAPI, services []kubeService) string {
	section := inventory.BuildAdvancedNetworkSection(
		[]inventory.ProjectedResource{v2Row("gateways", item)},
		nil, v2Rows("services", services))
	return section.GatewayAPIGateways[0].Address
}

// v2CollectHTTPRouteParents·Targets — collectHTTPRoute* 승계: parents·targets
// 키는 httproute 디코드가 소유다.
func v2CollectHTTPRouteParents(route kubeHTTPRoute) []string {
	return v2StrList(v2Row("httproutes", route), "parents")
}

func v2CollectHTTPRouteTargets(route kubeHTTPRoute) []string {
	return v2StrList(v2Row("httproutes", route), "targets")
}
