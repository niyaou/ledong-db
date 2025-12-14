package handler

import (
	"net/http"
	"strconv"

	"ledong-db/internal/service"

	"github.com/gin-gonic/gin"
)

type CourseHandler struct {
	service *service.CourseService
}

func NewCourseHandler(svc *service.CourseService) *CourseHandler {
	return &CourseHandler{service: svc}
}

// TotalCourse 查询课程列表
// @Summary      查询课程列表
// @Description  根据开始时间查询课程列表，支持按会员编号过滤和分页
// @Tags         课程
// @Accept       json
// @Produce      json
// @Param        startTime  query     string  true   "开始时间，格式：2006-01-02 15:04:05"
// @Param        number     query     string  false  "会员编号，可选"
// @Param        pageNum    query     int     false  "页码，从1开始，默认1"
// @Success      200        {object}  service.CoursePage
// @Failure      400       {object}  Response
// @Failure      500       {object}  Response
// @Router       /course/total [get]
func (h *CourseHandler) TotalCourse(c *gin.Context) {
	startTime := c.Query("startTime")
	if startTime == "" {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "startTime is required"})
		return
	}

	var number *string
	if num := c.Query("number"); num != "" {
		number = &num
	}

	pageNum := 1
	if pageNumStr := c.Query("pageNum"); pageNumStr != "" {
		if pn, err := strconv.Atoi(pageNumStr); err == nil && pn > 0 {
			pageNum = pn
		}
	}

	pageSize := 100

	result, err := h.service.TotalCourse(startTime, number, pageNum, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
