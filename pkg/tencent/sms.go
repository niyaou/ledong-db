package tencent

import (
	"context"
	"errors"
	"fmt"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

type Client struct {
	smsClient  *sms.Client
	appId      string
	signName   string
	templateId string
}

type SendResult struct {
	RequestID string
	SerialNo  string
	Code      string
	Message   string
	Fee       uint64
}

func NewClient(secretId, secretKey, region, appId, signName, templateId string) (*Client, error) {
	credential := common.NewCredential(secretId, secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "sms.tencentcloudapi.com"
	cpf.HttpProfile.ReqMethod = "POST"
	cpf.HttpProfile.ReqTimeout = 60
	cpf.SignMethod = "HmacSHA256"
	smsClient, err := sms.NewClient(credential, region, cpf)
	if err != nil {
		return nil, err
	}

	return &Client{
		smsClient:  smsClient,
		appId:      appId,
		signName:   signName,
		templateId: templateId,
	}, nil
}

func (c *Client) SendContext(ctx context.Context, phone string, params []string) (SendResult, error) {
	req := sms.NewSendSmsRequest()
	req.SmsSdkAppId = &c.appId
	req.SignName = &c.signName
	req.TemplateId = &c.templateId
	// String[] templateParamSet = {"12月14日麓坊校区","次卡5次","50元次卡30年卡100"};
	req.TemplateParamSet = stringSliceToPtrSlice(params)
	phoneNumber := "+86" + phone
	req.PhoneNumberSet = []*string{&phoneNumber}
	response, err := c.smsClient.SendSmsWithContext(ctx, req)
	if err != nil {
		return SendResult{}, fmt.Errorf("send sms request: %w", err)
	}
	return sendResultFromResponse(response)
}

func sendResultFromResponse(response *sms.SendSmsResponse) (SendResult, error) {
	if response == nil || response.Response == nil {
		return SendResult{}, errors.New("send sms request: empty provider response")
	}

	result := SendResult{RequestID: stringValue(response.Response.RequestId)}
	if len(response.Response.SendStatusSet) != 1 || response.Response.SendStatusSet[0] == nil {
		return result, errors.New("send sms request: missing provider send status")
	}

	status := response.Response.SendStatusSet[0]
	result.SerialNo = stringValue(status.SerialNo)
	result.Code = stringValue(status.Code)
	result.Message = stringValue(status.Message)
	if status.Fee != nil {
		result.Fee = *status.Fee
	}
	if result.Code != "Ok" {
		return result, fmt.Errorf("send sms rejected by provider: code=%s message=%s", result.Code, result.Message)
	}

	return result, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringSliceToPtrSlice(strs []string) []*string {
	result := make([]*string, len(strs))
	for i, s := range strs {
		val := s
		result[i] = &val
	}
	return result
}
