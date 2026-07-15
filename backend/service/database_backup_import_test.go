package service

import (
	"strings"
	"testing"
)

func TestNormalizeDatabaseRestoreScriptTargetsSelectedSchema(t *testing.T) {
	script := "-- Schema: source_db\nCREATE DATABASE `source_db`;\nUSE `source_db`;\nDROP TABLE IF EXISTS `source_db`.`users`;"
	normalized := normalizeDatabaseRestoreScript(script, "target_db")
	if strings.Contains(normalized, "CREATE DATABASE") || strings.Contains(normalized, "USE `source_db`") {
		t.Fatalf("restore script still controls the source schema: %s", normalized)
	}
	if !strings.Contains(normalized, "USE `target_db`;") || !strings.Contains(normalized, "`target_db`.`users`") {
		t.Fatalf("restore script did not target the selected schema: %s", normalized)
	}
}

func TestSplitMySQLRestoreStatementsKeepsDelimiterBlock(t *testing.T) {
	script := "USE `demo`;\nDELIMITER $$\nCREATE PROCEDURE `demo`.`p`()\nBEGIN\nSELECT 1;\nSELECT 2;\nEND$$\nDELIMITER ;\nINSERT INTO `demo`.`t` VALUES (1);\n"
	statements := splitMySQLRestoreStatements(script)
	if len(statements) != 3 {
		t.Fatalf("expected 3 statements, got %d: %#v", len(statements), statements)
	}
	if !strings.Contains(statements[1], "SELECT 1;") || !strings.Contains(statements[1], "SELECT 2;") {
		t.Fatalf("procedure body was split unexpectedly: %s", statements[1])
	}
}
