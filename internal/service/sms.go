package service

import (
	"ledong-db/internal/database"
	"ledong-db/internal/model"
	"ledong-db/pkg/tencent"
)

type SmsService struct {
	client *tencent.Client
}

func NewSmsService(client *tencent.Client) *SmsService {
	return &SmsService{client: client}
}

func (s *SmsService) Send(phone string, params []string) error {
	if err := s.client.Send(phone, params); err != nil {
		return err
	}
	
	log := &model.SmsLog{
		Phone:   phone,
		Content: params[0],
		Status:  "success",
	}
	database.DB.Create(log)
	return nil
}
