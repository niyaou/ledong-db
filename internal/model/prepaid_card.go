package model

import (
	"time"

	"gorm.io/gorm"
)

// PrepaidCard 预付费卡模型
type PrepaidCard struct {
	ID                uint64     `gorm:"primaryKey;column:id" json:"id"`
	Name              string     `gorm:"column:name" json:"name"`
	Number            string     `gorm:"column:number;index" json:"number"`
	Court             string     `gorm:"column:court" json:"court"`
	RestCharge        float32    `gorm:"column:rest_charge;default:0" json:"rest_charge"`
	AnnualCount       float32    `gorm:"column:annual_count;default:0" json:"annual_count"`
	TimesCount        float32    `gorm:"column:times_count;default:0" json:"times_count"`
	EquivalentBalance int        `gorm:"column:equivalent_balance;default:0" json:"equivalent_balance"`
	Younths           int        `gorm:"column:younths;default:0" json:"younths"`
	Adults            int        `gorm:"column:adults;default:0" json:"adults"`
	AnnualExpireTime  *time.Time `gorm:"column:annual_expire_time;index" json:"annual_expire_time,omitempty"`
	TimesExpireTime   *time.Time `gorm:"column:times_expire_time;index" json:"times_expire_time,omitempty"`

	// 关联关系
	Courses []Course `gorm:"many2many:course_member;joinForeignKey:member_id;joinReferences:course_id" json:"courses,omitempty"`
	Charges []Charge `gorm:"foreignKey:PrepaidCardID" json:"charges,omitempty"`
	Spends  []Spend  `gorm:"foreignKey:PrepaidCardID" json:"spends,omitempty"`

	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (PrepaidCard) TableName() string {
	return "prepaid_card"
}

// 查询示例：
// 1. 查询预付费卡列表，同时加载关联的课程、充值记录、消费记录
//    var cards []PrepaidCard
//    db.Preload("Courses").Preload("Charges").Preload("Spends").Find(&cards)
//
// 2. 查询单个预付费卡，加载所有关联数据
//    var card PrepaidCard
//    db.Preload("Courses").Preload("Charges").Preload("Spends").First(&card, id)
//
// 3. 聚合查询：查询预付费卡的充值总额（非N+1，直接在数据库层面计算）
//    var total float32
//    db.Model(&PrepaidCard{}).Where("id = ?", cardID).
//       Select("COALESCE(SUM(rest_charge), 0)").Scan(&total)
