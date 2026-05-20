package model

import "time"

type Dept struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	ParentID   uint      `json:"parentId"`
	DeptType   int       `json:"deptType" gorm:"default:3;not null"`
	DeptName   string    `json:"deptName" gorm:"size:64;not null"`
	DeptStatus int       `json:"deptStatus" gorm:"default:1;not null"`
	CreatedAt  time.Time `json:"createTime"`
	UpdatedAt  time.Time `json:"updateTime"`
}

func (Dept) TableName() string {
	return "sys_dept"
}
