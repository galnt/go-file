package controller

import (
	"encoding/json"
	"fmt"
	"go-file/common"
	"go-file/model"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type captchaInfo struct {
	target  int
	expires int64 // Unix timestamp
}

var (
	// captchaStore 存储生成的验证码 token 对应的目标值和过期时间
	captchaStore = make(map[string]captchaInfo)
	// verifiedCaptchaStore 存储已验证通过的验证码 token 及其过期时间，用于短信发送验证
	verifiedCaptchaStore = make(map[string]time.Time)
	captchaMutex         sync.RWMutex
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

// Register 用户注册接口（已禁用，改用手机号登录）
func Register(c *gin.Context) {
	c.HTML(http.StatusForbidden, "register.html", gin.H{
		"message":  "注册功能已禁用，请使用手机号登录",
		"option":   common.OptionMap,
		"username": c.GetString("username"),
	})
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

// APIUserRegister JSON API注册接口（已禁用，改用手机号登录）
func APIUserRegister(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{
		"success": false,
		"message": "注册功能已禁用，请使用手机号登录",
	})
	return
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
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "无效的令牌或用户ID不匹配",
		})
		return
	}

	userID = user.UserID

	// 获取浏览历史 - 使用userID作为openid查询
	var browseHistories []model.BrowseHistory
	err := model.DB.Where("open_id = ?", userID).Order("view_time desc").Find(&browseHistories).Error
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

// CreateBrowseHistory 记录用户浏览历史
func CreateBrowseHistory(c *gin.Context) {
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
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "无效的令牌",
		})
		return
	}

	// 解析请求体
	var req struct {
		UserID       string `json:"user_id"`
		Path         string `json:"path"`
		ActivityName string `json:"activity_name"`
		ActivityInfo string `json:"activity_info"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	// 使用token中的用户ID作为OpenID，如果req.UserID不为空则验证匹配
	openID := user.UserID
	if req.UserID != "" && req.UserID != openID {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "用户ID不匹配",
		})
		return
	}

	// 构建浏览URL：使用path作为NanoID，如果path是相对路径则添加基础URL
	browseURL := req.Path
	if !strings.Contains(browseURL, "://") && !strings.HasPrefix(browseURL, "http") {
		// 假设path是NanoID，构建完整URL
		browseURL = fmt.Sprintf("/explorer?path=%s", req.Path)
	}

	// 创建浏览历史记录
	record := model.BrowseHistory{
		OpenID:        openID,    // 使用token验证后的用户ID
		NanoID:        req.Path,  // 假设path就是NanoID
		BrowseURL:     browseURL, // 构建更完整的URL
		GraphicRecord: "",        // 图形记录留空
		ViewTime:      time.Now(),
	}

	// 保存到数据库
	if err := model.DB.Create(&record).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "记录浏览历史失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "记录成功",
	})
}

// 滑动验证码生成
type CaptchaResponse struct {
	Token   string `json:"token"`
	Image   string `json:"image,omitempty"` // base64图片，可选
	Slider  int    `json:"slider"`          // 滑块目标位置（0-100）
	Expires int64  `json:"expires"`         // 过期时间戳
}

// GenerateCaptcha 生成滑动验证码
func GenerateCaptcha(c *gin.Context) {
	// 生成一个随机目标位置
	token := uuid.New().String()
	token = strings.Replace(token, "-", "", -1)
	// 随机目标位置 30-70
	rand.Seed(time.Now().UnixNano())
	slider := rand.Intn(40) + 30
	expires := time.Now().Add(5 * time.Minute).Unix()

	// 存储到内存中，用于后续验证
	captchaMutex.Lock()
	captchaStore[token] = captchaInfo{
		target:  slider,
		expires: expires,
	}
	captchaMutex.Unlock()

	c.JSON(http.StatusOK, CaptchaResponse{
		Token:   token,
		Slider:  slider,
		Expires: expires,
	})
}

// 滑动验证码验证请求
type VerifyCaptchaRequest struct {
	Token      string `json:"token"`
	SlideValue int    `json:"slide_value"` // 用户滑动值
}

// VerifyCaptcha 验证滑动验证码
func VerifyCaptcha(c *gin.Context) {
	var req VerifyCaptchaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}
	// 从内存中获取之前生成的目标值
	captchaMutex.RLock()
	info, exists := captchaStore[req.Token]
	captchaMutex.RUnlock()
	if !exists {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "验证码已过期或无效",
		})
		return
	}
	// 检查是否过期
	if time.Now().Unix() > info.expires {
		// 过期则删除
		captchaMutex.Lock()
		delete(captchaStore, req.Token)
		captchaMutex.Unlock()
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "验证码已过期",
		})
		return
	}
	// 计算误差
	diff := req.SlideValue - info.target
	if diff < 0 {
		diff = -diff
	}
	// 允许误差5
	if diff <= 5 {
		// 验证通过，转移到已验证存储，有效期5分钟（用于短信发送）
		captchaMutex.Lock()
		delete(captchaStore, req.Token)
		verifiedCaptchaStore[req.Token] = time.Now().Add(5 * time.Minute)
		captchaMutex.Unlock()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "验证通过",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "验证失败",
	})
}

// 发送短信验证码请求
type SendSmsRequest struct {
	Phone          string `json:"phone"`
	CaptchaVerified string `json:"captcha_verified"` // 滑动验证通过标记，前端传递 "true"
}

// SendSmsCode 发送短信验证码
func SendSmsCode(c *gin.Context) {
	var req SendSmsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}
	// 验证手机号格式
	if len(req.Phone) != 11 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "手机号格式不正确",
		})
		return
	}
	// 验证滑动验证是否通过（前端传递 captcha_verified 为 "true"）
	if req.CaptchaVerified != "true" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请先完成滑动验证",
		})
		return
	}
	// 可在此处添加频率限制，例如同一手机号60秒内只能发送一次

	// 生成6位随机验证码
	code := model.GenerateSmsCode()
	// 保存到数据库，有效期10分钟
	err := model.CreateSmsCode(req.Phone, code, 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "发送失败，请重试",
		})
		return
	}
	// 实际应调用短信服务商API发送
	// 此处模拟发送
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "短信验证码已发送",
		// "code":    code, // 测试时返回，正式环境不应返回
	})
}

// 手机号登录请求
type PhoneLoginRequest struct {
	Phone   string `json:"phone"`
	SmsCode string `json:"sms_code"`
}

// PhoneLogin 手机号登录
func PhoneLogin(c *gin.Context) {
	var req PhoneLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}
	// 验证短信验证码
	if !model.VerifySmsCode(req.Phone, req.SmsCode) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "验证码错误或已过期",
		})
		return
	}
	// 查找用户，如果不存在则自动创建
	var user model.User
	err := model.DB.Where("phone = ?", req.Phone).First(&user).Error
	if err != nil {
		// 用户不存在，自动创建
		userID, _ := model.GenerateUserID()
		user = model.User{
			UserID:    userID,
			Phone:     req.Phone,
			Username:  "phone_" + req.Phone,
			Nickname:  "用户" + req.Phone[len(req.Phone)-4:],
			AvatarURL: "",
			Password:  "", // 无密码
			Role:      common.RoleCommonUser,
			Status:    common.UserStatusEnabled,
			LastLogin: time.Now(),
			CreatedAt: time.Now(),
		}
		if err := user.Insert(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "用户创建失败",
			})
			return
		}
	} else {
		// 更新最后登录时间
		user.LastLogin = time.Now()
		model.DB.Save(&user)
	}
	// 生成用户token
	if user.Token == "" {
		user.Token = uuid.New().String()
		user.Token = strings.Replace(user.Token, "-", "", -1)
		user.Update()
	}
	// 设置session
	session := sessions.Default(c)
	session.Set("id", user.Id)
	session.Set("username", user.Username)
	session.Set("role", user.Role)
	session.Save()

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "登录成功",
		"token":      user.Token,
		"user_id":    user.UserID,
		"username":   user.Username,
		"nickname":   user.Nickname,
		"avatar_url": user.AvatarURL,
	})
}
