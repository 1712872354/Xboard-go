package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/xboard/xboard/internal/model"
	"gorm.io/gorm"
)

// UserService handles user business logic
type UserService struct {
	db     *gorm.DB
	setting *SettingService
}

func NewUserService(db *gorm.DB, setting *SettingService) *UserService {
	return &UserService{db: db, setting: setting}
}

// CreateUser creates a new user account
func (s *UserService) CreateUser(email, password string, opts ...UserOption) (*model.User, error) {
	if email == "" || password == "" {
		return nil, errors.New("email and password are required")
	}

	// Check existing
	var count int64
	s.db.Model(&model.User{}).Where("email = ?", email).Count(&count)
	if count > 0 {
		return nil, errors.New("email already exists")
	}

	user := &model.User{
		Email:    email,
		Password: password, // will be hashed by hook
		Token:    generateUUID(),
		UUID:     generateUUID(),
	}

	for _, opt := range opts {
		opt(user)
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("create user failed: %w", err)
	}

	return user, nil
}

type UserOption func(*model.User)

func WithPlan(planID uint) UserOption {
	return func(u *model.User) { u.PlanID = planID }
}

func WithInviteUser(inviteUserID uint) UserOption {
	return func(u *model.User) { u.InviteUserID = inviteUserID }
}

func WithBalance(balance float64) UserOption {
	return func(u *model.User) { u.Balance = balance }
}

func WithTransferEnable(bytes int64) UserOption {
	return func(u *model.User) { u.TransferEnable = bytes }
}

func WithExpiredAt(t time.Time) UserOption {
	return func(u *model.User) { u.ExpiredAt = t }
}

// AddBalance adds balance to user account (with pessimistic lock)
func (s *UserService) AddBalance(userID uint, amount float64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, userID).Error; err != nil {
			return err
		}
		if user.Balance+amount < 0 {
			return errors.New("insufficient balance")
		}
		return tx.Model(&user).Update("balance", gorm.Expr("balance + ?", amount)).Error
	})
}

// GetUserByEmail retrieves user by email
func (s *UserService) GetUserByEmail(email string) (*model.User, error) {
	var user model.User
	err := s.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// GetUserByToken retrieves user by subscription token
func (s *UserService) GetUserByToken(token string) (*model.User, error) {
	var user model.User
	err := s.db.Where("token = ?", token).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// BatchCreateUsers creates multiple users in a transaction
func (s *UserService) BatchCreateUsers(users []*model.User) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, user := range users {
			if err := tx.Create(user).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateUser updates user fields
func (s *UserService) UpdateUser(userID uint, updates map[string]interface{}) error {
	return s.db.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error
}

// GetUsersByPlan returns users subscribed to a specific plan
func (s *UserService) GetUsersByPlan(planID uint) ([]model.User, error) {
	var users []model.User
	err := s.db.Where("plan_id = ? AND expired_at > ?", planID, time.Now().Unix()).Find(&users).Error
	return users, err
}

// CountUsers counts total users
func (s *UserService) CountUsers() (int64, error) {
	var count int64
	err := s.db.Model(&model.User{}).Count(&count).Error
	return count, err
}

// generateUUID generates a UUID string
func generateUUID() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(i + 1)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
