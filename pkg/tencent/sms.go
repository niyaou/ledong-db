package tencent

import (
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

func (c *Client) Send(phone string, params []string) error {
	req := sms.NewSendSmsRequest()
	req.SmsSdkAppId = &c.appId
	req.SignName = &c.signName
	req.TemplateId = &c.templateId
	// String[] templateParamSet = {"12月14日麓坊校区","次卡5次","50元次卡30年卡100"};
	req.TemplateParamSet = stringSliceToPtrSlice(params)
	phoneNumber := "+86" + phone
	req.PhoneNumberSet = []*string{&phoneNumber}
	// return nil
	_, err := c.smsClient.SendSms(req)
	return err
}

func stringSliceToPtrSlice(strs []string) []*string {
	result := make([]*string, len(strs))
	for i, s := range strs {
		val := s
		result[i] = &val
	}
	return result
}
