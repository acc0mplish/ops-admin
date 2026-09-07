// Cloud-account backfill tests (plan phase4 r2 §2 N12 — C1): the
// asset_cloud_account → V2 chain propagation (J4 credential collapse as one
// SecretRef JSON blob) with per-account error isolation (J6), idempotent
// re-runs, stale marking, and the finops link/binding collapse (A7).
package inventory_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/util"
	v1model "ops-admin/backend/model"
)

// v1 source DDL — minimal column shapes (gorm default naming verified against
// model.AssetCloudAccount / model.IntegrationFinOpsAccount).
const assetCloudAccountDDL = `CREATE TABLE IF NOT EXISTS asset_cloud_account (
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

const integrationFinopsAccountDDL = `CREATE TABLE IF NOT EXISTS integration_finops_account (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	provider TEXT NOT NULL,
	account_identifier TEXT DEFAULT '',
	access_key TEXT DEFAULT '',
	secret_key TEXT DEFAULT '',
	billing_token TEXT DEFAULT '',
	status INTEGER DEFAULT 1,
	provider_connection_uid TEXT DEFAULT '',
	updated_at DATETIME
)`

type cloudAccountSeed struct {
	name, provider, accessKey, secretKey string
	regions                              string
	status                               int
	updatedAt                            time.Time
}

func seedCloudAccounts(t *testing.T, db *gorm.DB, seeds []cloudAccountSeed) []uint {
	t.Helper()
	ids := make([]uint, 0, len(seeds))
	for _, s := range seeds {
		status := s.status
		if status == 0 {
			status = 1
		}
		updatedAt := s.updatedAt
		if updatedAt.IsZero() {
			updatedAt = time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
		}
		if err := db.Table("asset_cloud_account").Create(map[string]any{
			"name": s.name, "provider": s.provider, "access_key": s.accessKey,
			"secret_key": s.secretKey, "regions": s.regions, "region": "cn-hangzhou",
			"status": status, "description": "", "updated_at": updatedAt,
		}).Error; err != nil {
			t.Fatalf("seed asset_cloud_account %s: %v", s.name, err)
		}
		var id uint
		if err := db.Table("asset_cloud_account").Where("name = ?", s.name).Select("id").Scan(&id).Error; err != nil {
			t.Fatalf("reload cloud account id: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

type finopsSeed struct {
	name, provider, accessKey, secretKey, billingToken string
}

func seedFinopsAccounts(t *testing.T, db *gorm.DB, seeds []finopsSeed) []uint {
	t.Helper()
	ids := make([]uint, 0, len(seeds))
	for _, s := range seeds {
		if err := db.Table("integration_finops_account").Create(map[string]any{
			"name": s.name, "provider": s.provider, "access_key": s.accessKey,
			"secret_key": s.secretKey, "billing_token": s.billingToken,
			"status": 1, "updated_at": time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC),
		}).Error; err != nil {
			t.Fatalf("seed integration_finops_account %s: %v", s.name, err)
		}
		var id uint
		if err := db.Table("integration_finops_account").Where("name = ?", s.name).Select("id").Scan(&id).Error; err != nil {
			t.Fatalf("reload finops account id: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

// cloudChainOf loads the V2 chain pieces backfilled for a v1 source row.
type cloudChainOf struct {
	Connection model.ProviderConnection
	Context    model.ProviderContext
	Ref        model.SecretRef
	Inventory  *model.ProviderCredentialBinding
	Billing    *model.ProviderCredentialBinding
}

func loadCloudChain(t *testing.T, db *gorm.DB, sourceModel string, sourceID uint) (cloudChainOf, bool) {
	t.Helper()
	out := cloudChainOf{}
	err := db.Where("source_model = ? AND source_id = ?", sourceModel, sourceID).First(&out.Connection).Error
	if err == gorm.ErrRecordNotFound {
		return out, false
	}
	if err != nil {
		t.Fatalf("load backfilled connection: %v", err)
	}
	if err := db.Where("connection_id = ?", out.Connection.ID).First(&out.Context).Error; err != nil {
		t.Fatalf("load backfilled context: %v", err)
	}
	var inventory, billing model.ProviderCredentialBinding
	if err := db.Where("provider_connection_id = ? AND purpose = ?", out.Connection.ID, "inventory").First(&inventory).Error; err == nil {
		out.Inventory = &inventory
	}
	if err := db.Where("provider_connection_id = ? AND purpose = ?", out.Connection.ID, "billing").First(&billing).Error; err == nil {
		out.Billing = &billing
	}
	if out.Inventory != nil {
		if err := db.First(&out.Ref, out.Inventory.SecretRefID).Error; err != nil {
			t.Fatalf("load backfilled secret_ref: %v", err)
		}
	}
	return out, true
}

func finopsLink(t *testing.T, db *gorm.DB, id uint) string {
	t.Helper()
	var uid string
	if err := db.Table("integration_finops_account").Where("id = ?", id).
		Select("provider_connection_uid").Scan(&uid).Error; err != nil {
		t.Fatalf("reload finops link: %v", err)
	}
	return uid
}

// TestCloudBackfillSealsCredentialBlobAndIsIdempotent — J4: one cloud account
// becomes connection + account context + ONE SecretRef carrying the sealed
// JSON credential blob; re-runs are idempotent; the v1 row keeps its P-class
// plaintext (§13-9 — copy, never erase).
func TestCloudBackfillSealsCredentialBlobAndIsIdempotent(t *testing.T) {
	db := newInventoryDB(t)
	if err := db.Exec(assetCloudAccountDDL).Error; err != nil {
		t.Fatalf("create asset_cloud_account: %v", err)
	}
	ids := seedCloudAccounts(t, db, []cloudAccountSeed{
		{name: "prod-ali", provider: "alicloud", accessKey: "AKID-legacy", secretKey: "SK-legacy", regions: `["cn-hangzhou","cn-beijing"]`},
		{name: "prod-tc", provider: "tencentcloud", accessKey: "TCID-legacy", secretKey: "TCSK-legacy", regions: `["ap-guangzhou"]`},
	})

	report, err := inventory.RunCloudAccountBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("backfill run 1: %v", err)
	}
	if report.Created != 2 {
		t.Errorf("run 1 created = %d, want 2", report.Created)
	}

	chain, ok := loadCloudChain(t, db, "asset_cloud_account", ids[0])
	if !ok {
		t.Fatal("aliyun chain missing after backfill")
	}
	if chain.Connection.ProviderType != "aliyun" {
		t.Errorf("provider_type = %q, want aliyun (alicloud normalized)", chain.Connection.ProviderType)
	}
	if chain.Context.Kind != "account" || chain.Context.ExternalID != "AKID-legacy" {
		t.Errorf("context = kind %q external %q, want account/AKID-legacy", chain.Context.Kind, chain.Context.ExternalID)
	}
	var metadata map[string]any
	raw, err := json.Marshal(chain.Context.MetadataJSON)
	if err != nil {
		t.Fatalf("marshal context metadata: %v", err)
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("decode context metadata: %v", err)
	}
	if metadata["sourceProvider"] != "alicloud" {
		t.Errorf("sourceProvider = %v, want alicloud (raw lexicon kept)", metadata["sourceProvider"])
	}
	regions, _ := metadata["regions"].([]any)
	if len(regions) != 2 {
		t.Errorf("regions in metadata = %v, want 2 entries", metadata["regions"])
	}

	// The SecretRef material decrypts to the sealed JSON blob.
	plain, err := util.DecryptSecretV2(chain.Ref.Ciphertext)
	if err != nil {
		t.Fatalf("decrypt backfilled secret_ref: %v", err)
	}
	var cred map[string]string
	if err := json.Unmarshal([]byte(plain), &cred); err != nil {
		t.Fatalf("sealed material is not the credential JSON blob: %v", err)
	}
	if cred["accessKey"] != "AKID-legacy" || cred["secretKey"] != "SK-legacy" {
		t.Errorf("sealed blob = %v, want the legacy ak/sk pair", cred)
	}
	if chain.Inventory == nil || chain.Inventory.Purpose != "inventory" {
		t.Fatalf("inventory binding missing: %+v", chain.Inventory)
	}

	// tencentcloud normalized onto tencent.
	tcChain, ok := loadCloudChain(t, db, "asset_cloud_account", ids[1])
	if !ok || tcChain.Connection.ProviderType != "tencent" {
		t.Fatalf("tencent chain provider_type = %+v/%v, want tencent", tcChain.Connection.ProviderType, ok)
	}

	// The v1 row keeps its P-class plaintext — copy, never erase (§13-9).
	var v1Access string
	if err := db.Table("asset_cloud_account").Where("id = ?", ids[0]).
		Select("access_key").Scan(&v1Access).Error; err != nil {
		t.Fatal(err)
	}
	if v1Access != "AKID-legacy" {
		t.Errorf("v1 access_key rewritten to %q — backfill must be read-only on v1", v1Access)
	}

	// Runs 2 and 3 — idempotent.
	for run := 2; run <= 3; run++ {
		again, err := inventory.RunCloudAccountBackfill(context.Background(), db)
		if err != nil {
			t.Fatalf("backfill run %d: %v", run, err)
		}
		if again.Created != 0 || again.Updated != 0 || again.Unchanged != 2 {
			t.Errorf("run %d report = %+v, want created=0 updated=0 unchanged=2", run, again)
		}
		var connCount, refCount, bindingCount int64
		db.Model(&model.ProviderConnection{}).Where("source_model = ?", "asset_cloud_account").Count(&connCount)
		db.Model(&model.SecretRef{}).Where("path LIKE ?", "backfill/asset_cloud_account/%").Count(&refCount)
		db.Model(&model.ProviderCredentialBinding{}).Where("purpose = ?", "inventory").Count(&bindingCount)
		if connCount != 2 || refCount != 2 || bindingCount != 2 {
			t.Fatalf("run %d duplicated chain rows: conn=%d ref=%d binding=%d", run, connCount, refCount, bindingCount)
		}
	}
}

// TestCloudBackfillFinopsCollapse — A7's three branches, one cloud account
// per branch (J4's connection-per-purpose one-row rule means the three
// branches must not contend for the same connection's billing binding): exact
// credential equality shares the inventory SecretRef for the billing
// binding; the same accessKey with a different secret re-points billing at
// its own SecretRef; a different accessKey gets a separate chain.
func TestCloudBackfillFinopsCollapse(t *testing.T) {
	db := newInventoryDB(t)
	if err := db.Exec(assetCloudAccountDDL).Error; err != nil {
		t.Fatalf("create asset_cloud_account: %v", err)
	}
	if err := db.Exec(integrationFinopsAccountDDL).Error; err != nil {
		t.Fatalf("create integration_finops_account: %v", err)
	}
	ids := seedCloudAccounts(t, db, []cloudAccountSeed{
		{name: "cloud-exact", provider: "alicloud", accessKey: "AK-A", secretKey: "SK-A", regions: `["cn-hangzhou"]`},
		{name: "cloud-rotated", provider: "alicloud", accessKey: "AK-B", secretKey: "SK-B", regions: `["cn-hangzhou"]`},
	})
	finopsIDs := seedFinopsAccounts(t, db, []finopsSeed{
		{name: "finops-exact", provider: "alicloud", accessKey: "AK-A", secretKey: "SK-A"},
		{name: "finops-rotated", provider: "alicloud", accessKey: "AK-B", secretKey: "SK-B-ROTATED"},
		{name: "finops-other", provider: "alicloud", accessKey: "AK-C", secretKey: "SK-C"},
	})

	report, err := inventory.RunCloudAccountBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}

	exactChain, ok := loadCloudChain(t, db, "asset_cloud_account", ids[0])
	if !ok {
		t.Fatal("exact branch cloud chain missing")
	}
	rotatedCloudChain, ok := loadCloudChain(t, db, "asset_cloud_account", ids[1])
	if !ok {
		t.Fatal("rotated branch cloud chain missing")
	}

	// (a) exact match — the billing binding shares the inventory SecretRef.
	if finopsLink(t, db, finopsIDs[0]) != exactChain.Connection.UID {
		t.Fatalf("exact-match finops link = %q, want %q", finopsLink(t, db, finopsIDs[0]), exactChain.Connection.UID)
	}
	var shared model.ProviderCredentialBinding
	if err := db.Where("provider_connection_id = ? AND purpose = ?", exactChain.Connection.ID, "billing").First(&shared).Error; err != nil {
		t.Fatalf("billing binding missing for the shared branch: %v", err)
	}
	if shared.SecretRefID != exactChain.Ref.ID {
		t.Errorf("shared billing binding points at secret_ref %d, want the inventory ref %d", shared.SecretRefID, exactChain.Ref.ID)
	}
	if report.CollapsedShared != 1 {
		t.Errorf("collapsedShared = %d, want 1 for the exact-match branch", report.CollapsedShared)
	}

	// (b) same accessKey, rotated secret — billing points at its own
	// SecretRef (§7.4 forbids sharing different material), the link still
	// reuses the connection.
	if finopsLink(t, db, finopsIDs[1]) != rotatedCloudChain.Connection.UID {
		t.Errorf("rotated-secret branch link = %q, want the cloud connection uid (connection reuse)", finopsLink(t, db, finopsIDs[1]))
	}
	var rotated model.ProviderCredentialBinding
	if err := db.Where("provider_connection_id = ? AND purpose = ?", rotatedCloudChain.Connection.ID, "billing").First(&rotated).Error; err != nil {
		t.Fatalf("rotated branch billing binding missing: %v", err)
	}
	if rotated.SecretRefID == rotatedCloudChain.Ref.ID {
		t.Errorf("rotated secret must not share the inventory SecretRef (§7.4)")
	}

	// (c) different accessKey — separate chain with its own billing binding.
	otherChain, ok := loadCloudChain(t, db, "integration_finops_account", finopsIDs[2])
	if !ok {
		t.Fatal("mismatched finops account must get its own chain")
	}
	if otherChain.Connection.ProviderType != "aliyun" {
		t.Errorf("own-chain provider_type = %q, want aliyun", otherChain.Connection.ProviderType)
	}
	if otherChain.Billing == nil {
		t.Error("own-chain billing binding missing")
	}
	if finopsLink(t, db, finopsIDs[2]) != otherChain.Connection.UID {
		t.Errorf("own-chain link = %q, want %q", finopsLink(t, db, finopsIDs[2]), otherChain.Connection.UID)
	}
	if report.FinopsLinked != 3 {
		t.Errorf("finopsLinked = %d, want 3", report.FinopsLinked)
	}

	// Re-run — everything idempotent, link counts stay stable.
	again, err := inventory.RunCloudAccountBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("re-run: %v", err)
	}
	if again.Created != 0 || again.Updated != 0 {
		t.Errorf("re-run created/updated = %d/%d, want 0/0", again.Created, again.Updated)
	}
	if again.FinopsLinked != 0 {
		t.Errorf("re-run finopsLinked = %d, want 0 (links already written)", again.FinopsLinked)
	}
	if again.CollapsedShared != 0 {
		t.Errorf("re-run collapsedShared = %d, want 0 (binding already in place)", again.CollapsedShared)
	}
	if finopsLink(t, db, finopsIDs[0]) != exactChain.Connection.UID {
		t.Error("re-run must not disturb the collapse links")
	}
}

// TestCloudBackfillStaleMarkingAndErrorIsolation — J6: v1 rows that vanish
// mark their chains stale; a failed seal halts only its own account while the
// remaining accounts and the stale marking still run.
func TestCloudBackfillStaleMarkingAndErrorIsolation(t *testing.T) {
	db := newInventoryDB(t)
	if err := db.Exec(assetCloudAccountDDL).Error; err != nil {
		t.Fatalf("create asset_cloud_account: %v", err)
	}
	ids := seedCloudAccounts(t, db, []cloudAccountSeed{
		{name: "doomed", provider: "aliyun", accessKey: "AK-DOOMED", secretKey: "SK", regions: `[]`},
		{name: "broken", provider: "aliyun", accessKey: "v2:missing-key:not-an-envelope", secretKey: "SK", regions: `[]`},
		{name: "survivor", provider: "aliyun", accessKey: "AK-OK", secretKey: "SK", regions: `[]`},
	})

	report, err := inventory.RunCloudAccountBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if report.Failed != 1 {
		t.Errorf("failed = %d, want 1 (only the unsealable account)", report.Failed)
	}
	if len(report.FailedSources) != 1 {
		t.Fatalf("failedSources = %v, want exactly the broken account", report.FailedSources)
	}
	if _, ok := loadCloudChain(t, db, "asset_cloud_account", ids[1]); ok {
		t.Error("the broken account must not leave a chain (halt is row-scoped)")
	}
	if _, ok := loadCloudChain(t, db, "asset_cloud_account", ids[2]); !ok {
		t.Error("survivor account must still be backfilled after the failure (error isolation)")
	}

	// Stale marking: remove the doomed v1 row, re-run.
	if err := db.Table("asset_cloud_account").Where("id = ?", ids[0]).Delete(nil).Error; err != nil {
		t.Fatalf("delete doomed row: %v", err)
	}
	second, err := inventory.RunCloudAccountBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if second.MarkedStale != 1 {
		t.Errorf("markedStale = %d, want 1", second.MarkedStale)
	}
	var conn model.ProviderConnection
	if err := db.Where("source_model = ? AND source_id = ?", "asset_cloud_account", ids[0]).First(&conn).Error; err != nil {
		t.Fatalf("reload doomed connection: %v", err)
	}
	if !conn.StaleSource {
		t.Error("doomed connection must be stale_source=true")
	}
}

// TestCloudBackfillIncrementalCheckpoint — §5.4a: a re-run reprocesses only
// rows whose updated_at moved past the stored checkpoint.
func TestCloudBackfillIncrementalCheckpoint(t *testing.T) {
	db := newInventoryDB(t)
	if err := db.Exec(assetCloudAccountDDL).Error; err != nil {
		t.Fatalf("create asset_cloud_account: %v", err)
	}
	ids := seedCloudAccounts(t, db, []cloudAccountSeed{
		{name: "quiet", provider: "aliyun", accessKey: "AK-Q", secretKey: "SK-Q", regions: `[]`},
		{name: "moved", provider: "aliyun", accessKey: "AK-M", secretKey: "SK-M", regions: `[]`,
			updatedAt: time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)},
	})
	if _, err := inventory.RunCloudAccountBackfill(context.Background(), db); err != nil {
		t.Fatalf("first run: %v", err)
	}

	if err := db.Table("asset_cloud_account").Where("id = ?", ids[1]).
		Updates(map[string]any{"name": "moved-renamed", "updated_at": time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC)}).Error; err != nil {
		t.Fatalf("touch source row: %v", err)
	}
	report, err := inventory.RunCloudAccountBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if report.Updated != 1 || report.Unchanged != 1 {
		t.Errorf("second run = %+v, want updated=1 unchanged=1", report)
	}
	var name string
	if err := db.Model(&model.ProviderConnection{}).Where("source_model = ? AND source_id = ?", "asset_cloud_account", ids[1]).
		Select("name").Scan(&name).Error; err != nil {
		t.Fatal(err)
	}
	if name != "moved-renamed" {
		t.Errorf("changed row not propagated: connection name = %q", name)
	}
}

// TestCloudBackfillRotationAndEdgeCases raises the branch coverage of the
// propagation: non-normalizable and credential-less sources are skipped, an
// envelope-sourced v1 column is decrypted before re-sealing, and a finops
// secret rotation between runs re-points the single billing binding at a
// dedicated SecretRef (§7.4 — different material never shares).
func TestCloudBackfillRotationAndEdgeCases(t *testing.T) {
	db := newInventoryDB(t)
	if err := db.Exec(assetCloudAccountDDL).Error; err != nil {
		t.Fatalf("create asset_cloud_account: %v", err)
	}
	if err := db.Exec(integrationFinopsAccountDDL).Error; err != nil {
		t.Fatalf("create integration_finops_account: %v", err)
	}

	// An envelope-sourced access key (a v2-written cell) must decrypt on read.
	envelopeAK, err := util.EncryptSecretV2("AK-ENVELOPE")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(assetCloudAccountDDL).Error; err != nil {
		t.Fatal(err)
	}
	ids := seedCloudAccounts(t, db, []cloudAccountSeed{
		{name: "env-ak", provider: "tencent", accessKey: envelopeAK, secretKey: "SK-ENV", regions: `[]`},
		{name: "unknown-provider", provider: "custom", accessKey: "AK-X", secretKey: "SK-X", regions: `[]`},
		{name: "no-credential", provider: "aliyun", accessKey: "", secretKey: "SK-Y", regions: `[]`},
	})
	finopsIDs := seedFinopsAccounts(t, db, []finopsSeed{
		{name: "finops-plain", provider: "tencentcloud", accessKey: "AK-ENVELOPE", secretKey: "SK-ENV"},
	})

	report, err := inventory.RunCloudAccountBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if report.Skipped != 2 {
		t.Errorf("skipped = %d, want 2 (custom provider + empty accessKey)", report.Skipped)
	}
	chain, ok := loadCloudChain(t, db, "asset_cloud_account", ids[0])
	if !ok {
		t.Fatal("envelope-ak chain missing")
	}
	plain, err := util.DecryptSecretV2(chain.Ref.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	var blob map[string]string
	if err := json.Unmarshal([]byte(plain), &blob); err != nil {
		t.Fatal(err)
	}
	if blob["accessKey"] != "AK-ENVELOPE" {
		t.Errorf("envelope source not decrypted into the blob: %v", blob)
	}
	if finopsLink(t, db, finopsIDs[0]) != chain.Connection.UID {
		t.Fatalf("finops link = %q, want the shared connection", finopsLink(t, db, finopsIDs[0]))
	}

	// Rotate the finops v1 secret between runs: the single billing binding
	// must re-point from the shared inventory SecretRef at a dedicated one.
	if err := db.Table("integration_finops_account").Where("id = ?", finopsIDs[0]).
		Update("secret_key", "SK-ENV-ROTATED").Error; err != nil {
		t.Fatal(err)
	}
	second, err := inventory.RunCloudAccountBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	if finopsLink(t, db, finopsIDs[0]) != chain.Connection.UID {
		t.Error("rotation must keep the connection link")
	}
	var billing model.ProviderCredentialBinding
	if err := db.Where("provider_connection_id = ? AND purpose = ?", chain.Connection.ID, "billing").First(&billing).Error; err != nil {
		t.Fatalf("billing binding missing after rotation: %v", err)
	}
	if billing.SecretRefID == chain.Ref.ID {
		t.Error("rotated billing binding must not point at the shared inventory SecretRef (§7.4)")
	}
	_ = second

	// The finops-only chain: a provider+key that matches nothing gets its own
	// chain; deleting the source row marks it stale on the next run.
	seedFinopsAccounts(t, db, []finopsSeed{
		{name: "finops-lone", provider: "tencent", accessKey: "AK-LONE", secretKey: "SK-LONE"},
	})
	if _, err := inventory.RunCloudAccountBackfill(context.Background(), db); err != nil {
		t.Fatalf("run 3: %v", err)
	}
	var lone v1model.IntegrationFinOpsAccount
	if err := db.Where("name = ?", "finops-lone").First(&lone).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", lone.ID).Delete(&v1model.IntegrationFinOpsAccount{}).Error; err != nil {
		t.Fatal(err)
	}
	third, err := inventory.RunCloudAccountBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("run 4: %v", err)
	}
	if third.MarkedStale < 1 {
		t.Errorf("markedStale = %d, want ≥1 (deleted finops source)", third.MarkedStale)
	}
}

// TestCloudBackfillSourceRotationRefreshesRefs — the update path: rotating a
// cloud account's v1 secret (and moving its updated_at past the checkpoint)
// refreshes the sealed SecretRef material; a broken finops envelope fails its
// row in isolation; re-sealing a rotated billing SecretRef updates it in
// place.
func TestCloudBackfillSourceRotationRefreshesRefs(t *testing.T) {
	db := newInventoryDB(t)
	if err := db.Exec(assetCloudAccountDDL).Error; err != nil {
		t.Fatalf("create asset_cloud_account: %v", err)
	}
	if err := db.Exec(integrationFinopsAccountDDL).Error; err != nil {
		t.Fatalf("create integration_finops_account: %v", err)
	}
	ids := seedCloudAccounts(t, db, []cloudAccountSeed{
		{name: "acc", provider: "aliyun", accessKey: "AK-1", secretKey: "SK-1", regions: `[]`},
	})
	finopsIDs := seedFinopsAccounts(t, db, []finopsSeed{
		{name: "finops-match", provider: "aliyun", accessKey: "AK-1", secretKey: "SK-1"},
		{name: "finops-broken", provider: "aliyun", accessKey: "v2:missing-key:zzzz", secretKey: "SK-B"},
	})

	if _, err := inventory.RunCloudAccountBackfill(context.Background(), db); err != nil {
		t.Fatalf("run 1: %v", err)
	}
	chain, _ := loadCloudChain(t, db, "asset_cloud_account", ids[0])
	oldCiphertext := chain.Ref.Ciphertext

	// Rotate the source secret and move the checkpoint.
	if err := db.Table("asset_cloud_account").Where("id = ?", ids[0]).Updates(map[string]any{
		"secret_key": "SK-1-ROTATED",
		"updated_at": time.Date(2026, 9, 4, 8, 0, 0, 0, time.UTC),
	}).Error; err != nil {
		t.Fatal(err)
	}
	report, err := inventory.RunCloudAccountBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	if report.Updated != 1 {
		t.Errorf("updated = %d, want 1", report.Updated)
	}
	refreshed, _ := loadCloudChain(t, db, "asset_cloud_account", ids[0])
	if refreshed.Ref.Ciphertext == oldCiphertext {
		t.Error("rotated source secret must refresh the sealed SecretRef material")
	}
	plain, err := util.DecryptSecretV2(refreshed.Ref.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	var blob map[string]string
	if err := json.Unmarshal([]byte(plain), &blob); err != nil {
		t.Fatal(err)
	}
	if blob["secretKey"] != "SK-1-ROTATED" {
		t.Errorf("sealed blob carries stale secretKey: %v", blob)
	}
	var binding model.ProviderCredentialBinding
	if err := db.Where("provider_connection_id = ? AND purpose = ?", refreshed.Connection.ID, "billing").First(&binding).Error; err != nil {
		t.Fatalf("billing binding missing: %v", err)
	}

	// Rotate the finops secret again: the dedicated billing SecretRef is
	// re-sealed in place (same deterministic uid).
	if err := db.Table("integration_finops_account").Where("id = ?", finopsIDs[0]).
		Update("secret_key", "SK-1-ROTATED-AGAIN").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := inventory.RunCloudAccountBackfill(context.Background(), db); err != nil {
		t.Fatalf("run 3: %v", err)
	}
	var reloaded model.SecretRef
	if err := db.First(&reloaded, binding.SecretRefID).Error; err != nil {
		t.Fatal(err)
	}
	billingPlain, err := util.DecryptSecretV2(reloaded.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	var billingBlob map[string]string
	if err := json.Unmarshal([]byte(billingPlain), &billingBlob); err != nil {
		t.Fatal(err)
	}
	if billingBlob["secretKey"] != "SK-1-ROTATED-AGAIN" {
		t.Errorf("billing SecretRef not re-sealed: %v", billingBlob)
	}
}

// TestCloudBackfillEnvelopeComponentsAndSkipPaths — envelope-sourced
// secret_key/billing_token columns decrypt before re-sealing, and finops rows
// outside the collapse vocabulary (custom provider, empty accessKey) are
// skipped without touching the pipeline.
func TestCloudBackfillEnvelopeComponentsAndSkipPaths(t *testing.T) {
	db := newInventoryDB(t)
	if err := db.Exec(assetCloudAccountDDL).Error; err != nil {
		t.Fatalf("create asset_cloud_account: %v", err)
	}
	if err := db.Exec(integrationFinopsAccountDDL).Error; err != nil {
		t.Fatalf("create integration_finops_account: %v", err)
	}
	envelopeSK, err := util.EncryptSecretV2("SK-ENV-SK")
	if err != nil {
		t.Fatal(err)
	}
	envelopeToken, err := util.EncryptSecretV2("TOK-ENV")
	if err != nil {
		t.Fatal(err)
	}
	seedCloudAccounts(t, db, []cloudAccountSeed{
		{name: "acc-env", provider: "alicloud", accessKey: "AK-E", secretKey: envelopeSK, regions: `[]`},
	})
	seedFinopsAccounts(t, db, []finopsSeed{
		{name: "finops-env", provider: "alicloud", accessKey: "AK-E", secretKey: "SK-E", billingToken: envelopeToken},
		{name: "finops-custom", provider: "custom", accessKey: "AK-C", secretKey: "SK-C"},
		{name: "finops-empty", provider: "alicloud", accessKey: "", secretKey: "SK-Z"},
	})

	report, err := inventory.RunCloudAccountBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if report.Skipped != 2 {
		t.Errorf("skipped = %d, want 2 (custom provider + empty accessKey)", report.Skipped)
	}
	var chain cloudChainOf
	var ok bool
	if chain, ok = loadCloudChain(t, db, "asset_cloud_account", 1); !ok {
		t.Fatal("chain missing")
	}
	plain, err := util.DecryptSecretV2(chain.Ref.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	var blob map[string]string
	if err := json.Unmarshal([]byte(plain), &blob); err != nil {
		t.Fatal(err)
	}
	if blob["secretKey"] != "SK-ENV-SK" {
		t.Errorf("envelope secret_key not decrypted into the blob: %v", blob)
	}

	// The idempotent re-run walks every branch again without writes.
	if _, err := inventory.RunCloudAccountBackfill(context.Background(), db); err != nil {
		t.Fatalf("run 2: %v", err)
	}
}

// TestCloudBackfillIsolatesFinopsLinkFailure — J6 / ④review MEDIUM-1: a
// failed §5.4 finops link write is isolated per account (Failed++ +
// FailedSources, continue) — the pipeline does not error, the remaining
// accounts keep propagating, and the report survives.
func TestCloudBackfillIsolatesFinopsLinkFailure(t *testing.T) {
	db := newInventoryDB(t)
	if err := db.Exec(assetCloudAccountDDL).Error; err != nil {
		t.Fatalf("create asset_cloud_account: %v", err)
	}
	if err := db.Exec(integrationFinopsAccountDDL).Error; err != nil {
		t.Fatalf("create integration_finops_account: %v", err)
	}
	seedCloudAccounts(t, db, []cloudAccountSeed{
		{name: "cloud-1", provider: "aliyun", accessKey: "AK1", secretKey: "SK1", regions: `["cn-hangzhou"]`},
	})
	finopsIDs := seedFinopsAccounts(t, db, []finopsSeed{
		{name: "finops-ok", provider: "aliyun", accessKey: "AK1", secretKey: "SK1"},
		{name: "finops-broken", provider: "aliyun", accessKey: "AK9", secretKey: "SK9"},
	})

	// The broken account's link write fails: a sqlite trigger aborts only the
	// provider_connection_uid UPDATE of that row (reads stay healthy, so the
	// failure is link-write scoped, not source-read scoped).
	trigger := fmt.Sprintf(`CREATE TRIGGER fail_finops_link_%d BEFORE UPDATE ON integration_finops_account
		WHEN NEW.id = %d
		BEGIN SELECT RAISE(ABORT, 'link rejected'); END`, finopsIDs[1], finopsIDs[1])
	if err := db.Exec(trigger).Error; err != nil {
		t.Fatalf("create link-failure trigger: %v", err)
	}

	report, err := inventory.RunCloudAccountBackfill(context.Background(), db)
	if err != nil {
		t.Fatalf("link failure must not halt the pipeline: %v", err)
	}
	if report.Failed != 1 {
		t.Fatalf("report.Failed = %d, want 1 (the broken link only)", report.Failed)
	}
	want := fmt.Sprintf("integration_finops_account:%d", finopsIDs[1])
	if len(report.FailedSources) != 1 || report.FailedSources[0] != want {
		t.Fatalf("report.FailedSources = %v, want [%s]", report.FailedSources, want)
	}
	// The healthy finops account still collapsed onto the cloud chain and got
	// its link backfilled.
	if link := finopsLink(t, db, finopsIDs[0]); link == "" {
		t.Fatalf("healthy finops link was not backfilled")
	}
	if report.FinopsLinked != 1 {
		t.Fatalf("report.FinopsLinked = %d, want 1", report.FinopsLinked)
	}
}
