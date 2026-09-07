package proxmox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/contracttest"
)

// --- T45 승계: proxmox 어댑터가 contract harness 4단얫(§23.1)을 통과한다. ---

func TestProxmoxAdapterPassesContractHarness(t *testing.T) {
	f := newDiscoveryFixture(t)
	contracttest.RunContractSuite(t, f.adapter, f)
}

// Descriptor 계약(§3.1) — Type/버전/ContextKinds/BuiltIn/빈 ConfigSpec.
func TestProxmoxDescriptor(t *testing.T) {
	d := NewAdapter().Descriptor()
	if d.Type != "proxmox" || d.AdapterVersion != "1" || d.ProtocolVersion != "1" {
		t.Errorf("Descriptor = %+v", d)
	}
	if len(d.ContextKinds) != 1 || d.ContextKinds[0] != "cluster" || !d.BuiltIn {
		t.Errorf("Descriptor ContextKinds/BuiltIn = %+v/%v", d.ContextKinds, d.BuiltIn)
	}
	if spec := d.ConfigSchema(); len(spec.Fields) != 0 {
		t.Errorf("ConfigSchema = %+v, want empty spec", spec)
	}
	if err := NewAdapter().Close(); err != nil {
		t.Errorf("Close = %v, want nil", err)
	}
}

// Validate 계약(§3.1) — Material["inventory"] 파싱 + GET /version.
func TestProxmoxValidate(t *testing.T) {
	f := newDiscoveryFixture(t)
	ctx := context.Background()

	if err := f.adapter.Validate(ctx, f.Connection()); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	t.Run("missing_material", func(t *testing.T) {
		err := f.adapter.Validate(ctx, contract.ConnectionView{Endpoint: f.endpoint})
		if err == nil || !strings.Contains(err.Error(), "missing credential material") {
			t.Fatalf("Validate without Material: err = %v", err)
		}
		assertNoTokenMaterial(t, err)
	})

	t.Run("malformed_material", func(t *testing.T) {
		conn := contract.ConnectionView{Endpoint: f.endpoint, Material: map[string]string{"inventory": "{not-json"}}
		err := f.adapter.Validate(ctx, conn)
		if err == nil {
			t.Fatal("Validate with malformed credential blob succeeded")
		}
		assertNoTokenMaterial(t, err)
	})

	t.Run("empty_version_rejected", func(t *testing.T) {
		f2 := newDiscoveryFixture(t)
		f2.mock.setMode("badjson")
		if err := f2.adapter.Validate(ctx, f2.Connection()); err == nil {
			t.Fatal("Validate against bad-json /version succeeded")
		}
	})
}

// Health 계약(§3.1) — 클러스터명·노드 수·쿼럼 상태. standalone 폴백(A11)에서는
// 노드명과 standalone 마커. 어느 쪽도 토큰 물질을 담지 않는다.
func TestProxmoxHealth(t *testing.T) {
	f := newDiscoveryFixture(t)
	res := f.adapter.Health(context.Background(), f.Connection())
	if !res.Healthy {
		t.Fatalf("Health = %+v, want healthy", res)
	}
	for _, want := range []string{"testcluster", "3", "quorate"} {
		if !strings.Contains(res.Message, want) {
			t.Errorf("Health.Message %q lacks %q", res.Message, want)
		}
	}
	assertNoTokenMaterial(t, errors.New(res.Message))

	// standalone 폴백(A11 실측 형상 — cluster행 부재): 노드명 폴백·standalone 표기.
	f2 := newDiscoveryFixture(t)
	f2.mock.setStandalone(true)
	res2 := f2.adapter.Health(context.Background(), f2.Connection())
	if !res2.Healthy {
		t.Fatalf("standalone Health = %+v, want healthy", res2)
	}
	if !strings.Contains(res2.Message, "pve1") || !strings.Contains(res2.Message, "standalone") {
		t.Errorf("standalone Health.Message = %q, want node-name fallback + standalone marker", res2.Message)
	}
	if strings.Contains(res2.Message, "testcluster") {
		t.Errorf("standalone Health.Message %q fabricates the cluster name", res2.Message)
	}
	assertNoTokenMaterial(t, errors.New(res2.Message))
}

// 섹션 커서 — 고정 순서 status→nodes→qemu→lxc→storage→network, 노드×섹션 순회
// (판정 J3). 호출 경로 순서를 와이어 캡처로 단얫한다.
func TestProxmoxDiscoverSectionCursorOrder(t *testing.T) {
	f := newDiscoveryFixture(t)
	all, pages := walkAll(t, f)
	if len(all) != len(f.Seed()) {
		t.Fatalf("discovery total = %d, want %d", len(all), len(f.Seed()))
	}
	if pages < 1 {
		t.Fatalf("walk produced %d pages, want at least 1", pages)
	}
	want := []string{
		"/api2/json/nodes/pve1/qemu",
		"/api2/json/nodes/pve2/qemu",
		"/api2/json/nodes/pve1/lxc",
		"/api2/json/nodes/pve2/lxc",
		"/api2/json/nodes/pve1/storage",
		"/api2/json/nodes/pve2/storage",
		"/api2/json/nodes/pve1/network",
		"/api2/json/nodes/pve2/network",
	}
	// 순회는 stateless라 페이지마다 /nodes를 재조회한다 — 전체 와이어에서
	// 순회 전용 경로를 제외한 프로젝션이 정확히 섹션×노드 순서와 일치해야 하고,
	// 워크는 항상 /cluster/status에서 시작한다.
	got := f.mock.snap().getPaths
	if len(got) == 0 || got[0] != "/api2/json/cluster/status" {
		t.Fatalf("walk must start at /cluster/status, got %v", got)
	}
	var family []string
	for _, p := range got {
		if p == "/api2/json/cluster/status" || p == "/api2/json/nodes" {
			continue
		}
		// 커서 재개는 현재 유닛을 재조회한다(페이지 경계 — 정상). 연속 중복은
		// 접고, 그 뒤로는 유닛 순서가 정확히 섹션×노드 순서여야 한다.
		if len(family) > 0 && family[len(family)-1] == p {
			continue
		}
		family = append(family, p)
	}
	if len(family) != len(want) {
		t.Fatalf("family paths = %v, want %v", family, want)
	}
	for i := range want {
		if family[i] != want[i] {
			t.Fatalf("wire path %d = %q, want %q (full: %v)", i, family[i], want[i], got)
		}
	}
}

// QEMU와 LXC는 다른 kind — 붕괴 금지(§14.3). compute.vm(qemu) 3건과
// compute.system_container(lxc) 1건이 서로 다른 URN 체계로 분리되는지 단얫.
func TestProxmoxDiscoverKindSeparationQemuLxc(t *testing.T) {
	f := newDiscoveryFixture(t)
	all, _ := walkAll(t, f)

	var vms, containers int
	for _, r := range all {
		switch r.Kind {
		case "compute.vm":
			vms++
			if r.Subtype != "qemu" {
				t.Errorf("compute.vm %q Subtype = %q, want qemu", r.ExternalURN, r.Subtype)
			}
			if !strings.Contains(r.ExternalURN, ":vm:") {
				t.Errorf("compute.vm URN %q lacks the vm segment", r.ExternalURN)
			}
		case "compute.system_container":
			containers++
			if r.Subtype != "lxc" {
				t.Errorf("compute.system_container %q Subtype = %q, want lxc", r.ExternalURN, r.Subtype)
			}
			if !strings.Contains(r.ExternalURN, ":system_container:") {
				t.Errorf("system_container URN %q lacks its segment", r.ExternalURN)
			}
		}
	}
	if vms != 3 || containers != 1 {
		t.Fatalf("qemu vm count = %d (want 3), lxc container count = %d (want 1)", vms, containers)
	}
}

// 노드 다운 시나리오(§9.2): offline 멤버 pve3의 hypervisor_node 행은
// /cluster/status에서 관측되고 IP가 보존되며(증류 계약 4), 그 노드의
// 게스트·스토리지·네트워크는 그 세대에서 결번이다. 어댑터는 tombstone·삭제
// 마커를 어떤 형태로도 내지 않는다(삭제는 싱크러너의 소관).
func TestProxmoxDiscoverOfflineNode(t *testing.T) {
	f := newDiscoveryFixture(t)
	all, _ := walkAll(t, f)

	foundOfflineNode := false
	for _, r := range all {
		if r.Kind == "compute.hypervisor_node" && r.ExternalID == "pve3" {
			foundOfflineNode = true
			if r.Normalized["online"] != false {
				t.Errorf("pve3 Normalized.online = %v, want false", r.Normalized["online"])
			}
			if r.Normalized["ip"] != "10.20.0.13" {
				t.Errorf("offline member ip not preserved: %v", r.Normalized["ip"])
			}
		}
		if strings.HasPrefix(r.ExternalID, "pve3/") {
			t.Errorf("offline node pve3 must not yield per-node resources, got %q", r.ExternalURN)
		}
		for _, key := range []string{"deleted", "tombstone"} {
			if _, ok := r.Normalized[key]; ok {
				t.Errorf("resource %q carries a %q marker — the adapter never emits deletion markers", r.ExternalURN, key)
			}
		}
	}
	if !foundOfflineNode {
		t.Fatal("offline node pve3 hypervisor_node row missing — /cluster/status is the only surface reporting it")
	}

	for _, p := range f.mock.snap().getPaths {
		if strings.HasPrefix(p, "/api2/json/nodes/pve3/") {
			t.Errorf("offline node pve3 was queried: %s", p)
		}
	}
}

// ④리뷰 HIGH-1 회귀: 페이지 사이에 커서 노드가 순회 목록에서 사라지면
// (멤버십 변동·재부팅 플래핑) 재개 유닛은 index 0부터 시작해야 한다. 잔류
// index가 앞 항목을 무음 스킵하면 sync 결장 사다리(§9.2 — 2연속 결장→
// stale_candidate·3연속→tombstone)가 라이브 리소스를 오탐 삭제할 수 있다.
// 소멸 노드의 서수를 목록에서 되찾을 수 없어 advanceFamily는 다음 섹션으로
// 건너뛴다(MEDIUM-2 — 잔여 노드의 해당 섹션은 이번 세대 결번·커서 서수
// 재설계 이월). 단, 도착한 유닛의 항목은 전수가 보여야 한다.
func TestProxmoxDiscoverVanishedNodeResetsIndex(t *testing.T) {
	f := newDiscoveryFixture(t)
	f.mock.setDropNode("pve1")

	page, err := f.adapter.Discover(context.Background(), contract.DiscoverRequest{
		ContextID:  1,
		Cursor:     "storage|pve1|1",
		Connection: f.Connection(),
	})
	if err != nil {
		t.Fatalf("Discover(vanished cursor): %v", err)
	}

	want := len(discoveryNetworkSeed("pve2")) // network는 최종 섹션 — pve2 시드 전수
	if len(page.Resources) != want {
		t.Errorf("vanished-node resume must start at index 0: got %d resources, want full pve2 network seed %d (silent skip = stale index)",
			len(page.Resources), want)
	}
	for _, r := range page.Resources {
		if r.Kind != "network.interface" || r.Subtype != "bridge" {
			t.Errorf("expected pve2 network interface, got kind=%q subtype=%q urn=%q", r.Kind, r.Subtype, r.ExternalURN)
		}
		if !strings.HasSuffix(r.ExternalURN, "pve2/vmbr0") {
			t.Errorf("expected urn …pve2/vmbr0, got %q", r.ExternalURN)
		}
	}
	if page.NextCursor != "" {
		t.Errorf("network is the last section — walk must terminate, got cursor %q", page.NextCursor)
	}
}

// standalone 시나리오(A11 — cluster행 없이 node행만): 노드행만 정규화되고,
// 클러스터 신원이 허위 구성되지 않는다. VM 리소스의 상태 필드에 클러스터
// 쿼럼이 붕괴되어 들어가지 않는다(A10 — VM 헬스 붕괴 금지).
func TestProxmoxDiscoverStandalone(t *testing.T) {
	f := newDiscoveryFixture(t)
	f.mock.setStandalone(true)
	all, _ := walkAll(t, f)

	known := map[string]bool{}
	for _, k := range DiscoveryKinds {
		known[k] = true
	}
	var nodeRows int
	for _, r := range all {
		if !known[r.Kind] {
			t.Errorf("standalone discovery emitted kind %q outside the declared kind set", r.Kind)
		}
		if strings.Contains(strings.ToLower(r.ExternalURN), "cluster") && r.Kind != "compute.hypervisor_node" {
			t.Errorf("standalone discovery fabricated a cluster identity: %q", r.ExternalURN)
		}
		if r.Kind == "compute.hypervisor_node" {
			nodeRows++
			for _, key := range []string{"quorum", "quorate"} {
				if _, ok := r.Normalized[key]; ok {
					t.Errorf("node resource %q carries cluster-level %q — cluster quorum must not collapse into node state", r.ExternalURN, key)
				}
			}
		}
	}
	if nodeRows != 1 {
		t.Fatalf("standalone node rows = %d, want 1", nodeRows)
	}
	var vms int
	for _, r := range all {
		if r.Kind == "compute.vm" {
			vms++
			if _, ok := r.Normalized["status"]; !ok {
				t.Errorf("vm %q lacks its own status — VM health must stay on the VM resource", r.ExternalURN)
			}
		}
	}
	if vms == 0 {
		t.Error("standalone walk yielded no qemu resources — guest sections must still be traversed")
	}
}

// 페이지 상한 — 어떤 페이지도 pageSize를 초과하지 않고, 재조립하면 시드 집합과
// 정확히 일치한다(하네스 단얫 1의 명시 재단얫 — pageSize 2로 강제).
func TestProxmoxDiscoverPageSizeBound(t *testing.T) {
	mock := newDiscoveryMock(t)
	adapter := NewAdapter(WithPageSize(2))
	f := &discoveryFixture{t: t, mock: mock, adapter: adapter, endpoint: mock.srv.URL}

	ctx := context.Background()
	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		page, err := adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Cursor: cursor, Connection: f.Connection()})
		if err != nil {
			t.Fatalf("Discover(cursor=%q): %v", cursor, err)
		}
		pages++
		if len(page.Resources) > 2 {
			t.Fatalf("page %d carries %d resources, exceeds the forced bound 2", pages, len(page.Resources))
		}
		for _, r := range page.Resources {
			if seen[r.ExternalURN] {
				t.Fatalf("duplicate URN %q across pages", r.ExternalURN)
			}
			seen[r.ExternalURN] = true
		}
		if page.NextCursor == "" {
			break
		}
		if pages > 64 {
			t.Fatal("discovery did not terminate")
		}
		cursor = page.NextCursor
	}
	if len(seen) != len(f.Seed()) {
		t.Fatalf("reassembly total = %d, want %d", len(seen), len(f.Seed()))
	}
}

// 커서 변형 거부 — 알 수 없는 섹션·음수 인덱스·형식 파손.
func TestProxmoxDiscoverRejectsMalformedCursor(t *testing.T) {
	f := newDiscoveryFixture(t)
	for name, cursor := range map[string]string{
		"unknown section": "volumes|pve1|0",
		"no node part":    "qemu",
		"no index":        "qemu|pve1",
		"bad index":       "qemu|pve1|x",
		"negative index":  "qemu|pve1|-1",
	} {
		_, err := f.adapter.Discover(context.Background(), contract.DiscoverRequest{ContextID: 1, Cursor: cursor, Connection: f.Connection()})
		if err == nil {
			t.Errorf("%s: Discover(cursor=%q) succeeded, want rejection", name, cursor)
			continue
		}
		assertNoTokenMaterial(t, err)
	}
}

// 자격 물질 경로(J2 — 어댑터는 req.Connection만 본다): Material 부재·불량·
// 빈 성분은 명시적 오류다(§3.1 "missing credential material" 문언 승계).
func TestProxmoxDiscoverCredentialPaths(t *testing.T) {
	f := newDiscoveryFixture(t)
	ctx := context.Background()

	t.Run("missing_material", func(t *testing.T) {
		_, err := f.adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1})
		if err == nil || !strings.Contains(err.Error(), "missing credential material") {
			t.Fatalf("Discover without Material[inventory]: err = %v", err)
		}
		assertNoTokenMaterial(t, err)
	})

	t.Run("malformed_material", func(t *testing.T) {
		conn := contract.ConnectionView{Material: map[string]string{"inventory": "plain-blob"}}
		if _, err := f.adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Connection: conn}); err == nil {
			t.Fatal("Discover with malformed credential blob succeeded")
		}
	})

	t.Run("incomplete_material", func(t *testing.T) {
		conn := contract.ConnectionView{Material: map[string]string{"inventory": `{"tokenUser":"u","tokenID":"t"}`}}
		if _, err := f.adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Connection: conn}); err == nil {
			t.Fatal("Discover with an incomplete credential blob succeeded")
		}
	})
}

// claim 13 보강 승계: 디스커버리 경로의 모든 에러 문자열에 토큰 헤더 접두와
// 자격 성분 원문이 부재함을 단얫한다.
func TestProxmoxDiscoveryErrorsNeverCarryTokenMaterial(t *testing.T) {
	scenarios := []struct {
		name  string
		exite func(t *testing.T, f *discoveryFixture) error
	}{
		{"read 5xx", func(t *testing.T, f *discoveryFixture) error {
			f.mock.setMode("fail500")
			_, err := f.adapter.Discover(context.Background(), contract.DiscoverRequest{ContextID: 1, Connection: f.Connection()})
			return err
		}},
		{"read 403", func(t *testing.T, f *discoveryFixture) error {
			f.mock.setMode("fail403")
			_, err := f.adapter.Discover(context.Background(), contract.DiscoverRequest{ContextID: 1, Connection: f.Connection()})
			return err
		}},
		{"read 429", func(t *testing.T, f *discoveryFixture) error {
			f.mock.setMode("fail429")
			_, err := f.adapter.Discover(context.Background(), contract.DiscoverRequest{ContextID: 1, Connection: f.Connection()})
			return err
		}},
		{"bad json", func(t *testing.T, f *discoveryFixture) error {
			f.mock.setMode("badjson")
			_, err := f.adapter.Discover(context.Background(), contract.DiscoverRequest{ContextID: 1, Connection: f.Connection()})
			return err
		}},
		{"transport refused", func(t *testing.T, f *discoveryFixture) error {
			f.mock.srv.Close()
			_, err := f.adapter.Discover(context.Background(), contract.DiscoverRequest{ContextID: 1, Connection: f.Connection()})
			return err
		}},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			f := newDiscoveryFixture(t)
			err := sc.exite(t, f)
			if err == nil {
				t.Fatalf("%s: expected an error", sc.name)
			}
			assertNoTokenMaterial(t, err)
			if strings.Contains(fmt.Sprint(err), testTokenSecret) {
				t.Errorf("error surface carries the mock secret literal: %v", err)
			}
		})
	}
}

// badjson은 비신호 일반 에러로 남는다(어휘 확장 금지 — client_test.go의
// 분류 계약이 어댑터 경로에서도 유지되는지 확인).
func TestProxmoxDiscoverBadJSONStaysPlainError(t *testing.T) {
	f := newDiscoveryFixture(t)
	f.mock.setMode("badjson")
	_, err := f.adapter.Discover(context.Background(), contract.DiscoverRequest{ContextID: 1, Connection: f.Connection()})
	if err == nil {
		t.Fatal("bad json should fail")
	}
	var sig *contract.ProviderSignalError
	if errors.As(err, &sig) {
		t.Fatalf("decode failure must stay a plain error, got signal %q", sig.Kind)
	}
}

// 자격 헤더 — 디스커버리 경로의 모든 요청이 성분 조립 헤더를 실는다(§14.3 계약 2).
func TestProxmoxDiscoverySendsAssembledAuthHeader(t *testing.T) {
	f := newDiscoveryFixture(t)
	walkAll(t, f)
	snap := f.mock.snap()
	if len(snap.authHeaders) == 0 {
		t.Fatal("mock captured no requests")
	}
	for i, h := range snap.authHeaders {
		if h != expectedAuthHeader {
			t.Fatalf("request %d Authorization mismatch", i)
		}
	}
}

// --- claim 17 형식: mapping.md ↔ normalizer 동치 (p4 11필드 처분 완결 준용) ---
//
// normalizer가 해독하는 모든 PVE 응답 필드가 mapping.md 커버리지 표에
// mapped/dropped(reason)로 처분되어 있어야 한다. PVE는 zero prior code라
// legacy 집합은 스펙상 공집합(§25) — 커버리지 대상은 어댑터가 실제 디코드하는
// 필드 집합이며, 이 표가 그 집합의 유일한 열거다. 구조체에 필드를 추가하면서
// 이 표와 mapping.md를 갱신하지 않으면 이 단얫이 발화한다.

func mappingDoc(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("mapping.md")
	if err != nil {
		t.Fatalf("read mapping.md: %v", err)
	}
	return string(b)
}

// proxmoxFieldNames — 구조체의 json 필드명 전수(mapping.md 커버리지 표의 와이어
// 형태 — k8s mapping_test.go legacyFieldNames 승계).
func proxmoxFieldNames(t *testing.T, item any) []string {
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

// mappingCoverageTable — 정규화기가 디코드하는 필드 전수의 처분
// ("<섹션>.<와이어필드>" → 처분 등급). 키 집합은 정규화기 디코드 구조체
// (clusterStatusEntry·guestItem·storageItem·networkItem)의 json 태그와 기계 동치.
var mappingCoverageTable = map[string]string{
	// /cluster/status node행 → compute.hypervisor_node
	"status.type":   "dropped(dispatch-only)",
	"status.name":   "mapped",
	"status.ip":     "mapped",
	"status.online": "mapped",
	// qemu·lxc 공용 게스트 행 → compute.vm / compute.system_container
	"guest.vmid":    "mapped",
	"guest.name":    "mapped",
	"guest.status":  "mapped",
	"guest.template": "mapped",
	"guest.maxmem":  "mapped",
	"guest.maxcpu":  "mapped",
	"guest.maxdisk": "mapped",
	"guest.tags":    "mapped",
	"guest.uptime":  "mapped",
	// storage 행 → storage.pool
	"storage.storage": "mapped",
	"storage.type":    "mapped",
	"storage.content": "mapped",
	"storage.status":  "mapped",
	"storage.active":  "mapped",
	"storage.shared":  "mapped",
	"storage.total":   "mapped",
	"storage.used":    "mapped",
	"storage.avail":   "mapped",
	// network 행 → network.interface
	"network.iface":       "mapped",
	"network.type":        "mapped",
	"network.active":      "mapped",
	"network.autostart":   "mapped",
	"network.vlan-aware":  "mapped",
	"network.ports":       "mapped",
	"network.slaves":      "mapped",
	"network.address":     "mapped",
	"network.gateway":     "mapped",
}

// TestProxmoxMappingCoverage — claim 17: 처분 완결 + zero-prior-code 헤더 +
// kind/Subtype 표 정합.
func TestProxmoxMappingCoverage(t *testing.T) {
	doc := mappingDoc(t)

	// 헤더 계약(N5): legacy 비교 집합이 스펙상 공집합임을 명시한다.
	for _, token := range []string{"zero prior code", "legacy comparison set is empty"} {
		if !strings.Contains(doc, token) {
			t.Errorf("mapping.md lacks the mandated header token %q", token)
		}
	}
	// kind/Subtype 표(J3)와 URN 규칙 문언.
	for _, token := range []string{
		"compute.hypervisor_node", "compute.vm", "qemu", "compute.system_container", "lxc",
		"storage.pool", "network.interface", "urn:proxmox",
	} {
		if !strings.Contains(doc, token) {
			t.Errorf("mapping.md lacks kind/Subtype table token %q", token)
		}
	}

	sections := []struct {
		path string
		item any
	}{
		{"status.", clusterStatusEntry{}},
		{"guest.", guestItem{}},
		{"storage.", storageItem{}},
		{"network.", networkItem{}},
	}
	for _, section := range sections {
		for _, field := range proxmoxFieldNames(t, section.item) {
			key := section.path + field
			disposition, ok := mappingCoverageTable[key]
			if !ok {
				t.Errorf("decoded field %s is not classified in the mapping coverage table", key)
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

	// 표와 구조체의 역방향 정합 — 표에만 있고 구조체가 디코드하지 않는 키는
	// 문서 부패다(정규화기가 실제로 읽는 집합 = 키 집합).
	decoded := map[string]bool{}
	for _, section := range sections {
		for _, field := range proxmoxFieldNames(t, section.item) {
			decoded[section.path+field] = true
		}
	}
	for key := range mappingCoverageTable {
		if !decoded[key] {
			t.Errorf("coverage table key %s names no decoded field", key)
		}
	}
}

// Raw 바운드 — Raw가 MaxRawBytes를 초과하면 절단 보존 + normalized["truncated"]
// 스탬프(p4 A12 승계 — 정규화 경로의 계약을 직접 단얫).
func TestProxmoxRawLimitTruncation(t *testing.T) {
	huge := make([]byte, MaxRawBytes+16)
	for i := range huge {
		huge[i] = 'a'
	}
	entry := clusterStatusEntry{Type: "node", Name: "pve-big", IP: string(huge), Online: 1}
	res, err := normalizeHypervisorNode(1, entry)
	if err != nil {
		t.Fatalf("normalizeHypervisorNode: %v", err)
	}
	blob, err := json.Marshal(res.Raw)
	if err != nil {
		t.Fatalf("marshal raw: %v", err)
	}
	if len(blob) > MaxRawBytes {
		t.Fatalf("raw size %d exceeds the %d bound", len(blob), MaxRawBytes)
	}
	if res.Normalized["truncated"] != true {
		t.Fatalf("truncation stamp missing: %v", res.Normalized["truncated"])
	}
}
