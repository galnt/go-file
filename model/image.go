package model

import (
	"go-file/common"
	"os"
	"path/filepath"
)

type Image struct {
	Filename string `json:"type" gorm:"primaryKey"`
	Uploader string `json:"uploader"`
	Time     string `json:"time"`
}

func AllImage() ([]*Image, error) {
	var images []*Image
	err := DB.Find(&images).Error
	return images, err
}

func (image *Image) Insert() error {
	return DB.Create(image).Error
}

func (image *Image) Delete() error {
	err := DB.Delete(image).Error
	_ = os.Remove(filepath.Join(common.ImageUploadPath, image.Filename))
	return err
}
