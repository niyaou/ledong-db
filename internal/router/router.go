package router

import (
	"ledong-db/internal/handler"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func NewRouter(smsHandler *handler.SmsHandler) *gin.Engine {
	r := gin.Default()

	r.GET("/health", healthCheck)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")
	{
		sms := api.Group("/sms")
		{
			sms.POST("/send", smsHandler.Send)
		}
	}

	return r
}

// healthCheck 健康检查
// @Summary      健康检查
// @Description  检查服务是否正常运行
// @Tags         系统
// @Success      200  {object}  map[string]string
// @Router       /health [get]
func healthCheck(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}
