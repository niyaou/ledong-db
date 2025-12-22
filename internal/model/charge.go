package model

import (
	"time"

	"gorm.io/gorm"
)

// Charge 充值记录模型
type Charge struct {
	ID          uint64    `gorm:"primaryKey;column:id" json:"id"`
	Charge      float32   `gorm:"column:charge" json:"charge"`
	Times       float32   `gorm:"column:times" json:"times"`
	AnnualTimes float32   `gorm:"column:annual_times" json:"annualTimes"`
	Notified    int       `gorm:"column:notified" json:"notified"`
	Worth       int       `gorm:"column:worth;default:0" json:"worth"`
	Court       string    `gorm:"column:court" json:"court"`
	ChargedTime time.Time `gorm:"column:charged_time;index" json:"chargedTime"`
	Description string    `gorm:"column:description;size:200" json:"description"`

	// 外键字段
	PrepaidCardID uint64 `gorm:"column:prepaid_card_id;index" json:"prepaidCardId"`
	CoachID       uint64 `gorm:"column:coach_id;index" json:"coachId"`

	// 关联关系
	PrepaidCard PrepaidCard `gorm:"foreignKey:PrepaidCardID" json:"prepaidCard,omitempty"`
	Coach       Coach       `gorm:"foreignKey:CoachID" json:"coach,omitempty"`

	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (Charge) TableName() string {
	return "charge"
}

// 查询示例：
// 1. 查询充值记录列表，同时加载预付费卡和教练信息（避免N+1查询）
//    var charges []Charge
//    db.Preload("PrepaidCard").Preload("Coach").Find(&charges)
//
// 2. 查询某个预付费卡的所有充值记录
//    var charges []Charge
//    db.Where("prepaid_card_id = ?", cardID).
//       Preload("Coach").
//       Order("charged_time DESC").
//       Find(&charges)
//
// 3. 条件查询：查询某个时间段的充值记录
//    var charges []Charge
//    db.Where("charged_time >= ? AND charged_time <= ?", startTime, endTime).
//       Preload("PrepaidCard").Preload("Coach").
//       Find(&charges)
