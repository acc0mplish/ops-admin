package tencent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/metrics"
)

// ProviderName is the Prometheus provider label (metrics 렌더 라인
// provider="tencent" — §18.2 라벨 형식, 계획 §0.7-6).
const ProviderName = "tencent"

// defaultPageSize bounds each Discover page (default). WithPageSize overrides
// — the contracttest harness asserts the forced bound.
const defaultPageSize = 100

// Adapter is the V2 read-only Tencent Cloud adapter (BaseAdapter + Discoverer,
// contract §11 — 계획 §2 N6). 어댑터는 제어평면 테이블·DB 핸들 무소유(arch rule
// 2) — 자격은 ConnectionView.Material["inventory"](브로커 Resolve) JSON
// blob으로만 받는다(J2), 리전은 Connection.Config["regions"](판정 J3 — Tencent는
// 리전 열거 API 미사용·계정 리전 필수, legacy 동일).
type Adapter struct {
	metrics          *metrics.Counters
	pageSize         int
	endpointOverride func() string
}

// Option configures an Adapter at construction.
type Option func(*Adapter)

// WithCounters shares a metrics counter set with the caller (compose —
// sync/compare 리포트 아티팩트가 동일 카운터의 Render를 병기한다, J6).
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

// WithEndpointOverride installs the highest-priority endpoint resolver
// (계획 §3.1 계약 형상 — nil·빈 반환값이면 Connection.Endpoint → 기본 순).
// 제품 배선은 이 옵션을 쓰지 않는다.
func WithEndpointOverride(fn func() string) Option {
	return func(a *Adapter) { a.endpointOverride = fn }
}

// NewAdapter builds the tencent adapter and registers its provider row in the
// counter set so Render carries the {provider="tencent"} 0 line the gate ③
// flat proof reads.
func NewAdapter(opts ...Option) *Adapter {
	a := &Adapter{metrics: metrics.New(), pageSize: defaultPageSize}
	a.metrics.RegisterProviders(ProviderName)
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Descriptor — Type "tencent", AdapterVersion "1", ProtocolVersion "1",
// ContextKinds ["account"], BuiltIn true (계획 §3.1 — §7.3 Kind 어휘).
func (*Adapter) Descriptor() contract.ProviderTypeDescriptor {
	return contract.ProviderTypeDescriptor{
		Type:            ProviderName,
		AdapterVersion:  "1",
		ProtocolVersion: "1",
		ConfigSchema:    func() contract.ConfigSpec { return contract.ConfigSpec{} },
		ContextKinds:    []string{"account"},
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

// resolveCredentials parses the inventory material JSON blob out of the
// connection view — the J2 path: the adapter reads req.Connection only.
func resolveCredentials(conn contract.ConnectionView) (credential, error) {
	material := ""
	if conn.Material != nil {
		material = strings.TrimSpace(conn.Material["inventory"])
	}
	if material == "" {
		return credential{}, fmt.Errorf("tencent: missing credential material Material[\"inventory\"] (broker purpose \"inventory\" resolve — J2)")
	}
	var blob struct {
		AccessKey string `json:"accessKey"`
		SecretKey string `json:"secretKey"`
	}
	if err := json.Unmarshal([]byte(material), &blob); err != nil {
		return credential{}, errors.New("tencent: credential material parse failed (want {\"accessKey\",\"secretKey\"} JSON)")
	}
	if blob.AccessKey == "" || blob.SecretKey == "" {
		return credential{}, errors.New("tencent: credential material missing accessKey or secretKey")
	}
	return credential{accessKey: blob.AccessKey, secretKey: blob.SecretKey}, nil
}

// regionsOf reads the account region list — Tencent has no region enumeration
// on this path (판정 J3, legacy util/tencentcloud.go GetInstances 동일 계약:
// "at least one Tencent Cloud region is required").
func regionsOf(conn contract.ConnectionView) ([]string, error) {
	raw, ok := conn.Config["regions"]
	if !ok || raw == nil {
		return nil, errors.New("tencent: no regions configured (Connection.Config[\"regions\"] — 계정 리전 필수, 판정 J3)")
	}
	// Config는 JSON 디코드 산물이므로 []any가 정형, []string도 수용한다.
	var regions []string
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					regions = append(regions, trimmed)
				}
			}
		}
	case []string:
		for _, s := range v {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				regions = append(regions, trimmed)
			}
		}
	}
	if len(regions) == 0 {
		return nil, errors.New("tencent: no regions configured (Connection.Config[\"regions\"] — 계정 리전 필수, 판정 J3)")
	}
	return regions, nil
}

// encodeCursor — cursor format: "<region>|<offset>" (계획 §3.1 — 리전×페이지
// 순회, k8s section 커서 관례의 클라우드 변형). Terminal cursor is "".
func encodeCursor(region string, offset int) string { return region + "|" + strconv.Itoa(offset) }

func splitCursor(regions []string, cursor string) (string, int, error) {
	if cursor == "" {
		if len(regions) == 0 {
			return "", 0, errors.New("tencent: empty cursor over an empty region list")
		}
		return regions[0], 0, nil
	}
	region, offsetPart, ok := strings.Cut(cursor, "|")
	if !ok {
		return "", 0, fmt.Errorf("tencent: malformed cursor %q (want <region>|<offset>)", cursor)
	}
	offset, err := strconv.Atoi(offsetPart)
	if err != nil || offset < 0 {
		return "", 0, fmt.Errorf("tencent: malformed cursor offset in %q (want a non-negative integer)", cursor)
	}
	for _, r := range regions {
		if r == region {
			return region, offset, nil
		}
	}
	return "", 0, fmt.Errorf("tencent: cursor region %q is not in the account region list", region)
}

// endpointFor resolves one request's base endpoint: override →
// Connection.Endpoint → provider default (계획 §3.1).
func (a *Adapter) endpointFor(conn contract.ConnectionView) string {
	if a.endpointOverride != nil {
		if ep := strings.TrimSpace(a.endpointOverride()); ep != "" {
			return normalizeEndpoint(ep)
		}
	}
	return normalizeEndpoint(conn.Endpoint)
}

// Validate — 자격 파싱 + 리전 존재 + 첫 리전 DescribeInstances 1페이지 프로브
// (계획 §3.1 — tencent는 DescribeInstances 1리전).
func (a *Adapter) Validate(ctx context.Context, conn contract.ConnectionView) error {
	client, err := a.buildClient(conn, 0)
	if err != nil {
		return err
	}
	if _, err := client.describeInstances(ctx, 0, 1); err != nil {
		return fmt.Errorf("tencent: validate: %w", err)
	}
	return nil
}

// Health — 같은 1리전 프로브 + 계정·리전 규모 보고(Message에 자격 물질 미포함 —
// 계획 §3.1·보존 제약 3).
func (a *Adapter) Health(ctx context.Context, conn contract.ConnectionView) contract.HealthResult {
	regions, err := regionsOf(conn)
	if err != nil {
		return contract.HealthResult{Healthy: false, Message: err.Error()}
	}
	client, err := a.buildClient(conn, 0)
	if err != nil {
		return contract.HealthResult{Healthy: false, Message: err.Error()}
	}
	page, err := client.describeInstances(ctx, 0, 1)
	if err != nil {
		return contract.HealthResult{Healthy: false, Message: fmt.Sprintf("tencent: health probe failed: %v", err)}
	}
	return contract.HealthResult{
		Healthy: true,
		Message: fmt.Sprintf("tencent: %d region(s) configured, probe of %q total=%d instance(s)",
			len(regions), regions[0], page.Response.TotalCount),
	}
}

// buildClient assembles the region client for regions[idx] — credentials and
// regions resolved here so every entry point shares the J2 path.
func (a *Adapter) buildClient(conn contract.ConnectionView, regionIdx int) (*cvmClient, error) {
	cred, err := resolveCredentials(conn)
	if err != nil {
		return nil, err
	}
	regions, err := regionsOf(conn)
	if err != nil {
		return nil, err
	}
	return newCVMClient(cred, a.endpointFor(conn), regions[regionIdx], a.metrics), nil
}

// Discover returns one page of compute.vm resources, walking the account's
// regions × DescribeInstances offset pages (커서 "<region>|<offset>"). Page
// bound = pageSize — a page never exceeds it (contracttest 단얫 1).
func (a *Adapter) Discover(ctx context.Context, req contract.DiscoverRequest) (contract.DiscoverPage, error) {
	cred, err := resolveCredentials(req.Connection)
	if err != nil {
		return contract.DiscoverPage{}, err
	}
	regions, err := regionsOf(req.Connection)
	if err != nil {
		return contract.DiscoverPage{}, err
	}
	region, offset, err := splitCursor(regions, req.Cursor)
	if err != nil {
		return contract.DiscoverPage{}, err
	}
	endpoint := a.endpointFor(req.Connection)

	resources := make([]contract.DiscoveredResource, 0, a.pageSize)
	for i := indexOf(regions, region); i < len(regions); i++ {
		client := newCVMClient(cred, endpoint, regions[i], a.metrics)
		for {
			remaining := a.pageSize - len(resources)
			if remaining <= 0 {
				return contract.DiscoverPage{Resources: resources, NextCursor: encodeCursor(regions[i], offset)}, nil
			}
			page, err := client.describeInstances(ctx, int64(offset), int64(remaining))
			if err != nil {
				return contract.DiscoverPage{}, err
			}
			for _, inst := range page.Response.InstanceSet {
				res, err := normalizeInstance(req.ContextID, regions[i], inst)
				if err != nil {
					return contract.DiscoverPage{}, err
				}
				resources = append(resources, res)
				offset++
			}
			// 리전 소진 — 프로바이더 총수 또는 빈 페이지로 판정(빈 페이지
			// 무한 루프 금지). offset은 다음 리전에서 0으로 리셋된다.
			if len(page.Response.InstanceSet) == 0 || int64(offset) >= page.Response.TotalCount {
				break
			}
		}
		offset = 0
		if len(resources) >= a.pageSize && i+1 < len(regions) {
			return contract.DiscoverPage{Resources: resources, NextCursor: encodeCursor(regions[i+1], 0)}, nil
		}
	}
	return contract.DiscoverPage{Resources: resources, NextCursor: ""}, nil
}

func indexOf(regions []string, region string) int {
	for i, r := range regions {
		if r == region {
			return i
		}
	}
	return 0
}

// Compile-time interface conformance (contract §11).
var (
	_ contract.BaseAdapter = (*Adapter)(nil)
	_ contract.Discoverer  = (*Adapter)(nil)
)
