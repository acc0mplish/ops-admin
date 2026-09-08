// Package metrics is the in-process counter shape for the §18.2 control-plane
// metrics. It started as the gate ③ proof (PR 20, J6 판정 — the proof target
// is the VALUE, `provider_rate_limit_total` flat) and now carries the full
// §18.2 M1 family set (14종) with standard Prometheus text exposition
// rendering. The /internal/metrics endpoint wiring stays with the engine lane
// (V2 계획 P6) — the package itself stays endpoint-free.
//
// Purity: this package imports the standard library only — it sits beside
// contract as vocabulary-adjacent infrastructure and must stay importable
// from adapters and the contracttest harness alike.
package metrics

import (
	"sync"
)

// apiErrKey — provider_api_errors_total 라벨 3성분(§18.2 라벨 형식).
type apiErrKey struct {
	provider string
	op       string
	code     string
}

// providerOpKey — provider_api_latency_seconds 라벨 쌍.
type providerOpKey struct {
	provider string
	op       string
}

// connModeKey — inventory_sync_duration_seconds 라벨 쌍.
type connModeKey struct {
	connection string
	mode       string
}

// opStatusKey — provider_task_duration_seconds 라벨 쌍.
type opStatusKey struct {
	operation string
	status    string
}

// opCodeKey — provider_task_failures_total 라벨 쌍.
type opCodeKey struct {
	operation string
	code      string
}

// purposeBackendKey — secret_access_total 라벨 쌍.
type purposeBackendKey struct {
	purpose string
	backend string
}

// Counters is the mutex-guarded §18.2 M1 family set (2종 기존 + 12종 신규).
// Zero value is not usable — use New.
type Counters struct {
	mu         sync.Mutex
	providers  map[string]bool      // 등록 프로바이더 — Render가 0행을 포함해 내보낸다(게이트 ③ flat 증명용)
	rateLimits map[string]uint64    // provider_rate_limit_total{provider=...}
	apiErrors  map[apiErrKey]uint64 // provider_api_errors_total{provider,op,code}

	// counter families (§18.2 M1)
	syncChanges    map[string]uint64            // inventory_sync_resource_changes_total{connection}
	syncPartial    map[string]uint64            // inventory_sync_partial_total{connection}
	taskFailures   map[opCodeKey]uint64         // provider_task_failures_total{operation,code}
	taskRetries    map[string]uint64            // provider_task_retries_total{operation}
	leaseExpired   uint64                       // worker_lease_expired_total
	staleResources map[string]uint64            // resource_stale_total{kind}
	secretAccess   map[purposeBackendKey]uint64 // secret_access_total{purpose,backend}

	// histogram families (§18.2 M1) — histogram.go
	apiLatency   map[providerOpKey]*histSample // provider_api_latency_seconds{provider,op}
	syncDuration map[connModeKey]*histSample   // inventory_sync_duration_seconds{connection,mode}
	taskDuration map[opStatusKey]*histSample   // provider_task_duration_seconds{operation,status}

	// gauge families (§18.2 M1) — gauge.go
	health        map[string]bool // provider_health{connection} 0/1
	queueDepth    uint64          // worker_queue_depth
	queueDepthSet bool            // worker_queue_depth 패밀리 존재 — 명시적 Set 호출 시에만 렌더
}

// New returns an empty counter set.
func New() *Counters {
	return &Counters{
		providers:      make(map[string]bool),
		rateLimits:     make(map[string]uint64),
		apiErrors:      make(map[apiErrKey]uint64),
		syncChanges:    make(map[string]uint64),
		syncPartial:    make(map[string]uint64),
		taskFailures:   make(map[opCodeKey]uint64),
		taskRetries:    make(map[string]uint64),
		staleResources: make(map[string]uint64),
		secretAccess:   make(map[purposeBackendKey]uint64),
		apiLatency:     make(map[providerOpKey]*histSample),
		syncDuration:   make(map[connModeKey]*histSample),
		taskDuration:   make(map[opStatusKey]*histSample),
		health:         make(map[string]bool),
	}
}

// RegisterProviders pre-registers provider rows so Render emits zero-valued
// lines for untouched providers — the gate ③ "flat" proof needs the
// {provider="kubernetes"} 0 line to EXIST. Adapters register their own
// provider name at construction; compose may register more.
func (c *Counters) RegisterProviders(names ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, n := range names {
		if n != "" {
			c.providers[n] = true
		}
	}
}

// IncRateLimit records one rate-limit signal from a provider client wrapper
// (HTTP 429 / Retry-After). The adapter REST wrapper is the only caller.
func (c *Counters) IncRateLimit(provider string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.providers[provider] = true
	c.rateLimits[provider]++
}

// IncAPIError records one provider API error with its §18.2 label triple.
func (c *Counters) IncAPIError(provider, op, code string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.providers[provider] = true
	c.apiErrors[apiErrKey{provider: provider, op: op, code: code}]++
}

// AddSyncResourceChanges records the created+updated resource count of one
// sync run (가정 A2: changes = created + updated).
func (c *Counters) AddSyncResourceChanges(connection string, n uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncChanges[connection] += n
}

// IncSyncPartial records one partial sync completion for a connection.
func (c *Counters) IncSyncPartial(connection string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncPartial[connection]++
}

// IncTaskFailure records one task terminal in {failed, timed_out} with its
// error code (가정 A4).
func (c *Counters) IncTaskFailure(operation, code string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.taskFailures[opCodeKey{operation: operation, code: code}]++
}

// IncTaskRetry records one task requeue for retry (가정 A4: requeueForRetry만
// 발화 — reaper 재큐는 lease_expired 쪽이 담당).
func (c *Counters) IncTaskRetry(operation string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.taskRetries[operation]++
}

// IncLeaseExpired records one CAS-won expired lease handled by the reaper
// (재큐·소진 양 갈래 모두).
func (c *Counters) IncLeaseExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.leaseExpired++
}

// IncStaleResource records one stale_candidate transition, by resource kind
// (가정 A3: tombstoned는 미가산).
func (c *Counters) IncStaleResource(kind string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.staleResources[kind]++
}

// IncSecretAccess records one successful secret broker resolve (가정 A5:
// 성공만 가산). purpose·backend는 binding/ref 필드에서 온다.
func (c *Counters) IncSecretAccess(purpose, backend string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.secretAccess[purposeBackendKey{purpose: purpose, backend: backend}]++
}
