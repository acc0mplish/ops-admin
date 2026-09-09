package kubernetes

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/metrics"
)

// ProviderName is the Prometheus provider label (metrics 렌더 라인
// provider="kubernetes" — §18.2 라벨 형식, 게이트 ③ 증거).
const ProviderName = "kubernetes"

// pageSize bounds each Discover page (default). WithPageSize overrides — the
// contracttest harness asserts the forced bound.
const defaultPageSize = 100

// Adapter is the V2 read-only K8s/K3s adapter (BaseAdapter + Discoverer,
// contract §11). 어댑터는 제어평면 테이블·DB 핸들 무소유(arch rule 2) —
// kubeconfig는 ConnectionView.Material["inventory"](브로커 Resolve)로만
// 받는다(J2). 게이트웨이 모드는 주입된 dialer로만 구성된다(A4 — 실환경
// 게이트웨이 전송은 M2).
type Adapter struct {
	metrics       *metrics.Counters
	gatewayDialer func(ctx context.Context, network, addr string) (net.Conn, error)
	pageSize      int
}

// Option configures an Adapter at construction.
type Option func(*Adapter)

// WithCounters shares a metrics counter set with the caller (compose —
// sync/compare 리포트 아티팩트가 동일 카운터의 Render를 병기한다, J6).
func WithCounters(c *metrics.Counters) Option {
	return func(a *Adapter) { a.metrics = c }
}

// WithGatewayDialer injects the gateway hop dialer (A4 모의 테스트면).
func WithGatewayDialer(fn func(ctx context.Context, network, addr string) (net.Conn, error)) Option {
	return func(a *Adapter) { a.gatewayDialer = fn }
}

// WithPageSize overrides the per-Discover page bound (페이징 단얫용).
func WithPageSize(n int) Option {
	return func(a *Adapter) {
		if n > 0 {
			a.pageSize = n
		}
	}
}

// NewAdapter builds the kubernetes adapter and registers its provider row in
// the counter set so Render carries the {provider="kubernetes"} 0 line the
// gate ③ flat proof reads.
func NewAdapter(opts ...Option) *Adapter {
	a := &Adapter{metrics: metrics.New(), pageSize: defaultPageSize}
	a.metrics.RegisterProviders(ProviderName)
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Descriptor — Type "kubernetes", AdapterVersion "1", ProtocolVersion "1",
// ContextKinds ["cluster"], BuiltIn true (계획 §3.1).
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

// distribution detection (T43 — A3): GitVersion "+k3s" 접미사 → k3s.
func detectDistribution(gitVersion string) string {
	if strings.Contains(gitVersion, "+k3s") {
		return "k3s"
	}
	return "kubernetes"
}

// versionResponse is the /version payload subset (legacy kubeVersionResponse).
type versionResponse struct {
	GitVersion string `json:"gitVersion"`
}

// resolveRuntime parses the kubeconfig out of the connection view — the J2
// path: the adapter reads req.Connection only.
func resolveRuntime(conn contract.ConnectionView) (clusterRuntime, error) {
	kubeconfig := ""
	if conn.Material != nil {
		kubeconfig = strings.TrimSpace(conn.Material["inventory"])
	}
	if kubeconfig == "" {
		return clusterRuntime{}, fmt.Errorf("kubernetes: missing credential material Material[\"inventory\"] (broker purpose \"inventory\" resolve — J2)")
	}
	return parseKubeConfig(kubeconfig)
}

// dialContextFor judges the gateway hop configuration from
// Connection.Config (A4/T45 게이트웨이 변형 — §3.1 형상과 동일 입력).
func (a *Adapter) dialContextFor(conn contract.ConnectionView) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	mode, _ := conn.Config["connection_mode"].(string)
	if mode != "gateway" {
		return nil, nil
	}
	gatewayID, _ := conn.Config["gateway_id"].(string)
	if gatewayID == "" {
		return nil, fmt.Errorf("kubernetes: gateway mode connection is missing gateway_id")
	}
	if a.gatewayDialer == nil {
		return nil, fmt.Errorf("kubernetes: gateway mode requires a configured gateway dialer — Phase 2 proof is direct-connect only, the real gateway hop lands with M2 (A4)")
	}
	return a.gatewayDialer, nil
}

// buildClient resolves credentials + transport for one request.
func (a *Adapter) buildClient(conn contract.ConnectionView) (*k8sClient, error) {
	rt, err := resolveRuntime(conn)
	if err != nil {
		return nil, err
	}
	dialer, err := a.dialContextFor(conn)
	if err != nil {
		return nil, err
	}
	return newK8sClient(rt, dialer, a.metrics)
}

// Validate — kubeconfig 파싱 + /version 조회 (계획 §3.1).
func (a *Adapter) Validate(ctx context.Context, conn contract.ConnectionView) error {
	client, err := a.buildClient(conn)
	if err != nil {
		return err
	}
	var v versionResponse
	if err := client.getJSON(ctx, "/version", nil, &v); err != nil {
		return fmt.Errorf("kubernetes: validate: %w", err)
	}
	if strings.TrimSpace(v.GitVersion) == "" {
		return fmt.Errorf("kubernetes: validate: empty version")
	}
	return nil
}

// Health — /version 도달성 + distribution 판정(Message로 보고).
func (a *Adapter) Health(ctx context.Context, conn contract.ConnectionView) contract.HealthResult {
	client, err := a.buildClient(conn)
	if err != nil {
		return contract.HealthResult{Healthy: false, Message: err.Error()}
	}
	var v versionResponse
	if err := client.getJSON(ctx, "/version", nil, &v); err != nil {
		return contract.HealthResult{Healthy: false, Message: fmt.Sprintf("kubernetes: version probe failed: %v", err)}
	}
	dist := detectDistribution(v.GitVersion)
	return contract.HealthResult{
		Healthy: true,
		Message: fmt.Sprintf("kubernetes %s (distribution: %s)", v.GitVersion, dist),
	}
}

// --- Discoverer — 섹션 커서 페이징 (§14.1 리소스 목록 순). ---

// sectionTable is the fixed discovery section order — plan §3.1 base:
// nodes→namespaces→pods→workloads(deploy/statefulset/daemonset)→services→
// ingresses→configmaps→secrets→pv→pvc→storageclasses, extended by P 계획
// J-P1-1 (P1-A: replicasets·jobs·cronjobs·endpoints). path는 legacy
// fetchK8sData의 raw REST 경로와 동일하다.
type sectionTable struct {
	name string
	path string
}

var discoverSections = []sectionTable{
	{"nodes", "/api/v1/nodes"},
	{"namespaces", "/api/v1/namespaces"},
	{"pods", "/api/v1/pods"},
	{"deployments", "/apis/apps/v1/deployments"},
	{"statefulsets", "/apis/apps/v1/statefulsets"},
	{"daemonsets", "/apis/apps/v1/daemonsets"},
	// P1-A — replicaset은 pod→워크로드 역추적 체인 원천(J-P1-9 부수), job·cronjob
	// 수집은 비교 집합 합류(P1-C2)의 필요조건이다(§9-10 — 스코프 변경은 P1-C2).
	{"replicasets", "/apis/apps/v1/replicasets"},
	{"jobs", "/apis/batch/v1/jobs"},
	{"cronjobs", "/apis/batch/v1/cronjobs"},
	{"services", "/api/v1/services"},
	// P1-A — v2-only 보조종: service.endpoints 집계(J-P1-9)의 원천.
	{"endpoints", "/api/v1/endpoints"},
	{"ingresses", "/apis/networking.k8s.io/v1/ingresses"},
	{"configmaps", "/api/v1/configmaps"},
	{"secrets", "/api/v1/secrets"},
	{"persistentvolumes", "/api/v1/persistentvolumes"},
	{"persistentvolumeclaims", "/api/v1/persistentvolumeclaims"},
	{"storageclasses", "/apis/storage.k8s.io/v1/storageclasses"},
}

// cursor format: "<section>|<continue-token>" — each section delegates to the
// K8s `continue` token. Terminal cursor is "".
func encodeCursor(section, token string) string { return section + "|" + token }

func splitCursor(cursor string) (section, token string, err error) {
	if cursor == "" {
		return discoverSections[0].name, "", nil
	}
	section, token, ok := strings.Cut(cursor, "|")
	if !ok {
		return "", "", fmt.Errorf("kubernetes: malformed cursor %q (want <section>|<token>)", cursor)
	}
	for _, s := range discoverSections {
		if s.name == section {
			return section, token, nil
		}
	}
	return "", "", fmt.Errorf("kubernetes: unknown cursor section %q", section)
}

// Discover returns one page of resources, delegating pagination to the K8s
// continue token per section. Page bound = pageSize — a page never exceeds it
// (contracttest 단얫 1).
func (a *Adapter) Discover(ctx context.Context, req contract.DiscoverRequest) (contract.DiscoverPage, error) {
	client, err := a.buildClient(req.Connection)
	if err != nil {
		return contract.DiscoverPage{}, err
	}

	section, token, err := splitCursor(req.Cursor)
	if err != nil {
		return contract.DiscoverPage{}, err
	}

	resources := make([]contract.DiscoveredResource, 0, a.pageSize)
	for idx := 0; idx < len(discoverSections); idx++ {
		if discoverSections[idx].name != section {
			continue
		}
		// Walk sections from `section` until the page bound fills or all
		// sections end.
		for {
			remaining := a.pageSize - len(resources)
			limit := a.pageSize
			if remaining < limit {
				limit = remaining
			}
			if limit <= 0 {
				break
			}
			var envelope listEnvelope
			if err := client.getJSON(ctx, discoverSections[idx].path, map[string]string{
				"limit":    strconv.Itoa(limit),
				"continue": token,
			}, &envelope); err != nil {
				return contract.DiscoverPage{}, err
			}
			for _, item := range envelope.Items {
				res, err := normalizeSection(req.ContextID, section, item)
				if err != nil {
					return contract.DiscoverPage{}, err
				}
				resources = append(resources, res)
			}
			if envelope.Metadata.Continue != "" {
				token = envelope.Metadata.Continue
				break // mid-section: delegate to the K8s continue token
			}
			token = ""
			// Section exhausted — advance.
			if len(resources) >= a.pageSize {
				break
			}
			if idx+1 >= len(discoverSections) {
				return contract.DiscoverPage{Resources: resources, NextCursor: ""}, nil
			}
			idx++
			section = discoverSections[idx].name
		}

		next := ""
		if token != "" {
			next = encodeCursor(section, token)
		} else if idx+1 < len(discoverSections) {
			next = encodeCursor(discoverSections[idx+1].name, "")
		}
		return contract.DiscoverPage{Resources: resources, NextCursor: next}, nil
	}

	// Requested section not found (should be unreachable — splitCursor
	// validates) — treat as terminal.
	return contract.DiscoverPage{Resources: resources, NextCursor: ""}, nil
}

// Compile-time interface conformance (contract §11).
var (
	_ contract.BaseAdapter = (*Adapter)(nil)
	_ contract.Discoverer  = (*Adapter)(nil)
)
