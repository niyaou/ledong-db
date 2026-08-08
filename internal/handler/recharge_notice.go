package handler

import (
	"errors"
	"net/http"
	"strconv"

	"ledong-db/internal/service"

	"github.com/gin-gonic/gin"
)

type RechargeNoticeHandler struct {
	service *service.RechargeNoticeService
}

func NewRechargeNoticeHandler(svc *service.RechargeNoticeService) *RechargeNoticeHandler {
	return &RechargeNoticeHandler{service: svc}
}

type acknowledgeRechargeNoticeRequest struct {
	Version uint64 `json:"version" binding:"required"`
}

type rechargeNoticeErrorResponse struct {
	Code      int    `json:"code"`
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
}

// ListSubmissions godoc
// @Summary      查询教练待处理填报
// @Description  分页混合返回待审课程和待知悉充值，仅用于管理员收件箱
// @Tags         教练填报
// @Produce      json
// @Param        pageNum   query int false "页码，从1开始" default(1)
// @Param        pageSize  query int false "每页数量，最大100" default(30)
// @Success      200 {object} service.CoachSubmissionPage
// @Failure      400 {object} rechargeNoticeErrorResponse
// @Failure      401 {object} rechargeNoticeErrorResponse
// @Failure      500 {object} rechargeNoticeErrorResponse
// @Router       /coach-submissions [get]
func (h *RechargeNoticeHandler) ListSubmissions(c *gin.Context) {
	if !verifySecure(c) {
		writeRechargeNoticeError(c, http.StatusUnauthorized, service.RechargeNoticeErrorUnauthorized, "未授权")
		return
	}
	pageNum, pageSize, ok := parseRechargeNoticePage(c)
	if !ok {
		return
	}
	page, err := h.service.ListSubmissions(pageNum, pageSize)
	if err != nil {
		logBusinessFailure(c, "coach_submission_list", err)
		writeRechargeNoticeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

// List godoc
// @Summary      查询充值待办
// @Description  按状态和业务日期分页查询充值待办历史
// @Tags         教练填报
// @Produce      json
// @Param        status     query string true  "PENDING或ACKNOWLEDGED"
// @Param        pageNum    query int    false "页码，从1开始" default(1)
// @Param        pageSize   query int    false "每页数量，最大100" default(30)
// @Param        startDate  query string false "开始业务日期 YYYY-MM-DD"
// @Param        endDate    query string false "结束业务日期 YYYY-MM-DD"
// @Success      200 {object} service.RechargeNoticePage
// @Failure      400 {object} rechargeNoticeErrorResponse
// @Failure      401 {object} rechargeNoticeErrorResponse
// @Failure      500 {object} rechargeNoticeErrorResponse
// @Router       /recharge-notices [get]
func (h *RechargeNoticeHandler) List(c *gin.Context) {
	if !verifySecure(c) {
		writeRechargeNoticeError(c, http.StatusUnauthorized, service.RechargeNoticeErrorUnauthorized, "未授权")
		return
	}
	pageNum, pageSize, ok := parseRechargeNoticePage(c)
	if !ok {
		return
	}
	page, err := h.service.ListNotices(c.Query("status"), c.Query("startDate"), c.Query("endDate"), pageNum, pageSize)
	if err != nil {
		logBusinessFailure(c, "recharge_notice_list", err)
		writeRechargeNoticeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

// Acknowledge godoc
// @Summary      确认已知悉充值待办
// @Description  按内容版本并发确认；相同版本重复请求幂等成功
// @Tags         教练填报
// @Accept       json
// @Produce      json
// @Param        id      path uint64 true "充值待办ID"
// @Param        request body acknowledgeRechargeNoticeRequest true "内容版本"
// @Success      200 {object} service.RechargeNoticeDTO
// @Failure      400 {object} rechargeNoticeErrorResponse
// @Failure      401 {object} rechargeNoticeErrorResponse
// @Failure      404 {object} rechargeNoticeErrorResponse
// @Failure      409 {object} rechargeNoticeErrorResponse
// @Failure      500 {object} rechargeNoticeErrorResponse
// @Router       /recharge-notices/{id}/acknowledge [post]
func (h *RechargeNoticeHandler) Acknowledge(c *gin.Context) {
	if !verifySecure(c) {
		writeRechargeNoticeError(c, http.StatusUnauthorized, service.RechargeNoticeErrorUnauthorized, "未授权")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		writeRechargeNoticeError(c, http.StatusBadRequest, service.RechargeNoticeErrorInvalidRequest, "充值待办ID格式错误")
		return
	}
	var request acknowledgeRechargeNoticeRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.Version == 0 {
		writeRechargeNoticeError(c, http.StatusBadRequest, service.RechargeNoticeErrorInvalidRequest, "version必须为正整数")
		return
	}
	notice, err := h.service.Acknowledge(id, request.Version)
	if err != nil {
		var noticeErr *service.RechargeNoticeError
		if errors.As(err, &noticeErr) && rechargeNoticeHTTPStatus(noticeErr.Code) < http.StatusInternalServerError {
			logBusinessRejected(c, "recharge_notice_acknowledge", err, "recharge_notice_id", id, "version", request.Version, "error_code", noticeErr.Code)
		} else {
			logBusinessFailure(c, "recharge_notice_acknowledge", err, "recharge_notice_id", id, "version", request.Version)
		}
		writeRechargeNoticeServiceError(c, err)
		return
	}
	logBusinessSuccess(c, "recharge_notice_acknowledge", "recharge_notice_id", id, "version", request.Version)
	c.JSON(http.StatusOK, notice)
}

func parseRechargeNoticePage(c *gin.Context) (int, int, bool) {
	pageNum := 1
	pageSize := 30
	var err error
	if raw := c.Query("pageNum"); raw != "" {
		pageNum, err = strconv.Atoi(raw)
		if err != nil || pageNum < 1 {
			writeRechargeNoticeError(c, http.StatusBadRequest, service.RechargeNoticeErrorInvalidRequest, "pageNum必须为正整数")
			return 0, 0, false
		}
	}
	if raw := c.Query("pageSize"); raw != "" {
		pageSize, err = strconv.Atoi(raw)
		if err != nil || pageSize < 1 || pageSize > 100 {
			writeRechargeNoticeError(c, http.StatusBadRequest, service.RechargeNoticeErrorInvalidRequest, "pageSize必须是1到100")
			return 0, 0, false
		}
	}
	return pageNum, pageSize, true
}

func writeRechargeNoticeServiceError(c *gin.Context, err error) {
	var noticeErr *service.RechargeNoticeError
	if !errors.As(err, &noticeErr) {
		writeRechargeNoticeError(c, http.StatusInternalServerError, service.RechargeNoticeErrorInternal, "服务器内部错误")
		return
	}
	writeRechargeNoticeError(c, rechargeNoticeHTTPStatus(noticeErr.Code), noticeErr.Code, noticeErr.Message)
}

func rechargeNoticeHTTPStatus(code string) int {
	switch code {
	case service.RechargeNoticeErrorInvalidRequest:
		return http.StatusBadRequest
	case service.RechargeNoticeErrorUnauthorized:
		return http.StatusUnauthorized
	case service.RechargeNoticeErrorNotFound:
		return http.StatusNotFound
	case service.RechargeNoticeErrorVersionConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func writeRechargeNoticeError(c *gin.Context, status int, errorCode, message string) {
	c.JSON(status, rechargeNoticeErrorResponse{Code: 1, ErrorCode: errorCode, Message: message})
}
