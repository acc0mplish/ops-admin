// k8sassembly.go — V2 k8s 조립 라이브러리(계획 P 계획 r3 J-P1-4·P1-D).
//
// 저장된 관측(ProjectedResource)에서 legacy `model.K8sClusterDetail` 43필드를
// 재구성한다. 소비는 P1-E(Z 오라클 승계) 이후다 — 본 파일은 스왑 대응 구현이며
// 섹션별 조립을 순수 함수로 export한다(쓰기 없음·DB 무지 — R-P8: 순수 함수는
// rows만으로 돈다).
//
// 산식·포맷의 권위는 v1 오라클이다(승계 원장 — 계획 J-P1-5):
//
//	calculateK8sAggregateMetrics  service/k8s_build_net.go:503
//	calculateHealthScore          service/k8s_path.go:335
//	formatUsagePercent            service/k8s_path.go:346
//	buildOverviewDistribution     service/k8s_overview.go:16
//	resolveK8sNetworkCIDRs        service/k8s_overview.go:31
//	formatWorkloadResourceSummary service/k8s_build_pod.go:459
//	formatCPUMilli·formatMemoryBytes service/k8s_build_pod.go:489·496
//	humanizeAge·formatTimestamp   service/k8s_path.go:445·467
//	joinNodeRoles·nodeReadyStatus service/k8s_path.go:375·355
//	cronJobReadyText              service/k8s_path.go:493
//	formatServiceListPort         service/k8s_build_pod.go:271
//	joinAndLimit·uniqueNonEmptyStrings service/k8s_build_net.go:66·75
//	k8sStatusText                 service/k8s_path.go:313
package inventory

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/internal/infra/contract"
	v1model "ops-admin/backend/model"
)

// --- 관측 값 리더 — DB JSON 역직렬화(float64·[]any·map[string]any)와 테스트
// 편의 Go 네이티브(int·[]string·map[string]string)를 모두 흡수한다. ---

// jsonFloat은 compare.go의 패키지 공용 구현을 쓴다(float64·int·int64·json.Number).

func jsonInt(v any) (int64, bool) {
	f, ok := jsonFloat(v)
	if !ok {
		return 0, false
	}
	return int64(math.Round(f)), true
}

func jsonString(v any) string {
	s, _ := v.(string)
	return s
}

func jsonBool(v any) bool {
	b, _ := v.(bool)
	return b
}

// jsonStrList absorbs []string (in-memory) and []any of strings (DB JSON).
func jsonStrList(v any) []string {
	switch list := v.(type) {
	case []string:
		return list
	case []any:
		out := make([]string, 0, len(list))
		for _, item := range list {
			out = append(out, jsonString(item))
		}
		return out
	}
	return nil
}

// jsonObjectList absorbs []contract.JSONMap (in-memory) and []any of maps
// (DB JSON) — the shape of normalized.containers·ports.
func jsonObjectList(v any) []contract.JSONMap {
	switch list := v.(type) {
	case []contract.JSONMap:
		return list
	case []any:
		out := make([]contract.JSONMap, 0, len(list))
		for _, item := range list {
			if m, ok := item.(map[string]any); ok {
				out = append(out, contract.JSONMap(m))
			}
		}
		return out
	}
	return nil
}

// jsonStrMap absorbs map[string]string (in-memory) and map[string]any (DB
// JSON) — the shape of normalized.selector. 파일 상단 흡수 계약(두 형상 모두)에
// 맞춘다 — P1-E 글루가 발견한 갭(DB JSON 왕복 형상에서 셀렉터 매칭이 공백이
// 되는 결함, 승계 스왑 동치 실패로 검출).
func jsonStrMap(v any) map[string]string {
	if m, ok := v.(map[string]string); ok {
		return m
	}
	if m, ok := v.(map[string]any); ok {
		out := make(map[string]string, len(m))
		for key, value := range m {
			out[key] = jsonString(value)
		}
		return out
	}
	return nil
}

func jsonAnyMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// --- 행 접근자 — Raw의 metadata 성분(수집 규약 mapping.md §2) ---

func rawNamespace(row ProjectedResource) string {
	return jsonString(row.Raw["namespace"])
}

func rawCreationTimestamp(row ProjectedResource) string {
	return jsonString(row.Raw["creationTimestamp"])
}

// rowOwners는 Raw.ownerReferences(uid·kind·name 3성분 — P1-A 수집)를 되살린다.
func rowOwners(row ProjectedResource) []contract.JSONMap {
	return jsonObjectList(row.Raw["ownerReferences"])
}

func ownerField(owner contract.JSONMap, key string) string {
	return jsonString(owner[key])
}

func matchLabels(labels, selector map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}

// --- 공용 포맷터 (v1 오라클 동치 — 센티널·수량 표시는 조립 소유) ---

// FallbackText mirrors legacy fallbackText — 공백은 "-" 센티널.
func FallbackText(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

// IntLabel mirrors legacy intLabel — "%d"+suffix (distribution "3 nodes").
func IntLabel(value int, suffix string) string {
	return strconv.Itoa(value) + suffix
}

// HumanizeAge mirrors legacy humanizeAge — RFC3339 → "Just now"/m/h/d/날짜.
// 비결정 구간(30일 이내)은 VOLATILE로 비교 집합 밖이다(§15.3).
func HumanizeAge(timestamp string) string {
	createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(timestamp))
	if err != nil {
		return "-"
	}
	duration := time.Since(createdAt)
	if duration < time.Minute {
		return "Just now"
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	if duration < 30*24*time.Hour {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
	return createdAt.Format("2006-01-02")
}

// FormatTimestamp mirrors legacy formatTimestamp — "2006-01-02 15:04".
func FormatTimestamp(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}

// FormatCPUMilli mirrors legacy formatCPUMilli — 1000의 배수는 "N cores".
func FormatCPUMilli(value int64) string {
	if value >= 1000 && value%1000 == 0 {
		return fmt.Sprintf("%d cores", value/1000)
	}
	return fmt.Sprintf("%dm", value)
}

// FormatMemoryBytes mirrors legacy formatMemoryBytes — GiB 소수 1자리 vs Mi.
func FormatMemoryBytes(value int64) string {
	if value >= 1024*1024*1024 {
		return fmt.Sprintf("%.1fGi", float64(value)/(1024*1024*1024))
	}
	return fmt.Sprintf("%.0fMi", float64(value)/(1024*1024))
}

// FormatMemoryMBFromBytes mirrors legacy formatMemoryMB의 표시부(10^6 MB 반올림
// — TestCharBuildNodeItems "8590 MB"). 원천이 allocatableMemoryGB(GB 3소수)라
// 왕복 손실이 있어 bytes 역변환(round) 후 표시한다 — 역변환은
// allocatableBytes(하단)가 소유하고 본 함수는 순수 표시만 한다.
func FormatMemoryMBFromBytes(bytes int64) string {
	if bytes <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d MB", int64(math.Round(float64(bytes)/1000/1000)))
}

// FormatCoresText renders allocatableCoresGB back to v1's raw-string CPU 표시
// 형상(TestCharBuildNodeItems CPU:"4"). m 접미 원문량("4000m")은 GB에서 복원되지
// 않는다 — 매핑 표 §4.3에 기록된 알려진 왕복 한계다(P1-D 판단 기록).
func FormatCoresText(cores float64) string {
	return strconv.FormatFloat(cores, 'f', -1, 64)
}

// FormatServiceListPort mirrors legacy formatServiceListPort — "80:30080/TCP".
func FormatServiceListPort(port int, nodePort int, protocol string) string {
	proto := strings.TrimSpace(protocol)
	if proto == "" {
		proto = "TCP"
	}
	if nodePort > 0 {
		return fmt.Sprintf("%d:%d/%s", port, nodePort, proto)
	}
	return fmt.Sprintf("%d/%s", port, proto)
}

// JoinAndLimit mirrors legacy joinAndLimit — trim·""·"-" 폐기 중복 제거 후
// limit 초과분은 " +%d" 가산 표기, 공백은 "-".
func JoinAndLimit(values []string, limit int) string {
	values = uniqueNonEmptyStrings(values)
	if len(values) == 0 {
		return "-"
	}
	if limit > 0 && len(values) > limit {
		return strings.Join(values[:limit], ", ") + fmt.Sprintf(" +%d", len(values)-limit)
	}
	return strings.Join(values, ", ")
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "-" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// StatusText mirrors legacy k8sStatusText — cluster 표시 어휘.
func StatusText(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "warning":
		return "Partial Alerts"
	case "offline":
		return "Offline"
	default:
		return "Running"
	}
}

// NodeReadyStatus mirrors legacy nodeReadyStatus의 v1 어휘(Ready/NotReady/
// Unknown). V2 수집은 readyCondition(True/False/부재)을 보존한다(판단 기록
// P1-D-1 — healthState 2치로는 v1 3치 표시 계약 재현 불가).
func NodeReadyStatus(readyCondition string) string {
	switch strings.TrimSpace(readyCondition) {
	case "True":
		return "Ready"
	case "False":
		return "NotReady"
	default:
		return "Unknown"
	}
}

// JoinNodeRoles mirrors legacy joinNodeRoles의 결합부 — 수집측이 이미 v1 어휘
// (빈 접미→worker·정렬·중복 허용)로 확정하므로 결합만 하되, 빈 목록은 v1의
// "역할 없음 → worker" 기본값으로 닫는다(표시 경계 방어 — 정규화 이전 관측 호환).
func JoinNodeRoles(roles []string) string {
	joined := make([]string, 0, len(roles))
	for _, role := range roles {
		if strings.TrimSpace(role) != "" {
			joined = append(joined, role)
		}
	}
	if len(joined) == 0 {
		return "worker"
	}
	sort.Strings(joined)
	return strings.Join(joined, ",")
}

// CronJobReadyText mirrors legacy cronJobReadyText — suspend 우선.
func CronJobReadyText(suspend bool, active int) string {
	if suspend {
		return "Suspended"
	}
	if active > 0 {
		return fmt.Sprintf("%d Active", active)
	}
	return "Scheduled"
}

// --- 집계 (판정 ①② (가) — V2 인벤토리 흡수, 사용자 승인 2026-09-10) ---

// AggregateMetrics mirrors legacy k8sAggregateMetrics — 요청치 합과 경보 수.
type AggregateMetrics struct {
	TotalAllocCPUMilli    int64
	TotalAllocMemoryBytes int64
	TotalReqCPUMilli      int64
	TotalReqMemoryBytes   int64
	AlertCount            int
}

// allocatableBytes·allocatableMilli는 GB(3소수) 역변환이다. quantityToGB·
// quantityToCores가 round3로 저장하므로 round 역변환으로 원 정수량이 복원된다
// (오차 상한 0.0005 GB — "Prefer requested capacity" 표시 원문은 capacityRaw로
// 별도 보존한다).
func allocatableMilli(coresGB float64) int64 { return int64(math.Round(coresGB * 1000)) }
func allocatableBytes(gb float64) int64      { return int64(math.Round(gb * 1024 * 1024 * 1024)) }

// CalculateK8sAggregateMetrics mirrors legacy calculateK8sAggregateMetrics —
// 노드는 allocatable 합과 (Ready 아님 || unschedulable) 경보, pod는
// failed/pending/unknown 경보와 실행 컨테이너 requests 합.
func CalculateK8sAggregateMetrics(nodeRows []ProjectedResource, podRows []ProjectedResource) AggregateMetrics {
	var metrics AggregateMetrics
	for _, row := range nodeRows {
		if v, ok := jsonFloat(row.Normalized["allocatableCoresGB"]); ok {
			metrics.TotalAllocCPUMilli += allocatableMilli(v)
		}
		if v, ok := jsonFloat(row.Normalized["allocatableMemoryGB"]); ok {
			metrics.TotalAllocMemoryBytes += allocatableBytes(v)
		}
		if NodeReadyStatus(jsonString(row.Normalized["readyCondition"])) != "Ready" || jsonBool(row.Normalized["unschedulable"]) {
			metrics.AlertCount++
		}
	}
	for _, row := range podRows {
		switch strings.ToLower(jsonString(row.Normalized["phase"])) {
		case "failed", "pending", "unknown":
			metrics.AlertCount++
		}
		for _, container := range jsonObjectList(row.Normalized["containers"]) {
			requests := jsonAnyMap(container["requests"])
			if v, ok := jsonInt(requests["cpuMilli"]); ok {
				metrics.TotalReqCPUMilli += v
			}
			if v, ok := jsonInt(requests["memBytes"]); ok {
				metrics.TotalReqMemoryBytes += v
			}
		}
	}
	return metrics
}

// CalculateHealthScore mirrors legacy calculateHealthScore — 경보 1건당 8점
// 감점, 하한 40.
func CalculateHealthScore(alertCount int) int {
	if alertCount <= 0 {
		return 100
	}
	score := 100 - alertCount*8
	if score < 40 {
		return 40
	}
	return score
}

// FormatUsagePercent mirrors legacy formatUsagePercent — 비율 1소수, 0 이하는
// "-" 센티널.
func FormatUsagePercent(used int64, total int64) string {
	if used <= 0 || total <= 0 {
		return "-"
	}
	value := float64(used) / float64(total) * 100
	return fmt.Sprintf("%.1f%%", value)
}

// --- distribution (조립 — 판정 ⑤ 화이트리스트 2키 + node podCIDR 폴백) ---

// BuildOverviewDistribution mirrors legacy buildOverviewDistribution — 5행 고정
// 라벨. cluster 속성(StatusText·Version·NodeCount)은 뷰에서, CIDR은 관측에서.
func BuildOverviewDistribution(cluster v1model.K8sClusterView, nodeRows []ProjectedResource, configMapRows []ProjectedResource) []v1model.K8sKVTextItem {
	serviceCIDR, podCIDR := ResolveK8sNetworkCIDRs(nodeRows, configMapRows)
	return []v1model.K8sKVTextItem{
		{Label: "Cluster Status", Value: cluster.StatusText},
		{Label: "Cluster Version", Value: FallbackText(cluster.Version)},
		{Label: "Node Count", Value: IntLabel(cluster.NodeCount, " nodes")},
		{Label: "Service CIDR", Value: serviceCIDR},
		{Label: "Pod Network", Value: podCIDR},
	}
}

// ResolveK8sNetworkCIDRs mirrors legacy resolveK8sNetworkCIDRs. serviceCIDR은
// kubeadm-config 2키 화이트리스트(판정 ⑤ b — 사용자 승인 2026-09-10)로 수집된
// normalized.serviceCIDR에서만 나오고, 부재 시 추측 없이 "Unknown"이다. podCIDR
// 폴백은 노드 podCIDRs(P1-A)의 정렬·중복 제거·"、" 결합이다.
func ResolveK8sNetworkCIDRs(nodeRows []ProjectedResource, configMapRows []ProjectedResource) (string, string) {
	serviceCIDR, podCIDR := "Unknown", "Unknown"
	for _, row := range configMapRows {
		if rawNamespace(row) != "kube-system" || row.DisplayName != "kubeadm-config" {
			continue
		}
		if v := jsonString(row.Normalized["serviceCIDR"]); v != "" {
			serviceCIDR = v
		}
		if v := jsonString(row.Normalized["podSubnetCIDR"]); v != "" {
			podCIDR = v
		}
	}
	if podCIDR == "Unknown" {
		cidrs := make([]string, 0)
		seen := map[string]struct{}{}
		for _, node := range nodeRows {
			for _, cidr := range jsonStrList(node.Normalized["podCIDRs"]) {
				cidr = strings.TrimSpace(cidr)
				if cidr == "" {
					continue
				}
				if _, exists := seen[cidr]; !exists {
					seen[cidr] = struct{}{}
					cidrs = append(cidrs, cidr)
				}
			}
		}
		if len(cidrs) > 0 {
			sort.Strings(cidrs)
			podCIDR = strings.Join(cidrs, "、")
		}
	}
	return serviceCIDR, podCIDR
}
