package logger

import (
	"bytes"
	"context"
	"encoding/json"
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
