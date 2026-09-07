package aliyun

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/metrics"
)

// ProviderName is the Prometheus provider label (metrics 렌더 라인
// provider="aliyun" — §18.2 라벨 형식) and the provider-type vocabulary value
// (contract §7.1 — "aliyun" 선언 어휘, 코어 분기 아님).
const ProviderName = "aliyun"

// pageSize bounds each Discover page (default). WithPageSize overrides — the
// contracttest harness asserts the forced bound. legacy v1도 PageSize=100
// (asset_cloud_aliyun.go 102-103행) — 기본값 승계.
const defaultPageSize = 100

// Adapter is the V2 read-only Aliyun ECS adapter (BaseAdapter + Discoverer,
// contract §11). 어댑터는 제어평면 테이블·DB 핸들 무소유(arch rule 2) — 자격은
// ConnectionView.Material["inventory"] JSON blob {accessKey,secretKey}로만
// 받는다(판정 J2·J4, 계획 §3.1).
type Adapter struct {
	metrics          *metrics.Counters
	pageSize         int
	endpointOverride func() string
}

// Option configures an Adapter at construction.
type Option func(*Adapter)

// WithCounters shares a metrics counter set with the caller (compose —
// sync/compare 리포트 아티팩트가 동일 카운터의 Render를 병기한다).
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

// WithEndpointOverride injects a process-level endpoint override — used below
// req.Connection.Endpoint precedence. It exists for harness paths that drive
// the adapter without stuffing the connection view; production wiring leaves
// it unset (the regional ECS default applies).
func WithEndpointOverride(fn func() string) Option {
	return func(a *Adapter) { a.endpointOverride = fn }
}

// NewAdapter builds the aliyun adapter and registers its provider row in the
// counter set so Render carries the {provider="aliyun"} 0 line the gate ③
// flat proof reads.
func NewAdapter(opts ...Option) *Adapter {
	a := &Adapter{metrics: metrics.New(), pageSize: defaultPageSize}
	a.metrics.RegisterProviders(ProviderName)
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Descriptor — Type "aliyun", AdapterVersion "1", ProtocolVersion "1",
// ContextKinds ["account"] (계획 §3.1 — 클라우드 계정 컨텍스트).
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

// Validate — 자격 파싱 + DescribeRegions 스모크 (계획 §3.1).
func (a *Adapter) Validate(ctx context.Context, conn contract.ConnectionView) error {
	cred, err := resolveCredential(conn)
	if err != nil {
		return err
	}
	client := a.clientFor(conn, cred)
	regions, err := client.describeRegions(ctx, "validate")
	if err != nil {
		return fmt.Errorf("aliyun: validate: %w", err)
	}
	if len(regions) == 0 {
		return fmt.Errorf("aliyun: validate: no region available")
	}
	return nil
}

// Health — DescribeRegions 도달성. Message는 계정 식별(마스킹)·리전 수를 담되
// 자격 물질은 담지 않는다(계획 §3.1).
func (a *Adapter) Health(ctx context.Context, conn contract.ConnectionView) contract.HealthResult {
	cred, err := resolveCredential(conn)
	if err != nil {
		return contract.HealthResult{Healthy: false, Message: err.Error()}
	}
	client := a.clientFor(conn, cred)
	regions, err := client.describeRegions(ctx, "health")
	if err != nil {
		return contract.HealthResult{Healthy: false, Message: fmt.Sprintf("aliyun: region probe failed: %v", err)}
	}
	return contract.HealthResult{
		Healthy: true,
		Message: fmt.Sprintf("aliyun: %d regions visible (account %s)", len(regions), maskedAccessKey(cred.accessKey)),
	}
}

// maskedAccessKey reduces the access key to a display-safe prefix — an
// identity hint, never the material itself (보존 제약 3).
func maskedAccessKey(accessKey string) string {
	if len(accessKey) <= 4 {
		return "***"
	}
	return accessKey[:4] + "***"
}

// --- Discoverer — 리전×페이지 커서 페이징 (판정 J3) ---

// cursor format: "<region>|<pageNumber>" — each Discover call serves one page
// of one region (k8s section 커서 관례 승계). Terminal cursor is "".
func encodeCursor(region string, page int) string {
	return region + "|" + strconv.Itoa(page)
}

func splitCursor(cursor string, regions []string) (region string, page int, err error) {
	if strings.TrimSpace(cursor) == "" {
		if len(regions) == 0 {
			return "", 0, fmt.Errorf("aliyun: no region available for discovery")
		}
		return regions[0], 1, nil
	}
	rawRegion, rawPage, ok := strings.Cut(cursor, "|")
	if !ok {
		return "", 0, fmt.Errorf("aliyun: malformed cursor %q (want <region>|<pageNumber>)", cursor)
	}
	page, convErr := strconv.Atoi(rawPage)
	if convErr != nil || page < 1 {
		return "", 0, fmt.Errorf("aliyun: malformed cursor %q (pageNumber must be a positive integer)", cursor)
	}
	for _, candidate := range regions {
		if candidate == rawRegion {
			return rawRegion, page, nil
		}
	}
	return "", 0, fmt.Errorf("aliyun: unknown cursor region %q", rawRegion)
}

// regionsFromConfig reads the account region list out of the connection
// config (계정 regions 설정 우선 — 판정 J3/R9; the caller population maps the
// context metadata). Accepts []string, []any (JSON array), and CSV strings.
func regionsFromConfig(conn contract.ConnectionView) []string {
	if conn.Config == nil {
		return nil
	}
	switch v := conn.Config["regions"].(type) {
	case []string:
		return trimRegions(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return trimRegions(out)
	case string:
		return trimRegions(strings.Split(v, ","))
	default:
		return nil
	}
}

func trimRegions(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// resolveRegions is the deterministic region order: the account config first,
// DescribeRegions enumeration as the legacy-matching fallback (판정 J3).
func (a *Adapter) resolveRegions(ctx context.Context, client *ecsClient, conn contract.ConnectionView, op string) ([]string, error) {
	if regions := regionsFromConfig(conn); len(regions) > 0 {
		return regions, nil
	}
	regions, err := client.describeRegions(ctx, op)
	if err != nil {
		return nil, err
	}
	if len(regions) == 0 {
		return nil, fmt.Errorf("aliyun: no region available")
	}
	return regions, nil
}

// Discover returns one page of compute.vm resources, walking the cursor chain
// region by region (legacy fetchAliyunCloudInstances의 리전×페이지 순회를
// 재개 가능한 커서로). Page bound = pageSize — a page never exceeds it
// (contracttest 단얫 1).
func (a *Adapter) Discover(ctx context.Context, req contract.DiscoverRequest) (contract.DiscoverPage, error) {
	cred, err := resolveCredential(req.Connection)
	if err != nil {
		return contract.DiscoverPage{}, err
	}
	client := a.clientFor(req.Connection, cred)
	regions, err := a.resolveRegions(ctx, client, req.Connection, "discover")
	if err != nil {
		return contract.DiscoverPage{}, err
	}
	region, page, err := splitCursor(req.Cursor, regions)
	if err != nil {
		return contract.DiscoverPage{}, err
	}

	resources := make([]contract.DiscoveredResource, 0, a.pageSize)
	for {
		remaining := a.pageSize - len(resources)
		batch, total, err := client.describeInstances(ctx, region, page, remaining)
		if err != nil {
			return contract.DiscoverPage{}, err
		}
		for _, item := range batch {
			raw, encErr := json.Marshal(item)
			if encErr != nil {
				return contract.DiscoverPage{}, fmt.Errorf("aliyun: instance %q encode: %w", item.InstanceID, encErr)
			}
			res, normErr := normalizeInstance(req.ContextID, raw, region)
			if normErr != nil {
				return contract.DiscoverPage{}, normErr
			}
			resources = append(resources, res)
		}

		// Region exhausted (legacy 종료 규칙 — 빈 배치 또는 TotalCount 도달).
		if len(batch) == 0 || page*remaining >= total {
			next := nextRegion(regions, region)
			if next == "" {
				return contract.DiscoverPage{Resources: resources, NextCursor: ""}, nil
			}
			if len(resources) >= a.pageSize {
				return contract.DiscoverPage{Resources: resources, NextCursor: encodeCursor(next, 1)}, nil
			}
			region = next
			page = 1
			continue
		}

		page++
		if len(resources) >= a.pageSize {
			return contract.DiscoverPage{Resources: resources, NextCursor: encodeCursor(region, page)}, nil
		}
	}
}

func nextRegion(regions []string, current string) string {
	for idx, candidate := range regions {
		if candidate == current && idx+1 < len(regions) {
			return regions[idx+1]
		}
	}
	return ""
}

// Compile-time interface conformance (contract §11).
var (
	_ contract.BaseAdapter = (*Adapter)(nil)
	_ contract.Discoverer  = (*Adapter)(nil)
)
