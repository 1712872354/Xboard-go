package service

import (
	"fmt"
	"time"

	"github.com/xboard/xboard/internal/event"
	"github.com/xboard/xboard/internal/model"
	"gorm.io/gorm"
)

// CommissionService handles commission/referral reward calculations
type CommissionService struct {
	db *gorm.DB
}

func NewCommissionService(db *gorm.DB) *CommissionService {
	return &CommissionService{db: db}
}

// SettleCommission settles commission for a completed order
// Called when an order is opened (paid and activated)
func (s *CommissionService) SettleCommission(order model.Order) error {
	if order.InviteUserID == 0 {
		return nil // no referrer
	}

	// Get referrer
	var inviter model.User
	if err := s.db.First(&inviter, order.InviteUserID).Error; err != nil {
		return fmt.Errorf("inviter not found: %w", err)
	}

	// Calculate commission amount
	commissionAmount := s.calculateCommission(order)
	if commissionAmount <= 0 {
		return nil
	}

	// Get commission rate from settings
	var commissionRate float64
	if setting, err := s.getSetting("commission_rate"); err == nil {
		fmt.Sscanf(setting, "%f", &commissionRate)
	}
	if commissionRate <= 0 {
		commissionRate = 10.0 // default 10%
	}

	// Create commission log
	commissionLog := &model.CommissionLog{
		InviteUserID: order.InviteUserID,
		OrderID:      order.ID,
		GetAmount:    order.TotalAmount,
		GiveAmount:   commissionAmount * commissionRate / 100,
		Status:       0, // pending
	}

	if err := s.db.Create(commissionLog).Error; err != nil {
		return fmt.Errorf("failed to create commission log: %w", err)
	}

	// Add commission to inviter's balance
	s.db.Model(&inviter).Update("balance", gorm.Expr("balance + ?", commissionLog.GiveAmount))

	return nil
}

// CheckPendingCommissions processes pending commissions
// Called by check_commission cron job
func (s *CommissionService) CheckPendingCommissions() error {
	// Get commission settlement delay from settings
	delayHours := 72 // default 72 hours
	if setting, err := s.getSetting("commission_settle_delay"); err == nil {
		var h int
		fmt.Sscanf(setting, "%d", &h)
		if h > 0 {
			delayHours = h
		}
	}

	cutoff := time.Now().Add(-time.Duration(delayHours) * time.Hour)

	// Find pending commissions older than cutoff
	var pendingLogs []model.CommissionLog
	s.db.Where("status = 0 AND created_at < ?", cutoff).Find(&pendingLogs)

	for _, clog := range pendingLogs {
		// Verify the order is still active (not refunded)
		var order model.Order
		if err := s.db.First(&order, clog.OrderID).Error; err != nil {
			continue
		}
		if order.Status != 2 { // not active
			// Cancel commission
			s.db.Model(&clog).Update("status", 2) // cancelled
			// Deduct from user balance
			var user model.User
			if err := s.db.First(&user, clog.InviteUserID).Error; err == nil {
				s.db.Model(&user).Update("balance", gorm.Expr("GREATEST(0, balance - ?)", clog.GiveAmount))
			}
			continue
		}

		// Settle commission
		s.db.Model(&clog).Update("status", 1) // settled
	}

	return nil
}

// calculateCommission calculates commission amount based on order
func (s *CommissionService) calculateCommission(order model.Order) float64 {
	if order.TotalAmount <= 0 {
		return 0
	}

	// Get commission type from settings
	commType := "percentage" // default
	if setting, err := s.getSetting("commission_type"); err == nil && setting != "" {
		commType = setting
	}

	switch commType {
	case "fixed":
		fixedAmount := 0.0
		if setting, err := s.getSetting("commission_fixed_amount"); err == nil {
			fmt.Sscanf(setting, "%f", &fixedAmount)
		}
		return fixedAmount
	default: // percentage
		rate := 10.0 // default 10%
		if setting, err := s.getSetting("commission_percentage"); err == nil {
			fmt.Sscanf(setting, "%f", &rate)
		}
		return order.TotalAmount * rate / 100
	}
}

// getSetting retrieves a setting value by key
func (s *CommissionService) getSetting(key string) (string, error) {
	var setting model.Setting
	if err := s.db.Where("`key` = ?", key).First(&setting).Error; err != nil {
		return "", err
	}
	return setting.Value, nil
}

// ---------------------------------------------------------------------------
// Integration helpers — call these from order/payment services
// ---------------------------------------------------------------------------

// FireOrderCreated publishes order created event
func FireOrderCreated(order *model.Order) {
	event.Publish(event.EventOrderCreated, map[string]interface{}{
		"order_id":     order.ID,
		"trade_no":     order.TradeNo,
		"user_id":      order.UserID,
		"plan_id":      order.PlanID,
		"total_amount": order.TotalAmount,
		"type":         order.Type,
	})
}

// FireOrderPaid publishes order paid event
func FireOrderPaid(order *model.Order) {
	event.Publish(event.EventOrderPaid, map[string]interface{}{
		"order_id":     order.ID,
		"trade_no":     order.TradeNo,
		"user_id":      order.UserID,
		"total_amount": order.TotalAmount,
	})
}

// FireOrderOpened publishes order opened event
func FireOrderOpened(order *model.Order) {
	event.Publish(event.EventOrderOpened, map[string]interface{}{
		"order_id": order.ID,
		"trade_no": order.TradeNo,
		"user_id":  order.UserID,
		"plan_id":  order.PlanID,
	})
}

// FireUserRegistered publishes user registered event
func FireUserRegistered(user *model.User) {
	event.Publish(event.EventUserRegistered, map[string]interface{}{
		"user_id": user.ID,
		"email":   user.Email,
	})
}

// FireTrafficReset publishes traffic reset event
func FireTrafficReset(userID uint) {
	event.Publish(event.EventTrafficReset, map[string]interface{}{
		"user_id": userID,
	})
}
