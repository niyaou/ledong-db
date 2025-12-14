package service

import (
	"fmt"
	"ledong-db/internal/database"
	"ledong-db/internal/model"
	"ledong-db/pkg/tencent"
	"strings"

	"gorm.io/gorm"
)

type SmsService struct {
	client *tencent.Client
	db     *gorm.DB
}

func NewSmsService(client *tencent.Client) *SmsService {
	return &SmsService{client: client, db: database.DB}
}

func (s *SmsService) Send(phone string, params []string) error {
	return s.client.Send(phone, params)
}

func (s *SmsService) Notify(id uint64) error {
	var course model.Course
	if err := s.db.Where("notified = ? AND id = ?", 0, id).
		Preload("Court").Preload("Spends.PrepaidCard").
		First(&course).Error; err != nil {
		return err
	}

	if err := s.NotifyCourse(&course); err != nil {
		return err
	}
	return s.db.Save(&course).Error
}

func (s *SmsService) NotifyCourse(course *model.Course) error {
	startTime := course.StartTime.Format("01月02日")
	court := course.Court.Name

	for _, spend := range course.Spends {
		member := spend.PrepaidCard
		if strings.HasPrefix(strings.ToLower(member.Number), "不发短信") {
			continue
		}

		var spendStr string
		if spend.Charge != 0 {
			spendStr += fmt.Sprintf("%.0f元", spend.Charge)
		}
		if spend.Times != 0 {
			spendStr += fmt.Sprintf("%.0f次", spend.Times)
		}
		if spend.AnnualTimes != 0 {
			spendStr += fmt.Sprintf("%.0f次", spend.AnnualTimes)
		}

		params := []string{
			startTime,
			court,
			spendStr,
			fmt.Sprintf("%.0f", member.RestCharge),
			fmt.Sprintf("%.0f", member.TimesCount),
			fmt.Sprintf("%.0f", member.AnnualCount),
		}

		if err := s.Send(member.Number, params); err != nil {
			return err
		}
	}

	course.Notified++
	return nil
}
