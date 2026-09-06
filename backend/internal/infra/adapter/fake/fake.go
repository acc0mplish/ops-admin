// Package fake is the first-class test adapter (§11.1 built-in list,
// §11 "fake is a first-class adapter, not an afterthought").
//
// Not the DNS provider domain at internal/domain/provider (spec §3 row 13,
// REMAIN) — this is a V2 infra adapter.
//
// Phase 1 (plan §3.8): the execution shape — Discoverer (fixed seeded
// resource pages), OperationExecutor (atomic execution counter — the N6
// "executed exactly once" measurement instrument — plus failAttempt/async
// payload controls), and TaskPoller (UPID dual-mode handle states). Register
// also declares the §3.7 capability pair inventory.full +
// orchestration.kubernetes.apply so the mapping-table validation actually
// fires on this adapter (phase0 가정 A7 해소).
package fake

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/metrics"
	"ops-admin/backend/internal/infra/registry"
)

// errUnknownHandle — Poll on a handle this adapter never issued.
var errUnknownHandle = errors.New("fake: unknown operation handle")

// scenarioVocabulary — the harness Fixture scenario names the fake can arm.
// The three error kinds are the closed contract.ProviderSignalError set.
var scenarioVocabulary = map[string]bool{
	contract.SignalRateLimited:      true,
	contract.SignalPermissionDenied: true,
	contract.SignalUnreachable:      true,
	"":                              true, // reset
}

// Adapter is the fake adapter. All methods use pointer receivers: the
// adapter carries a mutex-guarded execution counter and handle table —
// Register hands the registry a single *Adapter.
//
// Phase 2 PR 20 additions: the contracttest harness scenario arm (rate-limit/
// permission/unreachable divergence — 하네스 측정 기구의 카나리, T44) and the
// in-process rate-limit counter the harness reads back (단얫 3 — 반증 가능성).
type Adapter struct {
	mu        sync.Mutex
	execCount int
	lastReq   contract.OperationRequest
	lastPoll  contract.PollRequest
	handles   map[string]*fakeHandle
	scenario  string
	counters  *metrics.Counters
}

// fakeHandle is the §14.3 UPID stand-in: succeedAfter polling cycles to
// success, fail reports failure on the first poll.
type fakeHandle struct {
	polls        int
	succeedAfter int
	fail         bool
}

func (a *Adapter) initLocked() {
	if a.handles == nil {
		a.handles = make(map[string]*fakeHandle)
	}
}

// Descriptor — Type:"fake", AdapterVersion:"1", ProtocolVersion:"1"(가정 A5),
// ConfigSchema: 빈 ConfigSpec, ContextKinds: 없음, BuiltIn: true.
func (*Adapter) Descriptor() contract.ProviderTypeDescriptor {
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
func (*Adapter) Validate(_ context.Context, _ contract.ConnectionView) error { return nil }

// Health always reports healthy.
func (*Adapter) Health(_ context.Context, _ contract.ConnectionView) contract.HealthResult {
	return contract.HealthResult{Healthy: true}
}

// Close is a no-op.
func (*Adapter) Close() error { return nil }

// ProviderName is the fake provider's Prometheus label (metrics 패키지
// 렌더 라인의 provider="fake").
const ProviderName = "fake"

// seededResources is the fixed discovery page — orchestration.* kinds 3종
// (§3.8), one page, terminal NextCursor. Raw carries benign payload material
// so the contracttest redaction walk exercises a non-empty tree.
var seededResources = []contract.DiscoveredResource{
	{ExternalID: "cluster-1", ExternalURN: "urn:fake:cluster:cluster-1", Kind: "orchestration.cluster", DisplayName: "fake cluster", Raw: contract.JSONMap{"note": "seeded"}},
	{ExternalID: "node-1", ExternalURN: "urn:fake:node:node-1", Kind: "orchestration.node", DisplayName: "fake node", Raw: contract.JSONMap{"note": "seeded"}},
	{ExternalID: "workload-1", ExternalURN: "urn:fake:workload:workload-1", Kind: "orchestration.workload", DisplayName: "fake workload", Raw: contract.JSONMap{"note": "seeded"}},
}

// Discover returns the single seeded page; any non-empty cursor terminates
// paging with an empty page (deterministic termination, §3.8).
//
// Phase 2 PR 20: an armed scenario diverges instead of seeding — the
// contract.ProviderSignalError kind matches the scenario name, and the
// rate_limited scenario increments the in-process counter the contracttest
// harness reads back (계획 §3.7 단얫 3).
func (a *Adapter) Discover(_ context.Context, req contract.DiscoverRequest) (contract.DiscoverPage, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.scenario != "" {
		if a.scenario == contract.SignalRateLimited {
			a.countersLocked().IncRateLimit(ProviderName)
		}
		return contract.DiscoverPage{}, &contract.ProviderSignalError{
			Kind:    a.scenario,
			Message: "fake: armed scenario " + a.scenario,
		}
	}
	if req.Cursor != "" {
		return contract.DiscoverPage{}, nil
	}
	return contract.DiscoverPage{Resources: seededResources, NextCursor: ""}, nil
}

// Scenario arms (or resets, with "") a harness failure scenario. Invalid
// names are rejected — the closed ProviderSignalError kind set plus reset.
func (a *Adapter) Scenario(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !scenarioVocabulary[name] {
		return fmt.Errorf("fake: unknown scenario %q (want rate_limited|permission_denied|unreachable|\"\")", name)
	}
	a.scenario = name
	return nil
}

// RateCounter exposes the adapter's in-process counters — the contracttest
// harness rate-limit assertion reads it (계획 §3.7 단얫 3).
func (a *Adapter) RateCounter() *metrics.Counters {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.countersLocked()
}

// ProviderName is the Prometheus provider label this adapter's counters use.
func (*Adapter) ProviderName() string { return ProviderName }

func (a *Adapter) countersLocked() *metrics.Counters {
	if a.counters == nil {
		a.counters = metrics.New()
		a.counters.RegisterProviders(ProviderName)
	}
	return a.counters
}

// Execute — §3.8 controls, all read from Payload:
//
//	failAttempt == n  → the n-th execution attempt errors (retry simulation)
//	async == true     → an OperationHandle with a ProviderRef is returned,
//	                    forcing the Poll path (§14.3 UPID dual mode);
//	                    polls == n keeps the handle Running for n-1 polls first
//	                    (default 1); asyncFail == true polls to Failed
//	anything else     → null handle = synchronous completion (§14.3 PVE
//	                    background_delay shape: null-with-exit-status)
//
// Every call increments the execution counter — the §23.4 N6 "provider 이중
// 실행 횟수 단언" instrument. Int-valued payload fields accept int/int64/
// float64 (JSON round-trips turn ints into float64).
func (a *Adapter) Execute(_ context.Context, req contract.OperationRequest) (contract.OperationHandle, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.initLocked()

	a.execCount++
	a.lastReq = req
	a.lastReq.Payload = cloneJSONMap(req.Payload)

	if n := payloadInt(req.Payload, "failAttempt"); n > 0 && n == a.execCount {
		return contract.OperationHandle{}, fmt.Errorf("fake: injected failure on attempt %d (failAttempt)", a.execCount)
	}
	if async, _ := req.Payload["async"].(bool); async {
		succeedAfter := payloadInt(req.Payload, "polls")
		if succeedAfter < 1 {
			succeedAfter = 1
		}
		fail, _ := req.Payload["asyncFail"].(bool)
		ref := fmt.Sprintf("fake:upid:%d", a.execCount)
		a.handles[ref] = &fakeHandle{succeedAfter: succeedAfter, fail: fail}
		return contract.OperationHandle{ProviderRef: ref}, nil
	}
	return contract.OperationHandle{}, nil
}

// Poll advances a handle issued by Execute: succeeded/failed/running (§3.8).
// Polling never increments the execution counter — an async attempt is ONE
// execution followed by polls (§3.6 "Poll은 새 attempt를 만들지 않는다").
//
// Phase 3 M19/J12: the signature carries PollRequest{Handle, Connection} —
// the fake itself needs no credential material (stateless §3.5 path), but it
// RECORDS the request so the engine-circuit tests (N11) can assert the
// engine's two poll assembly sites deliver credentials on every poll. The
// shallow copy is safe: the engine assembles a fresh ConnectionView per poll,
// so there is no caller-side map to defend against.
func (a *Adapter) Poll(_ context.Context, req contract.PollRequest) (contract.OperationStatus, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.initLocked()

	a.lastPoll = req

	h, ok := a.handles[req.Handle.ProviderRef]
	if !ok {
		return contract.OperationStatus{}, fmt.Errorf("%w: %q", errUnknownHandle, req.Handle.ProviderRef)
	}
	h.polls++
	switch {
	case h.fail:
		return contract.OperationStatus{
			State:  contract.OperationStateFailed,
			Detail: contract.JSONMap{"handle": req.Handle.ProviderRef, "polls": h.polls},
		}, nil
	case h.polls >= h.succeedAfter:
		return contract.OperationStatus{
			State:  contract.OperationStateSucceeded,
			Detail: contract.JSONMap{"handle": req.Handle.ProviderRef, "polls": h.polls},
		}, nil
	default:
		return contract.OperationStatus{
			State:  contract.OperationStateRunning,
			Detail: contract.JSONMap{"handle": req.Handle.ProviderRef, "polls": h.polls},
		}, nil
	}
}

// LastPoll returns the most recent PollRequest the adapter saw — the engine's
// re-poll credential-delivery path (J12) is asserted through it (N11).
// Connection.Material is plaintext credential material: test-instrument only,
// never logged or serialized.
func (a *Adapter) LastPoll() contract.PollRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastPoll
}

// ExecCount reports how many execution attempts the provider double has run —
// the N6 single-execution assertion reads this.
func (a *Adapter) ExecCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.execCount
}

// LastRequest returns the most recent OperationRequest the adapter saw — the
// engine's UID→URN assembly (T-7) is asserted through it.
func (a *Adapter) LastRequest() contract.OperationRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastReq
}

// Register registers the fake provider type with the registry and declares
// its §3.7 capabilities (inventory.full + orchestration.kubernetes.apply).
func Register(r *registry.Registry) error {
	a := &Adapter{}
	if err := r.RegisterProviderType(a.Descriptor(), a); err != nil {
		return err
	}
	return r.RegisterCapabilities("fake",
		contract.Capability{
			Name:          "inventory.full",
			Version:       "1",
			ResourceKinds: []string{"orchestration.cluster", "orchestration.node", "orchestration.workload"},
			ReadOnly:      true,
		},
		contract.Capability{
			Name:          "orchestration.kubernetes.apply",
			Version:       "1",
			ResourceKinds: []string{"orchestration.workload"},
		},
	)
}

// Compile-time interface conformance — the pointer type carries the mutex
// and the Phase 1 execution interfaces; Register hands out *Adapter.
var (
	_ contract.BaseAdapter       = (*Adapter)(nil)
	_ contract.Discoverer        = (*Adapter)(nil)
	_ contract.OperationExecutor = (*Adapter)(nil)
	_ contract.TaskPoller        = (*Adapter)(nil)
)

// payloadInt reads an int-valued payload key across Go/JSON numeric types.
func payloadInt(payload contract.JSONMap, key string) int {
	v, ok := payload[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func cloneJSONMap(in contract.JSONMap) contract.JSONMap {
	if in == nil {
		return nil
	}
	out := make(contract.JSONMap, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
