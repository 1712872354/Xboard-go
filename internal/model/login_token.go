package model

import "time"

// LoginTokenType defines the type of one-time login token
type LoginTokenType string

const (
	TokenTypeMagicLink  LoginTokenType = "magic_link"
	TokenTypePasswordReset LoginTokenType = "password_reset"
	TokenTypeEmailVerify LoginTokenType = "email_verify"
)

// LoginToken represents a one-time token for magic link login / password reset / email verification
type LoginToken struct {
	Model
	Token     string         `gorm:"type:varchar(128);uniqueIndex;not null" json:"token"`
	UserID    uint           `gorm:"type:int(11);index;not null" json:"user_id"`
	TokenType LoginTokenType `gorm:"type:varchar(32);index;not null" json:"token_type"`
	Email     string         `gorm:"type:varchar(128);not null" json:"email"`
	ExpiresAt time.Time      `gorm:"not null" json:"expires_at"`
	Used      bool           `gorm:"default:0" json:"used"`
	UsedAt    *time.Time     `json:"used_at,omitempty"`
}

// TableName returns the table name
func (LoginToken) TableName() string {
	return "v2_login_tokens"
}
