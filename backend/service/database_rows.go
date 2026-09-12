package service

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"ops-admin/backend/model"
)

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
