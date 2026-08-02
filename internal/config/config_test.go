package config

import "testing"

func TestLoadServerLogConfigFromEnvironment(t *testing.T) {
	t.Setenv("SERVER_LOG_LEVEL", "warn")
	t.Setenv("SERVER_LOG_FILE", "D:/logs/ledong/server.log")
	t.Setenv("SERVER_LOG_MAX_SIZE_MB", "64")
	t.Setenv("SERVER_LOG_MAX_BACKUPS", "7")
	t.Setenv("SERVER_LOG_MAX_AGE_DAYS", "14")
	t.Setenv("SERVER_LOG_COMPRESS", "false")
	t.Setenv("SERVER_LOG_USE_LOCAL_TIME", "false")

	cfg := Load()
	if cfg.Server.LogLevel != "warn" || cfg.Server.LogFile != "D:/logs/ledong/server.log" {
		t.Fatalf("unexpected log destination config: %#v", cfg.Server)
	}
	if cfg.Server.LogMaxSizeMB != 64 || cfg.Server.LogMaxBackups != 7 || cfg.Server.LogMaxAgeDays != 14 {
		t.Fatalf("unexpected log retention config: %#v", cfg.Server)
	}
	if cfg.Server.LogCompress || cfg.Server.LogUseLocalTime {
		t.Fatalf("unexpected log boolean config: %#v", cfg.Server)
	}
}
