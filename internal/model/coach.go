package model

import "gorm.io/gorm"

// Coach 教练模型
type Coach struct {
	ID        uint64         `gorm:"primaryKey;column:coach_id" json:"id"`
	Name      string         `gorm:"column:name" json:"name"`
	Number    string         `gorm:"column:number;index" json:"number"`
	IsActive  int            `gorm:"column:is_active;default:1" json:"isActive"`
	Level     int            `gorm:"column:level" json:"level"`
	Courses   []Course       `gorm:"foreignKey:CoachID" json:"courses,omitempty"`
	Charges   []Charge       `gorm:"foreignKey:CoachID" json:"charges,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (Coach) TableName() string {
	return "coach"
}
