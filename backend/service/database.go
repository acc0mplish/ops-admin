package service

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

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
