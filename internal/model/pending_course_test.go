package model

import (
	"reflect"
	"testing"
)

func TestPendingJSONScanAndValue(t *testing.T) {
	var value PendingJSON
	if err := value.Scan([]byte(`[{"member_id":1}]`)); err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	got, err := value.Value()
	if err != nil {
		t.Fatalf("value failed: %v", err)
	}
	if !reflect.DeepEqual(got, []byte(`[{"member_id":1}]`)) {
		t.Fatalf("unexpected value: %q", got)
	}
}

func TestPendingJSONRejectsInvalidJSON(t *testing.T) {
	var value PendingJSON
	if err := value.Scan([]byte(`{`)); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}
