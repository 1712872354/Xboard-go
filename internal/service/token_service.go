package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/xboard/xboard/internal/model"
	"gorm.io/gorm"
)

// TokenService handles one-time tokens for magic link login, password reset, email verification
type TokenService struct {
	db *gorm.DB
}

func NewTokenService(db *gorm.DB) *TokenService {
	return &TokenService{db: db}
}

// GenerateToken creates a one-time token for the given type and user
func (s *TokenService) GenerateToken(userID uint, email string, tokenType model.LoginTokenType) (*model.LoginToken, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate token failed: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	loginToken := &model.LoginToken{
		Token:     token,
		UserID:    userID,
		TokenType: tokenType,
		Email:     email,
		ExpiresAt: time.Now().Add(30 * time.Minute), // 30 minutes expiry
	}

	if err := s.db.Create(loginToken).Error; err != nil {
		return nil, fmt.Errorf("save token failed: %w", err)
	}

	return loginToken, nil
}

// ValidateToken validates and consumes a one-time token
// Returns the token record if valid, error otherwise
func (s *TokenService) ValidateToken(token string, tokenType model.LoginTokenType) (*model.LoginToken, error) {
	var loginToken model.LoginToken
	if err := s.db.Where("token = ? AND token_type = ? AND used = 0 AND expires_at > ?",
		token, tokenType, time.Now()).First(&loginToken).Error; err != nil {
		return nil, fmt.Errorf("token无效或已过期")
	}

	// Mark as used (one-time use)
	now := time.Now()
	s.db.Model(&loginToken).Updates(map[string]interface{}{
		"used":    true,
		"used_at": &now,
	})

	return &loginToken, nil
}

// InvalidateUserTokens invalidates all unused tokens of a given type for a user
func (s *TokenService) InvalidateUserTokens(userID uint, tokenType model.LoginTokenType) error {
	return s.db.Model(&model.LoginToken{}).
		Where("user_id = ? AND token_type = ? AND used = 0", userID, tokenType).
		Update("used", true).Error
}

// GenerateRandomCode generates a numeric code of given length (for email verification)
func (s *TokenService) GenerateRandomCode(length int) string {
	codeBytes := make([]byte, length)
	rand.Read(codeBytes)
	code := ""
	for _, b := range codeBytes {
		code += fmt.Sprintf("%d", int(b)%10)
	}
	return code
}
