package service

import (
	"database/sql"
	"testing"

	"ops-admin/backend/model"
)

func TestSplitDBMSSQLStatements(t *testing.T) {
	statements := splitDBMSSQLStatements("SELECT 'a;b'; UPDATE users SET name = 'x' WHERE id = 1;")
	if len(statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(statements))
	}
}

func TestDetectSQLTypeSkipsComments(t *testing.T) {
	sqlType := detectSQLType("-- inspect users\nSELECT * FROM users")
	if sqlType != "SELECT" {
		t.Fatalf("expected SELECT, got %s", sqlType)
	}
}

func TestIsReadOnlySQL(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		readOnly bool
	}{
		{name: "select", sql: "SELECT * FROM users", readOnly: true},
		{name: "show", sql: "SHOW TABLES", readOnly: true},
		{name: "update", sql: "UPDATE users SET name = 'x' WHERE id = 1", readOnly: false},
		{name: "delete without where", sql: "DELETE FROM users", readOnly: false},
		{name: "read cte", sql: "WITH active AS (SELECT * FROM users) SELECT * FROM active", readOnly: true},
		{name: "write cte", sql: "WITH old AS (SELECT id FROM users) DELETE FROM users WHERE id IN (SELECT id FROM old)", readOnly: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := isReadOnlySQL(test.sql); actual != test.readOnly {
				t.Fatalf("expected readOnly=%v, got %v", test.readOnly, actual)
			}
		})
	}
}

func TestMySQLDumpLiteralEscapesValues(t *testing.T) {
	textType := &sql.ColumnType{}
	if actual := mysqlDumpLiteral("O'Reilly\\n\nnext", textType); actual != "'O\\'Reilly\\\\n\\nnext'" {
		t.Fatalf("unexpected escaped literal: %s", actual)
	}
	if actual := mysqlDumpLiteral(nil, textType); actual != "NULL" {
		t.Fatalf("expected NULL, got %s", actual)
	}
}

func TestResolveBackupSchema(t *testing.T) {
	asset := &model.AssetDatabase{DBName: "app_db"}
	if actual, err := resolveBackupSchema(asset, ""); err != nil || actual != "app_db" {
		t.Fatalf("expected app_db, got %q, err=%v", actual, err)
	}
	if _, err := resolveBackupSchema(&model.AssetDatabase{}, ""); err == nil {
		t.Fatal("expected an error for an unset schema")
	}
	if _, err := resolveBackupSchema(asset, "mysql"); err == nil {
		t.Fatal("expected an error for a system schema")
	}
}
