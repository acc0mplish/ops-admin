package main

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/util"
)

// inventoryScanKeys pins the secret key state for the scan tests.
func inventoryScanKeys(t *testing.T) {
	t.Helper()
	t.Setenv("OPS_SECRET_MASTER_KEYS", "")
	if err := util.ConfigureSecretMasterKeys(""); err != nil {
		t.Fatal(err)
	}
	util.ConfigureCredentialKey("inventory-scan-credential-seed")
	t.Cleanup(func() {
		util.ConfigureCredentialKey("")
		_ = util.ConfigureSecretMasterKeys("")
	})
}

// newInventoryScanDB opens a single-connection in-memory sqlite database with
// hand-written minimal tables, mirroring how the command queries the registry
// strings directly instead of importing models.
func newInventoryScanDB(t *testing.T, statements ...string) *gorm.DB {
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
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestRenderSecretInventoryText(t *testing.T) {
	report := secretInventoryReport{
		Fields: []secretInventoryField{
			{Model: "SSLCertificate", Table: "ssl_certificates", Column: "private_key_cipher", Class: "E-legacy", Counts: util.FieldCounts{Total: 2, V2: 1, Legacy: 1}},
		},
		Unknowns: []secretInventoryUnknown{{Table: "ssl_certificates", ID: 7, Field: "private_key_cipher", Length: 12}},
		Total:    util.FieldCounts{Total: 2, V2: 1, Legacy: 1},
		Note:     inventoryUnknownNote,
	}
	out := renderSecretInventoryText(report)
	for _, want := range []string{"SSLCertificate.private_key_cipher", "E-legacy", "TOTAL", "expected pre-migration state", "ssl_certificates id=7 field=private_key_cipher length=12"} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered report is missing %q:\n%s", want, out)
		}
	}
}

func TestRunSecretInventoryRejectsBadInput(t *testing.T) {
	if code := runSecretInventory([]string{"--undefined-flag"}); code != 1 {
		t.Fatalf("undefined flag must exit 1, got %d", code)
	}
	if code := runSecretInventory([]string{"--config", "/nonexistent/config.yaml"}); code != 1 {
		t.Fatalf("missing config must exit 1, got %d", code)
	}
}

func TestScanSecretColumnClassifiesRows(t *testing.T) {
	inventoryScanKeys(t)
	db := newInventoryScanDB(t, "CREATE TABLE ssl_certificates (id INTEGER PRIMARY KEY, private_key_cipher TEXT)")
	v2Value, err := util.EncryptSecretV2("v2-secret")
	if err != nil {
		t.Fatal(err)
	}
	legacyValue, err := util.EncryptSecret("legacy-secret")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{v2Value, legacyValue, "plaintext-secret"} {
		if err := db.Exec("INSERT INTO ssl_certificates (private_key_cipher) VALUES (?)", value).Error; err != nil {
			t.Fatal(err)
		}
	}
	field, ok := util.LookupSecretField("ssl_certificates", "private_key_cipher")
	if !ok {
		t.Fatal("registry lookup failed")
	}
	counts, unknowns, missing, err := scanSecretField(db, field)
	if err != nil || missing {
		t.Fatalf("scan failed: missing=%v err=%v", missing, err)
	}
	want := util.FieldCounts{Total: 3, V2: 1, Legacy: 1, Unknown: 1}
	if counts != want {
		t.Fatalf("counts = %+v, want %+v", counts, want)
	}
	// The plaintext value in an E-class column is UNKNOWN and is reported.
	if len(unknowns) != 1 || unknowns[0].ID != 3 || unknowns[0].Field != "private_key_cipher" {
		t.Fatalf("unknowns = %+v", unknowns)
	}
}

func TestScanScheduleTaskVariablesGate(t *testing.T) {
	inventoryScanKeys(t)
	db := newInventoryScanDB(t,
		"CREATE TABLE ops_script (id INTEGER PRIMARY KEY, variables TEXT)",
		"CREATE TABLE ops_schedule_task (id INTEGER PRIMARY KEY, script_id INTEGER, variables TEXT)",
	)
	v2Value, err := util.EncryptSecretV2("token-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ops_script (variables) VALUES (?)", `[{"name":"TOKEN","secret":true},{"name":"ENV","secret":false}]`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ops_schedule_task (script_id, variables) VALUES (1, ?)", `{"TOKEN":"`+v2Value+`","ENV":"plain"}`).Error; err != nil {
		t.Fatal(err)
	}
	field, ok := util.LookupSecretField("ops_schedule_task", "variables")
	if !ok {
		t.Fatal("registry lookup failed")
	}
	counts, unknowns, missing, err := scanSecretField(db, field)
	if err != nil || missing {
		t.Fatalf("scan failed: missing=%v err=%v", missing, err)
	}
	// The declared secret classifies as V2; the undeclared value is
	// NOT_SECRET-by-declaration, never PLAINTEXT or UNKNOWN.
	want := util.FieldCounts{Total: 2, V2: 1, NotSecret: 1}
	if counts != want {
		t.Fatalf("counts = %+v, want %+v", counts, want)
	}
	if len(unknowns) != 0 {
		t.Fatalf("no unknowns expected, got %+v", unknowns)
	}
}

// TestScanSecretColumnMissingColumnIsReported pins the schema-drift behavior:
// a registered column absent from the physical table measures as zero counts
// with missing=true instead of failing the whole inventory.
func TestScanSecretColumnMissingColumnIsReported(t *testing.T) {
	inventoryScanKeys(t)
	db := newInventoryScanDB(t, "CREATE TABLE asset_gateway (id INTEGER PRIMARY KEY, name TEXT)")
	field, ok := util.LookupSecretField("asset_gateway", "password")
	if !ok {
		t.Fatal("registry lookup failed")
	}
	counts, unknowns, missing, err := scanSecretField(db, field)
	if err != nil {
		t.Fatal(err)
	}
	if !missing {
		t.Fatal("a missing column must be reported as missing")
	}
	if counts != (util.FieldCounts{}) || len(unknowns) != 0 {
		t.Fatalf("missing column must measure zero: %+v %+v", counts, unknowns)
	}
}
