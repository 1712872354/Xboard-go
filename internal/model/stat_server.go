package model

import "time"

// StatServer represents per-server statistics
type StatServer struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	ServerID   uint      `gorm:"type:int(11);index" json:"server_id"`
	U          int64     `gorm:"type:bigint(20);default:0" json:"u"`
	D          int64     `gorm:"type:bigint(20);default:0" json:"d"`
	RecordedAt time.Time `gorm:"type:datetime;index" json:"recorded_at"`
}

// TableName returns the table name
func (StatServer) TableName() string {
	return "v2_stat_server"
}
