package model

// Ticket represents a support ticket
type Ticket struct {
	Model
	UserID  uint   `gorm:"type:int(11);index" json:"user_id"`
	Subject string `gorm:"type:varchar(256)" json:"subject"`
	Level   int    `gorm:"type:int(11);default:0" json:"level"`
	Status  int    `gorm:"type:int(11);default:0;index" json:"status"` // 0=open,1=closed

	// Relations
	User     *User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Messages []TicketMessage `gorm:"foreignKey:TicketID" json:"messages,omitempty"`
}

// TableName returns the table name
func (Ticket) TableName() string {
	return "v2_ticket"
}
