package kubernetes

import (
	"context"
	"errors"
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

// Health — /version 도달성 + distribution 판정(Message로 보고). ④ A′-1(P1-G1):
// probe 성공 시 kubeconfig 인증서 성분의 파생 관측을 Observation으로 실는다 —
// 베스트 에포트로, 자재 재해석 실패 시에도 healthy 판정은 유지하고 관측만
// 생략한다. 기록은 sweep 소관이다(compose/healthloop.go — arch rule 2).
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
	result := contract.HealthResult{
		Healthy: true,
		Message: fmt.Sprintf("kubernetes %s (distribution: %s)", v.GitVersion, dist),
	}
	if rt, rtErr := resolveRuntime(conn); rtErr == nil {
		result.Observation = certificateObservation(rt)
	}
	return result
}

// --- Discoverer — 섹션 커서 페이징 (§14.1 리소스 목록 순). ---

// sectionTable is the fixed discovery section order — plan §3.1 base:
// nodes→namespaces→pods→workloads(deploy/statefulset/daemonset)→services→
// ingresses→configmaps→secrets→pv→pvc→storageclasses, extended by P 계획
// J-P1-1 (P1-A: replicasets·jobs·cronjobs·endpoints, P1-C1: gateways·
// httproutes). path는 legacy fetchK8sData의 raw REST 경로와 동일하다.
type sectionTable struct {
	name string
	path string
}

// sectionFallbacks holds the alternate API versions tried in order when the
// primary path answers 404 — the GatewayAPI multi-version 폴백(J-P1-1, legacy
// buildGatewayAPIResourcePathsWithPreferred k8s_transport.go:279 의미론: v1
// 선호, 404 시 v1beta1). 전 후보가 404면 CRD 미설치 섹션(M-6)이라 스킵한다.
// 404 외 오류(401/403/429/전송)는 기존 신호 분류 그대로다. 표 밖 분리는 기존
// sectionTable 위치 리터럴 15행을 무Touch로 유지하기 위함이다.
var sectionFallbacks = map[string][]string{
	"gateways":   {"/apis/gateway.networking.k8s.io/v1beta1/gateways"},
	"httproutes": {"/apis/gateway.networking.k8s.io/v1beta1/httproutes"},
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
	// P1-C1 — GatewayAPI 수집(GatewayAPI CRD group gateway.networking.k8s.io).
	// v1 선호 → v1beta1 폴백(sectionFallbacks — J-P1-1). CRD 부재 클러스터에서는
	// 전 버전 404로 섹션 스킵(공허 녹색 — M-6·J-P1-8), 비교 집합 불참(I-P5 이월).
	{"gateways", "/apis/gateway.networking.k8s.io/v1/gateways"},
	{"httproutes", "/apis/gateway.networking.k8s.io/v1/httproutes"},
	{"services", "/api/v1/services"},
	// P1-A — v2-only 보조종: service.endpoints 집계(J-P1-9)의 원천.
	{"endpoints", "/api/v1/endpoints"},
	{"ingresses", "/apis/networking.k8s.io/v1/ingresses"},
	{"configmaps", "/api/v1/configmaps"},
	{"secrets", "/api/v1/secrets"},
	{"persistentvolumes", "/api/v1/persistentvolumes"},
	{"persistentvolumeclaims", "/api/v1/persistentvolumeclaims"},
	{"storageclasses", "/apis/storage.k8s.io/v1/storageclasses"},
	// P2-D — istio VirtualService 앵커(J-P1-1 — networking.istio.io v1beta1).
	// 스코프 외 종: 쓰기(k8s.istio.traffic_update)의 uid 닻 전용이고 비교 집합
	// 불참이다(R-P6 — S-2 읽기 패리티 제외와 무관). istio CRD 부재 클러스터에서는
	// 404 섹션 스킵(gateways와 동일 — M-6·J-P1-8).
	// Anchor path is v1beta1-only by design: istio serves VirtualService
	// predominantly on v1beta1 (v1 group-version has no notable serving
	// history), so unlike gatewayapi there is no sectionFallbacks entry.
	// The traffic executor still walks v1→v1beta1 PUT candidates — see
	// P2-D review LOW note (2026-09-10) if istio CRD versions are revisited.
	{"virtualservices", "/apis/networking.istio.io/v1beta1/virtualservices"},
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

// sectionPaths lists the candidate paths of one section in fallback order —
// the primary path first, then the version fallbacks (J-P1-1).
func sectionPaths(s sectionTable) []string {
	fallback := sectionFallbacks[s.name]
	paths := make([]string, 0, 1+len(fallback))
	paths = append(paths, s.path)
	paths = append(paths, fallback...)
	return paths
}

// fetchSectionPage lists one page trying each candidate path in order. A 404
// on a candidate advances to the next version (buildGatewayAPIResourcePaths
// 의미론); when every candidate 404s the section is ABSENT on the cluster
// (GatewayAPI CRD 미설치 — M-6) and the sentinel returns so the walk can skip
// it — 404는 섹션 스킵 신호고 그 외 오류(401/403/429/전송)는 기존 신호 분류
// 그대로 상위로 전파한다. 폴백은 호출 단위 재판정이다(커서는 섹션|토큰 형식
// 계약이라 버전을 실을 수 없다 — v1 404 1회가 페이지마다 재발한다, 비용 미미).
func fetchSectionPage(ctx context.Context, client *k8sClient, paths []string, query map[string]string) (listEnvelope, error) {
	for _, path := range paths {
		var envelope listEnvelope
		err := client.getJSON(ctx, path, query, &envelope)
		if err == nil {
			return envelope, nil
		}
		if errors.Is(err, errNotFound) {
			continue
		}
		return listEnvelope{}, err
	}
	return listEnvelope{}, errNotFound
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
			envelope, err := fetchSectionPage(ctx, client, sectionPaths(discoverSections[idx]), map[string]string{
				"limit":    strconv.Itoa(limit),
				"continue": token,
			})
			if err != nil && !errors.Is(err, errNotFound) {
				return contract.DiscoverPage{}, err
			}
			if errors.Is(err, errNotFound) {
				// 전 후보 경로 404 — 섹션 부재(CRD 미설치). 공허 엔벨로프로 흘려
				// 보내면 기존 섹션 소진 로직이 그대로 다음 섹션으로 전진한다.
				envelope = listEnvelope{}
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
