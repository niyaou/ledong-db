package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// CourseJSON Course 的 JSON 序列化结构，用于自定义时间格式
type CourseJSON struct {
	ID          uint64        `json:"id"`
	StartTime   string        `json:"startTime"`
	EndTime     string        `json:"endTime"`
	Duration    float32       `json:"duration"`
	CourseType  int           `json:"courseType"`
	IsAdult     *int          `json:"isAdult"`
	Notified    int           `json:"notified"`
	Description string        `json:"description"`
	Court       Court         `json:"court,omitempty"`
	Coach       Coach         `json:"coach"`
	Spends      []Spend       `json:"spend"`
	Members     []PrepaidCard `json:"member"`
}

// Course 课程模型
type Course struct {
	ID          uint64    `gorm:"primaryKey;column:id" json:"id"`
	StartTime   time.Time `gorm:"column:start_time;index" json:"startTime"`
	EndTime     time.Time `gorm:"column:end_time" json:"endTime"`
	Duration    float32   `gorm:"column:duration" json:"duration"`
	CourseType  int       `gorm:"column:course_type" json:"courseType"`     // -2体验课未成单,-1体验课成单,0订场，1班课，2私教
	IsAdult     *int      `gorm:"column:is_adult;default:1" json:"isAdult"` // 1=成人课程, 0=儿童课程
	Notified    int       `gorm:"column:notified" json:"notified"`
	Description string    `gorm:"column:description" json:"description"`

	// 外键字段
	CourtID uint64 `gorm:"column:court_id;index" json:"-"`
	CoachID uint64 `gorm:"column:coach_id;index" json:"-"`

	// 关联关系
	Court   Court         `gorm:"foreignKey:CourtID" json:"court,omitempty"`
	Coach   Coach         `gorm:"foreignKey:CoachID;references:ID" json:"coach"`
	Spends  []Spend       `gorm:"foreignKey:CourseID" json:"spend"`
	Members []PrepaidCard `gorm:"many2many:course_member;joinForeignKey:course_id;joinReferences:member_id" json:"member"`

	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (Course) TableName() string {
	return "course"
}

// MarshalJSON 自定义 JSON 序列化，将时间格式化为 "2006-01-02 15:04:05"
func (c Course) MarshalJSON() ([]byte, error) {
	courseJSON := CourseJSON{
		ID:          c.ID,
		StartTime:   c.StartTime.Format("2006-01-02 15:04:05"),
		EndTime:     c.EndTime.Format("2006-01-02 15:04:05"),
		Duration:    c.Duration,
		CourseType:  c.CourseType,
		IsAdult:     c.IsAdult,
		Notified:    c.Notified,
		Description: c.Description,
		Court:       c.Court,
		Coach:       c.Coach,
		Spends:      c.Spends,
		Members:     c.Members,
	}
	return json.Marshal(courseJSON)
}
