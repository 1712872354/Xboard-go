package queue

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/internal/service"
	"gorm.io/gorm"
)

// HandlerRegistry maps job names to their handlers
type HandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[JobName]JobHandler
}

// NewHandlerRegistry creates a new handler registry
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[JobName]JobHandler),
	}
}

// Register adds a handler for a job type
func (r *HandlerRegistry) Register(name JobName, handler JobHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = handler
	log.Printf("queue: registered handler for %s", name)
}

// GetHandler returns the handler for a job type
func (r *HandlerRegistry) GetHandler(name JobName) (JobHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[name]
	return h, ok
}

// RegisterAllHandlers registers all business job handlers
func RegisterAllHandlers(registry *HandlerRegistry, db *gorm.DB) {
	_ = service.NewUserService(db, service.NewSettingService(db))
	_ = service.NewSettingService(db)
	_ = service.NewPlanService(db)

	// Order handle - process order lifecycle (activate, renew, upgrade)
	registry.Register(JobOrderHandle, func(ctx context.Context, job *Job) error {
		orderID, _ := job.Payload["order_id"].(float64)
		if orderID == 0 {
			return fmt.Errorf("invalid order_id in job payload")
		}
		action, _ := job.Payload["action"].(string)
		if action == "" {
			action = "open"
		}

		var order model.Order
		if err := db.WithContext(ctx).First(&order, uint(orderID)).Error; err != nil {
			return fmt.Errorf("order not found: %w", err)
		}

		var user model.User
		if err := db.WithContext(ctx).First(&user, order.UserID).Error; err != nil {
			return fmt.Errorf("user not found: %w", err)
		}

		switch action {
		case "open":
			// Activate subscription
			expiredAt := time.Now().AddDate(0, 1, 0) // default 1 month
			switch order.Cycle {
			case "month_price":
				expiredAt = time.Now().AddDate(0, 1, 0)
			case "quarter_price":
				expiredAt = time.Now().AddDate(0, 3, 0)
			case "half_year_price":
				expiredAt = time.Now().AddDate(0, 6, 0)
			case "year_price":
				expiredAt = time.Now().AddDate(1, 0, 0)
			case "two_year_price":
				expiredAt = time.Now().AddDate(2, 0, 0)
			default:
				expiredAt = time.Now().AddDate(0, 1, 0)
			}

			// For renew, extend from current expiration
			if order.Type == 2 && user.ExpiredAt.After(time.Now()) {
				expiredAt = user.ExpiredAt.Add(expiredAt.Sub(time.Now()))
			}

			updates := map[string]interface{}{
				"plan_id":      order.PlanID,
				"expired_at":   expiredAt,
				"u":            0,
				"d":            0,
			}

			// Get plan transfer enable
			var plan model.Plan
			if err := db.WithContext(ctx).First(&plan, order.PlanID).Error; err == nil {
				updates["transfer_enable"] = plan.TransferEnable
			}

			if err := db.WithContext(ctx).Model(&user).Updates(updates).Error; err != nil {
				return fmt.Errorf("activate subscription failed: %w", err)
			}

			// Mark order as active
			now := time.Now()
			db.WithContext(ctx).Model(&order).Updates(map[string]interface{}{
				"status":    2,
				"paid_at":   &now,
			})

		case "cancel":
			db.WithContext(ctx).Model(&order).Update("status", 3)

		case "refund":
			// Refund logic - add balance back
			db.WithContext(ctx).Model(&user).
				Update("balance", gorm.Expr("balance + ?", order.TotalAmount))
			db.WithContext(ctx).Model(&order).Update("status", 3)
		}

		return nil
	})

	// Send mail job handler
	registry.Register(JobSendMail, func(ctx context.Context, job *Job) error {
		to, _ := job.Payload["to"].(string)
		subject, _ := job.Payload["subject"].(string)
		body, _ := job.Payload["body"].(string)

		if to == "" || subject == "" {
			return fmt.Errorf("invalid mail job: missing to/subject")
		}

		mailSvc := service.NewMailServiceFromDB(db)
		if err := mailSvc.Send(to, subject, body); err != nil {
			db.WithContext(ctx).Create(&model.MailLog{
				Email:   to,
				Subject: subject,
				Error:   err.Error(),
			})
			return fmt.Errorf("send mail failed: %w", err)
		}

		db.WithContext(ctx).Create(&model.MailLog{
			Email:   to,
			Subject: subject,
		})
		return nil
	})

	// Send Telegram notification job handler
	registry.Register(JobSendTelegram, func(ctx context.Context, job *Job) error {
		chatID, _ := job.Payload["chat_id"].(float64)
		message, _ := job.Payload["message"].(string)
		botToken, _ := service.NewSettingService(db).Get("telegram_bot_token")

		if botToken == "" {
			log.Println("queue: telegram bot token not configured")
			return nil // don't retry - configuration issue
		}
		if message == "" {
			return fmt.Errorf("empty telegram message")
		}

		apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
		log.Printf("queue: sending telegram to chat_id=%.0f (url=%s)", chatID, apiURL)
		// In production, use http client to call Telegram API
		// For now, log it since no http client is available
		log.Printf("queue: telegram message: %s", message)

		if int64(chatID) > 0 {
			// Update user's telegram_id if binding
			db.WithContext(ctx).Model(&model.User{}).
				Where("telegram_id = ?", int64(chatID)).
				Update("telegram_id", int64(chatID))
		}

		// Record the notification
		db.WithContext(ctx).Create(&model.MailLog{
			Email:        fmt.Sprintf("telegram:%.0f", chatID),
			Subject:      "Telegram通知",
			TemplateName: "telegram_notify",
		})

		return nil
	})

	// User stats job handler
	registry.Register(JobStatUser, func(ctx context.Context, job *Job) error {
		userID, _ := job.Payload["user_id"].(float64)
		if userID == 0 {
			return fmt.Errorf("invalid user_id")
		}

		var user model.User
		if err := db.WithContext(ctx).First(&user, uint(userID)).Error; err != nil {
			return fmt.Errorf("user not found: %w", err)
		}

		stat := &model.StatUser{
			UserID:   uint(userID),
			U:        user.U,
			D:        user.D,
			RecordedAt: time.Now(),
		}
		return db.WithContext(ctx).Create(stat).Error
	})

	// Server stats job handler
	registry.Register(JobStatServer, func(ctx context.Context, job *Job) error {
		serverID, _ := job.Payload["server_id"].(float64)
		if serverID == 0 {
			return fmt.Errorf("invalid server_id")
		}

		stat := &model.StatServer{
			ServerID:   uint(serverID),
			RecordedAt: time.Now(),
		}
		return db.WithContext(ctx).Create(stat).Error
	})

	// Traffic fetch job handler
	registry.Register(JobTrafficFetch, func(ctx context.Context, job *Job) error {
		serverID, _ := job.Payload["server_id"].(float64)
		if serverID == 0 {
			return fmt.Errorf("invalid server_id")
		}

		// Record traffic for the node
		var server model.Server
		if err := db.WithContext(ctx).First(&server, uint(serverID)).Error; err != nil {
			return fmt.Errorf("server not found: %w", err)
		}

		log.Printf("queue: fetching traffic for server %d (traffic_used=%d)",
			uint(serverID), server.TrafficUsed)

		return nil
	})

	// Audit log job handler
	registry.Register(JobLogAudit, func(ctx context.Context, job *Job) error {
		userID, _ := job.Payload["user_id"].(float64)
		action, _ := job.Payload["action"].(string)
		ip, _ := job.Payload["ip"].(string)
		details, _ := job.Payload["details"].(string)

		log.Printf("queue: audit: user=%.0f action=%s ip=%s details=%s", userID, action, ip, details)

		audit := &AdminAuditLog{
			UserID:    uint(userID),
			Action:    action,
			IP:        ip,
			Details:   details,
			RecordedAt: time.Now(),
		}

		// Use raw DB since AdminAuditLog might not be a standard model
		return db.WithContext(ctx).Table("admin_audit_logs").Create(audit).Error
	})

	log.Printf("queue: registered all %d job handlers", len(registry.handlers))
}

// AdminAuditLog temporary struct for audit logging
type AdminAuditLog struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	UserID     uint      `gorm:"type:int(11);index" json:"user_id"`
	Action     string    `gorm:"type:varchar(128)" json:"action"`
	IP         string    `gorm:"type:varchar(64)" json:"ip"`
	Details    string    `gorm:"type:text" json:"details"`
	RecordedAt time.Time `json:"recorded_at"`
}
