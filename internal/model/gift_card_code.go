package model

import "time"

// GiftCardCode represents a specific gift card code generated from a template
type GiftCardCode struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	TemplateID uint      `gorm:"type:int(11);index" json:"template_id"`
	Code       string    `gorm:"type:varchar(64);uniqueIndex" json:"code"`
	Status     int       `gorm:"type:int(11);default:0" json:"status"`
	UsedAt     time.Time `gorm:"type:datetime" json:"used_at"`
	UsedBy     uint      `gorm:"type:int(11);default:0" json:"used_by"`
	CreatedAt  time.Time `json:"created_at"`

	// Relations
	Template *GiftCardTemplate `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
	Usages   []GiftCardUsage   `gorm:"foreignKey:GiftCardID" json:"usages,omitempty"`
}

// TableName returns the table name
func (GiftCardCode) TableName() string {
	return "v2_gift_card_code"
}
