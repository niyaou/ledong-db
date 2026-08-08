// @title           Ledong DB API
// @version         1.0
// @description     乐动数据库服务API文档
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:31168
// @BasePath  /api

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "ledong-db/docs"
	"ledong-db/internal/cache"
	"ledong-db/internal/config"
	"ledong-db/internal/constants"
	"ledong-db/internal/database"
	"ledong-db/internal/handler"
	"ledong-db/internal/logger"
	"ledong-db/internal/router"
	"ledong-db/internal/service"
	"ledong-db/pkg/tencent"
)

func main() {
	cfg := config.Load()
	logCloser, err := logger.Init(logger.Config{
		Level:        cfg.Server.LogLevel,
		File:         cfg.Server.LogFile,
		MaxSizeMB:    cfg.Server.LogMaxSizeMB,
		MaxBackups:   cfg.Server.LogMaxBackups,
		MaxAgeDays:   cfg.Server.LogMaxAgeDays,
		Compress:     cfg.Server.LogCompress,
		UseLocalTime: cfg.Server.LogUseLocalTime,
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "initialize logging: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := logCloser.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "close log file: %v\n", err)
		}
	}()
	logger.Info("logging initialized", "file", cfg.Server.LogFile, "level", cfg.Server.LogLevel, "max_size_mb", cfg.Server.LogMaxSizeMB, "max_backups", cfg.Server.LogMaxBackups, "max_age_days", cfg.Server.LogMaxAgeDays, "compress", cfg.Server.LogCompress)

	if err := constants.ConfigureBusinessTime(); err != nil {
		logger.Fatal("业务时区初始化失败", "time_zone", constants.BusinessTimeZone, "error", err)
	}
	logger.Info("business time zone initialized", "time_zone", constants.BusinessTimeZone)

	if err := database.Init(cfg.Database); err != nil {
		logger.Fatal("数据库初始化失败", "error", err)
	}
	logger.Info("database initialized", "host", cfg.Database.Host, "port", cfg.Database.Port, "database", cfg.Database.Database)

	if err := cache.Init(cfg.Cache.MaxSizeMB, cfg.Cache.Shards, cfg.Cache.LifeWindow, cfg.Cache.CleanWindow); err != nil {
		logger.Fatal("缓存初始化失败", "error", err)
	}
	logger.Info("cache initialized", "max_size_mb", cfg.Cache.MaxSizeMB, "shards", cfg.Cache.Shards, "life_window", cfg.Cache.LifeWindow, "clean_window", cfg.Cache.CleanWindow)

	smsClient, err := tencent.NewClient(cfg.Tencent.SecretId, cfg.Tencent.SecretKey, cfg.Tencent.Region, cfg.Tencent.SmsAppId, cfg.Tencent.SignName, cfg.Tencent.TemplateId)
	if err != nil {
		logger.Fatal("腾讯云客户端初始化失败", "error", err)
	}
	logger.Info("tencent sms client initialized", "region", cfg.Tencent.Region, "sms_app_id", cfg.Tencent.SmsAppId, "template_id", cfg.Tencent.TemplateId)

	smsService := service.NewSmsService(smsClient)
	smsHandler := handler.NewSmsHandler(smsService)
	userService := service.NewUserService()
	courseService := service.NewCourseService(userService, smsService)
	courseHandler := handler.NewCourseHandler(courseService, smsService)
	pendingCourseService := service.NewPendingCourseService(courseService)
	pendingCourseHandler := handler.NewPendingCourseHandler(pendingCourseService)
	rechargeNoticeService := service.NewRechargeNoticeService(pendingCourseService)
	rechargeNoticeHandler := handler.NewRechargeNoticeHandler(rechargeNoticeService)
	efficiencyService := service.NewEfficiencyService()
	efficiencyHandler := handler.NewEfficiencyHandler(efficiencyService)
	cardService := service.NewCardService(userService)
	cardHandler := handler.NewCardHandler(cardService)
	userHandler := handler.NewUserHandler(userService, cardService, smsService)
	r := router.NewRouter(smsHandler, courseHandler, efficiencyHandler, userHandler, cardHandler, pendingCourseHandler, rechargeNoticeHandler)

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		logger.Info("服务器启动", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("服务器启动失败", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	receivedSignal := <-quit

	logger.Info("server shutdown started", "signal", receivedSignal.String())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("服务器关闭失败", "error", err)
	}
	logger.Info("服务器已关闭")
}
