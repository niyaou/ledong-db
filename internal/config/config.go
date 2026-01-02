package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig   `mapstructure:"server"`
	Database  DatabaseConfig `mapstructure:"database"`
	Tencent   TencentConfig  `mapstructure:"tencent"`
	Cache     CacheConfig    `mapstructure:"cache"`
	SecretKey string         `mapstructure:"secret_key"`
}

type ServerConfig struct {
	Port     string `mapstructure:"port"`
	LogLevel string `mapstructure:"log_level"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

type TencentConfig struct {
	SecretId   string `mapstructure:"secret_id"`
	SecretKey  string `mapstructure:"secret_key"`
	Region     string `mapstructure:"region"`
	SmsAppId   string `mapstructure:"sms_app_id"`
	SignName   string `mapstructure:"sign_name"`
	TemplateId string `mapstructure:"template_id"`
}

type CacheConfig struct {
	MaxSizeMB   int           `mapstructure:"max_size_mb"`
	Shards      int           `mapstructure:"shards"`
	LifeWindow  time.Duration `mapstructure:"life_window"`
	CleanWindow time.Duration `mapstructure:"clean_window"`
}

func Load() *Config {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath(findProjectRoot())
	v.AddConfigPath(getExecutableDir())

	v.SetEnvPrefix("")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.SetDefault("server.port", "8080")
	v.SetDefault("server.log_level", "info")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", "3306")
	v.SetDefault("database.user", "root")
	v.SetDefault("database.database", "ledong_db")
	v.SetDefault("cache.max_size_mb", 100)
	v.SetDefault("cache.shards", 1024)
	v.SetDefault("cache.life_window", "10m")
	v.SetDefault("cache.clean_window", "5m")
	v.SetDefault("tencent.region", "ap-guangzhou")
	v.SetDefault("tencent.sms_app_id", "1400779674")
	v.SetDefault("tencent.sign_name", "成都乐动精灵体育")
	v.SetDefault("tencent.template_id", "1640539")

	if err := v.ReadInConfig(); err != nil {
		_ = err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return &Config{}
	}

	return &cfg
}

func findProjectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func getExecutableDir() string {
	execPath, err := os.Executable()
	if err != nil {
		return ""
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return ""
	}
	return filepath.Dir(execPath)
}
