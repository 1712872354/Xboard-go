package model

// GiftCardTemplate represents a gift card template
type GiftCardTemplate struct {
	Model
	Name     string  `gorm:"type:varchar(128)" json:"name"`
	Type     int     `gorm:"type:int(11);default:0" json:"type"`
	Value    float64 `gorm:"type:decimal(10,2);default:0" json:"value"`
	Count    *int    `gorm:"type:int(11);default:0" json:"count"`
	TotalUse int     `gorm:"type:int(11);default:0" json:"total_use"`
}

// TableName returns the table name
func (GiftCardTemplate) TableName() string {
	return "v2_gift_card_template"
}
