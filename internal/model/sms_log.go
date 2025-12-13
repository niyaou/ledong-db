package model

import (
	"time"

	"gorm.io/gorm"
)

type SmsLog struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Phone     string         `gorm:"index" json:"phone"`
	Content   string         `json:"content"`
	Status    string         `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
