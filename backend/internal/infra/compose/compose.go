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
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/internal/infra/metrics"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/infra/secrets"
	"ops-admin/backend/internal/tasks"
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
	// The fake stays a first-class registered adapter (§11.1) — its harness
	// scenarios keep the measurement instrument reachable in every stack.
	if err := fake.Register(reg); err != nil {
		return nil, err
	}

	broker := secrets.NewBroker(db)
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
