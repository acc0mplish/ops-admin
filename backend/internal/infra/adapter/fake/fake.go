// Package fake is the first-class test adapter (§11.1 built-in list,
// §11 "fake is a first-class adapter, not an afterthought").
//
// Not the DNS provider domain at internal/domain/provider (spec §3 row 13,
// REMAIN) — this is a V2 infra adapter.
//
// Phase 0: registration only — Discoverer/inventory capabilities and
// behavior land with the Phase 1 contract test harness (가정 A7).
// Accordingly it implements neither Discoverer nor OperationExecutor in
// Phase 0, so the V5 minimum guard rejects any capability declaration on it.
package fake

import (
	"context"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/registry"
)

// Adapter is the fake BaseAdapter. Zero value is usable.
type Adapter struct{}

// Descriptor — Type:"fake", AdapterVersion:"1", ProtocolVersion:"1"(가정 A5),
// ConfigSchema: 빈 ConfigSpec, ContextKinds: 없음, BuiltIn: true.
func (Adapter) Descriptor() contract.ProviderTypeDescriptor {
	return contract.ProviderTypeDescriptor{
		Type:            "fake",
		AdapterVersion:  "1",
		ProtocolVersion: "1",
		ConfigSchema:    func() contract.ConfigSpec { return contract.ConfigSpec{} },
		ContextKinds:    nil,
		BuiltIn:         true,
	}
}

// Validate always succeeds — the fake accepts any connection view.
func (Adapter) Validate(_ context.Context, _ contract.ConnectionView) error { return nil }

// Health always reports healthy.
func (Adapter) Health(_ context.Context, _ contract.ConnectionView) contract.HealthResult {
	return contract.HealthResult{Healthy: true}
}

// Close is a no-op.
func (Adapter) Close() error { return nil }

// Register registers the fake provider type with the registry.
func Register(r *registry.Registry) error {
	return r.RegisterProviderType(Adapter{}.Descriptor(), Adapter{})
}

// Compile-time interface conformance.
var _ contract.BaseAdapter = Adapter{}
