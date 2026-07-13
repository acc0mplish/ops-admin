package model

import "time"

type DatabaseSQLHistory struct {
	ID                 uint      `json:"id" gorm:"primaryKey"`
	DatabaseID         uint      `json:"databaseId" gorm:"index;not null"`
	DatabaseName       string    `json:"databaseName" gorm:"size:128"`
	SchemaName         string    `json:"schemaName" gorm:"size:128;index"`
	TargetTable        string    `json:"tableName" gorm:"column:table_name;size:128;index"`
	SQLType            string    `json:"sqlType" gorm:"size:32;index"`
	SQLText            string    `json:"sqlText" gorm:"type:longtext"`
	ExecutionID        string    `json:"executionId" gorm:"size:64;uniqueIndex"`
	Operator           string    `json:"operator" gorm:"size:128;index"`
	ClientIP           string    `json:"clientIp" gorm:"size:64;index"`
	Environment        string    `json:"environment" gorm:"size:64;index"`
	AccessMode         string    `json:"accessMode" gorm:"size:32"`
	Status             int       `json:"status" gorm:"default:1;not null"`
	RowsAffected       int64     `json:"rowsAffected" gorm:"default:0"`
	DurationMs         int64     `json:"durationMs" gorm:"default:0"`
	ErrorMessage       string    `json:"errorMessage" gorm:"type:text"`
	RollbackSQL        string    `json:"rollbackSql" gorm:"type:longtext"`
	RollbackConfidence string    `json:"rollbackConfidence" gorm:"size:32"`
	CreatedAt          time.Time `json:"createTime"`
}

func (DatabaseSQLHistory) TableName() string {
	return "dbms_sql_history"
}

type DatabaseTransferTask struct {
	ID               uint       `json:"id" gorm:"primaryKey"`
	TaskType         string     `json:"taskType" gorm:"size:32;index"`
	Status           string     `json:"status" gorm:"size:32;index"`
	Progress         int        `json:"progress" gorm:"default:0"`
	Message          string     `json:"message" gorm:"type:text"`
	DatabaseID       uint       `json:"databaseId" gorm:"index"`
	DatabaseName     string     `json:"databaseName" gorm:"size:128"`
	SchemaName       string     `json:"schemaName" gorm:"size:128"`
	PrimaryTable     string     `json:"tableName" gorm:"column:table_name;size:128"`
	SourceDatabaseID uint       `json:"sourceDatabaseId" gorm:"index"`
	SourceDatabase   string     `json:"sourceDatabase" gorm:"size:128"`
	SourceSchema     string     `json:"sourceSchema" gorm:"size:128"`
	SourceTable      string     `json:"sourceTable" gorm:"size:128"`
	TargetDatabaseID uint       `json:"targetDatabaseId" gorm:"index"`
	TargetDatabase   string     `json:"targetDatabase" gorm:"size:128"`
	TargetSchema     string     `json:"targetSchema" gorm:"size:128"`
	TargetTable      string     `json:"targetTable" gorm:"size:128"`
	FileName         string     `json:"fileName" gorm:"size:255"`
	FileContent      string     `json:"-" gorm:"type:longtext"`
	ExecutionMode    string     `json:"executionMode" gorm:"size:32"`
	Operator         string     `json:"operator" gorm:"size:128"`
	RowsAffected     int64      `json:"rowsAffected" gorm:"default:0"`
	StartedAt        *time.Time `json:"startedAt"`
	FinishedAt       *time.Time `json:"finishedAt"`
	CreatedAt        time.Time  `json:"createTime"`
	UpdatedAt        time.Time  `json:"updateTime"`
}

func (DatabaseTransferTask) TableName() string {
	return "dbms_transfer_task"
}

type DatabaseBackupPlan struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Name          string     `json:"name" gorm:"size:128;not null;index"`
	DatabaseID    uint       `json:"databaseId" gorm:"index;not null"`
	DatabaseName  string     `json:"databaseName" gorm:"size:128;index"`
	SchemaName    string     `json:"schemaName" gorm:"size:128"`
	CronExpr      string     `json:"cronExpr" gorm:"size:128;not null"`
	RetentionDays int        `json:"retentionDays" gorm:"default:7"`
	Status        int        `json:"status" gorm:"default:1;index"`
	Description   string     `json:"description" gorm:"size:255"`
	LastRunAt     *time.Time `json:"lastRunAt"`
	NextRunAt     *time.Time `json:"nextRunAt"`
	CreatedAt     time.Time  `json:"createTime"`
	UpdatedAt     time.Time  `json:"updateTime"`
}

func (DatabaseBackupPlan) TableName() string {
	return "dbms_backup_plan"
}

type DatabaseBackupRecord struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	PlanID       uint       `json:"planId" gorm:"index"`
	PlanName     string     `json:"planName" gorm:"size:128"`
	DatabaseID   uint       `json:"databaseId" gorm:"index;not null"`
	DatabaseName string     `json:"databaseName" gorm:"size:128;index"`
	SchemaName   string     `json:"schemaName" gorm:"size:128"`
	TriggerType  string     `json:"triggerType" gorm:"size:32;index"`
	Status       string     `json:"status" gorm:"size:32;index"`
	FileName     string     `json:"fileName" gorm:"size:255"`
	FileContent  string     `json:"-" gorm:"type:longtext"`
	FileSize     int64      `json:"fileSize"`
	Message      string     `json:"message" gorm:"type:text"`
	Operator     string     `json:"operator" gorm:"size:128"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	CreatedAt    time.Time  `json:"createTime"`
}

func (DatabaseBackupRecord) TableName() string {
	return "dbms_backup_record"
}
