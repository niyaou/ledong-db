package handler

import (
	"net/http"

	"ledong-db/internal/service"

	"github.com/gin-gonic/gin"
)

type EfficiencyHandler struct {
	service *service.EfficiencyService
}

func NewEfficiencyHandler(svc *service.EfficiencyService) *EfficiencyHandler {
	return &EfficiencyHandler{service: svc}
}

// Efficient 教练效率统计
// @Summary      教练效率统计
// @Description  根据时间范围统计教练和校区的效率数据
// @Tags         教练
// @Accept       json
// @Produce      json
// @Param        startTime  query     string  true   "开始时间，格式：2006-01-02 15:04:05"
// @Param        endTime    query     string  true   "结束时间，格式：2006-01-02 15:04:05"
// @Success      200        {object}  service.EfficiencyResponse
// @Failure      400       {object}  Response
// @Failure      500       {object}  Response
// @Router       /coach/efficient [get]
func (h *EfficiencyHandler) Efficient(c *gin.Context) {
	startTime := c.Query("startTime")
	if startTime == "" {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "startTime is required"})
		return
	}

	endTime := c.Query("endTime")
	if endTime == "" {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "endTime is required"})
		return
	}

	result, err := h.service.AnalyseEfficiency(startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
