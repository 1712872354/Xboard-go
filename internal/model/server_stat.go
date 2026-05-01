package model

import "time"

// ServerStat represents a server statistics record
type ServerStat struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	ServerID   uint      `gorm:"type:int(11);index" json:"server_id"`
	U          int64     `gorm:"type:bigint(20);default:0" json:"u"`
	D          int64     `gorm:"type:bigint(20);default:0" json:"d"`
	RecordType string    `gorm:"type:varchar(16);index" json:"record_type"`
	RecordedAt time.Time `gorm:"type:datetime;index" json:"recorded_at"`
}

// TableName returns the table name
func (ServerStat) TableName() string {
	return "v2_server_stat"
}
