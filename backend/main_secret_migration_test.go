package main

import (
	"strings"
	"testing"

	"ops-admin/backend/util"
)

// TestReencryptMigratesLegacyToV2 covers the Step 2 happy path: E-class legacy
// values are re-written as v2 envelopes, verified byte-for-byte against the
// in-memory plaintext, checkpointed, and skipped entirely on a re-run.
func TestReencryptMigratesLegacyToV2(t *testing.T) {
	inventoryScanKeys(t)
	db := newInventoryScanDB(t, "CREATE TABLE ssl_certificates (id INTEGER PRIMARY KEY, private_key_cipher TEXT)")
	legacy, err := util.EncryptSecret("legacy-private-key")
	if err != nil {
		t.Fatal(err)
	}
	v2Value, err := util.EncryptSecretV2("already-migrated")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{legacy, v2Value, ""} {
		if err := db.Exec("INSERT INTO ssl_certificates (private_key_cipher) VALUES (?)", value).Error; err != nil {
			t.Fatal(err)
		}
	}
	report, err := reencryptSecretsInDB(db, secretMigrationOptions{BackupAcknowledged: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Migrated != 1 {
		t.Fatalf("exactly the legacy row must migrate, got %d", report.Migrated)
	}
	var migrated string
	if err := db.Raw("SELECT private_key_cipher FROM ssl_certificates WHERE id = 1").Scan(&migrated).Error; err != nil {
		t.Fatal(err)
	}
	plain, err := util.DecryptSecretV2(migrated)
	if err != nil || plain != "legacy-private-key" {
		t.Fatalf("migrated row does not round trip as v2: %q %v", plain, err)
	}
	// The untouched rows keep their stored shape.
	var untouched string
	if err := db.Raw("SELECT private_key_cipher FROM ssl_certificates WHERE id = 2").Scan(&untouched).Error; err != nil {
		t.Fatal(err)
	}
	if untouched != v2Value {
		t.Fatalf("v2 row must be left alone: %q", untouched)
	}
	// Resumability: the second run migrates nothing new because every
	// processed row carries a checkpoint.
	second, err := reencryptSecretsInDB(db, secretMigrationOptions{BackupAcknowledged: true})
	if err != nil {
		t.Fatal(err)
	}
	if second.Migrated != 0 || second.SkippedCheckpoint != report.Processed {
		t.Fatalf("re-run must skip checkpointed rows: migrated=%d skipped=%d processed=%d", second.Migrated, second.SkippedCheckpoint, report.Processed)
	}
}

// TestReencryptRequiresBackupAcknowledgement pins the §4.5 safety gate: the
// command cannot take the column backup itself, so it refuses every write
// until the operator acknowledges the backup with a flag.
func TestReencryptRequiresBackupAcknowledgement(t *testing.T) {
	inventoryScanKeys(t)
	db := newInventoryScanDB(t, "CREATE TABLE ssl_certificates (id INTEGER PRIMARY KEY, private_key_cipher TEXT)")
	legacy, err := util.EncryptSecret("unbacked-up-value")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ssl_certificates (private_key_cipher) VALUES (?)", legacy).Error; err != nil {
		t.Fatal(err)
	}
	_, err = reencryptSecretsInDB(db, secretMigrationOptions{})
	if err == nil {
		t.Fatal("write without the backup acknowledgement must be refused")
	}
	if !strings.Contains(err.Error(), "backup") {
		t.Fatalf("refusal must explain the backup requirement: %v", err)
	}
	var value string
	if err := db.Raw("SELECT private_key_cipher FROM ssl_certificates WHERE id = 1").Scan(&value).Error; err != nil {
		t.Fatal(err)
	}
	if value != legacy {
		t.Fatal("refused run must not have written anything")
	}
	// Dry-run is read-only and therefore needs no acknowledgement.
	dryRun, err := reencryptSecretsInDB(db, secretMigrationOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry run needs no backup acknowledgement: %v", err)
	}
	if dryRun.Migrated != 0 {
		t.Fatalf("dry run must not migrate, got %d", dryRun.Migrated)
	}
	if dryRun.PlannedMigrations != 1 {
		t.Fatalf("dry run must announce the migration target count, got %d", dryRun.PlannedMigrations)
	}
}

// TestReencryptHaltsOnUnknownQuarantine pins the UNKNOWN contract: the row is
// reported for quarantine (model, row id, field), never treated as plaintext,
// and the command halts leaving the mixed state behind for the dual-key
// reader to serve.
func TestReencryptHaltsOnUnknownQuarantine(t *testing.T) {
	inventoryScanKeys(t)
	db := newInventoryScanDB(t, "CREATE TABLE ssl_certificates (id INTEGER PRIMARY KEY, private_key_cipher TEXT)")
	legacy, err := util.EncryptSecret("migratable-value")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{legacy, "this-claims-nothing"} {
		if err := db.Exec("INSERT INTO ssl_certificates (private_key_cipher) VALUES (?)", value).Error; err != nil {
			t.Fatal(err)
		}
	}
	report, err := reencryptSecretsInDB(db, secretMigrationOptions{BackupAcknowledged: true})
	if err == nil {
		t.Fatal("UNKNOWN must halt the migration")
	}
	if !strings.Contains(err.Error(), "quarantine") {
		t.Fatalf("halt must name the quarantine action: %v", err)
	}
	if len(report.Quarantine) != 1 {
		t.Fatalf("quarantine report must carry the row, got %+v", report.Quarantine)
	}
	item := report.Quarantine[0]
	if item.Table != "ssl_certificates" || item.ID != 2 || item.Field != "private_key_cipher" {
		t.Fatalf("quarantine entry must identify model row and field, got %+v", item)
	}
	// The row that halts is untouched; the earlier row is already migrated
	// and keeps serving through the dual-key reader.
	var halted string
	if err := db.Raw("SELECT private_key_cipher FROM ssl_certificates WHERE id = 2").Scan(&halted).Error; err != nil {
		t.Fatal(err)
	}
	if halted != "this-claims-nothing" {
		t.Fatalf("UNKNOWN row must be left for quarantine, got %q", halted)
	}
	var migrated string
	if err := db.Raw("SELECT private_key_cipher FROM ssl_certificates WHERE id = 1").Scan(&migrated).Error; err != nil {
		t.Fatal(err)
	}
	if plain, err := util.DecryptSecretV2(migrated); err != nil || plain != "migratable-value" {
		t.Fatalf("earlier row must already be v2: %q %v", plain, err)
	}
}

// TestReencryptRejectsPClass pins the r2.4 ordering rule: P-class fields must
// never be swept into Step 2; the command aborts with an explanation unless
// the operator explicitly excludes them.
func TestReencryptRejectsPClass(t *testing.T) {
	inventoryScanKeys(t)
	db := newInventoryScanDB(t,
		"CREATE TABLE ssl_certificates (id INTEGER PRIMARY KEY, private_key_cipher TEXT)",
		"CREATE TABLE asset_credential (id INTEGER PRIMARY KEY, password TEXT)",
	)
	if err := db.Exec("INSERT INTO ssl_certificates (private_key_cipher) VALUES (?)", "").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO asset_credential (password) VALUES (?)", "plaintext-password").Error; err != nil {
		t.Fatal(err)
	}
	_, err := reencryptSecretsInDB(db, secretMigrationOptions{BackupAcknowledged: true})
	if err == nil {
		t.Fatal("P-class fields must be rejected without explicit exclusion")
	}
	if !strings.Contains(err.Error(), "P-class") {
		t.Fatalf("rejection must name the ordering rule: %v", err)
	}
	report, err := reencryptSecretsInDB(db, secretMigrationOptions{BackupAcknowledged: true, ExcludePClass: true})
	if err != nil {
		t.Fatalf("explicit exclusion must let the run proceed: %v", err)
	}
	if len(report.ExcludedPClass) == 0 {
		t.Fatal("excluded P-class fields must be listed in the report")
	}
	var password string
	if err := db.Raw("SELECT password FROM asset_credential WHERE id = 1").Scan(&password).Error; err != nil {
		t.Fatal(err)
	}
	if password != "plaintext-password" {
		t.Fatalf("P-class values must stay untouched: %q", password)
	}
}

// TestReencryptSkipsPreexistingCheckpoints simulates an interrupted run: a
// checkpoint row for a still-legacy value must cause the skip, proving the
// command resumes from (table, pk) rather than rewriting history.
func TestReencryptSkipsPreexistingCheckpoints(t *testing.T) {
	inventoryScanKeys(t)
	db := newInventoryScanDB(t, "CREATE TABLE ssl_certificates (id INTEGER PRIMARY KEY, private_key_cipher TEXT)")
	first, err := util.EncryptSecret("first-value")
	if err != nil {
		t.Fatal(err)
	}
	second, err := util.EncryptSecret("second-value")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{first, second} {
		if err := db.Exec("INSERT INTO ssl_certificates (private_key_cipher) VALUES (?)", value).Error; err != nil {
			t.Fatal(err)
		}
	}
	// A previous (interrupted) run has created the checkpoint table and
	// recorded row 2 before dying.
	if err := ensureSecretCheckpointTable(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ops_secret_migration_checkpoint (table_name, pk) VALUES ('ssl_certificates', 2)").Error; err != nil {
		t.Fatal(err)
	}
	report, err := reencryptSecretsInDB(db, secretMigrationOptions{BackupAcknowledged: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.SkippedCheckpoint != 1 || report.Migrated != 1 {
		t.Fatalf("only the un-checkpointed row may migrate: %+v", report)
	}
	var value string
	if err := db.Raw("SELECT private_key_cipher FROM ssl_certificates WHERE id = 2").Scan(&value).Error; err != nil {
		t.Fatal(err)
	}
	if value != second {
		t.Fatalf("checkpointed row must be skipped verbatim: %q", value)
	}
}

// TestReencryptMigratesDeclaredScheduleVariables covers the mixed-declaration
// column: only per-value declared secrets migrate; undeclared values stay
// declaration-exempt.
func TestReencryptMigratesDeclaredScheduleVariables(t *testing.T) {
	inventoryScanKeys(t)
	db := newInventoryScanDB(t,
		"CREATE TABLE ops_script (id INTEGER PRIMARY KEY, variables TEXT)",
		"CREATE TABLE ops_schedule_task (id INTEGER PRIMARY KEY, script_id INTEGER, variables TEXT)",
	)
	legacy, err := util.EncryptSecret("schedule-token")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ops_script (variables) VALUES (?)", `[{"name":"TOKEN","secret":true}]`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ops_schedule_task (script_id, variables) VALUES (1, ?)", `{"TOKEN":"`+legacy+`","ENV":"plain-env"}`).Error; err != nil {
		t.Fatal(err)
	}
	report, err := reencryptSecretsInDB(db, secretMigrationOptions{BackupAcknowledged: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Migrated != 1 {
		t.Fatalf("only the declared legacy variable must migrate, got %d", report.Migrated)
	}
	var stored string
	if err := db.Raw("SELECT variables FROM ops_schedule_task WHERE id = 1").Scan(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stored, "v2:") {
		t.Fatalf("declared secret must be a v2 envelope: %s", stored)
	}
	if !strings.Contains(stored, "ENV") {
		t.Fatalf("undeclared sibling variables must survive: %s", stored)
	}
}

// TestVerifyGatePassesAfterMigration covers Step 3: after a successful Step 2
// run the gate passes with a spot-decrypt sample and emits report counts.
func TestVerifyGatePassesAfterMigration(t *testing.T) {
	inventoryScanKeys(t)
	db := newInventoryScanDB(t, "CREATE TABLE ssl_certificates (id INTEGER PRIMARY KEY, private_key_cipher TEXT)")
	legacy, err := util.EncryptSecret("sampled-value")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ssl_certificates (private_key_cipher) VALUES (?)", legacy).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := reencryptSecretsInDB(db, secretMigrationOptions{BackupAcknowledged: true}); err != nil {
		t.Fatal(err)
	}
	report, err := verifySecretsInDB(db)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed {
		t.Fatalf("gate must pass on fully migrated data: %+v", report)
	}
	if report.Total.V2 != 1 {
		t.Fatalf("counts must classify the migrated row as V2: %+v", report.Total)
	}
	if len(report.Samples) != 1 {
		t.Fatalf("every v2 row is inside the sample for a one-row table, got %+v", report.Samples)
	}
	if !report.Samples[0].Decrypted {
		t.Fatalf("sample must decrypt: %+v", report.Samples[0])
	}
}

// TestVerifyGateFailsOnUnmigratedData covers the gate's negative direction:
// any LEGACY, PLAINTEXT or UNKNOWN row fails the gate and must be visible in
// the report for the operator.
func TestVerifyGateFailsOnUnmigratedData(t *testing.T) {
	inventoryScanKeys(t)
	db := newInventoryScanDB(t, "CREATE TABLE ssl_certificates (id INTEGER PRIMARY KEY, private_key_cipher TEXT)")
	legacy, err := util.EncryptSecret("still-legacy")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{legacy, "not-an-envelope"} {
		if err := db.Exec("INSERT INTO ssl_certificates (private_key_cipher) VALUES (?)", value).Error; err != nil {
			t.Fatal(err)
		}
	}
	report, err := verifySecretsInDB(db)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed {
		t.Fatal("gate must fail while legacy and unknown rows exist")
	}
	if report.Total.Legacy != 1 || report.Total.Unknown != 1 {
		t.Fatalf("report must carry the failing counts: %+v", report.Total)
	}
}

// TestVerifySpotDecryptSampleSize pins the §4.4 Step 3 sampling floor: at
// least 10% of rows, or at least 500 rows on big tables, never more rows than
// exist.
func TestVerifySpotDecryptSampleSize(t *testing.T) {
	cases := []struct{ total, want int }{
		{0, 0}, {1, 1}, {5, 1}, {10, 1}, {99, 10}, {100, 10},
		{499, 50}, {500, 500}, {1000, 500}, {4999, 500},
	}
	for _, tc := range cases {
		if got := secretSampleSize(tc.total); got != tc.want {
			t.Fatalf("secretSampleSize(%d) = %d, want %d", tc.total, got, tc.want)
		}
	}
}
