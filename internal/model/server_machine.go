package model

import "time"

// ServerMachine represents a physical or virtual machine hosting servers
type ServerMachine struct {
	Model
	Name       string     `gorm:"type:varchar(128)" json:"name"`
	Remark     string     `gorm:"type:varchar(256)" json:"remark"`
	Parent     uint       `gorm:"type:int(11);default:0" json:"parent"`
	Sort       int        `gorm:"type:int(11);default:0" json:"sort"`
	Token      string     `gorm:"type:varchar(64);uniqueIndex" json:"token"`
	IsActive   int        `gorm:"type:tinyint(1);default:0" json:"is_active"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
	LoadStatus string    `gorm:"type:varchar(32);default:''" json:"load_status"`
}

// TableName returns the table name
func (ServerMachine) TableName() string {
	return "v2_server_machine"
}
