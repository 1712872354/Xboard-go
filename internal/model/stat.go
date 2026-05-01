package model

import "time"

// Stat represents a daily statistics record
type Stat struct {
	ID               uint      `gorm:"primarykey" json:"id"`
	RegisterCount    int       `gorm:"type:int(11);default:0" json:"register_count"`
	TradeCount       int       `gorm:"type:int(11);default:0" json:"trade_count"`
	TradeAmount      float64   `gorm:"type:decimal(10,2);default:0" json:"trade_amount"`
	CommissionCount  int       `gorm:"type:int(11);default:0" json:"commission_count"`
	CommissionAmount float64   `gorm:"type:decimal(10,2);default:0" json:"commission_amount"`
	PaidUserCount    int       `gorm:"type:int(11);default:0" json:"paid_user_count"`
	RecordedAt       time.Time `gorm:"type:datetime;index" json:"recorded_at"`
}

// TableName returns the table name
func (Stat) TableName() string {
	return "v2_stat"
}
