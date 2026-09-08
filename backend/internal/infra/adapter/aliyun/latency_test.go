package aliyun

// 초이크 latency 계기(§18.2 B) — request 1회(성공 경로)가
// provider_api_latency_seconds의 bucket·_sum·_count 라인을 _count 1로 남기는지
// 단얫한다(계획 T-2 델타 계약 — 행위 1회 → 패밀리 라인 +1).

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/metrics"
)

func TestRequestObservesAPILatency(t *testing.T) {
	mock := newMockAliyun(t)
	counters := metrics.New()
	c := &ecsClient{
		cred:     credential{accessKey: mockAccessKey, secretKey: mockSecretKey},
		endpoint: func(string) string { return mock.srv.URL },
		metrics:  counters,
		http:     &http.Client{Timeout: requestTimeout},
	}
	var body map[string]any
	if err := c.request(context.Background(), "DescribeInstances", "cn-mock-a", "discover", nil, &body); err != nil {
		t.Fatalf("request: %v", err)
	}

	render := counters.Render()
	if !strings.Contains(render, "# TYPE provider_api_latency_seconds histogram") {
		t.Errorf("render missing the histogram TYPE header:\n%s", render)
	}
	if got := renderLineValue(t, render, `provider_api_latency_seconds_count{provider="aliyun",op="discover"}`); got != 1 {
		t.Errorf("latency count = %v, want 1 after one request", got)
	}
	if !strings.Contains(render, `provider_api_latency_seconds_bucket{provider="aliyun",op="discover",le="+Inf"} 1`) {
		t.Errorf("render missing the +Inf bucket line:\n%s", render)
	}
}

// renderLineValue — "{labels} value" 형태 렌더 라인의 마지막 필드를 수치로
// 파싱한다(값 단얫 전용 — 존재하지 않으면 테스트 실패).
func renderLineValue(t *testing.T, render, key string) float64 {
	t.Helper()
	for _, line := range strings.Split(render, "\n") {
		if strings.HasPrefix(line, key+" ") {
			fields := strings.Fields(line)
			v, err := strconv.ParseFloat(fields[len(fields)-1], 64)
			if err != nil {
				t.Fatalf("parse %q: %v", line, err)
			}
			return v
		}
	}
	t.Fatalf("render missing %q:\n%s", key, render)
	return 0
}
