package tencent

import (
	"testing"

	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

func TestSendResultFromResponse(t *testing.T) {
	requestID := "request-id"
	serialNo := "serial-no"
	code := "Ok"
	message := "send success"
	fee := uint64(1)
	response := &sms.SendSmsResponse{Response: &sms.SendSmsResponseParams{
		RequestId: &requestID,
		SendStatusSet: []*sms.SendStatus{{
			SerialNo: &serialNo,
			Code:     &code,
			Message:  &message,
			Fee:      &fee,
		}},
	}}

	result, err := sendResultFromResponse(response)
	if err != nil {
		t.Fatalf("sendResultFromResponse returned error: %v", err)
	}
	if result.RequestID != requestID || result.SerialNo != serialNo || result.Code != code || result.Fee != fee {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestSendResultFromResponseRejectsProviderFailure(t *testing.T) {
	code := "FailedOperation.PhoneNumberInBlacklist"
	message := "phone number is in blacklist"
	response := &sms.SendSmsResponse{Response: &sms.SendSmsResponseParams{
		SendStatusSet: []*sms.SendStatus{{Code: &code, Message: &message}},
	}}

	result, err := sendResultFromResponse(response)
	if err == nil {
		t.Fatal("sendResultFromResponse returned nil error")
	}
	if result.Code != code {
		t.Fatalf("provider code = %q, want %q", result.Code, code)
	}
}

func TestSendResultFromResponseRejectsEmptyResponse(t *testing.T) {
	if _, err := sendResultFromResponse(nil); err == nil {
		t.Fatal("sendResultFromResponse returned nil error")
	}
}
