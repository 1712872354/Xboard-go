package model

import "time"

// Order represents a purchase order
type Order struct {
	Model
	UserID          uint      `gorm:"type:int(11);index" json:"user_id"`
	PlanID          uint      `gorm:"type:int(11);index" json:"plan_id"`
	PaymentID       uint      `gorm:"type:int(11);default:0" json:"payment_id"`
	InviteUserID    uint      `gorm:"type:int(11);default:0" json:"invite_user_id"`
	Type            int       `gorm:"type:int(11);default:0" json:"type"`
	Cycle           string    `gorm:"type:varchar(32)" json:"cycle"`
	PeriodStart     time.Time `gorm:"type:datetime" json:"period_start"`
	PeriodEnd       time.Time `gorm:"type:datetime" json:"period_end"`
	TradeNo         string    `gorm:"type:varchar(64);uniqueIndex" json:"trade_no"`
	Status          int       `gorm:"type:int(11);default:0;index" json:"status"` // 1=pending,2=active,3=canceled,4=completed
	CallbackNo      string    `gorm:"type:varchar(64)" json:"callback_no"`
	TotalAmount     float64   `gorm:"type:decimal(10,2);default:0" json:"total_amount"`
	DiscountAmount  float64   `gorm:"type:decimal(10,2);default:0" json:"discount_amount"`
	SurplusAmount   float64   `gorm:"type:decimal(10,2);default:0" json:"surplus_amount"`
	SurplusMethod   string    `gorm:"type:varchar(32)" json:"surplus_method"`
	Rebate          int       `gorm:"type:tinyint(1);default:0" json:"rebate"`
	CommissionStatus int      `gorm:"type:int(11);default:0" json:"commission_status"`
	PaidAt          time.Time `gorm:"type:datetime" json:"paid_at"`

	// Relations
	User    *User    `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Plan    *Plan    `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
	Payment *Payment `gorm:"foreignKey:PaymentID" json:"payment,omitempty"`
}

// TableName returns the table name
func (Order) TableName() string {
	return "v2_order"
}
