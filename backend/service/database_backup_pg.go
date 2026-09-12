package service

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

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
