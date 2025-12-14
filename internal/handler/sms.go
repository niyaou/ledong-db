package handler

import (
	"net/http"

	"ledong-db/internal/service"

	"github.com/gin-gonic/gin"
)

type SmsHandler struct {
	service *service.SmsService
}

func NewSmsHandler(svc *service.SmsService) *SmsHandler {
	return &SmsHandler{service: svc}
}

type SendSmsRequest struct {
	Phone  string   `json:"phone" binding:"required" example:"13800138000"` // 手机号
	Params []string `json:"params" binding:"required" example:"验证码,5"`      // 短信参数
}

type Response struct {
	Code    int    `json:"code" example:"0"`          // 响应码，0表示成功
	Message string `json:"message" example:"success"` // 响应消息
}

// Send 发送短信
// @Summary      发送短信
// @Description  通过腾讯云发送短信
// @Tags         短信
// @Accept       json
// @Produce      json
// @Param        request  body      SendSmsRequest  true  "短信请求"
// @Success      200      {object}  Response        "成功"
// @Failure      400      {object}  Response        "请求参数错误"
// @Failure      500      {object}  Response        "服务器错误"
// @Router       /sms/send [post]
func (h *SmsHandler) Send(c *gin.Context) {
	var req SendSmsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: err.Error()})
		return
	}

	if err := h.service.Send(req.Phone, req.Params); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "success"})
}
