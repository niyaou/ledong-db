package constants

import (
	"testing"
	"time"
)

func TestConfigureBusinessTime(t *testing.T) {
	original := time.Local
	t.Cleanup(func() { time.Local = original })

	if err := ConfigureBusinessTime(); err != nil {
		t.Fatalf("ConfigureBusinessTime() error = %v", err)
	}
	if got := time.Local.String(); got != BusinessTimeZone {
		t.Fatalf("time.Local = %q, want %q", got, BusinessTimeZone)
	}
}
