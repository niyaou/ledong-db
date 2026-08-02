package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"ledong-db/internal/logger"
	"ledong-db/pkg/tencent"
)

type fakeSMSSender struct {
	result tencent.SendResult
	err    error
	phone  string
	params []string
}

func (f *fakeSMSSender) SendContext(_ context.Context, phone string, params []string) (tencent.SendResult, error) {
	f.phone = phone
	f.params = params
	return f.result, f.err
}

func TestSmsServiceLogsSuccessfulSend(t *testing.T) {
	var output bytes.Buffer
	original := logger.Default()
	logger.SetDefault(logger.New(&output, "info"))
	t.Cleanup(func() { logger.SetDefault(original) })

	sender := &fakeSMSSender{result: tencent.SendResult{RequestID: "provider-request", SerialNo: "serial", Code: "Ok", Fee: 1}}
	service := NewSmsService(sender)
	if err := service.SendContext(context.Background(), "13800138000", []string{"private-content"}); err != nil {
		t.Fatalf("SendContext returned error: %v", err)
	}

	logs := output.String()
	if sender.phone != "13800138000" || !strings.Contains(logs, `"phone":"13800138000"`) {
		t.Fatalf("full phone number was not recorded: %s", logs)
	}
	if !strings.Contains(logs, `"provider_request_id":"provider-request"`) || !strings.Contains(logs, `"provider_code":"Ok"`) {
		t.Fatalf("provider result was not recorded: %s", logs)
	}
	if strings.Contains(logs, "private-content") {
		t.Fatal("SMS template parameters were written to logs")
	}
}

func TestSmsServiceLogsFailedSend(t *testing.T) {
	var output bytes.Buffer
	original := logger.Default()
	logger.SetDefault(logger.New(&output, "info"))
	t.Cleanup(func() { logger.SetDefault(original) })

	sender := &fakeSMSSender{result: tencent.SendResult{RequestID: "provider-request", Code: "LimitExceeded"}, err: errors.New("rejected")}
	service := NewSmsService(sender)
	if err := service.SendContext(context.Background(), "13800138000", nil); err == nil {
		t.Fatal("SendContext returned nil error")
	}
	if !strings.Contains(output.String(), `"msg":"sms send failed"`) || !strings.Contains(output.String(), `"provider_code":"LimitExceeded"`) {
		t.Fatalf("failed send was not recorded: %s", output.String())
	}
}
