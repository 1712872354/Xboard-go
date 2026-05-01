package model

// GiftCardUsage represents a usage record of a gift card on an order
type GiftCardUsage struct {
	Model
	GiftCardID uint    `gorm:"type:int(11);index" json:"gift_card_id"`
	OrderID    uint    `gorm:"type:int(11);index" json:"order_id"`
	Value      float64 `gorm:"type:decimal(10,2);default:0" json:"value"`

	// Relations
	GiftCard *GiftCardCode `gorm:"foreignKey:GiftCardID" json:"gift_card,omitempty"`
	Order    *Order        `gorm:"foreignKey:OrderID" json:"order,omitempty"`
}

// TableName returns the table name
func (GiftCardUsage) TableName() string {
	return "v2_gift_card_usage"
}
