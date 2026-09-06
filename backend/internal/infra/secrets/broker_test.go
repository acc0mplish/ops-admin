package secrets

import (
	"context"
	"strings"
	"testing"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/testutil"
	"ops-admin/backend/util"
)

// seedChain is the seeder: connection → secret_ref → binding.
func seedChain(t *testing.T, db *gorm.DB, connectionUID, purpose, ciphertext string) uint {
	t.Helper()
	conn := model.ProviderConnection{UID: connectionUID, ProviderType: "proxmox", Name: "seed", Endpoint: "https://seed:8006"}
	if err := db.Create(&conn).Error; err != nil {
		t.Fatalf("seed connection: %v", err)
	}
	ref := model.SecretRef{UID: "sec-" + connectionUID, Backend: "internal", Ciphertext: ciphertext}
	if err := db.Create(&ref).Error; err != nil {
		t.Fatalf("seed secret_ref: %v", err)
	}
	binding := model.ProviderCredentialBinding{
		ProviderConnectionID: conn.ID,
		Purpose:              purpose,
		SecretRefID:          ref.ID,
	}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatalf("seed binding: %v", err)
	}
	return conn.ID
}

// §7.4 purpose vocabulary — the closed M1 set the broker gates on (§3.5).
func TestCredentialPurposeVocabulary(t *testing.T) {
	if len(CredentialPurposes) != 6 {
		t.Errorf("CredentialPurposes length = %d, want 6 (§7.4)", len(CredentialPurposes))
	}
	for _, p := range []string{"inventory", "operations", "billing", "console", "monitoring", "backup"} {
		if !knownCredentialPurpose(p) {
			t.Errorf("knownCredentialPurpose(%q) = false, want true", p)
		}
	}
	if knownCredentialPurpose("operationsX") {
		t.Error("knownCredentialPurpose accepted a value outside the §7.4 set")
	}
}

// T21 — TestBrokerResolvesPurposeScoped: a matching binding decrypts and the
// plaintext round-trips through the pinned key set. Two calls must each go to
// the store (no caching, §1 J7 ②).
func TestBrokerResolvesPurposeScoped(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	if err := db.AutoMigrate(&model.ProviderConnection{}, &model.ProviderCredentialBinding{}, &model.SecretRef{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	const plain = "root-password-material"
	sealed, err := util.EncryptSecretV2(plain)
	if err != nil {
		t.Fatalf("EncryptSecretV2: %v", err)
	}
	seedChain(t, db, "conn-1", "operations", sealed)

	broker := NewBroker(db)
	got, err := broker.Resolve(context.Background(), "conn-1", "operations")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Value != plain {
		t.Errorf("Resolve value = %q, want the sealed plaintext round-trip", got.Value)
	}
	if got.Purpose != "operations" {
		t.Errorf("Resolve purpose = %q, want operations", got.Purpose)
	}
	if got.SecretRefUID != "sec-conn-1" {
		t.Errorf("Resolve secretRefUID = %q, want sec-conn-1", got.SecretRefUID)
	}

	// Non-caching (J7 ②): rotate the ciphertext under a new envelope and the
	// next call must observe the new material, not a remembered view.
	sealed2, err := util.EncryptSecretV2("rotated-material")
	if err != nil {
		t.Fatalf("EncryptSecretV2(2): %v", err)
	}
	if err := db.Model(&model.SecretRef{}).Where("uid = ?", "sec-conn-1").Update("ciphertext", sealed2).Error; err != nil {
		t.Fatalf("rotate ciphertext: %v", err)
	}
	got2, err := broker.Resolve(context.Background(), "conn-1", "operations")
	if err != nil {
		t.Fatalf("Resolve after rotate: %v", err)
	}
	if got2.Value != "rotated-material" {
		t.Errorf("Resolve after rotate returned stale material — broker caches (J7 ②)")
	}
}

// T22 — TestBrokerRejectsPurposeMismatch: a binding whose purpose differs from
// the requested one is a hard error with no fallback (§1 J7 ①).
func TestBrokerRejectsPurposeMismatch(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	if err := db.AutoMigrate(&model.ProviderConnection{}, &model.ProviderCredentialBinding{}, &model.SecretRef{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	const plain = "inventory-only-material"
	sealed, err := util.EncryptSecretV2(plain)
	if err != nil {
		t.Fatalf("EncryptSecretV2: %v", err)
	}
	seedChain(t, db, "conn-2", "inventory", sealed)

	broker := NewBroker(db)
	_, err = broker.Resolve(context.Background(), "conn-2", "operations")
	if err == nil {
		t.Fatal("purpose mismatch resolved — forged-purpose access was not blocked")
	}
	if strings.Contains(err.Error(), plain) {
		t.Errorf("error message leaks plaintext material: %v", err)
	}
}

// The remaining Resolve error paths — an out-of-vocabulary purpose is a hard
// error before any query (§7.4 closed set), and unknown connection, missing
// binding, and a dangling secret_ref reference all resolve to errors rather
// than empty material.
func TestBrokerRejectsUnresolvableChains(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	if err := db.AutoMigrate(&model.ProviderConnection{}, &model.ProviderCredentialBinding{}, &model.SecretRef{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	sealed, err := util.EncryptSecretV2("material")
	if err != nil {
		t.Fatalf("EncryptSecretV2: %v", err)
	}
	seedChain(t, db, "conn-3", "inventory", sealed)
	// A binding whose secret_ref row does not exist.
	var conn model.ProviderConnection
	if err := db.Where("uid = ?", "conn-3").First(&conn).Error; err != nil {
		t.Fatalf("reload connection: %v", err)
	}
	if err := db.Create(&model.ProviderCredentialBinding{ProviderConnectionID: conn.ID, Purpose: "operations", SecretRefID: 99999}).Error; err != nil {
		t.Fatalf("seed dangling binding: %v", err)
	}

	broker := NewBroker(db)
	cases := []struct {
		name          string
		connectionUID string
		purpose       string
	}{
		{"out-of-vocabulary purpose", "conn-3", "forged"},
		{"unknown connection", "conn-missing", "operations"},
		{"no binding for purpose", "conn-3", "billing"},
		{"dangling secret_ref", "conn-3", "operations"},
	}
	for _, tc := range cases {
		got, err := broker.Resolve(context.Background(), tc.connectionUID, tc.purpose)
		if err == nil {
			t.Errorf("%s: Resolve succeeded (%+v) — hard error expected", tc.name, got)
		}
		if got.Value != "" {
			t.Errorf("%s: returned material on failure — must return zero view", tc.name)
		}
	}
}

// T23 — TestBrokerRejectsNonV2Ciphertext: legacy and plaintext shapes must be
// hard errors — the broker reads through DecryptSecretV2 only (§4.3, §1 J7 ③).
func TestBrokerRejectsNonV2Ciphertext(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t)
	if err := db.AutoMigrate(&model.ProviderConnection{}, &model.ProviderCredentialBinding{}, &model.SecretRef{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	broker := NewBroker(db)
	for name, ciphertext := range map[string]string{
		"plaintext":  "raw-root-password",
		"empty":      "",
		"legacy":     "legacy-ciphertext-base64==",
		"unknownkey": "v2:not-a-known-key:AAAA",
	} {
		seedChain(t, db, "conn-"+name, "operations", ciphertext)
		_, err := broker.Resolve(context.Background(), "conn-"+name, "operations")
		if err == nil {
			t.Errorf("%s ciphertext resolved — non-v2 envelope must be a hard error", name)
		}
	}
}
