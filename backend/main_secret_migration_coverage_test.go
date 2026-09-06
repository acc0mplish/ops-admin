package main

import (
	"fmt"
	"strings"
	"testing"

	"gorm.io/gorm"

	"ops-admin/backend/internal/testutil"
	"ops-admin/backend/util"
)

// TestRunReencryptSecretsFlagHandling covers the CLI surface of the Step 2
// command: malformed flags and a missing config fail with exit 1 before any
// database work happens.
func TestRunReencryptSecretsFlagHandling(t *testing.T) {
	if code := runReencryptSecrets([]string{"--undefined-flag"}); code != 1 {
		t.Fatalf("undefined flag must exit 1, got %d", code)
	}
	if code := runReencryptSecrets([]string{"--config", "/nonexistent/config.yaml"}); code != 1 {
		t.Fatalf("missing config must exit 1, got %d", code)
	}
}

// TestRunVerifySecretsFlagHandling covers the CLI surface of the Step 3 gate.
func TestRunVerifySecretsFlagHandling(t *testing.T) {
	if code := runVerifySecrets([]string{"--undefined-flag"}); code != 1 {
		t.Fatalf("undefined flag must exit 1, got %d", code)
	}
}

// TestRunSecretMigrationLoadFailure pins the error wrapping of the mandatory
// initialization order: a config failure aborts before key or db setup.
func TestRunSecretMigrationLoadFailure(t *testing.T) {
	_, err := runSecretMigration("/nonexistent/config.yaml", secretMigrationOptions{})
	if err == nil {
		t.Fatal("missing config must abort the migration")
	}
	if !strings.Contains(err.Error(), "load config") {
		t.Fatalf("error must name the failing stage: %v", err)
	}
}

// TestReencryptDryRunLeavesVariablesUntouched pins the dry-run contract on
// the mixed-declaration column: the plan is announced, nothing is written and
// no checkpoint table is created.
func TestReencryptDryRunLeavesVariablesUntouched(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t,
		"CREATE TABLE ops_script (id INTEGER PRIMARY KEY, variables TEXT)",
		"CREATE TABLE ops_schedule_task (id INTEGER PRIMARY KEY, script_id INTEGER, variables TEXT)",
	)
	legacy, err := util.EncryptSecret("dry-run-token")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ops_script (variables) VALUES (?)", `[{"name":"TOKEN","secret":true}]`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ops_schedule_task (script_id, variables) VALUES (1, ?)", `{"TOKEN":"`+legacy+`"}`).Error; err != nil {
		t.Fatal(err)
	}
	report, err := reencryptSecretsInDB(db, secretMigrationOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.PlannedMigrations != 1 || report.Migrated != 0 {
		t.Fatalf("dry run must plan without migrating: planned=%d migrated=%d", report.PlannedMigrations, report.Migrated)
	}
	if db.Migrator().HasTable(secretCheckpointTable) {
		t.Fatal("dry run must not create the checkpoint table")
	}
	var stored string
	if err := db.Raw("SELECT variables FROM ops_schedule_task WHERE id = 1").Scan(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, "v2:") {
		t.Fatalf("dry run must not rewrite the variables column: %s", stored)
	}
}

// TestReencryptHaltsOnMalformedVariablesJSON pins the UNKNOWN handling of the
// variables column: a value that claims no parseable JSON shape halts the run
// and lands in the quarantine report, never in plaintext interpretation.
func TestReencryptHaltsOnMalformedVariablesJSON(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t,
		"CREATE TABLE ops_script (id INTEGER PRIMARY KEY, variables TEXT)",
		"CREATE TABLE ops_schedule_task (id INTEGER PRIMARY KEY, script_id INTEGER, variables TEXT)",
	)
	if err := db.Exec("INSERT INTO ops_schedule_task (script_id, variables) VALUES (1, 'not-a-json-map')").Error; err != nil {
		t.Fatal(err)
	}
	report, err := reencryptSecretsInDB(db, secretMigrationOptions{BackupAcknowledged: true})
	if err == nil {
		t.Fatal("malformed variables JSON must halt the migration")
	}
	if !strings.Contains(err.Error(), "quarantine") {
		t.Fatalf("halt must name the quarantine action: %v", err)
	}
	if len(report.Quarantine) != 1 || report.Quarantine[0].ID != 1 {
		t.Fatalf("quarantine must carry the malformed row, got %+v", report.Quarantine)
	}
}

// TestVerifySamplesScheduleVariables closes the sampler gap on the
// mixed-declaration column: declared v2 variables enter the spot-decrypt
// sample and the gate report carries them.
func TestVerifySamplesScheduleVariables(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t,
		"CREATE TABLE ops_script (id INTEGER PRIMARY KEY, variables TEXT)",
		"CREATE TABLE ops_schedule_task (id INTEGER PRIMARY KEY, script_id INTEGER, variables TEXT)",
	)
	v2Value, err := util.EncryptSecretV2("sampled-variable")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ops_script (variables) VALUES (?)", `[{"name":"TOKEN","secret":true}]`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ops_schedule_task (script_id, variables) VALUES (1, ?)", `{"TOKEN":"`+v2Value+`"}`).Error; err != nil {
		t.Fatal(err)
	}
	report, err := verifySecretsInDB(db)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed {
		t.Fatalf("gate must pass: %+v", report)
	}
	if len(report.Samples) != 1 {
		t.Fatalf("the declared variable must be sampled, got %+v", report.Samples)
	}
	sample := report.Samples[0]
	if sample.Field != "TOKEN" || sample.ID != 1 || !sample.Decrypted {
		t.Fatalf("sample must identify the variable and decrypt: %+v", sample)
	}
}

// TestVerifyGateFailsOnCorruptedV2Sample pins the spot-decrypt direction of
// the gate: a value that classifies as V2 but does not decrypt fails the gate
// even though the counts look clean.
func TestVerifyGateFailsOnCorruptedV2Sample(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t, "CREATE TABLE ssl_certificates (id INTEGER PRIMARY KEY, private_key_cipher TEXT)")
	// Well-formed envelope header with the known current key id but an
	// invalid payload: the classifier accepts it as V2, the spot decrypt
	// must not.
	if err := db.Exec("INSERT INTO ssl_certificates (private_key_cipher) VALUES (?)", "v2:legacy:AAAA").Error; err != nil {
		t.Fatal(err)
	}
	report, err := verifySecretsInDB(db)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed {
		t.Fatal("gate must fail when a sampled v2 row does not decrypt")
	}
	if report.Total.V2 != 1 || report.Total.Unknown != 0 {
		t.Fatalf("counts alone must look clean for this failure mode: %+v", report.Total)
	}
	if !strings.Contains(report.GateFailure, "spot decrypt failed") {
		t.Fatalf("gate failure must name the spot decrypt: %s", report.GateFailure)
	}
}

// TestVerifySamplingFloorOnLargeTable exercises the >=500-row branch of the
// sampling floor against a table of 600 v2 rows.
func TestVerifySamplingFloorOnLargeTable(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t, "CREATE TABLE ssl_certificates (id INTEGER PRIMARY KEY, private_key_cipher TEXT)")
	for index := 0; index < 600; index++ {
		v2Value, err := util.EncryptSecretV2(fmt.Sprintf("bulk-value-%d", index))
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("INSERT INTO ssl_certificates (private_key_cipher) VALUES (?)", v2Value).Error; err != nil {
			t.Fatal(err)
		}
	}
	report, err := verifySecretsInDB(db)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed {
		t.Fatalf("gate must pass on uniform v2 data: %s", report.GateFailure)
	}
	if report.SampledRows != 500 {
		t.Fatalf("600 v2 rows must sample exactly 500, got %d", report.SampledRows)
	}
	if len(report.Samples) != report.SampledRows {
		t.Fatalf("sample list must match the sampled count: %d vs %d", len(report.Samples), report.SampledRows)
	}
}

// TestRenderSecretMigrationText covers the dry-run, exclusion and quarantine
// rendering branches of the Step 2 summary.
func TestRenderSecretMigrationText(t *testing.T) {
	dryRun := renderSecretMigrationText(secretMigrationReport{
		DryRun:            true,
		PlannedMigrations: 2,
		Fields:            []secretMigrationField{{Model: "OpsScheduleTask", Table: "ops_schedule_task", Column: "variables", Class: "E-legacy"}},
	})
	for _, want := range []string{"dry run", "no writes", "planned-migrations=2"} {
		if !strings.Contains(dryRun, want) {
			t.Fatalf("dry-run render is missing %q:\n%s", want, dryRun)
		}
	}
	writeRun := renderSecretMigrationText(secretMigrationReport{
		Migrated:          3,
		ExcludedPClass:    []string{"asset_credential.password"},
		Quarantine:        []secretInventoryUnknown{{Table: "ssl_certificates", ID: 9, Field: "private_key_cipher", Length: 4}},
		SkippedCheckpoint: 5,
	})
	for _, want := range []string{"migrated=3", "skipped-checkpoint=5", "excluded P-class fields (1)", "asset_credential.password", "quarantine required", "id=9"} {
		if !strings.Contains(writeRun, want) {
			t.Fatalf("write-run render is missing %q:\n%s", want, writeRun)
		}
	}
}

// TestRenderSecretVerificationText covers the pass and fail rendering of the
// Step 3 gate report.
func TestRenderSecretVerificationText(t *testing.T) {
	passed := renderSecretVerificationText(secretVerificationReport{
		Passed:      true,
		Total:       util.FieldCounts{Total: 4, V2: 3, Empty: 1},
		SampledRows: 1,
		Samples:     []secretVerifySample{{Table: "ssl_certificates", ID: 2, Field: "private_key_cipher", Decrypted: true}},
	})
	for _, want := range []string{"PASS", "total=4", "v2=3", "sampled=1", "id=2"} {
		if !strings.Contains(passed, want) {
			t.Fatalf("pass render is missing %q:\n%s", want, passed)
		}
	}
	failed := renderSecretVerificationText(secretVerificationReport{
		Total:       util.FieldCounts{Total: 1, Legacy: 1},
		GateFailure: "unmigrated rows remain: legacy=1",
	})
	for _, want := range []string{"FAIL", "reason:", "unmigrated rows remain"} {
		if !strings.Contains(failed, want) {
			t.Fatalf("fail render is missing %q:\n%s", want, failed)
		}
	}
}

// TestCheckpointTableRebuildFromNarrowSchema pins the schema migration of
// the checkpoint store: a table left by an earlier build with the narrower
// (table, pk) key is dropped and rebuilt with the cell-level key, and the
// sweep re-checkpoints the already-migrated cells under the wider key.
func TestCheckpointTableRebuildFromNarrowSchema(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t, "CREATE TABLE ssl_certificates (id INTEGER PRIMARY KEY, private_key_cipher TEXT)")
	legacy, err := util.EncryptSecret("rebuild-probe")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ssl_certificates (private_key_cipher) VALUES (?)", legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE " + secretCheckpointTable + " (table_name VARCHAR(191) NOT NULL, pk BIGINT NOT NULL, PRIMARY KEY (table_name, pk))").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO " + secretCheckpointTable + " (table_name, pk) VALUES ('ssl_certificates', 1)").Error; err != nil {
		t.Fatal(err)
	}
	report, err := reencryptSecretsInDB(db, secretMigrationOptions{BackupAcknowledged: true})
	if err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn(secretCheckpointTable, "column_name") {
		t.Fatal("narrow checkpoint table must be rebuilt with the column_name key")
	}
	// The stale checkpoint is gone, so the legacy cell was migrated anyway.
	if report.Migrated != 1 {
		t.Fatalf("rebuild must not strand the legacy cell, migrated=%d", report.Migrated)
	}
	var count int64
	if err := db.Table(secretCheckpointTable).Where("table_name = ? AND column_name = ? AND pk = ?", "ssl_certificates", "private_key_cipher", 1).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("the migrated cell must be re-checkpointed under the wider key, got %d", count)
	}
}

// TestStrideSampleEdges pins the deterministic stride selection at its edges.
func TestStrideSampleEdges(t *testing.T) {
	if got := strideSample(0, 1); got != nil {
		t.Fatalf("empty table must sample nothing, got %v", got)
	}
	if got := strideSample(3, 0); got != nil {
		t.Fatalf("zero sample size must sample nothing, got %v", got)
	}
	got := strideSample(2, 5)
	if len(got) != 2 {
		t.Fatalf("oversized sample request must clamp to total, got %v", got)
	}
}

// TestReencryptVerifyConsumesPersistedBytes proves the verify step observes
// the database, not the sealed in-memory envelope: a tampered read-back must
// fail the migration, skip the checkpoint, and leave the row healable by the
// next (untampered) run.
func TestReencryptVerifyConsumesPersistedBytes(t *testing.T) {
	testutil.PinSecretKeys(t)
	db := testutil.OpenMemoryDB(t, "CREATE TABLE ssl_certificates (id INTEGER PRIMARY KEY, private_key_cipher TEXT)")
	legacy, err := util.EncryptSecret("tamper-probe-value")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ssl_certificates (private_key_cipher) VALUES (?)", legacy).Error; err != nil {
		t.Fatal(err)
	}
	original := reencryptPersistedReader
	t.Cleanup(func() { reencryptPersistedReader = original })
	reencryptPersistedReader = func(db *gorm.DB, table, column string, id uint) (string, error) {
		// Pretend the persisted bytes do not decrypt back to the plaintext.
		return "v2:legacy:dGFtcGVyZWQ", nil
	}
	report, err := reencryptSecretsInDB(db, secretMigrationOptions{BackupAcknowledged: true})
	if err == nil {
		t.Fatal("a tampered read-back must fail the migration")
	}
	if !strings.Contains(err.Error(), "persisted value does not decrypt") {
		t.Fatalf("failure must name the persisted verify step: %v", err)
	}
	if report.Migrated != 0 {
		t.Fatalf("an unverified row must not be counted as migrated, got %d", report.Migrated)
	}
	var count int64
	if err := db.Table(secretCheckpointTable).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("an unverified row must carry no checkpoint, got %d", count)
	}
	// The next run with an honest reader heals the row: the v2 write from the
	// failed run re-classifies as v2 and re-checkpoints.
	reencryptPersistedReader = original
	retry, err := reencryptSecretsInDB(db, secretMigrationOptions{BackupAcknowledged: true})
	if err != nil {
		t.Fatalf("retry after a failed verify must succeed: %v", err)
	}
	if retry.Migrated != 0 {
		t.Fatalf("the v2 value from the failed run needs no rewrite, got %d", retry.Migrated)
	}
	var value string
	if err := db.Raw("SELECT private_key_cipher FROM ssl_certificates WHERE id = 1").Scan(&value).Error; err != nil {
		t.Fatal(err)
	}
	if plain, err := util.DecryptSecretV2(value); err != nil || plain != "tamper-probe-value" {
		t.Fatalf("row must round trip as v2 after the retry: %q %v", plain, err)
	}
}
