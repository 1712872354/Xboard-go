package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/internal/plugin"
	"gorm.io/gorm"
)

// Order status constants
const (
	OrderStatusPending  = 1
	OrderStatusActive   = 2
	OrderStatusCanceled = 3
	OrderStatusDeleted  = 4
)

// Order type constants
const (
	OrderTypeNew         = 1
	OrderTypeRenew       = 2
	OrderTypeUpgrade     = 3
	OrderTypeResetTraffic = 4
)

// OrderService handles order business logic
type OrderService struct {
	db        *gorm.DB
	userSvc   *UserService
	planSvc   *PlanService
	couponSvc *CouponService
	hook      *plugin.HookManager
}

func NewOrderService(
	db *gorm.DB,
	userSvc *UserService,
	planSvc *PlanService,
	couponSvc *CouponService,
	hook *plugin.HookManager,
) *OrderService {
	return &OrderService{
		db:        db,
		userSvc:   userSvc,
		planSvc:   planSvc,
		couponSvc: couponSvc,
		hook:      hook,
	}
}

// CreateOrder creates a new order with full validation
func (s *OrderService) CreateOrder(userID, planID uint, cycle string, couponCode string) (*model.Order, error) {
	user, err := s.getUser(userID)
	if err != nil {
		return nil, err
	}

	plan, err := s.planSvc.GetPlanByID(planID)
	if err != nil {
		return nil, fmt.Errorf("套餐不存在")
	}

	if plan.Enable != 1 {
		return nil, fmt.Errorf("套餐已停用")
	}

	order := &model.Order{
		UserID:           userID,
		PlanID:           planID,
		Type:             OrderTypeNew,
		Cycle:            cycle,
		Status:           OrderStatusPending,
		TotalAmount:      s.calculateAmount(plan, cycle),
		SurplusMethod:    "none",
		CommissionStatus: 0,
	}

	// Coupon handling
	if couponCode != "" {
		coupon, err := s.couponSvc.ValidateCoupon(couponCode, userID, planID)
		if err == nil {
			order.DiscountAmount = s.couponSvc.CalculateDiscount(coupon, order.TotalAmount)
			order.TotalAmount -= order.DiscountAmount
		}
	}

	// Determine order type
	if user.PlanID > 0 {
		if user.PlanID != planID {
			order.Type = OrderTypeUpgrade
			surplus, err := s.calculateSurplusValue(user, plan)
			if err == nil && surplus > 0 {
				order.SurplusAmount = surplus
				order.SurplusMethod = "balance"
				order.TotalAmount -= surplus
			}
		} else {
			order.Type = OrderTypeRenew
		}
	}

	if order.TotalAmount < 0 {
		order.TotalAmount = 0
	}

	if user.InviteUserID > 0 {
		order.InviteUserID = user.InviteUserID
	}

	order.TradeNo = s.generateTradeNo()

	if err := s.db.Create(order).Error; err != nil {
		return nil, fmt.Errorf("创建订单失败: %w", err)
	}

	return order, nil
}

// OpenOrder processes payment and activates subscription
func (s *OrderService) OpenOrder(orderID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var order model.Order
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&order, orderID).Error; err != nil {
			return err
		}
		if order.Status != OrderStatusPending {
			return errors.New("订单状态不允许操作")
		}

		var user model.User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, order.UserID).Error; err != nil {
			return err
		}

		updates := make(map[string]interface{})

		switch order.Type {
		case OrderTypeNew, OrderTypeRenew:
			updates["plan_id"] = order.PlanID
			expiredAt := time.Now()
			switch order.Cycle {
			case "monthly":
				expiredAt = expiredAt.AddDate(0, 1, 0)
			case "quarter":
				expiredAt = expiredAt.AddDate(0, 3, 0)
			case "half_year":
				expiredAt = expiredAt.AddDate(0, 6, 0)
			case "yearly":
				expiredAt = expiredAt.AddDate(1, 0, 0)
			default:
				expiredAt = expiredAt.AddDate(0, 1, 0)
			}
			updates["expired_at"] = expiredAt.Unix()

			if plan, err := s.planSvc.GetPlanByID(order.PlanID); err == nil {
				updates["transfer_enable"] = plan.TransferEnable
				updates["u"] = 0
				updates["d"] = 0
				updates["device_limit"] = plan.DeviceLimit
				updates["speed_limit"] = plan.SpeedLimit
			}

		case OrderTypeUpgrade:
			updates["plan_id"] = order.PlanID

		case OrderTypeResetTraffic:
			updates["u"] = 0
			updates["d"] = 0
		}

		if err := tx.Model(&user).Updates(updates).Error; err != nil {
			return err
		}

		return tx.Model(&order).Updates(map[string]interface{}{
			"status":  OrderStatusActive,
			"paid_at": time.Now(),
		}).Error
	})
}

// CloseOrder cancels a pending order
func (s *OrderService) CloseOrder(orderID uint) error {
	return s.db.Model(&model.Order{}).
		Where("id = ? AND status = ?", orderID, OrderStatusPending).
		Update("status", OrderStatusCanceled).Error
}

// CloseExpiredOrders closes all pending orders older than the specified duration
// Returns the number of orders closed
func (s *OrderService) CloseExpiredOrders(expireDuration time.Duration) (int64, error) {
	cutoff := time.Now().Add(-expireDuration)
	result := s.db.Model(&model.Order{}).
		Where("status = ? AND created_at < ?", OrderStatusPending, cutoff).
		Update("status", OrderStatusCanceled)
	return result.RowsAffected, result.Error
}

func (s *OrderService) getUser(userID uint) (*model.User, error) {
	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("用户不存在")
	}
	return &user, nil
}

func (s *OrderService) calculateAmount(plan *model.Plan, cycle string) float64 {
	switch cycle {
	case "monthly":
		return plan.MonthlyPrice
	case "quarter":
		return plan.QuarterPrice
	case "half_year":
		return plan.HalfYearPrice
	case "yearly":
		return plan.YearPrice
	case "two_year":
		return plan.TwoYearPrice
	case "three_year":
		return plan.ThreeYearPrice
	case "onetime":
		return plan.OnetimePrice
	case "reset":
		return plan.ResetPrice
	default:
		return plan.MonthlyPrice
	}
}

func (s *OrderService) calculateSurplusValue(user *model.User, newPlan *model.Plan) (float64, error) {
	if user.ExpiredAt.IsZero() || user.ExpiredAt.Before(time.Now()) {
		return 0, nil
	}
	oldPlan, err := s.planSvc.GetPlanByID(user.PlanID)
	if err != nil {
		return 0, err
	}
	remainingDays := user.ExpiredAt.Sub(time.Now()).Hours() / 24
	if remainingDays <= 0 {
		return 0, nil
	}
	totalDays := 30.0
	remainingRatio := remainingDays / totalDays
	if remainingRatio > 1 {
		remainingRatio = 1
	}
	return oldPlan.MonthlyPrice * remainingRatio * 0.8, nil
}

func (s *OrderService) generateTradeNo() string {
	now := time.Now()
	return fmt.Sprintf("%s%05d%03d",
		now.Format("20060102150405"),
		now.UnixMilli()%100000,
		now.Nanosecond()%1000,
	)
}

// GetUserOrders retrieves orders for a user
func (s *OrderService) GetUserOrders(userID uint, status int, page, pageSize int) ([]model.Order, int64, error) {
	var orders []model.Order
	query := s.db.Model(&model.Order{}).Where("user_id = ?", userID)
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	var total int64
	query.Count(&total)
	err := query.Preload("Plan").Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&orders).Error
	return orders, total, err
}

// GetPendingOrders returns all pending orders
func (s *OrderService) GetPendingOrders(limit int) ([]model.Order, error) {
	var orders []model.Order
	err := s.db.Where("status = ?", OrderStatusPending).
		Preload("User").Preload("Plan").
		Limit(limit).Find(&orders).Error
	return orders, err
}
