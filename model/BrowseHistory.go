package model

import "time"

// BrowseHistory 用户浏览活动记录表
type BrowseHistory struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	OpenID        string    `gorm:"index" json:"openid"`      // 用户OpenID，关联 WeChatUser
	NanoID        string    `gorm:"index" json:"nano_id"`     // 活动目录NanoID，关联 Activity.NanoID
	BrowseURL     string    `json:"browse_url"`               // 浏览的完整URL，如 http://127.0.0.1:3000/explorer?path=xxx
	GraphicRecord string    `json:"graphic_record"`           // 图形记录（截图路径、base64等）
	ViewTime      time.Time `gorm:"autoCreateTime" json:"view_time"`
}

// GetBrowseHistoryByOpenID 根据 OpenID 查询浏览记录（关联 Activity 信息）
func GetBrowseHistoryByOpenID(openid string) ([]BrowseHistory, error) {
	var records []BrowseHistory
	err := DB.Where("openid = ?", openid).Order("view_time desc").Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}
