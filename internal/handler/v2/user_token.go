package v2

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/pkg/response"
	"gorm.io/gorm"
)

// UserTokenHandler handles API Token management
type UserTokenHandler struct {
	db *gorm.DB
}

func NewUserTokenHandler(db *gorm.DB) *UserTokenHandler {
	return &UserTokenHandler{db: db}
}

// List returns all API tokens for the current user
func (h *UserTokenHandler) List(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	var tokens []model.APIToken
	h.db.Where("user_id = ?", uid).Order("id DESC").Find(&tokens)
	response.Success(c, tokens)
}

// Create creates a new API token
func (h *UserTokenHandler) Create(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	var req struct {
		Name      string   `json:"name" binding:"required"`
		Abilities []string `json:"abilities"`
		ExpiresIn int      `json:"expires_in"` // hours, 0 = never
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// Validate abilities
	validAbilities := []string{"read", "write", "subscribe", "order", "ticket"}
	for _, a := range req.Abilities {
		valid := false
		for _, va := range validAbilities {
			if a == va {
				valid = true
				break
			}
		}
		if !valid {
			response.BadRequest(c, fmt.Sprintf("无效的权限: %s", a))
			return
		}
	}

	// Generate token
	plainToken := generateTokenString()
	tokenHash := sha256Hash(plainToken)

	// Calculate expiry
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		exp := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
		expiresAt = &exp
	}

	// Create token record
	token := &model.APIToken{
		UserID:    uid,
		Name:      req.Name,
		Token:     tokenHash,
		Abilities: strings.Join(req.Abilities, ","),
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	if err := h.db.Create(token).Error; err != nil {
		response.InternalError(c, "创建Token失败")
		return
	}

	response.Success(c, gin.H{
		"id":         token.ID,
		"name":       token.Name,
		"token":      plainToken, // Only returned once
		"abilities":  req.Abilities,
		"expires_at": expiresAt,
		"created_at": token.CreatedAt,
	})
}

// Abilities returns available token abilities
func (h *UserTokenHandler) Abilities(c *gin.Context) {
	response.Success(c, []gin.H{
		{"value": "read", "label": "读取"},
		{"value": "write", "label": "写入"},
		{"value": "subscribe", "label": "订阅"},
		{"value": "order", "label": "订单"},
		{"value": "ticket", "label": "工单"},
	})
}

// Delete removes a specific API token
func (h *UserTokenHandler) Delete(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	tokenID := c.Param("tokenId")
	var id uint
	fmt.Sscanf(tokenID, "%d", &id)
	if id == 0 {
		response.BadRequest(c, "无效的Token ID")
		return
	}

	result := h.db.Where("id = ? AND user_id = ?", id, uid).Delete(&model.APIToken{})
	if result.RowsAffected == 0 {
		response.NotFound(c, "Token不存在")
		return
	}

	response.Success(c, nil)
}

// DeleteAll removes all API tokens for the current user
func (h *UserTokenHandler) DeleteAll(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	h.db.Where("user_id = ?", uid).Delete(&model.APIToken{})
	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func generateTokenString() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "xboard_" + hex.EncodeToString(b)
}

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
