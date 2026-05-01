package model

// InviteCode represents an invitation code
type InviteCode struct {
	Model
	UserID uint   `gorm:"type:int(11);index" json:"user_id"`
	Code   string `gorm:"type:varchar(64);uniqueIndex" json:"code"`
	Status int    `gorm:"type:int(11);default:0" json:"status"`

	// Relations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName returns the table name
func (InviteCode) TableName() string {
	return "v2_invite_code"
}
