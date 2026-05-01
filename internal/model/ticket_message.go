package model

// TicketMessage represents a message within a support ticket
type TicketMessage struct {
	Model
	TicketID uint   `gorm:"type:int(11);index" json:"ticket_id"`
	UserID   uint   `gorm:"type:int(11);index" json:"user_id"`
	Message  string `gorm:"type:text" json:"message"`

	// Relations
	Ticket *Ticket `gorm:"foreignKey:TicketID" json:"ticket,omitempty"`
	User   *User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName returns the table name
func (TicketMessage) TableName() string {
	return "v2_ticket_message"
}
