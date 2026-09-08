package tencent

// 초이크 latency 계기(§18.2 B) — do 1회(describeInstances 경유, 기존 계기
// 어휘 op "discover")가 provider_api_latency_seconds의 bucket·_sum·_count
// 라인을 _count 1로 남기는지 단얫한다(계획 T-2 델타 계약).

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/metrics"
)

func TestDoObservesAPILatency(t *testing.T) {
	mock := newMockCVM(t)
	counters := metrics.New()
	client := &cvmClient{
		http:       &http.Client{Timeout: requestTimeout},
		credential: credential{accessKey: testAccessKey, secretKey: testSecretKey},
		endpoint:   mock.srv.URL,
		region:     testRegionA,
		metrics:    counters,
	}
	if _, err := client.describeInstances(context.Background(), 0, 1); err != nil {
		t.Fatalf("describeInstances: %v", err)
	}

	render := counters.Render()
	if !strings.Contains(render, "# TYPE provider_api_latency_seconds histogram") {
		t.Errorf("render missing the histogram TYPE header:\n%s", render)
	}
	if got := renderLineValue(t, render, `provider_api_latency_seconds_count{provider="tencent",op="discover"}`); got != 1 {
		t.Errorf("latency count = %v, want 1 after one describeInstances", got)
	}
	if !strings.Contains(render, `provider_api_latency_seconds_bucket{provider="tencent",op="discover",le="+Inf"} 1`) {
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
