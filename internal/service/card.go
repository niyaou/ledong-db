package service

import (
	"errors"
	"fmt"
	"ledong-db/internal/constants"
	"ledong-db/internal/database"
	"ledong-db/internal/model"
	"strings"
	"time"

	"gorm.io/gorm"
)

type CardService struct {
	db          *gorm.DB
	userService *UserService
}

func NewCardService(userService *UserService) *CardService {
	return &CardService{
		db:          database.DB,
		userService: userService,
	}
}

type ChargePage = Page[model.Charge]

func (s *CardService) SetRestCharge(number string, charged, times, annualTimes *float32, annualExpireTime string, worth *int, court, description, coachName, chargeTime string) (*model.Charge, error) {
	user, err := s.userService.FindByNumber(number)
	if err != nil {
		return nil, err
	}

	var coach model.Coach
	if err := s.db.Where("name = ?", coachName).First(&coach).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("教练不存在")
		}
		return nil, err
	}

	var parsedTime time.Time
	var parseErr error
	for _, format := range constants.TimeFormats {
		parsedTime, parseErr = time.ParseInLocation(format, chargeTime, time.Local)
		if parseErr == nil {
			break
		}
	}
	if parseErr != nil {
		return nil, fmt.Errorf("时间格式错误: %w", parseErr)
	}

	charge := &model.Charge{
		Charge:        0,
		Times:         0,
		AnnualTimes:   0,
		Court:         court,
		CoachID:       coach.ID,
		PrepaidCardID: user.ID,
		Description:   description,
		ChargedTime:   parsedTime,
	}

	if worth != nil && *worth != 0 {
		charge.Worth = *worth
		if err := s.userService.SetEquivalentChange(number, *worth); err != nil {
			return nil, err
		}
	}

	if charged != nil && *charged != 0 {
		charge.Charge = *charged
		if err := s.userService.SetRestChargeChange(number, *charged); err != nil {
			return nil, err
		}
	}

	if times != nil && *times != 0 {
		charge.Times = *times
		if err := s.userService.SetRestTimesChange(number, *times); err != nil {
			return nil, err
		}
	}

	if annualTimes != nil && *annualTimes != 0 {
		charge.AnnualTimes = *annualTimes
		if err := s.userService.SetRestAnnualTimesChange(number, *annualTimes); err != nil {
			return nil, err
		}
	}

	if annualExpireTime != "" {
		var expiredTime time.Time
		var parseErr error
		for _, format := range constants.TimeFormats {
			expiredTime, parseErr = time.ParseInLocation(format, annualExpireTime, time.Local)
			if parseErr == nil {
				break
			}
		}
		if parseErr != nil {
			return nil, fmt.Errorf("年卡过期时间格式错误: %w", parseErr)
		}
		if err := s.userService.SetAnnualTimesExpired(number, expiredTime); err != nil {
			return nil, err
		}
	}

	if err := s.db.Create(charge).Error; err != nil {
		return nil, err
	}

	return charge, nil
}

func (s *CardService) RetreatCharge(id uint64) (*model.Charge, error) {
	var charge model.Charge
	if err := s.db.Preload("PrepaidCard").First(&charge, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	number := charge.PrepaidCard.Number

	if err := s.userService.SetRestChargeChange(number, -charge.Charge); err != nil {
		return nil, err
	}
	if err := s.userService.SetRestTimesChange(number, -charge.Times); err != nil {
		return nil, err
	}
	if err := s.userService.SetRestAnnualTimesChange(number, -charge.AnnualTimes); err != nil {
		return nil, err
	}
	if err := s.userService.SetEquivalentChange(number, -charge.Worth); err != nil {
		return nil, err
	}

	if err := s.db.Delete(&charge).Error; err != nil {
		return nil, err
	}

	return &charge, nil
}

func (s *CardService) GetCharged(number string, pageNum, pageSize int) (*ChargePage, error) {
	user, err := s.userService.FindByNumber(number)
	if err != nil {
		return nil, err
	}

	query := s.db.Model(&model.Charge{}).
		Where("prepaid_card_id = ?", user.ID).
		Order("charged_time DESC")

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var charges []model.Charge
	offset := (pageNum - 1) * pageSize
	if err := query.
		Preload("PrepaidCard").
		Preload("Coach").
		Offset(offset).
		Limit(pageSize).
		Find(&charges).Error; err != nil {
		return nil, err
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return &ChargePage{
		Content:          charges,
		TotalElements:    total,
		TotalPages:       totalPages,
		Size:             pageSize,
		Number:           pageNum - 1,
		NumberOfElements: len(charges),
		First:            pageNum == 1,
		Last:             totalPages == 0 || pageNum >= totalPages,
	}, nil
}

func (s *CardService) GetChargedTotal(pageNum, pageSize int) (*ChargePage, error) {
	query := s.db.Model(&model.Charge{}).
		Order("charged_time DESC")

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var charges []model.Charge
	offset := (pageNum - 1) * pageSize
	if err := query.
		Preload("PrepaidCard").
		Preload("Coach").
		Offset(offset).
		Limit(pageSize).
		Find(&charges).Error; err != nil {
		return nil, err
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return &ChargePage{
		Content:          charges,
		TotalElements:    total,
		TotalPages:       totalPages,
		Size:             pageSize,
		Number:           pageNum - 1,
		NumberOfElements: len(charges),
		First:            pageNum == 1,
		Last:             totalPages == 0 || pageNum >= totalPages,
	}, nil
}

func (s *CardService) GetChargedByCoach(coachNumber string, startTime, endTime string) (string, error) {
	var coach model.Coach
	if err := s.db.Where("name = ?", coachNumber).First(&coach).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrUserNotFound
		}
		return "", err
	}

	var parsedStartTime, parsedEndTime time.Time
	var parseErr error
	for _, format := range constants.TimeFormats {
		parsedStartTime, parseErr = time.ParseInLocation(format, startTime, time.Local)
		if parseErr == nil {
			break
		}
	}
	if parseErr != nil {
		return "", fmt.Errorf("开始时间格式错误: %w", parseErr)
	}

	for _, format := range constants.TimeFormats {
		parsedEndTime, parseErr = time.ParseInLocation(format, endTime, time.Local)
		if parseErr == nil {
			break
		}
	}
	if parseErr != nil {
		return "", fmt.Errorf("结束时间格式错误: %w", parseErr)
	}

	var charges []model.Charge
	if err := s.db.Where("coach_id = ? AND charged_time >= ? AND charged_time <= ?", coach.ID, parsedStartTime, parsedEndTime).
		Preload("PrepaidCard").
		Order("charged_time DESC").
		Find(&charges).Error; err != nil {
		return "", err
	}

	var builder strings.Builder
	for _, charge := range charges {
		builder.WriteString(fmt.Sprintf("%s   充值时间: %s   充值:%.0f元,  %.0f次, %.0f年卡次,  %d等值  备注：%s\r\n",
			charge.PrepaidCard.Name,
			charge.ChargedTime.Format("2006-01-02 15:04:05"),
			charge.Charge,
			charge.Times,
			charge.AnnualTimes,
			charge.Worth,
			charge.Description))
	}

	return builder.String(), nil
}

type SpendPage = Page[model.Spend]

func (s *CardService) GetSpend(number string, pageNum, pageSize int) (*SpendPage, error) {
	query := s.db.Model(&model.Spend{}).
		Joins("JOIN course ON spend.course_id = course.id").
		Where("course.deleted_at IS NULL").
		Order("course.start_time DESC")

	if number != "" {
		// 使用JOIN直接查询，避免先查询user再查询spend
		query = query.Joins("JOIN prepaid_card ON spend.prepaid_card_id = prepaid_card.id").
			Where("prepaid_card.number = ?", number)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var spends []model.Spend
	offset := (pageNum - 1) * pageSize
	if err := query.
		Preload("PrepaidCard").
		Preload("Course", "deleted_at IS NULL").
		Preload("Course.Court").
		Preload("Course.Members").
		Offset(offset).
		Limit(pageSize).
		Find(&spends).Error; err != nil {
		return nil, err
	}

	// 手动加载 Coach，因为 Preload 可能无法正确识别关联关系
	if len(spends) > 0 {
		var coachIDs []uint64
		for _, spend := range spends {
			if spend.Course.CoachID > 0 {
				coachIDs = append(coachIDs, spend.Course.CoachID)
			}
		}
		if len(coachIDs) > 0 {
			var coaches []model.Coach
			if err := s.db.Where("coach_id IN ?", coachIDs).Find(&coaches).Error; err == nil {
				coachMap := make(map[uint64]*model.Coach)
				for i := range coaches {
					coachMap[coaches[i].ID] = &coaches[i]
				}
				for i := range spends {
					if coach, ok := coachMap[spends[i].Course.CoachID]; ok {
						spends[i].Course.Coach = *coach
					}
				}
			}
		}
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return &SpendPage{
		Content:          spends,
		TotalElements:    total,
		TotalPages:       totalPages,
		Size:             pageSize,
		Number:           pageNum - 1,
		NumberOfElements: len(spends),
		First:            pageNum == 1,
		Last:             totalPages == 0 || pageNum >= totalPages,
	}, nil
}
