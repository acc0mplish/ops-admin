package service

import (
	"database/sql"
	"fmt"
	"strings"

	"ops-admin/backend/model"
)

func postgresQuoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(strings.TrimSpace(name), `"`, `""`) + `"`
}

func (s *Service) getPostgresResourceData(item *model.AssetDatabase, payload DBMSTableDataQueryPayload) (map[string]any, error) {
	db, cleanup, err := s.openAssetPostgresDatabase(*item)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	defer db.Close()
	schema := strings.TrimSpace(payload.Schema)
	if schema == "" {
		schema = "public"
	}
	columnsRows, err := db.Query(`SELECT column_name, data_type FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`, schema, payload.Table)
	if err != nil {
		return nil, err
	}
	columns := make([]databaseTableColumn, 0)
	columnNames := make([]string, 0)
	for columnsRows.Next() {
		var name, dataType string
		if err := columnsRows.Scan(&name, &dataType); err != nil {
			columnsRows.Close()
			return nil, err
		}
		columns = append(columns, databaseTableColumn{Name: name, DataType: dataType, ColumnType: dataType})
		columnNames = append(columnNames, name)
	}
	columnsRows.Close()
	if len(columnNames) == 0 {
		return nil, fmt.Errorf("resource does not exist or has no readable columns")
	}
	if payload.FilterKey != "" {
		valid := false
		for _, name := range columnNames {
			if name == payload.FilterKey {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("invalid filter column")
		}
	}
	from := postgresQuoteIdentifier(schema) + "." + postgresQuoteIdentifier(payload.Table)
	where := ""
	args := make([]any, 0, 3)
	if strings.TrimSpace(payload.FilterText) != "" {
		if payload.FilterKey != "" {
			where = " WHERE CAST(" + postgresQuoteIdentifier(payload.FilterKey) + " AS TEXT) ILIKE $1"
			args = append(args, "%"+strings.TrimSpace(payload.FilterText)+"%")
		} else {
			clauses := make([]string, 0, len(columnNames))
			for _, name := range columnNames {
				clauses = append(clauses, "CAST("+postgresQuoteIdentifier(name)+" AS TEXT) ILIKE $1")
			}
			where = " WHERE " + strings.Join(clauses, " OR ")
			args = append(args, "%"+strings.TrimSpace(payload.FilterText)+"%")
		}
	}
	var total int64
	if err := db.QueryRow("SELECT COUNT(*) FROM "+from+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	queryArgs := append([]any{}, args...)
	limitPos := len(queryArgs) + 1
	offsetPos := len(queryArgs) + 2
	queryArgs = append(queryArgs, payload.PageSize, (payload.PageNum-1)*payload.PageSize)
	rows, err := db.Query("SELECT * FROM "+from+where+fmt.Sprintf(" LIMIT $%d OFFSET $%d", limitPos, offsetPos), queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	_, dataRows, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	return map[string]any{"schema": schema, "table": payload.Table, "columns": columns, "rows": dataRows, "total": total, "pageNum": payload.PageNum, "pageSize": payload.PageSize, "readOnly": true, "resourceType": "table"}, nil
}

func postgresTableColumns(db *sql.DB, schema, table string) ([]databaseTableColumn, error) {
	rows, err := db.Query(`
		SELECT
			columns.column_name,
			columns.data_type,
			columns.udt_name,
			CASE WHEN primary_key.column_name IS NULL THEN '' ELSE 'PRI' END AS column_key,
			columns.is_nullable,
			columns.column_default,
			CASE WHEN columns.is_identity = 'YES' THEN 'identity' ELSE '' END AS extra
		FROM information_schema.columns AS columns
		LEFT JOIN (
			SELECT key_columns.table_schema, key_columns.table_name, key_columns.column_name
			FROM information_schema.table_constraints AS constraints
			JOIN information_schema.key_column_usage AS key_columns
				ON constraints.constraint_name = key_columns.constraint_name
				AND constraints.table_schema = key_columns.table_schema
				AND constraints.table_name = key_columns.table_name
			WHERE constraints.constraint_type = 'PRIMARY KEY'
		) AS primary_key
			ON primary_key.table_schema = columns.table_schema
			AND primary_key.table_name = columns.table_name
			AND primary_key.column_name = columns.column_name
		WHERE columns.table_schema = $1 AND columns.table_name = $2
		ORDER BY columns.ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make([]databaseTableColumn, 0)
	for rows.Next() {
		var column databaseTableColumn
		if err := rows.Scan(&column.Name, &column.DataType, &column.ColumnType, &column.ColumnKey, &column.IsNullable, &column.ColumnDefault, &column.Extra); err != nil {
			return nil, err
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func postgresTableName(schema, table string) string {
	return postgresQuoteIdentifier(schema) + "." + postgresQuoteIdentifier(table)
}

func postgresPrimaryKeys(columns []databaseTableColumn) []string {
	keys := make([]string, 0)
	for _, column := range columns {
		if strings.EqualFold(column.ColumnKey, "PRI") {
			keys = append(keys, column.Name)
		}
	}
	return keys
}

func postgresJoinIdentifiers(columns []string) string {
	items := make([]string, 0, len(columns))
	for _, column := range columns {
		items = append(items, postgresQuoteIdentifier(column))
	}
	return strings.Join(items, ", ")
}

func postgresFilterClause(columns []databaseTableColumn, key, text string) (string, []any, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil, nil
	}
	key = strings.TrimSpace(key)
	if key != "" {
		for _, column := range columns {
			if column.Name == key {
				return " WHERE CAST(" + postgresQuoteIdentifier(key) + " AS TEXT) ILIKE $1", []any{"%" + text + "%"}, nil
			}
		}
		return "", nil, fmt.Errorf("filter column does not exist")
	}
	parts := make([]string, 0, len(columns))
	for _, column := range columns {
		parts = append(parts, "CAST("+postgresQuoteIdentifier(column.Name)+" AS TEXT) ILIKE $1")
	}
	if len(parts) == 0 {
		return "", nil, nil
	}
	return " WHERE (" + strings.Join(parts, " OR ") + ")", []any{"%" + text + "%"}, nil
}

func postgresWhereByPrimaryKey(columns []databaseTableColumn, row map[string]any, offset int) (string, []any, error) {
	keys := postgresPrimaryKeys(columns)
	if len(keys) == 0 {
		return "", nil, fmt.Errorf("the current table has no primary key; rows cannot be updated or deleted directly")
	}
	clauses := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys))
	for index, key := range keys {
		value, exists := row[key]
		if !exists {
			return "", nil, fmt.Errorf("missing primary-key column %s", key)
		}
		clauses = append(clauses, fmt.Sprintf("%s = $%d", postgresQuoteIdentifier(key), offset+index+1))
		args = append(args, normalizeJSONValue(value))
	}
	return strings.Join(clauses, " AND "), args, nil
}

func postgresSQLLiteral(value any) string {
	if boolean, ok := value.(bool); ok {
		if boolean {
			return "TRUE"
		}
		return "FALSE"
	}
	return sqlLiteral(value)
}

func postgresRollbackWhere(columns []databaseTableColumn, row map[string]any) string {
	keys := postgresPrimaryKeys(columns)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if value, exists := row[key]; exists {
			parts = append(parts, postgresQuoteIdentifier(key)+" = "+postgresSQLLiteral(value))
		}
	}
	return strings.Join(parts, " AND ")
}

func postgresInsertRollback(schema, table string, columns []databaseTableColumn, row map[string]any) string {
	where := postgresRollbackWhere(columns, row)
	if where == "" {
		return ""
	}
	return fmt.Sprintf("DELETE FROM %s WHERE %s;", postgresTableName(schema, table), where)
}

func postgresUpdateRollback(schema, table string, columns []databaseTableColumn, row map[string]any) string {
	sets := make([]string, 0)
	for _, column := range columns {
		if value, exists := row[column.Name]; exists {
			sets = append(sets, postgresQuoteIdentifier(column.Name)+" = "+postgresSQLLiteral(value))
		}
	}
	where := postgresRollbackWhere(columns, row)
	if len(sets) == 0 || where == "" {
		return ""
	}
	return fmt.Sprintf("UPDATE %s SET %s WHERE %s;", postgresTableName(schema, table), strings.Join(sets, ", "), where)
}

func postgresDeleteRollback(schema, table string, columns []databaseTableColumn, row map[string]any) string {
	keys := make([]string, 0)
	values := make([]string, 0)
	for _, column := range columns {
		if value, exists := row[column.Name]; exists {
			keys = append(keys, column.Name)
			values = append(values, postgresSQLLiteral(value))
		}
	}
	if len(keys) == 0 {
		return ""
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);", postgresTableName(schema, table), postgresJoinIdentifiers(keys), strings.Join(values, ", "))
}

func (s *Service) getPostgresTableData(item *model.AssetDatabase, payload DBMSTableDataQueryPayload) (map[string]any, error) {
	if payload.PageNum < 1 {
		payload.PageNum = 1
	}
	if payload.PageSize < 1 || payload.PageSize > 200 {
		payload.PageSize = 25
	}
	schema := defaultSchema(item, payload.Schema)
	db, cleanup, err := s.openAssetPostgresDatabase(*item)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	defer db.Close()

	columns, err := postgresTableColumns(db, schema, payload.Table)
	if err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("table does not exist or has no readable columns")
	}
	where, args, err := postgresFilterClause(columns, payload.FilterKey, payload.FilterText)
	if err != nil {
		return nil, err
	}
	from := postgresTableName(schema, payload.Table)
	var total int64
	if err := db.QueryRow("SELECT COUNT(*) FROM "+from+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	queryArgs := append([]any{}, args...)
	limitPos := len(queryArgs) + 1
	offsetPos := len(queryArgs) + 2
	queryArgs = append(queryArgs, payload.PageSize, (payload.PageNum-1)*payload.PageSize)
	rows, err := db.Query("SELECT * FROM "+from+where+fmt.Sprintf(" LIMIT $%d OFFSET $%d", limitPos, offsetPos), queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columnNames, dataRows, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"schema":      schema,
		"table":       payload.Table,
		"columns":     columns,
		"columnNames": columnNames,
		"rows":        dataRows,
		"primaryKeys": postgresPrimaryKeys(columns),
		"total":       total,
		"pageNum":     payload.PageNum,
		"pageSize":    payload.PageSize,
		"filterKey":   payload.FilterKey,
		"filterText":  payload.FilterText,
	}, nil
}

func (s *Service) insertPostgresTableRow(item *model.AssetDatabase, payload DBMSTableInsertPayload) (map[string]any, error) {
	if err := ensureDatabaseWritable(item); err != nil {
		return nil, err
	}
	schema := defaultSchema(item, payload.Schema)
	db, cleanup, err := s.openAssetPostgresDatabase(*item)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	defer db.Close()
	columns, err := postgresTableColumns(db, schema, payload.Table)
	if err != nil {
		return nil, err
	}
	insertColumns := make([]string, 0)
	values := make([]any, 0)
	for _, column := range columns {
		if value, exists := payload.Row[column.Name]; exists {
			insertColumns = append(insertColumns, column.Name)
			values = append(values, normalizeJSONValue(value))
		}
	}
	if len(insertColumns) == 0 {
		return nil, fmt.Errorf("no rows are available to insert")
	}
	placeholders := make([]string, 0, len(insertColumns))
	for index := range insertColumns {
		placeholders = append(placeholders, fmt.Sprintf("$%d", index+1))
	}
	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", postgresTableName(schema, payload.Table), postgresJoinIdentifiers(insertColumns), strings.Join(placeholders, ", "))
	primaryKeys := postgresPrimaryKeys(columns)
	insertedRow := make(map[string]any, len(payload.Row))
	for key, value := range payload.Row {
		insertedRow[key] = value
	}
	var affected int64 = 1
	if len(primaryKeys) > 0 {
		returningSQL := insertSQL + " RETURNING " + postgresJoinIdentifiers(primaryKeys)
		scanTargets := make([]any, len(primaryKeys))
		for index := range scanTargets {
			scanTargets[index] = new(any)
		}
		if err := db.QueryRow(returningSQL, values...).Scan(scanTargets...); err != nil {
			return nil, err
		}
		for index, key := range primaryKeys {
			insertedRow[key] = *(scanTargets[index].(*any))
		}
	} else {
		result, err := db.Exec(insertSQL, values...)
		if err != nil {
			return nil, err
		}
		affected, _ = result.RowsAffected()
	}
	rollbackSQL := postgresInsertRollback(schema, payload.Table, columns, insertedRow)
	s.logDBSQLHistory(item, schema, payload.Table, "INSERT", insertSQL, 1, affected, 0, "", rollbackSQL)
	return map[string]any{"rowsAffected": affected, "rollbackSql": rollbackSQL}, nil
}

func (s *Service) updatePostgresTableRow(item *model.AssetDatabase, payload DBMSTableUpdatePayload) (map[string]any, error) {
	if err := ensureDatabaseWritable(item); err != nil {
		return nil, err
	}
	schema := defaultSchema(item, payload.Schema)
	db, cleanup, err := s.openAssetPostgresDatabase(*item)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	defer db.Close()
	columns, err := postgresTableColumns(db, schema, payload.Table)
	if err != nil {
		return nil, err
	}
	if !databaseColumnsHavePrimaryKey(columns) {
		return nil, fmt.Errorf("the current table has no primary key; rows cannot be edited directly")
	}
	setParts := make([]string, 0)
	args := make([]any, 0)
	for _, column := range columns {
		current, exists := payload.Current[column.Name]
		if !exists || fmt.Sprintf("%v", normalizeJSONValue(current)) == fmt.Sprintf("%v", normalizeJSONValue(payload.Original[column.Name])) {
			continue
		}
		setParts = append(setParts, fmt.Sprintf("%s = $%d", postgresQuoteIdentifier(column.Name), len(args)+1))
		args = append(args, normalizeJSONValue(current))
	}
	if len(setParts) == 0 {
		return map[string]any{"rowsAffected": 0, "rollbackSql": ""}, nil
	}
	where, whereArgs, err := postgresWhereByPrimaryKey(columns, payload.Original, len(args))
	if err != nil {
		return nil, err
	}
	args = append(args, whereArgs...)
	updateSQL := fmt.Sprintf("UPDATE %s SET %s WHERE %s", postgresTableName(schema, payload.Table), strings.Join(setParts, ", "), where)
	result, err := db.Exec(updateSQL, args...)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	rollbackSQL := postgresUpdateRollback(schema, payload.Table, columns, payload.Original)
	s.logDBSQLHistory(item, schema, payload.Table, "UPDATE", updateSQL, 1, affected, 0, "", rollbackSQL)
	return map[string]any{"rowsAffected": affected, "rollbackSql": rollbackSQL}, nil
}

func (s *Service) deletePostgresTableRow(item *model.AssetDatabase, payload DBMSTableDeletePayload) (map[string]any, error) {
	if err := ensureDatabaseWritable(item); err != nil {
		return nil, err
	}
	schema := defaultSchema(item, payload.Schema)
	db, cleanup, err := s.openAssetPostgresDatabase(*item)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	defer db.Close()
	columns, err := postgresTableColumns(db, schema, payload.Table)
	if err != nil {
		return nil, err
	}
	where, args, err := postgresWhereByPrimaryKey(columns, payload.Row, 0)
	if err != nil {
		return nil, err
	}
	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE %s", postgresTableName(schema, payload.Table), where)
	result, err := db.Exec(deleteSQL, args...)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	rollbackSQL := postgresDeleteRollback(schema, payload.Table, columns, payload.Row)
	s.logDBSQLHistory(item, schema, payload.Table, "DELETE", deleteSQL, 1, affected, 0, "", rollbackSQL)
	return map[string]any{"rowsAffected": affected, "rollbackSql": rollbackSQL}, nil
}
