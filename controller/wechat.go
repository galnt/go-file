package controller

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-file/common"
	"go-file/model"
)

// 微信用户信息结构
type WechatUserInfo struct {
	OpenID     string `json:"openid"`
	Nickname   string `json:"nickname"`
	AvatarURL  string `json:"avatar_url"`
	Gender     int    `json:"gender"`
	City       string `json:"city"`
	Province   string `json:"province"`
	Country    string `json:"country"`
	UnionID    string `json:"unionid"`
	SessionKey string `json:"session_key"`
}

// 微信登录请求
type WechatLoginRequest struct {
	Code string `json:"code"`
}

// 微信登录响应
type WechatLoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
	User    *struct {
		OpenID    string `json:"openid"`
		Nickname  string `json:"nickname"`
		AvatarURL string `json:"avatar_url"`
	} `json:"user,omitempty"`
}

// 批量下载请求
type BatchDownloadRequest struct {
	FilePaths []string `json:"file_paths"`
}

// 批量下载响应
type BatchDownloadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	ZipURL  string `json:"zip_url,omitempty"`
}

// WechatLogin 微信登录接口
// @Summary 微信登录
// @Description 通过微信授权码登录
// @Tags 微信
// @Accept json
// @Produce json
// @Param request body WechatLoginRequest true "登录请求"
// @Success 200 {object} WechatLoginResponse
// @Router /api/wechat/login [post]
func WechatLogin(c *gin.Context) {
	var req WechatLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, WechatLoginResponse{
			Success: false,
			Message: "无效的请求参数",
		})
		return
	}

	if req.Code == "" {
		c.JSON(http.StatusBadRequest, WechatLoginResponse{
			Success: false,
			Message: "授权码不能为空",
		})
		return
	}

	// 在实际应用中，这里应该调用微信API获取用户信息
	// 为了简化，我们模拟微信登录流程
	userInfo, err := getWechatUserInfo(req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, WechatLoginResponse{
			Success: false,
			Message: "微信登录失败: " + err.Error(),
		})
		return
	}

	// 检查用户是否已存在
	var user model.User
	// 使用微信OpenID作为用户名
	username := "wx_" + userInfo.OpenID
	db := model.DB.Where("username = ?", username).First(&user)

	if db.RowsAffected == 0 {
		// 创建新用户
		user = model.User{
			Username:    username,
			DisplayName: userInfo.Nickname,
			Password:    uuid.New().String(), // 随机密码，用户不会用到
			Role:        common.RoleCommonUser,
			Status:      common.UserStatusEnabled,
			Token:       generateUserToken(),
		}

		// 保存微信用户信息到额外字段（在实际项目中可能需要扩展User表）
		if err := user.Insert(); err != nil {
			c.JSON(http.StatusInternalServerError, WechatLoginResponse{
				Success: false,
				Message: "创建用户失败: " + err.Error(),
			})
			return
		}
	} else {
		// 更新用户信息
		user.DisplayName = userInfo.Nickname
		user.Token = generateUserToken()
		if err := user.Update(); err != nil {
			c.JSON(http.StatusInternalServerError, WechatLoginResponse{
				Success: false,
				Message: "更新用户信息失败: " + err.Error(),
			})
			return
		}
	}

	// 返回登录成功信息
	c.JSON(http.StatusOK, WechatLoginResponse{
		Success: true,
		Message: "登录成功",
		Token:   user.Token,
		User: &struct {
			OpenID    string `json:"openid"`
			Nickname  string `json:"nickname"`
			AvatarURL string `json:"avatar_url"`
		}{
			OpenID:    userInfo.OpenID,
			Nickname:  userInfo.Nickname,
			AvatarURL: userInfo.AvatarURL,
		},
	})
}

// GetWechatUserInfo 获取微信用户信息（需要微信开放平台或公众平台权限）
// 这里是一个模拟实现
func getWechatUserInfo(code string) (*WechatUserInfo, error) {
	// 在实际应用中，这里应该调用微信API：
	// 1. 使用code获取access_token和openid
	// 2. 使用access_token获取用户信息

	// 模拟返回用户信息
	return &WechatUserInfo{
		OpenID:     "wx_" + code + "_" + uuid.New().String()[:8],
		Nickname:   "微信用户",
		AvatarURL:  "https://thirdwx.qlogo.cn/mmopen/vi_32/POgEwh4mIHO4nibH0KlMECNjjGxQUq24ZEaGT4poC6icRiccVGKSyXwibcPq4BWmiaIGuG1icwxaQX6grC9VemZoJ8rg/132",
		Gender:     1,
		City:       "深圳",
		Province:   "广东",
		Country:    "中国",
		SessionKey: "mock_session_key",
	}, nil
}

// generateUserToken 生成用户token
func generateUserToken() string {
	return uuid.New().String()
}

// BatchDownload 批量下载接口
// @Summary 批量下载文件
// @Description 批量下载选中的文件，打包为ZIP
// @Tags 下载
// @Accept json
// @Produce json
// @Param request body BatchDownloadRequest true "下载请求"
// @Success 200 {object} BatchDownloadResponse
// @Router /api/download/batch [post]
func BatchDownload(c *gin.Context) {
	var req BatchDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, BatchDownloadResponse{
			Success: false,
			Message: "无效的请求参数",
		})
		return
	}

	if len(req.FilePaths) == 0 {
		c.JSON(http.StatusBadRequest, BatchDownloadResponse{
			Success: false,
			Message: "请选择要下载的文件",
		})
		return
	}

	// 限制一次最多下载20个文件
	if len(req.FilePaths) > 20 {
		c.JSON(http.StatusBadRequest, BatchDownloadResponse{
			Success: false,
			Message: "一次最多只能下载20个文件",
		})
		return
	}

	// 创建临时ZIP文件
	tempDir := os.TempDir()
	zipFileName := fmt.Sprintf("download_%d.zip", time.Now().Unix())
	zipFilePath := filepath.Join(tempDir, zipFileName)

	// 创建ZIP文件
	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, BatchDownloadResponse{
			Success: false,
			Message: "创建ZIP文件失败: " + err.Error(),
		})
		return
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 添加文件到ZIP
	successCount := 0
	for _, filePath := range req.FilePaths {
		// 安全检查：确保文件路径在upload目录下
		if !strings.HasPrefix(filePath, "/upload/") && !strings.HasPrefix(filePath, "upload/") {
			// 尝试从绝对路径转换为相对路径
			absPath, err := filepath.Abs(filePath)
			if err != nil {
				continue
			}
			
			// 检查是否在项目目录下
			projectRoot, _ := filepath.Abs(".")
			if !strings.HasPrefix(absPath, projectRoot) {
				continue
			}
			
			// 转换为相对路径
			relPath, err := filepath.Rel(projectRoot, absPath)
			if err != nil {
				continue
			}
			
			filePath = relPath
		}

		// 打开文件
		file, err := os.Open(filePath)
		if err != nil {
			// 文件不存在或无法打开，跳过
			continue
		}
		defer file.Close()

		// 获取文件信息
		fileInfo, err := file.Stat()
		if err != nil {
			continue
		}

		// 创建ZIP文件头
		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			continue
		}

		// 设置文件名
		header.Name = filepath.Base(filePath)
		header.Method = zip.Deflate

		// 创建ZIP写入器
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			continue
		}

		// 复制文件内容
		_, err = io.Copy(writer, file)
		if err != nil {
			continue
		}

		successCount++
	}

	if successCount == 0 {
		c.JSON(http.StatusInternalServerError, BatchDownloadResponse{
			Success: false,
			Message: "没有找到可下载的文件",
		})
		return
	}

	// 返回ZIP文件URL
	// 在实际应用中，这里应该将文件上传到CDN或提供临时下载链接
	zipURL := "/temp/" + zipFileName
	
	c.JSON(http.StatusOK, BatchDownloadResponse{
		Success: true,
		Message: fmt.Sprintf("成功打包 %d 个文件", successCount),
		ZipURL:  zipURL,
	})
}

// GetTempFile 获取临时文件
// @Summary 获取临时文件
// @Description 下载临时生成的ZIP文件
// @Tags 下载
// @Produce application/zip
// @Param filename path string true "文件名"
// @Success 200 {file} binary
// @Router /temp/{filename} [get]
func GetTempFile(c *gin.Context) {
	filename := c.Param("filename")
	
	// 安全检查
	if !strings.HasSuffix(filename, ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的文件类型",
		})
		return
	}

	// 防止目录遍历攻击
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的文件名",
		})
		return
	}

	tempDir := os.TempDir()
	filePath := filepath.Join(tempDir, filename)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "文件不存在或已过期",
		})
		return
	}

	// 设置响应头
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	
	// 发送文件
	c.File(filePath)

	// 文件下载后删除（在实际应用中可能需要定时清理）
	go func() {
		time.Sleep(5 * time.Minute)
		os.Remove(filePath)
	}()
}

// GetUserInfo 获取当前用户信息
// @Summary 获取用户信息
// @Description 获取当前登录用户的信息
// @Tags 用户
// @Produce json
// @Success 200 {object} WechatLoginResponse
// @Router /api/user/info [get]
func GetUserInfo(c *gin.Context) {
	// 从token中获取用户信息
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, WechatLoginResponse{
			Success: false,
			Message: "未登录",
		})
		return
	}

	// 验证token
	token = strings.Replace(token, "Bearer ", "", 1)
	user := model.ValidateUserToken(token)
	if user == nil {
		c.JSON(http.StatusUnauthorized, WechatLoginResponse{
			Success: false,
			Message: "登录已过期",
		})
		return
	}

	// 返回用户信息（这里简化处理，实际应该从数据库获取微信用户信息）
	c.JSON(http.StatusOK, WechatLoginResponse{
		Success: true,
		Message: "获取成功",
		User: &struct {
			OpenID    string `json:"openid"`
			Nickname  string `json:"nickname"`
			AvatarURL string `json:"avatar_url"`
		}{
			OpenID:    user.Username,
			Nickname:  user.DisplayName,
			AvatarURL: "https://thirdwx.qlogo.cn/mmopen/vi_32/POgEwh4mIHO4nibH0KlMECNjjGxQUq24ZEaGT4poC6icRiccVGKSyXwibcPq4BWmiaIGuG1icwxaQX6grC9VemZoJ8rg/132",
		},
	})
}