package model

// AdminAuditLog represents an admin action audit log
type AdminAuditLog struct {
	ID      uint   `gorm:"primarykey" json:"id"`
	UserID  uint   `gorm:"type:int(11);index" json:"user_id"`
	Action  string `gorm:"type:varchar(128);index" json:"action"`
	Data    string `gorm:"type:text" json:"data"`
	IP      string `gorm:"type:varchar(64)" json:"ip"`
	CreatedAt int64 `gorm:"type:bigint(20);autoCreateTime:milli" json:"created_at"`
}

// TableName returns the table name
func (AdminAuditLog) TableName() string {
	return "v2_admin_audit_log"
}
