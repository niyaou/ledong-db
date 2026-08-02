package router

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"ledong-db/internal/logger"

	"github.com/gin-gonic/gin"
)

const requestIDHeader = "X-Request-ID"

func requestLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(requestIDHeader))
		if !validRequestID(requestID) {
			requestID = newRequestID()
		}

		requestLogger := logger.Default().With("request_id", requestID)
		c.Request = c.Request.WithContext(logger.WithContext(c.Request.Context(), requestLogger))
		c.Header(requestIDHeader, requestID)

		startedAt := time.Now()
		c.Next()

		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"response_bytes", c.Writer.Size(),
			"remote_addr", c.Request.RemoteAddr,
		}

		switch status := c.Writer.Status(); {
		case status >= http.StatusInternalServerError:
			requestLogger.Error("http request completed", attrs...)
		case status >= http.StatusBadRequest:
			requestLogger.Warn("http request completed", attrs...)
		default:
			requestLogger.Info("http request completed", attrs...)
		}
	}
}

func recoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.FromContext(c.Request.Context()).Error(
					"http request panic",
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"panic", fmt.Sprint(recovered),
					"stack", string(debug.Stack()),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "internal server error"})
			}
		}()

		c.Next()
	}
}

func newRequestID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err == nil {
		return hex.EncodeToString(id[:])
	}
	return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
}

func validRequestID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, char := range id {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || strings.ContainsRune("-_.", char) {
			continue
		}
		return false
	}
	return true
}
