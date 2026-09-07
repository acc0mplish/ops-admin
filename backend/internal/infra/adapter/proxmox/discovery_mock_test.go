package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/contracttest"
)

// discovery_mock_test.go — Phase C(N6) 디스커버리 하니스의 모의 서버·Fixture 절.
// client_test.go(mockProxmox — Phase B 소유)의 뮤텍스·envelope·자격 리터럴
// 관례를 승계하되, 배치표 소유 경계(§12 — client_test.go는 Phase B 파일)를
// 지키기 위해 디스커버리 표면(/cluster/status·/nodes·
// /nodes/{n}/{qemu,lxc,storage,network})을 이 파일의 별도 서버로 재현한다.
// 모의 응답 형상은 PVE API2 공개 문서 기반(가정 A2). 파일은 800라인 상한
// (전역 코딩 규칙)에 따라 시나리오 절(discovery_test.go)과 분리되어 있다.

// pxTestPageSize — 하네스가 강제하는 페이지 상한(k8s/클라우드 testPageSize=4 관례).
const pxTestPageSize = 4

// discoveryStatusSeed — /cluster/status data 배열. pve3는 offline 멤버지만 ip를
// 보고한다(증류 계약 4 — 오프라인 노드 IP 보존의 입력). client_test.go의
// clusterStatusSeed와 동일 형상(같은 PVE 문서 형상의 재현 — 의도적 중복이 아니라
// 소유 경계 분리).
func discoveryStatusSeed() []map[string]any {
	return []map[string]any{
		{"type": "cluster", "name": "testcluster", "quorate": 1, "id": "cluster", "nodes": 3},
		{"type": "node", "name": "pve1", "ip": "10.20.0.11", "online": 1, "id": "node/pve1"},
		{"type": "node", "name": "pve2", "ip": "10.20.0.12", "online": 1, "id": "node/pve2"},
		{"type": "node", "name": "pve3", "ip": "10.20.0.13", "online": 0, "id": "node/pve3"},
	}
}

// discoveryGuestSeed — GET /nodes/{n}/qemu|lxc data 배열. pve1/101는 이름·용량
// 성분이 빠진 nil-sparse 행(p4 R8 교훈 — nil 포인터 시나리오의 입력)이고,
// pve1/100의 undecodedCanary 필드는 normalizer가 해독하지 않는 응답 필드에 심은
// redaction 카나리(contracttest.MarkerValue)다 — Raw/Normalized가 미해독 필드를
// 새지 않는다는 하네스 단얫 4의 공격 입력이 된다.
func discoveryGuestSeed(node, section string) []map[string]any {
	qemu := map[string][]map[string]any{
		"pve1": {
			{"vmid": 100, "name": "web-01", "status": "running", "template": 0,
				"maxmem": 2147483648, "maxcpu": 2, "maxdisk": 34359738368,
				"tags": "prod,web", "uptime": 86400,
				"undecodedCanary": contracttest.MarkerValue},
			{"vmid": 101, "status": "stopped"},
		},
		"pve2": {
			{"vmid": 200, "name": "edge-01", "status": "running",
				"maxmem": 4294967296, "maxcpu": 4, "maxdisk": 68719476736},
		},
	}
	lxc := map[string][]map[string]any{
		"pve1": {
			{"vmid": 110, "name": "ct-01", "status": "running",
				"maxmem": 536870912, "maxcpu": 1, "maxdisk": 8589934592},
		},
		"pve2": {},
	}
	if section == "qemu" {
		return qemu[node]
	}
	return lxc[node]
}

// discoveryStorageSeed — GET /nodes/{n}/storage data 배열.
func discoveryStorageSeed(node string) []map[string]any {
	all := map[string][]map[string]any{
		"pve1": {
			{"storage": "local", "type": "dir", "content": "iso,vztmpl,backup",
				"status": "available", "active": 1, "shared": 0,
				"total": 107374182400, "used": 53687091200, "avail": 53687091200},
			{"storage": "local-zfs", "type": "zfs", "content": "images,rootdir",
				"status": "available", "active": 1, "shared": 0,
				"total": 214748364800, "used": 107374182400, "avail": 107374182400},
		},
		"pve2": {
			{"storage": "local", "type": "dir", "content": "iso,vztmpl,backup",
				"status": "available", "active": 1, "shared": 0,
				"total": 107374182400, "used": 107374182400, "avail": 0},
		},
	}
	return all[node]
}

// discoveryNetworkSeed — GET /nodes/{n}/network data 배열. PVE는 vlan-aware를
// 하이픈 필드명으로 보고한다.
func discoveryNetworkSeed(node string) []map[string]any {
	all := map[string][]map[string]any{
		"pve1": {
			{"iface": "vmbr0", "type": "bridge", "active": 1, "autostart": 1,
				"vlan-aware": 1, "ports": "enp3s0"},
			{"iface": "enp3s0", "type": "eth", "active": 1, "autostart": 1},
		},
		"pve2": {
			{"iface": "vmbr0", "type": "bridge", "active": 1, "autostart": 0},
		},
	}
	return all[node]
}

// discoveryMock — 디스커버리 표면 전용 모의 PVE. mode는 모든 GET 경로의 응답
// 분기(client_test.go mockProxmox의 getMode 어휘 승계)다.
type discoveryMock struct {
	t   *testing.T
	srv *httptest.Server

	mu          sync.Mutex
	mode        string // "" ok | fail500 | fail401 | fail403 | fail429 | badjson
	standalone  bool   // /cluster/status가 cluster행 없이 node행 1개만 보고(A11)
	dropNode    string // 페이지 사이 멤버십 변동 — 이 노드를 status·/nodes에서 소멸(④리뷰 HIGH-1 시나리오)
	authHeaders []string
	getPaths    []string
}

func newDiscoveryMock(t *testing.T) *discoveryMock {
	t.Helper()
	m := &discoveryMock{t: t}
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/", m.serveAPI)
	m.srv = httptest.NewServer(mux)
	t.Cleanup(m.srv.Close)
	return m
}

func (m *discoveryMock) setMode(mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mode = mode
}

func (m *discoveryMock) setStandalone(standalone bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.standalone = standalone
}

func (m *discoveryMock) setDropNode(node string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dropNode = node
}

type discoverySnapshot struct {
	authHeaders []string
	getPaths    []string
}

func (m *discoveryMock) snap() discoverySnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return discoverySnapshot{
		authHeaders: append([]string(nil), m.authHeaders...),
		getPaths:    append([]string(nil), m.getPaths...),
	}
}

func (m *discoveryMock) serveAPI(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	auth := r.Header.Get("Authorization")
	m.authHeaders = append(m.authHeaders, auth)
	mode, standalone, dropNode := m.mode, m.standalone, m.dropNode
	m.mu.Unlock()

	if r.Method != http.MethodGet {
		http.Error(w, `{"data":null}`, http.StatusMethodNotAllowed)
		return
	}
	m.mu.Lock()
	m.getPaths = append(m.getPaths, r.URL.Path)
	m.mu.Unlock()

	if mode != "" {
		switch mode {
		case "fail500":
			http.Error(w, `{"data":null}`, http.StatusInternalServerError)
		case "fail401":
			http.Error(w, `{"data":null}`, http.StatusUnauthorized)
		case "fail403":
			http.Error(w, `{"data":null}`, http.StatusForbidden)
		case "fail429":
			http.Error(w, `{"data":null}`, http.StatusTooManyRequests)
		case "badjson":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not-json"))
		default:
			http.Error(w, `{"data":null}`, http.StatusBadRequest)
		}
		return
	}

	path := r.URL.Path
	switch {
	case path == "/api2/json/cluster/status":
		if standalone {
			envelope(w, []map[string]any{
				{"type": "node", "name": "pve1", "ip": "127.0.0.1", "online": 1, "id": "node/pve1"},
			})
			return
		}
		status := discoveryStatusSeed()
		if dropNode != "" {
			kept := status[:0]
			for _, row := range status {
				if row["name"] != dropNode {
					kept = append(kept, row)
				}
			}
			status = kept
		}
		envelope(w, status)

	case path == "/api2/json/version":
		envelope(w, map[string]any{"version": "8.2.4", "release": "8.2", "repoid": "mock"})

	case path == "/api2/json/nodes":
		nodes := []map[string]any{
			{"node": "pve1", "status": "online"},
			{"node": "pve2", "status": "online"},
		}
		if dropNode != "" {
			kept := nodes[:0]
			for _, row := range nodes {
				if row["node"] != dropNode {
					kept = append(kept, row)
				}
			}
			nodes = kept
		}
		envelope(w, nodes)

	case path == "/api2/json/nodes/pve1/qemu" || path == "/api2/json/nodes/pve2/qemu" ||
		path == "/api2/json/nodes/pve1/lxc" || path == "/api2/json/nodes/pve2/lxc":
		parts := strings.Split(path, "/")
		envelope(w, discoveryGuestSeed(parts[4], parts[5]))

	case path == "/api2/json/nodes/pve1/storage" || path == "/api2/json/nodes/pve2/storage":
		parts := strings.Split(path, "/")
		envelope(w, discoveryStorageSeed(parts[4]))

	case path == "/api2/json/nodes/pve1/network" || path == "/api2/json/nodes/pve2/network":
		parts := strings.Split(path, "/")
		envelope(w, discoveryNetworkSeed(parts[4]))

	default:
		http.Error(w, `{"data":null}`, http.StatusNotFound)
	}
}

// --- 하네스 Fixture — 모의 서버를 시나리오·자격·엔드포인트 주입면으로 감싼다. ---

type discoveryFixture struct {
	t        *testing.T
	mock     *discoveryMock
	adapter  *Adapter
	endpoint string
}

func newDiscoveryFixture(t *testing.T) *discoveryFixture {
	t.Helper()
	mock := newDiscoveryMock(t)
	adapter := NewAdapter(WithPageSize(pxTestPageSize))
	return &discoveryFixture{t: t, mock: mock, adapter: adapter, endpoint: mock.srv.URL}
}

// Seed — 독립 오라클: 기대 URN·kind를 normalizer가 아닌 손으로 적는다
// (self-fulfilling 방지 — k8s/클라우드 Fixture 동일 관례).
func (f *discoveryFixture) Seed() []contract.DiscoveredResource {
	return []contract.DiscoveredResource{
		{ExternalURN: "urn:proxmox:1:hypervisor_node:pve1", Kind: "compute.hypervisor_node"},
		{ExternalURN: "urn:proxmox:1:hypervisor_node:pve2", Kind: "compute.hypervisor_node"},
		{ExternalURN: "urn:proxmox:1:hypervisor_node:pve3", Kind: "compute.hypervisor_node"},
		{ExternalURN: "urn:proxmox:1:vm:pve1/100", Kind: "compute.vm", Subtype: "qemu"},
		{ExternalURN: "urn:proxmox:1:vm:pve1/101", Kind: "compute.vm", Subtype: "qemu"},
		{ExternalURN: "urn:proxmox:1:vm:pve2/200", Kind: "compute.vm", Subtype: "qemu"},
		{ExternalURN: "urn:proxmox:1:system_container:pve1/110", Kind: "compute.system_container", Subtype: "lxc"},
		{ExternalURN: "urn:proxmox:1:pool:pve1/local", Kind: "storage.pool", Subtype: "dir"},
		{ExternalURN: "urn:proxmox:1:pool:pve1/local-zfs", Kind: "storage.pool", Subtype: "zfs"},
		{ExternalURN: "urn:proxmox:1:pool:pve2/local", Kind: "storage.pool", Subtype: "dir"},
		{ExternalURN: "urn:proxmox:1:interface:pve1/vmbr0", Kind: "network.interface", Subtype: "bridge"},
		{ExternalURN: "urn:proxmox:1:interface:pve1/enp3s0", Kind: "network.interface", Subtype: "eth"},
		{ExternalURN: "urn:proxmox:1:interface:pve2/vmbr0", Kind: "network.interface", Subtype: "bridge"},
	}
}

func (*discoveryFixture) PageLimit() int { return pxTestPageSize }

func (f *discoveryFixture) Scenario(name string) error {
	switch name {
	case "":
		f.mock.setMode("")
		f.endpoint = f.mock.srv.URL
	case contract.SignalRateLimited:
		f.mock.setMode("fail429")
	case contract.SignalPermissionDenied:
		f.mock.setMode("fail403")
	case contract.SignalUnreachable:
		f.mock.setMode("")
		f.endpoint = "http://127.0.0.1:1" // connection refused
	default:
		return fmt.Errorf("discoveryFixture: unknown scenario %q", name)
	}
	return nil
}

func (f *discoveryFixture) Connection() contract.ConnectionView {
	return contract.ConnectionView{
		ProviderType: "proxmox",
		Endpoint:     f.endpoint,
		Material:     map[string]string{"inventory": testCredentialMaterial},
	}
}

// walkAll — 커서를 끝까지 순회해 (자원, 페이지 수)를 수집한다.
func walkAll(t *testing.T, f *discoveryFixture) ([]contract.DiscoveredResource, int) {
	t.Helper()
	ctx := context.Background()
	var all []contract.DiscoveredResource
	cursor := ""
	pages := 0
	for {
		page, err := f.adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Cursor: cursor, Connection: f.Connection()})
		if err != nil {
			t.Fatalf("Discover(cursor=%q): %v", cursor, err)
		}
		pages++
		all = append(all, page.Resources...)
		if page.NextCursor == "" {
			return all, pages
		}
		if pages > 64 {
			t.Fatalf("discovery did not terminate (cursor=%q)", page.NextCursor)
		}
		cursor = page.NextCursor
	}
}
