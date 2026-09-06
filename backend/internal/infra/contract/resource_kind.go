package contract

// §8.5 "Initial common kinds" — 순서·철자는 스펙 그대로.
var M1ResourceKinds = []string{
	"machine.baremetal",
	"compute.hypervisor_node",
	"compute.vm",
	"compute.system_container",
	"compute.image",
	"compute.template",
	"orchestration.cluster",
	"orchestration.node",
	"orchestration.namespace",
	"orchestration.workload",
	"orchestration.pod",
	"network.segment",
	"network.interface",
	"network.ip",
	"network.load_balancer",
	"storage.pool",
	"storage.volume",
	"storage.snapshot",
	"identity.account",
}

// Phase2ResourceKindExtensions — §8.5 어휘 확장 2종 (Phase 2 PR 20, J9 판정:
// "Vocabulary extensions land as reviewed diffs to M1ResourceKinds in the
// owning milestone's PR" 경로의 착지). M1ResourceKinds는 스펙 §8.5 19종
// verbatim 보존 계약(T1 단언)이라 확장분은 별도 슬라이스로 적재하고
// IsKnownResourceKind가 두 슬라이스를 모두 조회한다 — 어휘 조회부는 확장과
// 동시에 자동 인지(계획 R11 완화).
var Phase2ResourceKindExtensions = []string{
	"orchestration.configmap",
	"orchestration.secret",
}

// IsKnownResourceKind reports whether kind is in the closed M1 vocabulary
// (§8.5) or its reviewed extension slices — not as unguarded registrations
// (A9).
func IsKnownResourceKind(kind string) bool {
	for _, k := range M1ResourceKinds {
		if k == kind {
			return true
		}
	}
	for _, k := range Phase2ResourceKindExtensions {
		if k == kind {
			return true
		}
	}
	return false
}
