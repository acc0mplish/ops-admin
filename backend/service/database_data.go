package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"ops-admin/backend/model"
)

func (s *Service) GetDatabaseTableData(payload DBMSTableDataQueryPayload) (map[string]any, error) {
	if payload.DatabaseID == 0 || strings.TrimSpace(payload.Table) == "" {
		return nil, errors.New("select a table first")
	}
	asset, err := s.getAssetDatabase(payload.DatabaseID)
	if err != nil {
		return nil, err
	}
	if normalizeDatabaseType(asset.DBType) == "postgresql" {
		return s.getPostgresTableData(asset, payload)
	}
	if payload.DatabaseID == 0 || payload.Table == "" {
		return nil, errors.New("select a table first")
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
	if err := ensureMySQLFeature(item); err != nil {
		db.Close()
		cleanup()
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
		return nil, errors.New("enter SQL")
	}
	analysis, err := s.AnalyzeDatabaseSQL(payload)
	if err != nil {
		return nil, err
	}
	if analysis.WriteOperation && analysis.AccessMode == "readonly" {
		return nil, errors.New("the current database is read-only; write or schema-changing SQL is not allowed")
	}
	if analysis.WriteOperation && !payload.Confirmed {
		return nil, errors.New("write operations require confirmation before execution")
	}
	start := time.Now()
	item, db, cleanup, err := s.openDatabaseByID(payload.DatabaseID, payload.Schema)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	defer cleanup()

	trimmedSQL := strings.TrimSuffix(sqlText, ";")
	sqlType := analysis.SQLType
	history := model.DatabaseSQLHistory{
		DatabaseID:   item.ID,
		DatabaseName: item.Name,
		SchemaName:   defaultSchema(item, payload.Schema),
		SQLType:      sqlType,
		SQLText:      sqlText,
		ExecutionID:  newDBMSExecutionID(),
		Operator:     strings.TrimSpace(payload.Operator),
		ClientIP:     strings.TrimSpace(payload.ClientIP),
		Environment:  item.Env,
		AccessMode:   normalizeDatabaseAccessMode(item.AccessMode),
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
			"executionId":  history.ExecutionID,
			"analysis":     analysis,
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
		"executionId":  history.ExecutionID,
		"analysis":     analysis,
	}, nil
}

func (s *Service) AnalyzeDatabaseSQL(payload DBMSSQLExecutePayload) (*DBMSSQLAnalysis, error) {
	if payload.DatabaseID == 0 || strings.TrimSpace(payload.SQLText) == "" {
		return nil, errors.New("enter SQL")
	}
	item, err := s.getAssetDatabase(payload.DatabaseID)
	if err != nil {
		return nil, err
	}
	statements := splitDBMSSQLStatements(payload.SQLText)
	if len(statements) == 0 {
		return nil, errors.New("enter valid SQL")
	}
	sqlType := detectSQLType(statements[0])
	writeOperation := false
	riskLevel := "low"
	reasons := make([]string, 0)
	for _, statement := range statements {
		statementType := detectSQLType(statement)
		if !isReadOnlySQL(statement) {
			writeOperation = true
		}
		switch statementType {
		case "DROP", "TRUNCATE", "ALTER", "GRANT", "REVOKE", "RENAME":
			riskLevel = "high"
			reasons = append(reasons, statementType+" is a high-risk schema or privilege change")
		case "UPDATE", "DELETE":
			if !regexp.MustCompile(`(?i)\bWHERE\b`).MatchString(stripDBMSSQLComments(statement)) {
				riskLevel = "high"
				reasons = append(reasons, statementType+" does not contain a WHERE clause")
			} else if riskLevel != "high" {
				riskLevel = "medium"
			}
		case "INSERT", "REPLACE", "CREATE":
			if riskLevel != "high" {
				riskLevel = "medium"
			}
		default:
			if !isReadOnlySQL(statement) {
				riskLevel = "high"
				reasons = append(reasons, "contains SQL that cannot be verified as read-only")
			}
		}
	}
	if len(statements) > 1 {
		reasons = append(reasons, fmt.Sprintf("will execute %d SQL statements sequentially", len(statements)))
		if writeOperation && riskLevel == "low" {
			riskLevel = "medium"
		}
	}
	if writeOperation && normalizeEnvCode(item.Env) == "prod" {
		riskLevel = "high"
		reasons = append(reasons, "target database belongs to the production environment")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "no obvious high-risk characteristics detected")
	}
	return &DBMSSQLAnalysis{
		SQLType:        sqlType,
		StatementCount: len(statements),
		WriteOperation: writeOperation,
		RiskLevel:      riskLevel,
		Reasons:        reasons,
		DatabaseName:   item.Name,
		Schema:         defaultSchema(item, payload.Schema),
		Environment:    item.Env,
		AccessMode:     normalizeDatabaseAccessMode(item.AccessMode),
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
	asset, err := s.getAssetDatabase(payload.DatabaseID)
	if err != nil {
		return nil, err
	}
	if normalizeDatabaseType(asset.DBType) == "postgresql" {
		return s.insertPostgresTableRow(asset, payload)
	}
	item, db, cleanup, err := s.openDatabaseByID(payload.DatabaseID, payload.Schema)
	if err != nil {
		return nil, err
	}
	if err := ensureMySQLFeature(item); err != nil {
		db.Close()
		cleanup()
		return nil, err
	}
	if err := ensureDatabaseWritable(item); err != nil {
		db.Close()
		cleanup()
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
		return nil, errors.New("no data is available to insert")
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
	asset, err := s.getAssetDatabase(payload.DatabaseID)
	if err != nil {
		return nil, err
	}
	if normalizeDatabaseType(asset.DBType) == "postgresql" {
		return s.updatePostgresTableRow(asset, payload)
	}
	item, db, cleanup, err := s.openDatabaseByID(payload.DatabaseID, payload.Schema)
	if err != nil {
		return nil, err
	}
	if err := ensureMySQLFeature(item); err != nil {
		db.Close()
		cleanup()
		return nil, err
	}
	if err := ensureDatabaseWritable(item); err != nil {
		db.Close()
		cleanup()
		return nil, err
	}
	defer db.Close()
	defer cleanup()
	schema := defaultSchema(item, payload.Schema)
	columns, err := s.getTableColumns(db, schema, payload.Table)
	if err != nil {
		return nil, err
	}
	if !databaseColumnsHavePrimaryKey(columns) {
		return nil, errors.New("the current table has no primary key; direct result-set editing is not allowed")
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
	asset, err := s.getAssetDatabase(payload.DatabaseID)
	if err != nil {
		return nil, err
	}
	if normalizeDatabaseType(asset.DBType) == "postgresql" {
		return s.deletePostgresTableRow(asset, payload)
	}
	item, db, cleanup, err := s.openDatabaseByID(payload.DatabaseID, payload.Schema)
	if err != nil {
		return nil, err
	}
	if err := ensureMySQLFeature(item); err != nil {
		db.Close()
		cleanup()
		return nil, err
	}
	if err := ensureDatabaseWritable(item); err != nil {
		db.Close()
		cleanup()
		return nil, err
	}
	defer db.Close()
	defer cleanup()
	schema := defaultSchema(item, payload.Schema)
	columns, err := s.getTableColumns(db, schema, payload.Table)
	if err != nil {
		return nil, err
	}
	if !databaseColumnsHavePrimaryKey(columns) {
		return nil, errors.New("the current table has no primary key; deleting result-set rows is not allowed")
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
