package model

import "gorm.io/gorm"

// Spend 消费记录模型
type Spend struct {
	ID          uint64  `gorm:"primaryKey;column:id" json:"id"`
	Charge      float32 `gorm:"column:charge" json:"charge"` // 消费金额
	Times       float32 `gorm:"column:times" json:"times"`
	AnnualTimes float32 `gorm:"column:annual_times" json:"annual_times"`
	Description float32 `gorm:"column:description" json:"description"` // 注意：保持为float32类型
	Quantities  int     `gorm:"column:quantities;default:1" json:"quantities"`

	// 外键字段
	PrepaidCardID uint64 `gorm:"column:prepaid_card_id;index" json:"prepaid_card_id"`
	CourseID      uint64 `gorm:"column:course_id;index" json:"course_id"`

	// 关联关系
	PrepaidCard PrepaidCard `gorm:"foreignKey:PrepaidCardID" json:"prepaid_card,omitempty"`
	Course      Course      `gorm:"foreignKey:CourseID" json:"course,omitempty"`

	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (Spend) TableName() string {
	return "spend"
}

// 查询示例：
// 1. 查询消费记录列表，同时加载预付费卡和课程信息（避免N+1查询）
//    var spends []Spend
//    db.Preload("PrepaidCard").Preload("Course").Find(&spends)
//
// 2. 查询某个预付费卡的所有消费记录，按课程开始时间倒序
//    var spends []Spend
//    db.Where("prepaid_card_id = ?", cardID).
//       Preload("Course").
//       Joins("JOIN course ON spend.course_id = course.id").
//       Order("course.start_time DESC").
//       Find(&spends)
//
// 3. 查询某个课程的所有消费记录
//    var spends []Spend
//    db.Where("course_id = ?", courseID).
//       Preload("PrepaidCard").
//       Find(&spends)
//
// 4. 聚合查询：统计某个课程的消费总额（非N+1，直接在数据库层面计算）
//    var total float32
//    db.Model(&Spend{}).Where("course_id = ?", courseID).
//       Select("COALESCE(SUM(charge), 0)").Scan(&total)
