package service

import (
	"context"
	"fmt"
	"time"

	"github.com/xboard/xboard/internal/database"
	"github.com/xboard/xboard/internal/model"
	"gorm.io/gorm"
)

// LoginSecurityService handles login security (password error limits, IP rate limiting)
type LoginSecurityService struct {
	db *gorm.DB
}

func NewLoginSecurityService(db *gorm.DB) *LoginSecurityService {
	return &LoginSecurityService{db: db}
}

// CheckPasswordLimit checks if the email has exceeded password error limit
// Returns error if locked out
func (s *LoginSecurityService) CheckPasswordLimit(email string) error {
	enabled := s.getSettingInt("password_limit_enable", 1)
	if enabled == 0 {
		return nil
	}

	rdb := database.Redis
	if rdb == nil {
		return nil // no Redis, skip check
	}

	key := fmt.Sprintf("password_error_limit:%s", email)
	count, err := rdb.Get(Ctx(), key).Int()
	if err != nil {
		return nil // key doesn't exist
	}

	limit := s.getSettingInt("password_limit_count", 5)
	if count >= limit {
		expireMinutes := s.getSettingInt("password_limit_expire", 60)
		return fmt.Errorf("密码错误次数过多，请在 %d 分钟后重试", expireMinutes)
	}

	return nil
}

// RecordPasswordError increments the password error count for an email
func (s *LoginSecurityService) RecordPasswordError(email string) {
	rdb := database.Redis
	if rdb == nil {
		return
	}

	key := fmt.Sprintf("password_error_limit:%s", email)
	expireMinutes := s.getSettingInt("password_limit_expire", 60)

	rdb.Incr(Ctx(), key)
	rdb.Expire(Ctx(), key, time.Duration(expireMinutes)*time.Minute)
}

// ClearPasswordErrors clears the password error count (on successful login)
func (s *LoginSecurityService) ClearPasswordErrors(email string) {
	rdb := database.Redis
	if rdb == nil {
		return
	}

	key := fmt.Sprintf("password_error_limit:%s", email)
	rdb.Del(Ctx(), key)
}

// CheckRegisterIPLimit checks if the IP has exceeded registration limit
func (s *LoginSecurityService) CheckRegisterIPLimit(ip string) error {
	enabled := s.getSettingInt("register_limit_by_ip_enable", 0)
	if enabled == 0 {
		return nil
	}

	rdb := database.Redis
	if rdb == nil {
		return nil
	}

	key := fmt.Sprintf("register_ip_rate_limit:%s", ip)
	count, err := rdb.Get(Ctx(), key).Int()
	if err != nil {
		return nil
	}

	limit := s.getSettingInt("register_limit_count", 3)
	if count >= limit {
		expireMinutes := s.getSettingInt("register_limit_expire", 60)
		return fmt.Errorf("注册过于频繁，请在 %d 分钟后重试", expireMinutes)
	}

	return nil
}

// RecordRegisterIP increments the registration count for an IP
func (s *LoginSecurityService) RecordRegisterIP(ip string) {
	rdb := database.Redis
	if rdb == nil {
		return
	}

	key := fmt.Sprintf("register_ip_rate_limit:%s", ip)
	expireMinutes := s.getSettingInt("register_limit_expire", 60)

	rdb.Incr(Ctx(), key)
	rdb.Expire(Ctx(), key, time.Duration(expireMinutes)*time.Minute)
}

// IsLoginLocked checks if email login is locked (alias for CheckPasswordLimit)
func (s *LoginSecurityService) IsLoginLocked(email string) (bool, string) {
	err := s.CheckPasswordLimit(email)
	if err != nil {
		return true, err.Error()
	}
	return false, ""
}

// RecordFailedLogin records a failed login attempt (alias for RecordPasswordError)
func (s *LoginSecurityService) RecordFailedLogin(email string) {
	s.RecordPasswordError(email)
}

// ClearFailedLogins clears failed login count (alias for ClearPasswordErrors)
func (s *LoginSecurityService) ClearFailedLogins(email string) {
	s.ClearPasswordErrors(email)
}

// CheckRegisterRate checks if IP registration is rate limited
func (s *LoginSecurityService) CheckRegisterRate(ip string) (bool, string) {
	if err := s.CheckRegisterIPLimit(ip); err != nil {
		return false, err.Error()
	}
	return true, ""
}

// RecordRegister records a registration attempt
func (s *LoginSecurityService) RecordRegister(ip string) {
	s.RecordRegisterIP(ip)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (s *LoginSecurityService) getSettingInt(key string, defaultVal int) int {
	var setting model.Setting
	if err := s.db.Where("`key` = ?", key).First(&setting).Error; err != nil {
		return defaultVal
	}
	var result int
	fmt.Sscanf(setting.Value, "%d", &result)
	if result == 0 && setting.Value != "0" {
		return defaultVal
	}
	return result
}

// Ctx returns a background context for Redis operations
func Ctx() context.Context {
	return context.Background()
}
