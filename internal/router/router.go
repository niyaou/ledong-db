package router

import (
	"github.com/gin-gonic/gin"
	"ledong-db/internal/handler"
)

func NewRouter(smsHandler *handler.SmsHandler) *gin.Engine {
	r := gin.Default()
	
	api := r.Group("/api")
	{
		sms := api.Group("/sms")
		{
			sms.POST("/send", smsHandler.Send)
		}
	}
	
	return r
}
