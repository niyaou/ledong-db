package database

import (
	"strings"
	"testing"

	"ledong-db/internal/config"
)

func TestBuildDSNUsesBusinessTimeZone(t *testing.T) {
	dsn := buildDSN(config.DatabaseConfig{})
	if !strings.Contains(dsn, "parseTime=True&loc=Asia%2FShanghai") {
		t.Fatalf("DSN does not use Asia/Shanghai: %s", dsn)
	}
	if !strings.Contains(dsn, "time_zone=%27%2B08%3A00%27") {
		t.Fatalf("DSN does not set the MySQL session timezone: %s", dsn)
	}
}
