package router

import (
	"go-file/controller"
	"go-file/middleware"

	"github.com/gin-gonic/gin"
)

func SetRouter(router *gin.Engine) {
	router.Use(middleware.AllStat())
	setWebRouter(router)
	setApiRouter(router)
	setMiniRouter(router)
	router.NoRoute(controller.Get404Page)
}
