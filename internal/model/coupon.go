package model

import "time"

// Coupon represents a discount coupon
type Coupon struct {
	Model
	Code              string    `gorm:"type:varchar(64);uniqueIndex" json:"code"`
	Name              string    `gorm:"type:varchar(128)" json:"name"`
	Type              int       `gorm:"type:int(11);default:1" json:"type"` // 1=ratio,2=amount
	Value             float64   `gorm:"type:decimal(10,2);default:0" json:"value"`
	Enable            int       `gorm:"type:tinyint(1);default:1" json:"enable"`
	LimitUse          int       `gorm:"type:int(11);default:0" json:"limit_use"`
	LimitUseSameUser  int       `gorm:"type:int(11);default:0" json:"limit_use_same_user"`
	LimitPlanIDs      Ints      `gorm:"type:json" json:"limit_plan_ids"`
	LimitPeriodStart  time.Time `gorm:"type:datetime" json:"limit_period_start"`
	LimitPeriodEnd    time.Time `gorm:"type:datetime" json:"limit_period_end"`
	StartedAt         time.Time `gorm:"type:datetime" json:"started_at"`
	EndedAt           time.Time `gorm:"type:datetime" json:"ended_at"`
	UsedCount         int       `gorm:"type:int(11);default:0" json:"used_count"`
}

// TableName returns the table name
func (Coupon) TableName() string {
	return "v2_coupon"
}
