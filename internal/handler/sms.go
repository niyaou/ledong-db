package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"ledong-db/internal/service"
)

type SmsHandler struct {
	service *service.SmsService
}

func NewSmsHandler(svc *service.SmsService) *SmsHandler {
	return &SmsHandler{service: svc}
}

type SendSmsRequest struct {
	Phone  string   `json:"phone" binding:"required"`
	Params []string `json:"params" binding:"required"`
}

func (h *SmsHandler) Send(c *gin.Context) {
	var req SendSmsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}

	if err := h.service.Send(req.Phone, req.Params); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}
