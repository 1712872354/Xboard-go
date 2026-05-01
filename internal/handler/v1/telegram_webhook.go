package v1

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/pkg/response"
	"gorm.io/gorm"
)

// TelegramWebhookHandler handles Telegram Bot Webhook
type TelegramWebhookHandler struct {
	db        *gorm.DB
	botToken  string
	adminChat int64
}

func NewTelegramWebhookHandler(db *gorm.DB, botToken string, adminChat int64) *TelegramWebhookHandler {
	return &TelegramWebhookHandler{
		db:        db,
		botToken:  botToken,
		adminChat: adminChat,
	}
}

// Handle receives Telegram Webhook updates
func (h *TelegramWebhookHandler) Handle(c *gin.Context) {
	var update struct {
		UpdateID int `json:"update_id"`
		Message  struct {
			MessageID int `json:"message_id"`
			From      struct {
				ID        int64  `json:"id"`
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
				Username  string `json:"username"`
			} `json:"from"`
			Chat struct {
				ID   int64  `json:"id"`
				Type string `json:"type"`
			} `json:"chat"`
			Text string `json:"text"`
		} `json:"message"`
	}

	if err := c.ShouldBindJSON(&update); err != nil {
		response.BadRequest(c, "invalid update")
		return
	}

	// Verify bot token
	token := c.Query("token")
	if token != h.botToken {
		response.Unauthorized(c, "invalid bot token")
		return
	}

	chatID := update.Message.Chat.ID
	text := strings.TrimSpace(update.Message.Text)

	// Parse command
	if strings.HasPrefix(text, "/") {
		parts := strings.Fields(text)
		cmd := parts[0]
		args := parts[1:]

		switch {
		case cmd == "/start":
			h.handleStart(c, chatID, args)
		case cmd == "/bind":
			h.handleBind(c, chatID, args)
		case cmd == "/unbind":
			h.handleUnbind(c, chatID)
		case cmd == "/sub":
			h.handleSub(c, chatID)
		case cmd == "/traffic":
			h.handleTraffic(c, chatID)
		case cmd == "/help":
			h.handleHelp(c, chatID)
		default:
			h.sendMessage(c, chatID, "未知命令，发送 /help 查看帮助")
		}
	} else {
		h.sendMessage(c, chatID, "请使用命令与我交互，发送 /help 查看帮助")
	}
}

// handleStart handles /start command
func (h *TelegramWebhookHandler) handleStart(c *gin.Context, chatID int64, args []string) {
	if len(args) > 0 {
		// Try to bind with code
		code := args[0]
		var user model.User
		if err := h.db.Where("telegram_verify_code = ?", code).First(&user).Error; err == nil {
			// Bind user
			h.db.Model(&user).Updates(map[string]interface{}{
				"telegram_id":         chatID,
				"telegram_verify_code": "",
			})
			h.sendMessage(c, chatID, fmt.Sprintf("✅ 绑定成功！\n用户: %s", user.Email))
			return
		}
	}
	h.sendMessage(c, chatID, "欢迎使用 XBoard Bot！\n\n可用命令:\n/bind <code> - 绑定账号\n/unbind - 解绑账号\n/sub - 获取订阅链接\n/traffic - 查看流量\n/help - 查看帮助")
}

// handleBind handles /bind command
func (h *TelegramWebhookHandler) handleBind(c *gin.Context, chatID int64, args []string) {
	if len(args) == 0 {
		h.sendMessage(c, chatID, "用法: /bind <绑定码>\n请在网站上获取绑定码")
		return
	}

	code := args[0]
	var user model.User
	if err := h.db.Where("telegram_verify_code = ?", code).First(&user).Error; err != nil {
		h.sendMessage(c, chatID, "❌ 绑定码无效或已过期")
		return
	}

	// Check if already bound
	var existing model.User
	if h.db.Where("telegram_id = ? AND id != ?", chatID, user.ID).First(&existing).Error == nil {
		h.sendMessage(c, chatID, "❌ 此Telegram账号已被其他用户绑定")
		return
	}

	// Bind
	h.db.Model(&user).Updates(map[string]interface{}{
		"telegram_id":         chatID,
		"telegram_verify_code": "",
	})
	h.sendMessage(c, chatID, fmt.Sprintf("✅ 绑定成功！\n用户: %s", user.Email))
}

// handleUnbind handles /unbind command
func (h *TelegramWebhookHandler) handleUnbind(c *gin.Context, chatID int64) {
	var user model.User
	if err := h.db.Where("telegram_id = ?", chatID).First(&user).Error; err != nil {
		h.sendMessage(c, chatID, "❌ 您还未绑定账号")
		return
	}

	h.db.Model(&user).Update("telegram_id", 0)
	h.sendMessage(c, chatID, "✅ 已解绑")
}

// handleSub handles /sub command
func (h *TelegramWebhookHandler) handleSub(c *gin.Context, chatID int64) {
	var user model.User
	if err := h.db.Where("telegram_id = ?", chatID).First(&user).Error; err != nil {
		h.sendMessage(c, chatID, "❌ 请先绑定账号 /bind <code>")
		return
	}

	// Get app URL from settings
	var setting model.Setting
	h.db.Where("`key` = ?", "app_url").First(&setting)
	subBaseURL := setting.Value
	if subBaseURL == "" {
		subBaseURL = "http://localhost"
	}

	subURL := fmt.Sprintf("%s/api/v1/client/subscribe?token=%s", subBaseURL, user.Token)
	h.sendMessage(c, chatID, fmt.Sprintf("📄 订阅链接:\n%s", subURL))
}

// handleTraffic handles /traffic command
func (h *TelegramWebhookHandler) handleTraffic(c *gin.Context, chatID int64) {
	var user model.User
	if err := h.db.Where("telegram_id = ?", chatID).First(&user).Error; err != nil {
		h.sendMessage(c, chatID, "❌ 请先绑定账号 /bind <code>")
		return
	}

	usedGB := float64(user.U+user.D) / 1024 / 1024 / 1024
	totalGB := float64(user.TransferEnable) / 1024 / 1024 / 1024
	percent := 0.0
	if totalGB > 0 {
		percent = usedGB / totalGB * 100
	}

	// Format expired time
	expiredStr := "已过期"
	if user.ExpiredAt.Unix() > 0 {
		expiredStr = user.ExpiredAt.Format("2006-01-02")
	}

	msg := fmt.Sprintf(`📊 流量统计

用户: %s
已用: %.2f GB / %.2f GB (%.1f%%)
到期: %s`,
		user.Email, usedGB, totalGB, percent, expiredStr)

	h.sendMessage(c, chatID, msg)
}

// handleHelp handles /help command
func (h *TelegramWebhookHandler) handleHelp(c *gin.Context, chatID int64) {
	help := `📖 XBoard Bot 帮助

/start - 开始使用
/bind <code> - 绑定账号
/unbind - 解绑账号
/sub - 获取订阅链接
/traffic - 查看流量统计
/help - 查看帮助`
	h.sendMessage(c, chatID, help)
}

// sendMessage sends a text message to a Telegram chat
func (h *TelegramWebhookHandler) sendMessage(c *gin.Context, chatID int64, text string) {
	// Use Telegram Bot API to send message
	_ = fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", h.botToken)

	// For now, just log the message
	c.JSON(http.StatusOK, gin.H{
		"method":     "sendMessage",
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	})
}

// Suppress unused warning
var _ = strconv.Itoa
