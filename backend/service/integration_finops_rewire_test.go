// FinOps rewire tests (plan phase4 r2 §2 M4 — C2, 판정 J5): the linked path
// resolves billing material through the V2 secrets broker into a LOCAL COPY
// only, the trailing save is a narrow (last_sync_at, next_sync_at) UPDATE
// that can never carry the credential columns (claim 16), credential rotation
// through the v1 save path unlinks (provider_connection_uid=NULL), and the
// cloud-account save path unlinks its backfilled inventory binding.
package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/migrate"
	inframodel "ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/testutil"
	"ops-admin/backend/model"
	"ops-admin/backend/util"
)

func newFinopsRewireDB(t *testing.T) *gorm.DB {
	t.Helper()
	testutil.PinSecretKeys(t)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}
	if err := db.AutoMigrate(&model.IntegrationFinOpsAccount{}, &model.IntegrationFinOpsCostRecord{}, &model.IntegrationFinOpsSyncLog{}, &model.AssetCloudAccount{}); err != nil {
		t.Fatalf("automigrate v1 finops tables: %v", err)
	}
	return db
}

// seedBillingChain writes connection → secret_ref (sealed JSON credential
// blob) → billing binding, and returns the connection uid.
func seedBillingChain(t *testing.T, db *gorm.DB, uid string, accessKey, secretKey, billingToken string) string {
	t.Helper()
	blob, err := json.Marshal(map[string]string{"accessKey": accessKey, "secretKey": secretKey, "billingToken": billingToken})
	if err != nil {
		t.Fatalf("marshal credential blob: %v", err)
	}
	envelope, err := util.EncryptSecretV2(string(blob))
	if err != nil {
		t.Fatalf("seal credential blob: %v", err)
	}
	conn := inframodel.ProviderConnection{UID: uid, ProviderType: "aliyun", Name: "seed cloud", Endpoint: "", Status: "active"}
	if err := db.Create(&conn).Error; err != nil {
		t.Fatalf("seed connection: %v", err)
	}
	ref := inframodel.SecretRef{UID: uid + "-ref", Backend: "internal", Path: "test/" + uid, Ciphertext: envelope}
	if err := db.Create(&ref).Error; err != nil {
		t.Fatalf("seed secret_ref: %v", err)
	}
	binding := inframodel.ProviderCredentialBinding{ProviderConnectionID: conn.ID, Purpose: "billing", SecretRefID: ref.ID, Status: "active"}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatalf("seed billing binding: %v", err)
	}
	return uid
}

func seedFinopsAccount(t *testing.T, db *gorm.DB, row model.IntegrationFinOpsAccount) uint {
	t.Helper()
	row.Currency = "CNY"
	row.Status = 1
	if row.SyncFrequency == "" {
		row.SyncFrequency = "daily"
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed finops account: %v", err)
	}
	return row.ID
}

func rawFinopsCredentialColumns(t *testing.T, db *gorm.DB, id uint) (access, secret, token, link string) {
	t.Helper()
	var row struct {
		Access string
		Secret string
		Token  string
		Link   string
	}
	err := db.Raw("SELECT access_key AS access, secret_key AS secret, billing_token AS token, provider_connection_uid AS link FROM integration_finops_account WHERE id = ?", id).Scan(&row).Error
	if err != nil {
		t.Fatalf("reload credential columns: %v", err)
	}
	return row.Access, row.Secret, row.Token, row.Link
}

// billingCapture is the custom billing endpoint stand-in: it records the
// Authorization header of every request and answers an empty month.
func billingCapture(t *testing.T, seen *[]string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"records":[]}`))
	}))
	t.Cleanup(server.Close)
	return server
}

// TestSyncFinOpsAccountMonthsRewiresThroughBroker — the linked path resolves
// the billing purpose through the broker and injects the material into the
// local copy only: the provider request carries the broker token while the v1
// row's credential columns stay byte-identical (claim 16), and the narrow
// trailing update still stamps the sync schedule columns.
func TestSyncFinOpsAccountMonthsRewiresThroughBroker(t *testing.T) {
	db := newFinopsRewireDB(t)
	uid := seedBillingChain(t, db, "cloudconn0000000000000000000001", "AK-BROKER", "SK-BROKER", "TOK-BROKER")
	month := time.Now().Format("2006-01")
	var seen []string
	endpoint := billingCapture(t, &seen)

	id := seedFinopsAccount(t, db, model.IntegrationFinOpsAccount{
		Name: "linked", Provider: "custom", BillingEndpoint: endpoint.URL,
		// Stale v1 material — the link, not the row, must feed the request.
		AccessKey: "AK-ROW-OLD", SecretKey: "SK-ROW-OLD", BillingToken: "TOK-ROW-OLD",
		ProviderConnectionUID: uid,
	})

	result, err := (&Service{db: db}).SyncFinOpsAccountMonths(id, "manual", month, month)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(seen) != 1 || seen[0] != "Bearer TOK-BROKER" {
		t.Fatalf("billing request authorization = %v, want one Bearer TOK-BROKER (broker path)", seen)
	}
	if len(result.Months) != 1 || result.Months[0].Status != "success" {
		t.Fatalf("sync result = %+v, want one successful month", result)
	}
	access, secret, token, link := rawFinopsCredentialColumns(t, db, id)
	if access != "AK-ROW-OLD" || secret != "SK-ROW-OLD" || token != "TOK-ROW-OLD" {
		t.Errorf("credential columns rewritten: access=%q secret=%q token=%q — claim 16 violated", access, secret, token)
	}
	if link != uid {
		t.Errorf("link rewritten to %q, want %q", link, uid)
	}
	var lastSync *time.Time
	if err := db.Table("integration_finops_account").Where("id = ?", id).Select("last_sync_at").Scan(&lastSync).Error; err != nil {
		t.Fatal(err)
	}
	if lastSync == nil {
		t.Error("narrow trailing update must still stamp last_sync_at")
	}
}

// TestSyncFinOpsAccountMonthsUnlinkedFallback — a NULL link keeps the pre-
// rewire behavior: the row's own columns feed the request.
func TestSyncFinOpsAccountMonthsUnlinkedFallback(t *testing.T) {
	db := newFinopsRewireDB(t)
	month := time.Now().Format("2006-01")
	var seen []string
	endpoint := billingCapture(t, &seen)

	id := seedFinopsAccount(t, db, model.IntegrationFinOpsAccount{
		Name: "unlinked", Provider: "custom", BillingEndpoint: endpoint.URL,
		AccessKey: "AK-ROW", SecretKey: "SK-ROW", BillingToken: "TOK-ROW",
	})

	if _, err := (&Service{db: db}).SyncFinOpsAccountMonths(id, "manual", month, month); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(seen) != 1 || seen[0] != "Bearer TOK-ROW" {
		t.Fatalf("fallback authorization = %v, want Bearer TOK-ROW (row direct-read)", seen)
	}
}

// TestSyncFinOpsAccountMonthsBrokenLinkFailsClosed — a link whose chain is
// missing (or whose material is not the credential blob) fails the sync
// instead of silently falling back to possibly-stale v1 material.
func TestSyncFinOpsAccountMonthsBrokenLinkFailsClosed(t *testing.T) {
	db := newFinopsRewireDB(t)
	month := time.Now().Format("2006-01")
	id := seedFinopsAccount(t, db, model.IntegrationFinOpsAccount{
		Name: "dangling", Provider: "custom",
		BillingEndpoint: "http://127.0.0.1:1/none",
		AccessKey:       "AK-ROW", BillingToken: "TOK-ROW",
		ProviderConnectionUID: "no-such-connection-uid",
	})
	if _, err := (&Service{db: db}).SyncFinOpsAccountMonths(id, "manual", month, month); err == nil {
		t.Fatal("dangling link must fail the sync, not fall back")
	}
}

// TestSaveFinOpsAccountClearsLinkOnCredentialRotation — J5-3: rotating a
// credential field through the v1 save path nulls the link in the same
// transaction; a save that touches no credential field keeps it.
func TestSaveFinOpsAccountClearsLinkOnCredentialRotation(t *testing.T) {
	db := newFinopsRewireDB(t)
	svc := &Service{db: db}
	uid := seedBillingChain(t, db, "cloudconn0000000000000000000002", "AK", "SK", "")
	id := seedFinopsAccount(t, db, model.IntegrationFinOpsAccount{
		Name: "rotate-me", Provider: "alicloud", AccessKey: "AK-OLD", SecretKey: "SK-OLD",
		ProviderConnectionUID: uid,
	})

	// Name-only save keeps the link.
	if _, err := svc.SaveFinOpsAccount(FinOpsAccountPayload{ID: id, Name: "rotate-me", Provider: "alicloud", SyncEnabled: true, SyncFrequency: "daily"}); err != nil {
		t.Fatalf("name-only save: %v", err)
	}
	if _, _, _, link := rawFinopsCredentialColumns(t, db, id); link != uid {
		t.Fatalf("name-only save dropped the link: %q", link)
	}

	// Credential rotation clears it.
	if _, err := svc.SaveFinOpsAccount(FinOpsAccountPayload{ID: id, Name: "rotate-me", Provider: "alicloud", AccessKey: "AK-NEW", SecretKey: "SK-NEW", SyncEnabled: true, SyncFrequency: "daily"}); err != nil {
		t.Fatalf("rotation save: %v", err)
	}
	access, secret, _, link := rawFinopsCredentialColumns(t, db, id)
	if access != "AK-NEW" || secret != "SK-NEW" {
		t.Errorf("rotation not persisted: access=%q secret=%q", access, secret)
	}
	if link != "" {
		t.Errorf("rotated account link = %q, want NULL", link)
	}
}

// TestUpdateAssetCloudAccountUnlinksInventoryBinding — J5-3 symmetric hook:
// rotating the v1 cloud account secret deletes the backfilled inventory
// binding so no sync can read the stale material; untouched fields keep it.
func TestUpdateAssetCloudAccountUnlinksInventoryBinding(t *testing.T) {
	db := newFinopsRewireDB(t)
	svc := &Service{db: db}

	account := model.AssetCloudAccount{Name: "acc", Provider: "aliyun", AccessKey: "AK-OLD", SecretKey: "SK-OLD", Regions: []string{"cn-hangzhou"}, Status: 1}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("seed cloud account: %v", err)
	}
	envelope, err := util.EncryptSecretV2(`{"accessKey":"AK-OLD","secretKey":"SK-OLD"}`)
	if err != nil {
		t.Fatal(err)
	}
	conn := inframodel.ProviderConnection{UID: "cloudconn0000000000000000000003", ProviderType: "aliyun", Name: "acc", Status: "active", SourceModel: "asset_cloud_account", SourceID: account.ID}
	if err := db.Create(&conn).Error; err != nil {
		t.Fatalf("seed connection: %v", err)
	}
	ref := inframodel.SecretRef{UID: "cloudconn0000000000000000000003-ref", Backend: "internal", Ciphertext: envelope}
	if err := db.Create(&ref).Error; err != nil {
		t.Fatal(err)
	}
	binding := inframodel.ProviderCredentialBinding{ProviderConnectionID: conn.ID, Purpose: "inventory", SecretRefID: ref.ID, Status: "active"}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatal(err)
	}

	bindingCount := func() int64 {
		t.Helper()
		var n int64
		db.Model(&inframodel.ProviderCredentialBinding{}).Where("id = ?", binding.ID).Count(&n)
		return n
	}

	// Region-only update keeps the binding.
	if err := svc.UpdateAssetCloudAccount(AssetCloudAccountPayload{ID: account.ID, Name: "acc", Provider: "aliyun", AccessKey: "AK-OLD", SecretKey: "", Regions: []string{"cn-hangzhou", "cn-shanghai"}, Status: 1}); err != nil {
		t.Fatalf("region-only update: %v", err)
	}
	if bindingCount() != 1 {
		t.Fatal("region-only update must keep the inventory binding")
	}

	// Secret rotation removes it.
	if err := svc.UpdateAssetCloudAccount(AssetCloudAccountPayload{ID: account.ID, Name: "acc", Provider: "aliyun", AccessKey: "AK-OLD", SecretKey: "SK-ROTATED", Regions: []string{"cn-hangzhou"}, Status: 1}); err != nil {
		t.Fatalf("rotation update: %v", err)
	}
	if bindingCount() != 0 {
		t.Error("credential rotation must delete the inventory binding")
	}
}
