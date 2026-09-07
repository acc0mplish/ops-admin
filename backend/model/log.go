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

	// step0005 (V2 Phase 3, plan §3.5 — J3/F-8): the §18.1 audit EXTEND. The
	// named triplet plus the JSON context are all NULLABLE — every v1 row
	// stays NULL, no backfill, no defaults (F-8 ①). The remaining §18.1
	// fields (request/trace id, connection/context/resource uid, operation
	// metadata, approval fields, request/result hashes) live inside
	// v2_context as JSON text; assertions decode in Go (F-8 ② — no byte
	// compare). The v2 lane's request_summary carries hashes and error codes
	// only — never serialized request bodies (VK-11).
	TaskUID       *string `json:"taskUid" gorm:"size:64;index"`
	PolicyVersion *string `json:"policyVersion" gorm:"size:64"`
	Mutating      *bool   `json:"mutating"`
	V2Context     *string `json:"v2Context" gorm:"type:text"`
}

func (OperationLog) TableName() string {
	return "sys_operation_log"
}
