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

// IsKnownResourceKind reports whether kind is in the closed M1 vocabulary
// (§8.5). Vocabulary extensions land as reviewed diffs to M1ResourceKinds
// in the owning milestone's PR — not as unguarded registrations (A9).
func IsKnownResourceKind(kind string) bool {
	for _, k := range M1ResourceKinds {
		if k == kind {
			return true
		}
	}
	return false
}
