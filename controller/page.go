package controller

import (
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go-file/common"
	"go-file/model"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

func GetIndexPage(c *gin.Context) {
	query := c.Query("query")
	isQuery := query != ""
	p, _ := strconv.Atoi(c.Query("p"))
	if p < 0 {
		p = 0
	}
	next := p + 1
	prev := common.IntMax(0, p-1)

	startIdx := p * common.ItemsPerPage

	files, err := model.QueryFiles(query, startIdx)
	if err != nil {
		c.HTML(http.StatusOK, "error.html", gin.H{
			"message":  err.Error(),
			"option":   common.OptionMap,
			"username": c.GetString("username"),
		})
		return
	}
	if len(files) < common.ItemsPerPage {
		next = 0
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"message":  "",
		"option":   common.OptionMap,
		"username": c.GetString("username"),
		"files":    files,
		"isQuery":  isQuery,
		"next":     next,
		"prev":     prev,
	})
}

func GetManagePage(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	var uptime = time.Since(common.StartTime)
	session := sessions.Default(c)
	role := session.Get("role")
	c.HTML(http.StatusOK, "manage.html", gin.H{
		"message":                 "",
		"option":                  common.OptionMap,
		"username":                c.GetString("username"),
		"memory":                  fmt.Sprintf("%d MB", m.Sys/1024/1024),
		"uptime":                  common.Seconds2Time(int(uptime.Seconds())),
		"userNum":                 model.CountTable("users"),
		"fileNum":                 model.CountTable("files"),
		"imageNum":                model.CountTable("images"),
		"FileUploadPermission":    common.FileUploadPermission,
		"FileDownloadPermission":  common.FileDownloadPermission,
		"ImageUploadPermission":   common.ImageUploadPermission,
		"ImageDownloadPermission": common.ImageDownloadPermission,
		"isAdmin":                 role == common.RoleAdminUser,
		"StatEnabled":             common.StatEnabled,
	})
}

func GetHistoryPage(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// 获取当前用户
	var user model.User
	if err := model.DB.Where("username = ?", username).First(&user).Error; err != nil {
		c.HTML(http.StatusOK, "history.html", gin.H{
			"message":  "用户不存在",
			"option":   common.OptionMap,
			"username": username,
			"history":  []map[string]interface{}{},
		})
		return
	}

	// 查找微信认证记录
	var auth model.UserAuth
	if err := model.DB.Where("user_id = ? AND identity_type IN (?, ?)", user.UserID, "wechat_mp", "wechat_web").First(&auth).Error; err != nil {
		// 没有微信认证记录，显示空历史
		c.HTML(http.StatusOK, "history.html", gin.H{
			"message":  "未绑定微信账号，无浏览历史",
			"option":   common.OptionMap,
			"username": username,
			"history":  []map[string]interface{}{},
		})
		return
	}

	// 获取浏览历史
	var records []model.BrowseHistory
	if err := model.DB.Where("openid = ?", auth.Identifier).Order("view_time DESC").Find(&records).Error; err != nil {
		c.HTML(http.StatusOK, "history.html", gin.H{
			"message":  "查询历史记录失败",
			"option":   common.OptionMap,
			"username": username,
			"history":  []map[string]interface{}{},
		})
		return
	}

	// 获取活动信息
	history := make([]map[string]interface{}, len(records))
	for i, r := range records {
		item := map[string]interface{}{
			"id":          r.ID,
			"nano_id":     r.NanoID,
			"browse_url":  r.BrowseURL,
			"view_time":   r.ViewTime.Format("2006-01-02 15:04:05"),
		}
		// 查询关联的活动
		var activity model.Activity
		if err := model.DB.Where("nano_id = ?", r.NanoID).First(&activity).Error; err == nil {
			item["activity_name"] = activity.Location // 暂时用Location作为活动名称
			item["location"] = activity.Location
			item["event_date"] = activity.EventDate
		} else {
			item["activity_name"] = "未知活动"
			item["location"] = ""
			item["event_date"] = ""
		}
		history[i] = item
	}

	c.HTML(http.StatusOK, "history.html", gin.H{
		"message":  "",
		"option":   common.OptionMap,
		"username": username,
		"history":  history,
	})
}

func GetImagePage(c *gin.Context) {
	c.HTML(http.StatusOK, "image.html", gin.H{
		"message":  "",
		"option":   common.OptionMap,
		"username": c.GetString("username"),
	})
}

func GetLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"message":  "",
		"option":   common.OptionMap,
		"username": c.GetString("username"),
	})
}

func GetRegisterPage(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", gin.H{
		"message":  "",
		"option":   common.OptionMap,
		"username": c.GetString("username"),
	})
}

func GetHelpPage(c *gin.Context) {
	c.HTML(http.StatusOK, "help.html", gin.H{
		"message":  "",
		"option":   common.OptionMap,
		"username": c.GetString("username"),
	})
}

func Get404Page(c *gin.Context) {
	c.HTML(http.StatusOK, "404.html", gin.H{
		"message":  "",
		"option":   common.OptionMap,
		"username": c.GetString("username"),
	})
}
