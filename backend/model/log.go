package model

import "time"

type LoginLog struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	Username      string    `json:"username" gorm:"size:64"`
	IPAddress     string    `json:"ipAddress" gorm:"size:64"`
	LoginLocation string    `json:"loginLocation" gorm:"size:128"`
	Browser       string    `json:"browser" gorm:"size:128"`
	OS            string    `json:"os" gorm:"size:128"`
	LoginStatus   int       `json:"loginStatus" gorm:"default:1;not null"`
	Message       string    `json:"message" gorm:"size:255"`
	LoginTime     time.Time `json:"loginTime"`
}

func (LoginLog) TableName() string {
	return "sys_login_info"
}

type OperationLog struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	AdminID        uint      `json:"adminId"`
	Username       string    `json:"username" gorm:"size:64"`
	Method         string    `json:"method" gorm:"size:16"`
	IP             string    `json:"ip" gorm:"size:64"`
	URL            string    `json:"url" gorm:"size:255"`
	Description    string    `json:"description" gorm:"size:255"`
	RiskLevel      string    `json:"riskLevel" gorm:"size:32;default:'normal'"`
	StatusCode     int       `json:"statusCode" gorm:"default:200"`
	Success        bool      `json:"success" gorm:"default:true"`
	DurationMs     int64     `json:"durationMs" gorm:"default:0"`
	RequestSummary string    `json:"requestSummary" gorm:"type:text"`
	CreatedAt      time.Time `json:"createTime"`
}

func (OperationLog) TableName() string {
	return "sys_operation_log"
}
