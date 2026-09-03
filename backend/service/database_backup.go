package service

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"ops-admin/backend/model"

	"github.com/robfig/cron/v3"
)

type DatabaseBackupScheduler struct {
	cron    *cron.Cron
	mu      sync.Mutex
	entries map[uint]cron.EntryID
}

type DatabaseBackupPlanPayload struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	DatabaseID    uint   `json:"databaseId"`
	SchemaName    string `json:"schemaName"`
	CronExpr      string `json:"cronExpr"`
	RetentionDays int    `json:"retentionDays"`
	Status        int    `json:"status"`
	Description   string `json:"description"`
}

type DatabaseManualBackupPayload struct {
	DatabaseID uint   `json:"databaseId"`
	SchemaName string `json:"schemaName"`
	Operator   string `json:"-"`
}

type DatabaseBackupImportPayload struct {
	BackupRecordID uint   `json:"backupRecordId"`
	DatabaseID     uint   `json:"databaseId"`
	SchemaName     string `json:"schemaName"`
	FileName       string `json:"fileName"`
	FileContent    string `json:"fileContent"`
	Confirmed      bool   `json:"confirmed"`
	Operator       string `json:"-"`
	ClientIP       string `json:"-"`
}

func (s *Service) initDatabaseBackupScheduler() {
	s.dbBackupOnce.Do(func() {
		s.dbBackupScheduler = &DatabaseBackupScheduler{
			cron: cron.New(
				cron.WithSeconds(),
				cron.WithLocation(time.Local),
				cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
			),
			entries: map[uint]cron.EntryID{},
		}
		s.dbBackupScheduler.cron.Start()
		var plans []model.DatabaseBackupPlan
		if err := s.db.Where("status = ?", 1).Find(&plans).Error; err == nil {
			for _, plan := range plans {
				_ = s.scheduleDatabaseBackupPlan(plan)
			}
		}
	})
}

func (s *Service) scheduleDatabaseBackupPlan(plan model.DatabaseBackupPlan) error {
	if s.dbBackupScheduler == nil {
		return nil
	}
	s.dbBackupScheduler.mu.Lock()
	defer s.dbBackupScheduler.mu.Unlock()
	if entryID, exists := s.dbBackupScheduler.entries[plan.ID]; exists {
		s.dbBackupScheduler.cron.Remove(entryID)
		delete(s.dbBackupScheduler.entries, plan.ID)
	}
	if plan.Status != 1 {
		return nil
	}
	entryID, err := s.dbBackupScheduler.cron.AddFunc(normalizeCronExpr(plan.CronExpr), func() {
		_, _ = s.RunDatabaseBackup(plan.ID, "schedule", "Scheduled Task")
	})
	if err != nil {
		return err
	}
	s.dbBackupScheduler.entries[plan.ID] = entryID
	entry := s.dbBackupScheduler.cron.Entry(entryID)
	next := entry.Next
	return s.db.Model(&model.DatabaseBackupPlan{}).Where("id = ?", plan.ID).Update("next_run_at", &next).Error
}

func (s *Service) ListDatabaseBackupPlans(pageNum, pageSize int, keyword, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.DatabaseBackupPlan{})
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR database_name LIKE ? OR schema_name LIKE ?", like, like, like)
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.DatabaseBackupPlan
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) SaveDatabaseBackupPlan(payload DatabaseBackupPlanPayload) error {
	if strings.TrimSpace(payload.Name) == "" || payload.DatabaseID == 0 {
		return errors.New("enter a plan name and select a database")
	}
	cronExpr := normalizeCronExpr(payload.CronExpr)
	if _, err := parseCronExpr(cronExpr); err != nil {
		return errors.New("invalid Cron expression format")
	}
	asset, err := s.getAssetDatabase(payload.DatabaseID)
	if err != nil {
		return err
	}
	if err := ensureLogicalBackupFeature(asset); err != nil {
		return err
	}
	schemaName, err := resolveBackupSchema(asset, payload.SchemaName)
	if err != nil {
		return err
	}
	retentionDays := payload.RetentionDays
	if retentionDays < 1 {
		retentionDays = 7
	}
	status := payload.Status
	if status == 0 {
		status = 1
	}
	item := model.DatabaseBackupPlan{
		ID:            payload.ID,
		Name:          strings.TrimSpace(payload.Name),
		DatabaseID:    asset.ID,
		DatabaseName:  asset.Name,
		SchemaName:    schemaName,
		CronExpr:      cronExpr,
		RetentionDays: retentionDays,
		Status:        status,
		Description:   strings.TrimSpace(payload.Description),
	}
	if payload.ID > 0 {
		if err := s.db.Model(&model.DatabaseBackupPlan{}).Where("id = ?", payload.ID).Updates(item).Error; err != nil {
			return err
		}
	} else if err := s.db.Create(&item).Error; err != nil {
		return err
	}
	return s.scheduleDatabaseBackupPlan(item)
}

func (s *Service) DeleteDatabaseBackupPlan(id uint) error {
	if id == 0 {
		return errors.New("select a backup plan")
	}
	if s.dbBackupScheduler != nil {
		s.dbBackupScheduler.mu.Lock()
		if entryID, exists := s.dbBackupScheduler.entries[id]; exists {
			s.dbBackupScheduler.cron.Remove(entryID)
			delete(s.dbBackupScheduler.entries, id)
		}
		s.dbBackupScheduler.mu.Unlock()
	}
	return s.db.Delete(&model.DatabaseBackupPlan{}, id).Error
}

func (s *Service) RunDatabaseBackup(planID uint, triggerType, operator string) (map[string]any, error) {
	var plan model.DatabaseBackupPlan
	if planID > 0 {
		if err := s.db.First(&plan, planID).Error; err != nil {
			return nil, err
		}
	}
	return s.createDatabaseBackupRecord(plan, plan.DatabaseID, plan.SchemaName, triggerType, operator)
}

func (s *Service) RunManualDatabaseBackup(payload DatabaseManualBackupPayload) (map[string]any, error) {
	if strings.TrimSpace(payload.SchemaName) == "" {
		return nil, errors.New("select a business database to back up")
	}
	return s.createDatabaseBackupRecord(model.DatabaseBackupPlan{}, payload.DatabaseID, payload.SchemaName, "manual", payload.Operator)
}

func (s *Service) createDatabaseBackupRecord(plan model.DatabaseBackupPlan, databaseID uint, schema, triggerType, operator string) (map[string]any, error) {
	if databaseID == 0 {
		return nil, errors.New("select a database to back up")
	}
	asset, err := s.getAssetDatabase(databaseID)
	if err != nil {
		return nil, err
	}
	if err := ensureLogicalBackupFeature(asset); err != nil {
		return nil, err
	}
	schema, err = resolveBackupSchema(asset, schema)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	record := model.DatabaseBackupRecord{
		PlanID:       plan.ID,
		PlanName:     plan.Name,
		DatabaseID:   asset.ID,
		DatabaseName: asset.Name,
		SchemaName:   schema,
		TriggerType:  triggerType,
		Status:       "running",
		Operator:     operator,
		StartedAt:    &now,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return nil, err
	}
	go s.executeDatabaseBackup(record.ID, plan)
	return map[string]any{"recordId": record.ID}, nil
}

func (s *Service) executeDatabaseBackup(recordID uint, plan model.DatabaseBackupPlan) {
	var record model.DatabaseBackupRecord
	if err := s.db.First(&record, recordID).Error; err != nil {
		return
	}
	content, filename, backupMethod, err := s.exportDatabaseBackup(record.DatabaseID, record.SchemaName)
	finishedAt := time.Now()
	updates := map[string]any{"finished_at": &finishedAt}
	if err != nil {
		updates["status"] = "failed"
		updates["message"] = err.Error()
	} else {
		updates["status"] = "success"
		updates["message"] = "Backup completed (" + backupMethod + ")"
		updates["file_name"] = filename
		updates["file_content"] = string(content)
		updates["file_size"] = len(content)
	}
	_ = s.db.Model(&model.DatabaseBackupRecord{}).Where("id = ?", recordID).Updates(updates).Error
	if plan.ID > 0 {
		_ = s.db.Model(&model.DatabaseBackupPlan{}).Where("id = ?", plan.ID).Update("last_run_at", &finishedAt).Error
		if plan.RetentionDays > 0 {
			expiredAt := time.Now().Add(-time.Duration(plan.RetentionDays) * 24 * time.Hour)
			_ = s.db.Where("plan_id = ? AND created_at < ?", plan.ID, expiredAt).Delete(&model.DatabaseBackupRecord{}).Error
		}
		var refreshed model.DatabaseBackupPlan
		if s.db.First(&refreshed, plan.ID).Error == nil {
			_ = s.scheduleDatabaseBackupPlan(refreshed)
		}
	}
}

func (s *Service) exportDatabaseBackup(databaseID uint, schema string) ([]byte, string, string, error) {
	asset, err := s.getAssetDatabase(databaseID)
	if err != nil {
		return nil, "", "", err
	}
	schema, err = resolveBackupSchema(asset, schema)
	if err != nil {
		return nil, "", "", err
	}
	if err := ensureLogicalBackupFeature(asset); err != nil {
		return nil, "", "", err
	}
	_, db, cleanup, err := s.openDatabaseByID(databaseID, schema)
	if err != nil {
		return nil, "", "", err
	}
	defer db.Close()
	defer cleanup()
	var content []byte
	var rowCount int64
	var backupMethod string
	switch normalizeDatabaseType(asset.DBType) {
	case "postgresql":
		content, rowCount, err = exportPostgreSQLLogicalBackup(db, schema)
		backupMethod = fmt.Sprintf("built-in PostgreSQL logical backup, %d rows", rowCount)
	default:
		content, rowCount, err = exportMySQLLogicalBackup(db, schema)
		backupMethod = fmt.Sprintf("built-in MySQL logical backup, %d rows", rowCount)
	}
	if err != nil {
		return nil, "", "", err
	}
	filenamePrefix := schema
	if normalizeDatabaseType(asset.DBType) == "postgresql" && strings.TrimSpace(asset.DBName) != "" {
		filenamePrefix = strings.TrimSpace(asset.DBName) + "_" + schema
	}
	filename := fmt.Sprintf("%s_%s.sql", sanitizeBackupFilename(filenamePrefix), time.Now().Format("20060102_150405"))
	return content, filename, backupMethod, nil
}

func resolveBackupSchema(asset *model.AssetDatabase, schema string) (string, error) {
	name := strings.TrimSpace(schema)
	if name == "" {
		name = strings.TrimSpace(asset.DBName)
	}
	if name == "" {
		return "", errors.New("database connection has no default database; select a business database to back up")
	}
	if normalizeDatabaseType(asset.DBType) == "postgresql" {
		lowerName := strings.ToLower(name)
		if lowerName == "pg_catalog" || lowerName == "information_schema" || strings.HasPrefix(lowerName, "pg_") {
			return "", errors.New("PostgreSQL system schemas cannot be backed up; select a business schema")
		}
		return name, nil
	}
	switch strings.ToLower(name) {
	case "mysql", "sys", "information_schema", "performance_schema":
		return "", errors.New("MySQL system databases cannot be backed up; select a business database")
	}
	return name, nil
}

func ensureLogicalBackupFeature(asset *model.AssetDatabase) error {
	switch normalizeDatabaseType(asset.DBType) {
	case "mysql", "postgresql":
		return nil
	default:
		return fmt.Errorf("%s does not support logical backup; supported engines are MySQL and PostgreSQL", databaseTypeDisplayName(asset.DBType))
	}
}

func exportMySQLLogicalBackup(db *sql.DB, schema string) ([]byte, int64, error) {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create a consistent backup snapshot: %w", err)
	}
	defer tx.Rollback()

	var builder strings.Builder
	builder.Grow(64 * 1024)
	builder.WriteString("-- Ops Admin MySQL Logical Backup\n")
	builder.WriteString("-- Schema: " + schema + "\n")
	builder.WriteString("-- Generated at: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
	builder.WriteString("-- Includes: schema, data, views, triggers, routines and events\n\n")
	builder.WriteString("SET NAMES utf8mb4;\n")
	builder.WriteString("SET FOREIGN_KEY_CHECKS=0;\n\n")

	if createDatabase, err := showCreateStatement(tx, "SHOW CREATE DATABASE "+quoteIdentifier(schema), "Create Database"); err == nil && createDatabase != "" {
		builder.WriteString(createDatabase)
		builder.WriteString(";\n")
	}
	builder.WriteString("USE " + quoteIdentifier(schema) + ";\n\n")

	tables, err := listSchemaObjects(tx, schema, "BASE TABLE")
	if err != nil {
		return nil, 0, err
	}
	var rowCount int64
	for _, table := range tables {
		qualified := quoteIdentifier(schema) + "." + quoteIdentifier(table)
		createSQL, err := showCreateStatement(tx, "SHOW CREATE TABLE "+qualified, "Create Table")
		if err != nil {
			return nil, rowCount, fmt.Errorf("failed to read the CREATE TABLE statement for %s: %w", table, err)
		}
		builder.WriteString("-- Table structure: " + table + "\n")
		builder.WriteString("DROP TABLE IF EXISTS " + qualified + ";\n")
		builder.WriteString(createSQL + ";\n\n")
		rows, err := writeTableData(tx, &builder, schema, table)
		if err != nil {
			return nil, rowCount, err
		}
		rowCount += rows
	}

	views, err := listSchemaObjects(tx, schema, "VIEW")
	if err != nil {
		return nil, rowCount, err
	}
	for _, view := range views {
		qualified := quoteIdentifier(schema) + "." + quoteIdentifier(view)
		createSQL, err := showCreateStatement(tx, "SHOW CREATE VIEW "+qualified, "Create View")
		if err != nil {
			return nil, rowCount, fmt.Errorf("failed to read view definition for %s: %w", view, err)
		}
		builder.WriteString("-- View: " + view + "\n")
		builder.WriteString("DROP VIEW IF EXISTS " + qualified + ";\n")
		builder.WriteString(createSQL + ";\n\n")
	}
	if err := writeTriggers(tx, &builder, schema); err != nil {
		return nil, rowCount, err
	}
	if err := writeRoutines(tx, &builder, schema); err != nil {
		return nil, rowCount, err
	}
	if err := writeEvents(tx, &builder, schema); err != nil {
		return nil, rowCount, err
	}
	builder.WriteString("SET FOREIGN_KEY_CHECKS=1;\n")
	builder.WriteString("-- Backup completed. Rows exported: " + strconv.FormatInt(rowCount, 10) + "\n")
	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("failed to commit backup snapshot: %w", err)
	}
	return []byte(builder.String()), rowCount, nil
}

// exportPostgreSQLLogicalBackup creates a portable SQL archive without relying
// on pg_dump. It intentionally stores table definitions and rows in one file so
// the archive can be restored by the platform backup-import task.
func exportPostgreSQLLogicalBackup(db *sql.DB, schema string) ([]byte, int64, error) {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create a consistent backup snapshot: %w", err)
	}
	defer tx.Rollback()

	var builder strings.Builder
	builder.Grow(64 * 1024)
	builder.WriteString("-- Ops Admin PostgreSQL Logical Backup\n")
	builder.WriteString("-- Schema: " + schema + "\n")
	builder.WriteString("-- Generated at: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
	builder.WriteString("-- Includes: table structure and data\n\n")
	builder.WriteString("BEGIN;\n")
	builder.WriteString("CREATE SCHEMA IF NOT EXISTS " + postgresQuoteIdentifier(schema) + ";\n\n")

	tables, err := listPostgreSQLBackupTables(tx, schema)
	if err != nil {
		return nil, 0, err
	}
	var rowCount int64
	for _, table := range tables {
		if err := writePostgreSQLTableDefinition(tx, &builder, schema, table); err != nil {
			return nil, rowCount, err
		}
		rows, err := writePostgreSQLTableData(tx, &builder, schema, table)
		if err != nil {
			return nil, rowCount, err
		}
		rowCount += rows
	}
	builder.WriteString("COMMIT;\n")
	builder.WriteString("-- Backup completed. Rows exported: " + strconv.FormatInt(rowCount, 10) + "\n")
	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("failed to commit backup snapshot: %w", err)
	}
	return []byte(builder.String()), rowCount, nil
}

func listPostgreSQLBackupTables(tx *sql.Tx, schema string) ([]string, error) {
	rows, err := tx.Query(`
		SELECT c.relname
		FROM pg_class AS c
		JOIN pg_namespace AS n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind IN ('r', 'p')
		ORDER BY c.relname`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tables := make([]string, 0)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

type postgresBackupColumn struct {
	name       string
	typeName   string
	notNull    bool
	defaultSQL string
	primaryKey bool
}

func postgresBackupColumns(tx *sql.Tx, schema, table string) ([]postgresBackupColumn, error) {
	rows, err := tx.Query(`
		SELECT
			a.attname,
			pg_catalog.format_type(a.atttypid, a.atttypmod),
			a.attnotnull,
			COALESCE(pg_get_expr(ad.adbin, ad.adrelid), ''),
			EXISTS (
				SELECT 1 FROM pg_index AS i
				WHERE i.indrelid = a.attrelid AND i.indisprimary AND a.attnum = ANY(i.indkey)
			)
		FROM pg_attribute AS a
		JOIN pg_class AS c ON c.oid = a.attrelid
		JOIN pg_namespace AS n ON n.oid = c.relnamespace
		LEFT JOIN pg_attrdef AS ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
		WHERE n.nspname = $1 AND c.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make([]postgresBackupColumn, 0)
	for rows.Next() {
		var column postgresBackupColumn
		if err := rows.Scan(&column.name, &column.typeName, &column.notNull, &column.defaultSQL, &column.primaryKey); err != nil {
			return nil, err
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func writePostgreSQLTableDefinition(tx *sql.Tx, builder *strings.Builder, schema, table string) error {
	columns, err := postgresBackupColumns(tx, schema, table)
	if err != nil {
		return fmt.Errorf("failed to read column definitions for table %s: %w", table, err)
	}
	if len(columns) == 0 {
		return fmt.Errorf("table %s has no columns available for backup", table)
	}
	qualified := postgresTableName(schema, table)
	definitions := make([]string, 0, len(columns)+1)
	primaryKeys := make([]string, 0)
	for _, column := range columns {
		definition := postgresQuoteIdentifier(column.name) + " " + column.typeName
		if column.defaultSQL != "" {
			definition += " DEFAULT " + column.defaultSQL
		}
		if column.notNull {
			definition += " NOT NULL"
		}
		definitions = append(definitions, definition)
		if column.primaryKey {
			primaryKeys = append(primaryKeys, postgresQuoteIdentifier(column.name))
		}
	}
	if len(primaryKeys) > 0 {
		definitions = append(definitions, "PRIMARY KEY ("+strings.Join(primaryKeys, ", ")+")")
	}
	builder.WriteString("-- Table structure: " + table + "\n")
	builder.WriteString("DROP TABLE IF EXISTS " + qualified + ";\n")
	builder.WriteString("CREATE TABLE " + qualified + " (\n  " + strings.Join(definitions, ",\n  ") + "\n);\n\n")
	return nil
}

func writePostgreSQLTableData(tx *sql.Tx, builder *strings.Builder, schema, table string) (int64, error) {
	qualified := postgresTableName(schema, table)
	rows, err := tx.Query("SELECT * FROM " + qualified)
	if err != nil {
		return 0, fmt.Errorf("failed to read data from table %s: %w", table, err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return 0, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return 0, err
	}
	values := make([]any, len(columns))
	pointers := make([]any, len(columns))
	for i := range values {
		pointers[i] = &values[i]
	}
	var total int64
	batch := make([]string, 0, 200)
	batchBytes := 0
	flush := func() {
		if len(batch) == 0 {
			return
		}
		builder.WriteString("INSERT INTO " + qualified + " (" + postgresJoinIdentifiers(columns) + ") VALUES\n")
		builder.WriteString(strings.Join(batch, ",\n"))
		builder.WriteString(";\n")
		batch = batch[:0]
		batchBytes = 0
	}
	for rows.Next() {
		if err := rows.Scan(pointers...); err != nil {
			return total, err
		}
		items := make([]string, len(values))
		for i, value := range values {
			items[i] = postgresDumpLiteral(value, types[i])
		}
		entry := "(" + strings.Join(items, ", ") + ")"
		if len(batch) >= 200 || batchBytes+len(entry) > 1024*1024 {
			flush()
		}
		batch = append(batch, entry)
		batchBytes += len(entry)
		total++
	}
	flush()
	if err := rows.Err(); err != nil {
		return total, err
	}
	if total > 0 {
		builder.WriteString("\n")
	}
	return total, nil
}

func postgresDumpLiteral(value any, columnType *sql.ColumnType) string {
	if value == nil {
		return "NULL"
	}
	typeName := ""
	if columnType != nil {
		typeName = strings.ToUpper(columnType.DatabaseTypeName())
	}
	switch value := value.(type) {
	case []byte:
		if strings.Contains(typeName, "BYTEA") {
			return "'\\x" + hex.EncodeToString(value) + "'"
		}
		return postgresQuotedString(string(value))
	case time.Time:
		return postgresQuotedString(value.Format(time.RFC3339Nano))
	case bool:
		if value {
			return "TRUE"
		}
		return "FALSE"
	case float32:
		return strconv.FormatFloat(float64(value), 'g', -1, 32)
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return "NULL"
		}
		return strconv.FormatFloat(value, 'g', -1, 64)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(value)
	default:
		return postgresQuotedString(fmt.Sprint(value))
	}
}

func postgresQuotedString(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func listSchemaObjects(tx *sql.Tx, schema, tableType string) ([]string, error) {
	rows, err := tx.Query(`SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = ? ORDER BY TABLE_NAME`, schema, tableType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		items = append(items, name)
	}
	return items, rows.Err()
}

func showCreateStatement(tx *sql.Tx, query, key string) (string, error) {
	rows, err := tx.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	_, data, err := scanRows(rows)
	if err != nil || len(data) == 0 {
		if err != nil {
			return "", err
		}
		return "", errors.New("no CREATE DATABASE or CREATE TABLE statement was returned")
	}
	value, ok := data[0][key]
	if !ok {
		return "", fmt.Errorf("column %s was not found", key)
	}
	return fmt.Sprint(value), nil
}

func writeTableData(tx *sql.Tx, builder *strings.Builder, schema, table string) (int64, error) {
	qualified := quoteIdentifier(schema) + "." + quoteIdentifier(table)
	rows, err := tx.Query("SELECT * FROM " + qualified)
	if err != nil {
		return 0, fmt.Errorf("failed to read data from table %s: %w", table, err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return 0, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return 0, err
	}
	if len(columns) == 0 {
		return 0, nil
	}
	columnNames := joinIdentifiers(columns)
	values := make([]any, len(columns))
	pointers := make([]any, len(columns))
	for i := range values {
		pointers[i] = &values[i]
	}
	var total int64
	batch := make([]string, 0, 200)
	batchBytes := 0
	flush := func() {
		if len(batch) == 0 {
			return
		}
		builder.WriteString("INSERT INTO " + qualified + " (" + columnNames + ") VALUES\n")
		builder.WriteString(strings.Join(batch, ",\n"))
		builder.WriteString(";\n")
		batch = batch[:0]
		batchBytes = 0
	}
	for rows.Next() {
		if err := rows.Scan(pointers...); err != nil {
			return total, err
		}
		items := make([]string, len(values))
		for i, value := range values {
			items[i] = mysqlDumpLiteral(value, types[i])
		}
		entry := "(" + strings.Join(items, ", ") + ")"
		if len(batch) >= 200 || batchBytes+len(entry) > 1024*1024 {
			flush()
		}
		batch = append(batch, entry)
		batchBytes += len(entry)
		total++
	}
	flush()
	if err := rows.Err(); err != nil {
		return total, err
	}
	if total > 0 {
		builder.WriteString("\n")
	}
	return total, nil
}

func mysqlDumpLiteral(value any, columnType *sql.ColumnType) string {
	if value == nil {
		return "NULL"
	}
	typeName := ""
	if columnType != nil {
		typeName = strings.ToUpper(columnType.DatabaseTypeName())
	}
	switch value := value.(type) {
	case []byte:
		if isBinaryMySQLType(typeName) {
			return "X'" + hex.EncodeToString(value) + "'"
		}
		if isNumericMySQLType(typeName) {
			return string(value)
		}
		return mysqlQuotedString(string(value))
	case time.Time:
		return mysqlQuotedString(value.Format("2006-01-02 15:04:05.999999"))
	case bool:
		if value {
			return "1"
		}
		return "0"
	case float32:
		return strconv.FormatFloat(float64(value), 'g', -1, 32)
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return "NULL"
		}
		return strconv.FormatFloat(value, 'g', -1, 64)
	case int:
		return strconv.Itoa(value)
	case int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(value)
	case string:
		if isNumericMySQLType(typeName) {
			return value
		}
		return mysqlQuotedString(value)
	default:
		return mysqlQuotedString(fmt.Sprint(value))
	}
}

func mysqlQuotedString(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "'", "\\'", "\x00", "\\0", "\n", "\\n", "\r", "\\r", "\x1a", "\\Z")
	return "'" + replacer.Replace(value) + "'"
}

func isBinaryMySQLType(typeName string) bool {
	return strings.Contains(typeName, "BLOB") || strings.Contains(typeName, "BINARY") || typeName == "BIT" || typeName == "GEOMETRY"
}

func isNumericMySQLType(typeName string) bool {
	for _, kind := range []string{"INT", "DECIMAL", "NUMERIC", "FLOAT", "DOUBLE", "REAL", "YEAR"} {
		if strings.Contains(typeName, kind) {
			return true
		}
	}
	return false
}

func writeTriggers(tx *sql.Tx, builder *strings.Builder, schema string) error {
	rows, err := tx.Query(`SELECT TRIGGER_NAME FROM INFORMATION_SCHEMA.TRIGGERS WHERE TRIGGER_SCHEMA = ? ORDER BY TRIGGER_NAME`, schema)
	if err != nil {
		return fmt.Errorf("failed to read trigger list: %w", err)
	}
	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, name := range names {
		qualified := quoteIdentifier(schema) + "." + quoteIdentifier(name)
		createSQL, err := showCreateStatement(tx, "SHOW CREATE TRIGGER "+qualified, "SQL Original Statement")
		if err != nil {
			return fmt.Errorf("failed to read trigger %s: %w", name, err)
		}
		writeDelimiterBlock(builder, "TRIGGER", qualified, createSQL)
	}
	return nil
}

func writeRoutines(tx *sql.Tx, builder *strings.Builder, schema string) error {
	rows, err := tx.Query(`SELECT ROUTINE_NAME, ROUTINE_TYPE FROM INFORMATION_SCHEMA.ROUTINES WHERE ROUTINE_SCHEMA = ? ORDER BY ROUTINE_TYPE, ROUTINE_NAME`, schema)
	if err != nil {
		return fmt.Errorf("failed to read stored-routine list: %w", err)
	}
	type routine struct {
		name string
		kind string
	}
	routines := make([]routine, 0)
	for rows.Next() {
		var name, routineType string
		if err := rows.Scan(&name, &routineType); err != nil {
			rows.Close()
			return err
		}
		routines = append(routines, routine{name: name, kind: routineType})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, routine := range routines {
		name, routineType := routine.name, routine.kind
		routineType = strings.ToUpper(routineType)
		qualified := quoteIdentifier(schema) + "." + quoteIdentifier(name)
		createColumn := "Create Procedure"
		if routineType == "FUNCTION" {
			createColumn = "Create Function"
		}
		createSQL, err := showCreateStatement(tx, "SHOW CREATE "+routineType+" "+qualified, createColumn)
		if err != nil {
			return fmt.Errorf("failed to read %s %s: %w", routineType, name, err)
		}
		writeDelimiterBlock(builder, routineType, qualified, createSQL)
	}
	return nil
}

func writeEvents(tx *sql.Tx, builder *strings.Builder, schema string) error {
	rows, err := tx.Query(`SELECT EVENT_NAME FROM INFORMATION_SCHEMA.EVENTS WHERE EVENT_SCHEMA = ? ORDER BY EVENT_NAME`, schema)
	if err != nil {
		return fmt.Errorf("failed to read event list: %w", err)
	}
	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, name := range names {
		qualified := quoteIdentifier(schema) + "." + quoteIdentifier(name)
		createSQL, err := showCreateStatement(tx, "SHOW CREATE EVENT "+qualified, "Create Event")
		if err != nil {
			return fmt.Errorf("failed to read event %s: %w", name, err)
		}
		writeDelimiterBlock(builder, "EVENT", qualified, createSQL)
	}
	return nil
}

func writeDelimiterBlock(builder *strings.Builder, objectType, qualifiedName, createSQL string) {
	builder.WriteString("DELIMITER $$\n")
	builder.WriteString("DROP " + objectType + " IF EXISTS " + qualifiedName + "$$\n")
	builder.WriteString(createSQL + "$$\n")
	builder.WriteString("DELIMITER ;\n\n")
}

func sanitizeBackupFilename(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, value)
	if value == "" {
		return "database"
	}
	return value
}

func (s *Service) ListDatabaseBackupRecords(pageNum, pageSize int, databaseID uint, status, triggerType string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.DatabaseBackupRecord{})
	if databaseID > 0 {
		query = query.Where("database_id = ?", databaseID)
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}
	if strings.TrimSpace(triggerType) != "" {
		query = query.Where("trigger_type = ?", triggerType)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.DatabaseBackupRecord
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) GetDatabaseBackupFile(id uint) ([]byte, string, error) {
	var record model.DatabaseBackupRecord
	if err := s.db.First(&record, id).Error; err != nil {
		return nil, "", err
	}
	if record.Status != "success" || record.FileContent == "" {
		return nil, "", errors.New("backup file is not available for download")
	}
	return []byte(record.FileContent), record.FileName, nil
}

func (s *Service) CreateDatabaseBackupImportTask(payload DatabaseBackupImportPayload) (map[string]any, error) {
	if payload.DatabaseID == 0 || strings.TrimSpace(payload.SchemaName) == "" {
		return nil, errors.New("select a target database connection and schema")
	}
	if !payload.Confirmed {
		return nil, errors.New("backup restoration requires risk confirmation")
	}
	asset, err := s.getAssetDatabase(payload.DatabaseID)
	if err != nil {
		return nil, err
	}
	if err := ensureLogicalBackupFeature(asset); err != nil {
		return nil, err
	}
	if err := ensureDatabaseWritable(asset); err != nil {
		return nil, err
	}

	fileName := strings.TrimSpace(payload.FileName)
	content := strings.TrimSpace(payload.FileContent)
	if payload.BackupRecordID > 0 {
		var record model.DatabaseBackupRecord
		if err := s.db.First(&record, payload.BackupRecordID).Error; err != nil {
			return nil, errors.New("selected backup record does not exist")
		}
		if record.Status != "success" || strings.TrimSpace(record.FileContent) == "" {
			return nil, errors.New("selected backup did not succeed or contains no backup data")
		}
		sourceAsset, sourceErr := s.getAssetDatabase(record.DatabaseID)
		if sourceErr != nil {
			return nil, sourceErr
		}
		if normalizeDatabaseType(sourceAsset.DBType) != normalizeDatabaseType(asset.DBType) {
			return nil, errors.New("source and target database types differ; cross-engine restoration is not allowed")
		}
		fileName = record.FileName
		content = record.FileContent
	}
	if content == "" {
		return nil, errors.New("select a platform backup or upload an SQL backup file")
	}
	if len(content) > 50*1024*1024 {
		return nil, errors.New("backup file must not exceed 50 MB")
	}
	if fileName == "" {
		fileName = "database-backup.sql"
	}
	if !strings.HasSuffix(strings.ToLower(fileName), ".sql") {
		return nil, errors.New("backup import supports only .sql files")
	}

	task := model.DatabaseTransferTask{
		TaskType:      "backup_import",
		Status:        "pending",
		Progress:      0,
		Message:       "Pending backup restoration",
		DatabaseID:    asset.ID,
		DatabaseName:  asset.Name,
		SchemaName:    strings.TrimSpace(payload.SchemaName),
		FileName:      fileName,
		FileContent:   content,
		ExecutionMode: "sequential",
		Operator:      payload.Operator,
	}
	if err := s.db.Create(&task).Error; err != nil {
		return nil, err
	}
	go s.runDatabaseBackupImportTask(task.ID, payload.ClientIP)
	return map[string]any{"taskId": task.ID}, nil
}

func (s *Service) runDatabaseBackupImportTask(taskID uint, clientIP string) {
	var task model.DatabaseTransferTask
	if err := s.db.First(&task, taskID).Error; err != nil {
		return
	}
	startedAt := time.Now()
	s.updateTransferTask(task.ID, map[string]any{
		"status": "running", "progress": 3, "message": "Parsing backup file", "started_at": &startedAt,
	})

	asset, db, cleanup, err := s.openDatabaseByID(task.DatabaseID, task.SchemaName)
	if err != nil {
		s.finishDatabaseBackupImportTask(task, startedAt, 0, err, clientIP)
		return
	}
	defer db.Close()
	defer cleanup()

	script := task.FileContent
	if normalizeDatabaseType(asset.DBType) == "postgresql" {
		script = normalizePostgreSQLRestoreScript(script, task.SchemaName)
	} else {
		script = normalizeDatabaseRestoreScript(script, task.SchemaName)
	}
	statements := splitMySQLRestoreStatements(script)
	if len(statements) == 0 {
		s.finishDatabaseBackupImportTask(task, startedAt, 0, errors.New("backup file contains no executable SQL"), clientIP)
		return
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		s.finishDatabaseBackupImportTask(task, startedAt, 0, err, clientIP)
		return
	}
	defer conn.Close()

	var rowsAffected int64
	for index, statement := range statements {
		result, execErr := conn.ExecContext(context.Background(), statement)
		if execErr != nil {
			err = fmt.Errorf("SQL restore statement %d/%d failed: %w", index+1, len(statements), execErr)
			break
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr == nil {
			rowsAffected += affected
		}
		progress := 5 + int(float64(index+1)/float64(len(statements))*90)
		s.updateTransferTask(task.ID, map[string]any{
			"progress": progress,
			"message":  fmt.Sprintf("Restoring %d/%d", index+1, len(statements)),
		})
	}
	if asset != nil {
		status := 1
		errText := ""
		if err != nil {
			status = 2
			errText = err.Error()
		}
		s.logDBSQLHistory(asset, task.SchemaName, "", "RESTORE", "Restore Backup: "+task.FileName, status, rowsAffected, time.Since(startedAt).Milliseconds(), errText, "")
	}
	s.finishDatabaseBackupImportTask(task, startedAt, rowsAffected, err, clientIP)
}

func (s *Service) finishDatabaseBackupImportTask(task model.DatabaseTransferTask, startedAt time.Time, rowsAffected int64, runErr error, _ string) {
	finishedAt := time.Now()
	updates := map[string]any{"finished_at": &finishedAt, "rows_affected": rowsAffected, "progress": 100}
	if runErr != nil {
		updates["status"] = "failed"
		updates["message"] = runErr.Error()
	} else {
		updates["status"] = "success"
		updates["message"] = fmt.Sprintf("backup restoration completed; %d rows affected in %s", rowsAffected, time.Since(startedAt).Round(time.Millisecond))
	}
	s.updateTransferTask(task.ID, updates)
}

func normalizeDatabaseRestoreScript(script, targetSchema string) string {
	targetSchema = strings.TrimSpace(targetSchema)
	sourceSchema := ""
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-- Schema:") {
			sourceSchema = strings.TrimSpace(strings.TrimPrefix(trimmed, "-- Schema:"))
			break
		}
	}
	if sourceSchema != "" && sourceSchema != targetSchema {
		script = strings.ReplaceAll(script, quoteIdentifier(sourceSchema)+".", quoteIdentifier(targetSchema)+".")
	}
	createDatabasePattern := regexp.MustCompile(`(?im)^\s*(CREATE|DROP)\s+DATABASE\b[^;]*;\s*$`)
	usePattern := regexp.MustCompile(`(?im)^\s*USE\s+[^;]+;\s*$`)
	script = createDatabasePattern.ReplaceAllString(script, "")
	script = usePattern.ReplaceAllString(script, "")
	return "USE " + quoteIdentifier(targetSchema) + ";\n" + script
}

func normalizePostgreSQLRestoreScript(script, targetSchema string) string {
	targetSchema = strings.TrimSpace(targetSchema)
	sourceSchema := ""
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-- Schema:") {
			sourceSchema = strings.TrimSpace(strings.TrimPrefix(trimmed, "-- Schema:"))
			break
		}
	}
	if sourceSchema == "" || sourceSchema == targetSchema {
		return script
	}
	script = strings.ReplaceAll(script, postgresQuoteIdentifier(sourceSchema)+".", postgresQuoteIdentifier(targetSchema)+".")
	script = strings.ReplaceAll(script, "CREATE SCHEMA IF NOT EXISTS "+postgresQuoteIdentifier(sourceSchema), "CREATE SCHEMA IF NOT EXISTS "+postgresQuoteIdentifier(targetSchema))
	return script
}

func splitMySQLRestoreStatements(script string) []string {
	delimiter := ";"
	statements := make([]string, 0)
	var current strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(script))
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 64*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "DELIMITER ") {
			delimiter = strings.TrimSpace(trimmed[len("DELIMITER "):])
			if delimiter == "" {
				delimiter = ";"
			}
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
		if strings.HasSuffix(trimmed, delimiter) {
			statement := strings.TrimSpace(current.String())
			statement = strings.TrimSpace(strings.TrimSuffix(statement, delimiter))
			if stripDBMSSQLComments(statement) != "" {
				statements = append(statements, statement)
			}
			current.Reset()
		}
	}
	if statement := strings.TrimSpace(current.String()); stripDBMSSQLComments(statement) != "" {
		statements = append(statements, statement)
	}
	return statements
}
