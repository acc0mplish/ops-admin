// Package registry implements the "Provider Type Registry" of spec §6 and
// the §11 registry validation rules (Phase 0 scope: V1–V6). Explicit
// composition, no global state (§11.1 "registered explicitly"): create a
// Registry with New, register, then read. Registration-time validation is
// authoritative; lookups are safe for concurrent use after registration.
package registry

import (
	"fmt"
	"regexp"
	"sort"
	"sync"

	"ops-admin/backend/internal/infra/contract"
)

// permissionPattern — V6 (가정 A14): RequiredPermission 허용 형식 2~4세그,
// 세그 내 하이픈 허용. v1 opdef 권한 unique 175건 전량 충족(claim 16).
var permissionPattern = regexp.MustCompile(`^[a-z0-9_-]+(:[a-z0-9_-]+){1,3}$`)

type providerEntry struct {
	descriptor contract.ProviderTypeDescriptor
	adapter    contract.BaseAdapter
}

// Registry holds provider types, capabilities by provider type, and
// operations. Zero value is not usable — use New.
type Registry struct {
	mu         sync.RWMutex
	providers  map[string]providerEntry
	caps       map[string]map[string]contract.Capability // providerType -> name -> capability
	operations map[string]contract.OperationDefinition   // name -> definition (single-version, A13)
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{
		providers:  make(map[string]providerEntry),
		caps:       make(map[string]map[string]contract.Capability),
		operations: make(map[string]contract.OperationDefinition),
	}
}

// RegisterProviderType registers a provider type descriptor with its adapter.
// V1: the type name must be in M1ProviderTypeNames; reserved names are
// rejected with the "descriptor lands with its milestone" reason (§7.1).
// V2: duplicate registration is rejected. V3: ContextKinds must be a subset
// of ProviderContextKinds (§7.3).
func (r *Registry) RegisterProviderType(d contract.ProviderTypeDescriptor, a contract.BaseAdapter) error {
	if d.Type == "" {
		return fmt.Errorf("registry: provider type name is empty")
	}
	if a == nil {
		return fmt.Errorf("registry: provider type %q: adapter is nil", d.Type)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	reserved := make(map[string]bool, len(contract.ReservedProviderTypeNames))
	for _, n := range contract.ReservedProviderTypeNames {
		reserved[n] = true
	}
	if reserved[d.Type] {
		return fmt.Errorf("registry: provider type %q is reserved; descriptor lands with its milestone (§7.1)", d.Type)
	}
	known := false
	for _, n := range contract.M1ProviderTypeNames {
		if n == d.Type {
			known = true
		}
	}
	if !known {
		return fmt.Errorf("registry: provider type %q: unknown name, not in the M1 vocabulary (§7.1)", d.Type)
	}
	if _, dup := r.providers[d.Type]; dup {
		return fmt.Errorf("registry: provider type %q already registered (descriptors are code — single registration point, §11)", d.Type)
	}
	ctxKinds := make(map[string]bool, len(contract.ProviderContextKinds))
	for _, k := range contract.ProviderContextKinds {
		ctxKinds[k] = true
	}
	for _, k := range d.ContextKinds {
		if !ctxKinds[k] {
			return fmt.Errorf("registry: provider type %q: unknown context kind %q (§7.3)", d.Type, k)
		}
	}
	r.providers[d.Type] = providerEntry{descriptor: d, adapter: a}
	return nil
}

// ProviderTypeNames returns the registered provider type names, sorted.
func (r *Registry) ProviderTypeNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for n := range r.providers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ProviderType looks up a registered provider type.
func (r *Registry) ProviderType(name string) (contract.ProviderTypeDescriptor, contract.BaseAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.providers[name]
	if !ok {
		return contract.ProviderTypeDescriptor{}, nil, false
	}
	return e.descriptor, e.adapter, true
}

// RegisterCapabilities attaches capabilities to a registered provider type.
// V4: the capability name must exist in M1CapabilityVocabulary, must not
// duplicate within the type, and every ResourceKind must be known (§8.5).
// V5 minimum guard: an adapter declaring a capability must implement at least
// one capability-serving interface (Discoverer or OperationExecutor); any
// capability declared by an adapter implementing neither is rejected.
// The per-capability interface mapping table is owned by the Phase 1 plan —
// no ReadOnly→interface dichotomy here (r2).
func (r *Registry) RegisterCapabilities(providerType string, caps ...contract.Capability) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.providers[providerType]; !ok {
		return fmt.Errorf("registry: capabilities for unregistered provider type %q (V4 premise)", providerType)
	}
	vocab := make(map[string]contract.CapabilityVocabularyEntry, len(contract.M1CapabilityVocabulary))
	for _, e := range contract.M1CapabilityVocabulary {
		vocab[e.Name] = e
	}
	existing := r.caps[providerType]
	// Two-pass: validate every element against the live vocabulary before any
	// write, so a failing element leaves the registry untouched (atomic batch —
	// registration-time validation is authoritative, §11).
	staged := make(map[string]bool, len(caps))
	for _, c := range caps {
		if _, ok := vocab[c.Name]; !ok {
			return fmt.Errorf("registry: capability %q: unknown name, not in the M1 capability vocabulary (§10.1)", c.Name)
		}
		if _, dup := existing[c.Name]; dup {
			return fmt.Errorf("registry: capability %q already registered for provider type %q", c.Name, providerType)
		}
		if staged[c.Name] {
			return fmt.Errorf("registry: capability %q duplicated within the batch for provider type %q", c.Name, providerType)
		}
		staged[c.Name] = true
		for _, k := range c.ResourceKinds {
			if !contract.IsKnownResourceKind(k) {
				return fmt.Errorf("registry: capability %q: unknown resource kind %q (§8.5)", c.Name, k)
			}
		}
		if !servesCapabilities(r.providers[providerType].adapter) {
			return fmt.Errorf("registry: capability %q declared by provider type %q whose adapter implements no capability-serving interface (Discoverer/OperationExecutor) (§11, V5 guard)", c.Name, providerType)
		}
	}
	if existing == nil {
		existing = make(map[string]contract.Capability)
	}
	for _, c := range caps {
		existing[c.Name] = c
	}
	r.caps[providerType] = existing
	return nil
}

// servesCapabilities reports whether the adapter implements at least one
// capability-serving interface (V5 minimum guard).
func servesCapabilities(a contract.BaseAdapter) bool {
	_, isDiscoverer := a.(contract.Discoverer)
	_, isExecutor := a.(contract.OperationExecutor)
	return isDiscoverer || isExecutor
}

// Capabilities returns the capabilities registered for a provider type,
// sorted by name. Empty for unregistered types or types with none.
func (r *Registry) Capabilities(providerType string) []contract.Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := r.caps[providerType]
	out := make([]contract.Capability, 0, len(m))
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		out = append(out, m[n])
	}
	return out
}

// RegisterOperation validates and registers an operation definition.
// V6 (single-version registry, A13): the operation name must be new — even a
// different version is rejected until a multi-version policy exists;
// RequiredCapability must be in the M1 vocabulary; every ResourceKind must be
// known (§8.5); RequiredPermission must match the v1 permission format
// (A14); a Mutating operation may not reference a ReadOnly capability.
func (r *Registry) RegisterOperation(def contract.OperationDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if def.Name == "" {
		return fmt.Errorf("registry: operation name is empty")
	}
	if _, dup := r.operations[def.Name]; dup {
		return fmt.Errorf("registry: operation %q already registered (single-version registry in Phase 0, A13)", def.Name)
	}
	var vocabEntry *contract.CapabilityVocabularyEntry
	for i := range contract.M1CapabilityVocabulary {
		if contract.M1CapabilityVocabulary[i].Name == def.RequiredCapability {
			vocabEntry = &contract.M1CapabilityVocabulary[i]
		}
	}
	if vocabEntry == nil {
		return fmt.Errorf("registry: operation %q: required capability %q is not in the M1 capability vocabulary (§10.1)", def.Name, def.RequiredCapability)
	}
	for _, k := range def.ResourceKinds {
		if !contract.IsKnownResourceKind(k) {
			return fmt.Errorf("registry: operation %q: unknown resource kind %q (§8.5)", def.Name, k)
		}
	}
	if !permissionPattern.MatchString(def.RequiredPermission) {
		return fmt.Errorf("registry: operation %q: required permission %q does not match the v1 permission format (2-4 lowercase segments, hyphen allowed; A14)", def.Name, def.RequiredPermission)
	}
	if def.Mutating && vocabEntry.ReadOnly {
		return fmt.Errorf("registry: operation %q is mutating but references readonly capability %q (§11)", def.Name, def.RequiredCapability)
	}
	r.operations[def.Name] = def
	return nil
}

// Operation looks up a registered operation by name.
func (r *Registry) Operation(name string) (contract.OperationDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.operations[name]
	return def, ok
}

// ResourceKinds returns a defensive copy of contract.M1ResourceKinds —
// callers cannot mutate the global vocabulary through the returned slice.
func (r *Registry) ResourceKinds() []string {
	return append([]string(nil), contract.M1ResourceKinds...)
}
