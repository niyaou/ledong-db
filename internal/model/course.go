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
	Spends  []Spend       `gorm:"foreignKey:CourseID" json:"spends,omitempty"`
	Members []PrepaidCard `gorm:"many2many:course_member;joinForeignKey:course_id;joinReferences:member_id" json:"members,omitempty"`

	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (Course) TableName() string {
	return "course"
}

// 查询示例：
// 1. 查询课程列表，同时加载教练、场地、会员信息（避免N+1查询）
//    var courses []Course
//    db.Preload("Coach").Preload("Court").Preload("Members").Find(&courses)
//
// 2. 查询单个课程详情
//    var course Course
//    db.Preload("Coach").Preload("Court").Preload("Members").Preload("Spends").First(&course, id)
//
// 3. 条件查询：查询某教练今年的课程
//    yearStart := time.Date(time.Now().Year(), 1, 1, 0, 0, 0, 0, time.Local)
//    var courses []Course
//    db.Where("coach_id = ? AND start_time >= ?", coachID, yearStart).
//       Preload("Court").Preload("Members").
//       Find(&courses)
//
// 4. 聚合查询：统计课程学员数（非N+1，直接在数据库层面计算）
//    var count int64
//    db.Table("course_member").Where("course_id = ?", courseID).Count(&count)
//
// 5. 聚合查询：统计课程所有学员消费总金额
//    var total float32
//    db.Table("spend").
//       Joins("JOIN course_member ON spend.prepaid_card_id = course_member.member_id").
//       Where("course_member.course_id = ?", courseID).
//       Select("COALESCE(SUM(spend.charge), 0)").
//       Scan(&total)
