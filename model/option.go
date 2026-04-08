package model

import (
	"errors"
	"go-file/common"
	"strconv"
	"strings"
)

type Option struct {
	Key   string `json:"key" gorm:"primaryKey"`
	Value string `json:"value"`
}

func AllOption() ([]*Option, error) {
	var options []*Option
	err := DB.Find(&options).Error
	return options, err
}

func InitOptionMap() {
	common.OptionMap = make(map[string]string)
	common.OptionMap["FileUploadPermission"] = strconv.Itoa(common.FileUploadPermission)
	common.OptionMap["FileDownloadPermission"] = strconv.Itoa(common.FileDownloadPermission)
	common.OptionMap["ImageUploadPermission"] = strconv.Itoa(common.ImageUploadPermission)
	common.OptionMap["ImageDownloadPermission"] = strconv.Itoa(common.ImageDownloadPermission)
	common.OptionMap["WebsiteName"] = "文件传输"
	common.OptionMap["FooterInfo"] = ""
	common.OptionMap["Version"] = common.Version
	common.OptionMap["Notice"] = ""
	options, _ := AllOption()
	for _, option := range options {
		updateOptionMap(option.Key, option.Value)
	}
}

func UpdateOption(key string, value string) error {
	if key == "StatEnabled" && value == "true" && !common.RedisEnabled {
		return errors.New("未启用 Redis，无法启用统计功能")
	}

	option := Option{
		Key:   key,
		Value: value,
	}
	// GORM v2: use Save or explicit Update
	result := DB.Model(&option).Where("key = ?", key).Update("value", option.Value)
	if result.RowsAffected == 0 {
		DB.Create(&option)
	}
	updateOptionMap(key, value)
	return nil
}

func updateOptionMap(key string, value string) {
	common.OptionMap[key] = value
	if strings.HasSuffix(key, "Permission") {
		intValue, _ := strconv.Atoi(value)
		switch key {
		case "FileUploadPermission":
			common.FileUploadPermission = intValue
		case "FileDownloadPermission":
			common.FileDownloadPermission = intValue
		case "ImageUploadPermission":
			common.ImageUploadPermission = intValue
		case "ImageDownloadPermission":
			common.ImageDownloadPermission = intValue
		}
	}
	if key == "StatEnabled" {
		common.StatEnabled = value == "true"
		if !common.RedisEnabled {
			common.StatEnabled = false
			common.OptionMap["StatEnabled"] = "false"
		}
	}
}
