package proxmox

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/metrics"
)

// adapter.go — V2 read-only PVE 어댑터 공개면 (계획 N3·§3.1 — BaseAdapter +
// Discoverer). 어댑터는 제어평면 테이블·DB 핸들 무소유(arch rule 2) — 자격은
// ConnectionView.Material["inventory"] JSON blob {tokenUser,tokenID,tokenSecret}
// 으로만 받고(판정 J5), 전송은 Phase B의 Client(요청 스코프 — stateless)를
// 매 요청 재조립해 쓴다. executor 소비면(Execute/Poll)은 Phase D 소관이라
// 여기 착지하지 않는다.

// ProviderName is the Prometheus provider label (metrics 렌더 라인
// provider="proxmox" — §18.2 라벨 형식) and the §7.1 vocabulary value
// (Phase A에서 승격된 어휘 — 코어 분기 아님).
const ProviderName = "proxmox"

// pageSize bounds each Discover page (default). WithPageSize overrides — the
// contracttest harness asserts the forced bound. PVE API는 자체 페이징이
// 없으므로(판정 J3) 상한은 어댑터가 클라이언트 측 슬라이싱으로 집행한다.
const defaultPageSize = 100

// Adapter is the V2 read-only Proxmox VE adapter (BaseAdapter + Discoverer,
// contract §11).
type Adapter struct {
	metrics  *metrics.Counters
	pageSize int
}

// Option configures an Adapter at construction.
type Option func(*Adapter)

// WithCounters shares a metrics counter set with the caller (compose —
// sync 리포트 아티팩트가 동일 카운터의 Render를 병기한다).
func WithCounters(c *metrics.Counters) Option {
	return func(a *Adapter) { a.metrics = c }
}

// WithPageSize overrides the per-Discover page bound (페이징 단얫용).
func WithPageSize(n int) Option {
	return func(a *Adapter) {
		if n > 0 {
			a.pageSize = n
		}
	}
}

// NewAdapter builds the proxmox adapter and registers its provider row in the
// counter set so Render carries the {provider="proxmox"} 0 line the gate ③
// flat proof reads (k8s/클라우드 NewAdapter 승계).
func NewAdapter(opts ...Option) *Adapter {
	a := &Adapter{metrics: metrics.New(), pageSize: defaultPageSize}
	a.metrics.RegisterProviders(ProviderName)
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Descriptor — Type "proxmox", AdapterVersion "1", ProtocolVersion "1",
// ContextKinds ["cluster"], BuiltIn true, 빈 ConfigSpec (계획 §3.1 —
// k8s/aliyun Descriptor와 동일 형상).
func (*Adapter) Descriptor() contract.ProviderTypeDescriptor {
	return contract.ProviderTypeDescriptor{
		Type:            ProviderName,
		AdapterVersion:  "1",
		ProtocolVersion: "1",
		ConfigSchema:    func() contract.ConfigSpec { return contract.ConfigSpec{} },
		ContextKinds:    []string{"cluster"},
		BuiltIn:         true,
	}
}

// RateCounter exposes the counter set — the contracttest harness rate-limit
// assertion and the report artifact path both read it.
func (a *Adapter) RateCounter() *metrics.Counters { return a.metrics }

// ProviderName is the Prometheus provider label this adapter reports under.
func (*Adapter) ProviderName() string { return ProviderName }

// Close is a no-op — the adapter holds no long-lived provider state; HTTP
// clients are per-request (stateless credentials via req.Connection).
func (*Adapter) Close() error { return nil }

// --- 자격·전송 조립 — req.Connection만 읽는다(arch rule 2). ---

// buildClient resolves credentials + transport posture for one request.
// ConfigJSON의 deployment_mode=reverse_proxy·tls_insecure는 Phase B 클라이언트의
// 배치 posture 옵션으로 번역된다(판정 J6(3)·A7).
func (a *Adapter) buildClient(conn contract.ConnectionView) (*Client, error) {
	material := ""
	if conn.Material != nil {
		material = conn.Material["inventory"]
	}
	cred, err := parseTokenCredential(material)
	if err != nil {
		return nil, err
	}
	opts := []clientOption{withMetrics(a.metrics)}
	if mode, _ := conn.Config["deployment_mode"].(string); mode == "reverse_proxy" {
		opts = append(opts, withReverseProxy())
	}
	if insecure, _ := conn.Config["tls_insecure"].(bool); insecure {
		opts = append(opts, withInsecureTLS())
	}
	return newClient(cred, conn.Endpoint, opts...), nil
}

// pveVersion is the GET /version payload subset (§3.1 Validate 스모크).
type pveVersion struct {
	Version string `json:"version"`
}

// clusterHealthEntry — Health가 /cluster/status를 해독하는 형상. normalizer의
// clusterStatusEntry(Phase B 전송 소유)에 quorum 성분이 없어 별도 디코드다 —
// 리소스 정규화 경로와 무관하다(A10).
type clusterHealthEntry struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Quorate *int   `json:"quorate"`
	Online  *int   `json:"online"`
	IP      string `json:"ip"`
}

// Validate — Material["inventory"] JSON 파싱 + GET /version (계획 §3.1 —
// realm·클러스터명 확인은 Health/디스커버리 표면의 소관).
func (a *Adapter) Validate(ctx context.Context, conn contract.ConnectionView) error {
	client, err := a.buildClient(conn)
	if err != nil {
		return err
	}
	var v pveVersion
	if err := client.get(ctx, "/version", "validate", &v); err != nil {
		return fmt.Errorf("proxmox: validate: %w", err)
	}
	if strings.TrimSpace(v.Version) == "" {
		return fmt.Errorf("proxmox: validate: empty version")
	}
	return nil
}

// Health — GET /cluster/status 도달성. Message는 클러스터명(또는 standalone
// 폴백 노드명 — A11)·노드 수·쿼럼 상태를 담되 토큰 물질은 담지 않는다(계획
// §3.1·claim 13).
func (a *Adapter) Health(ctx context.Context, conn contract.ConnectionView) contract.HealthResult {
	client, err := a.buildClient(conn)
	if err != nil {
		return contract.HealthResult{Healthy: false, Message: err.Error()}
	}
	var entries []clusterHealthEntry
	if err := client.get(ctx, "/cluster/status", "health", &entries); err != nil {
		return contract.HealthResult{Healthy: false, Message: fmt.Sprintf("proxmox: health probe failed: %v", err)}
	}
	var (
		cluster   *clusterHealthEntry
		firstNode string
		nodeCount int
	)
	for i := range entries {
		switch entries[i].Type {
		case "cluster":
			e := entries[i]
			cluster = &e
		case "node":
			nodeCount++
			if firstNode == "" {
				firstNode = entries[i].Name
			}
		}
	}
	if cluster != nil {
		quorum := "unknown"
		if cluster.Quorate != nil {
			if *cluster.Quorate != 0 {
				quorum = "quorate"
			} else {
				quorum = "not quorate"
			}
		}
		return contract.HealthResult{
			Healthy: true,
			Message: fmt.Sprintf("proxmox cluster %s: %d nodes (quorum: %s)", cluster.Name, nodeCount, quorum),
		}
	}
	// standalone 폴백(A11 — /cluster/status가 cluster행 없이 node행만 반환):
	// 클러스터명을 허위 구성하지 않고 노드명으로 보고한다.
	return contract.HealthResult{
		Healthy: true,
		Message: fmt.Sprintf("proxmox standalone node %s: %d node(s) (no cluster quorum)", firstNode, nodeCount),
	}
}

// --- Discoverer — 섹션 커서 페이징 (판정 J3) ---

// discoverSections is the fixed discovery section order — plan §3.1:
// status→nodes→qemu→lxc→storage→network. nodes는 리소스를 내지 않는 순회
// 전용 섹션이다(노드 목록 열거 — /cluster/status node행이 hypervisor_node
// 리소스의 유일 출처라 /nodes 재산출은 중복이 된다).
var discoverSections = []string{"status", "nodes", "qemu", "lxc", "storage", "network"}

// perNodeSections — 노드×섹션 순회 대상(qemu·lxc·storage·network — 노드 스코프
// API). 섹션 우선 순회: 같은 섹션의 모든 온라인 노드를 지나 다음 섹션으로 간다.
var perNodeSections = map[string]bool{"qemu": true, "lxc": true, "storage": true, "network": true}

// cursor format: "<section>|<node>|<index>" — section은 고정 순서의 현재 위치,
// node는 per-node 섹션의 현재 노드(status·nodes는 빈 값), index는 그 노드의
// 섹션 목록에서 다음에 반환할 오프셋(PVE API는 자체 페이징이 없으므로
// 클라이언트 측 슬라이싱 — 판정 J3). Terminal cursor is "".
func encodeCursor(section, node string, index int) string {
	return section + "|" + node + "|" + strconv.Itoa(index)
}

func splitCursor(cursor string) (section, node string, index int, err error) {
	if strings.TrimSpace(cursor) == "" {
		return discoverSections[0], "", 0, nil
	}
	rawSection, rest, ok := strings.Cut(cursor, "|")
	if !ok {
		return "", "", 0, fmt.Errorf("proxmox: malformed cursor %q (want <section>|<node>|<index>)", cursor)
	}
	known := false
	for _, s := range discoverSections {
		if s == rawSection {
			known = true
			break
		}
	}
	if !known {
		return "", "", 0, fmt.Errorf("proxmox: unknown cursor section %q", rawSection)
	}
	rawNode, rawIndex, ok := strings.Cut(rest, "|")
	if !ok {
		return "", "", 0, fmt.Errorf("proxmox: malformed cursor %q (want <section>|<node>|<index>)", cursor)
	}
	index, convErr := strconv.Atoi(rawIndex)
	if convErr != nil || index < 0 {
		return "", "", 0, fmt.Errorf("proxmox: malformed cursor %q (index must be a non-negative integer)", cursor)
	}
	return rawSection, rawNode, index, nil
}

// onlineNodes enumerates the node traversal list — GET /nodes의 online 행만
// (offline 노드의 per-node API는 실패하므로 순회에서 제외하고, 그 노드는
// /cluster/status node행으로만 관측된다 — 판정 J3·§9.2).
func onlineNodes(ctx context.Context, client *Client) ([]string, error) {
	var rows []struct {
		Node   string `json:"node"`
		Status string `json:"status"`
	}
	if err := client.get(ctx, "/nodes", "discover", &rows); err != nil {
		return nil, err
	}
	nodes := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Status == "online" && r.Node != "" {
			nodes = append(nodes, r.Node)
		}
	}
	return nodes, nil
}

func indexOfNode(nodes []string, node string) int {
	for i, n := range nodes {
		if n == node {
			return i
		}
	}
	return -1
}

// advanceFamily moves the walk one (node, section) unit forward: next node in
// the same family, else the next section, else termination. 존재하지 않는 노드를
// 가리키는 커서(페이지 사이 노드 목록 변동)는 다음 섹션 선두로 건너뛴다 —
// 종결성이 항상 보장되는 유일 분기다.
func advanceFamily(section, node string, nodes []string) (nextSection, nextNode string, terminated bool) {
	if pos := indexOfNode(nodes, node); pos >= 0 && pos+1 < len(nodes) {
		return section, nodes[pos+1], false
	}
	idx := -1
	for i, s := range discoverSections {
		if s == section {
			idx = i
			break
		}
	}
	if idx+1 >= len(discoverSections) {
		return "", "", true
	}
	next := discoverSections[idx+1]
	if perNodeSections[next] {
		if len(nodes) == 0 {
			return advanceFamily(next, "", nodes) // 빈 노드 목록 — 다음 섹션으로 계단식
		}
		return next, nodes[0], false
	}
	return next, "", false
}

// Discover returns one page of resources, walking the cursor chain through the
// fixed section order. Page bound = pageSize — a page never exceeds it
// (contracttest 단얫 1). 실패한 노드의 리소스는 그 세대에서 결번이고 어댑터는
// tombstone을 내지 않는다(§9.2 — provider outage is not resource deletion).
func (a *Adapter) Discover(ctx context.Context, req contract.DiscoverRequest) (contract.DiscoverPage, error) {
	client, err := a.buildClient(req.Connection)
	if err != nil {
		return contract.DiscoverPage{}, err
	}

	section, node, index, err := splitCursor(req.Cursor)
	if err != nil {
		return contract.DiscoverPage{}, err
	}

	resources := make([]contract.DiscoveredResource, 0, a.pageSize)
	var nodes []string // 온라인 노드 순회 목록 — 최초 진입 시 1회 조회

	for {
		switch {
		case section == "status":
			var entries []clusterStatusEntry
			if err := client.get(ctx, "/cluster/status", "discover", &entries); err != nil {
				return contract.DiscoverPage{}, err
			}
			for i := index; i < len(entries) && len(resources) < a.pageSize; i++ {
				if entries[i].Type != "node" {
					// cluster행은 리소스 행이 아니다(ProviderContext — 등록 CLI
					// 소관). standalone(A11)에서도 노드행만 정규화한다.
					index++
					continue
				}
				res, normErr := normalizeHypervisorNode(req.ContextID, entries[i])
				if normErr != nil {
					return contract.DiscoverPage{}, normErr
				}
				resources = append(resources, res)
				index++
			}
			if len(resources) >= a.pageSize {
				return contract.DiscoverPage{Resources: resources, NextCursor: encodeCursor("status", "", index)}, nil
			}
			section, node, index = "nodes", "", 0

		case section == "nodes":
			// 순회 전용 섹션 — 리소스를 내지 않는다(위 주석).
			if nodes == nil {
				if nodes, err = onlineNodes(ctx, client); err != nil {
					return contract.DiscoverPage{}, err
				}
			}
			terminated := false
			section, node, terminated = advanceFamily("nodes", "", nodes)
			if terminated {
				return contract.DiscoverPage{Resources: resources, NextCursor: ""}, nil
			}

		default: // per-node family — qemu | lxc | storage | network
			if nodes == nil {
				if nodes, err = onlineNodes(ctx, client); err != nil {
					return contract.DiscoverPage{}, err
				}
			}
			if indexOfNode(nodes, node) < 0 {
				// 커서가 가리킨 노드가 순회 목록에 없다(변동) — 다음 유닛으로.
				// 사라진 노드는 목록에서 서수를 잃어 advanceFamily가 다음 섹션으로
				// 건너뛴다(잔여 노드의 해당 섹션은 이번 세대 결번 — §9.2 사다리가
				// 관측한다). 새 유닛은 반드시 index 0부터 — 잔류 index가 앞 항목을
				// 무음 스킵하게 두지 않는다(④리뷰 HIGH-1).
				terminated := false
				section, node, terminated = advanceFamily(section, node, nodes)
				if terminated {
					return contract.DiscoverPage{Resources: resources, NextCursor: ""}, nil
				}
				index = 0
				continue
			}

			remaining := a.pageSize - len(resources)
			unitDone := false
			switch section {
			case "qemu", "lxc":
				var items []guestItem
				if err := client.get(ctx, "/nodes/"+node+"/"+section, "discover", &items); err != nil {
					return contract.DiscoverPage{}, err
				}
				for i := index; i < len(items) && remaining > 0; i++ {
					res, normErr := normalizeGuest(req.ContextID, node, section, items[i])
					if normErr != nil {
						return contract.DiscoverPage{}, normErr
					}
					resources = append(resources, res)
					remaining--
					index++
				}
				unitDone = index >= len(items)
			case "storage":
				var items []storageItem
				if err := client.get(ctx, "/nodes/"+node+"/storage", "discover", &items); err != nil {
					return contract.DiscoverPage{}, err
				}
				for i := index; i < len(items) && remaining > 0; i++ {
					res, normErr := normalizeStorage(req.ContextID, node, items[i])
					if normErr != nil {
						return contract.DiscoverPage{}, normErr
					}
					resources = append(resources, res)
					remaining--
					index++
				}
				unitDone = index >= len(items)
			case "network":
				var items []networkItem
				if err := client.get(ctx, "/nodes/"+node+"/network", "discover", &items); err != nil {
					return contract.DiscoverPage{}, err
				}
				for i := index; i < len(items) && remaining > 0; i++ {
					res, normErr := normalizeNetwork(req.ContextID, node, items[i])
					if normErr != nil {
						return contract.DiscoverPage{}, normErr
					}
					resources = append(resources, res)
					remaining--
					index++
				}
				unitDone = index >= len(items)
			default:
				return contract.DiscoverPage{}, fmt.Errorf("proxmox: unknown cursor section %q", section)
			}

			if !unitDone {
				return contract.DiscoverPage{Resources: resources, NextCursor: encodeCursor(section, node, index)}, nil
			}
			terminated := false
			section, node, terminated = advanceFamily(section, node, nodes)
			if terminated {
				return contract.DiscoverPage{Resources: resources, NextCursor: ""}, nil
			}
			index = 0
		}
	}
}

// Compile-time interface conformance (contract §11).
var (
	_ contract.BaseAdapter = (*Adapter)(nil)
	_ contract.Discoverer  = (*Adapter)(nil)
)
