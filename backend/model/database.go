package model

import "time"

type DatabaseSQLHistory struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	DatabaseID   uint      `json:"databaseId" gorm:"index;not null"`
	DatabaseName string    `json:"databaseName" gorm:"size:128"`
	SchemaName   string    `json:"schemaName" gorm:"size:128;index"`
	TargetTable  string    `json:"tableName" gorm:"column:table_name;size:128;index"`
	SQLType      string    `json:"sqlType" gorm:"size:32;index"`
	SQLText      string    `json:"sqlText" gorm:"type:longtext"`
	Status       int       `json:"status" gorm:"default:1;not null"`
	RowsAffected int64     `json:"rowsAffected" gorm:"default:0"`
	DurationMs   int64     `json:"durationMs" gorm:"default:0"`
	ErrorMessage string    `json:"errorMessage" gorm:"type:text"`
	RollbackSQL  string    `json:"rollbackSql" gorm:"type:longtext"`
	CreatedAt    time.Time `json:"createTime"`
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
	RowsAffected     int64      `json:"rowsAffected" gorm:"default:0"`
	StartedAt        *time.Time `json:"startedAt"`
	FinishedAt       *time.Time `json:"finishedAt"`
	CreatedAt        time.Time  `json:"createTime"`
	UpdatedAt        time.Time  `json:"updateTime"`
}

func (DatabaseTransferTask) TableName() string {
	return "dbms_transfer_task"
}
