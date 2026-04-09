package controller

import (
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-file/common"
	"go-file/model"
	"net/http"
	"strings"
)

func Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	user := model.User{
		Username: username,
		Password: password,
	}
	user.ValidateAndFill()
	if user.Status != common.UserStatusEnabled {
		c.HTML(http.StatusForbidden, "login.html", gin.H{
			"message":  "用户名或密码错误，或者该用户已被封禁",
			"option":   common.OptionMap,
			"username": c.GetString("username"),
		})
		return
	}

	session := sessions.Default(c)
	session.Set("id", user.Id)
	session.Set("username", username)
	session.Set("role", user.Role)
	err := session.Save()
	if err != nil {
		c.HTML(http.StatusForbidden, "login.html", gin.H{
			"message":  "无法保存会话信息，请重试",
			"option":   common.OptionMap,
			"username": c.GetString("username"),
		})
		return
	}
	redirectUrl := c.Request.Referer()
	if strings.HasSuffix(redirectUrl, "/login") {
		redirectUrl = "/"
	}
	c.Redirect(http.StatusFound, redirectUrl)
	return
}

// Register 用户注册接口
func Register(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	displayName := c.PostForm("display_name")
	if displayName == "" {
		displayName = username
	}

	// 简单验证
	if username == "" || password == "" {
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"message":  "用户名和密码不能为空",
			"option":   common.OptionMap,
			"username": c.GetString("username"),
		})
		return
	}

	// 检查用户名是否已存在
	var existingUser model.User
	if err := model.DB.Where("username = ?", username).First(&existingUser).Error; err == nil {
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"message":  "用户名已存在",
			"option":   common.OptionMap,
			"username": c.GetString("username"),
		})
		return
	}

	// 创建新用户
	user := model.User{
		Username:    username,
		Password:    password,
		DisplayName: displayName,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	if err := user.Insert(); err != nil {
		c.HTML(http.StatusInternalServerError, "register.html", gin.H{
			"message":  "注册失败：" + err.Error(),
			"option":   common.OptionMap,
			"username": c.GetString("username"),
		})
		return
	}

	// 注册成功后自动登录
	session := sessions.Default(c)
	session.Set("id", user.Id)
	session.Set("username", username)
	session.Set("role", user.Role)
	err := session.Save()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "register.html", gin.H{
			"message":  "登录会话保存失败，请重新登录",
			"option":   common.OptionMap,
			"username": c.GetString("username"),
		})
		return
	}

	// 重定向到首页
	c.Redirect(http.StatusFound, "/")
	return
}

func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Options(sessions.Options{MaxAge: -1})
	session.Save()
	c.Redirect(http.StatusFound, "/login")
}

func UpdateSelf(c *gin.Context) {
	var user model.User
	err := json.NewDecoder(c.Request.Body).Decode(&user)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	user.Id = c.GetInt("id")
	role := c.GetInt("role")
	if role != common.RoleAdminUser {
		user.Role = 0
		user.Status = 0
	}
	// TODO: check Display Name to avoid XSS attack
	if err := user.Update(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

// CreateUser Only admin user can call this, so we can trust it
func CreateUser(c *gin.Context) {
	var user model.User
	err := json.NewDecoder(c.Request.Body).Decode(&user)
	user.DisplayName = user.Username
	// TODO: Check user.Status && user.Role
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}

	if err := user.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

type ManageRequest struct {
	Username string `json:"username"`
	Action   string `json:"action"`
}

// ManageUser Only admin user can do this
func ManageUser(c *gin.Context) {
	var req ManageRequest
	err := json.NewDecoder(c.Request.Body).Decode(&req)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	user := model.User{
		Username: req.Username,
	}
	// Fill attributes
	model.DB.Where(&user).First(&user)
	if user.Id == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户不存在",
		})
		return
	}
	switch req.Action {
	case "disable":
		user.Status = common.UserStatusDisabled
	case "enable":
		user.Status = common.UserStatusEnabled
	case "delete":
		if err := user.Delete(); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "promote":
		user.Role = common.RoleAdminUser
	case "demote":
		user.Role = common.RoleCommonUser
	}

	if err := user.Update(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func GenerateNewUserToken(c *gin.Context) {
	var user model.User
	user.Id = c.GetInt("id")
	// Fill attributes
	model.DB.Where(&user).First(&user)
	if user.Id == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户不存在",
		})
		return
	}
	user.Token = uuid.New().String()
	user.Token = strings.Replace(user.Token, "-", "", -1)

	if err := user.Update(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    user.Token,
	})
	return
}

// JSON登录请求结构
type JSONLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// JSON注册请求结构
type JSONRegisterRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// JSON登录响应
type JSONLoginResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Token    string `json:"token,omitempty"`
	UserID   string `json:"user_id,omitempty"`
	Username string `json:"username,omitempty"`
}

// APIUserLogin JSON API登录接口
func APIUserLogin(c *gin.Context) {
	var req JSONLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	username := req.Username
	password := req.Password
	user := model.User{
		Username: username,
		Password: password,
	}
	user.ValidateAndFill()
	if user.Status != common.UserStatusEnabled {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "用户名或密码错误，或者该用户已被封禁",
		})
		return
	}

	// 生成或获取用户token
	if user.Token == "" {
		user.Token = uuid.New().String()
		user.Token = strings.Replace(user.Token, "-", "", -1)
		if err := user.Update(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "无法生成用户令牌",
			})
			return
		}
	}

	// 更新最后登录时间
	user.UpdateLastLogin()

	c.JSON(http.StatusOK, JSONLoginResponse{
		Success:  true,
		Message:  "登录成功",
		Token:    user.Token,
		UserID:   user.UserID,
		Username: user.Username,
	})
}

// APIUserRegister JSON API注册接口
func APIUserRegister(c *gin.Context) {
	var req JSONRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	username := req.Username
	password := req.Password
	email := req.Email
	displayName := req.DisplayName
	if displayName == "" {
		displayName = username
	}

	// 简单验证
	if username == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "用户名和密码不能为空",
		})
		return
	}

	if len(password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "密码长度至少为6位",
		})
		return
	}

	// 检查用户名是否已存在
	var existingUser model.User
	if err := model.DB.Where("username = ?", username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "用户名已存在",
		})
		return
	}

	// 创建新用户
	user := model.User{
		Username:    username,
		Password:    password,
		DisplayName: displayName,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	if email != "" {
		// 如果有email字段，可以存储
	}
	if err := user.Insert(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "注册失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "注册成功",
		"user_id": user.UserID,
	})
}

// GetUserBrowseHistory 获取用户浏览历史
func GetUserBrowseHistory(c *gin.Context) {
	userID := c.Param("user_id")
	
	// 验证token
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权访问",
		})
		return
	}
	
	// 验证token是否有效
	token = strings.Replace(token, "Bearer ", "", 1)
	user := model.ValidateUserToken(token)
	if user == nil || user.UserID != userID {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "无效的令牌或用户ID不匹配",
		})
		return
	}
	
	// 获取浏览历史 - 使用userID作为openid查询
	var browseHistories []model.BrowseHistory
	err := model.DB.Where("openid = ?", userID).Order("view_time desc").Find(&browseHistories).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询浏览历史失败：" + err.Error(),
		})
		return
	}
	
	// 构建响应数据
	var historyData []map[string]interface{}
	for _, history := range browseHistories {
		// 获取关联的活动信息
		var activity model.Activity
		model.DB.Where("nano_id = ?", history.NanoID).First(&activity)
		
		item := map[string]interface{}{
			"id":            history.ID,
			"user_id":       history.OpenID,
			"nano_id":       history.NanoID,
			"browse_url":    history.BrowseURL,
			"view_time":     history.ViewTime,
			"activity_name": activity.Location, // 使用活动地点作为名称
			"activity_info": fmt.Sprintf("活动时间: %s", activity.EventDate),
		}
		historyData = append(historyData, item)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    historyData,
	})
}
