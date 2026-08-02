package handler

import (
	"errors"
	"net/http"
	"strconv"

	"ledong-db/internal/service"

	"github.com/gin-gonic/gin"
)

type CourseHandler struct {
	service    *service.CourseService
	smsService *service.SmsService
}

// CoachCourses 查询单个教练的正式课程
// @Summary      查询单个教练课程
// @Description  按自然月查询一个有效教练的全部正式课程和授课课时统计，只读
// @Tags         课程
// @Produce      json
// @Param        secure    header  string  true  "安全验证头"
// @Param        coachId   path    int     true  "教练ID"
// @Param        month     query   string  true  "统计月，格式：2006-01"
// @Success      200       {object} service.MonthlyCoachCoursesDTO
// @Failure      400       {object} Response
// @Failure      401       {object} Response
// @Failure      404       {object} Response
// @Failure      500       {object} Response
// @Router       /prepaidCard/course/coach/{coachId} [get]
func (h *CourseHandler) CoachCourses(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}

	coachID, err := strconv.ParseUint(c.Param("coachId"), 10, 64)
	if err != nil || coachID == 0 {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "教练ID格式错误"})
		return
	}
	month := c.Query("month")
	if month == "" {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "month不能为空"})
		return
	}

	result, err := h.service.CoachCourses(coachID, month)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCoachCourseInvalidMonth):
			c.JSON(http.StatusBadRequest, Response{Code: 1, Message: err.Error()})
		case errors.Is(err, service.ErrCoachCourseCoachAbsent):
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, result)
}

func NewCourseHandler(svc *service.CourseService, smsService *service.SmsService) *CourseHandler {
	return &CourseHandler{
		service:    svc,
		smsService: smsService,
	}
}

// TotalCourse 查询课程列表
// @Summary      查询课程列表
// @Description  根据开始和结束时间查询课程列表，支持按会员编号过滤和分页
// @Tags         课程
// @Accept       json
// @Produce      json
// @Param        startTime  query     string  true   "开始时间，格式：2006-01-02 15:04:05"
// @Param        endTime    query     string  false  "结束时间，格式：2006-01-02 15:04:05"
// @Param        number     query     string  false  "会员编号，可选"
// @Param        pageNum    query     int     false  "页码，从1开始，默认1"
// @Success      200        {object}  service.CoursePage
// @Failure      400       {object}  Response
// @Failure      500       {object}  Response
// @Router       /course/total [get]
func (h *CourseHandler) TotalCourse(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}
	startTime := c.Query("startTime")
	if startTime == "" {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "startTime is required"})
		return
	}

	endTime := c.Query("endTime")

	number := c.Query("number")
	if number != "" {
		result, err := h.service.MemberCourse(startTime, endTime, number, 1, 100)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
		return
	}

	pageNum := 1
	if pageNumStr := c.Query("pageNum"); pageNumStr != "" {
		if pn, err := strconv.Atoi(pageNumStr); err == nil && pn > 0 {
			pageNum = pn
		}
	}

	pageSize := 100
	var numberPtr *string
	result, err := h.service.TotalCourse(startTime, endTime, numberPtr, pageNum, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// CreateCourse 创建课程
// @Summary      创建课程
// @Description  创建新课程，支持添加会员和消费记录
// @Tags         课程
// @Accept       json
// @Produce      json
// @Param        secure       header    string   false  "安全验证头"
// @Param        startTime    formData  string   true   "开始时间"
// @Param        endTime      formData  string   true   "结束时间"
// @Param        coachName    formData  string   true   "教练名称"
// @Param        spendingTime formData  float32  true   "课程时长"
// @Param        courtName    formData  string   true   "场地名称"
// @Param        descript     formData  string   true   "描述"
// @Param        courseType   formData  int      true   "课程类型"
// @Param        membersObj   formData  string   true   "会员JSON对象"
// @Param        isAdult      formData  int      false  "是否成人课程"
// @Success      200          {object}  Response
// @Failure      400          {object}  Response
// @Failure      500          {object}  Response
// @Router       /prepaidCard/course/create [post]
func (h *CourseHandler) CreateCourse(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}

	startTime := c.PostForm("startTime")
	endTime := c.PostForm("endTime")
	coachName := c.PostForm("coachName")
	spendingTimeStr := c.PostForm("spendingTime")
	courtName := c.PostForm("courtName")
	descript := c.PostForm("descript")
	courseTypeStr := c.PostForm("courseType")
	membersObj := c.PostForm("membersObj")

	if startTime == "" || endTime == "" || coachName == "" || spendingTimeStr == "" || courtName == "" || courseTypeStr == "" || membersObj == "" {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "参数不完整"})
		return
	}

	spendingTime, err := strconv.ParseFloat(spendingTimeStr, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "课程时长格式错误"})
		return
	}

	courseType, err := strconv.Atoi(courseTypeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "课程类型格式错误"})
		return
	}

	var isAdult *int
	if isAdultStr := c.PostForm("isAdult"); isAdultStr != "" {
		if ia, err := strconv.Atoi(isAdultStr); err == nil {
			isAdult = &ia
		}
	}

	course, err := h.service.CreateCourse(startTime, endTime, coachName, float32(spendingTime), courtName, descript, courseType, membersObj, isAdult)
	if err != nil {
		logBusinessFailure(c, "course_create", err, "start_time", startTime, "end_time", endTime, "coach", coachName, "court", courtName, "course_type", courseType)
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}
	logBusinessSuccess(c, "course_create", "course_id", course.ID, "start_time", course.StartTime, "end_time", course.EndTime, "coach_id", course.CoachID, "court_id", course.CourtID, "course_type", course.CourseType, "member_count", len(course.Members))

	c.JSON(http.StatusOK, gin.H{"id": course.ID})
}

// RemoveCourseMember 移除课程成员
// @Summary      移除课程成员
// @Description  从课程中移除指定会员，并退还消费
// @Tags         课程
// @Accept       json
// @Produce      json
// @Param        secure  header    string  false  "安全验证头"
// @Param        id      path      int     true   "课程ID"
// @Param        member  path      string  true   "会员编号"
// @Success      200     {object}  Response
// @Failure      400     {object}  Response
// @Failure      500     {object}  Response
// @Router       /prepaidCard/course/{id}/{member} [delete]
func (h *CourseHandler) RemoveCourseMember(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}

	idStr := c.Param("id")
	member := c.Param("member")

	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "课程ID格式错误"})
		return
	}

	course, err := h.service.RemoveCourseMember(id, member)
	if err != nil {
		if err == service.ErrUserNotFound {
			logBusinessRejected(c, "course_member_remove", err, "course_id", id, "member", member)
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		logBusinessFailure(c, "course_member_remove", err, "course_id", id, "member", member)
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}
	logBusinessSuccess(c, "course_member_remove", "course_id", course.ID, "member", member)

	c.JSON(http.StatusOK, gin.H{})
}

// RemoveCourse 删除课程
// @Summary      删除课程
// @Description  删除课程并退还所有会员的消费
// @Tags         课程
// @Accept       json
// @Produce      json
// @Param        secure  header    string  false  "安全验证头"
// @Param        id      path      int     true   "课程ID"
// @Success      200     {object}  Response
// @Failure      400     {object}  Response
// @Failure      500     {object}  Response
// @Router       /prepaidCard/course/{id} [delete]
func (h *CourseHandler) RemoveCourse(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "课程ID格式错误"})
		return
	}

	course, err := h.service.RemoveCourse(id)
	if err != nil {
		if err == service.ErrUserNotFound {
			logBusinessRejected(c, "course_remove", err, "course_id", id)
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		logBusinessFailure(c, "course_remove", err, "course_id", id)
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}
	logBusinessSuccess(c, "course_remove", "course_id", course.ID, "spend_count", len(course.Spends))

	c.JSON(http.StatusOK, gin.H{})
}

// TrialCourseUpdate 更新体验课状态
// @Summary      更新体验课状态
// @Description  将体验课状态加1（从未成单变为成单）
// @Tags         课程
// @Accept       json
// @Produce      json
// @Param        secure  header    string  false  "安全验证头"
// @Param        id      path      int     true   "课程ID"
// @Success      200     {object}  Response
// @Failure      400     {object}  Response
// @Failure      500     {object}  Response
// @Router       /prepaidCard/course/trial/{id} [post]
func (h *CourseHandler) TrialCourseUpdate(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "课程ID格式错误"})
		return
	}

	course, err := h.service.TrialCourseUpdate(id)
	if err != nil {
		if err == service.ErrUserNotFound {
			logBusinessRejected(c, "trial_course_update", err, "course_id", id)
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		logBusinessFailure(c, "trial_course_update", err, "course_id", id)
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}
	logBusinessSuccess(c, "trial_course_update", "course_id", course.ID, "course_type", course.CourseType)

	c.JSON(http.StatusOK, gin.H{})
}

// NotifyCourse 课程通知
// @Summary      课程通知
// @Description  发送课程通知短信
// @Tags         课程
// @Accept       json
// @Produce      json
// @Param        secure    header    string  false  "安全验证头"
// @Param        courseId  formData  int     true   "课程ID"
// @Success      200       {object}  Response
// @Failure      400       {object}  Response
// @Failure      500       {object}  Response
// @Router       /prepaidCard/course/notify [post]
func (h *CourseHandler) NotifyCourse(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}

	courseIdStr := c.PostForm("courseId")
	courseId, err := strconv.ParseUint(courseIdStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "课程ID格式错误"})
		return
	}

	if err := h.service.Notify(c.Request.Context(), courseId); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

// DuplicatedCheck 课程重复检查
// @Summary      课程重复检查
// @Description  检查课程列表中是否有重复的课程
// @Tags         课程
// @Accept       json
// @Produce      json
// @Param        secure  header    string        false  "安全验证头"
// @Param        params  body      [][]interface{}  true   "课程参数列表"
// @Success      200     {array}   []interface{}
// @Failure      400     {object}  Response
// @Failure      500     {object}  Response
// @Router       /prepaidCard/course/duplicate [post]
func (h *CourseHandler) DuplicatedCheck(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}

	var params [][]interface{}
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "参数格式错误"})
		return
	}

	result := h.service.DuplicatedCheck(params)
	c.JSON(http.StatusOK, result)
}

// CourseSm 发送课程短信
// @Summary      发送课程短信
// @Description  发送自定义短信
// @Tags         课程
// @Accept       json
// @Produce      json
// @Param        secure  header    string   false  "安全验证头"
// @Param        number  formData  string   true   "手机号"
// @Param        params  formData  []string true   "短信参数"
// @Success      200     {object}  Response
// @Failure      400     {object}  Response
// @Failure      500     {object}  Response
// @Router       /prepaidCard/course/sm [post]
func (h *CourseHandler) CourseSm(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}

	number := c.PostForm("number")
	paramsStr := c.PostFormArray("params")

	if number == "" || len(paramsStr) == 0 {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "参数不完整"})
		return
	}

	if err := h.smsService.SendContext(c.Request.Context(), number, paramsStr); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}
