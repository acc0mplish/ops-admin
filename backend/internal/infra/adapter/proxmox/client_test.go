package proxmox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/metrics"
)

// --- 모의 PVE API2 서버 (계획 판정 J8 — 테스트 하니스 전용) ---
//
// PVE API2 JSON 응답 래핑 {"data": …}(가정 A2)을 재현한다. 요청 캡처로 토큰
// 헤더 조립(Authorization: PVEAPIToken=…)과 mutation 폼의 background_delay
// 첨부(판정 J4)를 단얫하고, 경로별 모드 분기로 페일오버 비대칭 4시나리오
// (계획 §5 Phase B 행·판정 J6(2))의 입력을 만든다.
//
// 모의 자격 리터럴은 테스트 전용 플레이스홀더다(claim 14 형식 승계 — tencent
// adapter_test.go 선례). 실토큰 값이 아님을 값 자체가 증명한다("mock-" 접두 ·
// "not-real" 접미)지만 keyword-entropy 룰은 키명+32자 값 성분으로 발화하므로
// secret-scan allowlist에 등재돼 있다(M11 — d861fa4, proxmox 테스트 패키지 한정).

const testTokenUser = "root@pam"
const testTokenID = "opsadmin-ro"
const testTokenSecret = "mock-pve-token-material-not-real"

// testCredentialMaterial — SecretRef 재질 JSON(판정 J5 — 브로커 Value 단일
// 문자열 계약). 테스트가 Material["inventory"]/["operations"]에 주입하는 형태.
const testCredentialMaterial = `{"tokenUser":"` + testTokenUser + `","tokenID":"` + testTokenID + `","tokenSecret":"` + testTokenSecret + `"}`

// expectedAuthHeader — 클라이언트가 조립해야 하는 헤더 값 그 자체(§14.3 증류
// 계약 2 — 어댑터가 전체 헤더를 저장하지 않고 성분에서 조립한다).
const expectedAuthHeader = "PVEAPIToken=" + testTokenUser + "!" + testTokenID + "=" + testTokenSecret

// testUPID — PVE가 mutation 응답 data로 돌려주는 UPID 원문. 전 성분:
// node=pve1 · pid=00123456 · pstart=000A1B2C · starttime=68BADBA0 ·
// type=qemu · id=start · user=root@pam (판정 J4 파싱 표면).
const testUPID = "UPID:pve1:00123456:000A1B2C:68BADBA0:qemu:start:root@pam:"

// clusterStatusSeed — /cluster/status data 배열. pve3는 offline 멤버지만 ip를
// 보고한다(증류 계약 4 — 오프라인 멤버 IP 후보 보존의 입력).
func clusterStatusSeed() []map[string]any {
	return []map[string]any{
		{"type": "cluster", "name": "testcluster", "quorate": 1, "id": "cluster"},
		{"type": "node", "name": "pve1", "ip": "10.20.0.11", "online": 1, "id": "node/pve1"},
		{"type": "node", "name": "pve2", "ip": "10.20.0.12", "online": 1, "id": "node/pve2"},
		{"type": "node", "name": "pve3", "ip": "10.20.0.13", "online": 0, "id": "node/pve3"},
	}
}

// mockProxmox — httptest 기반 PVE API2 모의 서버. getMode는 GET /nodes의
// 응답 분기, mutMode는 POST mutation의 응답 분기다.
type mockProxmox struct {
	t   *testing.T
	srv *httptest.Server

	mu          sync.Mutex
	getMode     string // "" ok | fail500 | fail401 | fail403 | fail429 | badjson
	mutMode     string // "" upid | sync_null | error5xx | slow
	standalone  bool   // /cluster/status가 자기 노드 1행만 보고
	authHeaders []string
	postForms   []url.Values
	postPaths   []string
	statusCalls int
	nodesCalls  int
}

func newMockProxmox(t *testing.T) *mockProxmox {
	t.Helper()
	m := &mockProxmox{t: t}
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/", m.serveAPI)
	m.srv = httptest.NewServer(mux)
	t.Cleanup(m.srv.Close)
	return m
}

func (m *mockProxmox) setGetMode(mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getMode = mode
}

func (m *mockProxmox) setMutMode(mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mutMode = mode
}

// snapshot — 캡처 상태의 일관 읽기(-race 안전).
type mockSnapshot struct {
	authHeaders []string
	postForms   []url.Values
	postPaths   []string
	statusCalls int
	nodesCalls  int
}

func (m *mockProxmox) snap() mockSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return mockSnapshot{
		authHeaders: append([]string(nil), m.authHeaders...),
		postForms:   append([]url.Values(nil), m.postForms...),
		postPaths:   append([]string(nil), m.postPaths...),
		statusCalls: m.statusCalls,
		nodesCalls:  m.nodesCalls,
	}
}

func envelope(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func (m *mockProxmox) serveAPI(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	auth := r.Header.Get("Authorization")
	m.authHeaders = append(m.authHeaders, auth)
	getMode, mutMode, standalone := m.getMode, m.mutMode, m.standalone
	m.mu.Unlock()

	path := r.URL.Path
	switch {
	case path == "/api2/json/cluster/status" && r.Method == http.MethodGet:
		m.mu.Lock()
		m.statusCalls++
		m.mu.Unlock()
		if standalone {
			envelope(w, []map[string]any{
				{"type": "node", "name": "pve1", "ip": "127.0.0.1", "online": 1, "id": "node/pve1"},
			})
			return
		}
		envelope(w, clusterStatusSeed())
		return

	case path == "/api2/json/nodes" && r.Method == http.MethodGet:
		m.mu.Lock()
		m.nodesCalls++
		m.mu.Unlock()
		switch getMode {
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
			envelope(w, []map[string]any{
				{"node": "pve1", "status": "online"},
				{"node": "pve2", "status": "online"},
			})
		}
		return

	case path == "/api2/json/nodes/pve1/qemu/100/status/start" && r.Method == http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, `{"data":null}`, http.StatusBadRequest)
			return
		}
		m.mu.Lock()
		m.postForms = append(m.postForms, r.PostForm)
		m.postPaths = append(m.postPaths, path)
		mode := mutMode
		m.mu.Unlock()
		switch mode {
		case "slow":
			time.Sleep(300 * time.Millisecond)
			envelope(w, testUPID)
		case "sync_null":
			envelope(w, nil)
		case "error5xx":
			http.Error(w, `{"data":null}`, http.StatusInternalServerError)
		case "strict_schema":
			// PVE 9.x 엄격 스키마 재현 — background_delay가 실린 첫 POST만
			// 400("property is not defined in schema")으로 거부. 적응 재시도
			// (delay 제거)은 UPID로 수용한다(§13-9 버전 적응 계약).
			m.mu.Lock()
			delayed := r.FormValue("background_delay") != ""
			m.mu.Unlock()
			if delayed {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"data":null,"errors":{"background_delay":"property is not defined in schema and the schema does not allow additional properties"},"message":"Parameter verification failed.\n"}`))
				return
			}
			envelope(w, testUPID)
		case "strict_schema_hard":
			// 재시도 후에도 실패하는 형상 — delay 유무와 무관한 400. 적응
			// 재시도가 원 에러를 전파하는지 단얫하는 입력.
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"data":null,"errors":{"vmid":"volume quota reached"},"message":"Parameter verification failed.\n"}`))
			return
		default:
			envelope(w, testUPID)
		}
		return

	case strings.HasPrefix(path, "/api2/json/nodes/pve1/tasks/") &&
		strings.HasSuffix(path, "/status") && r.Method == http.MethodGet:
		envelope(w, map[string]any{"status": "stopped", "exitstatus": "OK"})
		return

	default:
		http.Error(w, `{"data":null}`, http.StatusNotFound)
	}
}

// --- 클라이언트 조립 헬퍼 ---

func mustCredential(t *testing.T) tokenCredential {
	t.Helper()
	cred, err := parseTokenCredential(testCredentialMaterial)
	if err != nil {
		t.Fatalf("parseTokenCredential: %v", err)
	}
	return cred
}

func newTestClient(t *testing.T, m *mockProxmox, opts ...clientOption) *Client {
	t.Helper()
	opts = append([]clientOption{
		withMetrics(metrics.New()),
		withRequestTimeout(2 * time.Second),
	}, opts...)
	return newClient(mustCredential(t), m.srv.URL, opts...)
}

// assertNoTokenMaterial — claim 13 보강 단얫(r1.4 MEDIUM-9): 에러 체인의 모든
// 문자열 표현에 토큰 헤더 접두(PVEAPIToken=)와 자격 성분 원문이 부재함을
// 단얫한다. grep이 로그 호출 라인만 잡는 허점을 하니스 측에서 폐쇄한다.
func assertNoTokenMaterial(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	check := func(msg string) {
		if strings.Contains(msg, "PVEAPIToken=") {
			t.Errorf("error string leaks token header prefix: %q", msg)
		}
		if strings.Contains(msg, testTokenSecret) {
			t.Errorf("error string leaks token secret: %q", msg)
		}
	}
	var walk func(error)
	walk = func(e error) {
		if e == nil {
			return
		}
		check(e.Error())
		var se *contract.ProviderSignalError
		if errors.As(e, &se) {
			check("message:" + se.Message)
		}
		if u, ok := e.(interface{ Unwrap() error }); ok {
			walk(u.Unwrap())
		}
	}
	walk(err)
}

// nodeEndpoint — 후보 전환 목표 URL 예측(현재 엔드포인트의 포트를 승계한다).
func nodeEndpoint(m *mockProxmox, ip string) string {
	u, err := url.Parse(m.srv.URL)
	if err != nil {
		m.t.Fatalf("mock url parse: %v", err)
	}
	port := u.Port()
	if port == "" {
		port = defaultAPIPort
	}
	return "https://" + ip + ":" + port
}

// --- 계약 1: 토큰 헤더 조립 + {data} 래핑 디코드 (N1 — §14.3 계약 2·A2) ---

func TestClientAuthorizationHeaderAndEnvelope(t *testing.T) {
	m := newMockProxmox(t)
	c := newTestClient(t, m)

	var nodes []struct {
		Node   string `json:"node"`
		Status string `json:"status"`
	}
	if err := c.get(context.Background(), "/nodes", "discover", &nodes); err != nil {
		t.Fatalf("get /nodes: %v", err)
	}
	if len(nodes) != 2 || nodes[0].Node != "pve1" || nodes[1].Status != "online" {
		t.Fatalf("envelope data not decoded: %+v", nodes)
	}

	snap := m.snap()
	if len(snap.authHeaders) == 0 {
		t.Fatal("mock captured no requests")
	}
	for i, h := range snap.authHeaders {
		if h != expectedAuthHeader {
			t.Errorf("request %d Authorization = %q, want %q", i, h, expectedAuthHeader)
		}
	}

	// 성분 조립의 원천 — 재질 JSON 파싱 정합.
	cred := mustCredential(t)
	if cred.tokenUser != testTokenUser || cred.tokenID != testTokenID || cred.tokenSecret != testTokenSecret {
		t.Fatalf("credential components mismatch: %+v", cred)
	}
	if cred.authorizationValue() != expectedAuthHeader {
		t.Errorf("authorizationValue = %q, want %q", cred.authorizationValue(), expectedAuthHeader)
	}
}

func TestClientCredentialMaterialMissing(t *testing.T) {
	for name, material := range map[string]string{
		"empty":        "",
		"not-json":     "plain-kubeconfig-like",
		"empty-user":   `{"tokenID":"t","tokenSecret":"s"}`,
		"empty-id":     `{"tokenUser":"u","tokenSecret":"s"}`,
		"empty-secret": `{"tokenUser":"u","tokenID":"t"}`,
	} {
		_, err := parseTokenCredential(material)
		if err == nil {
			t.Errorf("%s: parseTokenCredential succeeded, want error", name)
		}
		assertNoTokenMaterial(t, err)
	}
}

// --- 계약 2: mutation의 background_delay 명시 첨부 (판정 J4 — 증류 계약 1) ---

func TestClientMutationAttachesBackgroundDelay(t *testing.T) {
	m := newMockProxmox(t)
	c := newTestClient(t, m)

	ctx := context.Background()
	form := url.Values{"node": {"pve1"}}
	data, err := c.postForm(ctx, "/nodes/pve1/qemu/100/status/start", form, "execute")
	if err != nil {
		t.Fatalf("postForm: %v", err)
	}
	var upid string
	if err := json.Unmarshal(data, &upid); err != nil {
		t.Fatalf("data decode: %v (raw %s)", err, data)
	}
	if upid != testUPID {
		t.Errorf("data = %q, want %q", upid, testUPID)
	}

	snap := m.snap()
	if len(snap.postForms) != 1 {
		t.Fatalf("captured %d post forms, want 1", len(snap.postForms))
	}
	delay := snap.postForms[0].Get("background_delay")
	if delay != strconv.Itoa(defaultBackgroundDelaySeconds) {
		t.Errorf("background_delay = %q, want %q", delay, strconv.Itoa(defaultBackgroundDelaySeconds))
	}
	if snap.postForms[0].Get("node") != "pve1" {
		t.Errorf("caller form fields lost: %v", snap.postForms[0])
	}

	// 동기 완료 분기 — data null은 raw로 전달된다(Phase D executor가 3분기).
	m.setMutMode("sync_null")
	nullData, err := c.postForm(ctx, "/nodes/pve1/qemu/100/status/start", nil, "execute")
	if err != nil {
		t.Fatalf("postForm sync_null: %v", err)
	}
	if strings.TrimSpace(string(nullData)) != "null" {
		t.Errorf("sync-complete data = %q, want null", nullData)
	}
}

// --- 계약 3: UPID 파싱 전 성분·변형 거부·핸들 왕복 (N2 — 판정 J4) ---

func TestUPIDParseAllComponents(t *testing.T) {
	u, err := parseUPID(testUPID)
	if err != nil {
		t.Fatalf("parseUPID: %v", err)
	}
	if u.Raw != testUPID {
		t.Errorf("Raw = %q, want %q", u.Raw, testUPID)
	}
	want := UPID{
		Raw:       testUPID,
		Node:      "pve1",
		PID:       "00123456",
		PStart:    "000A1B2C",
		StartTime: "68BADBA0",
		Type:      "qemu",
		ID:        "start",
		User:      "root@pam",
	}
	if u != want {
		t.Errorf("parsed = %+v, want %+v", u, want)
	}
}

func TestUPIDParseRejectsMutations(t *testing.T) {
	cases := []string{
		"",
		"UPID:",
		"UPID:pve1",
		"UPID:pve1:1:2:3",
		"upid:pve1:1:2:3:qemu:start:root@pam:",   // 접두 대소문자 변형
		"UPID::1:2:3:qemu:start:root@pam:",       // node 성분 부재
		"UPID:pve1:1:2:3:qemu:start:root@pam",    // 후행 콜론 부재
		"UPID:pve1:1:2:3:qemu:start:root@pam:x:", // 성분 초과
		"XEED:pve1:1:2:3:qemu:start:root@pam:",   // 접두 변형
	}
	for _, raw := range cases {
		_, err := parseUPID(raw)
		if err == nil {
			t.Errorf("parseUPID(%q) succeeded, want rejection", raw)
			continue
		}
		if !strings.Contains(err.Error(), "upid") {
			t.Errorf("parseUPID(%q) error %q lacks package prefix", raw, err)
		}
	}
}

func TestUPIDHandleRoundTrip(t *testing.T) {
	u, err := parseUPID(testUPID)
	if err != nil {
		t.Fatalf("parseUPID: %v", err)
	}

	ref := encodeUPIDRef("conn-123", u)
	if ref != "upid|conn-123|pve1|"+testUPID {
		t.Errorf("encodeUPIDRef = %q, want %q", ref, "upid|conn-123|pve1|"+testUPID)
	}

	h, err := decodeUPIDRef(ref)
	if err != nil {
		t.Fatalf("decodeUPIDRef: %v", err)
	}
	if h.ConnectionUID != "conn-123" {
		t.Errorf("ConnectionUID = %q, want conn-123", h.ConnectionUID)
	}
	if h.UPID != u {
		t.Errorf("round trip mismatch: got %+v want %+v", h.UPID, u)
	}

	// 재인코딩 왕복 정합(단얫 소유 — k8s decodeRolloutRef 선례).
	if again := encodeUPIDRef(h.ConnectionUID, h.UPID); again != ref {
		t.Errorf("re-encode = %q, want %q", again, ref)
	}
}

func TestUPIDHandleRejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"wrong marker":   "rollout|conn-123|pve1|" + testUPID,
		"too few parts":  "upid|conn-123|pve1",
		"too many parts": "upid|conn-123|pve1|" + testUPID + "|extra",
		"empty connUID":  "upid||pve1|" + testUPID,
		"node mismatch":  "upid|conn-123|pveX|" + testUPID,
		"raw not a UPID": "upid|conn-123|pve1|not-a-upid",
		"empty raw":      "upid|conn-123|pve1|",
	}
	for name, ref := range cases {
		if _, err := decodeUPIDRef(ref); err == nil {
			t.Errorf("%s: decodeUPIDRef(%q) succeeded, want rejection", name, ref)
		}
	}
}

// --- 계약 4: 페일오버 비대칭 — 시나리오 1 쓰기 타임아웃 전환 부재 (J6(2)) ---

func TestFailoverWriteTimeoutNeverSwitches(t *testing.T) {
	m := newMockProxmox(t)
	m.setMutMode("slow")
	c := newTestClient(t, m, withRequestTimeout(50*time.Millisecond))

	ctx := context.Background()
	for i := 0; i < failoverReadThreshold+1; i++ { // 임계 초과 반복해도 쓰기 실패는 증거가 아니다
		_, err := c.postForm(ctx, "/nodes/pve1/qemu/100/status/start", nil, "execute")
		if err == nil {
			t.Fatalf("attempt %d: postForm succeeded, want timeout", i)
		}
		assertNoTokenMaterial(t, err)
		// 시나리오가 실제 타임아웃을 만들었는지의 증거 — 클라이언트의 타임아웃
		// 식별 계약 메시지(do의 isTimeout 분기)로 단얫한다.
		var sig *contract.ProviderSignalError
		if !errors.As(err, &sig) || sig.Kind != contract.SignalUnreachable {
			t.Fatalf("attempt %d: error %v is not an unreachable signal", i, err)
		}
		if !strings.Contains(err.Error(), "request timed out") {
			t.Fatalf("attempt %d: error %v does not identify the timeout", i, err)
		}
		if c.endpoint != m.srv.URL {
			t.Fatalf("attempt %d: endpoint switched to %q on write timeout — timed-out write is never failover evidence", i, c.endpoint)
		}
	}
	if snap := m.snap(); snap.statusCalls != 0 {
		t.Errorf("cluster/status queried %d times on write failures, want 0", snap.statusCalls)
	}
	if c.readFailures != 0 {
		t.Errorf("read failure counter advanced to %d on write failures, want 0", c.readFailures)
	}
}

// --- 시나리오 2: 읽기 연속 실패 → 임계 도달 전환 ---

func TestFailoverReadConsecutiveFailuresSwitch(t *testing.T) {
	m := newMockProxmox(t)
	m.setGetMode("fail500")
	c := newTestClient(t, m)

	ctx := context.Background()
	if err := c.get(ctx, "/nodes", "discover", nil); err == nil {
		t.Fatal("first read should fail")
	} else {
		assertNoTokenMaterial(t, err)
		if c.endpoint != m.srv.URL {
			t.Fatalf("endpoint switched after 1 failure (%d < threshold %d)", c.readFailures, failoverReadThreshold)
		}
	}
	if err := c.get(ctx, "/nodes", "discover", nil); err == nil {
		t.Fatal("second read should fail")
	} else {
		assertNoTokenMaterial(t, err)
	}

	// 2회 연속 실패 → /cluster/status 재조회 → 첫 후보로 전환.
	if c.endpoint != nodeEndpoint(m, "10.20.0.11") {
		t.Errorf("endpoint = %q, want switch to %q", c.endpoint, nodeEndpoint(m, "10.20.0.11"))
	}
	if c.readFailures != 0 {
		t.Errorf("counter not reset after switch: %d", c.readFailures)
	}
	snap := m.snap()
	if snap.statusCalls != 1 {
		t.Errorf("cluster/status queried %d times, want 1", snap.statusCalls)
	}
	if snap.nodesCalls != 2 {
		t.Errorf("/nodes called %d times, want 2", snap.nodesCalls)
	}

	// 성공 응답은 카운터를 리셋한다(연속성 요건).
	c.resetReadFailures()
	if c.readFailures != 0 {
		t.Errorf("resetReadFailures left %d", c.readFailures)
	}
}

// --- 시나리오 3: 오프라인 멤버 IP 후보 보존 (증류 계약 4) ---

func TestFailoverPreservesOfflineMemberIP(t *testing.T) {
	// 단위: clusterNodeIPs는 online·offline을 모두 보존하고 cluster행·빈 ip는 건너뛴다.
	entries, err := func() ([]clusterStatusEntry, error) {
		var out []clusterStatusEntry
		blob, _ := json.Marshal(clusterStatusSeed())
		if err := json.Unmarshal(blob, &out); err != nil {
			return nil, err
		}
		return out, nil
	}()
	if err != nil {
		t.Fatalf("seed decode: %v", err)
	}
	ips := clusterNodeIPs(entries)
	want := []string{"10.20.0.11", "10.20.0.12", "10.20.0.13"}
	if len(ips) != len(want) {
		t.Fatalf("clusterNodeIPs = %v, want %v", ips, want)
	}
	for i, w := range want {
		if ips[i] != w {
			t.Errorf("clusterNodeIPs[%d] = %q, want %q", i, ips[i], w)
		}
	}

	// 행동: 임계 도달 → 전환 후보에 오프라인 멤버 IP가 남는다.
	m := newMockProxmox(t)
	m.setGetMode("fail500")
	c := newTestClient(t, m)
	c.readFailures = failoverReadThreshold
	if err := c.get(context.Background(), "/nodes", "discover", nil); err == nil {
		t.Fatal("primed read should fail")
	}
	if c.endpoint != nodeEndpoint(m, "10.20.0.11") {
		t.Errorf("endpoint = %q, want %q", c.endpoint, nodeEndpoint(m, "10.20.0.11"))
	}
	found := false
	for _, cand := range c.lastCandidates {
		if cand == nodeEndpoint(m, "10.20.0.13") {
			found = true
		}
	}
	if !found {
		t.Errorf("offline member ip missing from candidates: %v", c.lastCandidates)
	}
	if len(c.lastCandidates) != 3 {
		t.Errorf("candidates = %v, want 3 (online 2 + offline 1)", c.lastCandidates)
	}
}

// --- 시나리오 4: reverse-proxy 모드 — 전환 자체 금지 (증류 계약 5) ---

func TestFailoverReverseProxyPinned(t *testing.T) {
	m := newMockProxmox(t)
	m.setGetMode("fail500")
	c := newTestClient(t, m, withReverseProxy())

	for i := 0; i < failoverReadThreshold+2; i++ {
		if err := c.get(context.Background(), "/nodes", "discover", nil); err == nil {
			t.Fatalf("read %d should fail", i)
		} else {
			assertNoTokenMaterial(t, err)
		}
		if c.endpoint != m.srv.URL {
			t.Fatalf("read %d: endpoint switched to %q under reverse_proxy deployment mode", i, c.endpoint)
		}
	}
	if snap := m.snap(); snap.statusCalls != 0 {
		t.Errorf("cluster/status queried %d times under reverse_proxy mode, want 0", snap.statusCalls)
	}
	if c.lastCandidates != nil {
		t.Errorf("candidates recorded under reverse_proxy mode: %v", c.lastCandidates)
	}
}

// --- 발화 조건 한계(r1.4 MEDIUM-7): standalone — 후보가 자기 자신뿐이면 비발동 ---

func TestFailoverStandaloneNoSwitch(t *testing.T) {
	m := newMockProxmox(t)
	m.setGetMode("fail500")
	m.mu.Lock()
	m.standalone = true
	m.mu.Unlock()
	c := newTestClient(t, m)

	for i := 0; i < failoverReadThreshold+1; i++ {
		if err := c.get(context.Background(), "/nodes", "discover", nil); err == nil {
			t.Fatalf("read %d should fail", i)
		}
		if c.endpoint != m.srv.URL {
			t.Fatalf("read %d: endpoint switched to %q in standalone topology", i, c.endpoint)
		}
	}
	if c.lastCandidates != nil {
		t.Errorf("candidates recorded in standalone topology: %v", c.lastCandidates)
	}
}

// --- 에러 매핑 3종 (§3.1 — 5xx unreachable · 401/403 permission_denied · 쓰로틀 rate_limited) ---

func TestClientErrorSignalMapping(t *testing.T) {
	cases := []struct {
		name    string
		getMode string
		want    string
	}{
		{"internal 5xx", "fail500", contract.SignalUnreachable},
		{"unauthorized 401", "fail401", contract.SignalPermissionDenied},
		{"forbidden 403", "fail403", contract.SignalPermissionDenied},
		{"throttled 429", "fail429", contract.SignalRateLimited},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newMockProxmox(t)
			m.setGetMode(tc.getMode)
			c := newTestClient(t, m, withReverseProxy()) // 고정 엔드포인트 — 매핑만 관찰

			err := c.get(context.Background(), "/nodes", "discover", nil)
			if err == nil {
				t.Fatal("get should fail")
			}
			var sig *contract.ProviderSignalError
			if !errors.As(err, &sig) {
				t.Fatalf("error %v is not a ProviderSignalError", err)
			}
			if sig.Kind != tc.want {
				t.Errorf("signal kind = %q, want %q", sig.Kind, tc.want)
			}
			assertNoTokenMaterial(t, err)
		})
	}

	// 전송류 실패 → unreachable (페일오버 증거류 — 연결 클래스 실패).
	t.Run("transport unreachable", func(t *testing.T) {
		m := newMockProxmox(t)
		c := newTestClient(t, m, withReverseProxy())
		m.srv.Close()
		err := c.get(context.Background(), "/nodes", "discover", nil)
		if err == nil {
			t.Fatal("get against closed server should fail")
		}
		var sig *contract.ProviderSignalError
		if !errors.As(err, &sig) || sig.Kind != contract.SignalUnreachable {
			t.Fatalf("error %v is not unreachable signal", err)
		}
		assertNoTokenMaterial(t, err)
	})

	// 디코드 실패 — 비신호 일반 에러(어휘 확장 금지).
	t.Run("bad json plain error", func(t *testing.T) {
		m := newMockProxmox(t)
		m.setGetMode("badjson")
		c := newTestClient(t, m, withReverseProxy())
		err := c.get(context.Background(), "/nodes", "discover", nil)
		if err == nil {
			t.Fatal("bad json should fail")
		}
		var sig *contract.ProviderSignalError
		if errors.As(err, &sig) {
			t.Fatalf("decode failure must stay a plain error, got signal %q", sig.Kind)
		}
		assertNoTokenMaterial(t, err)
	})
}

// --- claim 13 보강(r1.4 MEDIUM-9): 모든 error 문자열의 자격 비로그 스윕 ---

func TestClientErrorsNeverCarryTokenMaterial(t *testing.T) {
	scenarios := []struct {
		name   string
		excite func(t *testing.T, c *Client, m *mockProxmox) error
	}{
		{"read 5xx", func(t *testing.T, c *Client, m *mockProxmox) error {
			m.setGetMode("fail500")
			return c.get(context.Background(), "/nodes", "discover", nil)
		}},
		{"read 401", func(t *testing.T, c *Client, m *mockProxmox) error {
			m.setGetMode("fail401")
			return c.get(context.Background(), "/nodes", "discover", nil)
		}},
		{"read 429", func(t *testing.T, c *Client, m *mockProxmox) error {
			m.setGetMode("fail429")
			return c.get(context.Background(), "/nodes", "discover", nil)
		}},
		{"read bad json", func(t *testing.T, c *Client, m *mockProxmox) error {
			m.setGetMode("badjson")
			return c.get(context.Background(), "/nodes", "discover", nil)
		}},
		{"write timeout", func(t *testing.T, c *Client, m *mockProxmox) error {
			m.setMutMode("slow")
			c.timeout = 50 * time.Millisecond // 시나리오 축소 — 실제 기한 초과 유발
			c.http.Timeout = 50 * time.Millisecond
			_, err := c.postForm(context.Background(), "/nodes/pve1/qemu/100/status/start", nil, "execute")
			return err
		}},
		{"write 5xx", func(t *testing.T, c *Client, m *mockProxmox) error {
			m.setMutMode("error5xx")
			_, err := c.postForm(context.Background(), "/nodes/pve1/qemu/100/status/start", nil, "execute")
			return err
		}},
		{"transport refused", func(t *testing.T, c *Client, m *mockProxmox) error {
			m.srv.Close()
			return c.get(context.Background(), "/nodes", "discover", nil)
		}},
		{"bad credential material", func(t *testing.T, _ *Client, _ *mockProxmox) error {
			_, err := parseTokenCredential(`{"tokenUser":"u","tokenID":"t"}`)
			return err
		}},
		{"upid malformed", func(t *testing.T, _ *Client, _ *mockProxmox) error {
			_, err := parseUPID("UPID:pve1")
			return err
		}},
		{"handle malformed", func(t *testing.T, _ *Client, _ *mockProxmox) error {
			_, err := decodeUPIDRef("upid||pve1|" + testUPID)
			return err
		}},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			m := newMockProxmox(t)
			c := newTestClient(t, m)
			err := sc.excite(t, c, m)
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

// TestClientPostFormAdaptsToStrictSchema — 버전 적응(§13-9 발견): PVE 9.x
// 엄격 스키마는 background_delay를 400으로 거부한다. 첫 응답이 이 거부면
// delay를 떼고 1회 재시도해 UPID를 받는다 — 수용 버전(기본 응답)과 거부
// 버전(재시도) 모두 동일 연산이 성공해야 한다.
func TestClientPostFormAdaptsToStrictSchema(t *testing.T) {
	m := newMockProxmox(t)
	m.setMutMode("strict_schema")
	c := newTestClient(t, m)

	data, err := c.postForm(context.Background(),
		"/nodes/pve1/qemu/100/status/start", url.Values{"node": {"pve1"}}, "execute")
	if err != nil {
		t.Fatalf("strict-schema PVE must still accept the operation after delay drop: %v", err)
	}
	var upid string
	if err := json.Unmarshal(data, &upid); err != nil || upid != testUPID {
		t.Fatalf("retry data = %q (%v), want %q", data, err, testUPID)
	}

	snap := m.snap()
	if len(snap.postForms) != 2 {
		t.Fatalf("captured %d post forms, want 2 (delayed attempt + degraded retry)", len(snap.postForms))
	}
	if snap.postForms[0].Get("background_delay") == "" {
		t.Error("first attempt must carry background_delay (optimization for accepting versions)")
	}
	if snap.postForms[1].Get("background_delay") != "" {
		t.Error("retry must drop background_delay (strict schema rejects it)")
	}
}

// TestClientPostFormSurfacesNonDelay400 — 400이어도 background_delay 거부가
// 아니면(예: vmid 오류) 재시도하지 않고 원 에러를 전파한다 — 저하 재시도는
// 스키마 차이에만 한정된다(멱등성 지배 원칙 유지).
func TestClientPostFormSurfacesNonDelay400(t *testing.T) {
	m := newMockProxmox(t)
	m.setMutMode("strict_schema_hard")
	c := newTestClient(t, m)

	_, err := c.postForm(context.Background(),
		"/nodes/pve1/qemu/100/status/start", url.Values{"node": {"pve1"}}, "execute")
	if err == nil {
		t.Fatal("non-delay 400 must surface as error")
	}
	if errors.Is(err, errBackgroundDelaySchema) {
		t.Fatalf("unrelated 400 must not be misread as delay-schema rejection: %v", err)
	}
	snap := m.snap()
	if len(snap.postForms) != 1 {
		t.Fatalf("captured %d post forms, want 1 (no retry for unrelated 400)", len(snap.postForms))
	}
}
