package service

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/model"

	_ "github.com/go-sql-driver/mysql"
)

type DBMSSQLExecutePayload struct {
	DatabaseID uint   `json:"databaseId"`
	Schema     string `json:"schema"`
	SQLText    string `json:"sqlText"`
	Confirmed  bool   `json:"confirmed"`
	Operator   string `json:"-"`
	ClientIP   string `json:"-"`
}

// DBMSCreateDatabasePayload creates a database on MySQL, or a schema in the
// currently connected PostgreSQL database. PostgreSQL's workbench tree is a
// schema tree, so creating a schema keeps the new object immediately usable.
type DBMSCreateDatabasePayload struct {
	DatabaseID uint   `json:"databaseId"`
	Name       string `json:"name"`
	Charset    string `json:"charset"`
	Collation  string `json:"collation"`
	Operator   string `json:"-"`
}

var mysqlNamedForeignKeyPattern = regexp.MustCompile("(?i)CONSTRAINT\\s+(?:`[^`]+`|[a-zA-Z0-9_]+)\\s+(FOREIGN\\s+KEY)")
var databaseObjectNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)
var mysqlCharsetOptionPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// mysqlImportCreateTableSQL keeps foreign keys during an automatic table copy,
// but drops their source-side names. MySQL requires foreign key names to be
// unique within a schema, so reusing the source name can make the import fail.
func mysqlImportCreateTableSQL(createSQL, sourceTable, targetTable string) string {
	createSQL = strings.Replace(
		createSQL,
		"CREATE TABLE `"+sourceTable+"`",
		"CREATE TABLE IF NOT EXISTS "+quoteIdentifier(targetTable),
		1,
	)
	return mysqlNamedForeignKeyPattern.ReplaceAllString(createSQL, "$1")
}

type DBMSTableDataQueryPayload struct {
	DatabaseID uint   `json:"databaseId"`
	Schema     string `json:"schema"`
	Table      string `json:"table"`
	PageNum    int    `json:"pageNum"`
	PageSize   int    `json:"pageSize"`
	FilterKey  string `json:"filterKey"`
	FilterText string `json:"filterText"`
}

type DBMSTableInsertPayload struct {
	DatabaseID uint           `json:"databaseId"`
	Schema     string         `json:"schema"`
	Table      string         `json:"table"`
	Row        map[string]any `json:"row"`
}

type DBMSTableUpdatePayload struct {
	DatabaseID uint           `json:"databaseId"`
	Schema     string         `json:"schema"`
	Table      string         `json:"table"`
	Original   map[string]any `json:"original"`
	Current    map[string]any `json:"current"`
}

type DBMSTableDeletePayload struct {
	DatabaseID uint           `json:"databaseId"`
	Schema     string         `json:"schema"`
	Table      string         `json:"table"`
	Row        map[string]any `json:"row"`
}

type DBMSImportPayload struct {
	SourceDatabaseID uint   `json:"sourceDatabaseId"`
	SourceSchema     string `json:"sourceSchema"`
	SourceTable      string `json:"sourceTable"`
	TargetDatabaseID uint   `json:"targetDatabaseId"`
	TargetSchema     string `json:"targetSchema"`
	TargetTable      string `json:"targetTable"`
	CreateIfMissing  bool   `json:"createIfMissing"`
	TruncateTarget   bool   `json:"truncateTarget"`
}

type DBMSExportPayload struct {
	DatabaseID  uint   `json:"databaseId"`
	Schema      string `json:"schema"`
	Table       string `json:"table"`
	IncludeData bool   `json:"includeData"`
}

type DBMSBatchSQLPayload struct {
	DatabaseID    uint   `json:"databaseId"`
	Schema        string `json:"schema"`
	SQLText       string `json:"sqlText"`
	FileName      string `json:"fileName"`
	ExecutionMode string `json:"executionMode"`
	Confirmed     bool   `json:"confirmed"`
	Operator      string `json:"-"`
	ClientIP      string `json:"-"`
}

type DBMSImportPrecheck struct {
	SourceDatabase string   `json:"sourceDatabase"`
	SourceSchema   string   `json:"sourceSchema"`
	SourceTable    string   `json:"sourceTable"`
	TargetDatabase string   `json:"targetDatabase"`
	TargetSchema   string   `json:"targetSchema"`
	TargetTable    string   `json:"targetTable"`
	EstimatedRows  int64    `json:"estimatedRows"`
	TargetExists   bool     `json:"targetExists"`
	CommonColumns  []string `json:"commonColumns"`
	MissingColumns []string `json:"missingColumns"`
	Warnings       []string `json:"warnings"`
	Ready          bool     `json:"ready"`
}

type DBMSSQLAnalysis struct {
	SQLType        string   `json:"sqlType"`
	StatementCount int      `json:"statementCount"`
	WriteOperation bool     `json:"writeOperation"`
	RiskLevel      string   `json:"riskLevel"`
	Reasons        []string `json:"reasons"`
	DatabaseName   string   `json:"databaseName"`
	Schema         string   `json:"schema"`
	Environment    string   `json:"environment"`
	AccessMode     string   `json:"accessMode"`
}

type databaseTableColumn struct {
	Name          string `json:"name"`
	DataType      string `json:"dataType"`
	ColumnType    string `json:"columnType"`
	ColumnKey     string `json:"columnKey"`
	IsNullable    string `json:"isNullable"`
	ColumnDefault any    `json:"columnDefault"`
	Extra         string `json:"extra"`
}

func normalizeDatabaseType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "postgres", "postgresql":
		return "postgresql"
	case "mongo", "mongodb":
		return "mongodb"
	case "redis":
		return "redis"
	default:
		return "mysql"
	}
}

func databasePort(v int) int {
	return databasePortByType("mysql", v)
}

func databaseCharset(v string) string {
	value := strings.ToLower(strings.TrimSpace(v))
	if value == "" {
		return "utf8mb4"
	}
	if strings.HasPrefix(value, "utf8mb4_") {
		return "utf8mb4"
	}
	if strings.HasPrefix(value, "utf8_") {
		return "utf8"
	}
	return value
}

func normalizeDatabaseAccessMode(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "readonly") {
		return "readonly"
	}
	return "readwrite"
}

func ensureDatabaseWritable(item *model.AssetDatabase) error {
	if item != nil && normalizeDatabaseAccessMode(item.AccessMode) == "readonly" {
		return errors.New("the current database is read-only; creating, editing, deleting, or importing data is not allowed")
	}
	return nil
}

func databaseColumnsHavePrimaryKey(columns []databaseTableColumn) bool {
	for _, column := range columns {
		if strings.EqualFold(column.ColumnKey, "PRI") {
			return true
		}
	}
	return false
}

func mysqlDSN(host string, port int, user, password, dbName, charset string) string {
	schema := strings.TrimSpace(dbName)
	if schema == "" {
		schema = "mysql"
	}
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&timeout=5s&readTimeout=20s&writeTimeout=20s&multiStatements=true&loc=Local",
		user,
		password,
		host,
		port,
		schema,
		charset,
	)
}

func inspectMySQLDatabase(host string, port int, user, password, dbName, charset string) (string, error) {
	item := model.AssetDatabase{
		Host:     host,
		Port:     port,
		Username: user,
		Password: password,
		DBName:   dbName,
		Charset:  charset,
	}
	db, cleanup, err := openMySQLDatabase(item, "")
	if err != nil {
		return "", err
	}
	defer cleanup()
	defer db.Close()
	if err := db.Ping(); err != nil {
		return "", err
	}
	var version string
	if err := db.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		return "", err
	}
	return version, nil
}

func (s *Service) inspectAssetMySQLDatabase(item model.AssetDatabase) (string, error) {
	db, cleanup, err := s.openAssetMySQLDatabase(item, "")
	if err != nil {
		return "", err
	}
	defer cleanup()
	defer db.Close()
	if err := db.Ping(); err != nil {
		return "", err
	}
	var version string
	if err := db.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		return "", err
	}
	return version, nil
}

func openMySQLDatabase(item model.AssetDatabase, schema string) (*sql.DB, func(), error) {
	db, err := sql.Open("mysql", mysqlDSN(item.Host, databasePort(item.Port), item.Username, item.Password, defaultSchema(&item, schema), databaseCharset(item.Charset)))
	return db, func() {}, err
}

func (s *Service) openAssetMySQLDatabase(item model.AssetDatabase, schema string) (*sql.DB, func(), error) {
	targetHost := strings.TrimSpace(item.Host)
	targetPort := databasePort(item.Port)
	cleanup := func() {}
	if normalizeConnectionMode(item.ConnectionMode) == "gateway" && item.GatewayID != nil && *item.GatewayID > 0 {
		localAddress, tunnelCleanup, err := s.startGatewayTunnel(*item.GatewayID, fmt.Sprintf("%s:%d", targetHost, targetPort))
		if err != nil {
			return nil, cleanup, err
		}
		cleanup = tunnelCleanup
		host, portText, err := net.SplitHostPort(localAddress)
		if err != nil {
			cleanup()
			return nil, func() {}, err
		}
		port, _ := strconv.Atoi(portText)
		item.Host = host
		item.Port = port
	}
	db, err := sql.Open("mysql", mysqlDSN(item.Host, databasePort(item.Port), item.Username, item.Password, defaultSchema(&item, schema), databaseCharset(item.Charset)))
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}
	return db, cleanup, nil
}

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

func (s *Service) buildDeleteRollbackSQL(schema, table string, columns []databaseTableColumn, row map[string]any) string {
	whereSQL, args := buildRowWhereClause(columns, row)
	literals := make([]string, 0, len(args))
	for _, arg := range args {
		literals = append(literals, sqlLiteral(arg))
	}
	parts := strings.Split(whereSQL, "?")
	builder := &strings.Builder{}
	for index, part := range parts {
		builder.WriteString(part)
		if index < len(literals) {
			builder.WriteString(literals[index])
		}
	}
	return fmt.Sprintf("DELETE FROM %s.%s WHERE %s;", quoteIdentifier(schema), quoteIdentifier(table), builder.String())
}

func (s *Service) buildUpdateRollbackSQL(schema, table string, columns []databaseTableColumn, row map[string]any) string {
	setParts := make([]string, 0)
	for _, col := range columns {
		if _, exists := row[col.Name]; exists {
			setParts = append(setParts, fmt.Sprintf("%s = %s", quoteIdentifier(col.Name), sqlLiteral(row[col.Name])))
		}
	}
	whereSQL, args := buildRowWhereClause(columns, row)
	literals := make([]string, 0, len(args))
	for _, arg := range args {
		literals = append(literals, sqlLiteral(arg))
	}
	parts := strings.Split(whereSQL, "?")
	builder := &strings.Builder{}
	for index, part := range parts {
		builder.WriteString(part)
		if index < len(literals) {
			builder.WriteString(literals[index])
		}
	}
	return fmt.Sprintf(
		"UPDATE %s.%s SET %s WHERE %s;",
		quoteIdentifier(schema),
		quoteIdentifier(table),
		strings.Join(setParts, ", "),
		builder.String(),
	)
}

func (s *Service) buildInsertRollbackSQL(schema, table string, columns []databaseTableColumn, row map[string]any) string {
	insertColumns := make([]string, 0)
	values := make([]string, 0)
	for _, col := range columns {
		if _, exists := row[col.Name]; exists {
			insertColumns = append(insertColumns, col.Name)
			values = append(values, sqlLiteral(row[col.Name]))
		}
	}
	return fmt.Sprintf(
		"INSERT INTO %s.%s (%s) VALUES (%s);",
		quoteIdentifier(schema),
		quoteIdentifier(table),
		joinIdentifiers(insertColumns),
		strings.Join(values, ", "),
	)
}

func (s *Service) getTableColumns(db *sql.DB, schema, table string) ([]databaseTableColumn, error) {
	rows, err := db.Query(`
		SELECT COLUMN_NAME, DATA_TYPE, COLUMN_TYPE, COLUMN_KEY, IS_NULLABLE, COLUMN_DEFAULT, EXTRA
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make([]databaseTableColumn, 0)
	for rows.Next() {
		var item databaseTableColumn
		if err := rows.Scan(&item.Name, &item.DataType, &item.ColumnType, &item.ColumnKey, &item.IsNullable, &item.ColumnDefault, &item.Extra); err != nil {
			return nil, err
		}
		columns = append(columns, item)
	}
	return columns, rows.Err()
}

func (s *Service) logDBSQLHistory(asset *model.AssetDatabase, schema, table, sqlType, sqlText string, status int, rowsAffected int64, durationMs int64, errMessage, rollbackSQL string) {
	rollbackConfidence := ""
	if strings.TrimSpace(rollbackSQL) != "" {
		rollbackConfidence = "high"
	}
	history := model.DatabaseSQLHistory{
		DatabaseID:         asset.ID,
		DatabaseName:       asset.Name,
		SchemaName:         schema,
		TargetTable:        table,
		SQLType:            sqlType,
		SQLText:            sqlText,
		ExecutionID:        newDBMSExecutionID(),
		Operator:           "System Operation",
		Environment:        asset.Env,
		AccessMode:         normalizeDatabaseAccessMode(asset.AccessMode),
		Status:             status,
		RowsAffected:       rowsAffected,
		DurationMs:         durationMs,
		ErrorMessage:       errMessage,
		RollbackSQL:        rollbackSQL,
		RollbackConfidence: rollbackConfidence,
	}
	_ = s.db.Create(&history).Error
}

func (s *Service) updateTransferTask(taskID uint, updates map[string]any) {
	_ = s.db.Model(&model.DatabaseTransferTask{}).Where("id = ?", taskID).Updates(updates).Error
}

func (s *Service) runExportTask(taskID uint, payload DBMSExportPayload) {
	startedAt := time.Now()
	s.updateTransferTask(taskID, map[string]any{
		"status":     "running",
		"progress":   10,
		"message":    "Reading table schema",
		"started_at": &startedAt,
	})

	data, filename, err := s.ExportDatabaseTable(payload.DatabaseID, payload.Schema, payload.Table, payload.IncludeData)
	finishedAt := time.Now()
	if err != nil {
		s.updateTransferTask(taskID, map[string]any{
			"status":      "failed",
			"progress":    100,
			"message":     err.Error(),
			"finished_at": &finishedAt,
		})
		return
	}
	s.updateTransferTask(taskID, map[string]any{
		"status":       "success",
		"progress":     100,
		"message":      "Export completed",
		"file_name":    filename,
		"file_content": string(data),
		"finished_at":  &finishedAt,
	})
}

func (s *Service) runImportTask(taskID uint, payload DBMSImportPayload) {
	startedAt := time.Now()
	s.updateTransferTask(taskID, map[string]any{
		"status":     "running",
		"progress":   10,
		"message":    "Preparing import task",
		"started_at": &startedAt,
	})

	s.updateTransferTask(taskID, map[string]any{
		"progress": 35,
		"message":  "Comparing source and target table schemas",
	})
	result, err := s.ImportDatabaseTable(payload)
	finishedAt := time.Now()
	if err != nil {
		s.updateTransferTask(taskID, map[string]any{
			"status":      "failed",
			"progress":    100,
			"message":     err.Error(),
			"finished_at": &finishedAt,
		})
		return
	}
	rowsAffected, _ := result["imported"].(int64)
	if rowsAffected == 0 {
		switch v := result["imported"].(type) {
		case int:
			rowsAffected = int64(v)
		case float64:
			rowsAffected = int64(v)
		}
	}
	s.updateTransferTask(taskID, map[string]any{
		"status":        "success",
		"progress":      100,
		"message":       "Import completed",
		"rows_affected": rowsAffected,
		"finished_at":   &finishedAt,
	})
}
