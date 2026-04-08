package model

import (
	"go-file/common"
	"os"
	"path"
	"strings"

	"gorm.io/gorm"
)

type File struct {
	Id              int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Filename        string `json:"filename"`
	Description     string `json:"description"`
	Uploader        string `json:"uploader"`
	Link            string `json:"link" gorm:"unique"`
	Time            string `json:"time"`
	DownloadCounter int    `json:"download_counter"`
	Path            string `json:"path"`
}

type LocalFile struct {
	Name         string
	Link         string
	Size         string
	Description  string
	IsFolder     bool
	ModifiedTime string
	Thumb        string
}

func AllFiles() ([]*File, error) {
	var files []*File
	err := DB.Find(&files).Error
	return files, err
}

func QueryPathFiles(query string) ([]*File, error) {
	var files []*File
	err := DB.Where("path = ?", query).Find(&files).Error
	return files, err
}

func QueryFiles(query string, startIdx int) ([]*File, error) {
	var files []*File
	query = strings.ToLower(query)
	err := DB.Limit(common.ItemsPerPage).Offset(startIdx).
		Where("filename LIKE ? or description LIKE ? or uploader LIKE ? or time LIKE ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%", "%"+query+"%").
		Order("id desc").Find(&files).Error
	return files, err
}

func (file *File) Insert() error {
	return DB.Create(file).Error
}

// Delete Make sure link is valid! Because we will use os.Remove to delete it!
func (file *File) Delete() error {
	err := DB.Delete(file).Error
	_ = os.Remove(path.Join(common.UploadPath, file.Link))
	return err
}

func UpdateDownloadCounter(link string) {
	DB.Model(&File{}).Where("link = ?", link).UpdateColumn("download_counter", gorm.Expr("download_counter + 1"))
}
