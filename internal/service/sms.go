package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ledong-db/internal/database"
	"ledong-db/internal/logger"
	"ledong-db/internal/model"
	"ledong-db/pkg/tencent"

	"gorm.io/gorm"
)

type SmsService struct {
	client smsSender
	db     *gorm.DB
}

type smsSender interface {
	SendContext(ctx context.Context, phone string, params []string) (tencent.SendResult, error)
}

func NewSmsService(client smsSender) *SmsService {
	return &SmsService{client: client, db: database.DB}
}

func (s *SmsService) Send(phone string, params []string) error {
	return s.SendContext(context.Background(), phone, params)
}

func (s *SmsService) SendContext(ctx context.Context, phone string, params []string) error {
	log := logger.FromContext(ctx)
	startedAt := time.Now()
	log.Info("sms send started", "phone", phone, "param_count", len(params))

	result, err := s.client.SendContext(ctx, phone, params)
	attrs := []any{
		"phone", phone,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"provider_request_id", result.RequestID,
		"provider_serial_no", result.SerialNo,
		"provider_code", result.Code,
	}
	if err != nil {
		log.Error("sms send failed", append(attrs, "error", err)...)
		return err
	}

	log.Info("sms send succeeded", append(attrs, "fee", result.Fee)...)
	return nil
}

func (s *SmsService) Notify(ctx context.Context, id uint64) error {
	var course model.Course
	if err := s.db.Where("notified = ? AND id = ? AND deleted_at IS NULL", 0, id).
		Preload("Court").Preload("Spends.PrepaidCard").
		First(&course).Error; err != nil {
		return err
	}

	if err := s.NotifyCourse(ctx, &course); err != nil {
		return err
	}
	return s.db.Save(&course).Error
}

func (s *SmsService) NotifyAll(ctx context.Context, id *uint64) error {
	query := s.db.Where("notified = ? AND deleted_at IS NULL", 0)
	if id != nil {
		query = query.Where("id = ?", *id)
	}

	var courses []model.Course
	if err := query.Preload("Court").Preload("Spends.PrepaidCard").Find(&courses).Error; err != nil {
		return err
	}

	for _, course := range courses {
		if err := s.NotifyCourse(ctx, &course); err != nil {
			return err
		}
		if err := s.db.Save(&course).Error; err != nil {
			return err
		}
	}

	return nil
}

func (s *SmsService) NotifyCourse(ctx context.Context, course *model.Course) error {
	startTime := course.StartTime.Format("01月02日")
	court := course.Court.Name

	sentCount := 0
	skippedCount := 0
	for _, spend := range course.Spends {
		member := spend.PrepaidCard
		if strings.HasPrefix(strings.ToLower(member.Number), "不发短信") {
			skippedCount++
			continue
		}

		var spendStr string
		if spend.Charge != 0 {
			spendStr += fmt.Sprintf("%.0f元", spend.Charge)
		}
		if spend.Times != 0 {
			spendStr += fmt.Sprintf("%.1f次", spend.Times)
		}
		if spend.AnnualTimes != 0 {
			spendStr += fmt.Sprintf("%.1f次", spend.AnnualTimes)
		}

		params := []string{
			startTime,
			court,
			spendStr,
			fmt.Sprintf("%.0f", member.RestCharge),
			fmt.Sprintf("%.1f", member.TimesCount),
			fmt.Sprintf("%.1f", member.AnnualCount),
		}

		if err := s.SendContext(ctx, member.Number, params); err != nil {
			logger.FromContext(ctx).Error("course sms notification failed", "course_id", course.ID, "sent_count", sentCount, "skipped_count", skippedCount, "error", err)
			return err
		}
		sentCount++
	}

	course.Notified++
	logger.FromContext(ctx).Info("course sms notification completed", "course_id", course.ID, "sent_count", sentCount, "skipped_count", skippedCount)
	return nil
}
