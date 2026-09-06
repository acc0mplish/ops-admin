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

// M1CapabilityVocabulary is the closed M1 capability vocabulary (§10.1).
// ReadOnly is vocabulary metadata only — it is never used to derive a
// serving interface (r2, V5): console.web_terminal is served by ConsoleBroker
// (M2+, not declared here) and cost.read is readonly but not a Discoverer.
// The capability→interface mapping table is owned by the Phase 1 plan.
var M1CapabilityVocabulary = []CapabilityVocabularyEntry{
	{Name: "inventory.full", ReadOnly: true, OwnerPhase: "M1"},
	{Name: "inventory.incremental", ReadOnly: true, OwnerPhase: "M1"},
	{Name: "orchestration.kubernetes.read", ReadOnly: true, OwnerPhase: "M1"},
	{Name: "orchestration.kubernetes.apply", ReadOnly: false, OwnerPhase: "M1/Phase3"}, // §10.1 "(Phase 3: restart only)"
	{Name: "compute.vm.read", ReadOnly: true, OwnerPhase: "M1"},
	{Name: "cost.read", ReadOnly: true, OwnerPhase: "M1"},
	{Name: "console.web_terminal", ReadOnly: true, OwnerPhase: "M1"},
}
