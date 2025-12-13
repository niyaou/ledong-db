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

func NewClient(secretId, secretKey, appId, signName, templateId string) (*Client, error) {
	credential := common.NewCredential(secretId, secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "sms.tencentcloudapi.com"
	smsClient, _ := sms.NewClient(credential, "", cpf)

	return &Client{
		smsClient:  smsClient,
		appId:      appId,
		signName:   signName,
		templateId: templateId,
	}, nil
}

func (c *Client) Send(phone string, params []string) error {

	return nil
}

func stringSliceToPtrSlice(strs []string) []*string {
	result := make([]*string, len(strs))
	for i, s := range strs {
		val := s
		result[i] = &val
	}
	return result
}
