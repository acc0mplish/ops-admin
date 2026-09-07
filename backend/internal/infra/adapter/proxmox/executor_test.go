package proxmox

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"ops-admin/backend/internal/infra/contract"
)

// executor_test.go — Phase D(N7) executor 하니스. client_test.go(Phase B 소유)의
// 자격 리터럴·envelope·assertNoTokenMaterial 관례와 discovery_mock_test.go의
// 소유 경계 분리 관례(client_test.go 무수정 — 배치표 경계)를 승계해, executor
// 표면(status mutation 3경로·snapshot·config·task status 재폴)을 이 파일 전용
// 모의 서버로 재현한다. 모의 응답 형상은 PVE API2 공개 문서 기반(가정 A2).

// 실행 타깃 상수 — URN 형상은 normalizer의 urnFor 산출물과 동일하다(§8.1).
const (
	execConnUID = "conn-pve-1"
	execVMURN   = "urn:proxmox:1:vm:pve1/100"
	execCTURN   = "urn:proxmox:1:system_container:pve1/110"
)

// executorMock — executor 표면 전용 모의 PVE. mutMode는 mutation 3경로
// (status|snapshot|config)의 응답 분기, taskStatus/exitStatus는 재폴 응답 행이다.
type executorMock struct {
	t   *testing.T
	srv *httptest.Server

	mu          sync.Mutex
	mutMode     string // "" upid | sync_null | error5xx
	taskStatus  string // PVE /tasks/{upid}/status 행의 status 성분
	exitStatus  string
	authHeaders []string
	mutMethod   []string
	mutPaths    []string
	mutForms    []url.Values
	taskPaths   []string
}

func newExecutorMock(t *testing.T) *executorMock {
	t.Helper()
	m := &executorMock{t: t, taskStatus: "stopped", exitStatus: "OK"}
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/", m.serveAPI)
	m.srv = httptest.NewServer(mux)
	t.Cleanup(m.srv.Close)
	return m
}

func (m *executorMock) setMutMode(mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mutMode = mode
}

func (m *executorMock) setTaskStatus(status, exitStatus string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taskStatus = status
	m.exitStatus = exitStatus
}

// snapshot — 캡처 상태의 일관 읽기(-race 안전).
type executorSnapshot struct {
	authHeaders []string
	mutMethod   []string
	mutPaths    []string
	mutForms    []url.Values
	taskPaths   []string
}

func (m *executorMock) snap() executorSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return executorSnapshot{
		authHeaders: append([]string(nil), m.authHeaders...),
		mutMethod:   append([]string(nil), m.mutMethod...),
		mutPaths:    append([]string(nil), m.mutPaths...),
		mutForms:    append([]url.Values(nil), m.mutForms...),
		taskPaths:   append([]string(nil), m.taskPaths...),
	}
}

func (m *executorMock) serveAPI(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	auth := r.Header.Get("Authorization")
	m.authHeaders = append(m.authHeaders, auth)
	m.mu.Unlock()

	rest := strings.TrimPrefix(r.URL.Path, "/api2/json/nodes/")
	segs := strings.Split(rest, "/")

	// task status 재폴 — GET /nodes/{node}/tasks/{upid}/status
	// (UPID의 콜론은 경로 성분에 그대로 흐른다 — PVE 네이티브 형상).
	if r.Method == http.MethodGet && len(segs) == 4 && segs[1] == "tasks" && segs[3] == "status" {
		m.mu.Lock()
		m.taskPaths = append(m.taskPaths, r.URL.Path)
		status, exit := m.taskStatus, m.exitStatus
		m.mu.Unlock()
		envelope(w, map[string]any{"status": status, "exitstatus": exit})
		return
	}

	// guest mutation — power(POST …/status/{action})·snapshot(POST …/snapshot)·
	// config(PUT …/config). 3경로 모두 폼을 캡처한다(background_delay·payload
	// 파라미터 단얫 재료).
	isPower := r.Method == http.MethodPost && len(segs) == 5 && segs[3] == "status"
	isSnapshot := r.Method == http.MethodPost && len(segs) == 4 && segs[3] == "snapshot"
	isConfig := r.Method == http.MethodPut && len(segs) == 4 && segs[3] == "config"
	if isPower || isSnapshot || isConfig {
		if err := r.ParseForm(); err != nil {
			http.Error(w, `{"data":null}`, http.StatusBadRequest)
			return
		}
		m.mu.Lock()
		m.mutMethod = append(m.mutMethod, r.Method)
		m.mutPaths = append(m.mutPaths, r.URL.Path)
		m.mutForms = append(m.mutForms, r.PostForm)
		mode := m.mutMode
		m.mu.Unlock()
		switch mode {
		case "sync_null":
			envelope(w, nil)
		case "error5xx":
			http.Error(w, `{"data":null}`, http.StatusInternalServerError)
		default:
			envelope(w, testUPID)
		}
		return
	}

	http.Error(w, `{"data":null}`, http.StatusNotFound)
}

// --- Fixture — adapter·모의 서버·실행 자격(Material["operations"])의 결합. ---

type executorFixture struct {
	t       *testing.T
	mock    *executorMock
	adapter *Adapter
}

func newExecutorFixture(t *testing.T) *executorFixture {
	t.Helper()
	mock := newExecutorMock(t)
	return &executorFixture{t: t, mock: mock, adapter: NewAdapter()}
}

func (f *executorFixture) opsConnection() contract.ConnectionView {
	return contract.ConnectionView{
		UID:          execConnUID,
		ProviderType: ProviderName,
		Endpoint:     f.mock.srv.URL,
		Material:     map[string]string{contract.CredentialPurposeOperations: testCredentialMaterial},
	}
}

// execute — 단일 Execute 호출 헬퍼.
func (f *executorFixture) execute(op string, urn string, payload contract.JSONMap) (contract.OperationHandle, error) {
	f.t.Helper()
	return f.adapter.Execute(context.Background(), contract.OperationRequest{
		OperationName: op,
		ResourceURN:   urn,
		Payload:       payload,
		Connection:    f.opsConnection(),
	})
}

// poll — 단일 Poll 호출 헬퍼.
func (f *executorFixture) poll(handle contract.OperationHandle, conn contract.ConnectionView) (contract.OperationStatus, error) {
	f.t.Helper()
	return f.adapter.Poll(context.Background(), contract.PollRequest{Handle: handle, Connection: conn})
}

// --- 자격 분리 — J5(§14.3 "operations token gated by approval"). ---

// TestExecuteRejectsInventoryCredentials — 실행 경로는 Material["operations"]만
// 수용한다. inventory 바인딩 재질로 Execute를 호출하면 와이어 요청 0회로 즉시
// 거부한다(자격 분리 단얫 — N7).
func TestExecuteRejectsInventoryCredentials(t *testing.T) {
	f := newExecutorFixture(t)

	conn := f.opsConnection()
	conn.Material = map[string]string{contract.CredentialPurposeInventory: testCredentialMaterial}
	if _, err := f.adapter.Execute(context.Background(), contract.OperationRequest{
		OperationName: PowerOperationName,
		ResourceURN:   execVMURN,
		Payload:       contract.JSONMap{"action": "start"},
		Connection:    conn,
	}); err == nil {
		t.Fatal("Execute accepted inventory-bound credentials, want the credential-separation refusal (J5)")
	} else if !strings.Contains(err.Error(), "operations") {
		t.Errorf("refusal = %q, want a mention of the operations material slot", err)
	}

	// 재질 슬롯 자체가 비어 있어도 같은 거부다.
	conn.Material = nil
	if _, err := f.adapter.Execute(context.Background(), contract.OperationRequest{
		OperationName: PowerOperationName,
		ResourceURN:   execVMURN,
		Payload:       contract.JSONMap{"action": "start"},
		Connection:    conn,
	}); err == nil {
		t.Fatal("Execute accepted a connection with no material at all")
	} else {
		assertNoTokenMaterial(t, err)
	}

	if snap := f.mock.snap(); len(snap.mutPaths) != 0 {
		t.Errorf("rejections reached the wire: %v", snap.mutPaths)
	}
}

// TestExecuteSelectsOperationsMaterial — inventory 슬롯에 무관 재질이 있어도
// operations 슬롯의 재질로 조립한다. inventory 값을 해석 불능 블롭으로 오염시켜,
// 성공이 곧 "operations를 읽었다"는 증거가 되게 한다(선택 단얫).
func TestExecuteSelectsOperationsMaterial(t *testing.T) {
	f := newExecutorFixture(t)
	f.mock.setMutMode("sync_null")

	conn := f.opsConnection()
	conn.Material[contract.CredentialPurposeInventory] = "not-a-pve-credential-blob"
	if _, err := f.adapter.Execute(context.Background(), contract.OperationRequest{
		OperationName: PowerOperationName,
		ResourceURN:   execVMURN,
		Payload:       contract.JSONMap{"action": "start"},
		Connection:    conn,
	}); err != nil {
		t.Fatalf("Execute with a poisoned inventory slot: %v", err)
	}
}

// --- UPID dual-mode 3분기 — 판정 J4(N7의 3경우). ---

// TestExecuteSyncNullSingleAttempt — 1분기: mutation 응답 data == null → 빈
// ProviderRef. 엔진 executeClaimed는 빈 핸들을 단일 attempt 성공으로 종단한다
// (§0.3 실측 — 엔진 무수정이 이 경로의 전제). background_delay 명시 첨부와
// 자격 헤더 조립을 함께 단얫한다.
func TestExecuteSyncNullSingleAttempt(t *testing.T) {
	f := newExecutorFixture(t)
	f.mock.setMutMode("sync_null")

	handle, err := f.execute(PowerOperationName, execVMURN, contract.JSONMap{"action": "start"})
	if err != nil {
		t.Fatalf("Execute(sync null): %v", err)
	}
	if handle.ProviderRef != "" {
		t.Errorf("ProviderRef = %q, want the empty synchronous-completion handle (J4 1분기)", handle.ProviderRef)
	}

	snap := f.mock.snap()
	if len(snap.mutPaths) != 1 || !strings.HasSuffix(snap.mutPaths[0], "/nodes/pve1/qemu/100/status/start") {
		t.Fatalf("mutation paths = %v, want exactly /nodes/pve1/qemu/100/status/start", snap.mutPaths)
	}
	if snap.mutMethod[0] != http.MethodPost {
		t.Errorf("mutation method = %q, want POST", snap.mutMethod[0])
	}
	if snap.mutForms[0].Get("background_delay") == "" {
		t.Error("mutation form carries no background_delay (J4 — 명시 첨부)")
	}
	for _, h := range snap.authHeaders {
		if h != expectedAuthHeader {
			t.Errorf("auth header = %q, want the operations-material assembly %q", h, expectedAuthHeader)
		}
	}
}

// TestExecuteUPIDThenPollConverges — 2분기: data == "UPID:…" → 핸들 인코딩
// `upid|<connUID>|<node>|<rawUPID>` → Poll(running) → Poll(stopped OK) 수렴.
func TestExecuteUPIDThenPollConverges(t *testing.T) {
	f := newExecutorFixture(t)
	f.mock.setTaskStatus("running", "")

	handle, err := f.execute(PowerOperationName, execVMURN, contract.JSONMap{"action": "start"})
	if err != nil {
		t.Fatalf("Execute(upid): %v", err)
	}
	if want := "upid|" + execConnUID + "|pve1|" + testUPID; handle.ProviderRef != want {
		t.Fatalf("ProviderRef = %q, want %q", handle.ProviderRef, want)
	}

	// 폴 1 — running → 다음 폴 사이클.
	st, err := f.poll(handle, f.opsConnection())
	if err != nil {
		t.Fatalf("Poll(running): %v", err)
	}
	if st.State != contract.OperationStateRunning {
		t.Errorf("Poll state = %q, want running", st.State)
	}

	// 폴 2 — stopped OK → Succeeded.
	f.mock.setTaskStatus("stopped", "OK")
	st, err = f.poll(handle, f.opsConnection())
	if err != nil {
		t.Fatalf("Poll(stopped OK): %v", err)
	}
	if st.State != contract.OperationStateSucceeded {
		t.Errorf("Poll state = %q, want succeeded", st.State)
	}

	// detail은 §3.2 redaction 허용 필드의 부분집합이다(stateless 핸들 계약 —
	// vmid·action·snapname은 폴 시점에 재현 불가). 키 전수 단얫.
	allowed := []string{"node", "vmid", "guestType", "action", "snapname", "upid", "exitStatus", "cores", "memoryMB"}
	for key := range st.Detail {
		ok := false
		for _, a := range allowed {
			if key == a {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("poll detail key %q is outside the §3.2 redaction allowlist", key)
		}
	}
	if st.Detail["node"] != "pve1" || st.Detail["upid"] != testUPID || st.Detail["exitStatus"] != "OK" || st.Detail["guestType"] != "qemu" {
		t.Errorf("poll detail = %v, want node=pve1 upid=<raw> exitStatus=OK guestType=qemu", st.Detail)
	}

	snap := f.mock.snap()
	if len(snap.taskPaths) != 2 {
		t.Fatalf("task paths = %v, want exactly 2 polls", snap.taskPaths)
	}
	if want := "/api2/json/nodes/pve1/tasks/" + testUPID + "/status"; snap.taskPaths[0] != want {
		t.Errorf("task path = %q, want %q (UPID node 라우팅 — 증류 계약 1)", snap.taskPaths[0], want)
	}
}

// TestExecuteSyncErrorFails — 3분기: 동기 에러(5xx)는 error 반환 — 엔진
// executor_error/operation_failed 분기(J4). 에러 체인은 신호 어휘(unreachable)로
// 분류되고 토큰 물질이 없다.
func TestExecuteSyncErrorFails(t *testing.T) {
	f := newExecutorFixture(t)
	f.mock.setMutMode("error5xx")

	if _, err := f.execute(PowerOperationName, execVMURN, contract.JSONMap{"action": "start"}); err == nil {
		t.Fatal("Execute succeeded on a 5xx mutation, want the sync-error branch (J4)")
	} else {
		var sig *contract.ProviderSignalError
		if !errors.As(err, &sig) || sig.Kind != contract.SignalUnreachable {
			t.Errorf("error = %v, want a provider signal (unreachable)", err)
		}
		assertNoTokenMaterial(t, err)
	}
}

// TestPollRejectsForeignConnection — 정합 가드(J12 k8s Poll 승계): 핸들이
// 자기서술하는 connUID와 폴 커넥션이 다르면 조립 버그다 — 다른 커넥션을 폴하는
// 오발사를 늦은 수렴 오탐보다 빨리 잡는다.
func TestPollRejectsForeignConnection(t *testing.T) {
	f := newExecutorFixture(t)

	handle, err := f.execute(PowerOperationName, execVMURN, contract.JSONMap{"action": "start"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	conn := f.opsConnection()
	conn.UID = "conn-pve-other"
	if _, err := f.poll(handle, conn); err == nil || !strings.Contains(err.Error(), execConnUID) {
		t.Errorf("Poll with a foreign connection = (%v), want a UID mismatch naming %q", err, execConnUID)
	}
}

// TestPollUnrecognizedTaskStatus — PVE task status 어휘(running|stopped) 밖의
// 값은 에러다 — 묵시적 Running 전이는 폴 수렴의 오탐 재료다.
func TestPollUnrecognizedTaskStatus(t *testing.T) {
	f := newExecutorFixture(t)

	handle, err := f.execute(PowerOperationName, execVMURN, contract.JSONMap{"action": "start"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	f.mock.setTaskStatus("zombie", "")
	if _, err := f.poll(handle, f.opsConnection()); err == nil {
		t.Fatal("Poll accepted an unrecognized task status")
	}
}

// --- payload 검증 — J7 화이트리스트(E-5). 검증 실패는 와이어 0회의 즉시 에러다. ---

// TestExecutePowerActionsRouteToProviderPaths — 4 액션의 경로 분기(J7)와 컨테이너
// (lxc) 경로. 알 수 없는 액션은 거부한다.
func TestExecutePowerActionsRouteToProviderPaths(t *testing.T) {
	f := newExecutorFixture(t)
	f.mock.setMutMode("sync_null")

	for _, action := range []string{"start", "shutdown", "stop", "reboot"} {
		if _, err := f.execute(PowerOperationName, execVMURN, contract.JSONMap{"action": action}); err != nil {
			t.Fatalf("Execute(power %s): %v", action, err)
		}
	}
	if _, err := f.execute(PowerOperationName, execCTURN, contract.JSONMap{"action": "stop"}); err != nil {
		t.Fatalf("Execute(lxc stop): %v", err)
	}

	snap := f.mock.snap()
	wantPaths := []string{
		"/api2/json/nodes/pve1/qemu/100/status/start",
		"/api2/json/nodes/pve1/qemu/100/status/shutdown",
		"/api2/json/nodes/pve1/qemu/100/status/stop",
		"/api2/json/nodes/pve1/qemu/100/status/reboot",
		"/api2/json/nodes/pve1/lxc/110/status/stop",
	}
	if len(snap.mutPaths) != len(wantPaths) {
		t.Fatalf("mutation paths = %v, want %v", snap.mutPaths, wantPaths)
	}
	for i, want := range wantPaths {
		if snap.mutPaths[i] != want {
			t.Errorf("mutation path[%d] = %q, want %q", i, snap.mutPaths[i], want)
		}
	}

	// 어휘 밖 액션 — 와이어 없이 거부.
	if _, err := f.execute(PowerOperationName, execVMURN, contract.JSONMap{"action": "destroy"}); err == nil {
		t.Error("Execute accepted a power action outside the guarded vocabulary")
	}
	if snap := f.mock.snap(); len(snap.mutPaths) != len(wantPaths) {
		t.Errorf("rejected action reached the wire: %v", snap.mutPaths)
	}
}

// TestExecuteSnapshotPayload — snapname 필수·비공백 문자열·경로·폼 실림.
func TestExecuteSnapshotPayload(t *testing.T) {
	f := newExecutorFixture(t)
	f.mock.setMutMode("sync_null")

	if _, err := f.execute(SnapshotOperationName, execVMURN, contract.JSONMap{"snapname": "pre-upgrade"}); err != nil {
		t.Fatalf("Execute(snapshot): %v", err)
	}
	snap := f.mock.snap()
	if !strings.HasSuffix(snap.mutPaths[0], "/nodes/pve1/qemu/100/snapshot") {
		t.Errorf("snapshot path = %q, want …/snapshot", snap.mutPaths[0])
	}
	if snap.mutForms[0].Get("snapname") != "pre-upgrade" {
		t.Errorf("snapshot form snapname = %q, want pre-upgrade", snap.mutForms[0].Get("snapname"))
	}

	for name, payload := range map[string]contract.JSONMap{
		"missing":  {},
		"empty":    {"snapname": ""},
		"nonstr":   {"snapname": 42},
		"extrakey": {"snapname": "ok", "mode": "fancy"},
	} {
		if _, err := f.execute(SnapshotOperationName, execVMURN, payload); err == nil {
			t.Errorf("Execute(snapshot %s payload) = nil error, want refusal", name)
		}
	}
	if snap := f.mock.snap(); len(snap.mutPaths) != 1 {
		t.Errorf("rejected snapshot payloads reached the wire: %v", snap.mutPaths)
	}
}

// TestExecuteConfigWhitelist — E-5: 화이트리스트 외 키·값 거부. PVE PUT config
// 파라미터 명칭(memory — MiB 정수)은 A6 확정분으로 폼에 번역된다.
func TestExecuteConfigWhitelist(t *testing.T) {
	f := newExecutorFixture(t)
	f.mock.setMutMode("sync_null")

	// cores 1건 — PUT …/config, form cores=<n>.
	if _, err := f.execute(ConfigOperationName, execVMURN, contract.JSONMap{"cores": 4}); err != nil {
		t.Fatalf("Execute(config cores): %v", err)
	}
	// memoryMB 1건 — PVE 파라미터 memory로 번역(A6). 엔진 디코드 경로의
	// float64 성분도 수용한다.
	if _, err := f.execute(ConfigOperationName, execVMURN, contract.JSONMap{"memoryMB": float64(2048)}); err != nil {
		t.Fatalf("Execute(config memoryMB): %v", err)
	}
	// 병기.
	if _, err := f.execute(ConfigOperationName, execVMURN, contract.JSONMap{"cores": 2, "memoryMB": 1024}); err != nil {
		t.Fatalf("Execute(config both): %v", err)
	}

	snap := f.mock.snap()
	if len(snap.mutPaths) != 3 {
		t.Fatalf("mutation paths = %v, want 3 config PUTs", snap.mutPaths)
	}
	for i := range snap.mutForms {
		if snap.mutMethod[i] != http.MethodPut {
			t.Errorf("config method[%d] = %q, want PUT", i, snap.mutMethod[i])
		}
		if !strings.HasSuffix(snap.mutPaths[i], "/nodes/pve1/qemu/100/config") {
			t.Errorf("config path[%d] = %q, want …/config", i, snap.mutPaths[i])
		}
	}
	if snap.mutForms[0].Get("cores") != "4" {
		t.Errorf("cores form = %q, want 4", snap.mutForms[0].Get("cores"))
	}
	if snap.mutForms[1].Get("memory") != "2048" {
		t.Errorf("memory form = %q, want 2048 (memoryMB → memory 번역 — A6)", snap.mutForms[1].Get("memory"))
	}
	if snap.mutForms[2].Get("cores") != "2" || snap.mutForms[2].Get("memory") != "1024" {
		t.Errorf("combined form = %v, want cores=2 memory=1024", snap.mutForms[2])
	}

	// 거부 면 — 키 밖·음수·비정수·문자열·빈 payload. 전부 와이어 0회.
	rejected := map[string]contract.JSONMap{
		"unknownkey": {"cores": 4, "diskGB": 10},
		"zero":       {"cores": 0},
		"negative":   {"memoryMB": -512},
		"fraction":   {"cores": 1.5},
		"string":     {"cores": "four"},
		"empty":      {},
	}
	before := len(f.mock.snap().mutPaths)
	for name, payload := range rejected {
		if _, err := f.execute(ConfigOperationName, execVMURN, payload); err == nil {
			t.Errorf("Execute(config %s payload) = nil error, want refusal (E-5)", name)
		}
	}
	if after := len(f.mock.snap().mutPaths); after != before {
		t.Errorf("rejected config payloads reached the wire: %d → %d", before, after)
	}
}

// --- URN 방어 — r1.4 HIGH-3(조립 직전 성분 검증). ---

// TestExecuteGuardsTamperedURNs — vmid는 숫자 전체, node는 [a-zA-Z0-9_-]+ 검증
// 통과 전엔 경로가 조립되지 않는다. 경로 순회·인접 리소스 침범·쿼리 오염의
// 조기 차단.
func TestExecuteGuardsTamperedURNs(t *testing.T) {
	f := newExecutorFixture(t)

	tampered := []string{
		"urn:proxmox:1:vm:pve1/100abc",   // vmid 비숫자
		"urn:proxmox:1:vm:pve1/10;0",     // vmid 특수문자
		"urn:proxmox:1:vm:pv e1/100",     // node 공백
		"urn:proxmox:1:vm:pve/1/100",     // node에 슬래시
		"urn:proxmox:1:vm:%2e%2e/100",    // node 퍼센트 인코딩
		"urn:proxmox:1:pool:pve1/local",  // 게스트 외 kind — 이 executor의 대상 아님
		"urn:k8s:1:workload:ns/deploy/x", // 비 PVE 접두
		"urn:proxmox:x:vm:pve1/100",      // context id 비숫자
		"urn:proxmox:1:vm:noslash",       // vmid 성분 부재
	}
	for _, urn := range tampered {
		if _, err := f.execute(PowerOperationName, urn, contract.JSONMap{"action": "start"}); err == nil {
			t.Errorf("Execute accepted tampered URN %q", urn)
		} else {
			assertNoTokenMaterial(t, err)
		}
	}
	if snap := f.mock.snap(); len(snap.mutPaths) != 0 {
		t.Errorf("tampered URNs reached the wire: %v", snap.mutPaths)
	}
}

// TestExecuteRejectsUnknownOperation — 본 executor의 오퍬레이션 면은 3종뿐이다
// (J7 — 유일 수용). 타 어댑터의 오퍬레이션 이름도 같은 거부다.
func TestExecuteRejectsUnknownOperation(t *testing.T) {
	f := newExecutorFixture(t)

	for _, op := range []string{"pve.guest.nuke", "k8s.workload.restart", ""} {
		if _, err := f.execute(op, execVMURN, contract.JSONMap{"action": "start"}); err == nil {
			t.Errorf("Execute accepted unknown operation %q", op)
		} else {
			assertNoTokenMaterial(t, err)
		}
	}
	if snap := f.mock.snap(); len(snap.mutPaths) != 0 {
		t.Errorf("unknown operations reached the wire: %v", snap.mutPaths)
	}
}

// --- 핸들 위조 — decodeUPIDRef 승계 + 폴 경로 성분 방어. ---

// TestPollGuardsForgedHandles — decodeUPIDRef의 형식·정합 검증을 승계하고, 정규형
// UPID라도 URL 활성 문자를 성분에 숨기면 폴 경로 조립을 거부한다(경로→쿼리 오염
// 차단 — HIGH-3 계열).
func TestPollGuardsForgedHandles(t *testing.T) {
	f := newExecutorFixture(t)

	forged := []string{
		"rollout|conn|ns|deployment|web|3", // 타 어댑터 형식
		"upid|conn|pve1|not-a-upid",        // UPID 형식 불일치
		"upid|conn|pve2|" + testUPID,       // node 세그먼트·원문 불일치
		"upid|conn|pve1|UPID:pve1:00123456:000A1B2C:68BADBA0:qemu:start:root@pam?x=1:", // 쿼리 오염 성분
	}
	for _, ref := range forged {
		if _, err := f.poll(contract.OperationHandle{ProviderRef: ref}, f.opsConnection()); err == nil {
			t.Errorf("Poll accepted forged handle %q", ref)
		} else {
			assertNoTokenMaterial(t, err)
		}
	}
	if snap := f.mock.snap(); len(snap.taskPaths) != 0 {
		t.Errorf("forged handles reached the wire: %v", snap.taskPaths)
	}
}

// TestPollMissingOperationsMaterial — 폴 자격도 같은 슬롯 규칙을 따른다(J12 —
// 엔진이 매 폴 조립). 재질 부재는 와이어 0회의 에러다.
func TestPollMissingOperationsMaterial(t *testing.T) {
	f := newExecutorFixture(t)

	handle, err := f.execute(PowerOperationName, execVMURN, contract.JSONMap{"action": "start"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	conn := f.opsConnection()
	conn.Material = nil
	if _, err := f.poll(handle, conn); err == nil {
		t.Fatal("Poll accepted a connection with no operations material")
	}
	if snap := f.mock.snap(); len(snap.taskPaths) != 0 {
		t.Errorf("credential-less poll reached the wire: %v", snap.taskPaths)
	}
}

// TestExecuteToleratesEngineEnvelopeKeys — 엔진이 Payload에 동봉하는 부기키
// (restartedAt J1 동결·resourceRevision J7 감사)는 오퍼레이션 화이트리스트의
// 검증 대상이 아니다(k8s executor 선례 — restartedAt 소비·resourceRevision
// 무시). 실엔드포인트 증명(§13-9, 2026-09-08)에서 이중 중첩이 아닌 평형
// 페이로드조차 부기키로 거부되던 결함의 회귀 단얫.
func TestExecuteToleratesEngineEnvelopeKeys(t *testing.T) {
	f := newExecutorFixture(t) // 기본 mutMode = UPID 반환 — 검증 통과 자체가 단얫

	envelope := contract.JSONMap{
		"action":           "start",
		"restartedAt":      "2026-09-08T07:48:51Z",
		"resourceRevision": "gen-42",
	}
	if _, err := f.execute(PowerOperationName, execVMURN, envelope); err != nil {
		t.Fatalf("power payload with engine envelope keys must pass whitelist: %v", err)
	}

	if _, err := f.execute(SnapshotOperationName, execVMURN, contract.JSONMap{
		"snapname":         "pre-upgrade",
		"restartedAt":      "2026-09-08T07:48:51Z",
		"resourceRevision": "gen-42",
	}); err != nil {
		t.Fatalf("snapshot payload with engine envelope keys must pass whitelist: %v", err)
	}

	if _, err := f.execute(ConfigOperationName, execVMURN, contract.JSONMap{
		"cores":            2,
		"restartedAt":      "2026-09-08T07:48:51Z",
		"resourceRevision": "gen-42",
	}); err != nil {
		t.Fatalf("config payload with engine envelope keys must pass whitelist: %v", err)
	}

	// 부기키 관용이 화이트리스트 자체를 무력화하지 않는다 — 모르는 키는 여전히 거부.
	if _, err := f.execute(PowerOperationName, execVMURN, contract.JSONMap{
		"action":  "start",
		"devices": "usb0",
	}); err == nil {
		t.Fatal("unknown payload key must still be rejected (E-5)")
	}
}
