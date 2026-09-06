package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"gorm.io/gorm"

	"ops-admin/backend/config"
	"ops-admin/backend/store"
	"ops-admin/backend/util"
)

// secretCheckpointTable is the durable (table, pk) record that makes Step 2
// resumable: an interrupted run leaves a mixed state the dual-key reader
// serves, and the next run skips every checkpointed row (§4.4).
const secretCheckpointTable = "ops_secret_migration_checkpoint"

// secretMigrationOptions carries the Step 2 operator switches.
type secretMigrationOptions struct {
	// DryRun classifies without writing: counts plus the number of rows the
	// real run would migrate.
	DryRun bool
	// BackupAcknowledged records that the §4.5 pre-migration column backup
	// was taken before this run. The command cannot take the backup itself,
	// so every non-dry-run execution is refused without it.
	BackupAcknowledged bool
	// ExcludePClass is the explicit exclusion of the P-class fields, whose
	// writer conversion is its own prerequisite (ordering rule r2.4).
	ExcludePClass bool
}

// secretMigrationField reports one §4.1 field's Step 2 outcome.
type secretMigrationField struct {
	Model   string           `json:"model"`
	Table   string           `json:"table"`
	Column  string           `json:"column"`
	Class   string           `json:"class"`
	Missing bool             `json:"missing,omitempty"`
	Counts  util.FieldCounts `json:"counts"`
}

// secretMigrationReport is the Step 2 artifact.
type secretMigrationReport struct {
	DryRun bool `json:"dryRun"`
	// PlannedMigrations is set on a dry run: the number of rows a real run
	// would rewrite.
	PlannedMigrations int `json:"plannedMigrations,omitempty"`
	// Migrated is the number of rows rewritten from legacy to v2 in this run.
	Migrated int `json:"migrated"`
	// Processed is the number of rows this run examined to a decision.
	Processed      int                      `json:"processed"`
	Fields         []secretMigrationField   `json:"fields"`
	ExcludedPClass []string                 `json:"excludedPClass,omitempty"`
	Quarantine     []secretInventoryUnknown `json:"quarantine,omitempty"`
	// SkippedCheckpoint counts rows left untouched because a previous run
	// already recorded them.
	SkippedCheckpoint int `json:"skippedCheckpoint"`
}

// runReencryptSecrets implements the offline "reencrypt-secrets" command
// (§4.4 Step 2): E-class fields only, row by row, classify → read → write v2
// under the current key → verify byte-equal → checkpoint. UNKNOWN halts with
// a quarantine report; P-class fields are refused without explicit exclusion.
func runReencryptSecrets(args []string) int {
	flags := flag.NewFlagSet("reencrypt-secrets", flag.ContinueOnError)
	configPath := flags.String("config", "config.yaml", "path to config.yaml")
	dryRun := flags.Bool("dry-run", false, "classify without writing")
	backupAcknowledged := flags.Bool("backup-acknowledged", false, "confirm the §4.5 pre-migration column backup was taken")
	excludePClass := flags.Bool("exclude-p-class", false, "explicitly exclude P-class fields (ordering rule r2.4)")
	asJSON := flags.Bool("json", false, "emit machine-readable JSON")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "reencrypt-secrets: %v\n", err)
		return 1
	}
	report, err := runSecretMigration(*configPath, secretMigrationOptions{
		DryRun:             *dryRun,
		BackupAcknowledged: *backupAcknowledged,
		ExcludePClass:      *excludePClass,
	})
	if err != nil {
		if *asJSON {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			_ = encoder.Encode(report)
		}
		fmt.Fprintf(os.Stderr, "reencrypt-secrets: %v\n", err)
		return 1
	}
	if *asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "reencrypt-secrets: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Print(renderSecretMigrationText(report))
	return 0
}

// runSecretMigration follows the mandatory initialization order of the
// inventory command — config, credential seed, master key set, database —
// so the migration tool and the runtime decrypt path share one key state.
func runSecretMigration(configPath string, opts secretMigrationOptions) (secretMigrationReport, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return secretMigrationReport{}, fmt.Errorf("load config: %w", err)
	}
	util.ConfigureCredentialKey(cfg.Security.CredentialKey)
	if err := util.ConfigureSecretMasterKeys(os.Getenv("OPS_SECRET_MASTER_KEYS")); err != nil {
		return secretMigrationReport{}, fmt.Errorf("parse OPS_SECRET_MASTER_KEYS: %w", err)
	}
	db, err := store.NewDB(cfg)
	if err != nil {
		return secretMigrationReport{}, fmt.Errorf("connect db: %w", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		defer sqlDB.Close()
	}
	return reencryptSecretsInDB(db, opts)
}

// reencryptSecretsInDB performs the Step 2 sweep against an open database.
func reencryptSecretsInDB(db *gorm.DB, opts secretMigrationOptions) (secretMigrationReport, error) {
	report := secretMigrationReport{DryRun: opts.DryRun, Fields: []secretMigrationField{}, ExcludedPClass: []string{}, Quarantine: []secretInventoryUnknown{}}
	if !opts.DryRun && !opts.BackupAcknowledged {
		return report, fmt.Errorf("pre-migration column backup not acknowledged: take a backup of the affected columns (mysqldump, §4.5) before the first write, then re-run with --backup-acknowledged")
	}
	if !opts.DryRun {
		if err := ensureSecretCheckpointTable(db); err != nil {
			return report, fmt.Errorf("prepare checkpoint table: %w", err)
		}
	}
	checkpoints := map[string]bool{}
	if !opts.DryRun {
		var err error
		if checkpoints, err = loadSecretCheckpoints(db); err != nil {
			return report, fmt.Errorf("load checkpoints: %w", err)
		}
	}
	for _, field := range util.SecretFields {
		if field.Class == util.ClassPlaintext {
			// A P-class field with no column in scope carries no data to
			// endanger; the rejection exists for data that is actually there.
			if !db.Migrator().HasColumn(field.Table, field.Column) {
				continue
			}
			if !opts.ExcludePClass {
				return report, fmt.Errorf("P-class field %s.%s must not be migrated by this step: the P-class writer conversion is its own prerequisite (ordering rule r2.4); re-run with --exclude-p-class to exclude it explicitly", field.Table, field.Column)
			}
			report.ExcludedPClass = append(report.ExcludedPClass, field.Table+"."+field.Column)
			continue
		}
		if field.MixedDeclaration {
			counts, missing, err := reencryptScheduleVariables(db, field, opts, checkpoints, &report)
			if err != nil {
				return report, err
			}
			report.Fields = append(report.Fields, secretMigrationField{Model: field.Model, Table: field.Table, Column: field.Column, Class: string(field.Class), Missing: missing, Counts: counts})
			continue
		}
		counts, missing, err := reencryptColumnField(db, field, opts, checkpoints, &report)
		if err != nil {
			return report, err
		}
		report.Fields = append(report.Fields, secretMigrationField{Model: field.Model, Table: field.Table, Column: field.Column, Class: string(field.Class), Missing: missing, Counts: counts})
	}
	return report, nil
}

// reencryptColumnField sweeps one plain E-class column row by row.
func reencryptColumnField(db *gorm.DB, field util.SecretField, opts secretMigrationOptions, checkpoints map[string]bool, report *secretMigrationReport) (util.FieldCounts, bool, error) {
	if !db.Migrator().HasColumn(field.Table, field.Column) {
		return util.FieldCounts{}, true, nil
	}
	counts := util.FieldCounts{}
	var lastID uint
	for {
		var rows []struct {
			ID    uint
			Value sql.NullString
		}
		if err := db.Table(field.Table).Select("id", field.Column+" AS value").Where("id > ?", lastID).Order("id asc").Limit(inventoryBatchSize).Scan(&rows).Error; err != nil {
			return counts, false, err
		}
		if len(rows) == 0 {
			break
		}
		formats := make([]util.SecretFormat, 0, len(rows))
		for _, row := range rows {
			lastID = row.ID
			if checkpoints[secretCheckpointKey(field.Table, row.ID)] {
				report.SkippedCheckpoint++
				continue
			}
			format := util.ClassifySecret(row.Value.String, field, true)
			formats = append(formats, format)
			switch format {
			case util.FormatV2, util.FormatEmpty, util.FormatNotSecret:
				if err := markSecretCheckpoint(db, field.Table, row.ID, opts.DryRun); err != nil {
					return counts, false, err
				}
			case util.FormatLegacy:
				if err := reencryptRowValue(db, field.Table, field.Column, row.ID, row.Value.String, field, opts, report); err != nil {
					return counts, false, err
				}
			default:
				report.Quarantine = append(report.Quarantine, secretInventoryUnknown{Table: field.Table, ID: row.ID, Field: field.Column, Length: len(row.Value.String)})
				return counts, false, fmt.Errorf("quarantine required: %s id=%d field=%s claims neither the v2 nor the legacy format; it must not be treated as plaintext", field.Table, row.ID, field.Column)
			}
			report.Processed++
		}
		counts = addFieldCounts(counts, util.AggregateFormats(formats))
		if len(rows) < inventoryBatchSize {
			break
		}
	}
	return counts, false, nil
}

// reencryptRowValue performs one row's read → write v2 → verify cycle. The
// verify step decrypts the new envelope and byte-compares it against the
// in-memory plaintext before any persistence is considered done.
func reencryptRowValue(db *gorm.DB, table, column string, id uint, stored string, field util.SecretField, opts secretMigrationOptions, report *secretMigrationReport) error {
	plaintext, err := util.ReadSecretField(stored, field, true)
	if err != nil {
		return fmt.Errorf("read %s id=%d %s: %w", table, id, column, err)
	}
	v2Value, err := util.EncryptSecretV2(plaintext)
	if err != nil {
		return fmt.Errorf("seal %s id=%d %s: %w", table, id, column, err)
	}
	reopened, err := util.DecryptSecretV2(v2Value)
	if err != nil || reopened != plaintext {
		return fmt.Errorf("verify %s id=%d %s failed: decrypt-as-v2 does not equal the in-memory plaintext", table, id, column)
	}
	if opts.DryRun {
		report.PlannedMigrations++
		return nil
	}
	if err := db.Table(table).Where("id = ?", id).Update(column, v2Value).Error; err != nil {
		return fmt.Errorf("write %s id=%d %s: %w", table, id, column, err)
	}
	report.Migrated++
	return markSecretCheckpoint(db, table, id, false)
}

// reencryptScheduleVariables sweeps the mixed-declaration variables column
// per value: only per-value declared secrets migrate, undeclared values stay
// declaration-exempt. Checkpoints are per task row.
func reencryptScheduleVariables(db *gorm.DB, field util.SecretField, opts secretMigrationOptions, checkpoints map[string]bool, report *secretMigrationReport) (util.FieldCounts, bool, error) {
	if !db.Migrator().HasColumn(field.Table, field.Column) {
		return util.FieldCounts{}, true, nil
	}
	secretNames, err := scriptDeclaredSecretNames(db)
	if err != nil {
		return util.FieldCounts{}, false, err
	}
	counts := util.FieldCounts{}
	var lastID uint
	for {
		var tasks []struct {
			ID        uint
			ScriptID  uint
			Variables sql.NullString
		}
		if err := db.Table(field.Table).Select("id", "script_id", "variables").Where("id > ?", lastID).Order("id asc").Limit(inventoryBatchSize).Scan(&tasks).Error; err != nil {
			return counts, false, err
		}
		if len(tasks) == 0 {
			break
		}
		for _, task := range tasks {
			lastID = task.ID
			if checkpoints[secretCheckpointKey(field.Table, task.ID)] {
				report.SkippedCheckpoint++
				continue
			}
			taskFormats, err := reencryptTaskVariables(db, task.ID, task.ScriptID, task.Variables, secretNames, field, opts, report)
			if err != nil {
				return counts, false, err
			}
			counts = addFieldCounts(counts, util.AggregateFormats(taskFormats))
			if err := markSecretCheckpoint(db, field.Table, task.ID, opts.DryRun); err != nil {
				return counts, false, err
			}
			report.Processed++
		}
		if len(tasks) < inventoryBatchSize {
			break
		}
	}
	return counts, false, nil
}

// reencryptTaskVariables migrates the declared legacy values of one task row,
// persisting the rewritten JSON map when anything changed.
func reencryptTaskVariables(db *gorm.DB, taskID, scriptID uint, stored sql.NullString, secretNames map[uint]map[string]bool, field util.SecretField, opts secretMigrationOptions, report *secretMigrationReport) ([]util.SecretFormat, error) {
	values := map[string]string{}
	if stored.Valid && stored.String != "" {
		if err := json.Unmarshal([]byte(stored.String), &values); err != nil {
			report.Quarantine = append(report.Quarantine, secretInventoryUnknown{Table: field.Table, ID: taskID, Field: field.Column, Length: len(stored.String)})
			return []util.SecretFormat{util.FormatUnknown}, fmt.Errorf("quarantine required: %s id=%d field=%s is not a parseable JSON map", field.Table, taskID, field.Column)
		}
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	formats := make([]util.SecretFormat, 0, len(names))
	changed := false
	for _, name := range names {
		value := values[name]
		gate := secretNames[scriptID][name]
		format := util.ClassifySecret(value, field, gate)
		formats = append(formats, format)
		switch format {
		case util.FormatLegacy:
			plaintext, err := util.ReadSecretField(value, field, gate)
			if err != nil {
				return formats, fmt.Errorf("read %s id=%d variable %s: %w", field.Table, taskID, name, err)
			}
			v2Value, err := util.EncryptSecretV2(plaintext)
			if err != nil {
				return formats, fmt.Errorf("seal %s id=%d variable %s: %w", field.Table, taskID, name, err)
			}
			reopened, err := util.DecryptSecretV2(v2Value)
			if err != nil || reopened != plaintext {
				return formats, fmt.Errorf("verify %s id=%d variable %s failed: decrypt-as-v2 does not equal the in-memory plaintext", field.Table, taskID, name)
			}
			if opts.DryRun {
				report.PlannedMigrations++
				continue
			}
			values[name] = v2Value
			changed = true
			report.Migrated++
		case util.FormatUnknown:
			report.Quarantine = append(report.Quarantine, secretInventoryUnknown{Table: field.Table, ID: taskID, Field: name, Length: len(value)})
			return formats, fmt.Errorf("quarantine required: %s id=%d variable %s claims neither the v2 nor the legacy format; it must not be treated as plaintext", field.Table, taskID, name)
		}
	}
	if changed {
		encoded, err := json.Marshal(values)
		if err != nil {
			return formats, err
		}
		if err := db.Table(field.Table).Where("id = ?", taskID).Update(field.Column, string(encoded)).Error; err != nil {
			return formats, fmt.Errorf("write %s id=%d: %w", field.Table, taskID, err)
		}
	}
	return formats, nil
}

// scriptDeclaredSecretNames loads the per-script declared-secret variable
// names from the ops_script variable metadata. Values without a declaring
// script are not secrets by declaration, mirroring the runtime gate.
func scriptDeclaredSecretNames(db *gorm.DB) (map[uint]map[string]bool, error) {
	secretNames := map[uint]map[string]bool{}
	var scripts []struct {
		ID        uint
		Variables sql.NullString
	}
	if err := db.Table("ops_script").Select("id", "variables").Find(&scripts).Error; err != nil {
		return nil, err
	}
	for _, script := range scripts {
		names := map[string]bool{}
		if script.Variables.Valid && script.Variables.String != "" {
			var declared []struct {
				Name   string `json:"name"`
				Secret bool   `json:"secret"`
			}
			// Malformed script metadata cannot declare secrets; the values it
			// would have gated then count as not-secret by declaration.
			if err := json.Unmarshal([]byte(script.Variables.String), &declared); err == nil {
				for _, variable := range declared {
					if variable.Secret {
						names[variable.Name] = true
					}
				}
			}
		}
		secretNames[script.ID] = names
	}
	return secretNames, nil
}

func secretCheckpointKey(table string, pk uint) string {
	return fmt.Sprintf("%s:%d", table, pk)
}

// ensureSecretCheckpointTable creates the checkpoint table with portable SQL
// (no auto-increment, composite primary key) so both the production MySQL and
// the test sqlite accept it.
func ensureSecretCheckpointTable(db *gorm.DB) error {
	return db.Exec("CREATE TABLE IF NOT EXISTS " + secretCheckpointTable +
		" (table_name VARCHAR(191) NOT NULL, pk BIGINT NOT NULL, PRIMARY KEY (table_name, pk))").Error
}

func loadSecretCheckpoints(db *gorm.DB) (map[string]bool, error) {
	var rows []struct {
		TableName string
		PK        uint
	}
	if err := db.Table(secretCheckpointTable).Select("table_name", "pk").Find(&rows).Error; err != nil {
		return nil, err
	}
	checkpoints := make(map[string]bool, len(rows))
	for _, row := range rows {
		checkpoints[secretCheckpointKey(row.TableName, row.PK)] = true
	}
	return checkpoints, nil
}

// markSecretCheckpoint records a processed row so later runs skip it. Dry
// runs never write checkpoints.
func markSecretCheckpoint(db *gorm.DB, table string, pk uint, dryRun bool) error {
	if dryRun {
		return nil
	}
	return db.Exec("INSERT INTO "+secretCheckpointTable+" (table_name, pk) VALUES (?, ?)", table, pk).Error
}

// renderSecretMigrationText renders the human-readable Step 2 summary.
func renderSecretMigrationText(report secretMigrationReport) string {
	var buffer strings.Builder
	if report.DryRun {
		buffer.WriteString("Secret re-encryption dry run (no writes performed)\n\n")
	} else {
		buffer.WriteString("Secret re-encryption (§4.4 Step 2)\n\n")
	}
	buffer.WriteString(fmt.Sprintf("migrated=%d processed=%d skipped-checkpoint=%d", report.Migrated, report.Processed, report.SkippedCheckpoint))
	if report.DryRun {
		buffer.WriteString(fmt.Sprintf(" planned-migrations=%d", report.PlannedMigrations))
	}
	buffer.WriteString("\n\n")
	for _, field := range report.Fields {
		missing := ""
		if field.Missing {
			missing = " (column missing)"
		}
		fmt.Fprintf(&buffer, "%s.%s [%s]%s migrated=%d legacy=%d v2=%d empty=%d not-secret=%d\n",
			field.Model, field.Column, field.Class, missing,
			report.Migrated, field.Counts.Legacy, field.Counts.V2, field.Counts.Empty, field.Counts.NotSecret)
	}
	if len(report.ExcludedPClass) > 0 {
		buffer.WriteString(fmt.Sprintf("\nexcluded P-class fields (%d): %s\n", len(report.ExcludedPClass), strings.Join(report.ExcludedPClass, ", ")))
	}
	if len(report.Quarantine) > 0 {
		buffer.WriteString("\nquarantine required (UNKNOWN rows; never treat as plaintext):\n")
		for _, item := range report.Quarantine {
			fmt.Fprintf(&buffer, "  %s id=%d field=%s length=%d\n", item.Table, item.ID, item.Field, item.Length)
		}
	}
	return buffer.String()
}
