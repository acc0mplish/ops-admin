// Package contracttest is the §23.1 contract tier: "every adapter runs the
// same harness: discovery paging, error taxonomy, rate-limit signaling,
// redaction of raw payloads". Phase 1 r2 E 이연분의 인수 형상(계획 J5) —
// fake이 첫 소비자(하네스 자체의 측정 기구 카나리, T44)이고 k8s 어댑터가 같은
// 하네스를 통과한다(T45 — §23.4 Q1의 fake·kubernetes 2행).
//
// The harness imports contract and metrics only — never an adapter package.
// Adapter test files (fake, kubernetes, …) call RunContractSuite with their
// own Fixture implementation.
package contracttest

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/metrics"
)

// MarkerValue is the canary secret the Fixture plants on the provider side of
// its simulated data. Assertion 4 (redaction) fails if any discovered
// resource's Raw/Normalized surfaces it anywhere. Fixtures plant it as a
// secret data VALUE (k8s: a Secret's data field); adapters must surface
// metadata only.
const MarkerValue = "contracttest-redaction-canary-do-not-leak"

// ScenarioReset resets a previously armed scenario ("" = none). The three
// error scenarios are the closed §9.3-adjacent signal set — the same kinds
// contract.ProviderSignalError carries.
const ScenarioReset = ""

// Fixture is the scenario/seed injection seam (계획 §3.7). An adapter test
// supplies it; the harness drives the Discoverer through it.
type Fixture interface {
	// Seed returns the full expected resource set — the paging walk must
	// reproduce exactly this set (count + ExternalURN identity), never a
	// subset or superset.
	Seed() []contract.DiscoveredResource
	// PageLimit is the page size the adapter is forced to (페이징 단얫 —
	// no page may exceed it). 0 disables the bound assertion.
	PageLimit() int
	// Scenario arms a named failure scenario for the NEXT Discover calls:
	// "rate_limited" | "permission_denied" | "unreachable". Scenario("")
	// resets to the nominal (seeding) behaviour.
	Scenario(name string) error
	// Connection is the credential view the harness feeds to every
	// DiscoverRequest (계획 §3.1 J2 — 어댑터는 req.Connection만 읽는다).
	// Without this seam the harness could only drive adapters that need no
	// material; the kubernetes adapter's harness pass (T45) exercises the
	// real Material["inventory"] path through it.
	Connection() contract.ConnectionView
}

// rateCounterSource is the optional seam for assertion 3 (rate-limit
// signaling): an adapter that owns a *metrics.Counters exposes it. Both
// built-in adapters (fake, kubernetes) implement it; adapters without
// counters make the harness skip assertion 3 — the k8s gate proof path
// (게이트 ③ 반증 가능성) MUST implement it.
type rateCounterSource interface {
	RateCounter() *metrics.Counters
}

// RunContractSuite runs the four §23.1 assertion families against d, driven
// by fx. Every violation is reported on t — the suite NEVER passes silently.
func RunContractSuite(t interface {
	Error(args ...any)
	Errorf(format string, args ...any)
	Helper()
}, d contract.Discoverer, fx Fixture) {
	t.Helper()
	for _, f := range SuiteFailures(d, fx) {
		t.Errorf("%s", f)
	}
}

// SuiteFailures executes the harness and returns one human-readable entry per
// contract violation (empty = conforming). Exposed separately from
// RunContractSuite so the harness's own falsifiability tests can assert that
// breaches are CAUGHT (측정 기구의 반증 가능성 — harness_test.go).
func SuiteFailures(d contract.Discoverer, fx Fixture) []string {
	var failures []string
	fail := func(format string, args ...any) {
		failures = append(failures, fmt.Sprintf(format, args...))
	}

	ctx := context.Background()

	// --- 단얫 1: discovery paging — 커서 순회 종결·중복 0·총 수 == 시드. ---
	seed := fx.Seed()
	wantURNs := make(map[string]bool, len(seed))
	for _, r := range seed {
		wantURNs[r.ExternalURN] = true
	}

	seen := make(map[string]bool)
	cursor := ""
	pages := 0
	total := 0
	maxPages := len(seed)*2 + 64 // empty sections may yield empty pages — bounded, not seeded-count bound
	terminated := false

	for !terminated {
		page, err := d.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Cursor: cursor, Connection: fx.Connection()})
		if err != nil {
			fail("assertion 1 (paging): Discover(cursor=%q): unexpected error: %v", cursor, err)
			break
		}
		pages++
		if limit := fx.PageLimit(); limit > 0 && len(page.Resources) > limit {
			fail("assertion 1 (paging): page %d has %d resources, exceeds forced page limit %d", pages, len(page.Resources), limit)
			break
		}
		for _, r := range page.Resources {
			if r.ExternalURN == "" {
				fail("assertion 1 (paging): discovered resource %q has empty ExternalURN", r.DisplayName)
				continue
			}
			if seen[r.ExternalURN] {
				fail("assertion 1 (paging): duplicate resource URN %q across pages", r.ExternalURN)
			}
			seen[r.ExternalURN] = true
			if !wantURNs[r.ExternalURN] {
				fail("assertion 1 (paging): discovered URN %q not in seed set", r.ExternalURN)
			}
			// --- 단얫 4: redaction — 마커는 시드 데이터에 상시 심어져 있어야
			// 단얫이 공허해지지 않는다(Fixture 계약). ---
			assertRedacted(&failures, r)
			total++
		}
		if page.NextCursor == "" {
			terminated = true
			break
		}
		if pages > maxPages {
			fail("assertion 1 (paging): discovery did not terminate after %d pages (cursor=%q)", maxPages, page.NextCursor)
			break
		}
		cursor = page.NextCursor
	}
	if terminated {
		if total != len(seed) {
			fail("assertion 1 (paging): discovery total = %d, want %d (seed set size)", total, len(seed))
		}
		for urn := range wantURNs {
			if !seen[urn] {
				fail("assertion 1 (paging): seed URN %q never discovered", urn)
			}
		}
	}

	// --- 단얫 2: error taxonomy — 시나리오별 ProviderSignalError.Kind 일치. ---
	scenarios := []string{contract.SignalRateLimited, contract.SignalPermissionDenied, contract.SignalUnreachable}
	for _, name := range scenarios {
		if err := fx.Scenario(name); err != nil {
			fail("assertion 2 (taxonomy): arm scenario %q: %v", name, err)
			continue
		}
		_, err := d.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Connection: fx.Connection()})
		var sig *contract.ProviderSignalError
		if !errors.As(err, &sig) {
			fail("assertion 2 (taxonomy): scenario %q: error = %v, want *contract.ProviderSignalError", name, err)
			continue
		}
		if sig.Kind != name {
			fail("assertion 2 (taxonomy): scenario %q: signal Kind = %q, want %q", name, sig.Kind, name)
		}
		assertNoCredentialMaterial(&failures, sig.Error())
	}
	if err := fx.Scenario(ScenarioReset); err != nil {
		fail("reset scenario: %v", err)
	}

	// --- 단얫 3: rate-limit signaling — rate_limited 시나리오가 계측 카운터를 증가
	// (반증 가능성 — 게이트 ③의 "flat"이 측정 기구 위에서만 주장될 수 있게 한다). ---
	if src, ok := d.(rateCounterSource); ok {
		c := src.RateCounter()
		if c == nil {
			fail("assertion 3 (rate signaling): adapter exposes a nil counter")
		} else {
			provider := providerLabelOf(d)
			before := countLineValue(c.Render(), "provider_rate_limit_total", provider)
			if err := fx.Scenario(contract.SignalRateLimited); err != nil {
				fail("assertion 3 (rate signaling): arm scenario rate_limited: %v", err)
			} else {
				if _, err := d.Discover(ctx, contract.DiscoverRequest{ContextID: 1, Connection: fx.Connection()}); err == nil {
					fail("assertion 3 (rate signaling): rate_limited scenario: Discover succeeded, want signal error")
				}
				if err := fx.Scenario(ScenarioReset); err != nil {
					fail("reset scenario: %v", err)
				}
				after := countLineValue(c.Render(), "provider_rate_limit_total", provider)
				if after != before+1 {
					fail("assertion 3 (rate signaling): rate-limit counter delta = %d, want 1 (before=%d after=%d)", after-before, before, after)
				}
			}
		}
	}

	return failures
}

// providerLabelOf picks the Prometheus provider label the adapter reports
// under — the adapter type's own name is the only honest source.
func providerLabelOf(d contract.Discoverer) string {
	type named interface{ ProviderName() string }
	if n, ok := d.(named); ok {
		return n.ProviderName()
	}
	return ""
}

// assertRedacted appends a failure if the marker value appears anywhere in the
// resource's Raw/Normalized payload trees.
func assertRedacted(failures *[]string, r contract.DiscoveredResource) {
	for _, tree := range []contract.JSONMap{r.Raw, r.Normalized} {
		if containsMarker(tree) {
			*failures = append(*failures, fmt.Sprintf(
				"assertion 4 (redaction): resource %q (%s) surfaces the secret marker in its payload", r.ExternalURN, r.Kind))
		}
	}
}

func containsMarker(v any) bool {
	switch x := v.(type) {
	case contract.JSONMap:
		// Named type ≠ map[string]any in a type switch — unwrap explicitly.
		return containsMarker(map[string]any(x))
	case map[string]any:
		for _, vv := range x {
			if containsMarker(vv) {
				return true
			}
		}
	case []any:
		for _, vv := range x {
			if containsMarker(vv) {
				return true
			}
		}
	case string:
		return strings.Contains(x, MarkerValue)
	}
	return false
}

// assertNoCredentialMaterial keeps error strings free of credential-shaped
// material (kubeconfig fragments) — 보존 제약 #7의 에러 경로 절반.
func assertNoCredentialMaterial(failures *[]string, msg string) {
	for _, token := range []string{"kubeconfig", "BEGIN CERTIFICATE", "BEGIN RSA", "bearer ", "Bearer "} {
		if strings.Contains(msg, token) {
			*failures = append(*failures, fmt.Sprintf(
				"assertion 2 (taxonomy): error message carries credential-shaped material (%q): %q", token, msg))
		}
	}
}

// countLineValue parses one `metric{labels...} value` line out of a rendered
// exposition. Missing line = 0 (Prometheus absence semantics), which is also
// the pre-scenario baseline shape.
func countLineValue(exposition, metric, provider string) uint64 {
	wantPrefix := metric
	if provider != "" {
		wantPrefix = metric + `{provider="` + provider + `"}`
	}
	for _, line := range strings.Split(exposition, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, wantPrefix+" ") {
			v, _ := strconvParseUint(strings.TrimSpace(strings.TrimPrefix(line, wantPrefix)))
			return v
		}
	}
	return 0
}

func strconvParseUint(s string) (uint64, bool) {
	var v uint64
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		v = v*10 + uint64(s[i]-'0')
	}
	return v, len(s) > 0
}
