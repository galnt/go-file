package model

import (
	"strings"

	"gorm.io/gorm"
)

type User struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Username    string `json:"username" gorm:"unique;not null"`
	Password    string `json:"password" gorm:"not null"`
	DisplayName string `json:"displayName"`
	Role        int    `json:"role" gorm:"type:int;default:1"`   // admin, common
	Status      int    `json:"status" gorm:"type:int;default:1"` // enabled, disabled
	Token       string `json:"token"`
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
