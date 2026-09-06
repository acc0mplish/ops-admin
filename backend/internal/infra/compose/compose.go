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
