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
// ConnectionView 필드 확장(브로커 목적 자격증명 등)은 Phase 1 PR 16.
type ConnectionView struct {
	ProviderType string
	Endpoint     string
	Config       JSONMap
}

type HealthResult struct {
	Healthy bool
	Message string
}

// DiscoverRequest — §9.1/9.2에서 도출 — 커서 페이징. 리소스 초안 레코드 정규화
// 형태는 Phase 2 PR 20.
type DiscoverRequest struct {
	ContextID uint
	Cursor    string
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
