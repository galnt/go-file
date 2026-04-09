package controller

import (
	"encoding/json"
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
