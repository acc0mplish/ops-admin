package model

import "time"

type Role struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	RoleName    string    `json:"roleName" gorm:"size:64;not null"`
	RoleKey     string    `json:"roleKey" gorm:"size:64;uniqueIndex;not null"`
	Status      int       `json:"status" gorm:"default:1;not null"`
	Description string    `json:"description" gorm:"size:255"`
	CreatedAt   time.Time `json:"createTime"`
	UpdatedAt   time.Time `json:"updateTime"`
}

func (Role) TableName() string {
	return "sys_role"
}

type RoleMenu struct {
	ID     uint `json:"id" gorm:"primaryKey"`
	RoleID uint `json:"roleId" gorm:"index;not null"`
	MenuID uint `json:"menuId" gorm:"index;not null"`
}

func (RoleMenu) TableName() string {
	return "sys_role_menu"
}
