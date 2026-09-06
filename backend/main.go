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

	engine, svc := router.New(cfg, db)
	server := &http.Server{Addr: ":" + cfg.App.Port, Handler: engine, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("start server failed: %v", err)
		}
	}()
	stop, cancelSignal := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelSignal()
	<-stop.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = svc.Shutdown(ctx)
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
