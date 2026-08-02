package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

type contextKey struct{}

var defaultLogger = New(os.Stdout, "info")

func Init(level string) {
	SetDefault(New(os.Stdout, level))
}

func New(w io.Writer, level string) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	return slog.New(slog.NewJSONHandler(w, opts))
}

func Default() *slog.Logger {
	return defaultLogger
}

func SetDefault(log *slog.Logger) {
	if log == nil {
		return
	}
	defaultLogger = log
	slog.SetDefault(log)
}

func WithContext(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, log)
}

func FromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if log, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && log != nil {
			return log
		}
	}
	return Default()
}

func Debug(msg string, args ...any) {
	Default().Debug(msg, args...)
}

func Info(msg string, args ...any) {
	Default().Info(msg, args...)
}

func Warn(msg string, args ...any) {
	Default().Warn(msg, args...)
}

func Error(msg string, args ...any) {
	Default().Error(msg, args...)
}

func Fatal(msg string, args ...any) {
	Default().Error(msg, args...)
	os.Exit(1)
}
