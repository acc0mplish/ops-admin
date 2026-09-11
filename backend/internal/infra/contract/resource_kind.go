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

// Phase6ResourceKindExtensions — P 계획 J-P1-0(r3)이 등재하는 kind 어휘 확장
// 4종. Phase2ResourceKindExtensions과 같은 reviewed-diff 경로(§8.5 어휘 확장)
// 으로 착지하며 M1ResourceKinds는 19종 verbatim 보존 계약(T1)이라 건드리지
// 않는다 — IsKnownResourceKind가 확장 슬라이스를 함께 조회하는 구조를 승계한다.
// 게이트웨이·HTTPRoute는 P1-C1 수집기, endpoint는 service.endpoints 집계 원천,
// virtualservice는 P2-D istio 오퍼레이션 uid 앵커다(비교 집합 불참 — I-P5 이월).
var Phase6ResourceKindExtensions = []string{
	"network.gateway",
	"network.http_route",
	"network.endpoint",
	"network.virtual_service",
}

// I10ResourceKindExtensions — I10 계획(§3.2.1)이 등재하는 kind 어휘 확장
// 2종. Phase2·Phase6 확장과 같은 reviewed-diff 경로(§8.5)로 착지하며
// M1ResourceKinds 19종 verbatim 보존 계약(T1)은 그대로다. destinationrule·
// serviceentry는 v1 create face 어휘에는 존재하지만 §8.5 초기·기존 확장
// 어느쪽에도 없어 신설한다 — network.virtual_service P2-D 선례를 승계하는
// "오퍼레이션 앵커 전용" kind다(수집·비교 집합 불참 — create 매니페스트
// 매핑 표 k8s_create_face.go만 소비).
var I10ResourceKindExtensions = []string{
	"network.destination_rule",
	"network.service_entry",
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
	for _, k := range Phase6ResourceKindExtensions {
		if k == kind {
			return true
		}
	}
	for _, k := range I10ResourceKindExtensions {
		if k == kind {
			return true
		}
	}
	return false
}
