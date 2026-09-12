package service

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"ops-admin/backend/model"
)

func (s *Service) ExportDatabaseTable(databaseID uint, schema string, table string, includeData bool) ([]byte, string, error) {
	if databaseID == 0 || strings.TrimSpace(table) == "" {
		return nil, "", errors.New("select a table first")
	}
	item, db, cleanup, err := s.openDatabaseByID(databaseID, schema)
	if err != nil {
		return nil, "", err
	}
	if err := ensureMySQLFeature(item); err != nil {
		db.Close()
		cleanup()
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
		return nil, "", errors.New("failed to retrieve the CREATE TABLE statement")
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
		return nil, errors.New("select source and target databases")
	}
	if strings.TrimSpace(payload.SourceTable) == "" {
		return nil, errors.New("select a source table")
	}
	sourceAsset, sourceDB, sourceCleanup, err := s.openDatabaseByID(payload.SourceDatabaseID, payload.SourceSchema)
	if err != nil {
		return nil, err
	}
	if err := ensureRelationalImportFeature(sourceAsset); err != nil {
		sourceDB.Close()
		sourceCleanup()
		return nil, err
	}
	defer sourceDB.Close()
	defer sourceCleanup()

	targetAsset, targetDB, targetCleanup, err := s.openDatabaseByID(payload.TargetDatabaseID, payload.TargetSchema)
	if err != nil {
		return nil, err
	}
	if err := ensureRelationalImportFeature(targetAsset); err != nil {
		targetDB.Close()
		targetCleanup()
		return nil, err
	}
	defer targetDB.Close()
	defer targetCleanup()
	if err := ensureDatabaseWritable(targetAsset); err != nil {
		return nil, err
	}

	sourceSchema := defaultSchema(sourceAsset, payload.SourceSchema)
	targetSchema := defaultSchema(targetAsset, payload.TargetSchema)
	targetTable := strings.TrimSpace(payload.TargetTable)
	if targetTable == "" {
		targetTable = strings.TrimSpace(payload.SourceTable)
	}

	if payload.CreateIfMissing {
		sourceType := normalizeDatabaseType(sourceAsset.DBType)
		targetType := normalizeDatabaseType(targetAsset.DBType)
		if sourceType != targetType {
			return nil, errors.New("automatic table creation is not supported for cross-database-type imports; create the target table first")
		}
		if sourceType == "postgresql" {
			if err := s.createPostgresImportTable(sourceDB, targetDB, sourceSchema, payload.SourceTable, targetSchema, targetTable); err != nil {
				return nil, err
			}
		} else {
			showSQL := fmt.Sprintf("SHOW CREATE TABLE %s", importTableName(sourceAsset, sourceSchema, payload.SourceTable))
			rows, err := sourceDB.Query(showSQL)
			if err != nil {
				return nil, err
			}
			_, list, err := scanRows(rows)
			rows.Close()
			if err != nil || len(list) == 0 {
				return nil, errors.New("failed to retrieve source-table schema")
			}
			createSQL := mysqlImportCreateTableSQL(fmt.Sprintf("%v", list[0]["Create Table"]), payload.SourceTable, targetTable)
			if _, err := targetDB.Exec(createSQL); err != nil {
				return nil, err
			}
		}
	}

	sourceColumns, err := s.getImportTableColumns(sourceAsset, sourceDB, sourceSchema, payload.SourceTable)
	if err != nil {
		return nil, err
	}
	targetColumns, err := s.getImportTableColumns(targetAsset, targetDB, targetSchema, targetTable)
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
		return nil, errors.New("source and target tables have no matching columns")
	}

	if payload.TruncateTarget {
		if _, err := targetDB.Exec(fmt.Sprintf("TRUNCATE TABLE %s", importTableName(targetAsset, targetSchema, targetTable))); err != nil {
			return nil, err
		}
	}

	selectSQL := fmt.Sprintf(
		"SELECT %s FROM %s",
		importJoinIdentifiers(sourceAsset, commonColumns),
		importTableName(sourceAsset, sourceSchema, payload.SourceTable),
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
	placeholders := importPlaceholders(targetAsset, len(commonColumns))
	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		importTableName(targetAsset, targetSchema, targetTable),
		importJoinIdentifiers(targetAsset, commonColumns),
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
		return nil, errors.New("select tables to export first")
	}
	asset, err := s.getAssetDatabase(payload.DatabaseID)
	if err != nil {
		return nil, err
	}
	task := model.DatabaseTransferTask{
		TaskType:     "export",
		Status:       "pending",
		Progress:     0,
		Message:      "Pending execution",
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
		return nil, errors.New("select source and target databases")
	}
	precheck, err := s.PrecheckImportTask(payload)
	if err != nil {
		return nil, err
	}
	if !precheck.Ready {
		return nil, errors.New("import precheck failed; resolve the risk items and retry")
	}
	sourceAsset, err := s.getAssetDatabase(payload.SourceDatabaseID)
	if err != nil {
		return nil, err
	}
	targetAsset, err := s.getAssetDatabase(payload.TargetDatabaseID)
	if err != nil {
		return nil, err
	}
	if err := ensureDatabaseWritable(targetAsset); err != nil {
		return nil, err
	}
	task := model.DatabaseTransferTask{
		TaskType:         "import",
		Status:           "pending",
		Progress:         0,
		Message:          "Pending execution",
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

func (s *Service) PrecheckImportTask(payload DBMSImportPayload) (*DBMSImportPrecheck, error) {
	if payload.SourceDatabaseID == 0 || payload.TargetDatabaseID == 0 || strings.TrimSpace(payload.SourceTable) == "" {
		return nil, errors.New("select a source database, source table, and target database")
	}
	sourceAsset, sourceDB, sourceCleanup, err := s.openDatabaseByID(payload.SourceDatabaseID, payload.SourceSchema)
	if err != nil {
		return nil, err
	}
	if err := ensureRelationalImportFeature(sourceAsset); err != nil {
		sourceDB.Close()
		sourceCleanup()
		return nil, err
	}
	defer sourceDB.Close()
	defer sourceCleanup()
	targetAsset, targetDB, targetCleanup, err := s.openDatabaseByID(payload.TargetDatabaseID, payload.TargetSchema)
	if err != nil {
		return nil, err
	}
	if err := ensureRelationalImportFeature(targetAsset); err != nil {
		targetDB.Close()
		targetCleanup()
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
	sourceColumns, err := s.getImportTableColumns(sourceAsset, sourceDB, sourceSchema, payload.SourceTable)
	if err != nil {
		return nil, err
	}
	if len(sourceColumns) == 0 {
		return nil, errors.New("source table does not exist or has no importable columns")
	}
	targetColumns, err := s.getImportTableColumns(targetAsset, targetDB, targetSchema, targetTable)
	if err != nil {
		return nil, err
	}
	var estimatedRows int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s", importTableName(sourceAsset, sourceSchema, payload.SourceTable))
	if err := sourceDB.QueryRow(countSQL).Scan(&estimatedRows); err != nil {
		return nil, err
	}

	targetNames := make(map[string]struct{}, len(targetColumns))
	for _, column := range targetColumns {
		targetNames[column.Name] = struct{}{}
	}
	commonColumns := make([]string, 0)
	missingColumns := make([]string, 0)
	for _, column := range sourceColumns {
		if _, exists := targetNames[column.Name]; exists || len(targetColumns) == 0 && payload.CreateIfMissing {
			commonColumns = append(commonColumns, column.Name)
		} else {
			missingColumns = append(missingColumns, column.Name)
		}
	}
	warnings := make([]string, 0)
	if normalizeDatabaseAccessMode(targetAsset.AccessMode) == "readonly" {
		warnings = append(warnings, "target database is read-only")
	}
	if len(targetColumns) == 0 && !payload.CreateIfMissing {
		warnings = append(warnings, "target table does not exist and automatic table creation is disabled")
	}
	if normalizeDatabaseType(sourceAsset.DBType) != normalizeDatabaseType(targetAsset.DBType) && payload.CreateIfMissing {
		warnings = append(warnings, "cross-database-type imports do not support automatic table creation; create the target table first")
	}
	if payload.TruncateTarget {
		warnings = append(warnings, "all target-table data will be cleared before import")
	}
	if len(missingColumns) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d source columns cannot be mapped to the target table", len(missingColumns)))
	}
	ready := normalizeDatabaseAccessMode(targetAsset.AccessMode) != "readonly" &&
		(len(targetColumns) > 0 || (payload.CreateIfMissing && normalizeDatabaseType(sourceAsset.DBType) == normalizeDatabaseType(targetAsset.DBType))) &&
		len(commonColumns) > 0
	return &DBMSImportPrecheck{
		SourceDatabase: sourceAsset.Name,
		SourceSchema:   sourceSchema,
		SourceTable:    payload.SourceTable,
		TargetDatabase: targetAsset.Name,
		TargetSchema:   targetSchema,
		TargetTable:    targetTable,
		EstimatedRows:  estimatedRows,
		TargetExists:   len(targetColumns) > 0,
		CommonColumns:  commonColumns,
		MissingColumns: missingColumns,
		Warnings:       warnings,
		Ready:          ready,
	}, nil
}

func (s *Service) CreateBatchSQLTask(payload DBMSBatchSQLPayload) (map[string]any, error) {
	if payload.DatabaseID == 0 || strings.TrimSpace(payload.SQLText) == "" {
		return nil, errors.New("select a database and provide SQL content")
	}
	analysis, err := s.AnalyzeDatabaseSQL(DBMSSQLExecutePayload{
		DatabaseID: payload.DatabaseID,
		Schema:     payload.Schema,
		SQLText:    payload.SQLText,
	})
	if err != nil {
		return nil, err
	}
	if analysis.WriteOperation && analysis.AccessMode == "readonly" {
		return nil, errors.New("the current database is read-only; batch SQL is not allowed")
	}
	if analysis.WriteOperation && !payload.Confirmed {
		return nil, errors.New("batch SQL writes require confirmation before execution")
	}
	executionMode := strings.ToLower(strings.TrimSpace(payload.ExecutionMode))
	if executionMode != "transaction" {
		executionMode = "sequential"
	}
	if executionMode == "transaction" {
		for _, statement := range splitDBMSSQLStatements(payload.SQLText) {
			switch detectSQLType(statement) {
			case "CREATE", "ALTER", "DROP", "TRUNCATE", "RENAME", "GRANT", "REVOKE":
				return nil, errors.New("transactional execution does not support DDL or privilege statements; use sequential execution")
			}
		}
	}
	asset, err := s.getAssetDatabase(payload.DatabaseID)
	if err != nil {
		return nil, err
	}
	fileName := strings.TrimSpace(payload.FileName)
	if fileName == "" {
		fileName = "batch.sql"
	}
	task := model.DatabaseTransferTask{
		TaskType:      "batch_sql",
		Status:        "pending",
		Progress:      0,
		Message:       "Pending execution",
		DatabaseID:    asset.ID,
		DatabaseName:  asset.Name,
		SchemaName:    defaultSchema(asset, payload.Schema),
		FileName:      fileName,
		FileContent:   payload.SQLText,
		ExecutionMode: executionMode,
		Operator:      payload.Operator,
	}
	if err := s.db.Create(&task).Error; err != nil {
		return nil, err
	}
	go s.runBatchSQLTask(task.ID, payload.ClientIP)
	return map[string]any{"taskId": task.ID, "analysis": analysis}, nil
}

func (s *Service) runBatchSQLTask(taskID uint, clientIP string) {
	var task model.DatabaseTransferTask
	if err := s.db.First(&task, taskID).Error; err != nil {
		return
	}
	startedAt := time.Now()
	_ = s.db.Model(&task).Updates(map[string]any{"status": "running", "progress": 5, "message": "Executing SQL", "started_at": &startedAt}).Error
	statements := splitDBMSSQLStatements(task.FileContent)
	var rowsAffected int64
	var runErr error
	if task.ExecutionMode == "transaction" {
		item, db, cleanup, err := s.openDatabaseByID(task.DatabaseID, task.SchemaName)
		if err != nil {
			runErr = err
		} else {
			defer db.Close()
			defer cleanup()
			tx, err := db.Begin()
			if err != nil {
				runErr = err
			} else {
				for index, statement := range statements {
					result, err := tx.Exec(statement)
					if err != nil {
						runErr = fmt.Errorf("SQL statement %d failed: %w", index+1, err)
						_ = tx.Rollback()
						break
					}
					affected, _ := result.RowsAffected()
					rowsAffected += affected
					progress := 5 + int(float64(index+1)/float64(len(statements))*85)
					_ = s.db.Model(&task).Updates(map[string]any{"progress": progress, "message": fmt.Sprintf("executed %d/%d statements", index+1, len(statements))}).Error
				}
				if runErr == nil {
					runErr = tx.Commit()
				}
				status := 1
				errText := ""
				if runErr != nil {
					status = 2
					errText = runErr.Error()
				}
				s.logDBSQLHistory(item, task.SchemaName, "", "BATCH", task.FileContent, status, rowsAffected, time.Since(startedAt).Milliseconds(), errText, "")
			}
		}
	} else {
		for index, statement := range statements {
			result, err := s.ExecuteDatabaseSQL(DBMSSQLExecutePayload{
				DatabaseID: task.DatabaseID,
				Schema:     task.SchemaName,
				SQLText:    statement,
				Confirmed:  true,
				Operator:   task.Operator,
				ClientIP:   clientIP,
			})
			if err != nil {
				runErr = fmt.Errorf("SQL statement %d failed: %w", index+1, err)
				break
			}
			switch value := result["rowsAffected"].(type) {
			case int64:
				rowsAffected += value
			case int:
				rowsAffected += int64(value)
			}
			progress := 5 + int(float64(index+1)/float64(len(statements))*85)
			_ = s.db.Model(&task).Updates(map[string]any{"progress": progress, "message": fmt.Sprintf("executed %d/%d statements", index+1, len(statements))}).Error
		}
	}
	finishedAt := time.Now()
	updates := map[string]any{"finished_at": &finishedAt, "rows_affected": rowsAffected}
	if runErr != nil {
		updates["status"] = "failed"
		updates["progress"] = 100
		updates["message"] = runErr.Error()
	} else {
		updates["status"] = "success"
		updates["progress"] = 100
		updates["message"] = fmt.Sprintf("execution completed: %d SQL statements", len(statements))
	}
	_ = s.db.Model(&task).Updates(updates).Error
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
		taskTypes := strings.Split(strings.TrimSpace(taskType), ",")
		query = query.Where("task_type IN ?", taskTypes)
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
		return nil, "", errors.New("the current task has no downloadable file")
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
	var db *sql.DB
	var cleanup func()
	switch normalizeDatabaseType(item.DBType) {
	case "mysql":
		db, cleanup, err = s.openAssetMySQLDatabase(*item, schema)
	case "postgresql":
		db, cleanup, err = s.openAssetPostgresDatabase(*item)
	default:
		return nil, nil, func() {}, errors.New("the current database type does not support SQL Workbench; review schema and connection status in the sidebar")
	}
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
