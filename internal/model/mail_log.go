package model

import "time"

// MailLog represents an email send log
type MailLog struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	Email        string    `gorm:"type:varchar(128);index" json:"email"`
	Subject      string    `gorm:"type:varchar(256)" json:"subject"`
	TemplateName string    `gorm:"type:varchar(128)" json:"template_name"`
	Status       int       `gorm:"type:int(11);default:0" json:"status"`
	Error        string    `gorm:"type:text" json:"error"`
	CreatedAt    time.Time `json:"created_at"`
}

// TableName returns the table name
func (MailLog) TableName() string {
	return "v2_mail_log"
}
