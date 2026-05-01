package model

// Payment represents a payment method
type Payment struct {
	Model
	UUID          string `gorm:"type:varchar(64);uniqueIndex" json:"uuid"`
	Payment       string `gorm:"type:varchar(64)" json:"payment"`
	Name          string `gorm:"type:varchar(128)" json:"name"`
	Icon          string `gorm:"type:varchar(256)" json:"icon"`
	Config        JSON   `gorm:"type:json" json:"config"`
	NotifyDomain  string `gorm:"type:varchar(256)" json:"notify_domain"`
	Handle        string `gorm:"type:varchar(64)" json:"handle"`
	Enable        int    `gorm:"type:tinyint(1);default:1" json:"enable"`
	Sort          int    `gorm:"type:int(11);default:0" json:"sort"`
}

// TableName returns the table name
func (Payment) TableName() string {
	return "v2_payment"
}
