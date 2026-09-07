package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"ops-admin/backend/internal/api/v2"
	"ops-admin/backend/internal/infra/compose"
	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/tasks"
	"ops-admin/backend/internal/testutil"
)

// A6 — OPS_ADMIN_ENGINE_* env overrides (J10): exactly three knobs
// (PollInterval·LeaseSeconds·ReaperGrace); TimeoutSeconds is deliberately
// NOT overridable (J10 기각 — opdef contract value). Invalid values warn and
// keep the default: the engine lane degrades, never the v1 boot (R11).

func TestEngineDefaultsMatchSpec(t *testing.T) {
	cfg := engineDefaults()
	if cfg.PollInterval != 2*time.Second {
		t.Errorf("default PollInterval = %s, want 2s (§13.1)", cfg.PollInterval)
	}
	if cfg.LeaseSeconds != 30 {
		t.Errorf("default LeaseSeconds = %d, want 30 (J10 spec value)", cfg.LeaseSeconds)
	}
	if cfg.ReaperGrace != 2*time.Second {
		t.Errorf("default ReaperGrace = %s, want 2s (J10 spec value)", cfg.ReaperGrace)
	}
	if cfg.WorkerID == "" {
		t.Error("default WorkerID is empty — attempts must be attributable")
	}
	if !strings.Contains(cfg.WorkerID, "-") {
		t.Errorf("default WorkerID = %q, want a host-pid composite for multi-instance distinguishability", cfg.WorkerID)
	}
}

func TestEngineConfigFromEnvOverrides(t *testing.T) {
	base := engineDefaults()
	env := map[string]string{
		"OPS_ADMIN_ENGINE_POLL_INTERVAL_MS": "250",
		"OPS_ADMIN_ENGINE_LEASE_SECONDS":    "5",
		"OPS_ADMIN_ENGINE_REAPER_GRACE_MS":  "500",
	}
	cfg, warnings := engineConfigFromEnv(base, func(key string) string { return env[key] })
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none for valid overrides", warnings)
	}
	if cfg.PollInterval != 250*time.Millisecond {
		t.Errorf("PollInterval = %s, want 250ms", cfg.PollInterval)
	}
	if cfg.LeaseSeconds != 5 {
		t.Errorf("LeaseSeconds = %d, want 5", cfg.LeaseSeconds)
	}
	if cfg.ReaperGrace != 500*time.Millisecond {
		t.Errorf("ReaperGrace = %s, want 500ms", cfg.ReaperGrace)
	}
	if cfg.WorkerID != base.WorkerID {
		t.Errorf("WorkerID = %q, want the untouched default %q", cfg.WorkerID, base.WorkerID)
	}
}

func TestEngineConfigFromEnvUnsetKeepsDefaults(t *testing.T) {
	base := engineDefaults()
	cfg, warnings := engineConfigFromEnv(base, func(string) string { return "" })
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none when nothing is set", warnings)
	}
	if cfg != base {
		t.Fatalf("config = %+v, want the untouched base %+v", cfg, base)
	}
}

func TestEngineConfigFromEnvInvalidWarnsAndKeepsDefault(t *testing.T) {
	base := engineDefaults()
	env := map[string]string{
		"OPS_ADMIN_ENGINE_POLL_INTERVAL_MS": "not-a-number",
		"OPS_ADMIN_ENGINE_LEASE_SECONDS":    "0",
		"OPS_ADMIN_ENGINE_REAPER_GRACE_MS":  "-5",
	}
	cfg, warnings := engineConfigFromEnv(base, func(key string) string { return env[key] })
	if cfg.PollInterval != base.PollInterval || cfg.LeaseSeconds != base.LeaseSeconds || cfg.ReaperGrace != base.ReaperGrace {
		t.Fatalf("config = %+v, want defaults kept on invalid input (garbage/zero/negative)", cfg)
	}
	if len(warnings) != 3 {
		t.Fatalf("warnings = %d (%v), want one per invalid variable", len(warnings), warnings)
	}
	for _, w := range warnings {
		if !strings.Contains(w, "OPS_ADMIN_ENGINE_") {
			t.Errorf("warning %q does not name the offending variable", w)
		}
	}
}

// VK-8 — the Start-time config log is ONE line carrying the effective values
// (the applied-configuration assertion surface for R5/E2E).
func TestEngineConfigSummaryOneLine(t *testing.T) {
	cfg := tasks.Config{WorkerID: "w1", PollInterval: 2 * time.Second, LeaseSeconds: 30, ReaperGrace: 2 * time.Second}
	line := engineConfigSummary(cfg)
	if strings.Count(line, "\n") != 0 {
		t.Errorf("summary is not one line: %q", line)
	}
	for _, want := range []string{"w1", "2s", "30", "poll_interval", "lease_seconds", "reaper_grace", "worker"} {
		if !strings.Contains(line, want) {
			t.Errorf("summary %q missing %q", line, want)
		}
	}
}

// A2 인계 — CancelStarter 자동 활성: *tasks.Engine이 RequestCancel(N6)을
// 갖춘 순간 v2.NewInfraAPIWithEngine의 타입 단언이 성공한다. 컴파일 시점
// 단얫이 곧 런타임 활성의 증명이다(동일 타입 단얫).
var _ v2.CancelStarter = (*tasks.Engine)(nil)

// gracefulShutdown (F-7): 엔진 정지 → 서비스 정리 → HTTP 드레인 순서와 ctx
// 예산. 순서는 기계적으로 단얫한다 — svc·server는 기록 페이크로, 엔진 슬롯은
// 실 엔진의 Stop 계약(루프 완전 드레인)으로.
type recordingStoppable struct {
	name  string
	order *[]string
	block chan struct{}
}

func (r *recordingStoppable) Shutdown(context.Context) error {
	if r.block != nil {
		<-r.block
	}
	*r.order = append(*r.order, r.name)
	return nil
}

func TestGracefulShutdownOrderEngineFirst(t *testing.T) {
	var order []string
	svc := &recordingStoppable{name: "svc", order: &order}
	server := &recordingStoppable{name: "server", order: &order}

	// A REAL started engine: Stop must complete inside the sequence (the
	// loops drain — T39's no-goroutine-outlives contract).
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}
	reg := registry.New()
	engine := tasks.NewEngine(db, reg, tasks.Config{WorkerID: "shutdown-test", PollInterval: time.Millisecond})
	engine.Start(context.Background())
	time.Sleep(5 * time.Millisecond) // let at least one empty tick fire

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	gracefulShutdown(ctx, engine, svc, server)

	if len(order) != 2 || order[0] != "svc" || order[1] != "server" {
		t.Fatalf("shutdown order = %v, want [svc server] — the HTTP drain is last (F-7)", order)
	}
	// The engine slot ran FIRST and completely: a manual cycle after the
	// sequence works (no deadlock; Stop waited for the in-flight tick) and a
	// second Stop is a harmless no-op.
	engine.Stop()
	if _, err := engine.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce after gracefulShutdown: %v — the engine must still be manually drivable", err)
	}
}

func TestGracefulShutdownNilEngineAndBudget(t *testing.T) {
	var order []string
	svc := &recordingStoppable{name: "svc", order: &order}
	server := &recordingStoppable{name: "server", order: &order}

	// nil engine (engine lane off) must not panic, and an already-expired
	// budget still runs the sequence — the order is not ctx-conditional.
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)
	gracefulShutdown(ctx, nil, svc, server)
	if len(order) != 2 {
		t.Fatalf("shutdown order = %v, want both stoppables run even on a dead budget", order)
	}
}

// A2 인계 — 종단 감사 배선의 컴파일 시점 계약: v2.RecordTaskTerminal은
// compose.TerminalAuditFunc에 대입 가능해야 한다(main이 어댑터 없이 배선).
// 어느 한쪽 시그니처가 드리프트하면 이 파일이 빌드를 실패한다.
var _ compose.TerminalAuditFunc = v2.RecordTaskTerminal
