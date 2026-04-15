package model

import (
	"fmt"
	"math/rand"
	"time"
)

type SmsCode struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Phone     string    `json:"phone" gorm:"type:varchar(20);not null;index"`
	Code      string    `json:"code" gorm:"type:varchar(10);not null"`
	Used      bool      `json:"used" gorm:"default:false"`
	ExpiresAt time.Time `json:"expires_at" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// 生成6位数字验证码
func GenerateSmsCode() string {
	// 使用当前时间作为随机数种子
	rand.Seed(time.Now().UnixNano())
	// 生成6位随机数字，范围 100000 ~ 999999
	return fmt.Sprintf("%06d", rand.Intn(900000)+100000)
}

// 验证短信验证码
func VerifySmsCode(phone, code string) bool {
	var sms SmsCode
	err := DB.Where("phone = ? AND code = ? AND used = ? AND expires_at > ?", phone, code, false, time.Now()).First(&sms).Error
	if err != nil {
		return false
	}
	// 标记为已使用
	sms.Used = true
	DB.Save(&sms)
	return true
}

// 创建短信验证码记录
func CreateSmsCode(phone, code string, expiresMinutes int) error {
	sms := SmsCode{
		Phone:     phone,
		Code:      code,
		Used:      false,
		ExpiresAt: time.Now().Add(time.Duration(expiresMinutes) * time.Minute),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return DB.Create(&sms).Error
}

// 清理过期的验证码记录
func CleanExpiredSmsCodes() error {
	return DB.Where("expires_at < ?", time.Now()).Delete(&SmsCode{}).Error
}
