// Package proxmox is the V2 PVE (Proxmox VE) adapter transport (Phase 5 PR
// 31a Phase B, 계획 N1·판정 J1). The transport is direct PVE REST
// (`<endpoint>/api2/json/...`) over the standard library — NOT an SDK import
// (§10.2 신규 의존 금지; mock 경로의 완전 엔드포인트 주입면이 직접 HTTP의
// 가치). Phase B는 전송 코어(자격 헤더·{data} 래핑·background_delay·페일오버
// 비대칭·TLS posture)와 UPID 파싱만 착지한다 — 어댑터 공개면(adapter.go)은
// 배치표 다음 Phase의 소유고, executor 소비면(Execute/Poll)은 Phase D다.
//
// The adapter owns no control-plane tables and no DB handle (arch rule 2).
// Credentials arrive only via the broker-resolved material blob
// ({"tokenUser","tokenID","tokenSecret"} — 판정 J5) and are assembled into a
// single Authorization header per request; nothing persists them.
package proxmox

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/metrics"
)

const (
	// providerName — §18.2 카운터 라벨. 대문자 상수 ProviderName은 배치표
	// 어댑터 공개면(§3.1)이 소유하므로 여기선 패키지 내부 라벨만 둔다.
	providerName = "proxmox"

	// apiPrefix — PVE API2 JSON 접두(A3 — Connection.Endpoint는
	// https://host:8006 전체 베이스, 클라이언트가 접미 조립).
	apiPrefix = "/api2/json"

	// defaultAPIPort — PVE API 데몬(pveproxy) 표준 포트. 현재 엔드포인트에
	// 포트 성분이 없을 때 후보 노드 URL 재조립에 쓴다.
	defaultAPIPort = "8006"

	// requestTimeout — 1요청 상한(tencent cvmClient 선례 승계).
	requestTimeout = 8 * time.Second

	// failoverReadThreshold — 읽기·연결류 실패의 연속 임계(A4 상수). 2회 연속
	// 실패 후 /cluster/status 재조회로 전환을 시도한다.
	failoverReadThreshold = 2

	// defaultBackgroundDelaySeconds — mutation에 명시 첨부하는 background_delay
	// 값(판정 J4 — 증류 계약 1). PVE는 1–30초 창을 받고, 창 내 완료 시 응답
	// data가 null이 된다(동기 완료 → null 핸들 → 엔진 단일 attempt 성공의
	// 3분기 경로). 값 자체는 증명 대상이 아니다(A4) — 계약은 "명시 첨부"이고
	// 기본 상수는 구현 재량이다. Phase D executor가 폼을 직접 채워 덮어쓸 수
	// 있다(부재 시에만 기본값 주입).
	defaultBackgroundDelaySeconds = 5
)

// tokenCredential — SecretRef 재질 JSON의 성분 해체(판정 J5). 전체 헤더
// 문자열 저장이 아니라 성분 저장: Health 메시지의 계정 식별 마스킹(tokenUser
// 접두)과 토큰 분리 표시가 명시적이 되고, 조립은 매 요청 이 시점에 일어난다.
type tokenCredential struct {
	tokenUser   string
	tokenID     string
	tokenSecret string
}

// parseTokenCredential — Material 블롭(JSON 문자열) 파싱. 세 성분 모두
// 필수다. 에러 문구에는 재질 평문이 실리지 않는다(보존 제약 7).
func parseTokenCredential(material string) (tokenCredential, error) {
	var blob struct {
		TokenUser   string `json:"tokenUser"`
		TokenID     string `json:"tokenID"`
		TokenSecret string `json:"tokenSecret"`
	}
	trimmed := strings.TrimSpace(material)
	if trimmed == "" {
		return tokenCredential{}, errors.New("proxmox: missing credential material (broker purpose resolve returned an empty blob)")
	}
	if err := json.Unmarshal([]byte(trimmed), &blob); err != nil {
		return tokenCredential{}, fmt.Errorf("proxmox: credential material is not the expected {tokenUser,tokenID,tokenSecret} JSON blob")
	}
	cred := tokenCredential{
		tokenUser:   strings.TrimSpace(blob.TokenUser),
		tokenID:     strings.TrimSpace(blob.TokenID),
		tokenSecret: strings.TrimSpace(blob.TokenSecret),
	}
	if cred.tokenUser == "" || cred.tokenID == "" || cred.tokenSecret == "" {
		return tokenCredential{}, errors.New("proxmox: credential material is incomplete (tokenUser, tokenID and tokenSecret are all required)")
	}
	return cred, nil
}

// authorizationValue — `PVEAPIToken=<tokenUser>!<tokenID>=<tokenSecret>` 조립
// (§14.3 증류 계약 2). 이 값은 요청 헤더로만 흐른다 — 로그·에러·테스트 출력
// 물질 아님(claim 13).
func (c tokenCredential) authorizationValue() string {
	return "PVEAPIToken=" + c.tokenUser + "!" + c.tokenID + "=" + c.tokenSecret
}

// clientSettings — 전송 posture 주입면. reverseProxy는 배치 모드
// (ConfigJSON {"deployment_mode":"reverse_proxy"} — 판정 J6(3)), insecureTLS는
// --insecure-tls 자가서명 posture(J6(3)·A7)다.
type clientSettings struct {
	metrics      *metrics.Counters
	reverseProxy bool
	insecureTLS  bool
	timeout      time.Duration
}

// clientOption — newClient 옵션(어댑터·테스트 공용 주입면).
type clientOption func(*clientSettings)

func withMetrics(m *metrics.Counters) clientOption { return func(s *clientSettings) { s.metrics = m } }

func withReverseProxy() clientOption { return func(s *clientSettings) { s.reverseProxy = true } }

func withInsecureTLS() clientOption { return func(s *clientSettings) { s.insecureTLS = true } }

func withRequestTimeout(d time.Duration) clientOption {
	return func(s *clientSettings) { s.timeout = d }
}

// Client — 단일 PVE 엔드포인트를 향한 API2 전송. 인스턴스는 요청 스코프로
// 쓰인다(k8s "HTTP clients are per-request" 원칙 승계): 페일오버 임계
// 카운터와 전환된 엔드포인트는 인스턴스 수명 안에서만 유효하고, 후보 목록은
// 절대 인스턴스에 캐시하지 않는다 — 매 전환 직전 /cluster/status 재조회로만
// 얻는다(보존 제약 6 — stateless).
type Client struct {
	http         *http.Client
	credential   tokenCredential
	endpoint     string        // 전체 베이스(scheme://host[:port]) — 전환으로 교체된다
	reverseProxy bool          // reverse_proxy 배치 모드 — 전환 금지(증류 계약 5)
	timeout      time.Duration // 1요청 상한(do의 ctx 기한 — 테스트 주입면)
	metrics      *metrics.Counters

	// readFailures — 읽기(GET)·연결류 실패의 연속 카운터. 요청 스코프 수명
	// (R4 완화 — 프로세스 재시작 리셋 허용). 쓰기 실패는 절대 가산하지 않는다.
	readFailures int

	// lastCandidates — 마지막 전환 시 사용한 후보 목록(관측면 — 캐시가 아니라
	// 방금 일어난 전환의 기록이며, 다음 전환에는 다시 재조회가 선행한다).
	lastCandidates []string
}

// newClient — 자격 성분과 베이스 엔드포인트로 전송을 조립한다.
func newClient(cred tokenCredential, endpoint string, opts ...clientOption) *Client {
	s := clientSettings{timeout: requestTimeout}
	for _, opt := range opts {
		opt(&s)
	}
	transport := &http.Transport{}
	if s.insecureTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // --insecure-tls posture (J6(3)·A7)
	}
	return &Client{
		http:         &http.Client{Timeout: s.timeout, Transport: transport},
		credential:   cred,
		endpoint:     normalizeEndpoint(endpoint),
		reverseProxy: s.reverseProxy,
		timeout:      s.timeout,
		metrics:      s.metrics,
	}
}

// normalizeEndpoint — 스킴 없는 값에 https://를 접두한다(tencent 선례 — mock
// 경로는 http 스킴을 그대로 수용).
func normalizeEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "https://127.0.0.1:" + defaultAPIPort
	}
	if !strings.Contains(endpoint, "://") {
		return "https://" + endpoint
	}
	return endpoint
}

// apiURL — 베이스 + /api2/json 접두 + 경로(A3).
func (c *Client) apiURL(path string) string {
	return strings.TrimRight(c.endpoint, "/") + apiPrefix + path
}

// --- 전송 코어 ---

// apiEnvelope — PVE API2의 {"data": …} 래핑(A2). data는 원문 RawMessage로
// 전달한다 — UPID 문자열·null(동기 완료)·배열·객체의 4형상을 호출자가
// 해석한다(Phase D executor의 3분기 포함).
type apiEnvelope struct {
	Data json.RawMessage `json:"data"`
}

// do — 1회 와이어 요청. 신호 분류와 자격 헤더 조립의 유일 지점이며, 페일오버
// 장부(get이 소유)에는 개입하지 않는다 — maybeFailover의 /cluster/status
// 재조회도 이 코어를 그대로 쓴다(재귀 유발 없음).
func (c *Client) do(ctx context.Context, method, path, op string, form url.Values) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var body io.Reader
	if form != nil {
		body = bytes.NewBufferString(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, c.apiURL(path), body)
	if err != nil {
		return nil, fmt.Errorf("proxmox: %s request build: %w", op, err)
	}
	req.Header.Set("Authorization", c.credential.authorizationValue())
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		c.noteAPIError(op, "transport")
		// 전송류 실패 = unreachable — 자격 물질 없는 신호 에러(tencent 승계).
		// 기기류 에러의 원문은 노출하지 않는다(단일 문장 계약 — 코드베이스
		// 에러 형상 승계). 타임아웃은 Message로 식별한다 — 페일오버 비대칭의
		// "쓰기 타임아웃" 판별과 폴러 상세(Phase D)가 읽는 유일한 단서다.
		msg := fmt.Sprintf("proxmox %s unreachable", op)
		if isTimeout(err) {
			msg = fmt.Sprintf("proxmox %s unreachable (request timed out)", op)
		}
		return nil, &contract.ProviderSignalError{
			Kind:    contract.SignalUnreachable,
			Message: msg,
		}
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.classify(op, resp.StatusCode)
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		c.noteAPIError(op, "decode")
		return nil, fmt.Errorf("proxmox: %s response is not an API2 {data} envelope: %w", op, err)
	}
	return envelope.Data, nil
}

// classify — provider 측 실패를 닫힌 §9.3 신호 어휘로 매핑한다(계획 §3.1 —
// PVE 5xx→unreachable·401/403→permission_denied·쓰로틀→rate_limited). 그 밖은
// 일반 error로 남는다(어휘 확장 금지). 어느 경로에도 자격 물질은 진입하지
// 않는다(claim 13).
func (c *Client) classify(op string, status int) error {
	switch {
	case status == http.StatusTooManyRequests:
		if c.metrics != nil {
			c.metrics.IncRateLimit(providerName)
		}
		return &contract.ProviderSignalError{
			Kind:    contract.SignalRateLimited,
			Message: fmt.Sprintf("proxmox %s rate limited (status %d)", op, status),
		}
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		c.noteAPIError(op, strconv.Itoa(status))
		return &contract.ProviderSignalError{
			Kind:    contract.SignalPermissionDenied,
			Message: fmt.Sprintf("proxmox %s permission denied (status %d) — check the API token's privileges", op, status),
		}
	case status >= 500:
		c.noteAPIError(op, strconv.Itoa(status))
		return &contract.ProviderSignalError{
			Kind:    contract.SignalUnreachable,
			Message: fmt.Sprintf("proxmox %s failed server-side (status %d)", op, status),
		}
	}
	c.noteAPIError(op, strconv.Itoa(status))
	return fmt.Errorf("proxmox: %s failed (status %d)", op, status)
}

func (c *Client) noteAPIError(op, code string) {
	if c.metrics != nil {
		c.metrics.IncAPIError(providerName, op, code)
	}
}

// isTimeout — 전송 실패가 기한 초과류인지 판별(http.Client 기한·ctx 기한
// 모두 — 페일오버 비대칭의 "타임아웃" 어휘의 구현면).
func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// --- 읽기 경로 — 페일오버 장부의 소유자 ---

// get — 읽기(GET) 1회. 실패는 연속 카운터를 가산하고 임계 도달 시 전환을
// 시도한다(연결류 실패·읽기 실패만이 전환 증거다 — 판정 J6(2)). 성공은
// 카운터를 리셋한다.
func (c *Client) get(ctx context.Context, path, op string, target any) error {
	data, err := c.do(ctx, http.MethodGet, path, op, nil)
	if err != nil {
		c.readFailures++
		if c.readFailures >= failoverReadThreshold {
			c.maybeFailover(ctx)
		}
		return err
	}
	c.readFailures = 0
	if target == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("proxmox: %s data decode: %w", op, err)
	}
	return nil
}

func (c *Client) resetReadFailures() { c.readFailures = 0 }

// maybeFailover — 임계 도달 시의 전환 절차(판정 J6(2) — §14.3 증류 계약
// 3·4·5). 규칙:
//
//  1. reverse_proxy 배치 모드면 전환 자체를 금지한다(내부 node IP로 전환하면
//     안 되는 배치 — 계약 5).
//  2. 전환 직전 현재 엔드포인트로 /cluster/status를 재조회해 node행 ip 전부
//     (online·offline 포함)를 후보로 삼는다(계약 4 — 오프라인 멤버 IP가
//     보고되는 유일 표면). 조회 실패 시 기존 엔드포인트를 유지한다.
//  3. 현재 호스트와 동일한 후보는 제외한다 — standalone(후보=자기 자신)에서는
//     비발동이 정상 거동이다(r1.4 MEDIUM-7 발화 조건 한계).
//
// 전환은 가용성 최적화일 뿐 정확성 경로가 아니다(R4) — 어떤 분기에서도
// 기존 엔드포인트 유지가 안전한 폴백이다.
func (c *Client) maybeFailover(ctx context.Context) {
	// 임계 소진 — 재조회 시도는 임계 주기당 1회로 제한한다(전환 실패 시
	// 다시 임계만큼 실패가 쌓여야 다음 시도가 열린다).
	c.readFailures = 0
	if c.reverseProxy {
		return
	}
	var entries []clusterStatusEntry
	data, err := c.do(ctx, http.MethodGet, "/cluster/status", "failover", nil)
	if err != nil || json.Unmarshal(data, &entries) != nil {
		// 조회·해석 실패 → 기존 엔드포인트 유지(전환은 최적화)
		return
	}
	candidates := failoverCandidates(entries, c.endpoint)
	if len(candidates) == 0 {
		return
	}
	c.lastCandidates = candidates
	c.endpoint = candidates[0]
}

// clusterStatusEntry — /cluster/status data 배열의 행. type은 "cluster" |
// "node"이고 ip는 node행만 가진다.
type clusterStatusEntry struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	IP     string `json:"ip"`
	Online int    `json:"online"`
}

// clusterNodeIPs — node행의 ip 전부(online·offline 무관). cluster행과 ip가
// 없는 행은 건너뛴다(증류 계약 4 — 후보 목록이 클러스터 저하 시 멤버를
// 잃지 않는 근거).
func clusterNodeIPs(entries []clusterStatusEntry) []string {
	ips := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Type == "node" && strings.TrimSpace(e.IP) != "" {
			ips = append(ips, strings.TrimSpace(e.IP))
		}
	}
	return ips
}

// failoverCandidates — 후보 ip를 전환 대상 URL로 재조립하고 현재 호스트를
// 제외한다. 포트는 현재 엔드포인트의 것을 승계하고(명시 포트 없으면
// defaultAPIPort), 스킴은 https로 고정한다(PVE pveproxy는 TLS 서비스).
func failoverCandidates(entries []clusterStatusEntry, currentEndpoint string) []string {
	current, err := url.Parse(currentEndpoint)
	if err != nil {
		return nil
	}
	port := current.Port()
	if port == "" {
		port = defaultAPIPort
	}
	currentHost := current.Hostname()
	candidates := make([]string, 0, len(entries))
	for _, ip := range clusterNodeIPs(entries) {
		if strings.EqualFold(ip, currentHost) {
			continue // 자기 자신으로의 전환은 전환이 아니다(standalone 비발동)
		}
		candidates = append(candidates, "https://"+net.JoinHostPort(ip, port))
	}
	return candidates
}

// --- 쓰기 경로 — 페일오버 증거 비대칭의 반대편 ---

// postForm — mutation(POST) 1회. background_delay를 명시 첨부한다(판정 J4 —
// 증류 계약 1: 창 내 완료 시 PVE가 data null을 돌려 동기 완료로 판정된다.
// 호출자가 이미 채웠다면 그 값을 존중한다 — Phase D executor의 덮어쓰기면).
//
// 반환 data는 원문이다: "UPID:…"(비동기)·null(동기 완료)·에러 — 3분기는
// executor(Phase D)가 해석한다.
//
// 페일오버 비대칭(판정 J6(2)): 쓰기 실패 — 타임아웃 포함 — 는 전환 증거가
// 아니다("A timed-out write is never failover evidence"). 착지 가능성이
// 불명인 쓰기의 재시도·전환은 멱등성이 지배하므로, 이 경로는 장부를
// 건드리지 않고 에러만 반환한다.
func (c *Client) postForm(ctx context.Context, path string, form url.Values, op string) (json.RawMessage, error) {
	if form == nil {
		form = url.Values{}
	}
	if form.Get("background_delay") == "" {
		form.Set("background_delay", strconv.Itoa(defaultBackgroundDelaySeconds))
	}
	return c.do(ctx, http.MethodPost, path, op, form)
}
