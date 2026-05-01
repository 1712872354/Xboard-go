package model

import "time"

// StatUser represents per-user statistics
type StatUser struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	UserID     uint      `gorm:"type:int(11);index" json:"user_id"`
	U          int64     `gorm:"type:bigint(20);default:0" json:"u"`
	D          int64     `gorm:"type:bigint(20);default:0" json:"d"`
	RecordedAt time.Time `gorm:"type:datetime;index" json:"recorded_at"`

	// Relations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName returns the table name
func (StatUser) TableName() string {
	return "v2_stat_user"
}
