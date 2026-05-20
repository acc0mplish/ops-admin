package main

import (
	"log"

	"ops-admin/backend/config"
	"ops-admin/backend/router"
	"ops-admin/backend/store"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

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

	engine := router.New(cfg, db)
	if err := engine.Run(":" + cfg.App.Port); err != nil {
		log.Fatalf("start server failed: %v", err)
	}
}
