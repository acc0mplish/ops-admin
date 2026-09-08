package metrics

import (
	"strings"
	"testing"
	"time"
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
	if len(lines) != 3 {
		t.Fatalf("Render lines = %d, want 3 (TYPE header + 2 rows): %q", len(lines), a)
	}
	if lines[0] != "# TYPE provider_rate_limit_total counter" {
		t.Errorf("first line = %q, want the family TYPE header", lines[0])
	}
	if !strings.HasPrefix(lines[1], `provider_rate_limit_total{provider="alpha"}`) {
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

// TestRenderFullFamilyGolden — §18.2 M1 14종 전 패밀리를 발화시킨 Render()
// 전문 byte 골든(T-1). TYPE 헤더·버킷 le 오름차순(+Inf)·_sum/_count·라벨
// 정렬·기존 2종 라인 byte 불변을 한 번에 단얫한다.
func TestRenderFullFamilyGolden(t *testing.T) {
	c := New()
	c.RegisterProviders("aliyun", "kubernetes")

	// A provider_health gauge {connection}
	c.SetHealth("conn-k8s", false)
	c.SetHealth("conn-aliyun", true)
	// B provider_api_latency_seconds histogram {provider, op}
	c.ObserveAPILatency("kubernetes", "list", 3*time.Millisecond)
	c.ObserveAPILatency("aliyun", "discover", 150*time.Millisecond)
	// provider_api_errors_total (기존)
	c.IncAPIError("aliyun", "discover", "500")
	// provider_rate_limit_total (기존)
	c.IncRateLimit("kubernetes")
	// C inventory_sync_duration_seconds histogram {connection, mode}
	c.ObserveSyncDuration("conn-a", "full", 2*time.Second)
	// D inventory_sync_resource_changes_total {connection}
	c.AddSyncResourceChanges("conn-a", 7)
	// E inventory_sync_partial_total {connection}
	c.IncSyncPartial("conn-a")
	// F provider_task_duration_seconds histogram {operation, status}
	c.ObserveTaskDuration("vm.create", "succeeded", 500*time.Millisecond)
	c.ObserveTaskDuration("vm.create", "failed", 4*time.Second)
	// G provider_task_failures_total {operation, code}
	c.IncTaskFailure("vm.create", "500")
	// H provider_task_retries_total {operation}
	c.IncTaskRetry("vm.create")
	// I worker_queue_depth gauge (무라벨)
	c.SetQueueDepth(3)
	// J worker_lease_expired_total (무라벨)
	c.IncLeaseExpired()
	// K resource_stale_total {kind}
	c.IncStaleResource("vm")
	c.IncStaleResource("disk")
	// L secret_access_total {purpose, backend}
	c.IncSecretAccess("inventory", "internal")

	got := c.Render()
	gotAgain := c.Render()
	if got != gotAgain {
		t.Fatalf("Render not deterministic across calls")
	}

	want := `# TYPE provider_health gauge
provider_health{connection="conn-aliyun"} 1
provider_health{connection="conn-k8s"} 0
# TYPE provider_api_latency_seconds histogram
provider_api_latency_seconds_bucket{provider="aliyun",op="discover",le="0.005"} 0
provider_api_latency_seconds_bucket{provider="aliyun",op="discover",le="0.01"} 0
provider_api_latency_seconds_bucket{provider="aliyun",op="discover",le="0.025"} 0
provider_api_latency_seconds_bucket{provider="aliyun",op="discover",le="0.05"} 0
provider_api_latency_seconds_bucket{provider="aliyun",op="discover",le="0.1"} 0
provider_api_latency_seconds_bucket{provider="aliyun",op="discover",le="0.25"} 1
provider_api_latency_seconds_bucket{provider="aliyun",op="discover",le="0.5"} 1
provider_api_latency_seconds_bucket{provider="aliyun",op="discover",le="1"} 1
provider_api_latency_seconds_bucket{provider="aliyun",op="discover",le="2.5"} 1
provider_api_latency_seconds_bucket{provider="aliyun",op="discover",le="5"} 1
provider_api_latency_seconds_bucket{provider="aliyun",op="discover",le="10"} 1
provider_api_latency_seconds_bucket{provider="aliyun",op="discover",le="+Inf"} 1
provider_api_latency_seconds_sum{provider="aliyun",op="discover"} 0.15
provider_api_latency_seconds_count{provider="aliyun",op="discover"} 1
provider_api_latency_seconds_bucket{provider="kubernetes",op="list",le="0.005"} 1
provider_api_latency_seconds_bucket{provider="kubernetes",op="list",le="0.01"} 1
provider_api_latency_seconds_bucket{provider="kubernetes",op="list",le="0.025"} 1
provider_api_latency_seconds_bucket{provider="kubernetes",op="list",le="0.05"} 1
provider_api_latency_seconds_bucket{provider="kubernetes",op="list",le="0.1"} 1
provider_api_latency_seconds_bucket{provider="kubernetes",op="list",le="0.25"} 1
provider_api_latency_seconds_bucket{provider="kubernetes",op="list",le="0.5"} 1
provider_api_latency_seconds_bucket{provider="kubernetes",op="list",le="1"} 1
provider_api_latency_seconds_bucket{provider="kubernetes",op="list",le="2.5"} 1
provider_api_latency_seconds_bucket{provider="kubernetes",op="list",le="5"} 1
provider_api_latency_seconds_bucket{provider="kubernetes",op="list",le="10"} 1
provider_api_latency_seconds_bucket{provider="kubernetes",op="list",le="+Inf"} 1
provider_api_latency_seconds_sum{provider="kubernetes",op="list"} 0.003
provider_api_latency_seconds_count{provider="kubernetes",op="list"} 1
# TYPE provider_api_errors_total counter
provider_api_errors_total{provider="aliyun",op="discover",code="500"} 1
# TYPE provider_rate_limit_total counter
provider_rate_limit_total{provider="aliyun"} 0
provider_rate_limit_total{provider="kubernetes"} 1
# TYPE inventory_sync_duration_seconds histogram
inventory_sync_duration_seconds_bucket{connection="conn-a",mode="full",le="0.05"} 0
inventory_sync_duration_seconds_bucket{connection="conn-a",mode="full",le="0.1"} 0
inventory_sync_duration_seconds_bucket{connection="conn-a",mode="full",le="0.25"} 0
inventory_sync_duration_seconds_bucket{connection="conn-a",mode="full",le="0.5"} 0
inventory_sync_duration_seconds_bucket{connection="conn-a",mode="full",le="1"} 0
inventory_sync_duration_seconds_bucket{connection="conn-a",mode="full",le="2.5"} 1
inventory_sync_duration_seconds_bucket{connection="conn-a",mode="full",le="5"} 1
inventory_sync_duration_seconds_bucket{connection="conn-a",mode="full",le="10"} 1
inventory_sync_duration_seconds_bucket{connection="conn-a",mode="full",le="30"} 1
inventory_sync_duration_seconds_bucket{connection="conn-a",mode="full",le="60"} 1
inventory_sync_duration_seconds_bucket{connection="conn-a",mode="full",le="120"} 1
inventory_sync_duration_seconds_bucket{connection="conn-a",mode="full",le="300"} 1
inventory_sync_duration_seconds_bucket{connection="conn-a",mode="full",le="+Inf"} 1
inventory_sync_duration_seconds_sum{connection="conn-a",mode="full"} 2
inventory_sync_duration_seconds_count{connection="conn-a",mode="full"} 1
# TYPE inventory_sync_resource_changes_total counter
inventory_sync_resource_changes_total{connection="conn-a"} 7
# TYPE inventory_sync_partial_total counter
inventory_sync_partial_total{connection="conn-a"} 1
# TYPE provider_task_duration_seconds histogram
provider_task_duration_seconds_bucket{operation="vm.create",status="failed",le="0.05"} 0
provider_task_duration_seconds_bucket{operation="vm.create",status="failed",le="0.1"} 0
provider_task_duration_seconds_bucket{operation="vm.create",status="failed",le="0.25"} 0
provider_task_duration_seconds_bucket{operation="vm.create",status="failed",le="0.5"} 0
provider_task_duration_seconds_bucket{operation="vm.create",status="failed",le="1"} 0
provider_task_duration_seconds_bucket{operation="vm.create",status="failed",le="2.5"} 0
provider_task_duration_seconds_bucket{operation="vm.create",status="failed",le="5"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="failed",le="10"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="failed",le="30"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="failed",le="60"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="failed",le="120"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="failed",le="300"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="failed",le="+Inf"} 1
provider_task_duration_seconds_sum{operation="vm.create",status="failed"} 4
provider_task_duration_seconds_count{operation="vm.create",status="failed"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="succeeded",le="0.05"} 0
provider_task_duration_seconds_bucket{operation="vm.create",status="succeeded",le="0.1"} 0
provider_task_duration_seconds_bucket{operation="vm.create",status="succeeded",le="0.25"} 0
provider_task_duration_seconds_bucket{operation="vm.create",status="succeeded",le="0.5"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="succeeded",le="1"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="succeeded",le="2.5"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="succeeded",le="5"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="succeeded",le="10"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="succeeded",le="30"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="succeeded",le="60"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="succeeded",le="120"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="succeeded",le="300"} 1
provider_task_duration_seconds_bucket{operation="vm.create",status="succeeded",le="+Inf"} 1
provider_task_duration_seconds_sum{operation="vm.create",status="succeeded"} 0.5
provider_task_duration_seconds_count{operation="vm.create",status="succeeded"} 1
# TYPE provider_task_failures_total counter
provider_task_failures_total{operation="vm.create",code="500"} 1
# TYPE provider_task_retries_total counter
provider_task_retries_total{operation="vm.create"} 1
# TYPE worker_queue_depth gauge
worker_queue_depth 3
# TYPE worker_lease_expired_total counter
worker_lease_expired_total 1
# TYPE resource_stale_total counter
resource_stale_total{kind="disk"} 1
resource_stale_total{kind="vm"} 1
# TYPE secret_access_total counter
secret_access_total{purpose="inventory",backend="internal"} 1
`
	if got != want {
		t.Fatalf("Render() full-family golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestRenderTypeHeaders — 14종 패밀리 각각의 `# TYPE` 헤더 존재 단얫
// (골든 비교 외에 TYPE 라인 자체를 독립 검증한다).
func TestRenderTypeHeaders(t *testing.T) {
	c := New()
	c.RegisterProviders("p")
	c.SetHealth("conn", true)
	c.ObserveAPILatency("p", "op", time.Second)
	c.IncAPIError("p", "op", "500")
	c.IncRateLimit("p")
	c.ObserveSyncDuration("conn", "full", time.Second)
	c.AddSyncResourceChanges("conn", 1)
	c.IncSyncPartial("conn")
	c.ObserveTaskDuration("op", "succeeded", time.Second)
	c.IncTaskFailure("op", "500")
	c.IncTaskRetry("op")
	c.SetQueueDepth(1)
	c.IncLeaseExpired()
	c.IncStaleResource("vm")
	c.IncSecretAccess("inventory", "internal")

	got := c.Render()
	for _, want := range []string{
		"# TYPE provider_health gauge",
		"# TYPE provider_api_latency_seconds histogram",
		"# TYPE provider_api_errors_total counter",
		"# TYPE provider_rate_limit_total counter",
		"# TYPE inventory_sync_duration_seconds histogram",
		"# TYPE inventory_sync_resource_changes_total counter",
		"# TYPE inventory_sync_partial_total counter",
		"# TYPE provider_task_duration_seconds histogram",
		"# TYPE provider_task_failures_total counter",
		"# TYPE provider_task_retries_total counter",
		"# TYPE worker_queue_depth gauge",
		"# TYPE worker_lease_expired_total counter",
		"# TYPE resource_stale_total counter",
		"# TYPE secret_access_total counter",
	} {
		if !strings.Contains(got, want+"\n") {
			t.Errorf("Render() missing %q header", want)
		}
	}
}

// TestRenderLabelValueEscaping — 라벨값의 `\"`·`\\`·개행 이스케이프(§1.2).
func TestRenderLabelValueEscaping(t *testing.T) {
	c := New()
	c.IncStaleResource(`we"ird\kind`)
	c.IncTaskRetry("op\nwith\nnewlines")

	got := c.Render()
	if !strings.Contains(got, `resource_stale_total{kind="we\"ird\\kind"} 1`) {
		t.Errorf("Render() = %q, want escaped quote and backslash in label value", got)
	}
	if !strings.Contains(got, `provider_task_retries_total{operation="op\nwith\nnewlines"} 1`) {
		t.Errorf("Render() = %q, want escaped newline in label value", got)
	}
}
