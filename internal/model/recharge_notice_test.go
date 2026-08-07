package model

import "testing"

func TestRechargeNoticeIsIndependentTable(t *testing.T) {
	if got := (RechargeNotice{}).TableName(); got != "coach_recharge_notice" {
		t.Fatalf("TableName() = %q", got)
	}
	if RechargeNoticeStatusPending == RechargeNoticeStatusAcknowledged {
		t.Fatal("recharge notice states must be distinct")
	}
}
