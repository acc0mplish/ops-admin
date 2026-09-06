package contracttest

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/metrics"
)

// stubAdapter is a minimal Discoverer the harness self-tests run against —
// the harness must fail LOUDLY (and correctly) on contract breaches, not just
// pass conforming adapters (측정 기구의 반증 가능성).
type stubAdapter struct {
	resources []contract.DiscoveredResource
	pageSize  int
	counter   *metrics.Counters
	scenario  string
}

func (s *stubAdapter) RateCounter() *metrics.Counters { return s.counter }

func (*stubAdapter) ProviderName() string { return "stub" }

func (s *stubAdapter) Discover(_ context.Context, req contract.DiscoverRequest) (contract.DiscoverPage, error) {
	if s.scenario != "" {
		if s.scenario == contract.SignalRateLimited {
			s.counter.IncRateLimit("stub")
		}
		return contract.DiscoverPage{}, &contract.ProviderSignalError{Kind: s.scenario, Message: "stub scenario"}
	}
	offset := 0
	if req.Cursor != "" {
		var err error
		offset, err = strconv.Atoi(req.Cursor)
		if err != nil || offset < 0 || offset > len(s.resources) {
			return contract.DiscoverPage{}, fmt.Errorf("stub: bad cursor %q", req.Cursor)
		}
	}
	end := len(s.resources)
	next := ""
	if s.pageSize > 0 && end-offset > s.pageSize {
		end = offset + s.pageSize
		next = strconv.Itoa(end)
	}
	return contract.DiscoverPage{Resources: s.resources[offset:end], NextCursor: next}, nil
}

type stubFixture struct {
	adapter *stubAdapter
	seed    []contract.DiscoveredResource
}

func (f *stubFixture) Seed() []contract.DiscoveredResource { return f.seed }
func (*stubFixture) PageLimit() int                        { return 2 }
func (f *stubFixture) Scenario(name string) error          { f.adapter.scenario = name; return nil }
func (*stubFixture) Connection() contract.ConnectionView   { return contract.ConnectionView{} }

func seeded() []contract.DiscoveredResource {
	return []contract.DiscoveredResource{
		{ExternalID: "a", ExternalURN: "urn:stub:a", Kind: "orchestration.node", Raw: contract.JSONMap{"k": "v"}},
		{ExternalID: "b", ExternalURN: "urn:stub:b", Kind: "orchestration.pod", Normalized: contract.JSONMap{"phase": "Running"}},
		{ExternalID: "c", ExternalURN: "urn:stub:c", Kind: "orchestration.workload"},
	}
}

// 정상 경로 — 페이징(2건씩)·분류·rate 신호·리덕션 전부 통과.
func TestHarnessPassesConformingAdapter(t *testing.T) {
	a := &stubAdapter{resources: seeded(), pageSize: 2, counter: metrics.New()}
	a.counter.RegisterProviders("stub")
	if got := SuiteFailures(a, &stubFixture{adapter: a, seed: seeded()}); len(got) != 0 {
		t.Fatalf("conforming adapter reported failures: %v", got)
	}
}

// 단얫 1 위반 — 시드와 총 수 불일치를 잡는다.
func TestHarnessCatchesCountMismatch(t *testing.T) {
	a := &stubAdapter{resources: seeded()[:1], counter: metrics.New()}
	got := SuiteFailures(a, &stubFixture{adapter: a, seed: seeded()})
	if len(got) == 0 || !strings.Contains(got[0], "discovery total") {
		t.Fatalf("count mismatch not caught: %v", got)
	}
}

// oversizedAdapter — 전체를 한 페이지로 돌려준다(상한 위반).
type oversizedAdapter struct{ inner *stubAdapter }

func (o *oversizedAdapter) RateCounter() *metrics.Counters { return o.inner.counter }
func (*oversizedAdapter) ProviderName() string             { return "stub" }
func (o *oversizedAdapter) Discover(ctx context.Context, req contract.DiscoverRequest) (contract.DiscoverPage, error) {
	page, err := o.inner.Discover(ctx, req)
	if err != nil {
		return page, err
	}
	return contract.DiscoverPage{Resources: o.inner.resources}, nil
}

// 단얫 1 위반 — 페이지 상한 초과.
func TestHarnessCatchesPageSizeViolation(t *testing.T) {
	inner := &stubAdapter{resources: seeded(), pageSize: 2, counter: metrics.New()}
	a := &oversizedAdapter{inner: inner}
	got := SuiteFailures(a, &stubFixture{adapter: inner, seed: seeded()})
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "exceeds forced page limit") {
		t.Fatalf("page-size violation not caught: %v", got)
	}
}

// 단얫 1 위반 — 비종결 커서 루프.
func TestHarnessCatchesNonTermination(t *testing.T) {
	a := &stubAdapter{
		resources: seeded(), pageSize: 2, counter: metrics.New(),
	}
	// stub은 항상 "more" 커서를 돌려준다 — 종결 없음을 잡아야 한다.
	a.counter = metrics.New()
	a.counter.RegisterProviders("stub")
	loop := &loopAdapter{inner: a}
	got := SuiteFailures(loop, &loopFixture{inner: &stubFixture{adapter: a, seed: seeded()}})
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "did not terminate") {
		t.Fatalf("non-termination not caught: %v", got)
	}
}

type loopAdapter struct{ inner *stubAdapter }

func (l *loopAdapter) ProviderName() string           { return "stub" }
func (l *loopAdapter) RateCounter() *metrics.Counters { return l.inner.counter }
func (l *loopAdapter) Discover(_ context.Context, _ contract.DiscoverRequest) (contract.DiscoverPage, error) {
	// 항상 빈 페이지 + 비종결 커서 — 종결 없는 루프만을 재현한다.
	return contract.DiscoverPage{Resources: nil, NextCursor: "endless"}, nil
}

type loopFixture struct{ inner *stubFixture }

func (f *loopFixture) Seed() []contract.DiscoveredResource { return f.inner.Seed() }
func (*loopFixture) PageLimit() int                        { return 2 }
func (f *loopFixture) Scenario(name string) error          { return f.inner.Scenario(name) }
func (f *loopFixture) Connection() contract.ConnectionView { return f.inner.Connection() }

// wrongKindAdapter — paging은 정상, 시나리오는 항상 "잘못된" 종을 보고한다.
type wrongKindAdapter struct {
	inner    *stubAdapter
	reported string
}

func (w *wrongKindAdapter) RateCounter() *metrics.Counters { return w.inner.counter }
func (*wrongKindAdapter) ProviderName() string             { return "stub" }
func (w *wrongKindAdapter) Discover(ctx context.Context, req contract.DiscoverRequest) (contract.DiscoverPage, error) {
	if w.inner.scenario != "" {
		return contract.DiscoverPage{}, &contract.ProviderSignalError{Kind: w.reported, Message: "stub scenario"}
	}
	return w.inner.Discover(ctx, req)
}

// 단얫 2 위반 — 잘못된 신호 종류.
func TestHarnessCatchesWrongSignalKind(t *testing.T) {
	inner := &stubAdapter{resources: seeded(), pageSize: 2, counter: metrics.New()}
	a := &wrongKindAdapter{inner: inner, reported: contract.SignalPermissionDenied}
	fx := &wrongKindFixture{inner: &stubFixture{adapter: inner, seed: seeded()}}
	got := SuiteFailures(a, fx)
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "signal Kind") {
		t.Fatalf("wrong signal kind not caught: %v", got)
	}
}

type wrongKindFixture struct{ inner *stubFixture }

func (f *wrongKindFixture) Seed() []contract.DiscoveredResource { return f.inner.Seed() }
func (*wrongKindFixture) PageLimit() int                        { return 2 }
func (f *wrongKindFixture) Scenario(name string) error          { return f.inner.Scenario(name) }
func (f *wrongKindFixture) Connection() contract.ConnectionView { return f.inner.Connection() }

// 단얫 3 위반 — rate_limited인데 카운터가 증가하지 않는 어댑터.
func TestHarnessCatchesMissingRateSignal(t *testing.T) {
	a := &noCounterAdapter{resources: seeded(), pageSize: 2, counter: metrics.New(), scenario: contract.SignalRateLimited}
	got := SuiteFailures(a, &noCounterFixture{adapter: a, seed: seeded()})
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "rate-limit counter delta") {
		t.Fatalf("missing rate signal not caught: %v", got)
	}
}

// noCounterAdapter — signal 종류는 맞게 돌려주지만 카운터를 증가시키지 않는다.
type noCounterAdapter struct {
	resources []contract.DiscoveredResource
	pageSize  int
	counter   *metrics.Counters
	scenario  string
}

func (s *noCounterAdapter) RateCounter() *metrics.Counters { return s.counter }
func (*noCounterAdapter) ProviderName() string             { return "stub" }
func (s *noCounterAdapter) Discover(_ context.Context, req contract.DiscoverRequest) (contract.DiscoverPage, error) {
	if s.scenario != "" {
		return contract.DiscoverPage{}, &contract.ProviderSignalError{Kind: s.scenario, Message: "stub scenario"}
	}
	if req.Cursor != "" {
		return contract.DiscoverPage{}, nil
	}
	return contract.DiscoverPage{Resources: s.resources}, nil
}

type noCounterFixture struct {
	adapter *noCounterAdapter
	seed    []contract.DiscoveredResource
}

func (f *noCounterFixture) Seed() []contract.DiscoveredResource { return f.seed }
func (*noCounterFixture) PageLimit() int                        { return 2 }
func (f *noCounterFixture) Scenario(name string) error          { f.adapter.scenario = name; return nil }
func (*noCounterFixture) Connection() contract.ConnectionView   { return contract.ConnectionView{} }

// 단얫 4 위반 — 시크릿 마커 노출.
func TestHarnessCatchesRedactionBreach(t *testing.T) {
	leaky := seeded()
	leaky[0].Normalized = contract.JSONMap{"token": "x" + MarkerValue}
	a := &stubAdapter{resources: leaky, pageSize: 2, counter: metrics.New()}
	got := SuiteFailures(a, &stubFixture{adapter: a, seed: seeded()})
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "redaction") {
		t.Fatalf("redaction breach not caught: %v", got)
	}
}

// Fixture.Connection 주입면이 DiscoverRequest에 실제로 실리는지(J2 경로).
func TestHarnessFeedsConnectionView(t *testing.T) {
	fx := &capturingFixture{seed: seeded()}
	a := &captureAdapter{}
	if got := SuiteFailures(a, fx); len(got) == 0 {
		t.Fatalf("expected failures (empty seed), got none")
	}
	if !fx.sawConnection {
		t.Fatalf("harness never passed fx.Connection() into DiscoverRequest")
	}
}

type capturingFixture struct {
	seed          []contract.DiscoveredResource
	sawConnection bool
}

func (f *capturingFixture) Seed() []contract.DiscoveredResource { return f.seed }
func (*capturingFixture) PageLimit() int                        { return 0 }
func (*capturingFixture) Scenario(string) error                 { return nil }
func (f *capturingFixture) Connection() contract.ConnectionView {
	f.sawConnection = true
	return contract.ConnectionView{Material: map[string]string{"inventory": "k"}}
}

type captureAdapter struct{}

func (*captureAdapter) Discover(_ context.Context, req contract.DiscoverRequest) (contract.DiscoverPage, error) {
	if req.Connection.Material == nil || req.Connection.Material["inventory"] != "k" {
		return contract.DiscoverPage{}, errorsNew("fixture connection not propagated")
	}
	return contract.DiscoverPage{}, nil
}

func errorsNew(s string) error { return &simpleErr{s} }

type simpleErr struct{ msg string }

func (e *simpleErr) Error() string { return e.msg }

// 유틸 단언 — countLineValue 파서.
func TestCountLineValueParser(t *testing.T) {
	exposition := "provider_rate_limit_total{provider=\"kubernetes\"} 0\nprovider_rate_limit_total{provider=\"fake\"} 3\n"
	if got := countLineValue(exposition, "provider_rate_limit_total", "kubernetes"); got != 0 {
		t.Errorf("kubernetes line = %d, want 0", got)
	}
	if got := countLineValue(exposition, "provider_rate_limit_total", "fake"); got != 3 {
		t.Errorf("fake line = %d, want 3", got)
	}
	if got := countLineValue(exposition, "provider_rate_limit_total", "absent"); got != 0 {
		t.Errorf("absent provider = %d, want 0 (Prometheus absence semantics)", got)
	}
}
