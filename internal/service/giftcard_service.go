package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/xboard/xboard/internal/model"
	"gorm.io/gorm"
)

// GiftCardService handles gift card template, code, and usage management
type GiftCardService struct {
	db *gorm.DB
}

func NewGiftCardService(db *gorm.DB) *GiftCardService {
	return &GiftCardService{db: db}
}

// ---------- GiftCardTemplate ----------

// CreateTemplate creates a new gift card template
func (s *GiftCardService) CreateTemplate(template *model.GiftCardTemplate) error {
	if template.Name == "" {
		return errors.New("template name is required")
	}
	if template.Value <= 0 {
		return errors.New("template value must be positive")
	}
	return s.db.Create(template).Error
}

// GetTemplate retrieves a gift card template by ID
func (s *GiftCardService) GetTemplate(id uint) (*model.GiftCardTemplate, error) {
	var template model.GiftCardTemplate
	err := s.db.First(&template, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("template not found")
		}
		return nil, err
	}
	return &template, nil
}

// ListTemplates returns all gift card templates
func (s *GiftCardService) ListTemplates() ([]model.GiftCardTemplate, error) {
	var templates []model.GiftCardTemplate
	err := s.db.Order("id DESC").Find(&templates).Error
	return templates, err
}

// UpdateTemplate updates a gift card template
func (s *GiftCardService) UpdateTemplate(id uint, updates map[string]interface{}) error {
	return s.db.Model(&model.GiftCardTemplate{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteTemplate deletes a gift card template (and its codes)
func (s *GiftCardService) DeleteTemplate(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Delete associated codes
		if err := tx.Where("template_id = ?", id).Delete(&model.GiftCardCode{}).Error; err != nil {
			return err
		}
		// Delete the template
		return tx.Delete(&model.GiftCardTemplate{}, id).Error
	})
}

// ---------- GiftCardCode ----------

// GenerateCodes generates gift card codes from a template
func (s *GiftCardService) GenerateCodes(templateID uint, count int) ([]model.GiftCardCode, error) {
	template, err := s.GetTemplate(templateID)
	if err != nil {
		return nil, err
	}

	var codes []model.GiftCardCode
	for i := 0; i < count; i++ {
		code, err := s.generateCode()
		if err != nil {
			return nil, fmt.Errorf("generate code failed: %w", err)
		}
		codes = append(codes, model.GiftCardCode{
			TemplateID: template.ID,
			Code:       code,
			Status:     0,
		})
	}

	if err := s.db.Create(&codes).Error; err != nil {
		return nil, err
	}

	// Update template count
	s.db.Model(&template).Update("count", gorm.Expr("COALESCE(count, 0) + ?", count))

	return codes, nil
}

// GetCode retrieves a gift card code by its code string
func (s *GiftCardService) GetCode(code string) (*model.GiftCardCode, error) {
	var gc model.GiftCardCode
	err := s.db.Where("code = ?", code).First(&gc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("gift card not found")
		}
		return nil, err
	}
	return &gc, nil
}

// ListCodesByTemplate returns all codes for a given template
func (s *GiftCardService) ListCodesByTemplate(templateID uint) ([]model.GiftCardCode, error) {
	var codes []model.GiftCardCode
	err := s.db.Where("template_id = ?", templateID).Order("id DESC").Find(&codes).Error
	return codes, err
}

// ListAllCodes returns all gift card codes
func (s *GiftCardService) ListAllCodes() ([]model.GiftCardCode, error) {
	var codes []model.GiftCardCode
	err := s.db.Preload("Template").Order("id DESC").Find(&codes).Error
	return codes, err
}

// RedeemCode redeems a gift card code for a user
func (s *GiftCardService) RedeemCode(code string, userID uint) (*model.GiftCardCode, error) {
	gc, err := s.GetCode(code)
	if err != nil {
		return nil, err
	}

	if gc.Status != 0 {
		return nil, errors.New("gift card already used")
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Mark code as used
		if err := tx.Model(&gc).Updates(map[string]interface{}{
			"status":  1,
			"used_by": userID,
			"used_at": time.Now(),
		}).Error; err != nil {
			return err
		}

		// Add balance to user
		template, err := s.GetTemplate(gc.TemplateID)
		if err != nil {
			return err
		}

		if err := tx.Model(&model.User{}).Where("id = ?", userID).
			Update("balance", gorm.Expr("balance + ?", template.Value)).Error; err != nil {
			return err
		}

		// Increment template usage counter
		return tx.Model(&model.GiftCardTemplate{}).Where("id = ?", gc.TemplateID).
			Update("total_use", gorm.Expr("total_use + 1")).Error
	})

	if err != nil {
		return nil, err
	}

	return gc, nil
}

// DeleteCode soft-deletes a gift card code
func (s *GiftCardService) DeleteCode(id uint) error {
	return s.db.Delete(&model.GiftCardCode{}, id).Error
}

// generateCode creates a random alphanumeric gift card code
func (s *GiftCardService) generateCode() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const codeLen = 16

	code := make([]byte, codeLen)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		code[i] = charset[n.Int64()]
	}

	// Format as XXXX-XXXX-XXXX-XXXX
	result := make([]byte, 0, 19)
	for i, c := range code {
		if i > 0 && i%4 == 0 {
			result = append(result, '-')
		}
		result = append(result, c)
	}

	return string(result), nil
}

// ---------- GiftCardUsage ----------

// ListUsageByCode returns usage records for a specific gift card
func (s *GiftCardService) ListUsageByCode(giftCardID uint) ([]model.GiftCardUsage, error) {
	var usages []model.GiftCardUsage
	err := s.db.Where("gift_card_id = ?", giftCardID).
		Preload("GiftCard").Preload("Order").
		Order("id DESC").Find(&usages).Error
	return usages, err
}

// RecordUsage records the use of a gift card on an order
func (s *GiftCardService) RecordUsage(giftCardID, orderID uint, value float64) error {
	usage := &model.GiftCardUsage{
		GiftCardID: giftCardID,
		OrderID:    orderID,
		Value:      value,
	}
	return s.db.Create(usage).Error
}
