package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Tencent  TencentConfig
	Cache    CacheConfig
}

type ServerConfig struct {
	Port string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

type TencentConfig struct {
	SecretId   string
	SecretKey  string
	SmsAppId   string
	SignName   string
	TemplateId string
}

type CacheConfig struct {
	MaxSizeMB   int
	Shards      int
	LifeWindow  time.Duration
	CleanWindow time.Duration
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "3306"),
			User:     getEnv("DB_USER", "root"),
			Password: getEnv("DB_PASSWORD", ""),
			Database: getEnv("DB_NAME", "ledong_db"),
		},
		Tencent: TencentConfig{
			SecretId:   getEnv("TENCENT_SECRET_ID", ""),
			SecretKey:  getEnv("TENCENT_SECRET_KEY", ""),
			SmsAppId:   getEnv("TENCENT_SMS_APP_ID", ""),
			SignName:   getEnv("TENCENT_SMS_SIGN_NAME", ""),
			TemplateId: getEnv("TENCENT_SMS_TEMPLATE_ID", ""),
		},
		Cache: CacheConfig{
			MaxSizeMB:   getEnvAsInt("CACHE_MAX_SIZE_MB", 100),
			Shards:      getEnvAsInt("CACHE_SHARDS", 1024),
			LifeWindow:  getEnvAsDuration("CACHE_LIFE_WINDOW", 10*time.Minute),
			CleanWindow: getEnvAsDuration("CACHE_CLEAN_WINDOW", 5*time.Minute),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
