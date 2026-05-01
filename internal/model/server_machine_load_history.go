package model

import "time"

// ServerMachineLoadHistory records historical load metrics of a machine
type ServerMachineLoadHistory struct {
	Model
	MachineID    uint    `gorm:"type:int(11);index;not null" json:"machine_id"`
	CPU          float64 `gorm:"type:decimal(5,2);default:0" json:"cpu"`
	MemTotal     int64   `gorm:"type:bigint(20);default:0" json:"mem_total"`
	MemUsed      int64   `gorm:"type:bigint(20);default:0" json:"mem_used"`
	DiskTotal    int64   `gorm:"type:bigint(20);default:0" json:"disk_total"`
	DiskUsed     int64   `gorm:"type:bigint(20);default:0" json:"disk_used"`
	NetInSpeed   int64   `gorm:"type:bigint(20);default:0" json:"net_in_speed"`
	NetOutSpeed  int64   `gorm:"type:bigint(20);default:0" json:"net_out_speed"`
	RecordedAt   time.Time `gorm:"not null" json:"recorded_at"`
}

// TableName returns the table name
func (ServerMachineLoadHistory) TableName() string {
	return "v2_server_machine_load_history"
}
