package model

import (
	"time"
)

// CheckInCampaign 打卡活动表（由系统管理员创建）
type CheckInCampaign struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Title       string    `gorm:"type:varchar(200);not null" json:"title"`          // 活动标题
	Description string    `gorm:"type:text" json:"description"`                    // 活动描述/打卡要求
	CoverImage  string    `gorm:"type:varchar(500)" json:"cover_image"`            // 封面图片
	StartTime   time.Time `json:"start_time"`                                      // 活动开始时间
	EndTime     time.Time `json:"end_time"`                                        // 活动结束时间
	IsActive    bool      `gorm:"default:true" json:"is_active"`                   // 是否启用
	CreatedBy   string    `gorm:"type:varchar(32)" json:"created_by"`              // 创建人(user_id)
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// CheckInTask 打卡任务（每个活动下的每日任务）
type CheckInTask struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CampaignID uint      `gorm:"index;not null" json:"campaign_id"`             // 所属活动ID
	Title      string    `gorm:"type:varchar(200);not null" json:"title"`       // 任务标题
	StartTime  time.Time `json:"start_time"`                                    // 任务开始时间
	EndTime    time.Time `json:"end_time"`                                      // 任务结束时间
	SortOrder  int       `gorm:"default:0" json:"sort_order"`                   // 排序
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// CheckInRecord 用户打卡记录
type CheckInRecord struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CampaignID uint      `gorm:"index;not null" json:"campaign_id"`             // 所属活动ID
	TaskID     uint      `gorm:"index;not null" json:"task_id"`                 // 所属任务ID
	UserID     string    `gorm:"index;type:varchar(32);not null" json:"user_id"` // 打卡用户ID
	Content    string    `gorm:"type:text" json:"content"`                      // 打卡文字内容
	Images     string    `gorm:"type:text" json:"images"`                       // 打卡图片，逗号分隔的URL列表
	Visibility string    `gorm:"type:varchar(20);default:'all'" json:"visibility"` // all=所有人, admin=仅管理员
	CheckDate  string    `gorm:"type:varchar(10);index" json:"check_date"`      // 打卡日期 YYYY-MM-DD
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// CheckInCampaignWithStats 活动统计（查询时使用）
type CheckInCampaignWithStats struct {
	CheckInCampaign
	CheckedCount  int    `json:"checked_count"`   // 今日已打卡人数（任务维度）
	TotalChecked  int    `json:"total_checked"`   // 活动总打卡人次
	TaskCount     int    `json:"task_count"`      // 任务总数
	PendingCount  int    `json:"pending_count"`   // 未完成数量（当前用户）
	UserChecked   bool   `json:"user_checked"`    // 当前用户今日是否已打卡
	StatusText    string `json:"status_text"`     // 状态文本：进行中/已过期/未开始
}

// CheckInRankItem 排行榜条目
type CheckInRankItem struct {
	UserID      string `json:"user_id"`
	Nickname    string `json:"nickname"`
	AvatarURL   string `json:"avatar_url"`
	CheckCount  int    `json:"check_count"`
	Rank        int    `json:"rank"`
}
