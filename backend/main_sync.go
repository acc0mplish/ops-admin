package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ops-admin/backend/config"
	"ops-admin/backend/internal/infra/compose"
	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/store"
	"ops-admin/backend/util"
)

// syncReportArtifact is the report artifact schema — a whitelist: run
// identity, §9.3 outcomes, counts, and the Prometheus metrics render
// (claims 5/6 evidence). Nothing credential-shaped (kubeconfig, SecretRef
// material) exists on this struct (보존 제약 #7). The §5.4 backfill summary
// the v1 schema once carried was dropped with the backfill itself (V2
// Phase 6 I-a — phase6-plan §J5 S7).
type syncReportArtifact struct {
	RunUID         string         `json:"runUid"`
	ConnectionUID  string         `json:"connectionUid"`
	Mode           string         `json:"mode"`
	Status         string         `json:"status"`
	Outcomes       map[string]int `json:"outcomes"`
	SeenCount      int            `json:"seenCount"`
	CreatedCount   int            `json:"createdCount"`
	UpdatedCount   int            `json:"updatedCount"`
	MissingCount   int            `json:"missingCount"`
	MetricsText    string         `json:"metricsText"`
	StartedAt      time.Time      `json:"startedAt"`
	CommittedAt    time.Time      `json:"committedAt"`
	FinishedAt     time.Time      `json:"finishedAt"`
	ArtifactSchema string         `json:"artifactSchema"`
}

// runSyncInventory implements the "sync-inventory" command: run one shadow
// sync (§9) and write the dated report artifact under the deployment's data
// directory. The §5.4 v1 backfill this command once ran first was retired in
// V2 Phase 6 I-a (phase6-plan §J5 S1c·S7) — register-k8s is the registration
// path now. A failed run reports through the artifact and exits non-zero
// only on infrastructure errors — a partial/failed sync outcome is data,
// not a crash.
func runSyncInventory(args []string) int {
	flags := flag.NewFlagSet("sync-inventory", flag.ContinueOnError)
	configPath := flags.String("config", "config.yaml", "path to config.yaml")
	connectionUID := flags.String("connection", "", "provider connection UID to sync (required)")
	mode := flags.String("mode", "full", "sync mode (full — resume/incremental land in Phase 4, plan §3.3)")
	dataDir := flags.String("data", "data", "data directory root for report artifacts")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "sync-inventory: %v\n", err)
		return 1
	}
	if *connectionUID == "" {
		fmt.Fprintln(os.Stderr, "sync-inventory: --connection is required")
		return 1
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync-inventory: load config: %v\n", err)
		return 1
	}
	util.ConfigureCredentialKey(cfg.Security.CredentialKey)
	if err := util.EnsureSecretKeySource(cfg.Security.CredentialKey); err != nil {
		fmt.Fprintf(os.Stderr, "sync-inventory: secret key source: %v\n", err)
		return 1
	}
	db, err := store.NewDB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync-inventory: connect db: %v\n", err)
		return 1
	}
	if err := migrate.Run(context.Background(), db); err != nil {
		fmt.Fprintf(os.Stderr, "sync-inventory: v2 schema migration: %v\n", err)
		return 1
	}

	stack, err := compose.Build(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync-inventory: compose stack: %v\n", err)
		return 1
	}

	ctx := context.Background()
	report, err := stack.Runner.RunSync(ctx, inventory.SyncInput{ConnectionUID: *connectionUID, Mode: *mode})
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync-inventory: sync: %v\n", err)
		return 1
	}

	artifact := syncReportArtifact{
		RunUID: report.RunUID, ConnectionUID: *connectionUID, Mode: *mode,
		Status: report.Status, Outcomes: report.Outcomes,
		MetricsText: report.MetricsText,
		StartedAt:   report.StartedAt, CommittedAt: report.CommittedAt, FinishedAt: report.FinishedAt,
		ArtifactSchema: "ops-admin.sync-report/v1",
	}
	path, err := writeSyncArtifact(*dataDir, *connectionUID, report.FinishedAt, artifact)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync-inventory: write report artifact: %v\n", err)
		return 1
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(artifact); err != nil {
		fmt.Fprintf(os.Stderr, "sync-inventory: encode report: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "sync-inventory: report artifact %s\n", path)
	return 0
}

// writeSyncArtifact writes the dated artifact:
// data/sync/<connection-uid>/<date>/<time>.json (plan §2 운영 산출, r2).
func writeSyncArtifact(dataDir, connectionUID string, at time.Time, artifact syncReportArtifact) (string, error) {
	dir := filepath.Join(dataDir, "sync", connectionUID, at.Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, at.Format("150405")+".json")
	b, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
