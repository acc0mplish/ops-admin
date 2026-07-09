package service

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
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
	if strings.EqualFold(strings.TrimSpace(v), "mysql") {
		return "mysql"
	}
	return "mysql"
}

func databasePort(v int) int {
	if v > 0 {
		return v
	}
	return 3306
}

func databaseCharset(v string) string {
	value := strings.TrimSpace(v)
	if value == "" {
		return "utf8mb4"
	}
	return value
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

func (s *Service) ListAssetDatabases(pageNum, pageSize int, keyword string, dbType string, status string, env string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.AssetDatabase{}).Preload("Gateway")
	if keyword != "" {
		query = query.Where("name like ? or host like ? or username like ? or db_name like ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if dbType != "" {
		query = query.Where("db_type = ?", normalizeDatabaseType(dbType))
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if env != "" {
		query = query.Where("env = ?", normalizeEnvCode(env))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.AssetDatabase
	if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Password = ""
	}
	return map[string]any{
		"list":     list,
		"total":    total,
		"pageNum":  pageNum,
		"pageSize": pageSize,
	}, nil
}

func (s *Service) GetAssetDatabase(id uint) (*model.AssetDatabase, error) {
	item, err := s.getAssetDatabase(id)
	if err != nil {
		return nil, err
	}
	item.Password = ""
	return item, nil
}

func (s *Service) CreateAssetDatabase(payload AssetDatabasePayload) error {
	item := model.AssetDatabase{
		Name:           Trimmed(payload.Name),
		DBType:         normalizeDatabaseType(payload.DBType),
		Host:           Trimmed(payload.Host),
		Port:           databasePort(payload.Port),
		Username:       Trimmed(payload.Username),
		Password:       payload.Password,
		ConnectionMode: normalizeConnectionMode(payload.ConnectionMode),
		GatewayID:      optionalGatewayID(payload.ConnectionMode, payload.GatewayID),
		DBName:         Trimmed(payload.DBName),
		Charset:        databaseCharset(payload.Charset),
		Env:            normalizeEnvCode(payload.Env),
		Status:         payload.Status,
		Description:    Trimmed(payload.Description),
	}
	if item.Name == "" {
		return errors.New("数据库名称不能为空")
	}
	if item.Host == "" {
		return errors.New("数据库地址不能为空")
	}
	if item.Username == "" {
		return errors.New("数据库用户名不能为空")
	}
	if err := validateGatewaySelection(item.ConnectionMode, item.GatewayID); err != nil {
		return err
	}
	if item.Status == 0 {
		item.Status = 1
	}
	now := time.Now()
	item.LastCheckTime = &now
	version, err := s.inspectAssetMySQLDatabase(item)
	if err == nil {
		item.Version = version
		item.ConnectStatus = 1
	} else {
		item.ConnectStatus = 2
	}
	return s.db.Create(&item).Error
}

func (s *Service) UpdateAssetDatabase(payload AssetDatabasePayload) error {
	existing, err := s.getAssetDatabase(payload.ID)
	if err != nil {
		return err
	}
	password := payload.Password
	if password == "" {
		password = existing.Password
	}
	updates := map[string]any{
		"name":            Trimmed(payload.Name),
		"db_type":         normalizeDatabaseType(payload.DBType),
		"host":            Trimmed(payload.Host),
		"port":            databasePort(payload.Port),
		"username":        Trimmed(payload.Username),
		"password":        password,
		"connection_mode": normalizeConnectionMode(payload.ConnectionMode),
		"gateway_id":      optionalGatewayID(payload.ConnectionMode, payload.GatewayID),
		"db_name":         Trimmed(payload.DBName),
		"charset":         databaseCharset(payload.Charset),
		"env":             normalizeEnvCode(payload.Env),
		"status":          payload.Status,
		"description":     Trimmed(payload.Description),
	}
	if Trimmed(payload.Name) == "" {
		return errors.New("数据库名称不能为空")
	}
	if Trimmed(payload.Host) == "" {
		return errors.New("数据库地址不能为空")
	}
	if Trimmed(payload.Username) == "" {
		return errors.New("数据库用户名不能为空")
	}
	if err := validateGatewaySelection(normalizeConnectionMode(payload.ConnectionMode), optionalGatewayID(payload.ConnectionMode, payload.GatewayID)); err != nil {
		return err
	}
	if payload.Status == 0 {
		updates["status"] = 1
	}
	now := time.Now()
	updates["last_check_time"] = &now
	probeItem := *existing
	probeItem.Host = Trimmed(payload.Host)
	probeItem.Port = databasePort(payload.Port)
	probeItem.Username = Trimmed(payload.Username)
	probeItem.Password = password
	probeItem.ConnectionMode = normalizeConnectionMode(payload.ConnectionMode)
	probeItem.GatewayID = optionalGatewayID(payload.ConnectionMode, payload.GatewayID)
	probeItem.DBName = Trimmed(payload.DBName)
	probeItem.Charset = databaseCharset(payload.Charset)
	probeItem.Env = normalizeEnvCode(payload.Env)
	version, err := s.inspectAssetMySQLDatabase(probeItem)
	if err == nil {
		updates["version"] = version
		updates["connect_status"] = 1
	} else {
		updates["version"] = ""
		updates["connect_status"] = 2
	}
	return s.db.Model(&model.AssetDatabase{}).Where("id = ?", payload.ID).Updates(updates).Error
}

func (s *Service) DeleteAssetDatabase(id uint) error {
	return s.db.Delete(&model.AssetDatabase{}, id).Error
}

func (s *Service) TestAssetDatabaseConnection(payload AssetDatabasePayload) (map[string]any, error) {
	dbType := normalizeDatabaseType(payload.DBType)
	if dbType != "mysql" {
		return nil, errors.New("当前仅支持 MySQL")
	}
	item := model.AssetDatabase{
		Host:           Trimmed(payload.Host),
		Port:           databasePort(payload.Port),
		Username:       Trimmed(payload.Username),
		Password:       payload.Password,
		ConnectionMode: normalizeConnectionMode(payload.ConnectionMode),
		GatewayID:      optionalGatewayID(payload.ConnectionMode, payload.GatewayID),
		DBName:         Trimmed(payload.DBName),
		Charset:        databaseCharset(payload.Charset),
		Env:            normalizeEnvCode(payload.Env),
	}
	if err := validateGatewaySelection(item.ConnectionMode, item.GatewayID); err != nil {
		return nil, err
	}
	version, err := s.inspectAssetMySQLDatabase(item)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"version":       version,
		"connectStatus": 1,
		"checkedAt":     time.Now(),
	}, nil
}

func (s *Service) GetDatabaseWorkbench(databaseID uint) (map[string]any, error) {
	item, err := s.getAssetDatabase(databaseID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":            item.ID,
		"name":          item.Name,
		"dbType":        item.DBType,
		"host":          item.Host,
		"port":          item.Port,
		"username":      item.Username,
		"dbName":        item.DBName,
		"charset":       item.Charset,
		"version":       item.Version,
		"connectStatus": item.ConnectStatus,
	}, nil
}

func (s *Service) GetDatabaseSchemaTree(databaseID uint) (map[string]any, error) {
	item, db, cleanup, err := s.openDatabaseByID(databaseID, "")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	defer cleanup()

	schemas := make([]map[string]any, 0)
	rows, err := db.Query(`
		SELECT SCHEMA_NAME
		FROM INFORMATION_SCHEMA.SCHEMATA
		WHERE SCHEMA_NAME NOT IN ('information_schema', 'performance_schema', 'mysql', 'sys')
		ORDER BY SCHEMA_NAME
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName string
		if err := rows.Scan(&schemaName); err != nil {
			return nil, err
		}
		tableRows, err := db.Query(`
			SELECT TABLE_NAME, TABLE_TYPE, TABLE_ROWS
			FROM INFORMATION_SCHEMA.TABLES
			WHERE TABLE_SCHEMA = ?
			ORDER BY TABLE_NAME
		`, schemaName)
		if err != nil {
			return nil, err
		}
		tables := make([]map[string]any, 0)
		for tableRows.Next() {
			var tableName, tableType string
			var tableRowsCount sql.NullInt64
			if err := tableRows.Scan(&tableName, &tableType, &tableRowsCount); err != nil {
				tableRows.Close()
				return nil, err
			}
			tables = append(tables, map[string]any{
				"name":      tableName,
				"type":      tableType,
				"rows":      tableRowsCount.Int64,
				"schema":    schemaName,
				"fullName":  schemaName + "." + tableName,
				"isDefault": tableName == item.DBName,
			})
		}
		tableRows.Close()
		schemas = append(schemas, map[string]any{
			"name":       schemaName,
			"tableCount": len(tables),
			"tables":     tables,
			"isCurrent":  schemaName == item.DBName,
		})
	}
	return map[string]any{
		"schemas":       schemas,
		"defaultSchema": defaultSchema(item, ""),
	}, nil
}

func (s *Service) GetDatabaseTableData(payload DBMSTableDataQueryPayload) (map[string]any, error) {
	if payload.DatabaseID == 0 || payload.Table == "" {
		return nil, errors.New("请先选择表")
	}
	if payload.PageNum < 1 {
		payload.PageNum = 1
	}
	if payload.PageSize < 1 {
		payload.PageSize = 25
	}
	schema := strings.TrimSpace(payload.Schema)
	item, db, cleanup, err := s.openDatabaseByID(payload.DatabaseID, schema)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	defer cleanup()
	schema = defaultSchema(item, schema)

	columns, err := s.getTableColumns(db, schema, payload.Table)
	if err != nil {
		return nil, err
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", quoteIdentifier(schema), quoteIdentifier(payload.Table))
	countArgs := make([]any, 0)
	if filterClause, filterArgs, err := buildTableFilterClause(columns, payload.FilterKey, payload.FilterText); err != nil {
		return nil, err
	} else if filterClause != "" {
		countSQL += " WHERE " + filterClause
		countArgs = append(countArgs, filterArgs...)
	}
	if err := db.QueryRow(countSQL, countArgs...).Scan(&total); err != nil {
		return nil, err
	}

	querySQL := fmt.Sprintf(
		"SELECT * FROM %s.%s LIMIT %d OFFSET %d",
		quoteIdentifier(schema),
		quoteIdentifier(payload.Table),
		payload.PageSize,
		(payload.PageNum-1)*payload.PageSize,
	)
	queryArgs := make([]any, 0)
	if filterClause, filterArgs, err := buildTableFilterClause(columns, payload.FilterKey, payload.FilterText); err != nil {
		return nil, err
	} else if filterClause != "" {
		querySQL = strings.Replace(querySQL, " LIMIT ", " WHERE "+filterClause+" LIMIT ", 1)
		queryArgs = append(queryArgs, filterArgs...)
	}
	rows, err := db.Query(querySQL, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columnNames, dataRows, err := scanRows(rows)
	if err != nil {
		return nil, err
	}

	primaryKeys := make([]string, 0)
	for _, item := range columns {
		if strings.EqualFold(item.ColumnKey, "PRI") {
			primaryKeys = append(primaryKeys, item.Name)
		}
	}

	return map[string]any{
		"schema":      schema,
		"table":       payload.Table,
		"columns":     columns,
		"columnNames": columnNames,
		"rows":        dataRows,
		"primaryKeys": primaryKeys,
		"total":       total,
		"pageNum":     payload.PageNum,
		"pageSize":    payload.PageSize,
		"filterKey":   payload.FilterKey,
		"filterText":  payload.FilterText,
	}, nil
}

func (s *Service) ExecuteDatabaseSQL(payload DBMSSQLExecutePayload) (map[string]any, error) {
	sqlText := strings.TrimSpace(payload.SQLText)
	if payload.DatabaseID == 0 || sqlText == "" {
		return nil, errors.New("请输入 SQL")
	}
	start := time.Now()
	item, db, cleanup, err := s.openDatabaseByID(payload.DatabaseID, payload.Schema)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	defer cleanup()

	trimmedSQL := strings.TrimSuffix(sqlText, ";")
	sqlType := detectSQLType(trimmedSQL)
	history := model.DatabaseSQLHistory{
		DatabaseID:   item.ID,
		DatabaseName: item.Name,
		SchemaName:   defaultSchema(item, payload.Schema),
		SQLType:      sqlType,
		SQLText:      sqlText,
	}

	if isQuerySQL(sqlType) {
		rows, err := db.Query(trimmedSQL)
		durationMs := time.Since(start).Milliseconds()
		if err != nil {
			history.Status = 2
			history.DurationMs = durationMs
			history.ErrorMessage = err.Error()
			s.db.Create(&history)
			return nil, err
		}
		defer rows.Close()
		columnNames, dataRows, err := scanRows(rows)
		if err != nil {
			history.Status = 2
			history.DurationMs = durationMs
			history.ErrorMessage = err.Error()
			s.db.Create(&history)
			return nil, err
		}
		history.Status = 1
		history.DurationMs = durationMs
		history.RowsAffected = int64(len(dataRows))
		s.db.Create(&history)
		return map[string]any{
			"sqlType":      sqlType,
			"columns":      columnNames,
			"rows":         dataRows,
			"rowsAffected": len(dataRows),
			"durationMs":   durationMs,
			"historyId":    history.ID,
		}, nil
	}

	result, err := db.Exec(trimmedSQL)
	durationMs := time.Since(start).Milliseconds()
	if err != nil {
		history.Status = 2
		history.DurationMs = durationMs
		history.ErrorMessage = err.Error()
		s.db.Create(&history)
		return nil, err
	}
	rowsAffected, _ := result.RowsAffected()
	history.Status = 1
	history.DurationMs = durationMs
	history.RowsAffected = rowsAffected
	s.db.Create(&history)
	return map[string]any{
		"sqlType":      sqlType,
		"rowsAffected": rowsAffected,
		"durationMs":   durationMs,
		"historyId":    history.ID,
	}, nil
}

func (s *Service) ListDatabaseSQLHistory(databaseID uint, pageNum, pageSize int) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	query := s.db.Model(&model.DatabaseSQLHistory{}).Where("database_id = ?", databaseID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.DatabaseSQLHistory
	if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"list":     list,
		"total":    total,
		"pageNum":  pageNum,
		"pageSize": pageSize,
	}, nil
}

func (s *Service) InsertDatabaseTableRow(payload DBMSTableInsertPayload) (map[string]any, error) {
	item, db, cleanup, err := s.openDatabaseByID(payload.DatabaseID, payload.Schema)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	defer cleanup()
	schema := defaultSchema(item, payload.Schema)
	columns, err := s.getTableColumns(db, schema, payload.Table)
	if err != nil {
		return nil, err
	}

	insertColumns := make([]string, 0)
	values := make([]any, 0)
	for _, col := range columns {
		if _, exists := payload.Row[col.Name]; exists {
			insertColumns = append(insertColumns, col.Name)
			values = append(values, normalizeJSONValue(payload.Row[col.Name]))
		}
	}
	if len(insertColumns) == 0 {
		return nil, errors.New("没有可插入的数据")
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(insertColumns)), ",")
	insertSQL := fmt.Sprintf(
		"INSERT INTO %s.%s (%s) VALUES (%s)",
		quoteIdentifier(schema),
		quoteIdentifier(payload.Table),
		joinIdentifiers(insertColumns),
		placeholders,
	)
	result, err := db.Exec(insertSQL, values...)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	insertedRow := make(map[string]any, len(payload.Row))
	for key, value := range payload.Row {
		insertedRow[key] = value
	}
	if lastInsertID, err := result.LastInsertId(); err == nil {
		for _, col := range columns {
			if strings.EqualFold(col.ColumnKey, "PRI") && strings.Contains(strings.ToLower(col.Extra), "auto_increment") {
				if _, exists := insertedRow[col.Name]; !exists || insertedRow[col.Name] == nil || insertedRow[col.Name] == "" {
					insertedRow[col.Name] = lastInsertID
				}
			}
		}
	}
	rollbackSQL := s.buildDeleteRollbackSQL(schema, payload.Table, columns, insertedRow)
	s.logDBSQLHistory(item, schema, payload.Table, "INSERT", insertSQL, 1, affected, 0, "", rollbackSQL)
	return map[string]any{"rowsAffected": affected, "rollbackSql": rollbackSQL}, nil
}

func (s *Service) UpdateDatabaseTableRow(payload DBMSTableUpdatePayload) (map[string]any, error) {
	item, db, cleanup, err := s.openDatabaseByID(payload.DatabaseID, payload.Schema)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	defer cleanup()
	schema := defaultSchema(item, payload.Schema)
	columns, err := s.getTableColumns(db, schema, payload.Table)
	if err != nil {
		return nil, err
	}

	setParts := make([]string, 0)
	args := make([]any, 0)
	for _, col := range columns {
		newVal, exists := payload.Current[col.Name]
		if !exists {
			continue
		}
		oldVal := payload.Original[col.Name]
		if fmt.Sprintf("%v", normalizeJSONValue(newVal)) == fmt.Sprintf("%v", normalizeJSONValue(oldVal)) {
			continue
		}
		setParts = append(setParts, fmt.Sprintf("%s = ?", quoteIdentifier(col.Name)))
		args = append(args, normalizeJSONValue(newVal))
	}
	if len(setParts) == 0 {
		return map[string]any{"rowsAffected": 0, "rollbackSql": ""}, nil
	}

	whereSQL, whereArgs := buildRowWhereClause(columns, payload.Original)
	updateSQL := fmt.Sprintf(
		"UPDATE %s.%s SET %s WHERE %s",
		quoteIdentifier(schema),
		quoteIdentifier(payload.Table),
		strings.Join(setParts, ", "),
		whereSQL,
	)
	args = append(args, whereArgs...)
	result, err := db.Exec(updateSQL, args...)
	if err != nil {
		return nil, err
	}
	rollbackSQL := s.buildUpdateRollbackSQL(schema, payload.Table, columns, payload.Original)
	affected, _ := result.RowsAffected()
	s.logDBSQLHistory(item, schema, payload.Table, "UPDATE", updateSQL, 1, affected, 0, "", rollbackSQL)
	return map[string]any{"rowsAffected": affected, "rollbackSql": rollbackSQL}, nil
}

func (s *Service) DeleteDatabaseTableRow(payload DBMSTableDeletePayload) (map[string]any, error) {
	item, db, cleanup, err := s.openDatabaseByID(payload.DatabaseID, payload.Schema)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	defer cleanup()
	schema := defaultSchema(item, payload.Schema)
	columns, err := s.getTableColumns(db, schema, payload.Table)
	if err != nil {
		return nil, err
	}
	whereSQL, args := buildRowWhereClause(columns, payload.Row)
	deleteSQL := fmt.Sprintf(
		"DELETE FROM %s.%s WHERE %s",
		quoteIdentifier(schema),
		quoteIdentifier(payload.Table),
		whereSQL,
	)
	result, err := db.Exec(deleteSQL, args...)
	if err != nil {
		return nil, err
	}
	rollbackSQL := s.buildInsertRollbackSQL(schema, payload.Table, columns, payload.Row)
	affected, _ := result.RowsAffected()
	s.logDBSQLHistory(item, schema, payload.Table, "DELETE", deleteSQL, 1, affected, 0, "", rollbackSQL)
	return map[string]any{"rowsAffected": affected, "rollbackSql": rollbackSQL}, nil
}

func (s *Service) ExportDatabaseTable(databaseID uint, schema string, table string, includeData bool) ([]byte, string, error) {
	if databaseID == 0 || strings.TrimSpace(table) == "" {
		return nil, "", errors.New("请先选择表")
	}
	item, db, cleanup, err := s.openDatabaseByID(databaseID, schema)
	if err != nil {
		return nil, "", err
	}
	defer db.Close()
	defer cleanup()
	schema = defaultSchema(item, schema)

	showSQL := fmt.Sprintf("SHOW CREATE TABLE %s.%s", quoteIdentifier(schema), quoteIdentifier(table))
	showRows, err := db.Query(showSQL)
	if err != nil {
		return nil, "", err
	}
	defer showRows.Close()
	_, dataRows, err := scanRows(showRows)
	if err != nil || len(dataRows) == 0 {
		return nil, "", errors.New("获取建表语句失败")
	}
	createSQL := fmt.Sprintf("%v", dataRows[0]["Create Table"])
	builder := &strings.Builder{}
	builder.WriteString("-- Ops Admin DBMS Export\n")
	builder.WriteString("-- Generated at " + time.Now().Format("2006-01-02 15:04:05") + "\n\n")
	builder.WriteString("DROP TABLE IF EXISTS " + quoteIdentifier(table) + ";\n")
	builder.WriteString(createSQL + ";\n\n")

	if includeData {
		dataSQL := fmt.Sprintf("SELECT * FROM %s.%s", quoteIdentifier(schema), quoteIdentifier(table))
		rows, err := db.Query(dataSQL)
		if err != nil {
			return nil, "", err
		}
		defer rows.Close()
		columnNames, list, err := scanRows(rows)
		if err != nil {
			return nil, "", err
		}
		for _, row := range list {
			values := make([]string, 0, len(columnNames))
			for _, name := range columnNames {
				values = append(values, sqlLiteral(row[name]))
			}
			builder.WriteString(fmt.Sprintf(
				"INSERT INTO %s (%s) VALUES (%s);\n",
				quoteIdentifier(table),
				joinIdentifiers(columnNames),
				strings.Join(values, ", "),
			))
		}
	}
	filename := fmt.Sprintf("%s_%s.sql", schema, table)
	return []byte(builder.String()), filename, nil
}

func (s *Service) ImportDatabaseTable(payload DBMSImportPayload) (map[string]any, error) {
	if payload.SourceDatabaseID == 0 || payload.TargetDatabaseID == 0 {
		return nil, errors.New("请选择源数据库和目标数据库")
	}
	if strings.TrimSpace(payload.SourceTable) == "" {
		return nil, errors.New("请选择源表")
	}
	sourceAsset, sourceDB, sourceCleanup, err := s.openDatabaseByID(payload.SourceDatabaseID, payload.SourceSchema)
	if err != nil {
		return nil, err
	}
	defer sourceDB.Close()
	defer sourceCleanup()

	targetAsset, targetDB, targetCleanup, err := s.openDatabaseByID(payload.TargetDatabaseID, payload.TargetSchema)
	if err != nil {
		return nil, err
	}
	defer targetDB.Close()
	defer targetCleanup()

	sourceSchema := defaultSchema(sourceAsset, payload.SourceSchema)
	targetSchema := defaultSchema(targetAsset, payload.TargetSchema)
	targetTable := strings.TrimSpace(payload.TargetTable)
	if targetTable == "" {
		targetTable = strings.TrimSpace(payload.SourceTable)
	}

	if payload.CreateIfMissing {
		showSQL := fmt.Sprintf("SHOW CREATE TABLE %s.%s", quoteIdentifier(sourceSchema), quoteIdentifier(payload.SourceTable))
		rows, err := sourceDB.Query(showSQL)
		if err != nil {
			return nil, err
		}
		_, list, err := scanRows(rows)
		rows.Close()
		if err != nil || len(list) == 0 {
			return nil, errors.New("获取源表结构失败")
		}
		createSQL := fmt.Sprintf("%v", list[0]["Create Table"])
		createSQL = strings.Replace(createSQL, "CREATE TABLE `"+payload.SourceTable+"`", "CREATE TABLE IF NOT EXISTS `"+targetTable+"`", 1)
		if _, err := targetDB.Exec(createSQL); err != nil {
			return nil, err
		}
	}

	sourceColumns, err := s.getTableColumns(sourceDB, sourceSchema, payload.SourceTable)
	if err != nil {
		return nil, err
	}
	targetColumns, err := s.getTableColumns(targetDB, targetSchema, targetTable)
	if err != nil {
		return nil, err
	}
	targetColumnSet := make(map[string]struct{}, len(targetColumns))
	for _, col := range targetColumns {
		targetColumnSet[col.Name] = struct{}{}
	}

	commonColumns := make([]string, 0)
	for _, col := range sourceColumns {
		if _, ok := targetColumnSet[col.Name]; ok {
			commonColumns = append(commonColumns, col.Name)
		}
	}
	if len(commonColumns) == 0 {
		return nil, errors.New("源表和目标表没有可匹配的字段")
	}

	if payload.TruncateTarget {
		if _, err := targetDB.Exec(fmt.Sprintf("TRUNCATE TABLE %s.%s", quoteIdentifier(targetSchema), quoteIdentifier(targetTable))); err != nil {
			return nil, err
		}
	}

	selectSQL := fmt.Sprintf(
		"SELECT %s FROM %s.%s",
		joinIdentifiers(commonColumns),
		quoteIdentifier(sourceSchema),
		quoteIdentifier(payload.SourceTable),
	)
	rows, err := sourceDB.Query(selectSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	_, rowList, err := scanRows(rows)
	if err != nil {
		return nil, err
	}

	tx, err := targetDB.Begin()
	if err != nil {
		return nil, err
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(commonColumns)), ",")
	insertSQL := fmt.Sprintf(
		"INSERT INTO %s.%s (%s) VALUES (%s)",
		quoteIdentifier(targetSchema),
		quoteIdentifier(targetTable),
		joinIdentifiers(commonColumns),
		placeholders,
	)
	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	defer stmt.Close()

	var imported int64
	for _, row := range rowList {
		args := make([]any, 0, len(commonColumns))
		for _, col := range commonColumns {
			args = append(args, row[col])
		}
		if _, err := stmt.Exec(args...); err != nil {
			tx.Rollback()
			return nil, err
		}
		imported++
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.logDBSQLHistory(targetAsset, targetSchema, targetTable, "IMPORT", insertSQL, 1, imported, 0, "", "")
	return map[string]any{"imported": imported, "targetTable": targetTable}, nil
}

func (s *Service) CreateExportTask(payload DBMSExportPayload) (map[string]any, error) {
	if payload.DatabaseID == 0 || strings.TrimSpace(payload.Table) == "" {
		return nil, errors.New("请先选择要导出的表")
	}
	asset, err := s.getAssetDatabase(payload.DatabaseID)
	if err != nil {
		return nil, err
	}
	task := model.DatabaseTransferTask{
		TaskType:     "export",
		Status:       "pending",
		Progress:     0,
		Message:      "等待执行",
		DatabaseID:   asset.ID,
		DatabaseName: asset.Name,
		SchemaName:   defaultSchema(asset, payload.Schema),
		PrimaryTable: strings.TrimSpace(payload.Table),
	}
	if err := s.db.Create(&task).Error; err != nil {
		return nil, err
	}
	go s.runExportTask(task.ID, payload)
	return map[string]any{"taskId": task.ID}, nil
}

func (s *Service) CreateImportTask(payload DBMSImportPayload) (map[string]any, error) {
	if payload.SourceDatabaseID == 0 || payload.TargetDatabaseID == 0 {
		return nil, errors.New("请选择源数据库和目标数据库")
	}
	sourceAsset, err := s.getAssetDatabase(payload.SourceDatabaseID)
	if err != nil {
		return nil, err
	}
	targetAsset, err := s.getAssetDatabase(payload.TargetDatabaseID)
	if err != nil {
		return nil, err
	}
	task := model.DatabaseTransferTask{
		TaskType:         "import",
		Status:           "pending",
		Progress:         0,
		Message:          "等待执行",
		SourceDatabaseID: sourceAsset.ID,
		SourceDatabase:   sourceAsset.Name,
		SourceSchema:     defaultSchema(sourceAsset, payload.SourceSchema),
		SourceTable:      strings.TrimSpace(payload.SourceTable),
		TargetDatabaseID: targetAsset.ID,
		TargetDatabase:   targetAsset.Name,
		TargetSchema:     defaultSchema(targetAsset, payload.TargetSchema),
		TargetTable:      strings.TrimSpace(payload.TargetTable),
		DatabaseID:       targetAsset.ID,
		DatabaseName:     targetAsset.Name,
	}
	if err := s.db.Create(&task).Error; err != nil {
		return nil, err
	}
	go s.runImportTask(task.ID, payload)
	return map[string]any{"taskId": task.ID}, nil
}

func (s *Service) ListTransferTasks(databaseID uint, taskType string, pageNum, pageSize int) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	query := s.db.Model(&model.DatabaseTransferTask{})
	if databaseID > 0 {
		query = query.Where("database_id = ? OR target_database_id = ? OR source_database_id = ?", databaseID, databaseID, databaseID)
	}
	if strings.TrimSpace(taskType) != "" {
		query = query.Where("task_type = ?", strings.TrimSpace(taskType))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.DatabaseTransferTask
	if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"list":     list,
		"total":    total,
		"pageNum":  pageNum,
		"pageSize": pageSize,
	}, nil
}

func (s *Service) GetTransferTaskFile(taskID uint) ([]byte, string, error) {
	var task model.DatabaseTransferTask
	if err := s.db.First(&task, taskID).Error; err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(task.FileContent) == "" {
		return nil, "", errors.New("当前任务没有可下载文件")
	}
	filename := strings.TrimSpace(task.FileName)
	if filename == "" {
		filename = fmt.Sprintf("dbms-task-%d.sql", task.ID)
	}
	return []byte(task.FileContent), filename, nil
}

func (s *Service) getAssetDatabase(id uint) (*model.AssetDatabase, error) {
	var item model.AssetDatabase
	if err := s.db.Preload("Gateway").Preload("Gateway.Credential").First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) openDatabaseByID(id uint, schema string) (*model.AssetDatabase, *sql.DB, func(), error) {
	item, err := s.getAssetDatabase(id)
	if err != nil {
		return nil, nil, func() {}, err
	}
	if normalizeDatabaseType(item.DBType) != "mysql" {
		return nil, nil, func() {}, errors.New("当前仅支持 MySQL")
	}
	db, cleanup, err := s.openAssetMySQLDatabase(*item, schema)
	if err != nil {
		return nil, nil, cleanup, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		cleanup()
		return nil, nil, func() {}, err
	}
	return item, db, cleanup, nil
}

func defaultSchema(item *model.AssetDatabase, schema string) string {
	value := strings.TrimSpace(schema)
	if value != "" {
		return value
	}
	if strings.TrimSpace(item.DBName) != "" {
		return strings.TrimSpace(item.DBName)
	}
	return "mysql"
}

func detectSQLType(sqlText string) string {
	fields := strings.Fields(strings.TrimSpace(sqlText))
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
		return "", nil, errors.New("筛选字段不存在")
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
	history := model.DatabaseSQLHistory{
		DatabaseID:   asset.ID,
		DatabaseName: asset.Name,
		SchemaName:   schema,
		TargetTable:  table,
		SQLType:      sqlType,
		SQLText:      sqlText,
		Status:       status,
		RowsAffected: rowsAffected,
		DurationMs:   durationMs,
		ErrorMessage: errMessage,
		RollbackSQL:  rollbackSQL,
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
		"message":    "正在读取表结构",
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
		"message":      "导出完成",
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
		"message":    "准备导入任务",
		"started_at": &startedAt,
	})

	s.updateTransferTask(taskID, map[string]any{
		"progress": 35,
		"message":  "正在比对源表和目标表结构",
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
		"message":       "导入完成",
		"rows_affected": rowsAffected,
		"finished_at":   &finishedAt,
	})
}
