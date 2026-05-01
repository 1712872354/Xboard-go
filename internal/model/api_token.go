package model

import (
	"strings"
	"time"
)

// APIToken represents a personal access token for API authentication
type APIToken struct {
	Model
	UserID     uint       `gorm:"type:int(11);index;not null" json:"user_id"`
	Name       string     `gorm:"type:varchar(255);not null" json:"name"`
	Token      string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"-"` // SHA256 hash
	Abilities  string     `gorm:"type:varchar(255);default:'*'" json:"abilities"`
	LastUsedAt *time.Time `gorm:"type:datetime" json:"last_used_at"`
	ExpiresAt  *time.Time `gorm:"type:datetime" json:"expires_at"`
	CreatedAt  time.Time  `gorm:"not null" json:"created_at"`

	// Relations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName returns the table name
func (APIToken) TableName() string {
	return "v2_user_tokens"
}

// Can checks if the token has a specific ability
func (t *APIToken) Can(ability string) bool {
	if t.Abilities == "*" {
		return true
	}
	abilities := strings.Split(t.Abilities, ",")
	for _, a := range abilities {
		if a == ability {
			return true
		}
	}
	return false
}

// IsExpired checks if the token has expired
func (t *APIToken) IsExpired() bool {
	if t.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*t.ExpiresAt)
}
