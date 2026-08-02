package router

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ledong-db/internal/logger"

	"github.com/gin-gonic/gin"
)

func TestRequestLoggingMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	original := logger.Default()
	logger.SetDefault(logger.New(&output, "info"))
	t.Cleanup(func() { logger.SetDefault(original) })

	r := gin.New()
	r.Use(requestLoggingMiddleware(), recoveryMiddleware())
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "/test?secret=not-logged", nil)
	req.Header.Set(requestIDHeader, "request-123")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, req)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get(requestIDHeader); got != "request-123" {
		t.Fatalf("request ID = %q, want request-123", got)
	}

	entry := decodeLogLine(t, output.Bytes())
	if entry["request_id"] != "request-123" || entry["path"] != "/test" || entry["status"] != float64(http.StatusNoContent) {
		t.Fatalf("unexpected request log: %#v", entry)
	}
	if bytes.Contains(output.Bytes(), []byte("not-logged")) {
		t.Fatal("request query was written to the access log")
	}
}

func TestRecoveryMiddlewareLogsPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	original := logger.Default()
	logger.SetDefault(logger.New(&output, "info"))
	t.Cleanup(func() { logger.SetDefault(original) })

	r := gin.New()
	r.Use(requestLoggingMiddleware(), recoveryMiddleware())
	r.GET("/panic", func(*gin.Context) { panic("boom") })

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, req)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if !bytes.Contains(output.Bytes(), []byte(`"msg":"http request panic"`)) || !bytes.Contains(output.Bytes(), []byte(`"status":500`)) {
		t.Fatalf("panic and access logs not found: %s", output.String())
	}
}

func TestValidRequestID(t *testing.T) {
	if !validRequestID("safe-ID_123.4") {
		t.Fatal("valid request ID was rejected")
	}
	if validRequestID("unsafe\nvalue") {
		t.Fatal("unsafe request ID was accepted")
	}
}

func decodeLogLine(t *testing.T, data []byte) map[string]any {
	t.Helper()
	scanner := bufio.NewScanner(bytes.NewReader(data))
	if !scanner.Scan() {
		t.Fatal("no log line found")
	}
	var entry map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
		t.Fatalf("decode log entry: %v", err)
	}
	return entry
}
