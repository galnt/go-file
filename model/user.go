package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type User struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID      string `gorm:"primaryKey;type:varchar(32)" json:"user_id"`
	Phone       string `gorm:"uniqueIndex;type:varchar(20)" json:"phone"` // 手机号注册/绑定
	Username    string `json:"username" gorm:"unique;not null"`
	Password    string `json:"password" gorm:"not null"`
	DisplayName string `json:"displayName"`
	Nickname    string `gorm:"type:varchar(100)" json:"nickname"`
	AvatarURL   string `gorm:"type:varchar(255)" json:"avatar_url"`

	// 核心关联字段
	UnionID   string    `gorm:"uniqueIndex;type:varchar(64)" json:"unionid"` // 微信跨平台唯一标识
	Role      int       `json:"role" gorm:"type:int;default:1"`              // admin, common
	Status    int       `json:"status" gorm:"type:int;default:1"`            // enabled, disabled
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
}

type UserAuth struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	UserID string `gorm:"index;type:varchar(32)" json:"user_id"` // 关联 users.user_id

	// 登录类型: "wechat_mp" (小程序), "wechat_web" (微信内网页)
	IdentityType string `gorm:"type:varchar(20)" json:"identity_type"`

	// 对应的 OpenID
	Identifier string `gorm:"uniqueIndex;type:varchar(64)" json:"identifier"`

	CreatedAt time.Time `json:"created_at"`
}

func (user *User) Insert() error {
	return DB.Create(user).Error
}

func (user *User) Update() error {
	return DB.Model(user).Updates(user).Error
}

func (user *User) Delete() error {
	return DB.Delete(user).Error
}

func (user *User) ValidateAndFill() {
	// GORM v2: use map or Select to query with zero-value fields
	DB.Where(&user).First(&user)
}

func ValidateUserToken(token string) (user *User) {
	if token == "" {
		return nil
	}
	token = strings.Replace(token, "Bearer ", "", 1)
	user = &User{}
	result := DB.Where("token = ?", token).First(user)
	if result.RowsAffected == 1 {
		return user
	}
	return nil
}

// Ensure gorm.ErrRecordNotFound is accessible
var _ = gorm.ErrRecordNotFound
