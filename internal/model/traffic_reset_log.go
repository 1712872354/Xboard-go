package model

import "time"

// TrafficResetLog represents a traffic reset record
type TrafficResetLog struct {
	Model
	UserID     uint      `gorm:"type:int(11);index" json:"user_id"`
	RecordedAt time.Time `gorm:"type:datetime" json:"recorded_at"`
	Before     int64     `gorm:"type:bigint(20);default:0" json:"before"`
	After      int64     `gorm:"type:bigint(20);default:0" json:"after"`

	// Relations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName returns the table name
func (TrafficResetLog) TableName() string {
	return "v2_traffic_reset_log"
}
