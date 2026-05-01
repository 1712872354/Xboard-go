package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/xboard/xboard/internal/model"
	"gorm.io/gorm"
)

// CaptchaService handles captcha verification
type CaptchaService struct {
	db *gorm.DB
}

func NewCaptchaService(db *gorm.DB) *CaptchaService {
	return &CaptchaService{db: db}
}

// VerifyCaptcha verifies a captcha token against the configured provider
func (s *CaptchaService) VerifyCaptcha(token, clientIP string) error {
	// Check if captcha is enabled
	enabled := s.getSettingInt("captcha_enable", 0)
	if enabled == 0 {
		return nil // captcha not enabled
	}

	captchaType := s.getSetting("captcha_type", "recaptcha")
	if token == "" {
		return fmt.Errorf("请完成人机验证")
	}

	switch captchaType {
	case "turnstile":
		return s.verifyTurnstile(token, clientIP)
	case "recaptcha-v3":
		return s.verifyRecaptchaV3(token, clientIP)
	case "recaptcha":
		return s.verifyRecaptcha(token, clientIP)
	default:
		return fmt.Errorf("不支持的验证码类型: %s", captchaType)
	}
}

// verifyRecaptcha verifies Google reCAPTCHA v2 token
func (s *CaptchaService) verifyRecaptcha(token, clientIP string) error {
	secret := s.getSetting("recaptcha_secret_key", "")
	if secret == "" {
		return fmt.Errorf("reCAPTCHA密钥未配置")
	}

	resp, err := http.PostForm("https://www.recaptcha.net/recaptcha/api/siteverify", url.Values{
		"secret":   {secret},
		"response": {token},
		"remoteip": {clientIP},
	})
	if err != nil {
		return fmt.Errorf("验证码验证请求失败")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("验证码验证响应读取失败")
	}

	var result struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("验证码验证响应解析失败")
	}

	if !result.Success {
		return fmt.Errorf("人机验证未通过")
	}

	return nil
}

// verifyRecaptchaV3 verifies Google reCAPTCHA v3 token
func (s *CaptchaService) verifyRecaptchaV3(token, clientIP string) error {
	secret := s.getSetting("recaptcha_secret_key", "")
	if secret == "" {
		return fmt.Errorf("reCAPTCHA密钥未配置")
	}

	resp, err := http.PostForm("https://www.recaptcha.net/recaptcha/api/siteverify", url.Values{
		"secret":   {secret},
		"response": {token},
		"remoteip": {clientIP},
	})
	if err != nil {
		return fmt.Errorf("验证码验证请求失败")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("验证码验证响应读取失败")
	}

	var result struct {
		Success    bool     `json:"success"`
		Score      float64  `json:"score"`
		Action     string   `json:"action"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("验证码验证响应解析失败")
	}

	if !result.Success {
		return fmt.Errorf("人机验证未通过")
	}

	// Check score threshold (default 0.5)
	threshold := s.getSettingFloat("recaptcha_v3_threshold", 0.5)
	if result.Score < threshold {
		return fmt.Errorf("人机验证分数过低(%.2f < %.2f)", result.Score, threshold)
	}

	return nil
}

// verifyTurnstile verifies Cloudflare Turnstile token
func (s *CaptchaService) verifyTurnstile(token, clientIP string) error {
	secret := s.getSetting("turnstile_secret_key", "")
	if secret == "" {
		return fmt.Errorf("Turnstile密钥未配置")
	}

	resp, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", url.Values{
		"secret":   {secret},
		"response": {token},
		"remoteip": {clientIP},
	})
	if err != nil {
		return fmt.Errorf("验证码验证请求失败")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("验证码验证响应读取失败")
	}

	var result struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("验证码验证响应解析失败")
	}

	if !result.Success {
		return fmt.Errorf("人机验证未通过")
	}

	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (s *CaptchaService) getSetting(key, defaultVal string) string {
	var setting model.Setting
	if err := s.db.Where("`key` = ?", key).First(&setting).Error; err != nil {
		return defaultVal
	}
	return setting.Value
}

func (s *CaptchaService) getSettingInt(key string, defaultVal int) int {
	val := s.getSetting(key, "")
	if val == "" {
		return defaultVal
	}
	var result int
	fmt.Sscanf(val, "%d", &result)
	if result == 0 && val != "0" {
		return defaultVal
	}
	return result
}

func (s *CaptchaService) getSettingFloat(key string, defaultVal float64) float64 {
	val := s.getSetting(key, "")
	if val == "" {
		return defaultVal
	}
	var result float64
	fmt.Sscanf(val, "%f", &result)
	if result == 0 && val != "0" && !strings.HasPrefix(val, "0.") {
		return defaultVal
	}
	return result
}

// Verify is an alias for VerifyCaptcha
func (s *CaptchaService) Verify(token, clientIP string) error {
	return s.VerifyCaptcha(token, clientIP)
}
