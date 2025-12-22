package service

import (
	"errors"
	"ledong-db/internal/database"
	"ledong-db/internal/model"
	"time"

	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService() *UserService {
	return &UserService{db: database.DB}
}

var (
	ErrUserExist    = errors.New("用户已存在")
	ErrUserNotFound = errors.New("用户不存在")
)

func (s *UserService) CreateUser(name, number, court string) (*model.PrepaidCard, error) {
	var existUser model.PrepaidCard
	if err := s.db.Where("number = ?", number).First(&existUser).Error; err == nil {
		return nil, ErrUserExist
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	user := &model.PrepaidCard{
		Name:              name,
		Number:            number,
		Court:             court,
		EquivalentBalance: 0,
		RestCharge:        0,
		TimesCount:        0,
		AnnualCount:       0,
		Younths:           0,
		Adults:            0,
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, err
	}

	return s.FindByNumber(number)
}

func (s *UserService) FindByNumber(number string) (*model.PrepaidCard, error) {
	var user model.PrepaidCard
	if err := s.db.Where("number = ?", number).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserService) SetRestChargeChange(number string, changed float32) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var user model.PrepaidCard
		if err := tx.Where("number = ?", number).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}
		user.RestCharge += changed
		return tx.Save(&user).Error
	})
}

func (s *UserService) SetEquivalentChange(number string, equivalent int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var user model.PrepaidCard
		if err := tx.Where("number = ?", number).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}
		user.EquivalentBalance += equivalent
		return tx.Save(&user).Error
	})
}

func (s *UserService) SetRestTimesChange(number string, times float32) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var user model.PrepaidCard
		if err := tx.Where("number = ?", number).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}
		user.TimesCount += times
		return tx.Save(&user).Error
	})
}

func (s *UserService) SetRestAnnualTimesChange(number string, annualTimes float32) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var user model.PrepaidCard
		if err := tx.Where("number = ?", number).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}
		user.AnnualCount += annualTimes
		return tx.Save(&user).Error
	})
}

func (s *UserService) SetTimesExpired(number string, expiredTime time.Time) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var user model.PrepaidCard
		if err := tx.Where("number = ?", number).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}
		user.TimesExpireTime = &expiredTime
		return tx.Save(&user).Error
	})
}

func (s *UserService) SetAnnualTimesExpired(number string, expiredTime time.Time) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var user model.PrepaidCard
		if err := tx.Where("number = ?", number).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}
		user.AnnualExpireTime = &expiredTime
		return tx.Save(&user).Error
	})
}

func (s *UserService) SetYonthAndAdult(number string, yonth, adult int) (*model.PrepaidCard, error) {
	var user model.PrepaidCard
	if err := s.db.Where("number = ?", number).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	user.Younths = yonth
	user.Adults = adult

	if err := s.db.Save(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *UserService) GetMembers() ([]model.PrepaidCard, error) {
	var members []model.PrepaidCard
	if err := s.db.Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (s *UserService) GetMember(number string) (*model.PrepaidCard, error) {
	return s.FindByNumber(number)
}

func (s *UserService) GetCoaches() ([]model.Coach, error) {
	var coaches []model.Coach
	if err := s.db.Where("is_active = ?", 1).Find(&coaches).Error; err != nil {
		return nil, err
	}
	return coaches, nil
}

func (s *UserService) GetCourts() ([]model.Court, error) {
	var courts []model.Court
	if err := s.db.Find(&courts).Error; err != nil {
		return nil, err
	}
	return courts, nil
}
