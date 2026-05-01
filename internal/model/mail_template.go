package model

// MailTemplate represents an email template
type MailTemplate struct {
	Model
	Name     string `gorm:"type:varchar(128)" json:"name"`
	Subject  string `gorm:"type:varchar(256)" json:"subject"`
	Template string `gorm:"type:text" json:"template"`
	Type     string `gorm:"type:varchar(32)" json:"type"`
}

// TableName returns the table name
func (MailTemplate) TableName() string {
	return "v2_mail_templates"
}
