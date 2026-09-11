// opdef_table_test.go — P2-E: 오퍼레이션 def 확정표 1:1 잠금(계획 r3 §J-P1-6·
// PC-8·G-P2c). 쓰기 오퍼레이션의 등재처 재해석(J-P1-7·계획 §10 미해결 5 — P2-E 확정)이 착지하는 지점이다:
// plan·execute는 파라미터 라우트라 sensitive-routes 골든에 10행 실기가 물리적
// 불가능하므로, 등재처는 코드(compose_k8s_ops.go opdef 변수 + compose.go
// restartOperation)와 본 표 테스트다 — descriptors are code(§3.3). 골든
// 292·452 불변은 유지 원칙이고 본 테스트는 골든을 읽지 않는다(대조면은 골든
// 생성기 입력 opdef.All() — 소재 사슬의 올바른 수준).
//
// 잠금 칼럼(전칸럼): RequiredPermission·RiskLevel·RequiresApproval·
// RequiredCapability·ResourceKinds·IdempotencyPolicy. 리터럴이 권위다 — 어댑터
// 상수 참조가 아니라 문자열을 잠근다(상수 재명명·권한 교체가 게이트를 우회하지
// 못한다).
package contracttest

import (
	"net/http"
	"reflect"
	"slices"
	"sort"
	"testing"

	"ops-admin/backend/internal/infra/compose"
	"ops-admin/backend/internal/testutil"
	"ops-admin/backend/opdef"
)

// resourceMutationKinds — apply·delete 공유 kind 면(compose_k8s_ops.go
// resourceMutationKinds와 1:1 — executor_resource.go 판단 기록: node·pod는 uid
// 신원이라 제외, istio는 P2-D 앵커, endpoints·storageclass는 v1 face 부재).
var resourceMutationKinds = []string{
	"orchestration.workload",
	"orchestration.namespace",
	"orchestration.configmap",
	"orchestration.secret",
	"network.load_balancer",
	"network.gateway",
	"network.http_route",
	"storage.volume",
}

// k8sOpDefTable — 계획 r3 §J-P1-6 확정표의 Go 이행. 11행 = restart(기존) +
// P2-A~D 신규 9종 + I10 k8s.resource.create 1종(§3.2 — 빈 kinds는 커넥션-스코프
// 부호화: kind 면은 매니페스트 매핑 표 contract.K8sCreateFace가 소유). 순서는
// 확정표 행 순서를 따르고 I10 행은 확정표 뒤에 붙인다.
var k8sOpDefTable = []struct {
	name        string
	permission  string
	risk        string
	approval    bool
	capability  string
	kinds       []string
	idempotency string
}{
	{
		name:        "k8s.workload.restart", // 기존 — compose.go restartOperation
		permission:  "assets:k8s:workload:restart",
		risk:        "medium",
		approval:    true,
		capability:  "orchestration.kubernetes.apply",
		kinds:       []string{"orchestration.workload"},
		idempotency: "provider_frozen_annotation",
	},
	{
		name:        "k8s.workload.scale",
		permission:  "assets:k8s:workload:scale",
		risk:        "medium",
		approval:    true,
		capability:  "orchestration.kubernetes.apply",
		kinds:       []string{"orchestration.workload"},
		idempotency: "provider_state_convergent",
	},
	{
		name:        "k8s.workload.image_update",
		permission:  "assets:k8s:workload:image",
		risk:        "medium",
		approval:    true,
		capability:  "orchestration.kubernetes.apply",
		kinds:       []string{"orchestration.workload"},
		idempotency: "provider_frozen_payload",
	},
	{
		name:        "k8s.workload.resources_update",
		permission:  "assets:k8s:workload:yaml",
		risk:        "medium",
		approval:    true,
		capability:  "orchestration.kubernetes.apply",
		kinds:       []string{"orchestration.workload"},
		idempotency: "provider_frozen_payload",
	},
	{
		name:        "k8s.node.labels_update",
		permission:  "assets:k8s:workload:yaml",
		risk:        "medium",
		approval:    true,
		capability:  "orchestration.kubernetes.apply",
		kinds:       []string{"orchestration.node"},
		idempotency: "provider_state_convergent",
	},
	{
		name:        "k8s.service.update",
		permission:  "assets:k8s:workload:yaml",
		risk:        "medium",
		approval:    true,
		capability:  "orchestration.kubernetes.apply",
		kinds:       []string{"network.load_balancer"},
		idempotency: "provider_state_convergent",
	},
	{
		name:        "k8s.resource.apply",
		permission:  "assets:k8s:workload:yaml",
		risk:        "high",
		approval:    true,
		capability:  "orchestration.kubernetes.apply",
		kinds:       resourceMutationKinds,
		idempotency: "provider_frozen_manifest",
	},
	{
		name:        "k8s.resource.delete",
		permission:  "assets:k8s:resource:delete",
		risk:        "high",
		approval:    true,
		capability:  "orchestration.kubernetes.apply",
		kinds:       resourceMutationKinds,
		idempotency: "provider_terminal_404",
	},
	{
		name:        "k8s.istio.traffic_update",
		permission:  "assets:k8s:advancednetwork",
		risk:        "medium",
		approval:    true,
		capability:  "orchestration.kubernetes.apply",
		kinds:       []string{"network.virtual_service"},
		idempotency: "provider_frozen_payload",
	},
	{
		name:        "k8s.httproute.traffic_update",
		permission:  "assets:k8s:advancednetwork",
		risk:        "medium",
		approval:    true,
		capability:  "orchestration.kubernetes.apply",
		kinds:       []string{"network.http_route"},
		idempotency: "provider_frozen_payload",
	},
	{
		name:        "k8s.resource.create", // I10 §3.2 — 커넥션-스코프(빈 ResourceKinds 부호화)
		permission:  "assets:k8s:workload:yaml",
		risk:        "high",
		approval:    true,
		capability:  "orchestration.kubernetes.apply",
		kinds:       []string{},
		idempotency: "provider_create_convergent",
	},
}

// pveDefNames — 레지스트리에 등록된 pve opdef 3종(계획 P2 확정표 밖 — compose.go
// registerProxmoxMutations). 전체 등록 집합 닫기(total=13)의 대응면이다.
var pveDefNames = []string{"pve.guest.power", "pve.guest.snapshot", "pve.guest.config"}

// representativeRoutePaths — sensitive-routes.txt:157-158의 plan·execute
// 파라미터 라우트 2개. 대표 권한(restart)은 골든 생성기 입력 opdef.All()이
// 소유하고 요청별 분해는 opdef.V2DynamicMiddleware + ResolveOperationPermission이
// 집행한다(routes_v2.go:33-34).
var representativeRoutePaths = []string{
	"/infra/resources/:uid/operations/:name/plan",
	"/infra/resources/:uid/operations/:name/execute",
}

func TestOperationDefTable(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)

	stack, err := compose.Build(db)
	if err != nil {
		t.Fatalf("compose.Build: %v", err)
	}
	reg := stack.Registry

	// --- 등록 집합 닫기: 14 = k8s 11종 + pve 3종. G-P2a 등록 계수의 등록면
	// 대응물(I10 create 추가분 포함) — 고스트 opdef·무단 추가가 본 단얫으로
	// 막힌다. ---
	ops := reg.Operations()
	if len(ops) != len(k8sOpDefTable)+len(pveDefNames) {
		t.Fatalf("registry holds %d operations, want %d (k8s %d + pve %d)",
			len(ops), len(k8sOpDefTable)+len(pveDefNames), len(k8sOpDefTable), len(pveDefNames))
	}
	gotNames := make([]string, 0, len(ops))
	for _, def := range ops {
		gotNames = append(gotNames, def.Name)
	}
	wantNames := make([]string, 0, len(k8sOpDefTable)+len(pveDefNames))
	for _, row := range k8sOpDefTable {
		wantNames = append(wantNames, row.name)
	}
	wantNames = append(wantNames, pveDefNames...)
	sort.Strings(wantNames)
	if !slices.Equal(gotNames, wantNames) {
		t.Errorf("registered operation names = %v, want %v (def table + pve trio)", gotNames, wantNames)
	}

	// --- 전칸럼 잠금: 11행 × 6칼럼. 확정표의 승인 열은 "필수" — 전 행 true,
	// risk high는 apply·delete·create 3종(PC-8 "risk high" 단얫의 I10 확장 —
	// create는 임의 kind 면이라 apply·delete 상향 선례를 승계; 권한 단얫은 행
	// 루프의 permission 비교가 곧 그것이다; yaml 공유 행이 create로 1개 늘어
	// 5행이 된다). ---
	highRisk := 0
	for _, row := range k8sOpDefTable {
		def, ok := reg.Operation(row.name)
		if !ok {
			t.Errorf("operation %q not registered", row.name)
			continue
		}
		if def.RequiredPermission != row.permission {
			t.Errorf("%s: RequiredPermission = %q, want %q", row.name, def.RequiredPermission, row.permission)
		}
		if def.RiskLevel != row.risk {
			t.Errorf("%s: RiskLevel = %q, want %q", row.name, def.RiskLevel, row.risk)
		}
		if def.RequiresApproval != row.approval {
			t.Errorf("%s: RequiresApproval = %v, want %v (확정표 승인 열 — 전 opdef 필수)", row.name, def.RequiresApproval, row.approval)
		}
		if def.RequiredCapability != row.capability {
			t.Errorf("%s: RequiredCapability = %q, want %q", row.name, def.RequiredCapability, row.capability)
		}
		gotKinds := append([]string(nil), def.ResourceKinds...)
		sort.Strings(gotKinds)
		wantKinds := append([]string(nil), row.kinds...)
		sort.Strings(wantKinds)
		if !slices.Equal(gotKinds, wantKinds) {
			t.Errorf("%s: ResourceKinds = %v, want %v", row.name, gotKinds, wantKinds)
		}
		if def.IdempotencyPolicy != row.idempotency {
			t.Errorf("%s: IdempotencyPolicy = %q, want %q", row.name, def.IdempotencyPolicy, row.idempotency)
		}

		// 공통 posture(확정표 밖 — compose_k8s_ops.go 선언 주석과 정합): 전 행
		// mutating·버전 1·redaction 스펙 소유. Timeout·Retry는 실행기 역학이라
		// 본 표가 잠그지 않는다.
		if !def.Mutating {
			t.Errorf("%s: Mutating = false, want true (쓰기 오퍼레이션 확정표)", row.name)
		}
		if def.Version != "1" {
			t.Errorf("%s: Version = %q, want \"1\" (단일 버전 레지스트리 A13)", row.name, def.Version)
		}
		if def.Redaction == nil {
			t.Errorf("%s: Redaction = nil, want a typed spec (§10.2)", row.name)
		} else if reflect.TypeOf(def.Redaction()).Kind() != reflect.Struct {
			t.Errorf("%s: redaction spec is %s, want a struct", row.name, reflect.TypeOf(def.Redaction()).Kind())
		}

		if row.risk == "high" {
			highRisk++
		}
	}
	if highRisk != 3 {
		t.Errorf("high-risk opdef count = %d, want 3 (apply·delete·create — §J-P1-6 risk 상향 + I10 §3.2)", highRisk)
	}

	// --- sensitive 대표 행 정합(J-P1-7 등재처 재해석의 기계 면): plan·execute
	// 대표행의 권한·risk·mutating은 restart def 행과 정합해야 한다 — 골든
	// 292행의 :157-158이 그 대표행에서 생성되므로, 본 단얫이 registry def ↔
	// opdef 대표행 ↔ 골든 사슬을 잠근다. ---
	restartDef, ok := reg.Operation("k8s.workload.restart")
	if !ok {
		t.Fatal("restart operation not registered — representative anchor missing")
	}
	allDefs := opdef.All()
	for _, path := range representativeRoutePaths {
		var rep *opdef.Def
		for i := range allDefs {
			if allDefs[i].Method == http.MethodPost && allDefs[i].Path == path {
				rep = &allDefs[i]
				break
			}
		}
		if rep == nil {
			t.Errorf("opdef.All() holds no POST row for %q (sensitive 대표행 — 골든 :157-158의 생성원)", path)
			continue
		}
		if rep.Permission != restartDef.RequiredPermission {
			t.Errorf("%s: representative permission = %q, want restart def %q", path, rep.Permission, restartDef.RequiredPermission)
		}
		if rep.Risk != restartDef.RiskLevel {
			t.Errorf("%s: representative risk = %q, want restart def %q", path, rep.Risk, restartDef.RiskLevel)
		}
		if !rep.Mutating {
			t.Errorf("%s: representative row not mutating", path)
		}
	}
}
