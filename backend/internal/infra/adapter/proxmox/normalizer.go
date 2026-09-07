package proxmox

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"ops-admin/backend/internal/infra/contract"
)

// normalizer.go — PVE 응답 → DiscoveredResource (계획 N4·판정 J3). 기존 kind
// 어휘만 재사용한다(확장 0): /cluster/status node행 → compute.hypervisor_node,
// qemu → compute.vm(Subtype "qemu"), lxc → compute.system_container(Subtype
// "lxc") — QEMU와 LXC는 다른 kind다(§14.3 붕괴 금지). storage → storage.pool
// (Subtype = PVE type), network → network.interface(Subtype = PVE type).
// /cluster/status의 cluster행은 리소스 행이 아니다(ProviderContext 성격 — 등록
// CLI 소관): standalone(A11)에서도 노드행만 정규화하고 클러스터 신원을 허위
// 구성하지 않는다. 클러스터 쿼럼은 어떤 리소스의 상태 필드로도 붕괴되지 않는다
// (A10 — VM 헬스 붕괴 금지).

// MaxRawBytes is the §8.2 "Raw payloads have size limits" bound. 64KiB is the
// plan's introduced value, not a spec number (p4 가정 A12 승계). Overflow
// truncates Raw and stamps normalized["truncated"]=true.
const MaxRawBytes = 64 << 10

// DiscoveryKinds — 이 어댑터의 디스커버리가 산출하는 kind 전수(compose의 읽기
// capability inventory.full ResourceKinds 선언이 소비한다 — 계획 §3.4·C2).
var DiscoveryKinds = []string{
	"compute.hypervisor_node",
	"compute.system_container",
	"compute.vm",
	"network.interface",
	"storage.pool",
}

// --- 섹션별 디코드 구조체 — mapping.md 커버리지 표의 와이어 필드 전수와
// 기계 동치다(TestProxmoxMappingCoverage가 reflect로 단얫). nil 포인터 필드는
// PVE가 성분을 생략하는 응답을 안전히 통과시킨다(p4 R8 교훈). ---

// guestItem — GET /nodes/{n}/qemu|lxc 행(qemu·lxc 공용 형상).
type guestItem struct {
	VMID     int      `json:"vmid"`
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Template *int     `json:"template"`
	MaxMem   *float64 `json:"maxmem"`
	MaxCPU   *float64 `json:"maxcpu"`
	MaxDisk  *float64 `json:"maxdisk"`
	Tags     string   `json:"tags"`
	Uptime   *float64 `json:"uptime"`
}

// storageItem — GET /nodes/{n}/storage 행.
type storageItem struct {
	Storage string   `json:"storage"`
	Type    string   `json:"type"`
	Content string   `json:"content"`
	Status  string   `json:"status"`
	Active  *int     `json:"active"`
	Shared  *int     `json:"shared"`
	Total   *float64 `json:"total"`
	Used    *float64 `json:"used"`
	Avail   *float64 `json:"avail"`
}

// networkItem — GET /nodes/{n}/network 행. PVE는 vlan-aware를 하이픈 필드명으로
// 보고한다.
type networkItem struct {
	Iface     string `json:"iface"`
	Type      string `json:"type"`
	Active    *int   `json:"active"`
	Autostart *int   `json:"autostart"`
	VLANAware *int   `json:"vlan-aware"`
	Ports     string `json:"ports"`
	Slaves    string `json:"slaves"`
	Address   string `json:"address"`
	Gateway   string `json:"gateway"`
}

// --- 단위 정규화 ---

const (
	mib = 1024.0 * 1024.0
	gib = 1024.0 * 1024.0 * 1024.0
)

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

// bytesToGB — PVE 바이트 용량 → GB(GiB 스케일, 3자리 반올림 — k8s
// quantityToGB와 동일 스케일).
func bytesToGB(b float64) float64 { return round3(b / gib) }

// splitCSV — PVE CSV 필드(content·tags) → 정렬되지 않은 리스트.
func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// --- URN 빌더 (§8.1 — mapping.md identity 표와 동치) ---
//
//	urn:proxmox:{ctx}:hypervisor_node:{node}
//	urn:proxmox:{ctx}:vm:{node}/{vmid}
//	urn:proxmox:{ctx}:system_container:{node}/{vmid}
//	urn:proxmox:{ctx}:pool:{node}/{storeid}
//	urn:proxmox:{ctx}:interface:{node}/{iface}
//
// kind 세그먼트는 kind의 마지막 성분(executor URN 파싱 계약 — 판정 J7의
// (vm|system_container) 문언)이고 노드 스코프 성분이 충돌을 피한다.
func urnFor(ctxID uint, kindSeg, scopedID string) string {
	return "urn:proxmox:" + strconv.FormatUint(uint64(ctxID), 10) + ":" + kindSeg + ":" + scopedID
}

// applyRawLimit enforces MaxRawBytes (A12): overflow truncates Raw and stamps
// normalized["truncated"]=true. The kept prefix is base64-wrapped so the
// re-marshalled Raw is bounded by construction (raw JSON escapes could
// otherwise regrow past the limit — k8s normalizer 승계).
func applyRawLimit(res contract.DiscoveredResource) contract.DiscoveredResource {
	b, err := json.Marshal(res.Raw)
	if err == nil && len(b) > MaxRawBytes {
		const overhead = 64
		cut := ((MaxRawBytes - overhead) / 4) * 3
		if cut > len(b) {
			cut = len(b)
		}
		res.Raw = contract.JSONMap{
			"truncated": true,
			"rawB64":    base64.StdEncoding.EncodeToString(b[:cut]),
		}
		if res.Normalized == nil {
			res.Normalized = contract.JSONMap{}
		}
		res.Normalized["truncated"] = true
	}
	return res
}

// --- 종별 정규화 — 반환 필드 집합은 mapping.md 정규화 표와 동치다. ---

// normalizeHypervisorNode — /cluster/status node행 → compute.hypervisor_node.
// offline 멤버도 /cluster/status가 보고하는 유일 표면이므로 그대로 리소스가 되고
// ip는 오프라인에서도 보존된다(증류 계약 4). 클러스터 쿼럼은 여기 진입하지
// 않는다(A10).
func normalizeHypervisorNode(ctxID uint, e clusterStatusEntry) (contract.DiscoveredResource, error) {
	name := strings.TrimSpace(e.Name)
	if name == "" {
		return contract.DiscoveredResource{}, fmt.Errorf("proxmox: cluster status node row carries no name")
	}
	online := e.Online != 0
	res := contract.DiscoveredResource{
		ExternalID:  name,
		ExternalURN: urnFor(ctxID, "hypervisor_node", name),
		Kind:        "compute.hypervisor_node",
		DisplayName: name,
		Raw: contract.JSONMap{
			"type":   "node",
			"name":   name,
			"ip":     e.IP,
			"online": e.Online,
		},
		Normalized: contract.JSONMap{
			"ip":     e.IP,
			"online": online,
			"status": nodeStatusText(online),
		},
	}
	return applyRawLimit(res), nil
}

// nodeStatusText — 노드 상태 어휘(mapping.md): online → "online", offline →
// "offline".
func nodeStatusText(online bool) string {
	if online {
		return "online"
	}
	return "offline"
}

// normalizeGuest — qemu|lxc 게스트 행 → compute.vm(Subtype qemu) |
// compute.system_container(Subtype lxc). 표시명은 name이 없으면 vmid로
// 폴백한다(nil-sparse 행 — p4 R8).
func normalizeGuest(ctxID uint, node, section string, g guestItem) (contract.DiscoveredResource, error) {
	if node == "" {
		return contract.DiscoveredResource{}, fmt.Errorf("proxmox: guest %d carries no node scope", g.VMID)
	}
	var kind, kindSeg, subtype string
	switch section {
	case "qemu":
		kind, kindSeg, subtype = "compute.vm", "vm", "qemu"
	case "lxc":
		kind, kindSeg, subtype = "compute.system_container", "system_container", "lxc"
	default:
		return contract.DiscoveredResource{}, fmt.Errorf("proxmox: unknown guest section %q", section)
	}
	scoped := node + "/" + strconv.Itoa(g.VMID)
	display := g.Name
	if display == "" {
		display = scoped
	}
	n := contract.JSONMap{
		"vmid":   g.VMID,
		"node":   node,
		"status": g.Status,
	}
	if g.Template != nil {
		n["template"] = *g.Template != 0
	}
	if g.MaxMem != nil {
		n["memoryMB"] = round3(*g.MaxMem / mib)
	}
	if g.MaxCPU != nil {
		n["cores"] = round3(*g.MaxCPU)
	}
	if g.MaxDisk != nil {
		n["diskGB"] = bytesToGB(*g.MaxDisk)
	}
	if g.Uptime != nil {
		n["uptimeSeconds"] = int64(math.Round(*g.Uptime))
	}
	n["tags"] = splitCSV(g.Tags)
	res := contract.DiscoveredResource{
		ExternalID:  scoped,
		ExternalURN: urnFor(ctxID, kindSeg, scoped),
		Kind:        kind,
		Subtype:     subtype,
		DisplayName: display,
		Raw: contract.JSONMap{
			"node":  node,
			"vmid":  g.VMID,
			"name":  g.Name,
			"type":  subtype,
			"guest": section,
		},
		Normalized: n,
	}
	return applyRawLimit(res), nil
}

// normalizeStorage — storage 행 → storage.pool(Subtype = PVE type).
func normalizeStorage(ctxID uint, node string, s storageItem) (contract.DiscoveredResource, error) {
	store := strings.TrimSpace(s.Storage)
	if node == "" || store == "" {
		return contract.DiscoveredResource{}, fmt.Errorf("proxmox: storage row carries no node/storeid scope")
	}
	scoped := node + "/" + store
	n := contract.JSONMap{
		"node":    node,
		"storeid": store,
		"content": splitCSV(s.Content),
		"status":  s.Status,
	}
	if s.Active != nil {
		n["active"] = *s.Active != 0
	}
	if s.Shared != nil {
		n["shared"] = *s.Shared != 0
	}
	if s.Total != nil {
		n["capacityGB"] = bytesToGB(*s.Total)
	}
	if s.Used != nil {
		n["usedGB"] = bytesToGB(*s.Used)
	}
	if s.Avail != nil {
		n["availGB"] = bytesToGB(*s.Avail)
	}
	res := contract.DiscoveredResource{
		ExternalID:  scoped,
		ExternalURN: urnFor(ctxID, "pool", scoped),
		Kind:        "storage.pool",
		Subtype:     s.Type,
		DisplayName: store,
		Raw: contract.JSONMap{
			"node":    node,
			"storage": store,
			"type":    s.Type,
			"content": splitCSV(s.Content),
		},
		Normalized: n,
	}
	return applyRawLimit(res), nil
}

// normalizeNetwork — network 행 → network.interface(Subtype = PVE type).
func normalizeNetwork(ctxID uint, node string, i networkItem) (contract.DiscoveredResource, error) {
	iface := strings.TrimSpace(i.Iface)
	if node == "" || iface == "" {
		return contract.DiscoveredResource{}, fmt.Errorf("proxmox: network row carries no node/iface scope")
	}
	scoped := node + "/" + iface
	n := contract.JSONMap{
		"node": node,
		"type": i.Type,
	}
	if i.Active != nil {
		n["active"] = *i.Active != 0
	}
	if i.Autostart != nil {
		n["autostart"] = *i.Autostart != 0
	}
	if i.VLANAware != nil {
		n["vlanAware"] = *i.VLANAware != 0
	}
	if i.Ports != "" {
		n["ports"] = i.Ports
	}
	if i.Slaves != "" {
		n["slaves"] = i.Slaves
	}
	if i.Address != "" {
		n["address"] = i.Address
	}
	if i.Gateway != "" {
		n["gateway"] = i.Gateway
	}
	res := contract.DiscoveredResource{
		ExternalID:  scoped,
		ExternalURN: urnFor(ctxID, "interface", scoped),
		Kind:        "network.interface",
		Subtype:     i.Type,
		DisplayName: iface,
		Raw: contract.JSONMap{
			"node":  node,
			"iface": iface,
			"type":  i.Type,
		},
		Normalized: n,
	}
	return applyRawLimit(res), nil
}
