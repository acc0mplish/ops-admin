package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"gorm.io/gorm"

	"ops-admin/backend/config"
	"ops-admin/backend/store"
	"ops-admin/backend/util"
)

// secretVerifySample records one spot-decrypted row of the §4.4 Step 3 gate.
type secretVerifySample struct {
	Table     string `json:"table"`
	ID        uint   `json:"id"`
	Field     string `json:"field"`
	Decrypted bool   `json:"decrypted"`
	Length    int    `json:"length"`
}

// secretVerificationField reports one §4.1 field's Step 3 classification.
type secretVerificationField struct {
	Model   string           `json:"model"`
	Table   string           `json:"table"`
	Column  string           `json:"column"`
	Missing bool             `json:"missing,omitempty"`
	Counts  util.FieldCounts `json:"counts"`
}

// secretVerificationReport is the Step 3 migration report artifact: counts
// plus the spot-decrypt sample results.
type secretVerificationReport struct {
	Fields      []secretVerificationField `json:"fields"`
	Total       util.FieldCounts          `json:"total"`
	Samples     []secretVerifySample      `json:"samples"`
	SampledRows int                       `json:"sampledRows"`
	Passed      bool                      `json:"passed"`
	// GateFailure names the reason the gate failed, empty when passed.
	GateFailure string `json:"gateFailure,omitempty"`
}

// runVerifySecrets implements the "verify-secrets" gate command (§4.4
// Step 3): the classifier runs again over every §4.1 field, a random-style
// sample of at least 10% (or at least 500 rows) per table is spot-decrypted,
// and the report is emitted as a JSON artifact when --out is given. Exit 0
// means the gate passed; any non-zero exit fails the gate.
func runVerifySecrets(args []string) int {
	flags := flag.NewFlagSet("verify-secrets", flag.ContinueOnError)
	configPath := flags.String("config", "config.yaml", "path to config.yaml")
	outPath := flags.String("out", "", "write the JSON migration report artifact to this path")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "verify-secrets: %v\n", err)
		return 1
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify-secrets: load config: %v\n", err)
		return 1
	}
	util.ConfigureCredentialKey(cfg.Security.CredentialKey)
	if err := util.ConfigureSecretMasterKeys(os.Getenv("OPS_SECRET_MASTER_KEYS")); err != nil {
		fmt.Fprintf(os.Stderr, "verify-secrets: parse OPS_SECRET_MASTER_KEYS: %v\n", err)
		return 1
	}
	db, err := store.NewDB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify-secrets: connect db: %v\n", err)
		return 1
	}
	if sqlDB, err := db.DB(); err == nil {
		defer sqlDB.Close()
	}
	report, err := verifySecretsInDB(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify-secrets: %v\n", err)
		return 1
	}
	if *outPath != "" {
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "verify-secrets: %v\n", err)
			return 1
		}
		if err := os.WriteFile(*outPath, append(encoded, '\n'), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "verify-secrets: write report artifact: %v\n", err)
			return 1
		}
	}
	fmt.Print(renderSecretVerificationText(report))
	if !report.Passed {
		return 1
	}
	return 0
}

// verifySecretsInDB re-runs the shared classifier over every registered field
// and spot-decrypts a stride sample of the v2 rows per field.
func verifySecretsInDB(db *gorm.DB) (secretVerificationReport, error) {
	report := secretVerificationReport{Fields: []secretVerificationField{}, Samples: []secretVerifySample{}}
	for _, field := range util.SecretFields {
		if !db.Migrator().HasColumn(field.Table, field.Column) {
			report.Fields = append(report.Fields, secretVerificationField{Model: field.Model, Table: field.Table, Column: field.Column, Missing: true})
			continue
		}
		var (
			counts   util.FieldCounts
			unknowns []secretInventoryUnknown
			err      error
		)
		if field.MixedDeclaration {
			counts, unknowns, err = scanScheduleTaskVariables(db, field)
		} else {
			counts, unknowns, _, err = scanSecretField(db, field)
		}
		if err != nil {
			return report, fmt.Errorf("scan %s.%s: %w", field.Table, field.Column, err)
		}
		report.Fields = append(report.Fields, secretVerificationField{Model: field.Model, Table: field.Table, Column: field.Column, Counts: counts})
		report.Total = addFieldCounts(report.Total, counts)
		if len(unknowns) > 0 {
			if report.GateFailure == "" {
				report.GateFailure = fmt.Sprintf("%s.%s holds %d UNKNOWN row(s)", field.Table, field.Column, len(unknowns))
			}
		}
		samples, err := spotDecryptField(db, field)
		if err != nil {
			return report, fmt.Errorf("sample %s.%s: %w", field.Table, field.Column, err)
		}
		report.Samples = append(report.Samples, samples...)
		report.SampledRows += len(samples)
	}
	if report.Total.Legacy > 0 || report.Total.Plaintext > 0 || report.Total.Unknown > 0 {
		if report.GateFailure == "" {
			report.GateFailure = fmt.Sprintf("unmigrated rows remain: legacy=%d plaintext=%d unknown=%d", report.Total.Legacy, report.Total.Plaintext, report.Total.Unknown)
		}
	}
	for _, sample := range report.Samples {
		if !sample.Decrypted {
			report.GateFailure = fmt.Sprintf("spot decrypt failed: %s id=%d field=%s", sample.Table, sample.ID, sample.Field)
			break
		}
	}
	report.Passed = report.GateFailure == ""
	return report, nil
}

// spotDecryptField collects the v2 rows of one field and decrypts a stride
// sample: at least 10% of the rows, at least 500 rows on big tables.
func spotDecryptField(db *gorm.DB, field util.SecretField) ([]secretVerifySample, error) {
	if field.MixedDeclaration {
		return spotDecryptScheduleVariables(db, field)
	}
	var candidates []struct {
		ID    uint
		Value string
	}
	var lastID uint
	for {
		var rows []struct {
			ID    uint
			Value sql.NullString
		}
		if err := db.Table(field.Table).Select("id", field.Column+" AS value").Where("id > ?", lastID).Order("id asc").Limit(inventoryBatchSize).Scan(&rows).Error; err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			lastID = row.ID
			if util.ClassifySecret(row.Value.String, field, true) == util.FormatV2 {
				candidates = append(candidates, struct {
					ID    uint
					Value string
				}{row.ID, row.Value.String})
			}
		}
		if len(rows) < inventoryBatchSize {
			break
		}
	}
	samples := make([]secretVerifySample, 0, secretSampleSize(len(candidates)))
	for _, candidate := range strideSample(len(candidates), secretSampleSize(len(candidates))) {
		_, err := util.DecryptSecretV2(candidates[candidate].Value)
		samples = append(samples, secretVerifySample{
			Table:     field.Table,
			ID:        candidates[candidate].ID,
			Field:     field.Column,
			Decrypted: err == nil,
			Length:    len(candidates[candidate].Value),
		})
	}
	return samples, nil
}

// spotDecryptScheduleVariables spot-decrypts the declared v2 variables of the
// mixed-declaration column with the same sampling floor.
func spotDecryptScheduleVariables(db *gorm.DB, field util.SecretField) ([]secretVerifySample, error) {
	secretNames, err := scriptDeclaredSecretNames(db)
	if err != nil {
		return nil, err
	}
	type candidate struct {
		table, value, name string
		id                 uint
	}
	var candidates []candidate
	var lastID uint
	for {
		var tasks []struct {
			ID        uint
			ScriptID  uint
			Variables sql.NullString
		}
		if err := db.Table(field.Table).Select("id", "script_id", "variables").Where("id > ?", lastID).Order("id asc").Limit(inventoryBatchSize).Scan(&tasks).Error; err != nil {
			return nil, err
		}
		if len(tasks) == 0 {
			break
		}
		for _, task := range tasks {
			lastID = task.ID
			var values map[string]string
			if !task.Variables.Valid || task.Variables.String == "" {
				continue
			}
			if err := json.Unmarshal([]byte(task.Variables.String), &values); err != nil {
				continue
			}
			for name, value := range values {
				gate := secretNames[task.ScriptID][name]
				if util.ClassifySecret(value, field, gate) == util.FormatV2 {
					candidates = append(candidates, candidate{table: field.Table, value: value, name: name, id: task.ID})
				}
			}
		}
		if len(tasks) < inventoryBatchSize {
			break
		}
	}
	samples := make([]secretVerifySample, 0, secretSampleSize(len(candidates)))
	for _, index := range strideSample(len(candidates), secretSampleSize(len(candidates))) {
		_, err := util.DecryptSecretV2(candidates[index].value)
		samples = append(samples, secretVerifySample{
			Table:     candidates[index].table,
			ID:        candidates[index].id,
			Field:     candidates[index].name,
			Decrypted: err == nil,
			Length:    len(candidates[index].value),
		})
	}
	return samples, nil
}

// secretSampleSize implements the §4.4 Step 3 sampling floor: at least 10% of
// the rows, or at least 500 rows on tables big enough, never more than exist.
func secretSampleSize(total int) int {
	if total <= 0 {
		return 0
	}
	size := (total + 9) / 10
	if total >= 500 && size < 500 {
		size = 500
	}
	if size > total {
		size = total
	}
	return size
}

// strideSample returns the deterministic stride selection of size entries out
// of total; the stride keeps the sample spread across the table instead of
// clustering on the first rows.
func strideSample(total, size int) []int {
	if size <= 0 || total <= 0 {
		return nil
	}
	if size > total {
		size = total
	}
	indexes := make([]int, 0, size)
	step := total / size
	for index := 0; index < total && len(indexes) < size; index += step {
		indexes = append(indexes, index)
	}
	return indexes
}

// renderSecretVerificationText renders the human-readable Step 3 gate report.
func renderSecretVerificationText(report secretVerificationReport) string {
	var buffer strings.Builder
	if report.Passed {
		buffer.WriteString("Secret verification gate (§4.4 Step 3): PASS\n\n")
	} else {
		buffer.WriteString("Secret verification gate (§4.4 Step 3): FAIL\n\n")
		buffer.WriteString("reason: " + report.GateFailure + "\n\n")
	}
	buffer.WriteString(fmt.Sprintf("total=%d v2=%d legacy=%d plaintext=%d empty=%d unknown=%d not-secret=%d sampled=%d\n\n",
		report.Total.Total, report.Total.V2, report.Total.Legacy, report.Total.Plaintext,
		report.Total.Empty, report.Total.Unknown, report.Total.NotSecret, report.SampledRows))
	for _, field := range report.Fields {
		missing := ""
		if field.Missing {
			missing = " (column missing)"
		}
		fmt.Fprintf(&buffer, "%s.%s%s v2=%d legacy=%d plaintext=%d unknown=%d empty=%d not-secret=%d\n",
			field.Model, field.Column, missing,
			field.Counts.V2, field.Counts.Legacy, field.Counts.Plaintext, field.Counts.Unknown, field.Counts.Empty, field.Counts.NotSecret)
	}
	if len(report.Samples) > 0 {
		buffer.WriteString(fmt.Sprintf("\nspot decrypt samples (%d):\n", len(report.Samples)))
		for _, sample := range report.Samples {
			status := "ok"
			if !sample.Decrypted {
				status = "FAILED"
			}
			fmt.Fprintf(&buffer, "  %s id=%d field=%s length=%d %s\n", sample.Table, sample.ID, sample.Field, sample.Length, status)
		}
	}
	return buffer.String()
}
