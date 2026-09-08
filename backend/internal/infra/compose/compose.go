// Package compose is the V2 infra assembly root (plan §2 PR 21 file 8):
// one place builds the registry (fake + kubernetes adapters), the secrets
// broker, the shared metrics counters, and the SyncRunner. The CLI
// (sync-inventory) and the Phase 3 v2 read API consume the same stack —
// adapters register exactly once, and the counters the adapter REST wrapper
// increments are the same set the report artifacts render (J6).
package compose

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/adapter/aliyun"
	"ops-admin/backend/internal/infra/adapter/fake"
	"ops-admin/backend/internal/infra/adapter/kubernetes"
	"ops-admin/backend/internal/infra/adapter/proxmox"
	"ops-admin/backend/internal/infra/adapter/tencent"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/internal/infra/metrics"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/infra/secrets"
	"ops-admin/backend/internal/tasks"
	"ops-admin/backend/store"
)

// Stack is the assembled V2 inventory stack.
type Stack struct {
	db       *gorm.DB
	Registry *registry.Registry
	Broker   *secrets.Broker
	Counters *metrics.Counters
	Runner   *inventory.SyncRunner
}

// Build assembles the stack over an open database handle. Registration-time
// validation failures (registry V1–V6) surface here as errors.
func Build(db *gorm.DB) (*Stack, error) {
	counters := metrics.New()
	// The adapter pre-registers its own row on its internal set; the shared
	// set gets the row here so the report render always carries the
	// {provider="kubernetes"} 0 line the gate ③ flat proof reads (J6).
	counters.RegisterProviders(kubernetes.ProviderName)
	counters.RegisterProviders(tencent.ProviderName)
	counters.RegisterProviders(kubernetes.ProviderName, aliyun.ProviderName)
	reg := registry.New()

	// The kubernetes adapter shares the stack's counters so the sync report
	// carries the rate-limit lines its own REST wrapper increments (J6).
	k8s := kubernetes.NewAdapter(kubernetes.WithCounters(counters))
	if err := reg.RegisterProviderType(k8s.Descriptor(), k8s); err != nil {
		return nil, err
	}
	// Phase 3 B(M4): kubernetes capability 3종 + restart opdef 등록. apply는
	// §3.7 매핑 표가 OperationExecutor를 요구한다(V5) — 어댑터가 executor를
	// 구현한 같은 Phase에 선언이 내려온다(§0.4 registry V5 요구의 이행).
	if err := registerKubernetes(reg); err != nil {
		return nil, err
	}
	// Phase 4 A (PR 27): aliyun 어댑터 등록 — 읽기 전용 inventory(계획 §5
	// Phase A, 판정 J10). capability 선언은 등록의 후행 계약(V4·V5 — §3.7
	// 매핑 표가 compute.vm.read에 Discoverer를 요구하고 어댑터가 그것을
	// 구현하므로 등록 시 타입 단얫이 통과한다).
	ali := aliyun.NewAdapter(aliyun.WithCounters(counters))
	if err := reg.RegisterProviderType(ali.Descriptor(), ali); err != nil {
		return nil, err
	}
	if err := registerAliyun(reg); err != nil {
		return nil, err
	}
	// Phase 4 B(M1 tencent분): the tencent inventory adapter (PR 28 — TC3
	// 직구현, 판정 J2(c)) shares the stack's counters and declares its read
	// capability pair over the compute.vm kind it discovers (판정 J10 —
	// inventory.full·compute.vm.read, 둘 다 §3.7 매핑 표가 Discoverer 필수).
	tn := tencent.NewAdapter(tencent.WithCounters(counters))
	if err := reg.RegisterProviderType(tn.Descriptor(), tn); err != nil {
		return nil, err
	}
	if err := reg.RegisterCapabilities(tencent.ProviderName,
		contract.Capability{
			Name:          "inventory.full",
			Version:       "1",
			ResourceKinds: []string{tencent.KindVM},
			ReadOnly:      true,
		},
		contract.Capability{
			Name:          "compute.vm.read",
			Version:       "1",
			ResourceKinds: []string{tencent.KindVM},
			ReadOnly:      true,
		},
	); err != nil {
		return nil, err
	}
	// Phase 5 C2 (PR 31a): the proxmox read-only adapter (BaseAdapter +
	// Discoverer) shares the stack's counters and declares the single read
	// capability over its whole discovery kind surface (§3.4 —
	// inventory.full의 ResourceKinds는 DiscoveryKinds 전수). mutation
	// capability·opdef 등록은 Phase D(31b) 소관 — executor 구현과 같은
	// Phase에 내려온다(§0.4 registry V5 요구의 이행).
	counters.RegisterProviders(proxmox.ProviderName)
	px := proxmox.NewAdapter(proxmox.WithCounters(counters))
	if err := reg.RegisterProviderType(px.Descriptor(), px); err != nil {
		return nil, err
	}
	if err := reg.RegisterCapabilities(proxmox.ProviderName,
		contract.Capability{
			Name:          "inventory.full",
			Version:       "1",
			ResourceKinds: proxmox.DiscoveryKinds,
			ReadOnly:      true,
		},
	); err != nil {
		return nil, err
	}
	// Phase 5 D (PR 31b): the proxmox mutation capability trio and the guarded
	// operation definitions (J7). V5는 mutation capability 3종에
	// OperationExecutor를 요구한다 — executor 구현과 같은 커밋에 내려온다(§0.4
	// registry V5 요구의 이행 — k8s apply 선례와 동일 형상).
	if err := registerProxmoxMutations(reg); err != nil {
		return nil, err
	}
	// The fake stays a first-class registered adapter (§11.1) — its harness
	// scenarios keep the measurement instrument reachable in every stack.
	if err := fake.Register(reg); err != nil {
		return nil, err
	}

	// The broker shares the stack's counters so every successful Resolve feeds
	// the §18.2 secret_access_total{purpose, backend} family (J6 — 계기면
	// 공유 규약과 동일 형상).
	broker := secrets.NewBrokerWithCounters(db, counters)
	return &Stack{
		db:       db,
		Registry: reg,
		Broker:   broker,
		Counters: counters,
		Runner:   inventory.NewSyncRunner(db, reg, broker, counters),
	}, nil
}

// TerminalAuditFunc is the §18.1 terminal-row writer the engine's
// OnTaskTerminal hook delegates to (J3(b)/J6). It is the exact signature of
// v2.RecordTaskTerminal — injected by the caller (main) because compose
// cannot import the api layer (api/v2 imports compose; the dependency must
// point one way). nil is legal: the hook then owns the observation refresh
// alone.
type TerminalAuditFunc func(ctx context.Context, db *gorm.DB, task model.ProviderTask, status string, detail contract.JSONMap) error

// BuildEngine assembles the durable task engine (§13) over the stack's
// registry — the SAME registry Build registered the adapters and operation
// definitions into, so submit/claim/execute resolve identically for the CLI
// sync path and the engine lane (adapters register exactly once). The J6
// OnTaskTerminal hook is wired here: observation refresh first
// (SyncRunner.RunSync on the task's connection — status independent: a failed
// rollout's observation is still the truth of a partial rollout), then the
// §18.1 terminal audit row. A refresh failure never skips the audit row —
// both legs run, the errors join, and the engine's fireTaskTerminal disposes
// the joined error without rolling the terminal back (observation_refresh_failed
// event).
func (s *Stack) BuildEngine(cfg tasks.Config, terminalAudit TerminalAuditFunc) *tasks.Engine {
	engine := tasks.NewEngine(s.db, s.Registry, cfg)
	// §18.2 M1 — 엔진·리퍼·폴 루프가 스택의 공유 Counters에 적립한다(어댑터 REST
	// 래퍼·sync·broker와 같은 set — J6 계기면 공유 규약). /internal/metrics 렌더가
	// task·worker 패밀리(F/G/H/I/J)를 함께 내보내는 배선점.
	engine.Metrics = s.Counters
	engine.OnTaskTerminal = func(ctx context.Context, task model.ProviderTask, status string, detail contract.JSONMap) error {
		refreshErr := s.refreshObservation(ctx, task.ResourceUID)
		var auditErr error
		if terminalAudit != nil {
			auditErr = terminalAudit(ctx, s.db, task, status, detail)
		}
		return errors.Join(refreshErr, auditErr)
	}
	return engine
}

// refreshObservation resolves the task's resource → context → connection
// chain (the same join shape the engine's resolveExecutionChain walks) and
// runs one full sync of that connection — the whole-connection form (J6:
// kind 규모에서 충분; incremental is the Phase 4 handoff).
func (s *Stack) refreshObservation(ctx context.Context, resourceUID string) error {
	var connUID string
	err := s.db.WithContext(ctx).
		Table("infra_resource").
		Select("provider_connection.uid").
		Joins("JOIN provider_context ON provider_context.id = infra_resource.context_id").
		Joins("JOIN provider_connection ON provider_connection.id = provider_context.connection_id").
		Where("infra_resource.uid = ? AND infra_resource.deleted_at IS NULL", resourceUID).
		Limit(1).
		Scan(&connUID).Error
	if err != nil {
		return fmt.Errorf("compose: resolve connection for terminal refresh of %q: %w", resourceUID, err)
	}
	if connUID == "" {
		return fmt.Errorf("compose: no live resource→context→connection chain for %q — terminal refresh skipped", resourceUID)
	}
	if _, err := s.Runner.RunSync(ctx, inventory.SyncInput{ConnectionUID: connUID}); err != nil {
		return fmt.Errorf("compose: terminal observation refresh of %q: %w", connUID, err)
	}
	return nil
}

// kubernetesReadKinds — k8s 디스커버리가 관측하는 kind 면(§3.1 섹션 표 ↔
// mapping.md 정규화 표와 동치 — 읽기 capability 2종이 같은 면을 선언한다).
var kubernetesReadKinds = []string{
	"orchestration.node",
	"orchestration.namespace",
	"orchestration.pod",
	"orchestration.workload",
	"orchestration.configmap",
	"orchestration.secret",
	"network.load_balancer",
	"storage.volume",
	"storage.pool",
}

// registerKubernetes declares the kubernetes capability triple (§10.1 M1 어휘 —
// inventory.full·orchestration.kubernetes.read/apply) and the Phase 3 restart
// operation definition. 정의의 소재는 코드다(§3.3 — descriptors are code).
func registerKubernetes(reg *registry.Registry) error {
	if err := reg.RegisterCapabilities(kubernetes.ProviderName,
		contract.Capability{
			Name:          "inventory.full",
			Version:       "1",
			ResourceKinds: kubernetesReadKinds,
			ReadOnly:      true,
		},
		contract.Capability{
			Name:          "orchestration.kubernetes.read",
			Version:       "1",
			ResourceKinds: kubernetesReadKinds,
			ReadOnly:      true,
		},
		contract.Capability{
			// §10.1 "(Phase 3: restart only)" — apply가 지금 하는 일은 restart뿐.
			Name:          "orchestration.kubernetes.apply",
			Version:       "1",
			ResourceKinds: []string{"orchestration.workload"},
		},
	); err != nil {
		return err
	}
	return reg.RegisterOperation(restartOperation)
}

// registerAliyun declares the aliyun read capability pair (판정 J10 — §10.1
// 어휘): inventory.full + compute.vm.read, 둘 다 compute.vm 1종(E-3 — vm family
// 한정). §3.7 매핑 표가 두 capability 모두 Discoverer를 요구한다 — 어댑터의
// Discoverer 구현이 등록 시 타입 단얫으로 검증된다(V5). opdef 없음 — Phase 4는
// 읽기 전용(§20).
func registerAliyun(reg *registry.Registry) error {
	return reg.RegisterCapabilities(aliyun.ProviderName,
		contract.Capability{
			Name:          "inventory.full",
			Version:       "1",
			ResourceKinds: []string{"compute.vm"},
			ReadOnly:      true,
		},
		contract.Capability{
			Name:          "compute.vm.read",
			Version:       "1",
			ResourceKinds: []string{"compute.vm"},
			ReadOnly:      true,
		},
	)
}

// restartOperation — k8s.workload.restart 정의(§3.3).
var restartOperation = contract.OperationDefinition{
	Name:    kubernetes.RestartOperationName,
	Version: "1",
	// §10.3 "role grants carry over" — 이미 시드된 v1 권한 재사용(J4,
	// 신규 권한 문자열 0 — 보존 제약 #2).
	RequiredPermission: "assets:k8s:workload:restart",
	RequiredCapability: "orchestration.kubernetes.apply",
	ResourceKinds:      []string{"orchestration.workload"},
	Mutating:           true,
	RiskLevel:          "medium",
	// J8 — V2 레인 신중 posture(첫 프로덕션 mutation의 승인 실증).
	RequiresApproval: true,
	// J1 — provider 수준 멱등: 동결 restartedAt의 byte-identical patch.
	IdempotencyPolicy: "provider_frozen_annotation",
	TimeoutSeconds:    30,
	RetryPolicy:       contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
	// §10.2 typed redaction spec — 결과 detail의 허용 필드를 타입으로 명시.
	// executor의 성공 detail 키와 1:1(generation·restartedAt·readyReplicas·
	// updated·serverURL) — 그 밖의 키는 허용되지 않는다.
	Redaction: func() any { return restartResultRedaction{} },
}

// restartResultRedaction — restart 결과 detail의 허용 필드(§10.2).
type restartResultRedaction struct {
	Generation    int64  `json:"generation"`
	RestartedAt   string `json:"restartedAt"`
	ReadyReplicas int    `json:"readyReplicas"`
	Updated       int    `json:"updated"`
	ServerURL     string `json:"serverURL"`
}

// proxmoxGuestKinds — proxmox mutation capability·opdef가 통제하는 게스트 kind
// 면(J7 — [compute.vm, compute.system_container]).
var proxmoxGuestKinds = []string{"compute.vm", "compute.system_container"}

// pveGuestOperatePermission — E-2 승인 신규 권한 1종(4세그 — v1 permissionPattern
// 충족, opdef 3종 공유). 단일 원천은 store.PVEGuestOperatePermission — compose는
// 참조만 한다(p5 followup; seed.go·compose 2중 선언 통일).

// pveGuardedRetry — J7 공통 재시도 보수: MaxAttempts 2(재시도 파괴 가능성의
// 완화 — R3)·BackoffSeconds 10(재시도는 승인 후 폴 회로에 한정하는 계약의
// 수치면 — A4).
var pveGuardedRetry = contract.RetryPolicy{MaxAttempts: 2, BackoffSeconds: 10}

// registerProxmoxMutations declares the proxmox mutation capability trio and the
// three guarded operation definitions (J7). 정의의 소재는 코드다(§3.3 —
// descriptors are code).
func registerProxmoxMutations(reg *registry.Registry) error {
	if err := reg.RegisterCapabilities(proxmox.ProviderName,
		contract.Capability{
			Name:          "compute.power.manage",
			Version:       "1",
			ResourceKinds: proxmoxGuestKinds,
		},
		contract.Capability{
			Name:          "storage.snapshot.manage",
			Version:       "1",
			ResourceKinds: proxmoxGuestKinds,
		},
		contract.Capability{
			Name:          "compute.config.apply",
			Version:       "1",
			ResourceKinds: proxmoxGuestKinds,
		},
	); err != nil {
		return err
	}
	if err := reg.RegisterOperation(pveGuestPowerOperation); err != nil {
		return err
	}
	if err := reg.RegisterOperation(pveGuestSnapshotOperation); err != nil {
		return err
	}
	return reg.RegisterOperation(pveGuestConfigOperation)
}

// pveGuestPowerOperation — pve.guest.power(J7): 4 액션(start|shutdown|stop|
// reboot)을 1 opdef로 묶는다(k8s restart 1종 선례 — 동일 승인자 인구, 액션 통제는
// executor의 payload 검증이 담당). IdempotencyPolicy는 A4 — 전원 상태는 상태
// 수렴형이다(이미 켜진 VM의 start는 PVE가 오류 반환).
var pveGuestPowerOperation = contract.OperationDefinition{
	Name:               proxmox.PowerOperationName,
	Version:            "1",
	RequiredPermission: store.PVEGuestOperatePermission,
	RequiredCapability: "compute.power.manage",
	ResourceKinds:      proxmoxGuestKinds,
	Mutating:           true,
	RiskLevel:          "medium",
	// J7 — guarded posture(승인 후에만 발행된다).
	RequiresApproval:  true,
	IdempotencyPolicy: "provider_state_convergent",
	TimeoutSeconds:    30,
	RetryPolicy:       pveGuardedRetry,
	// §10.2 typed redaction spec — 허용 필드는 §3.2 표의 op별 집합이고 executor가
	// 실제로 싣는 것은 그 부분집합이다(stateless 핸들 계약 — 폴 시점에 재현
	// 가능한 성분만 흘린다).
	Redaction: func() any { return pvePowerResultRedaction{} },
}

// pveGuestSnapshotOperation — pve.guest.snapshot(J7): snapname 필수 — POST
// …/snapshot.
var pveGuestSnapshotOperation = contract.OperationDefinition{
	Name:               proxmox.SnapshotOperationName,
	Version:            "1",
	RequiredPermission: store.PVEGuestOperatePermission,
	RequiredCapability: "storage.snapshot.manage",
	ResourceKinds:      proxmoxGuestKinds,
	Mutating:           true,
	RiskLevel:          "medium",
	RequiresApproval:   true,
	IdempotencyPolicy:  "provider_state_convergent",
	TimeoutSeconds:     30,
	RetryPolicy:        pveGuardedRetry,
	Redaction:          func() any { return pveSnapshotResultRedaction{} },
}

// pveGuestConfigOperation — pve.guest.config(J7): 화이트리스트(E-5) — {cores?,
// memoryMB?} 외 키 거부 — PUT …/config. 구성 변경은 게스트 가용성에 직결되므로
// 리스크 high로 분화한다.
var pveGuestConfigOperation = contract.OperationDefinition{
	Name:               proxmox.ConfigOperationName,
	Version:            "1",
	RequiredPermission: store.PVEGuestOperatePermission,
	RequiredCapability: "compute.config.apply",
	ResourceKinds:      proxmoxGuestKinds,
	Mutating:           true,
	RiskLevel:          "high",
	RequiresApproval:   true,
	IdempotencyPolicy:  "provider_state_convergent",
	TimeoutSeconds:     30,
	RetryPolicy:        pveGuardedRetry,
	Redaction:          func() any { return pveConfigResultRedaction{} },
}

// pve 결과 redaction 3종 — §10.2 typed specs. 허용 필드는 §3.2 표(성공 detail
// 허용 필드)와 1:1이다: {node, vmid, guestType, action?, snapname?, upid?,
// exitStatus, cores?, memoryMB?}.
type pvePowerResultRedaction struct {
	Node       string `json:"node"`
	VMID       string `json:"vmid"`
	GuestType  string `json:"guestType"`
	Action     string `json:"action"`
	UPID       string `json:"upid"`
	ExitStatus string `json:"exitStatus"`
}

type pveSnapshotResultRedaction struct {
	Node       string `json:"node"`
	VMID       string `json:"vmid"`
	GuestType  string `json:"guestType"`
	Snapname   string `json:"snapname"`
	UPID       string `json:"upid"`
	ExitStatus string `json:"exitStatus"`
}

type pveConfigResultRedaction struct {
	Node       string `json:"node"`
	VMID       string `json:"vmid"`
	GuestType  string `json:"guestType"`
	UPID       string `json:"upid"`
	ExitStatus string `json:"exitStatus"`
	Cores      int64  `json:"cores"`
	MemoryMB   int64  `json:"memoryMB"`
}
