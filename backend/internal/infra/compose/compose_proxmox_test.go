// compose_proxmox_test.go — Phase 5 proxmox 등록 단얫(C2 읽기 1종 + D mutation
// 3종·opdef 3종). compose_test.go가 650라인 경고선을 넘어 전역 코딩 규칙에 따라
// 테스트 미러링 시접(k8s/tencent 절과 동일 구분)으로 분리했다 — 단얫 내용의
// 변경은 없다(같은 패키지 compose_test).
package compose_test

import (
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/adapter/proxmox"
	"ops-admin/backend/internal/infra/compose"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/testutil"
)

// TestBuildRegistersProxmoxInventory — Phase 5 C2(M3 31a분/M4): compose는
// proxmox 읽기 전용 어댑터(계획 §3.4 — BaseAdapter + Discoverer)와 그 읽기
// capability 1종을 등록한다. inventory.full의 ResourceKinds는 어댑터
// DiscoveryKinds 전수다 — §3.7 매핑 표가 inventory.full에 Discoverer를 요구하므로
// 등록이 받아졌다는 것은 *Adapter가 등록 시 Discoverer 타입 단얫(V5)을 통과했다는
// 뜻이다. (31b) mutation capability 3종의 등록 단얫은
// TestBuildRegistersProxmoxOperations가 소유한다.
func TestBuildRegistersProxmoxInventory(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)

	stack, err := compose.Build(db)
	if err != nil {
		t.Fatalf("compose.Build: %v", err)
	}

	caps := stack.Registry.Capabilities("proxmox")
	got := map[string]contract.Capability{}
	for _, c := range caps {
		got[c.Name] = c
	}
	c, ok := got["inventory.full"]
	if !ok {
		t.Fatalf("proxmox capabilities = %v, missing inventory.full", caps)
	}
	if !c.ReadOnly {
		t.Error("capability inventory.full must be read-only (31a는 읽기 전용 — §3.4)")
	}
	if !slices.Equal(c.ResourceKinds, proxmox.DiscoveryKinds) {
		t.Errorf("capability ResourceKinds = %v, want the adapter DiscoveryKinds %v", c.ResourceKinds, proxmox.DiscoveryKinds)
	}

	// 공유 카운터에 proxmox provider 행이 있다(게이트 ③ flat 증명 형식).
	if want := `provider_rate_limit_total{provider="proxmox"} 0`; !strings.Contains(stack.Counters.Render(), want) {
		t.Errorf("counters render missing %q:\n%s", want, stack.Counters.Render())
	}
}

// TestBuildRegistersProxmoxOperations — Phase 5 D(M3 31b분): compose는 proxmox
// mutation capability 3종과 guarded opdef 3종(J7)을 등록한다. V5 실증: 3 capability
// 모두 §3.7 매핑 표가 OperationExecutor(옵션 TaskPoller)를 요구하고 어댑터가 둘 다
// 구현하므로 등록 통과가 곧 타입 단얫의 증거다 — executor 구현과 같은 커밋(§0.4).
func TestBuildRegistersProxmoxOperations(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)

	stack, err := compose.Build(db)
	if err != nil {
		t.Fatalf("compose.Build: %v", err)
	}

	// mutation capability 3종 — ReadOnly false·ResourceKinds는 게스트 kind 면
	// ([compute.vm, compute.system_container] — J7).
	caps := stack.Registry.Capabilities("proxmox")
	got := map[string]contract.Capability{}
	for _, c := range caps {
		got[c.Name] = c
	}
	for _, name := range []string{"compute.power.manage", "storage.snapshot.manage", "compute.config.apply"} {
		c, ok := got[name]
		if !ok {
			t.Errorf("proxmox capabilities = %v, missing mutation capability %q (Phase D — J7)", caps, name)
			continue
		}
		if c.ReadOnly {
			t.Errorf("mutation capability %q must not be read-only", name)
		}
		if !slices.Equal(c.ResourceKinds, []string{"compute.vm", "compute.system_container"}) {
			t.Errorf("capability %q ResourceKinds = %v, want [compute.vm compute.system_container]", name, c.ResourceKinds)
		}
	}

	// opdef 3종 — 공통 posture(J7·E-2): infra:pve:guest:operate 1종 공유·
	// Mutating·RequiresApproval·provider_state_convergent(A4)·Timeout 30·
	// Retry{2, 10}. Risk는 medium/medium/high로 분화.
	for _, tc := range []struct {
		name       string
		capability string
		risk       string
	}{
		{proxmox.PowerOperationName, "compute.power.manage", "medium"},
		{proxmox.SnapshotOperationName, "storage.snapshot.manage", "medium"},
		{proxmox.ConfigOperationName, "compute.config.apply", "high"},
	} {
		def, ok := stack.Registry.Operation(tc.name)
		if !ok {
			t.Errorf("operation %q not registered", tc.name)
			continue
		}
		if def.RequiredPermission != "infra:pve:guest:operate" {
			t.Errorf("operation %q RequiredPermission = %q, want infra:pve:guest:operate (E-2)", tc.name, def.RequiredPermission)
		}
		if def.RequiredCapability != tc.capability {
			t.Errorf("operation %q RequiredCapability = %q, want %q", tc.name, def.RequiredCapability, tc.capability)
		}
		if !slices.Equal(def.ResourceKinds, []string{"compute.vm", "compute.system_container"}) {
			t.Errorf("operation %q ResourceKinds = %v, want [compute.vm compute.system_container]", tc.name, def.ResourceKinds)
		}
		if !def.Mutating {
			t.Errorf("operation %q must be mutating", tc.name)
		}
		if def.RiskLevel != tc.risk {
			t.Errorf("operation %q RiskLevel = %q, want %q", tc.name, def.RiskLevel, tc.risk)
		}
		if !def.RequiresApproval {
			t.Errorf("operation %q must require approval (J7 — guarded)", tc.name)
		}
		if def.IdempotencyPolicy != "provider_state_convergent" {
			t.Errorf("operation %q IdempotencyPolicy = %q, want provider_state_convergent (A4)", tc.name, def.IdempotencyPolicy)
		}
		if def.TimeoutSeconds != 30 {
			t.Errorf("operation %q TimeoutSeconds = %d, want 30", tc.name, def.TimeoutSeconds)
		}
		if def.RetryPolicy != (contract.RetryPolicy{MaxAttempts: 2, BackoffSeconds: 10}) {
			t.Errorf("operation %q RetryPolicy = %+v, want {MaxAttempts:2, BackoffSeconds:10} (J7 — 낮은 MaxAttempts로 파괴 재시도 완화)", tc.name, def.RetryPolicy)
		}

		// §10.2 typed redaction spec — 허용 필드는 §3.2 표의 op별 집합이고
		// executor가 실제로 싣는 것은 그 부분집합이다(stateless 핸들 계약).
		if def.Redaction == nil {
			t.Errorf("operation %q carries no redaction spec", tc.name)
			continue
		}
		spec := reflect.TypeOf(def.Redaction())
		if spec.Kind() != reflect.Struct {
			t.Fatalf("redaction spec of %q is %s, want a struct", tc.name, spec.Kind())
		}
		fields := make([]string, 0, spec.NumField())
		for i := 0; i < spec.NumField(); i++ {
			tag := strings.SplitN(spec.Field(i).Tag.Get("json"), ",", 2)[0]
			if tag == "" || tag == "-" {
				t.Errorf("redaction field %q of %q lacks a json tag", spec.Field(i).Name, tc.name)
				continue
			}
			fields = append(fields, tag)
		}
		sort.Strings(fields)
		var want []string
		switch tc.name {
		case proxmox.PowerOperationName:
			want = []string{"action", "exitStatus", "guestType", "node", "upid", "vmid"}
		case proxmox.SnapshotOperationName:
			want = []string{"exitStatus", "guestType", "node", "snapname", "upid", "vmid"}
		default:
			want = []string{"cores", "exitStatus", "guestType", "memoryMB", "node", "upid", "vmid"}
		}
		if !slices.Equal(fields, want) {
			t.Errorf("redaction allowed fields of %q = %v, want %v (§3.2 표)", tc.name, fields, want)
		}
	}

	// V5 매핑 표의 실행 인터페이스 요구 — 등록된 어댑터가 실제로 둘 다 구현한다.
	_, adapter, ok := stack.Registry.ProviderType("proxmox")
	if !ok {
		t.Fatal("proxmox provider type not registered")
	}
	if _, ok := adapter.(contract.OperationExecutor); !ok {
		t.Error("proxmox adapter does not implement OperationExecutor (§3.7 mutation rows)")
	}
	if _, ok := adapter.(contract.TaskPoller); !ok {
		t.Error("proxmox adapter does not implement TaskPoller (J4 dual mode)")
	}
}
