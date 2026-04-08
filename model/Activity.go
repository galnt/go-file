package model

import (
	"time"

	"gorm.io/gorm"
)

// GetActivityByNanoID 根据 NanoID 查询活动信息
func GetActivityByNanoID(nanoID string) (*Activity, error) {
	var activity Activity
	result := DB.Where("nano_id = ?", nanoID).First(&activity)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil // 没找到返回 nil，不报错
		}
		return nil, result.Error
	}
	return &activity, nil
}

// IncrVisitCount 原子递增活动浏览次数
func IncrVisitCount(nanoID string) {
	DB.Model(&Activity{}).Where("nano_id = ?", nanoID).
		UpdateColumn("visit_count", gorm.Expr("visit_count + 1"))
}

// Activity 活动主表
type Activity struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Location   string    `json:"location"`
	Lat        float64   `gorm:"type:decimal(10,8)" json:"lat"` // 纬度
	Lng        float64   `gorm:"type:decimal(11,8)" json:"lng"` // 经度
	NanoID     string    `json:"nano_id"`                       // 唯一标识符，使用NanoID生成
	EventDate  string    `json:"event_date"`
	FilePath   string    `json:"file_path"` // 核心：主表封面图路径
	VisitCount int       `json:"visit_count"`
	OpenID     string    `json:"openid"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// ActivityPhoto 活动照片从表
type ActivityPhoto struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ActivityID uint      `json:"activity_id"`
	FilePath   string    `json:"file_path"` // 从表多图路径
	Uploader   string    `json:"uploader"`
	IsActive   bool      `gorm:"default:true" json:"is_active"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}
