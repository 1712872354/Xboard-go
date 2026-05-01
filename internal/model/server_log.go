package model

import "time"

// ServerLog represents a server usage log entry
type ServerLog struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	ServerID   uint      `gorm:"type:int(11);index" json:"server_id"`
	UserID     uint      `gorm:"type:int(11);index" json:"user_id"`
	U          int64     `gorm:"type:bigint(20);default:0" json:"u"`
	D          int64     `gorm:"type:bigint(20);default:0" json:"d"`
	Rate       string    `gorm:"type:varchar(16);default:1" json:"rate"`
	Method     string    `gorm:"type:varchar(32)" json:"method"`
	RecordedAt time.Time `gorm:"type:datetime;index" json:"recorded_at"`
}

// TableName returns the table name
func (ServerLog) TableName() string {
	return "v2_server_log"
}
