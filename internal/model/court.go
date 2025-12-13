package model

import "gorm.io/gorm"

// Court 场地模型
type Court struct {
	ID        uint64         `gorm:"primaryKey;column:id" json:"id"`
	Name      string         `gorm:"column:name" json:"name"`
	IsActive  int            `gorm:"column:is_active;default:1" json:"is_active"`
	Courses   []Course       `gorm:"foreignKey:CourtID" json:"courses,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (Court) TableName() string {
	return "court"
}
