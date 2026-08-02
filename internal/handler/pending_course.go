package handler

import (
	"errors"
	"net/http"
	"strconv"

	"ledong-db/internal/service"

	"github.com/gin-gonic/gin"
)

// PendingCourseHandler exposes the administrator-only pending-course queue.
// It intentionally leaves the legacy Excel course endpoints untouched.
type PendingCourseHandler struct {
	service *service.PendingCourseService
}

func NewPendingCourseHandler(svc *service.PendingCourseService) *PendingCourseHandler {
	return &PendingCourseHandler{service: svc}
}

type admitPendingCourseRequest struct {
	UpdatedAt string                     `json:"updatedAt"`
	Course    service.PendingCourseInput `json:"course"`
}

type pendingCourseErrorResponse struct {
	Code      int    `json:"code"`
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
	CourseID  uint64 `json:"courseId,omitempty"`
}

// List returns all pending messages with the display and balance values
// required by the administrator page. The service batches all associations.
func (h *PendingCourseHandler) List(c *gin.Context) {
	if !verifySecure(c) {
		writePendingCourseError(c, http.StatusUnauthorized, service.PendingErrorUnauthorized, "未授权", 0)
		return
	}

	courses, err := h.service.ListAll()
	if err != nil {
		logBusinessFailure(c, "pending_course_list", err)
		writePendingCourseServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, courses)
}

// Admit promotes a pending message through the existing CourseService and
// physically consumes the message only after a successful formal creation.
func (h *PendingCourseHandler) Admit(c *gin.Context) {
	if !verifySecure(c) {
		writePendingCourseError(c, http.StatusUnauthorized, service.PendingErrorUnauthorized, "未授权", 0)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		writePendingCourseError(c, http.StatusBadRequest, service.PendingErrorInvalidRequest, "待审课程ID格式错误", 0)
		return
	}

	var request admitPendingCourseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writePendingCourseError(c, http.StatusBadRequest, service.PendingErrorInvalidRequest, "请求数据格式错误", 0)
		return
	}
	if request.UpdatedAt == "" {
		writePendingCourseError(c, http.StatusBadRequest, service.PendingErrorInvalidRequest, "updatedAt不能为空", 0)
		return
	}

	course, err := h.service.Admit(id, request.UpdatedAt, request.Course)
	if err != nil {
		var pendingErr *service.PendingCourseError
		if errors.As(err, &pendingErr) && pendingCourseHTTPStatus(pendingErr.Code) < http.StatusInternalServerError {
			logBusinessRejected(c, "pending_course_admit", err, "pending_course_id", id, "error_code", pendingErr.Code, "course_id", pendingErr.CourseID)
		} else {
			logBusinessFailure(c, "pending_course_admit", err, "pending_course_id", id)
		}
		writePendingCourseServiceError(c, err)
		return
	}
	logBusinessSuccess(c, "pending_course_admit", "pending_course_id", id, "course_id", course.ID, "coach_id", course.CoachID, "court_id", course.CourtID, "course_type", course.CourseType)
	c.JSON(http.StatusOK, gin.H{"id": course.ID})
}

func writePendingCourseServiceError(c *gin.Context, err error) {
	var pendingErr *service.PendingCourseError
	if !errors.As(err, &pendingErr) {
		writePendingCourseError(c, http.StatusInternalServerError, service.PendingErrorInternal, "服务器内部错误", 0)
		return
	}
	writePendingCourseError(c, pendingCourseHTTPStatus(pendingErr.Code), pendingErr.Code, pendingErr.Message, pendingErr.CourseID)
}

func pendingCourseHTTPStatus(code string) int {
	switch code {
	case service.PendingErrorInvalidRequest, service.PendingErrorInvalidMemberSpend, service.PendingErrorDuplicateMember:
		return http.StatusBadRequest
	case service.PendingErrorUnauthorized:
		return http.StatusUnauthorized
	case service.PendingErrorNotFound, service.PendingErrorCoachNotFound, service.PendingErrorCourtNotFound, service.PendingErrorMemberNotFound:
		return http.StatusNotFound
	case service.PendingErrorUpdated, service.PendingErrorCourseDuplicate:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func writePendingCourseError(c *gin.Context, status int, errorCode, message string, courseID uint64) {
	c.JSON(status, pendingCourseErrorResponse{
		Code:      1,
		ErrorCode: errorCode,
		Message:   message,
		CourseID:  courseID,
	})
}
