package router

import (
	"go-file/common"
	"go-file/controller"
	"go-file/middleware"

	"github.com/gin-gonic/gin"
)

func setWebRouter(router *gin.Engine) {
	router.Use(middleware.GlobalWebRateLimit())
	// Always available
	// All page must have username in context
	router.GET("/", middleware.ExtractUserInfo(), controller.GetExplorerPageOrFile)
	router.GET("/public/static/:file", controller.GetStaticFile)
	router.GET("/public/lib/:file", controller.GetLibFile)
	router.GET("/public/icon/:file", controller.GetIconFile)
	router.GET("/login", middleware.ExtractUserInfo(), controller.GetLoginPage)
	router.POST("/login", middleware.CriticalRateLimit(), controller.Login)
	router.GET("/logout", controller.Logout)
	// 注册功能已禁用，改用手机号登录
	// router.GET("/register", middleware.ExtractUserInfo(), controller.GetRegisterPage)
	// router.POST("/register", middleware.CriticalRateLimit(), controller.Register)
	router.GET("/help", middleware.ExtractUserInfo(), controller.GetHelpPage)

	// 验证码相关
	router.GET("/api/captcha/generate", controller.GenerateCaptcha)
	router.POST("/api/captcha/verify", controller.VerifyCaptcha)
	router.POST("/api/send-sms", controller.SendSmsCode)
	router.POST("/api/phone-login", controller.PhoneLogin)

	// Download files
	fileDownloadAuth := router.Group("/")
	fileDownloadAuth.Use(middleware.DownloadRateLimit(), middleware.FileDownloadPermissionCheck())
	{
		fileDownloadAuth.GET("/upload/*filepath", controller.DownloadFile)
		fileDownloadAuth.GET("/explorer", middleware.ExtractUserInfo(), controller.GetExplorerPageOrFile)
	}

	imageDownloadAuth := router.Group("/")
	imageDownloadAuth.Use(middleware.DownloadRateLimit(), middleware.ImageDownloadPermissionCheck())
	{
		imageDownloadAuth.Static("/image", common.ImageUploadPath)
	}

	router.GET("/image", middleware.ExtractUserInfo(), controller.GetImagePage)

	router.GET("/video", middleware.ExtractUserInfo(), controller.GetVideoPage)

	basicAuth := router.Group("/")
	basicAuth.Use(middleware.WebAuth()) // WebAuth already has username in context
	{
		basicAuth.GET("/manage", controller.GetManagePage)
		basicAuth.GET("/history", controller.GetHistoryPage)
	}
}
