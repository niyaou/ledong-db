package model

import (
	"time"

	"gorm.io/gorm"
)

// Course 课程模型
type Course struct {
	ID          uint64    `gorm:"primaryKey;column:id" json:"id"`
	StartTime   time.Time `gorm:"column:start_time;index" json:"start_time"`
	EndTime     time.Time `gorm:"column:end_time" json:"end_time"`
	Duration    float32   `gorm:"column:duration" json:"duration"`
	CourseType  int       `gorm:"column:course_type" json:"course_type"`     // -2体验课未成单,-1体验课成单,0订场，1班课，2私教
	IsAdult     *int      `gorm:"column:is_adult;default:1" json:"is_adult"` // 1=成人课程, 0=儿童课程
	Notified    int       `gorm:"column:notified" json:"notified"`
	Description string    `gorm:"column:description" json:"description"`

	// 外键字段
	CourtID uint64 `gorm:"column:court_id;index" json:"court_id"`
	CoachID uint64 `gorm:"column:coach_id;index" json:"coach_id"`

	// 关联关系
	Court   Court         `gorm:"foreignKey:CourtID" json:"court,omitempty"`
	Coach   Coach         `gorm:"foreignKey:CoachID" json:"coach,omitempty"`
	Spends  []Spend       `gorm:"foreignKey:CourseID" json:"spend"`
	Members []PrepaidCard `gorm:"many2many:course_member;joinForeignKey:course_id;joinReferences:member_id" json:"member"`

	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (Course) TableName() string {
	return "course"
}
