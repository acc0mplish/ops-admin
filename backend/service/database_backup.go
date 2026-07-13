package service

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
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
		_, _ = s.RunDatabaseBackup(plan.ID, "schedule", "定时任务")
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
		return errors.New("请填写计划名称并选择数据库")
	}
	cronExpr := normalizeCronExpr(payload.CronExpr)
	if _, err := parseCronExpr(cronExpr); err != nil {
		return errors.New("Cron 表达式格式不正确")
	}
	asset, err := s.getAssetDatabase(payload.DatabaseID)
	if err != nil {
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
		return errors.New("请选择备份计划")
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
		return nil, errors.New("请选择需要备份的业务库")
	}
	return s.createDatabaseBackupRecord(model.DatabaseBackupPlan{}, payload.DatabaseID, payload.SchemaName, "manual", payload.Operator)
}

func (s *Service) createDatabaseBackupRecord(plan model.DatabaseBackupPlan, databaseID uint, schema, triggerType, operator string) (map[string]any, error) {
	if databaseID == 0 {
		return nil, errors.New("请选择需要备份的数据库")
	}
	asset, err := s.getAssetDatabase(databaseID)
	if err != nil {
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
		updates["message"] = "备份完成（" + backupMethod + "）"
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
	_, db, cleanup, err := s.openDatabaseByID(databaseID, schema)
	if err != nil {
		return nil, "", "", err
	}
	defer db.Close()
	defer cleanup()
	content, rowCount, err := exportMySQLLogicalBackup(db, schema)
	if err != nil {
		return nil, "", "", err
	}
	filename := fmt.Sprintf("%s_%s.sql", sanitizeBackupFilename(schema), time.Now().Format("20060102_150405"))
	return content, filename, fmt.Sprintf("内置 MySQL 逻辑备份，%d 行数据", rowCount), nil
}

func resolveBackupSchema(asset *model.AssetDatabase, schema string) (string, error) {
	name := strings.TrimSpace(schema)
	if name == "" {
		name = strings.TrimSpace(asset.DBName)
	}
	if name == "" {
		return "", errors.New("数据库连接未设置默认库，请选择需要备份的业务库")
	}
	switch strings.ToLower(name) {
	case "mysql", "sys", "information_schema", "performance_schema":
		return "", errors.New("不允许备份 MySQL 系统库，请选择业务库")
	}
	return name, nil
}

func exportMySQLLogicalBackup(db *sql.DB, schema string) ([]byte, int64, error) {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, 0, fmt.Errorf("创建一致性备份快照失败: %w", err)
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
			return nil, rowCount, fmt.Errorf("读取表 %s 的建表语句失败: %w", table, err)
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
			return nil, rowCount, fmt.Errorf("读取视图 %s 的定义失败: %w", view, err)
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
		return nil, 0, fmt.Errorf("提交备份快照失败: %w", err)
	}
	return []byte(builder.String()), rowCount, nil
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
		return "", errors.New("未返回建库或建表语句")
	}
	value, ok := data[0][key]
	if !ok {
		return "", fmt.Errorf("未找到 %s 字段", key)
	}
	return fmt.Sprint(value), nil
}

func writeTableData(tx *sql.Tx, builder *strings.Builder, schema, table string) (int64, error) {
	qualified := quoteIdentifier(schema) + "." + quoteIdentifier(table)
	rows, err := tx.Query("SELECT * FROM " + qualified)
	if err != nil {
		return 0, fmt.Errorf("读取表 %s 数据失败: %w", table, err)
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
		return fmt.Errorf("读取触发器列表失败: %w", err)
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
			return fmt.Errorf("读取触发器 %s 失败: %w", name, err)
		}
		writeDelimiterBlock(builder, "TRIGGER", qualified, createSQL)
	}
	return nil
}

func writeRoutines(tx *sql.Tx, builder *strings.Builder, schema string) error {
	rows, err := tx.Query(`SELECT ROUTINE_NAME, ROUTINE_TYPE FROM INFORMATION_SCHEMA.ROUTINES WHERE ROUTINE_SCHEMA = ? ORDER BY ROUTINE_TYPE, ROUTINE_NAME`, schema)
	if err != nil {
		return fmt.Errorf("读取存储程序列表失败: %w", err)
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
			return fmt.Errorf("读取%s %s 失败: %w", routineType, name, err)
		}
		writeDelimiterBlock(builder, routineType, qualified, createSQL)
	}
	return nil
}

func writeEvents(tx *sql.Tx, builder *strings.Builder, schema string) error {
	rows, err := tx.Query(`SELECT EVENT_NAME FROM INFORMATION_SCHEMA.EVENTS WHERE EVENT_SCHEMA = ? ORDER BY EVENT_NAME`, schema)
	if err != nil {
		return fmt.Errorf("读取事件列表失败: %w", err)
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
			return fmt.Errorf("读取事件 %s 失败: %w", name, err)
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
		return nil, "", errors.New("备份文件尚不可下载")
	}
	return []byte(record.FileContent), record.FileName, nil
}
