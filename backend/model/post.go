package model

import "time"

type Post struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	PostCode   string    `json:"postCode" gorm:"size:64;uniqueIndex;not null"`
	PostName   string    `json:"postName" gorm:"size:64;not null"`
	PostStatus int       `json:"postStatus" gorm:"default:1;not null"`
	Remark     string    `json:"remark" gorm:"size:255"`
	CreatedAt  time.Time `json:"createTime"`
	UpdatedAt  time.Time `json:"updateTime"`
}

func (Post) TableName() string {
	return "sys_post"
}
