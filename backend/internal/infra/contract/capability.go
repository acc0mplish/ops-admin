package contract

// Capability is spec §10.1 verbatim.
type Capability struct {
	Name          string
	Version       string
	ResourceKinds []string
	ReadOnly      bool
	RequestSpec   func() any // typed Go builders, not runtime JSON Schema (§10.1)
	ResultSpec    func() any
	Constraints   JSONMap
}

// CapabilityVocabularyEntry — §10.1 "Initial names (M1)" 7종의 어휘 메타데이터.
// ReadOnly는 가정 A4 표, OwnerPhase는 "r1의 vnc/spice… move to their owning
// phases" 문장의 계약화.
type CapabilityVocabularyEntry struct {
	Name       string
	ReadOnly   bool
	OwnerPhase string
}

// M1CapabilityVocabulary is the closed M1 capability vocabulary (§10.1) plus
// the Phase 5 PVE guarded-operations trio (plan M2 — §14.3 "read-only token for
// discovery, operations token gated by approval"). ReadOnly is vocabulary
// metadata only — it is never used to derive a serving interface (r2, V5):
// console.web_terminal is served by ConsoleBroker (M2+, not declared here) and
// cost.read is readonly but not a Discoverer. The capability→interface mapping
// table is CapabilityInterfaceMap below.
var M1CapabilityVocabulary = []CapabilityVocabularyEntry{
	{Name: "inventory.full", ReadOnly: true, OwnerPhase: "M1"},
	{Name: "inventory.incremental", ReadOnly: true, OwnerPhase: "M1"},
	{Name: "orchestration.kubernetes.read", ReadOnly: true, OwnerPhase: "M1"},
	{Name: "orchestration.kubernetes.apply", ReadOnly: false, OwnerPhase: "M1/Phase3"}, // §10.1 "(Phase 3: restart only)"
	{Name: "compute.vm.read", ReadOnly: true, OwnerPhase: "M1"},
	{Name: "cost.read", ReadOnly: true, OwnerPhase: "M1"},
	{Name: "console.web_terminal", ReadOnly: true, OwnerPhase: "M1"},
	// Phase 5 (PR 31b) — PVE 게스트 mutation 3종(J7). executor 구현과 같은
	// Phase에 승격된다(registry V5 — OperationExecutor 필수의 같은-커밋 요구).
	{Name: "compute.power.manage", ReadOnly: false, OwnerPhase: "Phase5"},
	{Name: "storage.snapshot.manage", ReadOnly: false, OwnerPhase: "Phase5"},
	{Name: "compute.config.apply", ReadOnly: false, OwnerPhase: "Phase5"},
}

// InterfaceRequirement is one row of the capability→interface mapping table
// (plan §3.7 — Phase 0 인계 1 이행). The registry's RegisterCapabilities
// type-asserts a provider type's adapter against RequiredInterface before
// accepting a capability declaration. No ReadOnly→interface dichotomy is
// used (phase0 r2 V5 판정 승계 — cost.read·console.web_terminal 반례).
type InterfaceRequirement struct {
	// Capability names the §10.1 vocabulary entry this row governs.
	Capability string
	// RequiredInterface names the interface an adapter MUST implement to
	// declare the capability: "Discoverer", "OperationExecutor", or ""
	// (none). "" is an explicit table VALUE, not a validation exclusion.
	RequiredInterface string
	// OptionalInterfaces names interfaces the execution path MAY use when
	// present (type-asserted at run time, never required at registration).
	OptionalInterfaces []string
	// Reason documents the row's provenance (§3.7 표의 근거·비고 열).
	Reason string
}

// CapabilityInterfaceMap — the §3.7 mapping table, one row per M1 capability
// name plus the Phase 5 PVE trio. cost.read (§3.2 row 18 — finops stays on the
// existing scheduler, no adapter interface) and console.web_terminal (§11
// ConsoleBroker M2+) carry an explicit empty requirement.
var CapabilityInterfaceMap = []InterfaceRequirement{
	{Capability: "inventory.full", RequiredInterface: "Discoverer",
		Reason: "§9 동기화가 Discoverer 페이징 소비"},
	{Capability: "inventory.incremental", RequiredInterface: "Discoverer",
		Reason: "동일(커서, §9.1)"},
	{Capability: "orchestration.kubernetes.read", RequiredInterface: "Discoverer",
		Reason: "Phase 2 k8s read-only 슬라이스 = 디스커버리 (A11)"},
	{Capability: "orchestration.kubernetes.apply", RequiredInterface: "OperationExecutor",
		OptionalInterfaces: []string{"TaskPoller", "TaskCanceller"},
		Reason:             "§14.3 UPID — 핸들 반환 시 폴 시도, 널 핸들은 동기 완료(이중 모드 r2.2)"},
	{Capability: "compute.vm.read", RequiredInterface: "Discoverer",
		Reason: "Phase 4 클라우드 인벤토리"},
	{Capability: "cost.read", RequiredInterface: "",
		Reason: "§3.2 row 18: finops는 기존 스케줄러 유지 — 어댑터 인터페이스 없음"},
	{Capability: "console.web_terminal", RequiredInterface: "",
		Reason: "§11 \"M2+, with the agent ADR\" — 미선언"},
	// Phase 5 (PR 31b) — PVE guarded operations 3종은 같은 실행면을 공유한다:
	// executor 필수(등록 시 타입 단얫) + 폴러 옵션(UPID dual-mode — J4).
	// TaskCanceller는 미구현 — PVE 태스크 취소는 이월(계획 §13 이월 유지).
	{Capability: "compute.power.manage", RequiredInterface: "OperationExecutor",
		OptionalInterfaces: []string{"TaskPoller"},
		Reason:             "Phase 5 PVE — POST …?background_delay → null|UPID dual mode (J4)"},
	{Capability: "storage.snapshot.manage", RequiredInterface: "OperationExecutor",
		OptionalInterfaces: []string{"TaskPoller"},
		Reason:             "동일 실행면 — POST …/snapshot → dual mode (J4·J7)"},
	{Capability: "compute.config.apply", RequiredInterface: "OperationExecutor",
		OptionalInterfaces: []string{"TaskPoller"},
		Reason:             "동일 실행면 — PUT …/config 화이트리스트(E-5) → dual mode (J4·J7)"},
}

// OperationStatus.State vocabulary (§3.9 — the Phase 0 inheritance
// "종단 상태는 Phase 1 폴러 설계 시점에 확정", resolved here). Succeeded/Failed
// are terminal; Running means the provider handle is still in flight and the
// task stays running — poll cycles are attempts, not a distinct state (§13.5).
// timed_out is engine-derived (CallTimeout/Deadline) and cancellation is a
// task-layer concern — neither is an adapter state.
const (
	OperationStateRunning   = "running"
	OperationStateSucceeded = "succeeded"
	OperationStateFailed    = "failed"
)

// IsTerminalOperationState reports whether an adapter-reported state ends
// the operation (§3.9).
func IsTerminalOperationState(state string) bool {
	return state == OperationStateSucceeded || state == OperationStateFailed
}

// §7.4 credential-binding purpose vocabulary, promoted to contract constants
// by the §3.6 extension (PR 17 — plan §3.5). The secrets package's
// CredentialPurposes slice stays the broker-side copy until its owning PR
// syncs it to these constants; the values are identical.
const (
	CredentialPurposeInventory  = "inventory"
	CredentialPurposeOperations = "operations"
	CredentialPurposeBilling    = "billing"
	CredentialPurposeConsole    = "console"
	CredentialPurposeMonitoring = "monitoring"
	CredentialPurposeBackup     = "backup"
)
