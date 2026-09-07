package aliyun

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

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/contracttest"
	"ops-admin/backend/internal/infra/metrics"
)

// --- 모의 Aliyun ECS RPC 서버 (§23.2 정신 — 실계정 의존 없는 httptest, 판정
// J1 Path M) ---
//
// legacy service/asset_cloud_aliyun.go의 raw RPC 응답 구조를 재현한다:
// DescribeRegions {Regions:{Region:[{RegionId}]}} + DescribeInstances
// {TotalCount, Instances:{Instance:[…]}}. 핸들러가 서명 파라미터를 재검증한다
// (HMAC-SHA1 왕복 — §6 "client 서명 왕복 — httptest가 재서명 검증까지 겸").

const (
	mockAccessKey = "mock-aliyun-ak-0001"
	mockSecretKey = "mock-aliyun-sk-0001"
)

func credentialBlob() string {
	return fmt.Sprintf(`{"accessKey":%q,"secretKey":%q}`, mockAccessKey, mockSecretKey)
}

type mockAliyun struct {
	t           *testing.T
	srv         *httptest.Server
	mu          sync.Mutex
	mode        string // "" nominal | rate_limited | permission_denied
	regions     []string
	instances   map[string][]map[string]any
	calls       int
	regionCalls int
	sigFails    int
	lastAction  string
}

func newMockAliyun(t *testing.T) *mockAliyun {
	m := &mockAliyun{
		t:         t,
		regions:   []string{"cn-mock-a", "cn-mock-b"},
		instances: map[string][]map[string]any{},
	}
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.srv.Close)
	return m
}

func writeRPCError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"Code": code, "Message": message})
}

func (m *mockAliyun) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++

	q := r.URL.Query()
	m.lastAction = q.Get("Action")

	switch m.mode {
	case contract.SignalRateLimited:
		writeRPCError(w, http.StatusTooManyRequests, "Throttling", "Request rate is too high")
		return
	case contract.SignalPermissionDenied:
		writeRPCError(w, http.StatusForbidden, "ForbiddenAccess", "The required operation is forbidden")
		return
	}

	// 서명 재검증 — Signature를 뺀 나머지 파라미터로 동일 규약 재계산.
	// 불일치는 aliyun 실제 규약과 같은 SignatureDoesNotMatch 코드로 응답한다.
	recomputed := signParams(withoutSignature(q), mockSecretKey)
	if recomputed != q.Get("Signature") {
		m.sigFails++
		writeRPCError(w, http.StatusBadRequest, "SignatureDoesNotMatch", "The signature does not match")
		return
	}

	switch q.Get("Action") {
	case "DescribeRegions":
		m.regionCalls++
		regionList := make([]map[string]string, 0, len(m.regions))
		for _, id := range m.regions {
			regionList = append(regionList, map[string]string{"RegionId": id})
		}
		writeJSON(w, map[string]any{"Regions": map[string]any{"Region": regionList}})
	case "DescribeInstances":
		m.serveInstances(w, q)
	default:
		writeRPCError(w, http.StatusBadRequest, "InvalidAction.NotFound", "unknown action")
	}
}

func (m *mockAliyun) serveInstances(w http.ResponseWriter, q url.Values) {
	items := m.instances[q.Get("RegionId")]
	page, _ := strconv.Atoi(q.Get("PageNumber"))
	size, _ := strconv.Atoi(q.Get("PageSize"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 100
	}
	start := (page - 1) * size
	end := start + size
	if start > len(items) {
		start = len(items)
	}
	if end > len(items) {
		end = len(items)
	}
	writeJSON(w, map[string]any{
		"TotalCount": len(items),
		"Instances":  map[string]any{"Instance": items[start:end]},
	})
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	// Encode failure can only come from a broken handler — the test client
	// would surface it as a truncated body.
	_ = json.NewEncoder(w).Encode(payload)
}

func withoutSignature(q url.Values) url.Values {
	out := url.Values{}
	for key, values := range q {
		if key == "Signature" {
			continue
		}
		for _, v := range values {
			out.Add(key, v)
		}
	}
	return out
}

// instance builds one ECS instance response object. The redaction canary is
// planted in a field the normalizer never reads (Description) — assertion 4
// fails if any Raw/Normalized tree surfaces it.
func instance(id string, fields map[string]any) map[string]any {
	obj := map[string]any{
		"InstanceId":   id,
		"InstanceName": "inst-" + id,
		"Description":  contracttest.MarkerValue,
		"Status":       "Running",
	}
	for k, v := range fields {
		obj[k] = v
	}
	return obj
}

func (m *mockAliyun) seedNominal() {
	m.instances["cn-mock-a"] = []map[string]any{
		instance("i-mock-1", map[string]any{
			"InstanceName": "web-1", "Cpu": 2, "Memory": 4096, "OSName": "Alibaba Cloud Linux",
			"RegionId": "cn-mock-a", "ZoneId": "cn-mock-a-a", "InstanceType": "ecs.g6.large",
			"PublicIpAddress": map[string]any{"IpAddress": []string{"47.1.2.3"}},
			"VpcAttributes":   map[string]any{"PrivateIpAddress": map[string]any{"IpAddress": []string{"10.0.0.1"}}},
			"SystemDisk":      map[string]any{"Size": 40},
			"DataDisks":       map[string]any{"Disk": []map[string]any{{"Size": 100}}},
		}),
		instance("i-mock-2", map[string]any{
			"Cpu": 8, "Memory": 16384, "OSName": "Ubuntu 22.04",
			"RegionId": "cn-mock-a", "InstanceType": "ecs.c7.xlarge",
			"InnerIpAddress": map[string]any{"IpAddress": []string{"10.0.0.2"}},
			"EipAddress":     map[string]any{"IpAddress": "47.1.2.4"},
			"NetworkInterfaces": map[string]any{"NetworkInterface": []map[string]any{
				{"PrimaryIpAddress": "10.0.0.9"},
			}},
		}),
		instance("i-mock-3", map[string]any{}), // nil-safe minimal shape
		instance("i-mock-4", map[string]any{"Cpu": 4, "Memory": 8192, "RegionId": "cn-mock-a"}),
		instance("i-mock-5", map[string]any{"Cpu": 1, "Memory": 1024, "RegionId": "cn-mock-a"}),
	}
	m.instances["cn-mock-b"] = []map[string]any{
		instance("i-mock-6", map[string]any{"Cpu": 2, "Memory": 2048, "RegionId": "cn-mock-b"}),
		instance("i-mock-7", map[string]any{"Cpu": 16, "Memory": 32768, "RegionId": "cn-mock-b"}),
		instance("i-mock-8", map[string]any{"Cpu": 2, "Memory": 4096, "RegionId": "cn-mock-b"}),
	}
}

// --- 하네스 Fixture — 모의 서버를 시나리오·자격·엔드포인트 주입면으로 감싼다. ---

const testPageSize = 4

type aliyunFixture struct {
	t        *testing.T
	mock     *mockAliyun
	adapter  *Adapter
	endpoint string
}

func newAliyunFixture(t *testing.T) *aliyunFixture {
	mock := newMockAliyun(t)
	mock.seedNominal()
	adapter := NewAdapter(WithPageSize(testPageSize))
	return &aliyunFixture{t: t, mock: mock, adapter: adapter, endpoint: mock.srv.URL}
}

func (f *aliyunFixture) Seed() []contract.DiscoveredResource {
	// Independent oracle: hand-written expected URNs/kinds — not produced by
	// the normalizer (self-fulfilling 방지).
	return []contract.DiscoveredResource{
		{ExternalURN: "urn:aliyun:1:compute.vm:i-mock-1", Kind: "compute.vm"},
		{ExternalURN: "urn:aliyun:1:compute.vm:i-mock-2", Kind: "compute.vm"},
		{ExternalURN: "urn:aliyun:1:compute.vm:i-mock-3", Kind: "compute.vm"},
		{ExternalURN: "urn:aliyun:1:compute.vm:i-mock-4", Kind: "compute.vm"},
		{ExternalURN: "urn:aliyun:1:compute.vm:i-mock-5", Kind: "compute.vm"},
		{ExternalURN: "urn:aliyun:1:compute.vm:i-mock-6", Kind: "compute.vm"},
		{ExternalURN: "urn:aliyun:1:compute.vm:i-mock-7", Kind: "compute.vm"},
		{ExternalURN: "urn:aliyun:1:compute.vm:i-mock-8", Kind: "compute.vm"},
	}
}

func (*aliyunFixture) PageLimit() int { return testPageSize }

func (f *aliyunFixture) Scenario(name string) error {
	f.mock.mu.Lock()
	defer f.mock.mu.Unlock()
	switch name {
	case "":
		f.mock.mode = ""
		f.endpoint = f.mock.srv.URL
	case contract.SignalRateLimited, contract.SignalPermissionDenied:
		f.mock.mode = name
	case contract.SignalUnreachable:
		f.mock.mode = ""
		f.endpoint = "http://127.0.0.1:1" // connection refused
	default:
		return fmt.Errorf("aliyunFixture: unknown scenario %q", name)
	}
	return nil
}

func (f *aliyunFixture) Connection() contract.ConnectionView {
	return contract.ConnectionView{
		ProviderType: "aliyun",
		Endpoint:     f.endpoint,
		Config:       contract.JSONMap{"regions": []string{"cn-mock-a", "cn-mock-b"}},
		Material:     map[string]string{"inventory": credentialBlob()},
	}
}

// T47(계획 §6) — aliyun 어댑터가 contract harness 4단얫(§23.1)을 통과한다.
func TestAliyunAdapterPassesContractHarness(t *testing.T) {
	f := newAliyunFixture(t)
	contracttest.RunContractSuite(t, f.adapter, f)
}

// Descriptor 계약(§3.1) — Type/ContextKinds/BuiltIn 고정값.
func TestAliyunDescriptor(t *testing.T) {
	d := NewAdapter().Descriptor()
	if d.Type != "aliyun" || d.AdapterVersion != "1" || d.ProtocolVersion != "1" {
		t.Errorf("Descriptor = %+v", d)
	}
	if len(d.ContextKinds) != 1 || d.ContextKinds[0] != "account" || !d.BuiltIn {
		t.Errorf("Descriptor ContextKinds/BuiltIn = %+v/%v", d.ContextKinds, d.BuiltIn)
	}
}

// 자격 물질 경로(J2 — 어댑터는 req.Connection만 본다): Material 부재·불량 JSON·
// 빈 성분은 모두 명시적 오류다. 서명 불일치는 mock이 실제 규약 코드로 응답하고
// permission_denied 신호로 분류된다.
func TestAliyunCredentialAndSignalPaths(t *testing.T) {
	f := newAliyunFixture(t)
	ctx := context.Background()

	t.Run("missing_material", func(t *testing.T) {
		_, err := f.adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1})
		if err == nil || !strings.Contains(err.Error(), "inventory") {
			t.Fatalf("Discover without Material[inventory]: err = %v", err)
		}
	})

	t.Run("malformed_material", func(t *testing.T) {
		conn := contract.ConnectionView{Material: map[string]string{"inventory": "{not-json"}}
		if _, err := f.adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Connection: conn}); err == nil {
			t.Fatalf("Discover with malformed credential blob succeeded")
		}
	})

	t.Run("empty_component", func(t *testing.T) {
		conn := contract.ConnectionView{Material: map[string]string{"inventory": `{"accessKey":"a","secretKey":""}`}}
		if _, err := f.adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Connection: conn}); err == nil {
			t.Fatalf("Discover with an empty secretKey succeeded")
		}
	})

	t.Run("signature_mismatch_signals_permission_denied", func(t *testing.T) {
		conn := f.Connection()
		conn.Material = map[string]string{"inventory": `{"accessKey":"mock-aliyun-ak-0001","secretKey":"wrong-secret"}`}
		_, err := f.adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Connection: conn})
		var sig *contract.ProviderSignalError
		if !errors.As(err, &sig) || sig.Kind != contract.SignalPermissionDenied {
			t.Fatalf("signature mismatch err = %v, want permission_denied signal", err)
		}
	})

	t.Run("error_messages_carry_no_credential_material", func(t *testing.T) {
		// 실패 경로(신호·커서·전송)의 에러 문자열은 자격 값을 인용하지 않는다
		// (보존 제약 3).
		probes := []string{mockAccessKey, mockSecretKey}
		f.mock.mu.Lock()
		f.endpoint = "http://127.0.0.1:1" // unreachable transport path
		f.mock.mu.Unlock()
		_, err := f.adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Connection: f.Connection()})
		if err == nil {
			t.Fatal("unreachable Discover succeeded")
		}
		f.mock.mu.Lock()
		f.endpoint = f.mock.srv.URL
		f.mock.mu.Unlock()
		_, cursorErr := f.adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Cursor: "bogus", Connection: f.Connection()})
		if cursorErr == nil {
			t.Fatal("malformed cursor Discover succeeded")
		}
		for _, failure := range []error{err, cursorErr} {
			for _, probe := range probes {
				if strings.Contains(failure.Error(), probe) {
					t.Errorf("error message leaks credential material %q: %v", probe, failure)
				}
			}
		}
	})
}

// 커서 규약(J3): "<region>|<pageNumber>" — 불량 커서는 명시적 오류다.
func TestAliyunCursorContract(t *testing.T) {
	f := newAliyunFixture(t)
	ctx := context.Background()
	conn := f.Connection()

	for _, cursor := range []string{"bogus", "cn-mock-a", "unknown-region|1", "cn-mock-a|zero"} {
		if _, err := f.adapter.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Cursor: cursor, Connection: conn}); err == nil {
			t.Errorf("cursor %q: Discover succeeded, want a malformed/unknown cursor error", cursor)
		}
	}
}

// 리전 출처(J3): Config.regions가 비면 DescribeRegions로 열거한다 —
// legacy fetchAliyunCloudInstances와 동일 폴백.
func TestAliyunRegionsFallbackToDescribeRegions(t *testing.T) {
	f := newAliyunFixture(t)
	conn := f.Connection()
	conn.Config = contract.JSONMap{} // no regions

	cursor := ""
	pages := 0
	seen := map[string]bool{}
	for {
		page, err := f.adapter.Discover(context.Background(), contract.DiscoverRequest{ContextID: 1, Cursor: cursor, Connection: conn})
		if err != nil {
			t.Fatalf("Discover: %v", err)
		}
		for _, r := range page.Resources {
			seen[r.ExternalURN] = true
		}
		pages++
		if page.NextCursor == "" {
			break
		}
		if pages > 16 {
			t.Fatal("discovery did not terminate")
		}
		cursor = page.NextCursor
	}
	if len(seen) != 8 {
		t.Fatalf("discovered %d resources, want 8", len(seen))
	}
	if f.mock.regionCalls == 0 {
		t.Fatal("DescribeRegions was never called — region fallback broken")
	}
}

// CSV·[]any 형태의 Config.regions도 수용한다(계정 설정 직렬화 여유).
func TestAliyunRegionsConfigShapes(t *testing.T) {
	cases := map[string]contract.JSONMap{
		"slice":   {"regions": []string{"cn-mock-a"}},
		"any":     {"regions": []any{"cn-mock-a"}},
		"csv":     {"regions": "cn-mock-a"},
		"invalid": {"regions": 42},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			f := newAliyunFixture(t)
			conn := f.Connection()
			conn.Config = cfg
			page, err := f.adapter.Discover(context.Background(), contract.DiscoverRequest{ContextID: 1, Connection: conn})
			if err != nil {
				t.Fatalf("Discover: %v", err)
			}
			if name == "invalid" {
				// 인식 불가 형태는 DescribeRegions 폴백으로 귀결된다(J3) —
				// 오류가 아니라 전 리전 열거.
				if len(page.Resources) != 4 || page.NextCursor == "" {
					t.Fatalf("invalid regions config fell back wrong: %d resources, cursor %q", len(page.Resources), page.NextCursor)
				}
				if f.mock.regionCalls == 0 {
					t.Error("DescribeRegions fallback never ran")
				}
				return
			}
			if len(page.Resources) != 4 {
				t.Errorf("page resources = %d, want 4 (region cn-mock-a seed)", len(page.Resources))
			}
			if page.NextCursor == "" {
				t.Error("NextCursor empty — region cn-mock-b must remain")
			}
		})
	}
}

// Validate/Health — DescribeRegions 스모크(§3.1). Health 메시지는 계정 식별
// (마스킹)·리전 수를 담되 자격 물질은 담지 않는다.
func TestAliyunValidateAndHealth(t *testing.T) {
	f := newAliyunFixture(t)
	ctx := context.Background()
	conn := f.Connection()

	if err := f.adapter.Validate(ctx, conn); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	h := f.adapter.Health(ctx, conn)
	if !h.Healthy {
		t.Fatalf("Health = %+v, want healthy", h)
	}
	if !strings.Contains(h.Message, "2 regions") {
		t.Errorf("Health message %q does not report the region count", h.Message)
	}
	if strings.Contains(h.Message, mockSecretKey) {
		t.Errorf("Health message leaks the secret material: %q", h.Message)
	}

	if err := f.Scenario(contract.SignalRateLimited); err != nil {
		t.Fatalf("Scenario: %v", err)
	}
	defer func() { _ = f.Scenario("") }()
	err := f.adapter.Validate(ctx, f.Connection())
	var sig *contract.ProviderSignalError
	if !errors.As(err, &sig) || sig.Kind != contract.SignalRateLimited {
		t.Fatalf("Validate(429) err = %v, want rate_limited signal", err)
	}
}

// 정규화 단위(§14.2 매핑 — mapping.md와 1:1): URN·kind/subtype·용량 수학·
// nil-safe 필드.
func TestAliyunNormalizerMapsInstance(t *testing.T) {
	full := instance("i-full", map[string]any{
		"InstanceName": "web-1", "Cpu": 2, "Memory": 4096, "OSName": "Alibaba Cloud Linux",
		"RegionId": "cn-mock-a", "ZoneId": "cn-mock-a-a", "Status": "Running", "InstanceType": "ecs.g6.large",
		"PublicIpAddress": map[string]any{"IpAddress": []string{"47.1.2.3", "47.1.2.30"}},
		"VpcAttributes":   map[string]any{"PrivateIpAddress": map[string]any{"IpAddress": []string{"10.0.0.1"}}},
		"SystemDisk":      map[string]any{"Size": 40},
		"DataDisks":       map[string]any{"Disk": []map[string]any{{"Size": 100}, {"Size": 60}}},
	})
	b, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	res, err := normalizeInstance(7, b, "cn-mock-a")
	if err != nil {
		t.Fatalf("normalizeInstance: %v", err)
	}
	if res.ExternalID != "i-full" || res.ExternalURN != "urn:aliyun:7:compute.vm:i-full" {
		t.Errorf("identity = %q/%q", res.ExternalID, res.ExternalURN)
	}
	if res.Kind != "compute.vm" || res.Subtype != "g6" {
		t.Errorf("Kind/Subtype = %q/%q, want compute.vm/g6", res.Kind, res.Subtype)
	}
	if res.DisplayName != "web-1" {
		t.Errorf("DisplayName = %q", res.DisplayName)
	}
	n := res.Normalized
	if n["region"] != "cn-mock-a" || n["zone"] != "cn-mock-a-a" || n["status"] != "Running" {
		t.Errorf("region/zone/status = %v/%v/%v", n["region"], n["zone"], n["status"])
	}
	if n["cpu"] != 2 || n["memoryGB"] != 4.0 || n["diskGB"] != 200 {
		t.Errorf("cpu/memoryGB/diskGB = %v/%v/%v, want 2/4/200", n["cpu"], n["memoryGB"], n["diskGB"])
	}
	if n["os"] != "Alibaba Cloud Linux" || n["instanceType"] != "ecs.g6.large" {
		t.Errorf("os/instanceType = %v/%v", n["os"], n["instanceType"])
	}
	ips := func(key string) []string {
		got, _ := n[key].([]string)
		return got
	}
	if got := ips("privateIps"); len(got) != 1 || got[0] != "10.0.0.1" {
		t.Errorf("privateIps = %v, want [10.0.0.1]", got)
	}
	if got := ips("publicIps"); len(got) != 2 {
		t.Errorf("publicIps = %v, want 2 entries", got)
	}
	hint, ok := n["sshHint"].(map[string]any)
	if !ok || hint["user"] != "root" || hint["port"] != 22 {
		t.Errorf("sshHint = %v, want {user:root,port:22} (provider default hint — legacy SSHUser/SSHPort는 dropped)", n["sshHint"])
	}
	if strings.Contains(marshalForScan(t, res.Raw)+marshalForScan(t, res.Normalized), contracttest.MarkerValue) {
		t.Error("the Description canary leaked into the payload trees")
	}

	t.Run("nil_safe_minimal", func(t *testing.T) {
		res, err := normalizeInstance(7, []byte(`{"InstanceId":"i-min"}`), "cn-mock-b")
		if err != nil {
			t.Fatalf("normalizeInstance(minimal): %v", err)
		}
		if res.ExternalURN != "urn:aliyun:7:compute.vm:i-min" {
			t.Errorf("URN = %q", res.ExternalURN)
		}
		if res.Subtype != "" {
			t.Errorf("Subtype = %q, want empty (no instance family)", res.Subtype)
		}
		if res.DisplayName != "i-min" {
			t.Errorf("DisplayName = %q, want the id fallback", res.DisplayName)
		}
		if res.Normalized["region"] != "cn-mock-b" {
			t.Errorf("region = %v, want the fallback region", res.Normalized["region"])
		}
	})

	t.Run("subtype_family_extraction", func(t *testing.T) {
		cases := map[string]string{
			"ecs.g6.large":     "g6",
			"ecs.gn6i-c4g1.xl": "gn6i-c4g1",
			"bare":             "bare",
			"":                 "",
		}
		for in, want := range cases {
			if got := instanceFamily(in); got != want {
				t.Errorf("instanceFamily(%q) = %q, want %q", in, got, want)
			}
		}
	})
}

// Raw 크기 상한(§8.2 — MaxRawBytes 64KiB): 초과분 절단 + normalized.truncated.
func TestAliyunRawSizeLimitTruncates(t *testing.T) {
	big := strings.Repeat("x", 100<<10)
	raw, err := json.Marshal(instance("i-big", map[string]any{"InstanceName": big}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	res, err := normalizeInstance(1, raw, "cn-mock-a")
	if err != nil {
		t.Fatalf("normalizeInstance: %v", err)
	}
	rawJSON := marshalForScan(t, res.Raw)
	if len(rawJSON) > MaxRawBytes {
		t.Errorf("Raw = %d bytes, exceeds MaxRawBytes %d", len(rawJSON), MaxRawBytes)
	}
	if res.Normalized["truncated"] != true {
		t.Errorf("normalized.truncated = %v, want true marker", res.Normalized["truncated"])
	}
}

// 옵션·Close — 카운터 공유면(WithCounters)과 stateless Close 계약.
func TestAliyunCloseAndOptions(t *testing.T) {
	shared := metrics.New()
	shared.RegisterProviders(ProviderName)
	a := NewAdapter(WithCounters(shared), WithPageSize(0))
	if a.RateCounter() != shared {
		t.Fatalf("WithCounters did not share the counter set")
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
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
