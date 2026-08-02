package handler

import (
	"ledong-db/internal/logger"

	"github.com/gin-gonic/gin"
)

func logBusinessSuccess(c *gin.Context, operation string, attrs ...any) {
	logger.FromContext(c.Request.Context()).Info(
		"business operation succeeded",
		append([]any{"operation", operation}, attrs...)...,
	)
}

func logBusinessFailure(c *gin.Context, operation string, err error, attrs ...any) {
	fields := append([]any{"operation", operation}, attrs...)
	if err != nil {
		fields = append(fields, "error", err)
	}
	logger.FromContext(c.Request.Context()).Error(
		"business operation failed",
		fields...,
	)
}

func logBusinessRejected(c *gin.Context, operation string, err error, attrs ...any) {
	fields := append([]any{"operation", operation}, attrs...)
	if err != nil {
		fields = append(fields, "error", err)
	}
	logger.FromContext(c.Request.Context()).Warn(
		"business operation rejected",
		fields...,
	)
}
