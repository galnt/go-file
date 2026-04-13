package model

import (
	"go-file/common"
	"os"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func createAdminAccount() {
	var user User
	DB.Where(User{Role: common.RoleAdminUser}).Attrs(User{
		Username:    "admin",
		Password:    "123456",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		DisplayName: "Administrator",
	}).FirstOrCreate(&user)
}

func CountTable(tableName string) (num int64) {
	DB.Table(tableName).Count(&num)
	return
}

func InitDB() (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	if os.Getenv("SQL_DSN") != "" {
		// Use MySQL
		db, err = gorm.Open(mysql.Open(os.Getenv("SQL_DSN")), gormConfig)
	} else {
		// Use SQLite (glebarez/sqlite - pure Go, no CGO)
		// _loc=auto 让驱动自动将 TEXT 格式的时间字符串解析为 time.Time
		db, err = gorm.Open(sqlite.Open(common.SQLitePath+"?_loc=auto"), gormConfig)
	}

	if err != nil {
		common.FatalLog("failed to connect to database: " + err.Error())
		return nil, err
	}

	DB = db
	DB.AutoMigrate(&File{})
	DB.AutoMigrate(&Image{})
	DB.AutoMigrate(&User{})
	DB.AutoMigrate(&UserAuth{})
	DB.AutoMigrate(&Option{})
	DB.AutoMigrate(&Activity{}, &ActivityPhoto{}, &BrowseHistory{})
	DB.AutoMigrate(&CheckInCampaign{}, &CheckInRecord{})

	createAdminAccount()
	return DB, nil
}
