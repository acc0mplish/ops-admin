// k8sassembly_sections.go — 조립 엔트리포인트와 cluster·nodes·namespaces 섹션.
// 계획 J-P1-4: AssembleK8sClusterDetail(db, conn, rows) — db는 ③ Name 조인과
// gateway 이름 읽기만(읽기 전용·쓰기 없음), 나머지는 rows만으로 순수 조립.
package inventory

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
	v1model "ops-admin/backend/model"
)

// AssembleK8sClusterDetail assembles the legacy K8sClusterDetail read model
// from the V2 public projection of one context. conn은 백필(§3.4)이 채운
// connection 행 — cluster 속성(name·endpoint·ConfigJSON env/tags/…
// node_count/monitor_datasource_id)의 원천이다. db는 gateway 이름·③
// monitor_datasource 이름 조인에만 쓴다.
//
// certificates[]는 판정 ④ 결착 전 부재다(G-P1f — 착수 금지).
func AssembleK8sClusterDetail(db *gorm.DB, conn *model.ProviderConnection, rows []ProjectedResource) (v1model.K8sClusterDetail, error) {
	if conn == nil {
		return v1model.K8sClusterDetail{}, fmt.Errorf("inventory: assemble k8s cluster detail: nil connection")
	}

	nodeRows := rowsOfKind(rows, "orchestration.node")
	podRows := rowsOfKind(rows, "orchestration.pod")
	namespaceRows := rowsOfKind(rows, "orchestration.namespace")
	configMapRows := rowsOfKind(rows, "orchestration.configmap")
	secretRows := rowsOfKind(rows, "orchestration.secret")
	serviceRows := rowsOfSubtype(rows, "network.load_balancer", "service")
	ingressRows := rowsOfSubtype(rows, "network.load_balancer", "ingress")
	endpointRows := rowsOfKind(rows, "network.endpoint")
	gatewayRows := rowsOfKind(rows, "network.gateway")
	httpRouteRows := rowsOfKind(rows, "network.http_route")

	pvcRows := make([]ProjectedResource, 0)
	pvRows := make([]ProjectedResource, 0)
	for _, row := range rows {
		switch row.Subtype {
		case "pvc":
			pvcRows = append(pvcRows, row)
		case "persistent_volume":
			pvRows = append(pvRows, row)
		}
	}

	// 집계(①② — overview 6필드)와 cluster 경보 승격은 v1 순서 그대로.
	metrics := CalculateK8sAggregateMetrics(nodeRows, podRows)
	cluster, err := buildClusterView(db, conn, nodeRows, metrics.AlertCount)
	if err != nil {
		return v1model.K8sClusterDetail{}, err
	}

	workloads := BuildWorkloadItems(rows)
	sort.Slice(workloads, func(i, j int) bool {
		if workloads[i].Namespace == workloads[j].Namespace {
			return workloads[i].Name < workloads[j].Name
		}
		return workloads[i].Namespace < workloads[j].Namespace
	})
	endpointCounts := BuildEndpointCounts(endpointRows)

	// ④ 결착 전 — Certificates는 영값(nil)으로 둔다(G-P1f 기계 판정 대상 외).
	return v1model.K8sClusterDetail{
		Cluster: cluster,
		Overview: v1model.K8sOverview{
			HealthScore:  CalculateHealthScore(metrics.AlertCount),
			CPUUsage:     FormatUsagePercent(metrics.TotalReqCPUMilli, metrics.TotalAllocCPUMilli),
			MemoryUsage:  FormatUsagePercent(metrics.TotalReqMemoryBytes, metrics.TotalAllocMemoryBytes),
			PodUsage:     fmt.Sprintf("%d Pods", len(podRows)),
			RequestRate:  fmt.Sprintf("%d Workloads", len(workloads)),
			AlertCount:   metrics.AlertCount,
			Distribution: BuildOverviewDistribution(cluster, nodeRows, configMapRows),
			Certificates: nil,
		},
		Nodes:           BuildNodeItems(nodeRows, podRows),
		Namespaces:      BuildNamespaceItems(namespaceRows, BuildNamespaceCounts(rows)),
		Pods:            BuildPodItemsWithWorkloads(rows),
		Workloads:       workloads,
		Network:         BuildNetworkSection(serviceRows, ingressRows, endpointCounts),
		AdvancedNetwork: BuildAdvancedNetworkSection(gatewayRows, httpRouteRows, serviceRows),
		ConfigStorage:   BuildConfigStorageSection(configMapRows, secretRows, pvcRows, pvRows),
	}, nil
}

// rowsOfKind/rowsOfSubtype — 섹션 필터(kind 문자열은 normalizer_sections의
// sectionKind와 mapping.md §1 어휘).
func rowsOfKind(rows []ProjectedResource, kind string) []ProjectedResource {
	out := make([]ProjectedResource, 0, len(rows))
	for _, row := range rows {
		if row.Kind == kind {
			out = append(out, row)
		}
	}
	return out
}

func rowsOfSubtype(rows []ProjectedResource, kind, subtype string) []ProjectedResource {
	out := make([]ProjectedResource, 0)
	for _, row := range rows {
		if row.Kind == kind && row.Subtype == subtype {
			out = append(out, row)
		}
	}
	return out
}

// workloadRows는 5종 워크로드 행만 골라낸다(replicaset·virtualservice 제외 —
// legacy Workloads 목록도 5종만 실는다).
func workloadRows(rows []ProjectedResource) []ProjectedResource {
	out := make([]ProjectedResource, 0)
	for _, row := range rows {
		if row.Kind != "orchestration.workload" {
			continue
		}
		switch row.Subtype {
		case "deployment", "statefulset", "daemonset", "job", "cronjob":
			out = append(out, row)
		}
	}
	return out
}

// --- cluster 뷰 (연결 행 + ③ 조인) ---

// buildClusterView mirrors legacy toK8sClusterView + 경보 승격(alertCount>0 →
// warning). NodeCount는 ConfigJSON["node_count"](백필 승계)에서 읽고 부재 시
// node 행 수 폴백이다 — v1 표시 계약(TestCharBuildOverviewDistribution
// NodeCount=3 vs 노드 1행)이 저장 속성을 고정하기 때문이다(P1-D 판단 기록).
func buildClusterView(db *gorm.DB, conn *model.ProviderConnection, nodeRows []ProjectedResource, alertCount int) (v1model.K8sClusterView, error) {
	config := conn.ConfigJSON
	if config == nil {
		config = contract.JSONMap{}
	}
	cluster := v1model.K8sClusterView{
		ID:                  conn.ID,
		Name:                conn.Name,
		Status:              conn.Status,
		StatusText:          StatusText(conn.Status),
		APIServer:           conn.Endpoint,
		Version:             conn.Version,
		NodeCount:           nodeCountFrom(config, len(nodeRows)),
		Env:                 jsonString(config["env"]),
		Tags:                jsonStringList(config["tags"]),
		ConnectionMode:      normalizeConnectionModeText(jsonString(config["connection_mode"])),
		GatewayID:           conn.GatewayID,
		MonitorDatasourceID: monitorDatasourceID(config),
		Description:         jsonString(config["description"]),
		LastSyncAt:          nil, // VOLATILE — §15.3 비교 집합 밖(판단 기록 P1-D-5).
		CreatedAt:           conn.CreatedAt,
		UpdatedAt:           conn.UpdatedAt,
	}
	if alertCount > 0 {
		cluster.Status = "warning"
		cluster.StatusText = StatusText("warning")
	}

	// gateway 이름(조립 조인)과 ③ monitor_datasource 이름(읽기 전용 조인).
	if conn.GatewayID != nil && *conn.GatewayID > 0 {
		var gateway struct{ Name string }
		if err := db.Table("asset_gateway").Select("name").Where("id = ?", *conn.GatewayID).Scan(&gateway).Error; err != nil {
			return v1model.K8sClusterView{}, fmt.Errorf("inventory: assemble cluster view: gateway join: %w", err)
		}
		cluster.GatewayName = gateway.Name
	}
	if cluster.MonitorDatasourceID != nil && *cluster.MonitorDatasourceID > 0 {
		var datasource struct{ Name string }
		if err := db.Table("monitor_datasource").Select("name").Where("id = ?", *cluster.MonitorDatasourceID).Scan(&datasource).Error; err != nil {
			return v1model.K8sClusterView{}, fmt.Errorf("inventory: assemble cluster view: monitor_datasource join: %w", err)
		}
		cluster.MonitorDatasourceName = datasource.Name
	}
	return cluster, nil
}

func nodeCountFrom(config map[string]any, fallback int) int {
	if n, ok := jsonInt(config["node_count"]); ok && n > 0 {
		return int(n)
	}
	return fallback
}

// monitorDatasourceID — 판정 ③ (혼합): Id는 등록 경로가 ConfigJSON에 흡수하고
// 조립이 그 값을 읽는다(Name은 위 조인).
func monitorDatasourceID(config map[string]any) *uint {
	n, ok := jsonInt(config["monitor_datasource_id"])
	if !ok || n <= 0 {
		return nil
	}
	id := uint(n)
	return &id
}

func jsonStringList(v any) []string {
	return jsonStrList(v)
}

// normalizeConnectionModeText mirrors legacy normalizeConnectionMode.
func normalizeConnectionModeText(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "gateway") {
		return "gateway"
	}
	return "direct"
}

// --- nodes 섹션 (집계: pods · 조립: 표시 전반) ---

// BuildNodeItems mirrors legacy buildNodeItems — 파드 카운트는 nodeName 집계,
// 분모는 capacityPods(TestCharBuildNodeItems "2/110").
func BuildNodeItems(nodeRows []ProjectedResource, podRows []ProjectedResource) []v1model.K8sNodeItem {
	podCountByNode := make(map[string]int)
	for _, pod := range podRows {
		if name := jsonString(pod.Normalized["nodeName"]); name != "" {
			podCountByNode[name]++
		}
	}

	items := make([]v1model.K8sNodeItem, 0, len(nodeRows))
	for _, node := range nodeRows {
		item := v1model.K8sNodeItem{
			Name:       node.DisplayName,
			Role:       JoinNodeRoles(jsonStrList(node.Normalized["roles"])),
			Status:     NodeReadyStatus(jsonString(node.Normalized["readyCondition"])),
			Version:    FallbackText(jsonString(node.Normalized["kubeletVersion"])),
			InternalIP: FallbackText(jsonString(node.Normalized["internalIP"])),
			OS:         FallbackText(jsonString(node.Normalized["osImage"])),
			Pods:       fmt.Sprintf("%d/%s", podCountByNode[node.DisplayName], capacityPodsText(node)),
		}
		if cores, ok := jsonFloat(node.Normalized["allocatableCoresGB"]); ok {
			item.CPU = FormatCoresText(cores)
		} else {
			item.CPU = "-"
		}
		if gb, ok := jsonFloat(node.Normalized["allocatableMemoryGB"]); ok {
			item.Memory = FormatMemoryMBFromBytes(allocatableBytes(gb))
		} else {
			item.Memory = "-"
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func capacityPodsText(node ProjectedResource) string {
	if v, ok := jsonInt(node.Normalized["capacityPods"]); ok {
		return strconv.FormatInt(v, 10)
	}
	return "-"
}

// --- namespaces 섹션 (집계 3필드) ---

// NamespaceCounts mirrors legacy buildNamespaceCounts의 익명 삼중 구조.
type NamespaceCounts struct {
	Pods      int
	Services  int
	Workloads int
}

// BuildNamespaceCounts mirrors legacy buildNamespaceCounts — pods·services는
// 행의 네임스페이스, workloads는 5종 워크로드 행(replicaset 제외 — legacy도
// 5종만 센다).
func BuildNamespaceCounts(rows []ProjectedResource) map[string]NamespaceCounts {
	counts := make(map[string]NamespaceCounts)
	bump := func(namespace string, which func(*NamespaceCounts)) {
		if namespace == "" {
			return
		}
		item := counts[namespace]
		which(&item)
		counts[namespace] = item
	}
	for _, row := range rows {
		switch row.Kind {
		case "orchestration.pod":
			bump(rawNamespace(row), func(c *NamespaceCounts) { c.Pods++ })
		case "network.load_balancer":
			if row.Subtype != "service" {
				continue
			}
			bump(rawNamespace(row), func(c *NamespaceCounts) { c.Services++ })
		case "orchestration.workload":
			// legacy가 세는 5종만(replicaset 제외 — buildNamespaceCounts는
			// Deployments·StatefulSet·DaemonSets·Jobs·CronJobs만 순회).
			switch row.Subtype {
			case "deployment", "statefulset", "daemonset", "job", "cronjob":
				bump(rawNamespace(row), func(c *NamespaceCounts) { c.Workloads++ })
			}
		}
	}
	return counts
}

// BuildNamespaceItems mirrors legacy buildNamespaceItems — 이름 정렬, 상태
// 폴백 "-", createdAt "2006-01-02 15:04".
func BuildNamespaceItems(namespaceRows []ProjectedResource, counts map[string]NamespaceCounts) []v1model.K8sNamespaceItem {
	items := make([]v1model.K8sNamespaceItem, 0, len(namespaceRows))
	for _, row := range namespaceRows {
		// namespace는 클러스터 스코프 종이라 Raw에 namespace가 없다 — 이름
		// 키로 카운트를 대조한다(legacy namespace.Metadata.Name 동일).
		stat := counts[row.DisplayName]
		items = append(items, v1model.K8sNamespaceItem{
			Name:      row.DisplayName,
			Status:    FallbackText(jsonString(row.Normalized["phase"])),
			Pods:      stat.Pods,
			Services:  stat.Services,
			Workloads: stat.Workloads,
			CreatedAt: FormatTimestamp(rawCreationTimestamp(row)),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}
