// Package metrics is the minimal in-process counter shape for the §18.2
// provider metrics (Phase 2 PR 20, J6 판정). The gate ③ proof target is the
// VALUE (`provider_rate_limit_total` flat), not an endpoint: the
// /internal/metrics endpoint and the full §18.2 table are explicitly deferred
// to Phase 3+ (§13 부채 대장). Sync/compare report artifacts carry the
// Render() text as their evidence payload.
//
// Purity: this package imports the standard library only — it sits beside
// contract as vocabulary-adjacent infrastructure and must stay importable
// from adapters and the contracttest harness alike.
package metrics

import (
	"sort"
	"strings"
	"sync"
)

// apiErrKey — provider_api_errors_total 라벨 3성분(§18.2 라벨 형식).
type apiErrKey struct {
	provider string
	op       string
	code     string
}

// Counters is the mutex-guarded minimal counter set. Zero value is not
// usable — use New.
type Counters struct {
	mu         sync.Mutex
	providers  map[string]bool      // 등록 프로바이더 — Render가 0행을 포함해 내보낸다(게이트 ③ flat 증명용)
	rateLimits map[string]uint64    // provider_rate_limit_total{provider=...}
	apiErrors  map[apiErrKey]uint64 // provider_api_errors_total{provider,op,code}
}

// New returns an empty counter set.
func New() *Counters {
	return &Counters{
		providers:  make(map[string]bool),
		rateLimits: make(map[string]uint64),
		apiErrors:  make(map[apiErrKey]uint64),
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

// Render produces Prometheus text exposition with the §18.2 label shapes:
//
//	provider_rate_limit_total{provider="kubernetes"} 0
//	provider_api_errors_total{provider="kubernetes",op="discover",code="429"} 2
//
// Every registered (or touched) provider yields a rate-limit line — zero
// included — sorted by provider for deterministic artifacts. API error lines
// appear only for (provider,op,code) triples that fired (Prometheus-side
// absence equals zero; the report artifact needs no empty combinatorics).
func (c *Counters) Render() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	providers := make([]string, 0, len(c.providers))
	for p := range c.providers {
		providers = append(providers, p)
	}
	sort.Strings(providers)

	var b strings.Builder
	for _, p := range providers {
		b.WriteString("provider_rate_limit_total{provider=\"")
		b.WriteString(p)
		b.WriteString("\"} ")
		b.WriteString(itoa(c.rateLimits[p]))
		b.WriteByte('\n')
	}
	keys := make([]apiErrKey, 0, len(c.apiErrors))
	for k := range c.apiErrors {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].provider != keys[j].provider {
			return keys[i].provider < keys[j].provider
		}
		if keys[i].op != keys[j].op {
			return keys[i].op < keys[j].op
		}
		return keys[i].code < keys[j].code
	})
	for _, k := range keys {
		b.WriteString("provider_api_errors_total{provider=\"")
		b.WriteString(k.provider)
		b.WriteString("\",op=\"")
		b.WriteString(k.op)
		b.WriteString("\",code=\"")
		b.WriteString(k.code)
		b.WriteString("\"} ")
		b.WriteString(itoa(c.apiErrors[k]))
		b.WriteByte('\n')
	}
	return b.String()
}

// itoa — uint64 to decimal without strconv import churn in the hot path
// (readability over micro-optimisation; strconv would be equally fine).
func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	return string(digits[i:])
}
