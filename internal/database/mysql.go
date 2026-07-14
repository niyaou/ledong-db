package database

import (
	"fmt"
	"net/url"
	"time"

	"ledong-db/internal/config"
	"ledong-db/internal/constants"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init(cfg config.DatabaseConfig) error {
	dsn := buildDSN(cfg)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		PrepareStmt: true,
	})
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	DB = db
	return nil
}

func buildDSN(cfg config.DatabaseConfig) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=%s&time_zone=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database,
		url.QueryEscape(constants.BusinessTimeZone),
		url.QueryEscape("'"+constants.BusinessTimeOffset+"'"),
	)
}
