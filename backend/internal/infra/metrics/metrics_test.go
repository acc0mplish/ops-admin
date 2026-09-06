package metrics

import (
	"strings"
	"testing"
)

// 게이트 ③의 증거 형식 단얫 — §18.2 라벨 형식 그대로.
func TestRenderLabelShapes(t *testing.T) {
	c := New()
	c.RegisterProviders("kubernetes")

	// 미증가 상태에서도 0행이 존재해야 "flat" 증명이 성립한다.
	got := c.Render()
	if !strings.Contains(got, `provider_rate_limit_total{provider="kubernetes"} 0`) {
		t.Fatalf("Render() = %q, want zero-valued kubernetes rate-limit line", got)
	}

	c.IncRateLimit("kubernetes")
	c.IncRateLimit("kubernetes")
	c.IncAPIError("kubernetes", "discover", "500")

	got = c.Render()
	if !strings.Contains(got, `provider_rate_limit_total{provider="kubernetes"} 2`) {
		t.Errorf("Render() = %q, want rate-limit counter 2", got)
	}
	if !strings.Contains(got, `provider_api_errors_total{provider="kubernetes",op="discover",code="500"} 1`) {
		t.Errorf("Render() = %q, want api error line", got)
	}
	if strings.HasSuffix(got, "\n\n") {
		t.Errorf("Render() trailing blank line: %q", got)
	}
}

func TestRenderDeterministicAndSorted(t *testing.T) {
	c := New()
	c.RegisterProviders("zeta", "alpha")
	c.IncRateLimit("zeta")

	a := c.Render()
	b := c.Render()
	if a != b {
		t.Fatalf("Render not deterministic:\n%q\n%q", a, b)
	}
	lines := strings.Split(strings.TrimSpace(a), "\n")
	if len(lines) != 2 {
		t.Fatalf("Render lines = %d, want 2: %q", len(lines), a)
	}
	if !strings.HasPrefix(lines[0], `provider_rate_limit_total{provider="alpha"}`) {
		t.Errorf("providers not sorted: %q", a)
	}
}

func TestRegisterProvidersIgnoresEmpty(t *testing.T) {
	c := New()
	c.RegisterProviders("")
	if got := c.Render(); strings.Contains(got, `provider=""`) {
		t.Errorf("empty provider name rendered: %q", got)
	}
}

func TestNewZeroState(t *testing.T) {
	if got := New().Render(); got != "" {
		t.Errorf("empty counters Render = %q, want empty", got)
	}
}
