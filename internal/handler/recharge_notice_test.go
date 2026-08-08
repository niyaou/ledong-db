package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ledong-db/internal/service"

	"github.com/gin-gonic/gin"
)

func TestRechargeNoticeHTTPStatus(t *testing.T) {
	tests := map[string]int{
		service.RechargeNoticeErrorInvalidRequest:  http.StatusBadRequest,
		service.RechargeNoticeErrorUnauthorized:    http.StatusUnauthorized,
		service.RechargeNoticeErrorNotFound:        http.StatusNotFound,
		service.RechargeNoticeErrorVersionConflict: http.StatusConflict,
		service.RechargeNoticeErrorInternal:        http.StatusInternalServerError,
	}
	for code, want := range tests {
		if got := rechargeNoticeHTTPStatus(code); got != want {
			t.Errorf("rechargeNoticeHTTPStatus(%q) = %d, want %d", code, got, want)
		}
	}
}

func TestParseRechargeNoticePageRejectsInvalidValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, target := range []string{"/?pageNum=0", "/?pageNum=x", "/?pageSize=0", "/?pageSize=101"} {
		response := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(response)
		context.Request = httptest.NewRequest(http.MethodGet, target, nil)
		_, _, ok := parseRechargeNoticePage(context)
		if ok || response.Code != http.StatusBadRequest {
			t.Fatalf("target %s: ok=%t status=%d", target, ok, response.Code)
		}
	}
}
