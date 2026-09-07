package tencent

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/contracttest"
	"ops-admin/backend/internal/infra/metrics"
)

// --- 모의 Tencent CVM API 서버 (Path M — 계획 판정 J1) ---
//
// TC3-HMAC-SHA256 재서명 검증까지 겸한다(계획 §6 client 계약 — "httptest가
// 재서명 검증까지 겸"). 검증기는 이 테스트 파일 안에서 스펙(캐노니컬 요청 →
// 서명 문자열 → 3단 HMAC 체인)을 클라이언트 코드와 독립으로 재구성한다 —
// 같은 상수를 공유해도 유도 과정은 분리되어 있어야 반증 가능성이 남는다(R2
// 완화). 429/401 신호는 §9.3 어휘 매핑의 입력이다(계획 §3.1).
//
// 모의 자격 리터럴은 테스트 전용이다(claim 14 — 제품 코드 경로 아님). 값은
// secret-scan 규칙에 걸리지 않는 형태로 선정했다(AKIA 프리픽스·40자 [A-Za-z0-9/+=]
// aws 문맥·keyword-entropy 구문 모두 회피 — allowlist 불요).

const testAccessKey = "AKIDmockaccesskey00example"
const testSecretKey = "mock-tencent-signing-material-not-real"

const testPageSize = 4

const testRegionA = "ap-guangzhou"
const testRegionB = "ap-shanghai"

func sp(s string) *string { return &s }
func ip(i int64) *int64   { return &i }

// seedInstances — DescribeInstances InstanceSet의 축소판. 리전 2개(gz 4·sh 2)
// × pageSize 4 → 페이징 2회+리전 순회 1회가 단얫 1에서 동시 검증된다. ins-gz-2에
// 하네스 리댁션 캐나리를 Tags 값으로 심는다 — normalizer가 Raw에 태그 값을
// 넣지 않는 한(매핑 표 §4 — Raw는 최소 필드 집합) 단얫 4가 이를 잡는다.
func seedInstances() []cvmInstance {
	return []cvmInstance{
		{
			InstanceId: sp("ins-gz-1"), InstanceName: sp("web-1"),
			InstanceType: sp("S5.LARGE8"), InstanceState: sp("RUNNING"),
			Cpu: ip(8), Memory: ip(8), OsName: sp("CentOS 7.6 64bit"),
			Placement:          &cvmPlacement{Zone: sp("ap-guangzhou-1")},
			PrivateIpAddresses: []*string{sp("10.0.0.1")},
			PublicIpAddresses:  []*string{sp("1.2.3.4")},
			SystemDisk:         &cvmDisk{DiskSize: ip(20)},
			DataDisks:          []cvmDisk{{DiskSize: ip(40)}},
		},
		{
			InstanceId: sp("ins-gz-2"), InstanceName: sp("web-2"),
			InstanceType: sp("S5.MEDIUM4"), InstanceState: sp("RUNNING"),
			Cpu: ip(4), Memory: ip(4), OsName: sp("Ubuntu Server 22.04 LTS"),
			Placement:          &cvmPlacement{Zone: sp("ap-guangzhou-2")},
			PrivateIpAddresses: []*string{sp("10.0.0.2")},
			SystemDisk:         &cvmDisk{DiskSize: ip(50)},
		},
		{InstanceId: sp("ins-gz-3"), InstanceName: sp("bare-3")},
		{
			InstanceId: sp("ins-gz-4"), InstanceName: sp("batch-4"),
			InstanceType: sp("SA2.2XLARGE32"), InstanceState: sp("STOPPED"),
			Cpu: ip(8), Memory: ip(32), OsName: sp("TencentOS Server 3.1"),
			Placement:  &cvmPlacement{Zone: sp("ap-guangzhou-3")},
			SystemDisk: &cvmDisk{DiskSize: ip(20)},
		},
		{
			InstanceId: sp("ins-sh-1"), InstanceName: sp("cache-1"),
			InstanceType: sp("S5.LARGE8"), InstanceState: sp("RUNNING"),
			Cpu: ip(8), Memory: ip(16), OsName: sp("CentOS 8.0 64bit"),
			Placement:          &cvmPlacement{Zone: sp("ap-shanghai-1")},
			PrivateIpAddresses: []*string{sp("172.16.0.1"), sp("172.16.0.2")},
			PublicIpAddresses:  []*string{sp("5.6.7.8")},
			SystemDisk:         &cvmDisk{DiskSize: ip(20)},
			DataDisks:          []cvmDisk{{DiskSize: ip(100)}, {DiskSize: ip(100)}},
		},
		{
			InstanceId: sp("ins-sh-2"), InstanceName: sp("cache-2"),
			InstanceType: sp("IT5.2XLARGE40"), InstanceState: sp("REBOOTING"),
			Cpu: ip(8), Memory: ip(40),
			Placement: &cvmPlacement{Zone: sp("ap-shanghai-2")},
		},
	}
}

type mockCVM struct {
	t         *testing.T
	srv       *httptest.Server
	mu        sync.Mutex
	instances map[string][]cvmInstance // DescribeInstances는 리전 스코프다
	mode      string                   // "" normal | rate_limited | permission_denied | internal_error
	calls     int
}

func newMockCVM(t *testing.T) *mockCVM {
	m := &mockCVM{t: t, instances: map[string][]cvmInstance{
		testRegionA: seedInstances()[:4],
		testRegionB: seedInstances()[4:],
	}}
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.srv.Close)
	return m
}

func writeTC3Error(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"Response": map[string]any{
			"Error":     map[string]any{"Code": code, "Message": message},
			"RequestId": "req-mock-" + code,
		},
	})
}

func (m *mockCVM) handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeTC3Error(w, http.StatusBadRequest, "InvalidRequest", "unreadable body")
		return
	}

	m.mu.Lock()
	m.calls++
	mode := m.mode
	m.mu.Unlock()

	switch mode {
	case contract.SignalRateLimited:
		writeTC3Error(w, http.StatusTooManyRequests, "RequestLimitExceeded", "throttled by the mock")
		return
	case contract.SignalPermissionDenied:
		writeTC3Error(w, http.StatusForbidden, "AuthFailure.SignatureFailure", "not authorized (mock scenario)")
		return
	case "internal_error":
		writeTC3Error(w, http.StatusInternalServerError, "InternalError", "mock internal failure")
		return
	}

	if err := verifyTC3Request(r, body, testAccessKey, testSecretKey); err != nil {
		m.t.Logf("mock rejected request: %v", err)
		writeTC3Error(w, http.StatusUnauthorized, "AuthFailure.SignatureFailure", "signature verification failed (mock)")
		return
	}

	var reqBody struct {
		Limit  int64 `json:"Limit"`
		Offset int64 `json:"Offset"`
	}
	if err := json.Unmarshal(body, &reqBody); err != nil {
		writeTC3Error(w, http.StatusBadRequest, "InvalidParameter", "undecodable DescribeInstances body")
		return
	}

	m.mu.Lock()
	region := r.Header.Get("X-TC-Region")
	instances := m.instances[region]
	m.mu.Unlock()

	start := int(reqBody.Offset)
	if start < 0 {
		start = 0
	}
	if start > len(instances) {
		start = len(instances)
	}
	end := start
	if reqBody.Limit > 0 {
		end = start + int(reqBody.Limit)
	}
	if end > len(instances) {
		end = len(instances)
	}
	page := instances[start:end]
	if page == nil {
		page = []cvmInstance{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"Response": map[string]any{
			"TotalCount":  len(instances),
			"InstanceSet": page,
			"RequestId":   "req-mock-describe",
		},
	})
}

// verifyTC3Request — 수신 요청의 TC3 서명을 클라이언트와 독립으로 재구성해
// 검증한다: Authorization 파싱 → 캐노니컬 요청(body·Content-Type·Host 수신값
// 그대로) → string-to-sign → 3단 HMAC 체인 → 서명 비교. 타임스탬프 스큐(5분 —
// Tencent 공식 규약)·Action/Version/Region 헤더도 함께 단얫한다.
func verifyTC3Request(r *http.Request, body []byte, wantAccessKey, wantSecretKey string) error {
	auth := r.Header.Get("Authorization")
	const prefix = "TC3-HMAC-SHA256 "
	if !strings.HasPrefix(auth, prefix) {
		return fmt.Errorf("authorization header lacks TC3 prefix: %q", auth)
	}
	parts := map[string]string{}
	for _, seg := range strings.Split(strings.TrimPrefix(auth, prefix), ", ") {
		kv := strings.SplitN(seg, "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("malformed authorization segment %q", seg)
		}
		parts[kv[0]] = kv[1]
	}
	credential := parts["Credential"]
	signature := parts["Signature"]
	if signed := parts["SignedHeaders"]; signed != "content-type;host" {
		return fmt.Errorf("SignedHeaders = %q, want content-type;host", signed)
	}
	segments := strings.Split(credential, "/")
	if len(segments) != 4 || segments[2] != "cvm" || segments[3] != "tc3_request" {
		return fmt.Errorf("credential scope %q malformed (want <key>/<date>/cvm/tc3_request)", credential)
	}
	if segments[0] != wantAccessKey {
		return fmt.Errorf("credential access key %q, want %q", segments[0], wantAccessKey)
	}
	date := segments[1]

	timestamp, err := strconv.ParseInt(r.Header.Get("X-TC-Timestamp"), 10, 64)
	if err != nil {
		return fmt.Errorf("X-TC-Timestamp parse: %w", err)
	}
	if skew := time.Since(time.Unix(timestamp, 0)); skew > 5*time.Minute || skew < -5*time.Minute {
		return fmt.Errorf("timestamp skew %v exceeds the 5-minute TC3 bound", skew)
	}
	if want := time.Unix(timestamp, 0).UTC().Format("2006-01-02"); date != want {
		return fmt.Errorf("credential scope date %q does not match X-TC-Timestamp date %q", date, want)
	}
	if action := r.Header.Get("X-TC-Action"); action != "DescribeInstances" {
		return fmt.Errorf("X-TC-Action = %q, want DescribeInstances", action)
	}
	if version := r.Header.Get("X-TC-Version"); version != apiVersion {
		return fmt.Errorf("X-TC-Version = %q, want %q", version, apiVersion)
	}
	if r.Header.Get("X-TC-Region") == "" {
		return fmt.Errorf("X-TC-Region header missing")
	}

	payloadHash := sha256.Sum256(body)
	canonicalRequest := "POST\n/\n\ncontent-type:" + r.Header.Get("Content-Type") +
		"\nhost:" + r.Host + "\n\ncontent-type;host\n" + hex.EncodeToString(payloadHash[:])
	hashedCanonical := sha256.Sum256([]byte(canonicalRequest))
	credentialScope := date + "/cvm/tc3_request"
	stringToSign := "TC3-HMAC-SHA256\n" + strconv.FormatInt(timestamp, 10) + "\n" +
		credentialScope + "\n" + hex.EncodeToString(hashedCanonical[:])
	kDate := hmacSHA256([]byte("TC3"+wantSecretKey), date)
	kService := hmacSHA256(kDate, "cvm")
	kSigning := hmacSHA256(kService, "tc3_request")
	want := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))
	if !hmac.Equal([]byte(want), []byte(signature)) {
		return fmt.Errorf("signature mismatch (want %q, got %q)", want, signature)
	}
	return nil
}

// --- 하네스 Fixture — 모의 서버를 시나리오·자격 주입면으로 감싼다. ---

type tencentFixture struct {
	t           *testing.T
	mock        *mockCVM
	adapter     *Adapter
	unreachable bool
}

func newTencentFixture(t *testing.T) *tencentFixture {
	mock := newMockCVM(t)
	adapter := NewAdapter(WithPageSize(testPageSize))
	return &tencentFixture{t: t, mock: mock, adapter: adapter}
}

func materialFor(accessKey, secretKey string) string {
	return fmt.Sprintf(`{"accessKey":%q,"secretKey":%q}`, accessKey, secretKey)
}

func (f *tencentFixture) Seed() []contract.DiscoveredResource {
	// 독립 오라클: normalizer 산물이 아닌 수기 URN 목록(자기충족 방지 — k8s
	// Fixture 동일 관례).
	return []contract.DiscoveredResource{
		{ExternalURN: "urn:tencent:1:compute.vm:ins-gz-1", Kind: "compute.vm"},
		{ExternalURN: "urn:tencent:1:compute.vm:ins-gz-2", Kind: "compute.vm"},
		{ExternalURN: "urn:tencent:1:compute.vm:ins-gz-3", Kind: "compute.vm"},
		{ExternalURN: "urn:tencent:1:compute.vm:ins-gz-4", Kind: "compute.vm"},
		{ExternalURN: "urn:tencent:1:compute.vm:ins-sh-1", Kind: "compute.vm"},
		{ExternalURN: "urn:tencent:1:compute.vm:ins-sh-2", Kind: "compute.vm"},
	}
}

func (*tencentFixture) PageLimit() int { return testPageSize }

func (f *tencentFixture) Scenario(name string) error {
	switch name {
	case "":
		f.mock.mu.Lock()
		f.mock.mode = ""
		f.mock.mu.Unlock()
		f.unreachable = false
	case contract.SignalRateLimited, contract.SignalPermissionDenied:
		f.mock.mu.Lock()
		f.mock.mode = name
		f.mock.mu.Unlock()
	case contract.SignalUnreachable:
		f.unreachable = true // connection refused (http://127.0.0.1:1)
	default:
		return fmt.Errorf("tencentFixture: unknown scenario %q", name)
	}
	return nil
}

func (f *tencentFixture) Connection() contract.ConnectionView {
	endpoint := f.mock.srv.URL
	if f.unreachable {
		endpoint = "http://127.0.0.1:1"
	}
	return contract.ConnectionView{
		ProviderType: ProviderName,
		Endpoint:     endpoint,
		Material:     map[string]string{"inventory": materialFor(testAccessKey, testSecretKey)},
		Config:       map[string]any{"regions": []any{testRegionA, testRegionB}},
	}
}

// T-B1 — tencent 어댑터가 k8s·fake와 동일한 contract harness를 통과한다
// (계획 §5 Phase B 검증 — 하네스 4단얫 GREEN·§23.4 Q1 3행).
func TestTencentAdapterPassesContractHarness(t *testing.T) {
	f := newTencentFixture(t)
	contracttest.RunContractSuite(t, f.adapter, f)
}

// --- TDD RED — 아래 테스트들이 실패하는 상태에서 구현을 시작한다. ---

// T-B2 — 자격 재질: Material["inventory"] JSON blob만이 자격 경로다(J2 —
// arch rule 2). 누락·오류 JSON·성분 누락 3경로 전부 에러이고, 에러 메시지에
// 자격 물질이 새지 않는다(보존 제약 3).
func TestCredentialMaterialRequired(t *testing.T) {
	cases := []struct {
		name     string
		material string
		wantErr  string
	}{
		{"missing key entirely", "", "missing credential material"},
		{"malformed json", "{not json", "credential material parse"},
		{"missing secretKey", `{"accessKey":"AKIDx"}`, "missing accessKey or secretKey"},
		{"missing accessKey", `{"secretKey":"x"}`, "missing accessKey or secretKey"},
		{"empty values", `{"accessKey":"","secretKey":""}`, "missing accessKey or secretKey"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveCredentials(contract.ConnectionView{
				Material: map[string]string{"inventory": tc.material},
			})
			if err == nil {
				t.Fatal("resolveCredentials succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tc.wantErr)
			}
			for _, secret := range []string{testSecretKey, testAccessKey} {
				if strings.Contains(err.Error(), secret) {
					t.Errorf("error message carries credential material %q: %q", secret, err.Error())
				}
			}
		})
	}
	// nil Material도 동일 에러.
	if _, err := resolveCredentials(contract.ConnectionView{}); err == nil {
		t.Error("nil Material accepted, want the missing-material error")
	}
}

// T-B3 — 리전 출처는 Connection.Config["regions"]뿐이다(판정 J3 — Tencent는
// 리전 열거 API 미사용·계정 리전 필수, legacy util/tencentcloud.go 동일). 빈
// 리전은 에러다.
func TestRegionsRequiredFromConfig(t *testing.T) {
	if _, err := regionsOf(contract.ConnectionView{}); err == nil {
		t.Fatal("nil Config accepted, want the no-regions error")
	}
	if _, err := regionsOf(contract.ConnectionView{Config: map[string]any{}}); err == nil {
		t.Fatal("empty regions accepted, want the no-regions error")
	}
	if _, err := regionsOf(contract.ConnectionView{Config: map[string]any{"regions": []any{}}}); err == nil {
		t.Fatal("empty regions list accepted, want the no-regions error")
	}
	regions, err := regionsOf(contract.ConnectionView{Config: map[string]any{
		"regions": []any{testRegionA, testRegionB},
	}})
	if err != nil {
		t.Fatalf("regionsOf: %v", err)
	}
	if len(regions) != 2 || regions[0] != testRegionA || regions[1] != testRegionB {
		t.Errorf("regions = %v, want [%s %s]", regions, testRegionA, testRegionB)
	}
	// whitespace trim + 빈 성분 스킵 (legacy strings.TrimSpace 관례 승계).
	regions, err = regionsOf(contract.ConnectionView{Config: map[string]any{
		"regions": []any{"  " + testRegionA + "  ", "", testRegionB},
	}})
	if err != nil {
		t.Fatalf("regionsOf(trim): %v", err)
	}
	if len(regions) != 2 || regions[0] != testRegionA {
		t.Errorf("trimmed regions = %v", regions)
	}
}

// T-B4 — 커서: "<region>|<offset>" 형식(계획 §3.1). 형식 위반·미설정 리전·
// 비수치 오프셋은 에러다.
func TestCursorValidation(t *testing.T) {
	regions := []string{testRegionA, testRegionB}
	if r, off, err := splitCursor(regions, ""); err != nil || r != testRegionA || off != 0 {
		t.Errorf("empty cursor = (%q, %d, %v), want (%q, 0, nil)", r, off, err, testRegionA)
	}
	if r, off, err := splitCursor(regions, testRegionB+"|40"); err != nil || r != testRegionB || off != 40 {
		t.Errorf("cursor = (%q, %d, %v), want (%q, 40, nil)", r, off, err, testRegionB)
	}
	for _, bad := range []string{testRegionA, testRegionA + "|x", "ap-tokyo|0", testRegionA + "|-1", testRegionA + "|0|9"} {
		if _, _, err := splitCursor(regions, bad); err == nil {
			t.Errorf("cursor %q accepted, want an error", bad)
		}
	}
}

// T-B5 — Discover의 리전×페이지 순회 실측: 시드 6개를 pageSize 4로 걷으면
// 페이지 2개(경계에서 리전 전환)·커서 1회로 전수 수집된다. 커서 직렬 재시작
// (중간 커서부터)도 동일 잔여 집합을 준다.
func TestDiscoverWalksRegionsAndPages(t *testing.T) {
	f := newTencentFixture(t)
	ctx := context.Background()

	first, err := f.adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Connection: f.Connection()})
	if err != nil {
		t.Fatalf("Discover(first): %v", err)
	}
	if len(first.Resources) != testPageSize {
		t.Fatalf("first page resources = %d, want %d", len(first.Resources), testPageSize)
	}
	wantNext := testRegionB + "|0"
	if first.NextCursor != wantNext {
		t.Fatalf("first NextCursor = %q, want %q", first.NextCursor, wantNext)
	}
	second, err := f.adapter.Discover(ctx, contract.DiscoverRequest{
		ContextID: 1, Cursor: first.NextCursor, Connection: f.Connection(),
	})
	if err != nil {
		t.Fatalf("Discover(second): %v", err)
	}
	if len(second.Resources) != 2 || second.NextCursor != "" {
		t.Fatalf("second page = (%d resources, cursor %q), want (2, \"\")", len(second.Resources), second.NextCursor)
	}
	got := map[string]bool{}
	for _, r := range append(append([]contract.DiscoveredResource{}, first.Resources...), second.Resources...) {
		got[r.ExternalURN] = true
	}
	for _, s := range f.Seed() {
		if !got[s.ExternalURN] {
			t.Errorf("seed URN %q never discovered", s.ExternalURN)
		}
	}
	if f.mock.calls != 2 {
		t.Errorf("mock calls = %d, want 2 (page 단위 API 호출 예산)", f.mock.calls)
	}
}

// T-B6 — 리전 순회 중간부터의 커서도 유효하다: 첫 리전이 비면(0 인스턴스)
// 즉시 다음 리전으로 넘어간다(빈 페이지 무한 루프 금지 — 하네스 maxPages 보호).
func TestDiscoverSkipsEmptyRegion(t *testing.T) {
	mock := newMockCVM(t)
	mock.mu.Lock()
	mock.instances = map[string][]cvmInstance{
		testRegionA: seedInstances()[:4],
		testRegionB: nil, // sh 리전은 0개가 된다
	}
	mock.mu.Unlock()
	adapter := NewAdapter(WithPageSize(2))
	page, err := adapter.Discover(context.Background(), contract.DiscoverRequest{
		ContextID: 3,
		Connection: contract.ConnectionView{
			Endpoint: mock.srv.URL,
			Material: map[string]string{"inventory": materialFor(testAccessKey, testSecretKey)},
			Config:   map[string]any{"regions": []any{testRegionA, testRegionB}},
		},
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(page.Resources) != 2 || page.NextCursor == "" {
		t.Fatalf("page = (%d, cursor %q), want (2, non-empty)", len(page.Resources), page.NextCursor)
	}
	// 잔여 4개(gz 2 + 빈 sh 리전 통과)를 마저 걷는다 — 종결 커서까지. 빈 마지막
	// 리전은 빈 종결 페이지를 낼 수 있다(하네스 maxPages가 흡수하는 형태).
	total := len(page.Resources)
	seen := map[string]bool{}
	for _, r := range page.Resources {
		seen[r.ExternalURN] = true
	}
	cursor := page.NextCursor
	for i := 0; ; i++ {
		if i > 10 {
			t.Fatal("discovery did not terminate within 10 pages")
		}
		page, err = adapter.Discover(context.Background(), contract.DiscoverRequest{
			ContextID: 3, Cursor: cursor,
			Connection: contract.ConnectionView{
				Endpoint: mock.srv.URL,
				Material: map[string]string{"inventory": materialFor(testAccessKey, testSecretKey)},
				Config:   map[string]any{"regions": []any{testRegionA, testRegionB}},
			},
		})
		if err != nil {
			t.Fatalf("Discover(continue): %v", err)
		}
		total += len(page.Resources)
		for _, r := range page.Resources {
			seen[r.ExternalURN] = true
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if total != 4 || len(seen) != 4 {
		t.Fatalf("walk collected %d resources (%d unique), want 4 — gz 전수·sh 공리전", total, len(seen))
	}
}

// T-B7 — 일반(비신호) API 실패: HTTP 500 + InternalError는 신호 어휘 밖의
// 일반 error로 남는다(계약 §9.3 — 신호 3종 외 일반 error). 계측은
// provider_api_errors_total로 기록된다.
func TestDiscoverSurfacesGenericAPIError(t *testing.T) {
	mock := newMockCVM(t)
	mock.mu.Lock()
	mock.mode = "internal_error"
	mock.mu.Unlock()
	counters := metrics.New()
	adapter := NewAdapter(WithCounters(counters))
	_, err := adapter.Discover(context.Background(), contract.DiscoverRequest{
		ContextID: 1,
		Connection: contract.ConnectionView{
			Endpoint: mock.srv.URL,
			Material: map[string]string{"inventory": materialFor(testAccessKey, testSecretKey)},
			Config:   map[string]any{"regions": []any{testRegionA}},
		},
	})
	if err == nil {
		t.Fatal("Discover succeeded against a failing mock, want an error")
	}
	var sig *contract.ProviderSignalError
	if errors.As(err, &sig) {
		t.Fatalf("InternalError surfaced as signal %q — it must stay a generic error", sig.Kind)
	}
	render := counters.Render()
	if !strings.Contains(render, `provider_api_errors_total{provider="tencent",op="discover",code="InternalError"}`) {
		t.Errorf("render missing the InternalError api-error line:\n%s", render)
	}
}

// T-B8 — 서명 검증기의 반증 가능성: 틀린 secretKey로 서명한 요청은 모의 서버가
// 401로 거부한다(검증기가 도장 찍기가 아님을 증명 — R2 완화).
func TestMockRejectsWrongSignature(t *testing.T) {
	mock := newMockCVM(t)
	adapter := NewAdapter()
	_, err := adapter.Discover(context.Background(), contract.DiscoverRequest{
		ContextID: 1,
		Connection: contract.ConnectionView{
			Endpoint: mock.srv.URL,
			Material: map[string]string{"inventory": materialFor(testAccessKey, "deliberately-wrong-signing-key-000000")},
			Config:   map[string]any{"regions": []any{testRegionA}},
		},
	})
	var sig *contract.ProviderSignalError
	if !errors.As(err, &sig) || sig.Kind != contract.SignalPermissionDenied {
		t.Fatalf("wrong-key request = %v, want permission_denied signal (mock 401)", err)
	}
}

// T-B9 — 타임스탬프 스큐: X-TC-Timestamp가 5분 규약 밖이면 모의 서버가
// 거부한다(시계 규약 검증 — 계획 §3.1 mock 경로 "canonical 인코딩·시계").
func TestMockRejectsStaleTimestamp(t *testing.T) {
	mock := newMockCVM(t)
	stale := time.Now().Add(-15 * time.Minute).Unix()
	err := mockSignedDiscover(mock, stale)
	if err == nil {
		t.Fatal("stale-timestamp request accepted, want an error")
	}
	var sig *contract.ProviderSignalError
	if !errors.As(err, &sig) || sig.Kind != contract.SignalPermissionDenied {
		t.Fatalf("stale timestamp = %v, want permission_denied signal (mock 401)", err)
	}
}

// mockSignedDiscover — 지정 타임스탬프로 서명한 DescribeInstances 1회(스큐
// 음성 제어 전용).
func mockSignedDiscover(mock *mockCVM, timestamp int64) error {
	client := &cvmClient{
		http:       &http.Client{Timeout: requestTimeout},
		credential: credential{accessKey: testAccessKey, secretKey: testSecretKey},
		endpoint:   mock.srv.URL,
		region:     testRegionA,
		metrics:    metrics.New(),
		timestamp:  func() int64 { return timestamp },
	}
	_, err := client.describeInstances(context.Background(), 0, 1)
	return err
}

// T-B10 — 정규화: §3.2 스키마 전필드 + URN(가정 A2) + Subtype 패밀리 파생.
func TestNormalizerBuildsURNAndNormalizedFields(t *testing.T) {
	res, err := normalizeInstance(7, testRegionA, cvmInstance{
		InstanceId: sp("ins-abc"), InstanceName: sp("web"),
		InstanceType: sp("S5.LARGE8"), InstanceState: sp("RUNNING"),
		Cpu: ip(8), Memory: ip(8), OsName: sp("CentOS 7.6 64bit"),
		Placement:          &cvmPlacement{Zone: sp("ap-guangzhou-1"), Region: sp(testRegionA)},
		PrivateIpAddresses: []*string{sp("10.0.0.1"), sp("10.0.0.9")},
		PublicIpAddresses:  []*string{sp("1.2.3.4")},
		SystemDisk:         &cvmDisk{DiskSize: ip(20)},
		DataDisks:          []cvmDisk{{DiskSize: ip(40)}, {}},
	})
	if err != nil {
		t.Fatalf("normalizeInstance: %v", err)
	}
	if res.ExternalURN != "urn:tencent:7:compute.vm:ins-abc" {
		t.Errorf("URN = %q, want urn:tencent:7:compute.vm:ins-abc (A2)", res.ExternalURN)
	}
	if res.ExternalID != "ins-abc" || res.Kind != KindVM {
		t.Errorf("id/kind = %q/%q", res.ExternalID, res.Kind)
	}
	if res.Subtype != "S5" {
		t.Errorf("Subtype = %q, want S5 (instance family from InstanceType prefix)", res.Subtype)
	}
	want := contract.JSONMap{
		"displayName":  "web",
		"region":       testRegionA,
		"zone":         "ap-guangzhou-1",
		"cpu":          float64(8),
		"memoryGB":     float64(8),
		"diskGB":       float64(60), // system 20 + data 40 (nil DiskSize 스킵)
		"os":           "CentOS 7.6 64bit",
		"privateIps":   []string{"10.0.0.1", "10.0.0.9"},
		"publicIps":    []string{"1.2.3.4"},
		"instanceType": "S5.LARGE8",
		"status":       "RUNNING",
	}
	for k, v := range want {
		if fmt.Sprint(res.Normalized[k]) != fmt.Sprint(v) {
			t.Errorf("normalized[%q] = %v, want %v", k, res.Normalized[k], v)
		}
	}
	hint, ok := res.Normalized["sshHint"].(contract.JSONMap)
	if !ok || hint["user"] != "root" || hint["port"] != 22 {
		t.Errorf("sshHint = %v, want {user:root, port:22} (§3.2 — 프로바이더 기본 힌트)", res.Normalized["sshHint"])
	}
}

// T-B11 — nil-safe(R8): 포인터 필드 전무 인스턴스도 파닉 없이 정규화된다.
// Placement 부재 시 리전은 호출자가 넘긴 계정 리전으로 채운다.
func TestNormalizerNilSafe(t *testing.T) {
	res, err := normalizeInstance(1, testRegionA, cvmInstance{InstanceId: sp("ins-nil")})
	if err != nil {
		t.Fatalf("normalizeInstance: %v", err)
	}
	if res.ExternalURN != "urn:tencent:1:compute.vm:ins-nil" {
		t.Errorf("URN = %q", res.ExternalURN)
	}
	if res.Subtype != "" {
		t.Errorf("Subtype = %q, want \"\" (no InstanceType)", res.Subtype)
	}
	if res.Normalized["region"] != testRegionA {
		t.Errorf("region = %v, want the caller's account region %q", res.Normalized["region"], testRegionA)
	}
	if res.Normalized["displayName"] != "ins-nil" {
		t.Errorf("displayName = %v, want the id fallback", res.Normalized["displayName"])
	}
	for _, k := range []string{"cpu", "memoryGB", "diskGB", "os", "zone", "instanceType", "status"} {
		if v, ok := res.Normalized[k]; ok && fmt.Sprint(v) != "" && fmt.Sprint(v) != "0" && fmt.Sprint(v) != "[]" {
			t.Errorf("normalized[%q] = %v, want empty/zero for an instance without the field", k, v)
		}
	}
	if _, ok := res.Normalized["privateIps"].([]string); !ok || len(res.Normalized["privateIps"].([]string)) != 0 {
		t.Errorf("privateIps = %v, want an empty list", res.Normalized["privateIps"])
	}
	if _, err := normalizeInstance(1, testRegionA, cvmInstance{}); err == nil {
		t.Error("instance without InstanceId accepted — identity는 URN 필수 성분이다")
	}
}

// T-B12 — Raw 상한(§8.2 — 64KiB는 계획 도입값 A12): 초과분 절단 +
// normalized.truncated 마커.
func TestRawSizeLimitTruncates(t *testing.T) {
	big := strings.Repeat("x", 100<<10)
	res, err := normalizeInstance(1, testRegionA, cvmInstance{
		InstanceId: sp("ins-big"), InstanceName: sp(big),
	})
	if err != nil {
		t.Fatalf("normalizeInstance: %v", err)
	}
	rawJSON := marshalForScan(t, res.Raw)
	if len(rawJSON) > MaxRawBytes {
		t.Errorf("Raw = %d bytes, exceeds MaxRawBytes %d", len(rawJSON), MaxRawBytes)
	}
	if res.Normalized["truncated"] != true {
		t.Errorf("normalized.truncated = %v, want the truncation marker", res.Normalized["truncated"])
	}
}

// T-B13 — 리댁션(보존 제약 3): 시드에 심은 마커(Tags 값)는 Raw·Normalized 어디에도
// 등장하지 않는다(하네스 단얫 4의 tencent측 구체 사례 — 매핑 표 §4 "tag 값 폐기").
func TestTagValuesNeverSurface(t *testing.T) {
	raw := map[string]any{
		"InstanceId": "ins-tag", "InstanceName": "tagged",
		"Tags": []map[string]string{{"Key": "secret", "Value": contracttest.MarkerValue}},
	}
	blob, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var inst cvmInstance
	if err := json.Unmarshal(blob, &inst); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	res, err := normalizeInstance(2, testRegionA, inst)
	if err != nil {
		t.Fatalf("normalizeInstance: %v", err)
	}
	for name, tree := range map[string]contract.JSONMap{"Raw": res.Raw, "Normalized": res.Normalized} {
		if strings.Contains(marshalForScan(t, tree), contracttest.MarkerValue) {
			t.Errorf("%s surfaces the tag value marker: %s", name, marshalForScan(t, tree))
		}
	}
}

// T-B14 — Validate/Health(계획 §3.1 — DescribeInstances 1리전 프로브). 메시지에
// 리전 수는 있고 자격 물질은 없다.
func TestValidateAndHealth(t *testing.T) {
	f := newTencentFixture(t)
	ctx := context.Background()

	if err := f.adapter.Validate(ctx, f.Connection()); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	h := f.adapter.Health(ctx, f.Connection())
	if !h.Healthy {
		t.Fatalf("Health = %+v, want healthy", h)
	}
	if !strings.Contains(h.Message, "2 region") {
		t.Errorf("Health message %q does not report the region count", h.Message)
	}
	if strings.Contains(h.Message, testSecretKey) || strings.Contains(h.Message, testAccessKey) {
		t.Errorf("Health message carries credential material: %q", h.Message)
	}

	// 자격 없는 계정은 Validate가 실패한다.
	broken := f.Connection()
	broken.Material = map[string]string{}
	if err := f.adapter.Validate(ctx, broken); err == nil {
		t.Error("Validate accepted a connection without credential material")
	}
	// 도달 불능 엔드포인트는 unreachable 신호로 분류된다.
	broken = f.Connection()
	broken.Endpoint = "http://127.0.0.1:1"
	err := f.adapter.Validate(ctx, broken)
	var sig *contract.ProviderSignalError
	if !errors.As(err, &sig) || sig.Kind != contract.SignalUnreachable {
		t.Fatalf("Validate against a dead endpoint = %v, want unreachable signal", err)
	}
}

// T-B15 — 엔드포인트 해석: Connection.Endpoint 우선, 빈 값이면 프로바이더
// 기본(계획 §3.1). WithEndpointOverride가 최우선이다.
func TestEndpointResolution(t *testing.T) {
	if got := normalizeEndpoint(""); got != "https://cvm.tencentcloudapi.com" {
		t.Errorf("normalizeEndpoint(\"\") = %q, want the provider default", got)
	}
	if got := normalizeEndpoint("cvm.tencentcloudapi.com"); got != "https://cvm.tencentcloudapi.com" {
		t.Errorf("schemeless endpoint = %q, want https:// prefixed", got)
	}
	if got := normalizeEndpoint("http://127.0.0.1:1234"); got != "http://127.0.0.1:1234" {
		t.Errorf("mock endpoint mutated: %q", got)
	}
	override := "http://127.0.0.1:9999"
	adapter := NewAdapter(WithEndpointOverride(func() string { return override }))
	conn := contract.ConnectionView{Endpoint: "http://127.0.0.1:1234"}
	if got := adapter.endpointFor(conn); got != override {
		t.Errorf("override not honored: endpoint = %q, want %q", got, override)
	}
	emptyOverride := NewAdapter(WithEndpointOverride(func() string { return "" }))
	if got := emptyOverride.endpointFor(conn); got != "http://127.0.0.1:1234" {
		t.Errorf("empty override must fall back to Connection.Endpoint, got %q", got)
	}
	if got := NewAdapter().endpointFor(contract.ConnectionView{}); got != "https://cvm.tencentcloudapi.com" {
		t.Errorf("default endpoint = %q", got)
	}
}

// T-B16 — 컴파일 타임 계약(§11): BaseAdapter + Discoverer 동시 충족.
func TestInterfaceConformance(t *testing.T) {
	var _ contract.BaseAdapter = (*Adapter)(nil)
	var _ contract.Discoverer = (*Adapter)(nil)
	d := NewAdapter()
	if d.Descriptor().Type != ProviderName {
		t.Errorf("descriptor type = %q, want %q", d.Descriptor().Type, ProviderName)
	}
	if got := d.Descriptor().ContextKinds; len(got) != 1 || got[0] != "account" {
		t.Errorf("ContextKinds = %v, want [account] (§7.3)", got)
	}
	if !d.Descriptor().BuiltIn {
		t.Error("descriptor must be BuiltIn")
	}
	if got := d.ProviderName(); got != ProviderName {
		t.Errorf("ProviderName() = %q, want %q", got, ProviderName)
	}
	if d.RateCounter() == nil {
		t.Error("RateCounter() returned nil — the harness rate assertion needs it")
	}
	if err := d.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func marshalForScan(t *testing.T, m contract.JSONMap) string {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal scan tree: %v", err)
	}
	return string(b)
}
