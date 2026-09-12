package service

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"ops-admin/backend/model"
)

func (s *Service) ListAssetDatabases(pageNum, pageSize int, keyword string, dbType string, status string, env string, tag string) (map[string]any, error) {
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
	if tag != "" {
		query = query.Where("tags LIKE ?", "%\""+strings.TrimSpace(tag)+"\"%")
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
		Port:           databasePortByType(payload.DBType, payload.Port),
		Username:       Trimmed(payload.Username),
		Password:       payload.Password,
		ConnectionMode: normalizeConnectionMode(payload.ConnectionMode),
		GatewayID:      optionalGatewayID(payload.ConnectionMode, payload.GatewayID),
		DBName:         Trimmed(payload.DBName),
		Charset:        databaseCharsetByType(payload.DBType, payload.Charset),
		Env:            normalizeEnvCode(payload.Env),
		Tags:           normalizeAssetTags(payload.Tags),
		AccessMode:     normalizeDatabaseAccessMode(payload.AccessMode),
		MonitorEnabled: payload.MonitorEnabled,
		Status:         payload.Status,
		Description:    Trimmed(payload.Description),
	}
	if item.Name == "" {
		return errors.New("database name is required")
	}
	if item.Host == "" {
		return errors.New("database address is required")
	}
	if databaseRequiresUsername(item.DBType) && item.Username == "" {
		return errors.New("database username is required")
	}
	if item.Env == "" {
		return errors.New("select an environment")
	}
	if err := validateGatewaySelection(item.ConnectionMode, item.GatewayID); err != nil {
		return err
	}
	if item.Status == 0 {
		item.Status = 1
	}
	now := time.Now()
	item.LastCheckTime = &now
	version, err := s.inspectAssetDatabase(item)
	if err == nil {
		item.Version = version
		item.ConnectStatus = 1
	} else {
		item.ConnectStatus = 2
	}
	if err := s.db.Create(&item).Error; err != nil {
		return err
	}
	s.recordAssetChange("database", item.ID, item.Name, "create", "Create Database Asset", payload.Operator)
	return nil
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
		"port":            databasePortByType(payload.DBType, payload.Port),
		"username":        Trimmed(payload.Username),
		"password":        password,
		"connection_mode": normalizeConnectionMode(payload.ConnectionMode),
		"gateway_id":      optionalGatewayID(payload.ConnectionMode, payload.GatewayID),
		"db_name":         Trimmed(payload.DBName),
		"charset":         databaseCharsetByType(payload.DBType, payload.Charset),
		"env":             normalizeEnvCode(payload.Env),
		"tags":            normalizeAssetTags(payload.Tags),
		"access_mode":     normalizeDatabaseAccessMode(payload.AccessMode),
		"monitor_enabled": payload.MonitorEnabled,
		"status":          payload.Status,
		"description":     Trimmed(payload.Description),
	}
	if Trimmed(payload.Name) == "" {
		return errors.New("database name is required")
	}
	if Trimmed(payload.Host) == "" {
		return errors.New("database address is required")
	}
	if databaseRequiresUsername(payload.DBType) && Trimmed(payload.Username) == "" {
		return errors.New("database username is required")
	}
	if normalizeEnvCode(payload.Env) == "" {
		return errors.New("select an environment")
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
	probeItem.DBType = normalizeDatabaseType(payload.DBType)
	probeItem.Port = databasePortByType(payload.DBType, payload.Port)
	probeItem.Username = Trimmed(payload.Username)
	probeItem.Password = password
	probeItem.ConnectionMode = normalizeConnectionMode(payload.ConnectionMode)
	probeItem.GatewayID = optionalGatewayID(payload.ConnectionMode, payload.GatewayID)
	probeItem.DBName = Trimmed(payload.DBName)
	probeItem.Charset = databaseCharsetByType(payload.DBType, payload.Charset)
	probeItem.Env = normalizeEnvCode(payload.Env)
	probeItem.AccessMode = normalizeDatabaseAccessMode(payload.AccessMode)
	version, err := s.inspectAssetDatabase(probeItem)
	if err == nil {
		updates["version"] = version
		updates["connect_status"] = 1
	} else {
		updates["version"] = ""
		updates["connect_status"] = 2
	}
	if err := s.db.Model(&model.AssetDatabase{}).Where("id = ?", payload.ID).Updates(updates).Error; err != nil {
		return err
	}
	s.recordAssetChange("database", payload.ID, payload.Name, "update", "Update Database Details", payload.Operator)
	return nil
}

func (s *Service) DeleteAssetDatabase(id uint) error {
	var item model.AssetDatabase
	_ = s.db.First(&item, id).Error
	if err := s.db.Delete(&model.AssetDatabase{}, id).Error; err != nil {
		return err
	}
	s.recordAssetChange("database", id, item.Name, "delete", "Delete Database Asset", "system")
	return nil
}

func (s *Service) TestAssetDatabaseConnection(payload AssetDatabasePayload) (map[string]any, error) {
	dbType := normalizeDatabaseType(payload.DBType)
	item := model.AssetDatabase{
		DBType:         dbType,
		Host:           Trimmed(payload.Host),
		Port:           databasePortByType(dbType, payload.Port),
		Username:       Trimmed(payload.Username),
		Password:       payload.Password,
		ConnectionMode: normalizeConnectionMode(payload.ConnectionMode),
		GatewayID:      optionalGatewayID(payload.ConnectionMode, payload.GatewayID),
		DBName:         Trimmed(payload.DBName),
		Charset:        databaseCharsetByType(dbType, payload.Charset),
		Env:            normalizeEnvCode(payload.Env),
		AccessMode:     normalizeDatabaseAccessMode(payload.AccessMode),
	}
	if err := validateGatewaySelection(item.ConnectionMode, item.GatewayID); err != nil {
		return nil, err
	}
	version, err := s.inspectAssetDatabase(item)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"dbType":        dbType,
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
		"accessMode":    item.AccessMode,
		"mode":          databaseMode(item.DBType),
		"capabilities":  databaseCapabilities(item.DBType),
	}, nil
}

func (s *Service) GetDatabaseSchemaTree(databaseID uint) (map[string]any, error) {
	item, err := s.getAssetDatabase(databaseID)
	if err != nil {
		return nil, err
	}
	if normalizeDatabaseType(item.DBType) != "mysql" {
		return s.getNonMySQLSchemaTree(item)
	}
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

func (s *Service) CreateDatabaseSchema(payload DBMSCreateDatabasePayload) (map[string]any, error) {
	name := strings.TrimSpace(payload.Name)
	if payload.DatabaseID == 0 || name == "" {
		return nil, errors.New("enter a database name")
	}
	if !databaseObjectNamePattern.MatchString(name) {
		return nil, errors.New("name must start with a letter or underscore, contain only letters, digits, and underscores, and be at most 63 characters")
	}

	item, db, cleanup, err := s.openDatabaseByID(payload.DatabaseID, "")
	if err != nil {
		return nil, err
	}
	defer cleanup()
	defer db.Close()
	if err := ensureDatabaseWritable(item); err != nil {
		return nil, err
	}

	objectType := "database"
	sqlText := ""
	switch normalizeDatabaseType(item.DBType) {
	case "mysql":
		charset := strings.TrimSpace(payload.Charset)
		if charset == "" {
			charset = "utf8mb4"
		}
		collation := strings.TrimSpace(payload.Collation)
		if collation == "" {
			collation = "utf8mb4_unicode_ci"
		}
		if !mysqlCharsetOptionPattern.MatchString(charset) || !mysqlCharsetOptionPattern.MatchString(collation) {
			return nil, errors.New("invalid character set or collation format")
		}
		if !strings.HasPrefix(strings.ToLower(collation), strings.ToLower(charset)+"_") {
			return nil, errors.New("collation must match the selected character set")
		}
		sqlText = "CREATE DATABASE " + quoteIdentifier(name) + " CHARACTER SET " + charset + " COLLATE " + collation
	case "postgresql":
		objectType = "schema"
		sqlText = "CREATE SCHEMA " + postgresQuoteIdentifier(name)
	default:
		return nil, errors.New("the current database type does not support creating databases")
	}
	if _, err := db.Exec(sqlText); err != nil {
		return nil, err
	}

	summary := "SQL Workbench Create Database: " + name
	if objectType == "schema" {
		summary = "SQL Workbench Create PostgreSQL Schema: " + name
	}
	s.recordAssetChange("database", item.ID, item.Name, "create_schema", summary, payload.Operator)
	return map[string]any{"name": name, "objectType": objectType}, nil
}

func (s *Service) GetDatabaseCharsetOptions(databaseID uint, charset string) (map[string]any, error) {
	item, db, cleanup, err := s.openDatabaseByID(databaseID, "")
	if err != nil {
		return nil, err
	}
	defer cleanup()
	defer db.Close()
	if normalizeDatabaseType(item.DBType) != "mysql" {
		return nil, errors.New("the current database type does not support character-set or collation settings")
	}

	charsetRows, err := db.Query(`
		SELECT CHARACTER_SET_NAME, DEFAULT_COLLATE_NAME
		FROM information_schema.CHARACTER_SETS
		ORDER BY CHARACTER_SET_NAME
	`)
	if err != nil {
		return nil, err
	}
	defer charsetRows.Close()
	charsets := make([]map[string]any, 0)
	for charsetRows.Next() {
		var name, defaultCollation string
		if err := charsetRows.Scan(&name, &defaultCollation); err != nil {
			return nil, err
		}
		charsets = append(charsets, map[string]any{"name": name, "defaultCollation": defaultCollation})
	}
	if err := charsetRows.Err(); err != nil {
		return nil, err
	}

	selectedCharset := strings.TrimSpace(charset)
	if selectedCharset == "" {
		selectedCharset = "utf8mb4"
	}
	collationRows, err := db.Query(`
		SELECT COLLATION_NAME, IS_DEFAULT
		FROM information_schema.COLLATIONS
		WHERE CHARACTER_SET_NAME = ?
		ORDER BY IS_DEFAULT DESC, COLLATION_NAME
	`, selectedCharset)
	if err != nil {
		return nil, err
	}
	defer collationRows.Close()
	collations := make([]map[string]any, 0)
	for collationRows.Next() {
		var name, isDefault string
		if err := collationRows.Scan(&name, &isDefault); err != nil {
			return nil, err
		}
		collations = append(collations, map[string]any{"name": name, "isDefault": strings.EqualFold(isDefault, "Yes")})
	}
	if err := collationRows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"charsets": charsets, "collations": collations}, nil
}
