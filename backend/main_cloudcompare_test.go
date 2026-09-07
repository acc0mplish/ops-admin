package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ops-admin/backend/internal/infra/adapter/aliyun"
	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/internal/infra/metrics"
	"ops-admin/backend/internal/infra/migrate"
	inframodel "ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/infra/secrets"
	"ops-admin/backend/internal/testutil"
	"ops-admin/backend/service"
	"ops-admin/backend/util"
)

// mockAliyunECS serves the Aliyun RPC JSON the legacy capture and the V2
// adapter both speak — one region, one instance, no signature validation (the
// mock pair run reads the SAME responses on both sides, which is exactly what
// makes a pass verdict meaningful, plan phase4 E-1(b)/E-2).
func mockAliyunECS(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		action := r.URL.Query().Get("Action")
		switch action {
		case "DescribeRegions":
			_, _ = w.Write([]byte(`{"Regions":{"Region":[{"RegionId":"cn-hangzhou"}]}}`))
		case "DescribeInstances":
			_, _ = w.Write([]byte(`{"TotalCount":1,"Instances":{"Instance":[{` +
				`"InstanceId":"i-mock-1","InstanceName":"web-1","Cpu":4,"Memory":8192,` +
				`"OSName":"Ubuntu 22.04","RegionId":"cn-hangzhou","ZoneId":"cn-hangzhou-a",` +
				`"InstanceType":"ecs.g6.large","Status":"Running",` +
				`"PublicIpAddress":{"IpAddress":["203.0.113.7"]},` +
				`"VpcAttributes":{"PrivateIpAddress":{"IpAddress":["10.0.0.1"]}},` +
				`"SystemDisk":{"Size":40}}]}}`))
		default:
			http.Error(w, "unknown action "+action, http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

const cloudAccountCompareDDL = `CREATE TABLE IF NOT EXISTS asset_cloud_account (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	provider TEXT NOT NULL,
	access_key TEXT DEFAULT '',
	secret_key TEXT DEFAULT '',
	regions TEXT DEFAULT '[]',
	region TEXT DEFAULT '',
	status INTEGER DEFAULT 1,
	description TEXT DEFAULT '',
	updated_at DATETIME
)`

// TestRunCompareCloudMockPairProducesPassArtifact drives the full E-1(b)
// interim proof path end to end: the v1 account → cloud backfill → one V2
// sync generation through the real adapter at the mock endpoint
// (Connection.Endpoint — J1 Path M), the legacy capture through the same mock
// via the E-2 development override, then the §15 cloud pair classified into a
// §13-10 interim-marked pass artifact.
func TestRunCompareCloudMockPairProducesPassArtifact(t *testing.T) {
	testutil.PinSecretKeys(t)
	srv := mockAliyunECS(t)
	ctx := context.Background()

	db := testutil.OpenMemoryDB(t)
	if err := migrate.Run(ctx, db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}
	if err := db.Exec(cloudAccountCompareDDL).Error; err != nil {
		t.Fatalf("create v1 source table: %v", err)
	}
	if err := db.Table("asset_cloud_account").Create(map[string]any{
		"name": "acct-mock", "provider": "aliyun", "access_key": "AK-mock",
		"secret_key": "SK-mock", "regions": `["cn-hangzhou"]`, "region": "cn-hangzhou",
		"status": 1, "updated_at": time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC),
	}).Error; err != nil {
		t.Fatalf("seed v1 cloud account: %v", err)
	}
	var accountID uint
	if err := db.Table("asset_cloud_account").Where("name = ?", "acct-mock").Select("id").Scan(&accountID).Error; err != nil || accountID == 0 {
		t.Fatalf("reload v1 cloud account id: %v (id %d)", err, accountID)
	}

	// §5.4 propagation — connection + account context + sealed credential.
	if _, err := inventory.RunCloudAccountBackfill(ctx, db); err != nil {
		t.Fatalf("RunCloudAccountBackfill: %v", err)
	}
	var conn inframodel.ProviderConnection
	if err := db.Where("source_model = ? AND source_id = ?", "asset_cloud_account", accountID).
		First(&conn).Error; err != nil {
		t.Fatalf("load backfilled connection: %v", err)
	}
	// J1 Path M — the V2 side reaches the mock through the sanctioned
	// Connection.Endpoint injection (no production wiring touched).
	if err := db.Model(&inframodel.ProviderConnection{}).Where("id = ?", conn.ID).
		Update("endpoint", srv.URL).Error; err != nil {
		t.Fatalf("inject mock endpoint: %v", err)
	}

	// One committed V2 generation through the real aliyun adapter.
	reg := registry.New()
	counters := metrics.New()
	adapter := aliyun.NewAdapter(aliyun.WithCounters(counters))
	if err := reg.RegisterProviderType(adapter.Descriptor(), adapter); err != nil {
		t.Fatalf("register adapter: %v", err)
	}
	runner := inventory.NewSyncRunner(db, reg, secrets.NewBroker(db), counters)
	syncReport, err := runner.RunSync(ctx, inventory.SyncInput{ConnectionUID: conn.UID, Mode: "full"})
	if err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	if syncReport.Status != "succeeded" {
		t.Fatalf("sync status = %q, want succeeded", syncReport.Status)
	}

	// The legacy side reads the SAME mock through the E-2 override.
	t.Setenv("GO_ENV", "development")
	t.Setenv(util.CloudEndpointOverrideEnvPrefix+"ALIYUN", srv.URL)
	if active, overrideErr := util.CloudEndpointOverrideActive("aliyun"); overrideErr != nil || !active {
		t.Fatalf("mock override inactive: %v", overrideErr)
	}
	svc := service.New(db)
	capture, err := svc.CaptureCloudInstancesForCompare(accountID)
	if err != nil {
		t.Fatalf("legacy capture: %v", err)
	}
	if len(capture.Instances) != 1 || capture.Instances[0].InstanceID != "i-mock-1" {
		t.Fatalf("legacy capture did not reach the mock: %+v", capture.Instances)
	}

	// V2 projection through the public queries.
	var pctx inframodel.ProviderContext
	if err := db.Where("connection_id = ?", conn.ID).Order("id").First(&pctx).Error; err != nil {
		t.Fatalf("load provider context: %v", err)
	}
	generation, err := inventory.LatestAuthoritativeGeneration(db, pctx.ID)
	if err != nil {
		t.Fatalf("LatestAuthoritativeGeneration: %v", err)
	}
	projected, err := inventory.ProjectResources(db, pctx.ID, generation.GenerationUID)
	if err != nil {
		t.Fatalf("ProjectResources: %v", err)
	}

	pairInput := inventory.CloudPairInput{
		AccountID: accountID, AccountName: "acct-mock", Attempt: 1,
		Legacy: capture,
		V2: inventory.ProjectedV2{
			SyncedAt: generation.CommittedAt, GenerationUID: generation.GenerationUID, Resources: projected,
		},
	}
	report := inventory.CompareCloudPair(pairInput)
	if report.Verdict != inventory.VerdictPass {
		t.Fatalf("mock pair verdict = %q, want pass (blockers %+v, volatiles %+v)",
			report.Verdict, report.Blockers, report.Volatiles)
	}

	// The §13-10 interim-marked artifact.
	artifact := inventory.CloudCompareArtifact{
		ArtifactSchema:   inventory.CloudCompareArtifactSchema,
		AccountID:        accountID,
		AccountName:      "acct-mock",
		Verdict:          report.Verdict,
		Attempt:          1,
		TsDeltaSeconds:   report.TsDelta.Seconds(),
		LegacyCapturedAt: capture.CapturedAt,
		LegacyHash:       inventory.CloudLegacyCaptureHash(capture),
		V2GenerationUID:  generation.GenerationUID,
		V2SyncedAt:       generation.CommittedAt,
		V2Hash:           inventory.ProjectionHash(pairInput.V2),
		Report:           report,
		Interim:          true,
		InterimReason:    "real-account proof pending (E-1): the pair was served by the development endpoint override",
		Scope:            "mock-endpoint",
	}
	dataDir := t.TempDir()
	path, err := writeCloudCompareArtifact(dataDir, accountID, capture.CapturedAt, artifact)
	if err != nil {
		t.Fatalf("writeCloudCompareArtifact: %v", err)
	}
	if !strings.Contains(path, filepath.Join("compare", "cloud")) {
		t.Fatalf("artifact path %q is not under compare/cloud/", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	t.Logf("mock pair artifact %s:\n%s", path, b)
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	if parsed["verdict"] != inventory.VerdictPass {
		t.Fatalf("artifact verdict = %v, want pass", parsed["verdict"])
	}
	if parsed["interim"] != true || parsed["scope"] != "mock-endpoint" {
		t.Fatalf("artifact interim marking missing: interim=%v scope=%v", parsed["interim"], parsed["scope"])
	}

	// An interim artifact cannot clear the formal gate (§13-10 promotion rule).
	if res := inventory.EvaluateCloudGate([]string{path}); res.Passed {
		t.Fatalf("the interim artifact must not clear the formal gate")
	}
}

// --account is mandatory for the capture path.
func TestRunCompareCloudRequiresAccount(t *testing.T) {
	if code := runCompareCloudInventory([]string{}); code != 1 {
		t.Fatalf("exit code = %d, want 1 for missing --account", code)
	}
}

// --gate evaluates the artifacts under the data directory: an empty directory
// fails the gate (exit 1) without touching the database.
func TestRunCompareCloudGateEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	if code := runCompareCloudInventory([]string{"--gate", "--account", "3", "--data", dir}); code != 1 {
		t.Fatalf("exit code = %d, want 1 for an empty artifact directory", code)
	}
}

// A set-but-invalid override is refused before any capture happens — the
// legacy side would otherwise reach the operating provider mid-"mock" run.
func TestRunCompareCloudRefusesInvalidOverride(t *testing.T) {
	t.Setenv("GO_ENV", "production")
	t.Setenv(util.CloudEndpointOverrideEnvPrefix+"ALIYUN", "http://127.0.0.1:9")
	if code := runCompareCloudInventory([]string{"--account", "1"}); code != 1 {
		t.Fatalf("exit code = %d, want 1 for an invalid override", code)
	}
}
