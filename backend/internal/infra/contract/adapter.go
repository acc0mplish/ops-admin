package contract

import "context"

// Adapter interfaces — spec §11 verbatim.
// Not declared here: ConsoleBroker (§11 "M2+, with the agent ADR"),
// EventSubscriber (removed in §11).

type BaseAdapter interface {
	Descriptor() ProviderTypeDescriptor
	Validate(ctx context.Context, connection ConnectionView) error
	Health(ctx context.Context, connection ConnectionView) HealthResult
	Close() error
}

type Discoverer interface {
	Discover(ctx context.Context, req DiscoverRequest) (DiscoverPage, error)
}

type OperationExecutor interface {
	Execute(ctx context.Context, req OperationRequest) (OperationHandle, error)
}

type TaskPoller interface {
	Poll(ctx context.Context, handle OperationHandle) (OperationStatus, error)
}

type TaskCanceller interface {
	Cancel(ctx context.Context, handle OperationHandle) error
}

// --- 지원 타입 최소 형상 (전부 가정 A6 — 각 필드의 출처 절을 주석으로 기록,
// 확장 시점 명시). ---

// ConnectionView — §7.2에서 도출 — 시크릿 값 결계: Config에 시크릿 소재 포함 금지.
type ConnectionView struct {
	ProviderType string
	Endpoint     string
	Config       JSONMap
	// Material — 브로커가 해석한 목적 스코프 자재. 키는 §7.4 목적 어휘(M1은
	// "operations" 1건). 평문이므로 로그·직렬화 금지. contract stdlib 순수성
	// 유지를 위해 타입은 map[string]string이다(secrets 패키지 import 불가).
	Material map[string]string
}

type HealthResult struct {
	Healthy bool
	Message string
}

// DiscoverRequest — §9.1/9.2에서 도출 — 커서 페이징. 리소스 초안 레코드 정규화
// 형태는 Phase 2 PR 20에서 착지: 어댑터는 리소스를 DiscoveredResource(정규화
// 필드 + Raw 크기 상한)로 반환한다.
//
// (Phase 2 PR 20) Connection — J2 판정: 어댑터의 Discover 시점 자격 전달 경로.
// 호출자(compose→SyncRunner)가 브로커 Resolve(ctx, connUID, "inventory") 결과를
// Connection.Material["inventory"]에 채운 ConnectionView를 구성해 전달한다.
// 어댑터는 req.Connection만 읽는다 — DB·브로커 무지 유지(arch rule 2).
// OperationRequest.Connection(실행 자격)은 Phase 3 인계(§13 N-1 잔존분).
type DiscoverRequest struct {
	ContextID  uint
	Cursor     string
	Connection ConnectionView
}

// DiscoveredResource — §8.1(신원) + §8.2(관측) 필드의 초안 병합.
type DiscoveredResource struct {
	ExternalID  string
	ExternalURN string // §8.1 UNIQUE(provider_context_id, kind, external_urn)의 성분
	Kind        string // IsKnownResourceKind 충족
	Subtype     string
	DisplayName string
	Raw         JSONMap
	Normalized  JSONMap
}

type DiscoverPage struct {
	Resources  []DiscoveredResource
	NextCursor string
}

// ProviderSignalError — §9.3 outcome 어휘와 1:1인 프로바이더 신호 에러
// (Phase 2 PR 20, J5 판정). 어댑터 트랜스포트가 provider 측 상태를 신호하면
// sync runner가 이 신호를 outcome/상태로 매핑한다. Kind는 아래 3종으로 닫혀
// 있고, 그 밖의 장애는 일반 error로 남는다(어휘 확장 금지 — 스펙이 준 것만).
// Message에는 자격 물질(kubeconfig 등)을 절대 포함하지 않는다(보존 제약 #7).
type ProviderSignalError struct {
	Kind    string // SignalRateLimited | SignalPermissionDenied | SignalUnreachable
	Message string
}

// Provider signal kinds — §9.3 어휘(rate_limited·permission_denied)와 1:1.
// unreachable은 §9.3 어휘 밖 신호로, outcome 맵에는 미가산되고 run 상태
// 매핑에서 처분된다(계획 §3.3).
const (
	SignalRateLimited      = "rate_limited"
	SignalPermissionDenied = "permission_denied"
	SignalUnreachable      = "unreachable"
)

func (e *ProviderSignalError) Error() string {
	if e.Message == "" {
		return "provider signal: " + e.Kind
	}
	return "provider signal (" + e.Kind + "): " + e.Message
}

// OperationRequest — §13(엔진)에서 도출 — 핸들은 provider-네이티브 작업 참조(예: UPID).
type OperationRequest struct {
	OperationName string
	ResourceURN   string
	Payload       JSONMap
}

type OperationHandle struct {
	ProviderRef string
}

type OperationStatus struct {
	State  string // §13.5 종단 상태(TimedOut·Cancelling·Cancelled 포함)는 Phase 1 폴러 설계 시점에 확정
	Detail JSONMap
}
