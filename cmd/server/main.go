package main

import (
	"fmt"
	"log"

	"github.com/allegro/bigcache/v3"
	"github.com/gin-gonic/gin"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/hints"

	"ledong-db/internal/cache"
	"ledong-db/internal/config"
	"ledong-db/internal/database"
	"ledong-db/internal/handler"
	"ledong-db/internal/router"
	"ledong-db/internal/service"
	"ledong-db/pkg/tencent"
)

var (
	_ *bigcache.BigCache
	_ *gin.Engine
	_ *common.Credential
	_ *sms.Client
	_ mysql.Dialector
	_ *gorm.DB
	_ hints.IndexHint
)

func main() {
	cfg := config.Load()

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Database.User, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port, cfg.Database.Database)
	if err := database.Init(dsn); err != nil {
		log.Fatal(err)
	}

	if err := cache.Init(cfg.Cache.MaxSizeMB, cfg.Cache.Shards, cfg.Cache.LifeWindow, cfg.Cache.CleanWindow); err != nil {
		log.Fatal(err)
	}

	smsClient, err := tencent.NewClient(cfg.Tencent.SecretId, cfg.Tencent.SecretKey, cfg.Tencent.SmsAppId, cfg.Tencent.SignName, cfg.Tencent.TemplateId)
	if err != nil {
		log.Fatal(err)
	}

	smsService := service.NewSmsService(smsClient)
	smsHandler := handler.NewSmsHandler(smsService)
	r := router.NewRouter(smsHandler)

	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatal(err)
	}
}
