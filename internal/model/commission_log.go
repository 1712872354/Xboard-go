package model

// CommissionLog represents a commission earning record
type CommissionLog struct {
	Model
	InviteUserID uint    `gorm:"type:int(11);index" json:"invite_user_id"`
	OrderID      uint    `gorm:"type:int(11);index" json:"order_id"`
	GetAmount    float64 `gorm:"type:decimal(10,2);default:0" json:"get_amount"`
	GiveAmount   float64 `gorm:"type:decimal(10,2);default:0" json:"give_amount"`
	Status       int     `gorm:"type:int(11);default:0" json:"status"`

	// Relations
	InviteUser *User  `gorm:"foreignKey:InviteUserID" json:"invite_user,omitempty"`
	Order      *Order `gorm:"foreignKey:OrderID" json:"order,omitempty"`
}

// TableName returns the table name
func (CommissionLog) TableName() string {
	return "v2_commission_log"
}
