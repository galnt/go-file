package model

import "time"

// --- 数据库模型 ---
type WeChatUser struct {
	ID        uint      `gorm:"primaryKey"`
	OpenID    string    `gorm:"uniqueIndex"`
	NickName  string    `json:"nickName"`
	AvatarURL string    `json:"avatarUrl"`
	LastLogin time.Time `json:"last_login"`
}
