package contract

// OperationDefinition is spec §10.2 verbatim.
type OperationDefinition struct {
	Name               string
	Version            string
	ResourceKinds      []string
	RequiredCapability string
	RequiredPermission string // v1-namespace string — 허용 형식 2~4세그·하이픈 허용(§10.3 서술과 v1 실측의 불일치는 가정 A14)
	Mutating           bool
	RiskLevel          string
	RequiresApproval   bool
	IdempotencyPolicy  string
	TimeoutSeconds     int
	RetryPolicy        RetryPolicy
	Redaction          func() any // typed redaction spec per result field (§10.2)
}

// RetryPolicy — 최소 형상(가정 A10) — §10.2가 타입명만 언급. 필드 확장은 Phase 1
// 태스크 엔진이 실제 재시도 동작을 구현할 때 그 시점의 요구로 결정.
type RetryPolicy struct {
	MaxAttempts    int
	BackoffSeconds int
}
