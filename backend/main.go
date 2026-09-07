package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ops-admin/backend/config"
	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/router"
	"ops-admin/backend/store"
	"ops-admin/backend/util"
)

func main() {
	// Read-only subcommand, dispatched before any server startup path.
	if len(os.Args) >= 2 && os.Args[1] == "inventory-secrets" {
		os.Exit(runSecretInventory(os.Args[2:]))
	}
	// Offline secret-migration subcommands (§4.4 Steps 2-3), dispatched
	// before any server startup path.
	if len(os.Args) >= 2 && os.Args[1] == "reencrypt-secrets" {
		os.Exit(runReencryptSecrets(os.Args[2:]))
	}
	if len(os.Args) >= 2 && os.Args[1] == "verify-secrets" {
		os.Exit(runVerifySecrets(os.Args[2:]))
	}
	// V2 shadow sync subcommand (Phase 2 PR 21): backfill + one sync run +
	// the dated report artifact, dispatched before any server startup path.
	if len(os.Args) >= 2 && os.Args[1] == "sync-inventory" {
		os.Exit(runSyncInventory(os.Args[2:]))
	}
	// V2 compare subcommand (Phase 2 PR 22): the §15 paired run + the dated
	// artifact + the 3-day gate check, dispatched before any server startup
	// path.
	if len(os.Args) >= 2 && os.Args[1] == "compare-inventory" {
		os.Exit(runCompareInventory(os.Args[2:]))
	}
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}
	util.ConfigureCredentialKey(cfg.Security.CredentialKey)
	// G-5 gate: a production startup without any secret key source fails;
	// the development fallback is permitted only when GO_ENV=development is
	// explicit.
	if err := util.EnsureSecretKeySource(cfg.Security.CredentialKey); err != nil {
		log.Fatalf("secret key source missing: %v", err)
	}

	db, err := store.NewDB(cfg)
	if err != nil {
		log.Fatalf("connect db failed: %v", err)
	}

	// v2 versioned migration runner (PR 15): fail-closed — a runner failure
	// blocks v1 startup (W-1).
	if err := migrate.Run(context.Background(), db); err != nil {
		log.Fatalf("v2 schema migration failed: %v", err)
	}

	if err := store.AutoMigrate(db); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}

	if err := store.Seed(db); err != nil {
		log.Fatalf("seed data failed: %v", err)
	}
	if err := store.CanonicalizeSeedLocalization(db); err != nil {
		log.Fatalf("canonicalize seed localization failed: %v", err)
	}

	// V2 Phase 3 (plan M9): the engine lane assembles AFTER the schema and
	// seeds are ready — compose once → engine (J6 hook) → fully-wired v2 API.
	// A stack failure disables this lane alone (R11); v1 boots unchanged.
	taskEngine, v2API := startEngineLane(db)

	engine, svc := router.New(cfg, db, v2API)
	server := &http.Server{Addr: ":" + cfg.App.Port, Handler: engine, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("start server failed: %v", err)
		}
	}()
	stop, cancelSignal := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelSignal()
	<-stop.Done()
	// F-7 shutdown order (engine_config.go gracefulShutdown): engine loops
	// stop first — in-flight engine work reaches its terminal commit while
	// the HTTP server still serves — then service cleanup, then the HTTP
	// drain inside the 10s budget.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	gracefulShutdown(ctx, taskEngine, svc, server)
}
