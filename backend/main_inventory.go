package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"gorm.io/gorm"

	"ops-admin/backend/config"
	"ops-admin/backend/store"
	"ops-admin/backend/util"
)

// inventoryBatchSize bounds the per-batch row scan of every registered
// column; the inventory is a deliberate full scan, kept read-only.
const inventoryBatchSize = 1000

// inventoryUnknownNote carries the standing interpretation guide for the
// report: mass UNKNOWN in E-class fields is the expected pre-migration state
// the inventory exists to size, not an incident.
const inventoryUnknownNote = "UNKNOWN in E-class fields means the stored value claims neither the v2 nor the legacy format. " +
	"Historical plaintext data in E-class fields reports mass UNKNOWN: this is the expected pre-migration state, not an incident. " +
	"The report exists to size exactly that; the migration gate is a later step."

type secretInventoryField struct {
	Model  string           `json:"model"`
	Table  string           `json:"table"`
	Column string           `json:"column"`
	Class  string           `json:"class"`
	Counts util.FieldCounts `json:"counts"`
	// Missing reports that the registered column does not exist in the
	// physical table. The measurement is zero counts, never a guess.
	Missing bool `json:"missing,omitempty"`
}

type secretInventoryUnknown struct {
	Table  string `json:"table"`
	ID     uint   `json:"id"`
	Field  string `json:"field"`
	Length int    `json:"length"`
}

type secretInventoryReport struct {
	Fields   []secretInventoryField   `json:"fields"`
	Unknowns []secretInventoryUnknown `json:"unknowns"`
	Total    util.FieldCounts         `json:"total"`
	Note     string                   `json:"note"`
}

// runSecretInventory implements the read-only "inventory-secrets" command: it
// classifies every §4.1 secret field and emits per-field format counts as the
// migration's pre-flight artifact. Measurement only — the command exits 0 even
// when UNKNOWN rows exist; config, master-key or database errors exit 1.
func runSecretInventory(args []string) int {
	flags := flag.NewFlagSet("inventory-secrets", flag.ContinueOnError)
	configPath := flags.String("config", "config.yaml", "path to config.yaml")
	asJSON := flags.Bool("json", false, "emit machine-readable JSON")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "inventory-secrets: %v\n", err)
		return 1
	}
	report, err := collectSecretInventory(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "inventory-secrets: %v\n", err)
		return 1
	}
	if *asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "inventory-secrets: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Print(renderSecretInventoryText(report))
	return 0
}

// collectSecretInventory follows the mandatory initialization order: config,
// credential seed, master key set, database. Skipping the credential seed step
// would derive the legacy key from the development fallback seed and misreport
// the whole inventory as UNKNOWN. No AutoMigrate, no seed: read-only command.
func collectSecretInventory(configPath string) (secretInventoryReport, error) {
	report := secretInventoryReport{Fields: []secretInventoryField{}, Unknowns: []secretInventoryUnknown{}, Note: inventoryUnknownNote}
	cfg, err := config.Load(configPath)
	if err != nil {
		return report, fmt.Errorf("load config: %w", err)
	}
	util.ConfigureCredentialKey(cfg.Security.CredentialKey)
	if err := util.ConfigureSecretMasterKeys(os.Getenv("OPS_SECRET_MASTER_KEYS")); err != nil {
		return report, fmt.Errorf("parse OPS_SECRET_MASTER_KEYS: %w", err)
	}
	db, err := store.NewDB(cfg)
	if err != nil {
		return report, fmt.Errorf("connect db: %w", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		defer sqlDB.Close()
	}
	for _, field := range util.SecretFields {
		counts, unknowns, missing, err := scanSecretField(db, field)
		if err != nil {
			return report, fmt.Errorf("scan %s.%s: %w", field.Table, field.Column, err)
		}
		report.Fields = append(report.Fields, secretInventoryField{Model: field.Model, Table: field.Table, Column: field.Column, Class: string(field.Class), Counts: counts, Missing: missing})
		report.Unknowns = append(report.Unknowns, unknowns...)
		report.Total = addFieldCounts(report.Total, counts)
	}
	return report, nil
}

// scanSecretField classifies every row of one registered column in batches.
// A registered column missing from the physical table is reported as a zero
// measurement with missing=true — a schema drift the report must surface, not
// an error to abort on. The mixed-declaration schedule column takes the
// per-value path instead.
func scanSecretField(db *gorm.DB, field util.SecretField) (util.FieldCounts, []secretInventoryUnknown, bool, error) {
	if !db.Migrator().HasColumn(field.Table, field.Column) {
		return util.FieldCounts{}, nil, true, nil
	}
	if field.MixedDeclaration {
		counts, unknowns, err := scanScheduleTaskVariables(db, field)
		return counts, unknowns, false, err
	}
	counts := util.FieldCounts{}
	unknowns := []secretInventoryUnknown{}
	var lastID uint
	for {
		var rows []struct {
			ID    uint
			Value sql.NullString
		}
		if err := db.Table(field.Table).Select("id", field.Column+" AS value").Where("id > ?", lastID).Order("id asc").Limit(inventoryBatchSize).Scan(&rows).Error; err != nil {
			return counts, unknowns, false, err
		}
		if len(rows) == 0 {
			break
		}
		formats := make([]util.SecretFormat, 0, len(rows))
		for _, row := range rows {
			lastID = row.ID
			format := util.ClassifySecret(row.Value.String, field, true)
			formats = append(formats, format)
			if format == util.FormatUnknown {
				unknowns = append(unknowns, secretInventoryUnknown{Table: field.Table, ID: row.ID, Field: field.Column, Length: len(row.Value.String)})
			}
		}
		counts = addFieldCounts(counts, util.AggregateFormats(formats))
		if len(rows) < inventoryBatchSize {
			break
		}
	}
	return counts, unknowns, false, nil
}

// scanScheduleTaskVariables classifies the mixed-declaration variables column
// per value, taking each declared-secret gate from the owning script's
// variable metadata. Values without a declaring script are not secrets by
// declaration, mirroring the runtime gate.
func scanScheduleTaskVariables(db *gorm.DB, field util.SecretField) (util.FieldCounts, []secretInventoryUnknown, error) {
	counts := util.FieldCounts{}
	unknowns := []secretInventoryUnknown{}
	secretNames := map[uint]map[string]bool{}
	var scripts []struct {
		ID        uint
		Variables sql.NullString
	}
	if err := db.Table("ops_script").Select("id", "variables").Find(&scripts).Error; err != nil {
		return counts, unknowns, err
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
	var lastID uint
	for {
		var tasks []struct {
			ID        uint
			ScriptID  uint
			Variables sql.NullString
		}
		if err := db.Table("ops_schedule_task").Select("id", "script_id", "variables").Where("id > ?", lastID).Order("id asc").Limit(inventoryBatchSize).Scan(&tasks).Error; err != nil {
			return counts, unknowns, err
		}
		if len(tasks) == 0 {
			break
		}
		formats := make([]util.SecretFormat, 0, len(tasks))
		for _, task := range tasks {
			lastID = task.ID
			var values map[string]string
			if task.Variables.Valid && task.Variables.String != "" {
				if err := json.Unmarshal([]byte(task.Variables.String), &values); err != nil {
					// The column is a JSON map at runtime; a value that claims
					// no parseable shape is reported as UNKNOWN.
					formats = append(formats, util.FormatUnknown)
					unknowns = append(unknowns, secretInventoryUnknown{Table: field.Table, ID: task.ID, Field: field.Column, Length: len(task.Variables.String)})
					continue
				}
			}
			names := make([]string, 0, len(values))
			for name := range values {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				value := values[name]
				format := util.ClassifySecret(value, field, secretNames[task.ScriptID][name])
				formats = append(formats, format)
				if format == util.FormatUnknown {
					unknowns = append(unknowns, secretInventoryUnknown{Table: field.Table, ID: task.ID, Field: name, Length: len(value)})
				}
			}
		}
		counts = addFieldCounts(counts, util.AggregateFormats(formats))
		if len(tasks) < inventoryBatchSize {
			break
		}
	}
	return counts, unknowns, nil
}

func addFieldCounts(a, b util.FieldCounts) util.FieldCounts {
	return util.FieldCounts{
		Total:     a.Total + b.Total,
		V2:        a.V2 + b.V2,
		Legacy:    a.Legacy + b.Legacy,
		Plaintext: a.Plaintext + b.Plaintext,
		Empty:     a.Empty + b.Empty,
		Unknown:   a.Unknown + b.Unknown,
		NotSecret: a.NotSecret + b.NotSecret,
	}
}

// renderSecretInventoryText renders the human-readable report table plus the
// UNKNOWN interpretation guide and a bounded list of UNKNOWN rows.
func renderSecretInventoryText(report secretInventoryReport) string {
	var buffer strings.Builder
	buffer.WriteString("Secret field inventory (v2 pre-flight measurement)\n\n")
	writer := tabwriter.NewWriter(&buffer, 2, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "FIELD\tCLASS\tTOTAL\tV2\tLEGACY\tPLAINTEXT\tEMPTY\tUNKNOWN\tNOT_SECRET")
	for _, field := range report.Fields {
		class := field.Class
		if field.Missing {
			class += " (column missing)"
		}
		fmt.Fprintf(writer, "%s.%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
			field.Model, field.Column, class,
			field.Counts.Total, field.Counts.V2, field.Counts.Legacy, field.Counts.Plaintext,
			field.Counts.Empty, field.Counts.Unknown, field.Counts.NotSecret)
	}
	fmt.Fprintf(writer, "TOTAL\t\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
		report.Total.Total, report.Total.V2, report.Total.Legacy, report.Total.Plaintext,
		report.Total.Empty, report.Total.Unknown, report.Total.NotSecret)
	writer.Flush()
	buffer.WriteString("\n")
	buffer.WriteString(report.Note)
	buffer.WriteString("\n")
	if len(report.Unknowns) > 0 {
		fmt.Fprintf(&buffer, "\nUNKNOWN rows (%d total, first 20 listed; --json carries the full list):\n", len(report.Unknowns))
		for index, item := range report.Unknowns {
			if index == 20 {
				buffer.WriteString("  …\n")
				break
			}
			fmt.Fprintf(&buffer, "  %s id=%d field=%s length=%d\n", item.Table, item.ID, item.Field, item.Length)
		}
	}
	return buffer.String()
}
