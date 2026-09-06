// Package compose is the V2 infra assembly root (plan §2 PR 21 file 8):
// one place builds the registry (fake + kubernetes adapters), the secrets
// broker, the shared metrics counters, and the SyncRunner. The CLI
// (sync-inventory) and the Phase 3 v2 read API consume the same stack —
// adapters register exactly once, and the counters the adapter REST wrapper
// increments are the same set the report artifacts render (J6).
package compose

import (
	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/adapter/fake"
	"ops-admin/backend/internal/infra/adapter/kubernetes"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/internal/infra/metrics"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/infra/secrets"
)

// Stack is the assembled V2 inventory stack.
type Stack struct {
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
	// The fake stays a first-class registered adapter (§11.1) — its harness
	// scenarios keep the measurement instrument reachable in every stack.
	if err := fake.Register(reg); err != nil {
		return nil, err
	}

	broker := secrets.NewBroker(db)
	return &Stack{
		Registry: reg,
		Broker:   broker,
		Counters: counters,
		Runner:   inventory.NewSyncRunner(db, reg, broker, counters),
	}, nil
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
