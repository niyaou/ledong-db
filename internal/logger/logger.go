package logger

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

type contextKey struct{}

var defaultLogger = New(os.Stdout, "info")

type Config struct {
	Level        string
	File         string
	MaxSizeMB    int
	MaxBackups   int
	MaxAgeDays   int
	Compress     bool
	UseLocalTime bool
}

func Init(cfg Config) (io.Closer, error) {
	return initWithWriter(cfg, os.Stdout)
}

func initWithWriter(cfg Config, console io.Writer) (io.Closer, error) {
	if strings.TrimSpace(cfg.File) == "" {
		return nil, errors.New("log file path cannot be empty")
	}
	if cfg.MaxSizeMB <= 0 {
		return nil, errors.New("log max size must be greater than zero")
	}
	if cfg.MaxBackups < 0 || cfg.MaxAgeDays < 0 {
		return nil, errors.New("log retention values cannot be negative")
	}

	directory := filepath.Dir(cfg.File)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, err
	}

	rollingFile := &lumberjack.Logger{
		Filename:   cfg.File,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		Compress:   cfg.Compress,
		LocalTime:  cfg.UseLocalTime,
	}
	SetDefault(New(io.MultiWriter(console, rollingFile), cfg.Level))
	return rollingFile, nil
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
