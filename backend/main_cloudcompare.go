package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/config"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/internal/infra/migrate"
	inframodel "ops-admin/backend/internal/infra/model"
	"ops-admin/backend/model"
	"ops-admin/backend/service"
	"ops-admin/backend/store"
	"ops-admin/backend/util"
)

// runCompareCloudInventory implements the "compare-inventory-cloud" command
// (plan phase4 §3.4 — the §15 cloud vm variant of the paired run): the legacy
// capture through a fresh Service (J7 — the same fetchCloudInstances path the
// v1 cloud sync drives), the V2 side read back through the public projection
// queries, paired on the instance id and classified into the dated artifact
// under the deployment's data directory. --gate instead evaluates the 3-day
// gate over the stored artifacts (§15.4 r2). It is a new file by design: the
// Phase 2 gate owns main_compare.go, whose merge stays frozen until the gate
// closes (plan §4 CLI freeze) — the cloud pair must not touch it.
func runCompareCloudInventory(args []string) int {
	flags := flag.NewFlagSet("compare-inventory-cloud", flag.ContinueOnError)
	configPath := flags.String("config", "config.yaml", "path to config.yaml")
	accountID := flags.Uint("account", 0, "v1 asset_cloud_account id to pair (required)")
	dataDir := flags.String("data", "data", "data directory root for report artifacts")
	gate := flags.Bool("gate", false, "evaluate the 3-day gate over stored artifacts instead of capturing a pair")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: %v\n", err)
		return 1
	}
	if *gate {
		return runCompareCloudGate(*dataDir, *accountID)
	}
	if *accountID == 0 {
		fmt.Fprintln(os.Stderr, "compare-inventory-cloud: --account is required")
		return 1
	}
	// Refuse a misconfigured endpoint override BEFORE touching any
	// infrastructure — a "mock" run whose override is silently inert would
	// send the legacy capture at the operating provider (E-2 gate, r2).
	if provider, overrideErr := util.ValidateCloudEndpointOverrides(); overrideErr != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: %s endpoint override: %v\n", provider, overrideErr)
		return 1
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: load config: %v\n", err)
		return 1
	}
	util.ConfigureCredentialKey(cfg.Security.CredentialKey)
	if err := util.EnsureSecretKeySource(cfg.Security.CredentialKey); err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: secret key source: %v\n", err)
		return 1
	}
	db, err := store.NewDB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: connect db: %v\n", err)
		return 1
	}
	if err := migrate.Run(context.Background(), db); err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: v2 schema migration: %v\n", err)
		return 1
	}

	// v1 cloud account row — read-only (R9: compare never writes v1 state).
	var account model.AssetCloudAccount
	if err := db.First(&account, *accountID).Error; err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: load asset_cloud_account %d: %v\n", *accountID, err)
		return 1
	}

	// V2 side: the backfilled connection, its account context, and the public
	// generation (§3.5 r2 공개 generation 규약).
	var conn inframodel.ProviderConnection
	err = db.Where("source_model = ? AND source_id = ? AND stale_source = ?", "asset_cloud_account", *accountID, false).
		First(&conn).Error
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: no live V2 connection for cloud account %d — run sync-inventory first: %v\n", *accountID, err)
		return 1
	}
	var pctx inframodel.ProviderContext
	if err := db.Where("connection_id = ?", conn.ID).Order("id").First(&pctx).Error; err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: load provider context: %v\n", err)
		return 1
	}
	generation, err := inventory.LatestAuthoritativeGeneration(db, pctx.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: %v\n", err)
		return 1
	}
	projected, err := inventory.ProjectResources(db, pctx.ID, generation.GenerationUID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: project resources: %v\n", err)
		return 1
	}

	// §13-10 interim decision: an active development endpoint override makes
	// the run machine-detectably interim (the mock-endpoint proof path, E-2).
	// Set-but-invalid overrides were already refused above the database path.
	interim := false
	if override, ok := util.CloudEndpointOverride(conn.ProviderType); ok && override != "" {
		interim = true
	}

	// Legacy capture — fresh Service (J7/J4): nothing cached, no v1 writes,
	// single-shot process; Shutdown after (R6).
	svc := service.New(db)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = svc.Shutdown(ctx)
	}()

	captureOnce := func() (inventory.CloudLegacyCapture, error) {
		return svc.CaptureCloudInstancesForCompare(*accountID)
	}

	legacy, err := captureOnce()
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: legacy capture: %v\n", err)
		return 1
	}
	attempt := 1
	pairInput := inventory.CloudPairInput{
		AccountID: *accountID, AccountName: account.Name, Attempt: attempt,
		Family: cloudFamilyOf(conn.ProviderType),
		Legacy: legacy,
		V2: inventory.ProjectedV2{
			SyncedAt: generation.CommittedAt, GenerationUID: generation.GenerationUID, Resources: projected,
		},
	}
	report := inventory.CompareCloudPair(pairInput)
	if report.Verdict == inventory.VerdictRePair {
		// §15.1 — beyond the 60s window: one re-pair; a second miss is a
		// BLOCKER (no unbounded recapture).
		attempt = 2
		legacy, err = captureOnce()
		if err != nil {
			fmt.Fprintf(os.Stderr, "compare-inventory-cloud: legacy re-capture: %v\n", err)
			return 1
		}
		pairInput.Attempt = attempt
		pairInput.Legacy = legacy
		report = inventory.CompareCloudPair(pairInput)
	}

	artifact := inventory.CloudCompareArtifact{
		ArtifactSchema:   inventory.CloudCompareArtifactSchema,
		AccountID:        *accountID,
		AccountName:      account.Name,
		Verdict:          report.Verdict,
		Attempt:          attempt,
		TsDeltaSeconds:   report.TsDelta.Seconds(),
		LegacyCapturedAt: pairInput.Legacy.CapturedAt,
		LegacyHash:       inventory.CloudLegacyCaptureHash(pairInput.Legacy),
		V2GenerationUID:  pairInput.V2.GenerationUID,
		V2SyncedAt:       pairInput.V2.SyncedAt,
		V2Hash:           inventory.ProjectionHash(pairInput.V2),
		Report:           report,
	}
	if interim {
		artifact.Interim = true
		artifact.InterimReason = "real-account proof pending (E-1): the pair was served by the development endpoint override"
		artifact.Scope = "mock-endpoint"
	}
	path, err := writeCloudCompareArtifact(*dataDir, *accountID, pairInput.Legacy.CapturedAt, artifact)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: write report artifact: %v\n", err)
		return 1
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(artifact); err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: encode report: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "compare-inventory-cloud: report artifact %s\n", path)
	if report.Verdict != inventory.VerdictPass {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: verdict %q — %d blocker(s)\n", report.Verdict, len(report.Blockers))
		return 1
	}
	if interim {
		fmt.Fprintln(os.Stderr, "compare-inventory-cloud: verdict pass is INTERIM (§13-10) — it cannot clear the formal gate until re-run without the mock endpoint")
	}
	return 0
}

// cloudFamilyOf resolves the §5.4 provider type to the family display-unit
// rules the compare engine consumes (per-adapter mapping.md §2). It lives in
// main, not the engine: the engine is a core package and must not branch on
// provider identifiers (arch R2) — it only ever sees the rules as data.
func cloudFamilyOf(providerType string) inventory.CloudFamily {
	switch contract.ProviderTypeAliases[strings.ToLower(strings.TrimSpace(providerType))] {
	case "tencent":
		// tencent mapping.md §2 — legacy 표시 GB = Memory/1024 ("MB 전제"
		// display), V2 memoryGB = the API GB value as-is.
		return inventory.CloudFamily{LegacyMemoryDivisor: 1024}
	default:
		// aliyun mapping.md §2 — legacy display (MB/1024) already is the V2
		// scale; any future family defaults to no conversion until its
		// mapping documents otherwise.
		return inventory.CloudFamily{}
	}
}

// runCompareCloudGate evaluates the 3-day cloud gate (§15.4 r2 + §13-10) over
// the stored artifacts of one cloud account — no database access needed.
func runCompareCloudGate(dataDir string, accountID uint) int {
	accountName := strconv.FormatUint(uint64(accountID), 10)
	result := inventory.EvaluateCloudGate(collectCloudCompareArtifacts(dataDir, accountName))
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory-cloud: encode gate result: %v\n", err)
		return 1
	}
	if result.Passed {
		return 0
	}
	for _, reason := range result.Reasons {
		fmt.Fprintln(os.Stderr, "compare-inventory-cloud: gate: "+reason)
	}
	return 1
}

// collectCloudCompareArtifacts lists the artifact files of one cloud account
// directory.
func collectCloudCompareArtifacts(dataDir, accountName string) []string {
	root := filepath.Join(dataDir, "compare", "cloud", accountName)
	var paths []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil // a missing directory is an empty gate input
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	return paths
}

// writeCloudCompareArtifact writes the dated artifact:
// data/compare/cloud/<account>/<date>/<time>.json (plan §2 운영 산출, r2) —
// the account directory makes the gate's same-account check machine-readable.
func writeCloudCompareArtifact(dataDir string, accountID uint, at time.Time, artifact inventory.CloudCompareArtifact) (string, error) {
	dir := filepath.Join(dataDir, "compare", "cloud", strconv.FormatUint(uint64(accountID), 10), at.Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, at.Format("150405")+".json")
	b, err := inventory.MarshalCloudArtifact(artifact)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
