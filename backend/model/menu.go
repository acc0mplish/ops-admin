package model

import "time"

type Menu struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	ParentID   uint      `json:"parentId"`
	MenuName   string    `json:"menuName" gorm:"size:100;not null"`
	Icon       string    `json:"icon" gorm:"size:100"`
	Value      string    `json:"value" gorm:"size:100"`
	MenuType   int       `json:"menuType" gorm:"default:2;not null"`
	URL        string    `json:"url" gorm:"size:200"`
	MenuStatus int       `json:"menuStatus" gorm:"default:1;not null"`
	Sort       int       `json:"sort" gorm:"default:0;not null"`
	CreatedAt  time.Time `json:"createTime"`
	UpdatedAt  time.Time `json:"updateTime"`
}

func (Menu) TableName() string {
	return "sys_menu"
}
