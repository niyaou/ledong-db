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
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "ledong-db/docs"
	"ledong-db/internal/cache"
	"ledong-db/internal/config"
	"ledong-db/internal/database"
	"ledong-db/internal/handler"
	"ledong-db/internal/logger"
	"ledong-db/internal/router"
	"ledong-db/internal/service"
	"ledong-db/pkg/tencent"
)

func main() {
	cfg := config.Load()
	logger.Init(cfg.Server.LogLevel)

	if err := database.Init(cfg.Database); err != nil {
		logger.Fatal("数据库初始化失败", "error", err)
	}

	if err := cache.Init(cfg.Cache.MaxSizeMB, cfg.Cache.Shards, cfg.Cache.LifeWindow, cfg.Cache.CleanWindow); err != nil {
		logger.Fatal("缓存初始化失败", "error", err)
	}

	smsClient, err := tencent.NewClient(cfg.Tencent.SecretId, cfg.Tencent.SecretKey, cfg.Tencent.Region, cfg.Tencent.SmsAppId, cfg.Tencent.SignName, cfg.Tencent.TemplateId)
	if err != nil {
		logger.Fatal("腾讯云客户端初始化失败", "error", err)
	}

	smsService := service.NewSmsService(smsClient)
	smsHandler := handler.NewSmsHandler(smsService)
	userService := service.NewUserService()
	courseService := service.NewCourseService(userService, smsService)
	courseHandler := handler.NewCourseHandler(courseService, smsService)
	efficiencyService := service.NewEfficiencyService()
	efficiencyHandler := handler.NewEfficiencyHandler(efficiencyService)
	cardService := service.NewCardService(userService)
	cardHandler := handler.NewCardHandler(cardService)
	userHandler := handler.NewUserHandler(userService, cardService, smsService)
	r := router.NewRouter(smsHandler, courseHandler, efficiencyHandler, userHandler, cardHandler)

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
	<-quit

	logger.Info("正在关闭服务器...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("服务器关闭失败", "error", err)
	}
	logger.Info("服务器已关闭")
}
