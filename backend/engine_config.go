package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/internal/api/v2"
	"ops-admin/backend/internal/infra/compose"
	"ops-admin/backend/internal/tasks"
)

// V2 task-engine startup (plan M9/A6/J10). The config source order is
// spec-value defaults → OPS_ADMIN_ENGINE_* env overrides. The config.yaml
// `engine:` section (plan M10) threads through the same defaults when it
// lands; this file owns the runtime override surface the E2E/CI lane uses.

// engineDefaults returns the spec-value baseline (J10): poll 2s (§13.1
// "default interval 2s, configurable"), lease 30s, reaper grace 2s. WorkerID
// is a host-pid composite so attempt rows and events attribute work even
// when several instances share a hostname.
func engineDefaults() tasks.Config {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "ops-admin"
	}
	return tasks.Config{
		WorkerID:     fmt.Sprintf("%s-%d", host, os.Getpid()),
		PollInterval: 2 * time.Second,
		LeaseSeconds: 30,
		ReaperGrace:  2 * time.Second,
	}
}

// Env override knobs (A6) — exactly three. There is deliberately NO
// TimeoutSeconds override: TimeoutSeconds is an opdef contract value (an
// input to audit and retry arithmetic); a runtime override would break
// definition/execution agreement (J10 기각).
const (
	envEnginePollIntervalMS = "OPS_ADMIN_ENGINE_POLL_INTERVAL_MS"
	envEngineLeaseSeconds   = "OPS_ADMIN_ENGINE_LEASE_SECONDS"
	envEngineReaperGraceMS  = "OPS_ADMIN_ENGINE_REAPER_GRACE_MS"
)

// engineConfigFromEnv applies the three OPS_ADMIN_ENGINE_* overrides onto
// base. An unset variable keeps the base value; an invalid one (garbage,
// zero, negative) keeps the base AND yields one warning naming the variable —
// the engine lane degrades loudly, never the v1 boot (R11). getenv is
// injected so tests need no process-env mutation.
func engineConfigFromEnv(base tasks.Config, getenv func(string) string) (tasks.Config, []string) {
	cfg := base
	var warnings []string

	if raw := strings.TrimSpace(getenv(envEnginePollIntervalMS)); raw != "" {
		if ms, err := strconv.Atoi(raw); err != nil || ms <= 0 {
			warnings = append(warnings, fmt.Sprintf("%s=%q is not a positive integer — keeping %s", envEnginePollIntervalMS, raw, base.PollInterval))
		} else {
			cfg.PollInterval = time.Duration(ms) * time.Millisecond
		}
	}
	if raw := strings.TrimSpace(getenv(envEngineLeaseSeconds)); raw != "" {
		if secs, err := strconv.Atoi(raw); err != nil || secs <= 0 {
			warnings = append(warnings, fmt.Sprintf("%s=%q is not a positive integer — keeping %d", envEngineLeaseSeconds, raw, base.LeaseSeconds))
		} else {
			cfg.LeaseSeconds = secs
		}
	}
	if raw := strings.TrimSpace(getenv(envEngineReaperGraceMS)); raw != "" {
		if ms, err := strconv.Atoi(raw); err != nil || ms <= 0 {
			warnings = append(warnings, fmt.Sprintf("%s=%q is not a positive integer — keeping %s", envEngineReaperGraceMS, raw, base.ReaperGrace))
		} else {
			cfg.ReaperGrace = time.Duration(ms) * time.Millisecond
		}
	}
	return cfg, warnings
}

// engineConfigSummary renders the VK-8 one-line effective-config log — the
// applied-configuration assertion surface (R5: a mis-tuned lease/grace pair
// is visible at startup, not discovered in recovery timing).
func engineConfigSummary(cfg tasks.Config) string {
	return fmt.Sprintf("worker=%s poll_interval=%s lease_seconds=%d reaper_grace=%s",
		cfg.WorkerID, cfg.PollInterval, cfg.LeaseSeconds, cfg.ReaperGrace)
}

// Stoppable is the Shutdown(ctx) surface gracefulShutdown needs —
// *service.Service and *http.Server both satisfy it.
type Stoppable interface {
	Shutdown(context.Context) error
}

// gracefulShutdown executes the F-7 order under the caller's ctx budget:
//
//	① engine.Stop  — the loops drain while the HTTP server still serves, so
//	                 in-flight engine work reaches its terminal commit inside
//	                 a live process (the crash-recovery contract's mirror
//	                 image: a planned stop is not a crash)
//	② svc.Shutdown  — service pool cleanup
//	③ server.Shutdown — the HTTP drain, last
//
// The engine stop is NOT ctx-bound: Stop waits for the in-flight
// RunOnce/ReapOnce cycle by contract (loops.go) — cutting it with the budget
// would reintroduce the crash window the order exists to avoid. A nil engine
// (engine lane off) skips step ① without touching the rest.
func gracefulShutdown(ctx context.Context, taskEngine *tasks.Engine, svc Stoppable, server Stoppable) {
	if taskEngine != nil {
		taskEngine.Stop()
	}
	if svc != nil {
		_ = svc.Shutdown(ctx)
	}
	if server != nil {
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}
}

// startEngineLane is the M9 assembly: compose runs ONCE here; the engine
// lane builds stack → engine (J6 hook: observation refresh + §18.1 terminal
// audit via v2.RecordTaskTerminal) → fully-assembled v2 API
// (NewInfraAPIWithEngine — the CancelStarter seam activates the moment the
// engine carries RequestCancel, N6). A stack failure disables the engine
// lane ALONE (R11): the returned engine·sweeper·render are nil and the
// router keeps its Phase 2 self-assembly path — v1 boots exactly as before,
// and /internal/metrics stays unregistered (the nil guard is claim 9).
//
// §18.2 P6 — the sweeper starts alongside the engine on the same derived
// base context (계획 §1.5) and its Stop is the caller's to invoke (main's
// post-gracefulShutdown step); the render func is the /internal/metrics
// handler source (계획 §1.3).
func startEngineLane(db *gorm.DB) (*tasks.Engine, *v2.InfraAPI, *compose.HealthSweeper, func() string) {
	stack, err := compose.Build(db)
	if err != nil {
		log.Printf("v2 engine lane disabled — stack build failed: %v (v1 services continue, R11)", err)
		return nil, nil, nil, nil
	}
	cfg, warnings := engineConfigFromEnv(engineDefaults(), os.Getenv)
	for _, warning := range warnings {
		log.Printf("v2 task engine config: %s", warning)
	}
	taskEngine := stack.BuildEngine(cfg, v2.RecordTaskTerminal)
	api := v2.NewInfraAPIWithEngine(db, stack.Registry, taskEngine)
	log.Printf("v2 task engine starting: %s", engineConfigSummary(cfg)) // VK-8
	baseCtx := context.Background()
	taskEngine.Start(baseCtx)
	healthSweep := compose.NewHealthSweeper(db, stack.Registry, stack.Broker, stack.Counters)
	healthSweep.Start(baseCtx)
	return taskEngine, api, healthSweep, stack.Counters.Render
}
