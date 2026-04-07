package router

import (
	"github.com/gin-gonic/gin"
	"go-file/controller"
	"go-file/middleware"
)

func setApiRouter(router *gin.Engine) {
	router.Use(middleware.GlobalAPIRateLimit())
	router.GET("/status", controller.GetStatus)
	router.POST("/api/file", middleware.FileUploadPermissionCheck(), controller.UploadFile)
	router.POST("/api/image", middleware.ImageUploadPermissionCheck(), controller.UploadImage)
	router.GET("/api/notice", controller.GetNotice)
	
	// 微信相关API（不需要认证）
	router.POST("/api/wechat/login", controller.WechatLogin)
	router.GET("/temp/:filename", controller.GetTempFile)
	
	basicAuth := router.Group("/api")
	basicAuth.Use(middleware.ApiAuth())
	{
		basicAuth.DELETE("/file", controller.DeleteFile)
		basicAuth.DELETE("/image", controller.DeleteImage)
		basicAuth.PUT("/user", middleware.NoTokenAuth(), controller.UpdateSelf)
		basicAuth.POST("/token", controller.GenerateNewUserToken)
		
		// 需要认证的API
		basicAuth.GET("/user/info", controller.GetUserInfo)
		basicAuth.POST("/download/batch", controller.BatchDownload)
	}
	adminAuth := router.Group("/api")
	adminAuth.Use(middleware.ApiAdminAuth())
	{
		adminAuth.POST("/user", controller.CreateUser)
		adminAuth.PUT("/manage_user", controller.ManageUser)
		adminAuth.GET("/option", controller.GetOptions)
		adminAuth.PUT("/option", controller.UpdateOption)
		statRouter := adminAuth.Group("/stat")
		{
			statRouter.GET("/ip", controller.GetIPs)
			statRouter.GET("/url", controller.GetURLs)
			statRouter.GET("/req", controller.GetReqs)
		}
	}
}
