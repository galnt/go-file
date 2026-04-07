package model

import "time"

// Activity 活动主表
type Activity struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Location   string    `json:"location"`
	EventDate  string    `json:"event_date"`
	FilePath   string    `json:"file_path"` // 核心：主表封面图路径
	VisitCount int       `json:"visit_count"`
	OpenID     string    `json:"openid"`
	CreatedAt  time.Time `json:"created_at"`
}

// ActivityPhoto 活动照片从表
type ActivityPhoto struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ActivityID uint      `json:"activity_id"`
	FilePath   string    `json:"file_path"` // 从表多图路径
	Uploader   string    `json:"uploader"`
	CreatedAt  time.Time `json:"created_at"`
	IsActive   bool      `gorm:"default:true" json:"is_active"`
}
