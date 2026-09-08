package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// main_metrics_test — P6 claim 9: /internal/metrics는 render가 있을 때 200
// text/plain으로 Render 전문을 응답하고, 스택 실패(R11)의 nil render에서는
// 등록 자체가 일어나지 않는다(라우트 미등록 nil 가드).

func metricsTestEngine(t *testing.T, render func() string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerMetricsRoute(engine, render)
	return engine
}

func TestRegisterMetricsRouteServesRender(t *testing.T) {
	render := "provider_rate_limit_total{provider=\"kubernetes\"} 0\n"
	engine := metricsTestEngine(t, func() string { return render })

	req := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /internal/metrics = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	if got := rec.Body.String(); got != render {
		t.Errorf("body = %q, want the render verbatim %q", got, render)
	}
}

func TestRegisterMetricsRouteNilRenderSkipsRegistration(t *testing.T) {
	// nil render — the R11 stack-failure boot shape. The route must not exist.
	engine := metricsTestEngine(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /internal/metrics with nil render = %d, want 404 (route unregistered)", rec.Code)
	}
}
