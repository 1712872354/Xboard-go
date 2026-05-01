package model

// ServerRoute represents a routing rule for servers
type ServerRoute struct {
	Model
	Match       JSON   `gorm:"type:json" json:"match"`
	Action      string `gorm:"type:varchar(64)" json:"action"`
	ActionValue string `gorm:"type:varchar(256)" json:"action_value"`
}

// TableName returns the table name
func (ServerRoute) TableName() string {
	return "v2_server_route"
}
