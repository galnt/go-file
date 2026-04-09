package model

import (
	"strings"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"gorm.io/gorm"
)

type User struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID      string `gorm:"uniqueIndex;type:varchar(32)" json:"user_id"`
	Phone       string `type:varchar(20)" json:"phone"` // 手机号注册/绑定
	Username    string `json:"username" gorm:"unique;not null"`
	Password    string `json:"password" gorm:"not null"`
	DisplayName string `json:"displayName"`
	Nickname    string `gorm:"type:varchar(100)" json:"nickname"`
	AvatarURL   string `gorm:"type:varchar(255)" json:"avatar_url"`

	// 核心关联字段
	UnionID   string    `type:varchar(64)" json:"unionid"`        // 微信跨平台唯一标识
	Role      int       `json:"role" gorm:"type:int;default:1"`   // admin, common
	Status    int       `json:"status" gorm:"type:int;default:1"` // enabled, disabled
	Token     string    `json:"token"`
	LastLogin time.Time `json:"last_login"` // 最后登录时间
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
	// 如果 UserID 为空，自动生成
	if user.UserID == "" {
		userID, err := GenerateUserID()
		if err != nil {
			return err
		}
		user.UserID = userID
	}
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

// GenerateUserID 生成唯一的用户ID（12位NanoID）
func GenerateUserID() (string, error) {
	alphabet := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz-"
	return gonanoid.Generate(alphabet, 12)
}

// UpdateLastLogin 更新用户最后登录时间
func (user *User) UpdateLastLogin() error {
	user.LastLogin = time.Now()
	return DB.Model(user).Update("last_login", user.LastLogin).Error
}

// FindOrCreateUserByWeChat 根据微信 OpenID 查找或创建用户
// identityType: "wechat_mp" (小程序), "wechat_web" (微信内网页)
// 返回用户、用户认证记录和错误
func FindOrCreateUserByWeChat(openID, identityType, nickName, avatarURL string) (*User, *UserAuth, error) {
	// 1. 检查是否已存在该微信账号的认证记录
	var auth UserAuth
	err := DB.Where("identifier = ? AND identity_type = ?", openID, identityType).First(&auth).Error
	if err == nil {
		// 认证记录存在，查找对应用户
		var user User
		err = DB.Where("user_id = ?", auth.UserID).First(&user).Error
		if err != nil {
			return nil, nil, err
		}
		// 更新用户信息（昵称、头像可能已变更）
		if nickName != "" && nickName != user.Nickname {
			user.Nickname = nickName
		}
		if avatarURL != "" && avatarURL != user.AvatarURL {
			user.AvatarURL = avatarURL
		}
		user.LastLogin = time.Now()
		DB.Save(&user)
		return &user, &auth, nil
	}

	// 2. 认证记录不存在，创建新用户和认证记录
	userID, err := GenerateUserID()
	if err != nil {
		return nil, nil, err
	}

	// 创建用户
	user := &User{
		UserID:    userID,
		Nickname:  nickName,
		AvatarURL: avatarURL,
		UnionID:   openID,
		LastLogin: time.Now(),
		CreatedAt: time.Now(),
		Role:      1,
		Status:    1,
	}
	err = DB.Create(user).Error
	if err != nil {
		return nil, nil, err
	}

	// 创建认证记录
	auth = UserAuth{
		UserID:       userID,
		IdentityType: identityType,
		Identifier:   openID,
		CreatedAt:    time.Now(),
	}
	err = DB.Create(&auth).Error
	if err != nil {
		// 如果创建失败，删除刚创建的用户（可选）
		DB.Delete(user)
		return nil, nil, err
	}

	return user, &auth, nil
}

// 返回用户id
func GetUserIdByOpenId(openID string) string {
	// 1. 检查是否已存在该微信账号的认证记录
	var auth UserAuth
	err := DB.Where("identifier = ?", openID).First(&auth).Error
	if err != nil {

	}
	return auth.UserID
}

// Ensure gorm.ErrRecordNotFound is accessible
var _ = gorm.ErrRecordNotFound
