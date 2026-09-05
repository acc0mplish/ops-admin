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
	"ops-admin/backend/router"
	"ops-admin/backend/store"
	"ops-admin/backend/util"
)

func main() {
	// Read-only subcommand, dispatched before any server startup path.
	if len(os.Args) >= 2 && os.Args[1] == "inventory-secrets" {
		os.Exit(runSecretInventory(os.Args[2:]))
	}
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}
	util.ConfigureCredentialKey(cfg.Security.CredentialKey)

	db, err := store.NewDB(cfg)
	if err != nil {
		log.Fatalf("connect db failed: %v", err)
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
