package service

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/model"
)

func defaultSchema(item *model.AssetDatabase, schema string) string {
	value := strings.TrimSpace(schema)
	if value != "" {
		return value
	}
	if normalizeDatabaseType(item.DBType) == "postgresql" {
		return "public"
	}
	if strings.TrimSpace(item.DBName) != "" {
		return strings.TrimSpace(item.DBName)
	}
	if normalizeDatabaseType(item.DBType) == "redis" {
		return "db0"
	}
	return "mysql"
}

func detectSQLType(sqlText string) string {
	fields := strings.Fields(stripDBMSSQLComments(sqlText))
	if len(fields) == 0 {
		return ""
	}
	return strings.ToUpper(fields[0])
}

func isQuerySQL(sqlType string) bool {
	switch sqlType {
	case "SELECT", "SHOW", "DESC", "DESCRIBE", "EXPLAIN", "WITH":
		return true
	default:
		return false
	}
}

func stripDBMSSQLComments(sqlText string) string {
	lineComment := regexp.MustCompile(`(?m)--[^\r\n]*|#[^\r\n]*`)
	blockComment := regexp.MustCompile(`(?s)/\*.*?\*/`)
	return strings.TrimSpace(blockComment.ReplaceAllString(lineComment.ReplaceAllString(sqlText, " "), " "))
}

func splitDBMSSQLStatements(sqlText string) []string {
	statements := make([]string, 0)
	var current strings.Builder
	var quote rune
	escaped := false
	for _, char := range sqlText {
		if escaped {
			current.WriteRune(char)
			escaped = false
			continue
		}
		if char == '\\' && quote != 0 {
			current.WriteRune(char)
			escaped = true
			continue
		}
		if quote != 0 {
			current.WriteRune(char)
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' || char == '`' {
			quote = char
			current.WriteRune(char)
			continue
		}
		if char == ';' {
			if statement := strings.TrimSpace(current.String()); stripDBMSSQLComments(statement) != "" {
				statements = append(statements, statement)
			}
			current.Reset()
			continue
		}
		current.WriteRune(char)
	}
	if statement := strings.TrimSpace(current.String()); stripDBMSSQLComments(statement) != "" {
		statements = append(statements, statement)
	}
	return statements
}

func isReadOnlySQL(sqlText string) bool {
	cleaned := strings.ToUpper(stripDBMSSQLComments(sqlText))
	sqlType := detectSQLType(cleaned)
	if sqlType == "WITH" {
		writePattern := regexp.MustCompile(`\b(INSERT|UPDATE|DELETE|REPLACE|CREATE|ALTER|DROP|TRUNCATE|GRANT|REVOKE|CALL)\b`)
		return !writePattern.MatchString(cleaned)
	}
	return isQuerySQL(sqlType)
}

func newDBMSExecutionID() string {
	now := time.Now()
	return fmt.Sprintf("SQL-%s-%06d", now.Format("20060102150405"), now.Nanosecond()/1000)
}

func scanRows(rows *sql.Rows) ([]string, []map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, nil, err
		}
		item := make(map[string]any, len(columns))
		for i, col := range columns {
			item[col] = normalizeScanValue(values[i])
		}
		result = append(result, item)
	}
	return columns, result, rows.Err()
}

func normalizeScanValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(v)
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	default:
		return v
	}
}

func normalizeJSONValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case float64:
		if v == float64(int64(v)) {
			return int64(v)
		}
		return v
	case string:
		return v
	default:
		return v
	}
}

func buildTableFilterClause(columns []databaseTableColumn, filterKey, filterText string) (string, []any, error) {
	key := strings.TrimSpace(filterKey)
	text := strings.TrimSpace(filterText)
	if text == "" {
		return "", nil, nil
	}
	if key != "" {
		for _, col := range columns {
			if col.Name == key {
				return fmt.Sprintf("%s LIKE ?", quoteIdentifier(key)), []any{"%" + text + "%"}, nil
			}
		}
		return "", nil, errors.New("filter column does not exist")
	}
	parts := make([]string, 0)
	args := make([]any, 0)
	for _, col := range columns {
		parts = append(parts, fmt.Sprintf("%s LIKE ?", quoteIdentifier(col.Name)))
		args = append(args, "%"+text+"%")
	}
	if len(parts) == 0 {
		return "", nil, nil
	}
	return "(" + strings.Join(parts, " OR ") + ")", args, nil
}

func quoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(strings.TrimSpace(name), "`", "``") + "`"
}

func joinIdentifiers(columns []string) string {
	items := make([]string, 0, len(columns))
	for _, col := range columns {
		items = append(items, quoteIdentifier(col))
	}
	return strings.Join(items, ", ")
}

func sqlLiteral(value any) string {
	if value == nil {
		return "NULL"
	}
	switch v := value.(type) {
	case bool:
		if v {
			return "1"
		}
		return "0"
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		text := fmt.Sprintf("%v", v)
		text = strings.ReplaceAll(text, "\\", "\\\\")
		text = strings.ReplaceAll(text, "'", "''")
		return "'" + text + "'"
	}
}

func buildRowWhereClause(columns []databaseTableColumn, row map[string]any) (string, []any) {
	primaryKeys := make([]string, 0)
	for _, col := range columns {
		if strings.EqualFold(col.ColumnKey, "PRI") {
			primaryKeys = append(primaryKeys, col.Name)
		}
	}
	keys := primaryKeys
	if len(keys) == 0 {
		keys = make([]string, 0)
		for _, col := range columns {
			if _, exists := row[col.Name]; exists {
				keys = append(keys, col.Name)
			}
		}
	}
	clauses := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys))
	for _, key := range keys {
		clauses = append(clauses, fmt.Sprintf("%s = ?", quoteIdentifier(key)))
		args = append(args, normalizeJSONValue(row[key]))
	}
	return strings.Join(clauses, " AND "), args
}
