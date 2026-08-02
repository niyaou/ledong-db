package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewProducesJSONAndHonorsLevel(t *testing.T) {
	var output bytes.Buffer
	log := New(&output, "warn")

	log.Info("hidden")
	log.Warn("visible", "component", "test")

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode log entry: %v", err)
	}
	if entry["msg"] != "visible" || entry["level"] != "WARN" || entry["component"] != "test" {
		t.Fatalf("unexpected log entry: %#v", entry)
	}
}

func TestLoggerRoundTripsThroughContext(t *testing.T) {
	log := New(&bytes.Buffer{}, "info")
	ctx := WithContext(context.Background(), log)

	if got := FromContext(ctx); got != log {
		t.Fatal("context logger was not returned")
	}
}

func TestInitWritesAndRotatesLogFile(t *testing.T) {
	original := Default()
	t.Cleanup(func() { SetDefault(original) })

	logFile := filepath.Join(t.TempDir(), "nested", "service.log")
	closer, err := initWithWriter(Config{
		Level:        "info",
		File:         logFile,
		MaxSizeMB:    1,
		MaxBackups:   2,
		MaxAgeDays:   1,
		UseLocalTime: true,
	}, io.Discard)
	if err != nil {
		t.Fatalf("initialize rolling logger: %v", err)
	}

	payload := strings.Repeat("x", 128*1024)
	for range 10 {
		Info("rotation test", "payload", payload)
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("close rolling logger: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(filepath.Dir(logFile), "service*.log"))
	if err != nil {
		t.Fatalf("find log files: %v", err)
	}
	if len(files) < 2 {
		t.Fatalf("log rotation did not create a backup: %v", files)
	}
	current, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read current log file: %v", err)
	}
	if !bytes.Contains(current, []byte(`"msg":"rotation test"`)) {
		t.Fatal("current log file does not contain structured entries")
	}
}

func TestInitRejectsInvalidRollingConfig(t *testing.T) {
	if _, err := initWithWriter(Config{MaxSizeMB: 1}, io.Discard); err == nil {
		t.Fatal("empty log file path was accepted")
	}
	if _, err := initWithWriter(Config{File: "service.log", MaxSizeMB: 0}, io.Discard); err == nil {
		t.Fatal("invalid max size was accepted")
	}
}
