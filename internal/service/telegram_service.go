package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xboard/xboard/internal/model"
	"gorm.io/gorm"
)

// TelegramService handles Telegram Bot API notifications
type TelegramService struct {
	botToken string
	db       *gorm.DB
}

func NewTelegramService(botToken string, db *gorm.DB) *TelegramService {
	return &TelegramService{botToken: botToken, db: db}
}

type telegramMessage struct {
	ChatID      int64  `json:"chat_id"`
	Text        string `json:"text"`
	ParseMode   string `json:"parse_mode,omitempty"`
	DisableWebPreview bool `json:"disable_web_page_preview,omitempty"`
}

type telegramResponse struct {
	OK     bool   `json:"ok"`
	ErrorCode int    `json:"error_code,omitempty"`
	Description string `json:"description,omitempty"`
}

// SendMessage sends a text message to a specific chat via Telegram Bot API
func (s *TelegramService) SendMessage(chatID int64, message string) error {
	if s.botToken == "" {
		return fmt.Errorf("bot token not configured")
	}

	msg := telegramMessage{
		ChatID:    chatID,
		Text:      message,
		ParseMode: "HTML",
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message failed: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result telegramResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("parse response failed: %w", err)
	}

	if !result.OK {
		return fmt.Errorf("telegram api error: %s", result.Description)
	}

	return nil
}

// SendPaymentNotification notifies admin of a new payment
func (s *TelegramService) SendPaymentNotification(orderID uint, amount float64) error {
	var order model.Order
	if err := s.db.Preload("User").First(&order, orderID).Error; err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	message := fmt.Sprintf(
		"<b>💰 新支付通知</b>\n\n"+
			"订单编号: <code>%s</code>\n"+
			"金额: <b>¥%.2f</b>\n"+
			"用户: %s\n"+
			"时间: %s",
		order.TradeNo,
		amount,
		order.User.Email,
		time.Now().Format("2006-01-02 15:04:05"),
	)

	chatID, err := s.getAdminChatID()
	if err != nil {
		return err
	}

	return s.SendMessage(chatID, message)
}

// SendTicketNotification notifies admin of a new support ticket
func (s *TelegramService) SendTicketNotification(ticketID uint, subject string) error {
	var ticket model.Ticket
	if err := s.db.Preload("User").First(&ticket, ticketID).Error; err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}

	message := fmt.Sprintf(
		"<b>📝 新工单通知</b>\n\n"+
			"工单编号: <code>#%d</code>\n"+
			"主题: %s\n"+
			"用户: %s\n"+
			"时间: %s",
		ticket.ID,
		subject,
		ticket.User.Email,
		time.Now().Format("2006-01-02 15:04:05"),
	)

	chatID, err := s.getAdminChatID()
	if err != nil {
		return err
	}

	return s.SendMessage(chatID, message)
}

// getAdminChatID retrieves the admin Telegram chat ID from settings
func (s *TelegramService) getAdminChatID() (int64, error) {
	var setting model.Setting
	if err := s.db.Where("`key` = ?", "telegram_admin_chat_id").First(&setting).Error; err != nil {
		return 0, fmt.Errorf("admin chat id not configured")
	}

	var chatID int64
	if _, err := fmt.Sscanf(setting.Value, "%d", &chatID); err != nil {
		return 0, fmt.Errorf("invalid chat id: %s", setting.Value)
	}

	return chatID, nil
}
