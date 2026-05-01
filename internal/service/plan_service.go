package service

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/xboard/xboard/internal/model"
	"gorm.io/gorm"
)

// PlanService handles plan business logic
type PlanService struct {
	db *gorm.DB
}

func NewPlanService(db *gorm.DB) *PlanService {
	return &PlanService{db: db}
}

func (s *PlanService) GetPlanByID(id uint) (*model.Plan, error) {
	var plan model.Plan
	err := s.db.First(&plan, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("套餐不存在")
		}
		return nil, err
	}
	return &plan, nil
}

func (s *PlanService) GetEnabledPlans() ([]model.Plan, error) {
	var plans []model.Plan
	err := s.db.Where("enable = ?", 1).Order("sort ASC").Find(&plans).Error
	return plans, err
}

func (s *PlanService) GetAllPlans() ([]model.Plan, error) {
	var plans []model.Plan
	err := s.db.Order("sort ASC").Find(&plans).Error
	return plans, err
}

func (s *PlanService) CreatePlan(plan *model.Plan) error {
	return s.db.Create(plan).Error
}

func (s *PlanService) UpdatePlan(id uint, updates map[string]interface{}) error {
	return s.db.Model(&model.Plan{}).Where("id = ?", id).Updates(updates).Error
}

func (s *PlanService) DeletePlan(id uint) error {
	return s.db.Delete(&model.Plan{}, id).Error
}

func (s *PlanService) GetPlanPrice(plan *model.Plan, cycle string) float64 {
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

func GetTransferEnableMB(bytes int64) float64 {
	return math.Round(float64(bytes)/1024/1024*100) / 100
}

// CouponService handles coupon validation and discount
type CouponService struct {
	db *gorm.DB
}

func NewCouponService(db *gorm.DB) *CouponService {
	return &CouponService{db: db}
}

func (s *CouponService) ValidateCoupon(code string, userID, planID uint) (*model.Coupon, error) {
	var coupon model.Coupon
	if err := s.db.Where("code = ? AND enable = ?", code, 1).First(&coupon).Error; err != nil {
		return nil, errors.New("优惠券不存在或已停用")
	}

	now := time.Now()
	if !coupon.StartedAt.IsZero() && now.Before(coupon.StartedAt) {
		return nil, errors.New("优惠券尚未到使用时间")
	}
	if !coupon.EndedAt.IsZero() && now.After(coupon.EndedAt) {
		return nil, errors.New("优惠券已过期")
	}

	if coupon.LimitUse > 0 && coupon.UsedCount >= coupon.LimitUse {
		return nil, errors.New("优惠券已被用完")
	}

	// Check plan limit - LimitPlanIDs is model.Ints ([]uint)
		if len(coupon.LimitPlanIDs) > 0 {
		found := false
		for _, pid := range coupon.LimitPlanIDs {
			if uint(pid) == planID {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("优惠券不适用于此套餐")
		}
	}

	return &coupon, nil
}

func (s *CouponService) CalculateDiscount(coupon *model.Coupon, amount float64) float64 {
	if coupon.Type == 1 {
		return math.Round(amount*coupon.Value/100*100) / 100
	}
	if coupon.Value > amount {
		return amount
	}
	return coupon.Value
}

func (s *CouponService) IncrementUsage(couponID uint) error {
	return s.db.Model(&model.Coupon{}).Where("id = ?", couponID).
		Update("used_count", gorm.Expr("used_count + 1")).Error
}

// SettingService manages key-value settings
type SettingService struct {
	db *gorm.DB
}

func NewSettingService(db *gorm.DB) *SettingService {
	return &SettingService{db: db}
}

func (s *SettingService) Get(key string) (string, error) {
	var setting model.Setting
	err := s.db.Where("`key` = ?", key).First(&setting).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return setting.Value, nil
}

func (s *SettingService) Set(key, value string) error {
	var setting model.Setting
	result := s.db.Where("`key` = ?", key).First(&setting)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return s.db.Create(&model.Setting{Key: key, Value: value}).Error
		}
		return result.Error
	}
	return s.db.Model(&setting).Update("value", value).Error
}

func (s *SettingService) GetInt(key string) (int, error) {
	val, err := s.Get(key)
	if err != nil || val == "" {
		return 0, err
	}
	var i int
	fmt.Sscanf(val, "%d", &i)
	return i, nil
}

func (s *SettingService) GetFloat(key string) (float64, error) {
	val, err := s.Get(key)
	if err != nil || val == "" {
		return 0, err
	}
	var f float64
	fmt.Sscanf(val, "%f", &f)
	return f, nil
}

func (s *SettingService) GetBool(key string) (bool, error) {
	val, err := s.Get(key)
	if err != nil {
		return false, err
	}
	return val == "1" || val == "true", nil
}
