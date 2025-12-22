package handler

import (
	"net/http"

	"ledong-db/internal/service"

	"github.com/gin-gonic/gin"
)

type CardHandler struct {
	service *service.CardService
}

func NewCardHandler(svc *service.CardService) *CardHandler {
	return &CardHandler{service: svc}
}

// GetSpend 查询消费记录
// @Summary      查询消费记录
// @Description  查询消费记录，支持按会员编号过滤
// @Tags         消费记录
// @Accept       json
// @Produce      json
// @Param        number  query     string  false  "会员编号，可选"
// @Success      200     {object}  service.SpendPage
// @Failure      400     {object}  Response
// @Failure      500     {object}  Response
// @Router       /prepaidCard/spend [get]
func (h *CardHandler) GetSpend(c *gin.Context) {
	number := c.Query("number")

	pageNum := 1
	pageSize := 1000

	result, err := h.service.GetSpend(number, pageNum, pageSize)
	if err != nil {
		if err == service.ErrUserNotFound {
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, result.Content)
}
