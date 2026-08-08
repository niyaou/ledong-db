package model

import "time"

const (
	RechargeNoticeStatusPending      = "PENDING"
	RechargeNoticeStatusAcknowledged = "ACKNOWLEDGED"
)

// RechargeNotice is a coach-reported administrator task. It is deliberately
// independent from real charge and course records.
type RechargeNotice struct {
	ID             uint64     `gorm:"primaryKey;column:id" json:"id"`
	CoachID        uint64     `gorm:"column:coach_id;index" json:"coachId"`
	MemberID       uint64     `gorm:"column:member_id;index" json:"memberId"`
	RechargeDate   time.Time  `gorm:"column:recharge_date;type:date" json:"rechargeDate"`
	Note           string     `gorm:"column:note;size:500" json:"note"`
	Status         string     `gorm:"column:status;size:20;index" json:"status"`
	Version        uint64     `gorm:"column:version" json:"version"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      time.Time  `gorm:"column:updated_at" json:"updatedAt"`
	AcknowledgedAt *time.Time `gorm:"column:acknowledged_at" json:"acknowledgedAt"`
}

func (RechargeNotice) TableName() string {
	return "coach_recharge_notice"
}
