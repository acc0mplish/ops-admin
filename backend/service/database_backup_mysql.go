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
	"time"
)

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
