package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"go-file/common"
	"go-file/model"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func GetExplorerPageOrFile(c *gin.Context) {
	path := c.DefaultQuery("path", "/")
	path, _ = url.PathUnescape(path)

	fullPath := filepath.Join(common.ExplorerRootPath, path)
	if !strings.HasPrefix(fullPath, common.ExplorerRootPath) {
		// We may being attacked!
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"message":  fmt.Sprintf("只能访问指定文件夹的子目录"),
			"option":   common.OptionMap,
			"username": c.GetString("username"),
		})
		return
	}
	root, err := os.Stat(fullPath)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"message":  "处理路径时发生了错误，请确认路径正确",
			"option":   common.OptionMap,
			"username": c.GetString("username"),
		})
		return
	}
	if root.IsDir() {
		localFilesPtr, readmeFileLink, err := getData(path, fullPath)
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"message":  err.Error(),
				"option":   common.OptionMap,
				"username": c.GetString("username"),
			})
			return
		}

		c.HTML(http.StatusOK, "explorer.html", gin.H{
			"message":        "",
			"option":         common.OptionMap,
			"username":       c.GetString("username"),
			"files":          localFilesPtr,
			"readmeFileLink": readmeFileLink,
		})
	} else {
		c.File(filepath.Join(common.ExplorerRootPath, path))
	}
}

func getDataFromFS(path string, fullPath string) (localFilesPtr *[]model.LocalFile, readmeFileLink string, err error) {
	var localFiles []model.LocalFile
	var tempFiles []model.LocalFile
	files, err := ioutil.ReadDir(fullPath)
	if err != nil {
		return
	}
	if path != "/" {
		parts := strings.Split(path, "/")
		// Add the special item: ".." which means parent dir
		if len(parts) > 0 {
			parts = parts[:len(parts)-1]
		}
		parentPath := strings.Join(parts, "/")
		parentFile := model.LocalFile{
			Name:         "..",
			Link:         "explorer?path=" + url.QueryEscape(parentPath),
			Size:         "",
			IsFolder:     true,
			ModifiedTime: "",
		}
		localFiles = append(localFiles, parentFile)
		path = strings.Trim(path, "/") + "/"
	} else {
		path = ""
	}

	// 缩略图目录（你可以根据项目实际路径改一下）
	thumbsDir := filepath.Join(fullPath, "thumbs")
	if path != "" {
		os.MkdirAll(thumbsDir, os.ModePerm)
	}

	// 查询数据库,把当前目录下的文件查询出来
	// query := path
	filesV, err := model.AllFiles()
	if err != nil {

	}

	// 创建 map[string]string
	fileMap := make(map[string]string)

	for _, file := range filesV {
		fileMap[file.Filename] = file.Description
	}

	for _, f := range files {
		link := "explorer?path=" + url.QueryEscape(path+f.Name())

		// 设置缩略图路径
		ext := strings.ToLower(filepath.Ext(f.Name()))
		baseName := strings.TrimSuffix(f.Name(), ext)
		thumbName := baseName + ".jpg"
		thumbPath := filepath.Join(thumbsDir, thumbName)
		webThumbPath := "explorer?path=" + url.QueryEscape(path+"/thumbs/"+thumbName) // 用于页面访问的路径

		// 判断是否是图片或视频
		isImage := isImageFile(ext)
		isVideo := isVideoFile(ext)

		// 只有在不是文件夹的时候才判断缩略图
		if !f.IsDir() && (isImage || isVideo) {
			if _, err := os.Stat(thumbPath); os.IsNotExist(err) {
				// 生成缩略图
				if isImage {
					_ = generateImageThumbnail(filepath.Join(fullPath, f.Name()), thumbPath, 200)
				} else if isVideo {
					_ = extractVideoThumbnail(filepath.Join(fullPath, f.Name()), thumbPath)
				}
			}
		}

		file := model.LocalFile{
			Name:         f.Name(),
			Link:         link,
			Size:         common.Bytes2Size(f.Size()),
			IsFolder:     f.Mode().IsDir(),
			ModifiedTime: f.ModTime().String()[:19],
			Description:  fileMap[f.Name()],
		}

		if !f.IsDir() && (isImage || isVideo) {
			file.Thumb = webThumbPath
		}

		if file.IsFolder {
			// 判断是 thumbs 目录跳过
			if file.Name == "thumbs" {
				continue
			}
			localFiles = append(localFiles, file)
		} else {
			tempFiles = append(tempFiles, file)
		}
		if f.Name() == "README.md" {
			readmeFileLink = link
		}
	}
	localFiles = append(localFiles, tempFiles...)
	localFilesPtr = &localFiles
	return
}

func isImageFile(ext string) bool {
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp"
}

func isVideoFile(ext string) bool {
	return ext == ".mp4" || ext == ".avi" || ext == ".mov" || ext == ".webm" || ext == ".mkv"
}

func getData(path string, fullPath string) (localFilesPtr *[]model.LocalFile, readmeFileLink string, err error) {
	if !common.ExplorerCacheEnabled {
		return getDataFromFS(path, fullPath)
	} else {
		ctx := context.Background()
		rdb := common.RDB
		key := "cacheExplorer:" + fullPath
		n, _ := rdb.Exists(ctx, key).Result()
		if n <= 0 {
			// Cache doesn't exist
			localFilesPtr, readmeFileLink, err = getDataFromFS(path, fullPath)
			if err != nil {
				return
			}
			// Start a coroutine to update cache
			go func() {
				var values []string
				for _, f := range *localFilesPtr {
					s, err := json.Marshal(f)
					if err != nil {
						return
					}
					values = append(values, string(s))
				}
				rdb.RPush(ctx, key, values)
				rdb.Expire(ctx, key, time.Duration(common.ExplorerCacheTimeout)*time.Second)
			}()
		} else {
			// Cache existed, use cached data
			var localFiles []model.LocalFile
			file := model.LocalFile{}
			for _, s := range rdb.LRange(ctx, key, 0, -1).Val() {
				err = json.Unmarshal([]byte(s), &file)
				if err != nil {
					return
				}
				if file.Name == "README.md" {
					readmeFileLink = file.Link
				}
				localFiles = append(localFiles, file)
			}
			localFilesPtr = &localFiles
		}
	}
	return
}
