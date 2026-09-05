package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/model"
	"ops-admin/backend/util"
)

// roundtripKeys pins the secret key state for the round-trip tests: implicit
// master key set, explicit credential seed, both restored afterwards.
func roundtripKeys(t *testing.T) {
	t.Helper()
	t.Setenv("OPS_SECRET_MASTER_KEYS", "")
	if err := util.ConfigureSecretMasterKeys(""); err != nil {
		t.Fatal(err)
	}
	util.ConfigureCredentialKey("roundtrip-credential-seed")
	t.Cleanup(func() {
		util.ConfigureCredentialKey("")
		_ = util.ConfigureSecretMasterKeys("")
	})
}

// newRoundtripDB opens a single-connection in-memory sqlite database and
// migrates only the models the exercised path touches (test-only dependency).
func newRoundtripDB(t *testing.T, models ...any) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}
	return db
}

// TestSavePublicDNSAccountWritesV2EnvelopeAndReadsBack drives the real DNS
// account save path into sqlite and verifies both sides of the dual-key
// reader: new writes carry the v2 envelope, pre-migration legacy envelopes
// stay readable, both restore their plaintext on the publicProvider path.
func TestSavePublicDNSAccountWritesV2EnvelopeAndReadsBack(t *testing.T) {
	roundtripKeys(t)
	db := newRoundtripDB(t, &model.PublicDNSAccount{}, &model.DNSAuditLog{})
	svc := &Service{db: db}
	if err := svc.SavePublicDNSAccount(PublicDNSAccountPayload{Name: "primary", Provider: "aliyun", AccessKey: "LTAI-access-key", SecretKey: "access-secret"}, DNSAuditActor{}); err != nil {
		t.Fatal(err)
	}
	var row model.PublicDNSAccount
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(row.AccessKeyCipher, "v2:") || !strings.HasPrefix(row.SecretKeyCipher, "v2:") {
		t.Fatalf("new writes must use the v2 envelope: %q %q", row.AccessKeyCipher, row.SecretKeyCipher)
	}
	if _, _, err := svc.publicProvider(row.ID); err != nil {
		t.Fatalf("runtime reader must accept the v2 envelope: %v", err)
	}
	accessField, ok := util.LookupSecretField("domain_public_dns_account", "access_key_cipher")
	if !ok {
		t.Fatal("registry lookup failed for access_key_cipher")
	}
	secretField, ok := util.LookupSecretField("domain_public_dns_account", "secret_key_cipher")
	if !ok {
		t.Fatal("registry lookup failed for secret_key_cipher")
	}
	if plain, err := util.ReadSecretField(row.AccessKeyCipher, accessField, false); err != nil || plain != "LTAI-access-key" {
		t.Fatalf("v2 access key not restored: %q %v", plain, err)
	}
	if plain, err := util.ReadSecretField(row.SecretKeyCipher, secretField, false); err != nil || plain != "access-secret" {
		t.Fatalf("v2 secret key not restored: %q %v", plain, err)
	}
	legacyAccess, err := util.EncryptSecret("legacy-access-key")
	if err != nil {
		t.Fatal(err)
	}
	legacySecret, err := util.EncryptSecret("legacy-secret-key")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.PublicDNSAccount{}).Where("id = ?", row.ID).Updates(map[string]any{"access_key_cipher": legacyAccess, "secret_key_cipher": legacySecret}).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.publicProvider(row.ID); err != nil {
		t.Fatalf("runtime reader must accept legacy envelopes: %v", err)
	}
	var legacyRow model.PublicDNSAccount
	if err := db.First(&legacyRow, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if plain, err := util.ReadSecretField(legacyRow.AccessKeyCipher, accessField, false); err != nil || plain != "legacy-access-key" {
		t.Fatalf("legacy access key not restored: %q %v", plain, err)
	}
	if plain, err := util.ReadSecretField(legacyRow.SecretKeyCipher, secretField, false); err != nil || plain != "legacy-secret-key" {
		t.Fatalf("legacy secret key not restored: %q %v", plain, err)
	}
}

// generateSelfSignedPair mints a valid certificate and matching EC private
// key so the manual upload path can run without external material.
func generateSelfSignedPair(t *testing.T, domain string) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: domain},
		DNSNames:     []string{domain},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(30 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	return certPEM, keyPEM
}

// TestManualCertificatePrivateKeyRoundTrip drives the manual certificate
// upload into sqlite and byte-compares the downloaded private key with the
// uploaded PEM through the download read path.
func TestManualCertificatePrivateKeyRoundTrip(t *testing.T) {
	roundtripKeys(t)
	db := newRoundtripDB(t, &model.SSLCertificate{}, &model.SSLCertificateDomain{}, &model.PublicDomainSnapshot{}, &model.SSLCertificateAuditLog{})
	if err := db.Create(&model.PublicDomainSnapshot{AccountID: 1, Provider: "aliyun", Domain: "example.com", Status: "success", SyncedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	certPEM, keyPEM := generateSelfSignedPair(t, "demo.example.com")
	svc := &Service{db: db}
	id, err := svc.UploadSSLCertificate(SSLCertificateUploadPayload{Name: "manual", MainDomain: "demo.example.com", CertificatePEM: certPEM, PrivateKeyPEM: keyPEM}, DNSAuditActor{})
	if err != nil {
		t.Fatal(err)
	}
	var cert model.SSLCertificate
	if err := db.First(&cert, id).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cert.PrivateKeyCipher, "v2:") {
		t.Fatalf("manual private key must be stored as v2: %q", cert.PrivateKeyCipher)
	}
	downloaded, _, _, err := svc.DownloadSSLCertificate(id, "private-key")
	if err != nil {
		t.Fatal(err)
	}
	if string(downloaded) != strings.TrimSpace(keyPEM) {
		t.Fatal("downloaded private key must match the uploaded bytes")
	}
}
