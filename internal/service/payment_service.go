package service

import (
	"fmt"
	"math"
	"time"

	"github.com/xboard/xboard/internal/event"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/internal/plugin"
	"gorm.io/gorm"
)

// PaymentService handles payment business logic
type PaymentService struct {
	db        *gorm.DB
	pluginMgr *plugin.Manager
	orderSvc  *OrderService
}

func NewPaymentService(db *gorm.DB, pluginMgr *plugin.Manager, orderSvc *OrderService) *PaymentService {
	return &PaymentService{
		db:        db,
		pluginMgr: pluginMgr,
		orderSvc:  orderSvc,
	}
}

// GetPayment retrieves a payment method by ID
func (s *PaymentService) GetPayment(id uint) (*model.Payment, error) {
	var payment model.Payment
	err := s.db.First(&payment, id).Error
	return &payment, err
}

// GetEnabledPayments returns all enabled payment methods
func (s *PaymentService) GetEnabledPayments() ([]model.Payment, error) {
	var payments []model.Payment
	err := s.db.Where("enable = ?", 1).Order("sort ASC").Find(&payments).Error
	return payments, err
}

// PayOrder processes payment for an order
func (s *PaymentService) PayOrder(orderID, paymentID uint) (*plugin.PaymentResult, error) {
	order := &model.Order{}
	if err := s.db.Preload("User").First(order, orderID).Error; err != nil {
		return nil, fmt.Errorf("订单不存在")
	}

	if order.Status != 1 {
		return nil, fmt.Errorf("订单状态不允许支付")
	}

	payment, err := s.GetPayment(paymentID)
	if err != nil {
		return nil, fmt.Errorf("支付方式不存在")
	}

	payPlugins := s.pluginMgr.GetEnabledPaymentPlugins()
	var payPlugin plugin.PaymentPlugin
	for _, p := range payPlugins {
		if p.Code() == payment.Handle {
			payPlugin = p
			break
		}
	}
	if payPlugin == nil {
		return nil, fmt.Errorf("支付插件未启用: %s", payment.Handle)
	}

	notifyURL := payment.NotifyDomain + "/api/v1/guest/payment/notify/" + payment.Handle
	if payment.NotifyDomain == "" {
		notifyURL = "/api/v1/guest/payment/notify/" + payment.Handle
	}

	payOrder := &plugin.PaymentOrder{
		OrderID:     order.ID,
		TradeNo:     order.TradeNo,
		TotalAmount: order.TotalAmount,
		Subject:     fmt.Sprintf("购买套餐 #%d", order.PlanID),
		NotifyURL:   notifyURL,
		ReturnURL:   payment.NotifyDomain + "/#/order",
		UserID:      order.UserID,
	}

	return payPlugin.Pay(payOrder)
}

// HandleNotify handles payment gateway callback
func (s *PaymentService) HandleNotify(handle string, rawData map[string]interface{}) (*plugin.PaymentNotification, error) {
	payPlugins := s.pluginMgr.GetEnabledPaymentPlugins()
	var payPlugin plugin.PaymentPlugin
	for _, p := range payPlugins {
		if p.Code() == handle {
			payPlugin = p
			break
		}
	}
	if payPlugin == nil {
		return nil, fmt.Errorf("支付插件不存在: %s", handle)
	}

	notification, err := payPlugin.Notify(rawData)
	if err != nil {
		return nil, err
	}

	if notification.Status == "success" {
		if err := s.processSuccessfulPayment(notification.TradeNo, notification.GatewayTradeNo, notification.Amount); err != nil {
			return nil, err
		}
	}

	return notification, nil
}

func (s *PaymentService) processSuccessfulPayment(tradeNo, gatewayTradeNo string, amount float64) error {
	// Find the order
	var order model.Order
	if err := s.db.Where("trade_no = ? AND status = ?", tradeNo, 1).First(&order).Error; err != nil {
		return fmt.Errorf("订单不存在或已被处理")
	}

	// Update order status
	if err := s.db.Model(&order).Updates(map[string]interface{}{
		"status":      2,
		"callback_no": gatewayTradeNo,
		"paid_at":     time.Now(),
	}).Error; err != nil {
		return fmt.Errorf("订单更新失败: %w", err)
	}

	// Fire order paid event
	event.Publish(event.EventOrderPaid, map[string]interface{}{
		"order_id":     order.ID,
		"trade_no":     order.TradeNo,
		"user_id":      order.UserID,
		"total_amount": order.TotalAmount,
	})

	// Process order activation (renew/upgrade/new plan)
	if err := s.activateOrder(&order); err != nil {
		// Log error but don't fail the payment confirmation
		fmt.Printf("[PaymentService] activateOrder error for order %d: %v\n", order.ID, err)
	}

	// Settle commission
	commSvc := NewCommissionService(s.db)
	if err := commSvc.SettleCommission(order); err != nil {
		fmt.Printf("[PaymentService] commission settle error for order %d: %v\n", order.ID, err)
	}

	return nil
}

// activateOrder handles plan activation/renewal after successful payment
func (s *PaymentService) activateOrder(order *model.Order) error {
	var user model.User
	if err := s.db.First(&user, order.UserID).Error; err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}

	var plan model.Plan
	if err := s.db.First(&plan, order.PlanID).Error; err != nil {
		return fmt.Errorf("套餐不存在: %w", err)
	}

	switch order.Type {
	case 1: // New plan
		s.db.Model(&user).Updates(map[string]interface{}{
			"plan_id":        plan.ID,
			"group_id":       plan.GroupID,
			"speed_limit":    plan.SpeedLimit,
			"transfer_enable": plan.TransferEnable,
			"device_limit":   plan.DeviceLimit,
		})
		s.extendExpiry(&user, plan, order.Cycle)

	case 2: // Renew
		s.extendExpiry(&user, plan, order.Cycle)
		// Reset traffic on renewal if configured
		if plan.ResetTrafficMethod == 1 {
			s.db.Model(&user).Updates(map[string]interface{}{
				"u": 0,
				"d": 0,
			})
		}

	case 3: // Upgrade
		s.db.Model(&user).Updates(map[string]interface{}{
			"plan_id":        plan.ID,
			"group_id":       plan.GroupID,
			"speed_limit":    plan.SpeedLimit,
			"transfer_enable": plan.TransferEnable,
			"device_limit":   plan.DeviceLimit,
		})

	case 4: // Reset traffic
		s.db.Model(&user).Updates(map[string]interface{}{
			"u": 0,
			"d": 0,
		})
	}

	// Fire order opened event
	event.Publish(event.EventOrderOpened, map[string]interface{}{
		"order_id": order.ID,
		"trade_no": order.TradeNo,
		"user_id":  order.UserID,
		"plan_id":  order.PlanID,
	})

	return nil
}

// extendExpiry extends user's plan expiry
func (s *PaymentService) extendExpiry(user *model.User, plan model.Plan, period string) {
	now := time.Now()
	currentExpiry := user.ExpiredAt
	if currentExpiry.Before(now) {
		currentExpiry = now
	}

	var extension time.Duration
	switch period {
	case "monthly_price":
		extension = 30 * 24 * time.Hour
	case "quarter_price":
		extension = 90 * 24 * time.Hour
	case "half_year_price":
		extension = 180 * 24 * time.Hour
	case "year_price":
		extension = 365 * 24 * time.Hour
	case "two_year_price":
		extension = 730 * 24 * time.Hour
	case "three_year_price":
		extension = 1095 * 24 * time.Hour
	default:
		extension = 30 * 24 * time.Hour
	}

	newExpiry := currentExpiry.Add(extension)
	s.db.Model(user).Update("expired_at", newExpiry)
	s.db.Model(user).Update("plan_expired_at", newExpiry)
}

// ServerService handles server/node business logic
type ServerService struct {
	db *gorm.DB
}

func NewServerService(db *gorm.DB) *ServerService {
	return &ServerService{db: db}
}

func (s *ServerService) GetServerByID(id uint) (*model.Server, error) {
	var server model.Server
	err := s.db.Preload("Parent").First(&server, id).Error
	return &server, err
}

func (s *ServerService) GetAvailableServers(userID, planID uint, groupID *uint) ([]model.Server, error) {
	var servers []model.Server
	query := s.db.Where("enable = ?", 1).Order("sort ASC")

	if groupID != nil && *groupID > 0 {
		query = query.Joins("JOIN v2_server_group_relation ON v2_server.id = v2_server_group_relation.server_id").
			Where("v2_server_group_relation.server_group_id = ?", *groupID)
	}

	err := query.Preload("Parent").Find(&servers).Error
	return servers, err
}

// TicketService handles ticket business logic
type TicketService struct {
	db *gorm.DB
}

func NewTicketService(db *gorm.DB) *TicketService {
	return &TicketService{db: db}
}

func (s *TicketService) CreateTicket(userID uint, subject, message string, level int) (*model.Ticket, error) {
	ticket := &model.Ticket{
		UserID:  userID,
		Subject: subject,
		Level:   level,
		Status:  0,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ticket).Error; err != nil {
			return err
		}
		msg := &model.TicketMessage{
			TicketID: ticket.ID,
			UserID:   userID,
			Message:  message,
		}
		return tx.Create(msg).Error
	})
	return ticket, err
}

// TrafficResetService handles traffic reset logic
type TrafficResetService struct {
	db *gorm.DB
}

func NewTrafficResetService(db *gorm.DB) *TrafficResetService {
	return &TrafficResetService{db: db}
}

func (s *TrafficResetService) ResetUserTraffic(userID uint) error {
	return s.db.Model(&model.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{"u": 0, "d": 0}).Error
}

// StatisticsService calculates platform statistics
type StatisticsService struct {
	db *gorm.DB
}

func NewStatisticsService(db *gorm.DB) *StatisticsService {
	return &StatisticsService{db: db}
}

func (s *StatisticsService) GetDashboardStats() (map[string]interface{}, error) {
	var (
		userCount       int64
		orderCount      int64
		orderAmount     float64
		activeUserCount int64
		serverCount     int64
	)

	s.db.Model(&model.User{}).Count(&userCount)
	s.db.Model(&model.User{}).Where("expired_at > ?", time.Now().Unix()).Count(&activeUserCount)
	s.db.Model(&model.Order{}).Count(&orderCount)
	s.db.Model(&model.Order{}).
		Select("COALESCE(SUM(total_amount), 0)").
		Where("status = ?", 2).
		Scan(&orderAmount)
	s.db.Model(&model.Server{}).Where("enable = 1").Count(&serverCount)

	return map[string]interface{}{
		"user_count":        userCount,
		"order_count":       orderCount,
		"order_amount":      math.Round(orderAmount*100) / 100,
		"active_user_count": activeUserCount,
		"server_count":      serverCount,
	}, nil
}
