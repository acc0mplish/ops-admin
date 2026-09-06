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
	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/internal/infra/migrate"
	inframodel "ops-admin/backend/internal/infra/model"
	"ops-admin/backend/model"
	"ops-admin/backend/service"
	"ops-admin/backend/store"
	"ops-admin/backend/util"
)

// runCompareInventory implements the "compare-inventory" command (§15): one
// paired run — the legacy capture through a fresh Service (J4: an empty
// process cache makes the first GetK8sClusterDetail call the uncached path
// the v1 API handlers call), the V2 side read back through the public
// projection queries — classified into the dated artifact under the
// deployment's data directory. --gate instead evaluates the 3-day gate over
// the stored artifacts (§15.4 r2). A BLOCKER verdict exits non-zero; the
// artifact itself is always the primary output.
func runCompareInventory(args []string) int {
	flags := flag.NewFlagSet("compare-inventory", flag.ContinueOnError)
	configPath := flags.String("config", "config.yaml", "path to config.yaml")
	clusterID := flags.Uint("cluster", 0, "v1 k8s_cluster id to pair (required)")
	dataDir := flags.String("data", "data", "data directory root for report artifacts")
	gate := flags.Bool("gate", false, "evaluate the 3-day gate over stored artifacts instead of capturing a pair")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory: %v\n", err)
		return 1
	}
	if *gate {
		return runCompareGate(*dataDir, *clusterID)
	}
	if *clusterID == 0 {
		fmt.Fprintln(os.Stderr, "compare-inventory: --cluster is required")
		return 1
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory: load config: %v\n", err)
		return 1
	}
	util.ConfigureCredentialKey(cfg.Security.CredentialKey)
	if err := util.EnsureSecretKeySource(cfg.Security.CredentialKey); err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory: secret key source: %v\n", err)
		return 1
	}
	db, err := store.NewDB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory: connect db: %v\n", err)
		return 1
	}
	if err := migrate.Run(context.Background(), db); err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory: v2 schema migration: %v\n", err)
		return 1
	}

	// v1 cluster row — read-only (R9: compare never writes v1 state).
	var cluster model.K8sCluster
	if err := db.First(&cluster, *clusterID).Error; err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory: load k8s_cluster %d: %v\n", *clusterID, err)
		return 1
	}

	// V2 side: the backfilled connection, its context, and the public
	// generation (§3.5 r2 공개 generation 규약).
	var conn inframodel.ProviderConnection
	err = db.Where("source_model = ? AND source_id = ? AND stale_source = ?", "k8s_cluster", *clusterID, false).
		First(&conn).Error
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory: no live V2 connection for cluster %d — run sync-inventory first: %v\n", *clusterID, err)
		return 1
	}
	var pctx inframodel.ProviderContext
	if err := db.Where("connection_id = ?", conn.ID).Order("id").First(&pctx).Error; err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory: load provider context: %v\n", err)
		return 1
	}
	generation, err := inventory.LatestAuthoritativeGeneration(db, pctx.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory: %v\n", err)
		return 1
	}
	projected, err := inventory.ProjectResources(db, pctx.ID, generation.GenerationUID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory: project resources: %v\n", err)
		return 1
	}

	// Legacy capture — fresh Service (J4): empty overview cache ⇒ the first
	// detail call is the uncached path; single-shot process, Shutdown after
	// (R6).
	svc := service.New(db)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = svc.Shutdown(ctx)
	}()

	captureOnce := func(at time.Time) (inventory.LegacyCapture, error) {
		detail, err := svc.GetK8sClusterDetail(*clusterID)
		if err != nil {
			return inventory.LegacyCapture{}, err
		}
		return legacyCaptureFromDetail(detail, at), nil
	}

	legacyAt := time.Now()
	legacy, err := captureOnce(legacyAt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory: legacy capture: %v\n", err)
		return 1
	}
	attempt := 1
	pairInput := inventory.PairInput{
		ClusterID: *clusterID, ClusterName: cluster.Name, Attempt: attempt,
		Legacy: legacy,
		V2: inventory.ProjectedV2{
			SyncedAt: generation.CommittedAt, GenerationUID: generation.GenerationUID, Resources: projected,
		},
	}
	report := inventory.ComparePair(pairInput)
	if report.Verdict == inventory.VerdictRePair {
		// §15.1 — beyond the 60s window: one re-pair, volatile sections ride
		// along; a second miss is a BLOCKER (no unbounded recapture).
		attempt = 2
		legacy, err = captureOnce(time.Now())
		if err != nil {
			fmt.Fprintf(os.Stderr, "compare-inventory: legacy re-capture: %v\n", err)
			return 1
		}
		pairInput.Attempt = attempt
		pairInput.Legacy = legacy
		report = inventory.ComparePair(pairInput)
	}

	artifact := inventory.CompareArtifact{
		ArtifactSchema:   inventory.CompareArtifactSchema,
		ClusterID:        *clusterID,
		ClusterName:      cluster.Name,
		Verdict:          report.Verdict,
		Attempt:          attempt,
		TsDeltaSeconds:   report.TsDelta.Seconds(),
		LegacyCapturedAt: pairInput.Legacy.CapturedAt,
		LegacyHash:       inventory.LegacyCaptureHash(pairInput.Legacy),
		V2GenerationUID:  pairInput.V2.GenerationUID,
		V2SyncedAt:       pairInput.V2.SyncedAt,
		V2Hash:           inventory.ProjectionHash(pairInput.V2),
		Report:           report,
	}
	path, err := writeCompareArtifact(*dataDir, *clusterID, pairInput.Legacy.CapturedAt, artifact)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory: write report artifact: %v\n", err)
		return 1
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(artifact); err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory: encode report: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "compare-inventory: report artifact %s\n", path)
	if report.Verdict != inventory.VerdictPass {
		fmt.Fprintf(os.Stderr, "compare-inventory: verdict %q — %d blocker(s)\n", report.Verdict, len(report.Blockers))
		return 1
	}
	return 0
}

// runCompareGate evaluates the 3-day gate (§15.4 r2) over the stored
// artifacts of one cluster — no database access needed.
func runCompareGate(dataDir string, clusterID uint) int {
	clusterName := strconv.FormatUint(uint64(clusterID), 10)
	result := inventory.EvaluateGate(collectCompareArtifacts(dataDir, clusterName))
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "compare-inventory: encode gate result: %v\n", err)
		return 1
	}
	if result.Passed {
		return 0
	}
	for _, reason := range result.Reasons {
		fmt.Fprintln(os.Stderr, "compare-inventory: gate: "+reason)
	}
	return 1
}

// collectCompareArtifacts lists the artifact files of one cluster directory.
func collectCompareArtifacts(dataDir, clusterName string) []string {
	root := filepath.Join(dataDir, "compare", clusterName)
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

// legacyCaptureFromDetail copies the compared sections of the v1 DTO into the
// engine's neutral capture — no credential-shaped field is carried (보존
// 제약 #7).
func legacyCaptureFromDetail(detail model.K8sClusterDetail, at time.Time) inventory.LegacyCapture {
	capture := inventory.LegacyCapture{CapturedAt: at}
	for _, n := range detail.Nodes {
		capture.Nodes = append(capture.Nodes, inventory.LegacyNode{
			Name: n.Name, Role: n.Role, Status: n.Status, CPU: n.CPU, Memory: n.Memory,
		})
	}
	for _, n := range detail.Namespaces {
		capture.Namespaces = append(capture.Namespaces, inventory.LegacyNamespace{Name: n.Name, Status: n.Status})
	}
	for _, p := range detail.Pods {
		capture.Pods = append(capture.Pods, inventory.LegacyPod{
			Name: p.Name, Namespace: p.Namespace, Status: p.Status, Restarts: p.Restarts,
		})
	}
	for _, w := range detail.Workloads {
		capture.Workloads = append(capture.Workloads, inventory.LegacyWorkload{
			Name: w.Name, Type: w.Type, Namespace: w.Namespace, Ready: w.Ready,
		})
	}
	for _, s := range detail.Network.Services {
		capture.Services = append(capture.Services, inventory.LegacyService{
			Name: s.Name, Namespace: s.Namespace, Type: s.Type, ClusterIP: s.ClusterIP, Ports: s.Ports,
		})
	}
	for _, i := range detail.Network.Ingresses {
		capture.Ingresses = append(capture.Ingresses, inventory.LegacyIngress{Name: i.Name, Namespace: i.Namespace})
	}
	for _, cm := range detail.ConfigStorage.ConfigMaps {
		capture.ConfigMaps = append(capture.ConfigMaps, inventory.LegacyConfigMap{
			Name: cm.Name, Namespace: cm.Namespace, Keys: cm.Keys,
		})
	}
	for _, s := range detail.ConfigStorage.Secrets {
		// The legacy secret list item serializes name/namespace/type/age only
		// — no key count (mapping.md §4.9 notes the keys row as v2-only for
		// the pair comparison).
		capture.Secrets = append(capture.Secrets, inventory.LegacySecret{
			Name: s.Name, Namespace: s.Namespace, Type: s.Type,
		})
	}
	for _, s := range detail.ConfigStorage.Storage {
		capture.Storage = append(capture.Storage, inventory.LegacyStorage{
			Name: s.Name, Kind: s.Kind, Namespace: s.Namespace,
			Capacity: s.Capacity, StorageClass: s.StorageClass, AccessModes: s.AccessModes,
		})
	}
	return capture
}

// writeCompareArtifact writes the dated artifact:
// data/compare/<cluster>/<date>/<time>.json (plan §2 운영 산출, r2) — the
// cluster directory makes the gate's same-cluster check machine-readable.
func writeCompareArtifact(dataDir string, clusterID uint, at time.Time, artifact inventory.CompareArtifact) (string, error) {
	dir := filepath.Join(dataDir, "compare", strconv.FormatUint(uint64(clusterID), 10), at.Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, at.Format("150405")+".json")
	b, err := inventory.MarshalArtifact(artifact)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
