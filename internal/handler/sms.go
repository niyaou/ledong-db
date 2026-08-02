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

type NotifyRequest struct {
	ID uint64 `json:"id" binding:"required" example:"123"` // 课程ID，必填
}

type Response struct {
	Code    int    `json:"code" example:"0"`          // 响应码，0表示成功
	Message string `json:"message" example:"success"` // 响应消息
}

// Notify 通知课程
// @Summary      通知课程
// @Description  按照课程查询用户并发送短信通知
// @Tags         短信
// @Accept       json
// @Produce      json
// @Param        request  body      NotifyRequest  true  "通知请求"
// @Success      200      {object}  Response        "成功"
// @Failure      400      {object}  Response        "请求参数错误"
// @Failure      500      {object}  Response        "服务器错误"
// @Router       /sms/notify [post]
func (h *SmsHandler) Notify(c *gin.Context) {
	var req NotifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: err.Error()})
		return
	}

	if err := h.service.Notify(c.Request.Context(), req.ID); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "success"})
}
