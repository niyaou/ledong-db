package handler

import (
	"bytes"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"ledong-db/internal/logger"

	"github.com/gin-gonic/gin"
)

func TestBusinessAuditLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	request := httptest.NewRequest("POST", "/operation", nil)
	request = request.WithContext(logger.WithContext(request.Context(), logger.New(&output, "info")))
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	logBusinessSuccess(context, "course_create", "course_id", 42)
	logBusinessFailure(context, "course_remove", errors.New("database unavailable"), "course_id", 42)

	logs := output.String()
	if !strings.Contains(logs, `"msg":"business operation succeeded"`) || !strings.Contains(logs, `"operation":"course_create"`) {
		t.Fatalf("success audit log missing: %s", logs)
	}
	if !strings.Contains(logs, `"msg":"business operation failed"`) || !strings.Contains(logs, `"error":"database unavailable"`) {
		t.Fatalf("failure audit log missing: %s", logs)
	}
}
