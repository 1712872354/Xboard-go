package service

import (
	"fmt"
	"math"
	"time"

	"github.com/xboard/xboard/internal/model"
	"gorm.io/gorm"
)

// DashboardService provides extended statistics beyond basic StatisticsService
type DashboardService struct {
	db *gorm.DB
}

func NewDashboardService(db *gorm.DB) *DashboardService {
	return &DashboardService{db: db}
}

// GetYearlyRevenue returns monthly revenue data for the current year
func (s *DashboardService) GetYearlyRevenue() ([]map[string]interface{}, error) {
	currentYear := time.Now().Year()
	startDate := time.Date(currentYear, 1, 1, 0, 0, 0, 0, time.UTC)

	var stats []model.Stat
	err := s.db.Where("recorded_at >= ?", startDate).
		Order("recorded_at ASC").Find(&stats).Error
	if err != nil {
		return nil, err
	}

	monthlyData := make(map[int]map[string]interface{})
	for i := 1; i <= 12; i++ {
		monthlyData[i] = map[string]interface{}{
			"month":             i,
			"trade_amount":      0.0,
			"trade_count":       0,
			"register_count":    0,
			"commission_amount": 0.0,
		}
	}

	for _, stat := range stats {
		month := stat.RecordedAt.Month()
		entry := monthlyData[int(month)]
		entry["trade_amount"] = entry["trade_amount"].(float64) + stat.TradeAmount
		entry["trade_count"] = entry["trade_count"].(int) + stat.TradeCount
		entry["register_count"] = entry["register_count"].(int) + stat.RegisterCount
		entry["commission_amount"] = entry["commission_amount"].(float64) + stat.CommissionAmount
	}

	var result []map[string]interface{}
	for i := 1; i <= 12; i++ {
		data := monthlyData[i]
		data["trade_amount"] = math.Round(data["trade_amount"].(float64)*100) / 100
		data["commission_amount"] = math.Round(data["commission_amount"].(float64)*100) / 100
		result = append(result, data)
	}

	return result, nil
}

// GetDailyStats returns daily statistics for the past N days
func (s *DashboardService) GetDailyStats(days int) ([]map[string]interface{}, error) {
	startDate := time.Now().AddDate(0, 0, -days)

	var stats []model.Stat
	err := s.db.Where("recorded_at >= ?", startDate).
		Order("recorded_at ASC").Find(&stats).Error
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, stat := range stats {
		result = append(result, map[string]interface{}{
			"date":              stat.RecordedAt.Format("2006-01-02"),
			"register_count":    stat.RegisterCount,
			"trade_count":       stat.TradeCount,
			"trade_amount":      math.Round(stat.TradeAmount*100) / 100,
			"commission_count":  stat.CommissionCount,
			"commission_amount": math.Round(stat.CommissionAmount*100) / 100,
			"paid_user_count":   stat.PaidUserCount,
		})
	}

	if result == nil {
		result = make([]map[string]interface{}, 0)
	}

	return result, nil
}

// GetUserGrowth returns user growth statistics (new users per day)
func (s *DashboardService) GetUserGrowth(days int) ([]map[string]interface{}, error) {
	startDate := time.Now().AddDate(0, 0, -days)

	type UserCount struct {
		DateStr string `gorm:"column:date_str"`
		Count   int    `gorm:"column:count"`
	}

	var rows []UserCount
	err := s.db.Raw(`
		SELECT DATE(created_at) AS date_str, COUNT(*) AS count
		FROM v2_user
		WHERE created_at >= ?
		GROUP BY DATE(created_at)
		ORDER BY date_str ASC
	`, startDate).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, row := range rows {
		result = append(result, map[string]interface{}{
			"date":  row.DateStr,
			"count": row.Count,
		})
	}

	if result == nil {
		result = make([]map[string]interface{}, 0)
	}

	return result, nil
}

// GetRevenueChart returns revenue chart data (daily revenue)
func (s *DashboardService) GetRevenueChart(days int) ([]map[string]interface{}, error) {
	startDate := time.Now().AddDate(0, 0, -days)

	type RevenueRow struct {
		DateStr string  `gorm:"column:date_str"`
		Amount  float64 `gorm:"column:amount"`
		Count   int     `gorm:"column:count"`
	}

	var rows []RevenueRow
	err := s.db.Raw(`
		SELECT DATE(paid_at) AS date_str,
			   COALESCE(SUM(total_amount), 0) AS amount,
			   COUNT(*) AS count
		FROM v2_order
		WHERE status = 2 AND paid_at >= ?
		GROUP BY DATE(paid_at)
		ORDER BY date_str ASC
	`, startDate).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, row := range rows {
		result = append(result, map[string]interface{}{
			"date":   row.DateStr,
			"amount": math.Round(row.Amount*100) / 100,
			"count":  row.Count,
		})
	}

	if result == nil {
		result = make([]map[string]interface{}, 0)
	}

	return result, nil
}

// GetServerTrafficStats returns traffic statistics for all servers
func (s *DashboardService) GetServerTrafficStats() ([]map[string]interface{}, error) {
	var servers []model.Server
	if err := s.db.Where("enable = 1").Find(&servers).Error; err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, server := range servers {
		usedPercent := 0.0
		if server.TrafficLimit > 0 {
			usedPercent = math.Round(float64(server.TrafficUsed)/float64(server.TrafficLimit)*10000) / 100
		}

		usedGB := math.Round(float64(server.TrafficUsed)/1024/1024/1024*100) / 100
		limitGB := math.Round(float64(server.TrafficLimit)/1024/1024/1024*100) / 100

		result = append(result, map[string]interface{}{
			"id":            server.ID,
			"name":          server.Name,
			"traffic_used":  usedGB,
			"traffic_limit": limitGB,
			"used_percent":  usedPercent,
			"online_count":  server.OnlineCount,
		})
	}

	if result == nil {
		result = make([]map[string]interface{}, 0)
	}

	return result, nil
}

// GetPaidUserStats returns statistics on paying vs free users
func (s *DashboardService) GetPaidUserStats() (map[string]interface{}, error) {
	var (
		totalUsers   int64
		paidUsers    int64
		activeUsers  int64
		totalOrders  int64
		totalRevenue float64
	)

	s.db.Model(&model.User{}).Count(&totalUsers)
	s.db.Model(&model.User{}).Where("plan_id > 0").Count(&paidUsers)
	s.db.Model(&model.User{}).Where("expired_at > ?", time.Now().Unix()).Count(&activeUsers)
	s.db.Model(&model.Order{}).Where("status = 2").Count(&totalOrders)
	s.db.Model(&model.Order{}).
		Select("COALESCE(SUM(total_amount), 0)").
		Where("status = 2").
		Scan(&totalRevenue)

	avgRevenuePerUser := 0.0
	if paidUsers > 0 {
		avgRevenuePerUser = math.Round(totalRevenue/float64(paidUsers)*100) / 100
	}

	return map[string]interface{}{
		"total_users":          totalUsers,
		"paid_users":           paidUsers,
		"paid_percent":         s.safePercent(paidUsers, totalUsers),
		"active_users":         activeUsers,
		"total_orders":         totalOrders,
		"total_revenue":        math.Round(totalRevenue*100) / 100,
		"avg_revenue_per_user": avgRevenuePerUser,
	}, nil
}

// GetOrderStats returns order statistics grouped by type
func (s *DashboardService) GetOrderStats() (map[string]interface{}, error) {
	type OrderStat struct {
		Type   int     `gorm:"column:type"`
		Count  int     `gorm:"column:count"`
		Amount float64 `gorm:"column:amount"`
	}

	var stats []OrderStat
	s.db.Model(&model.Order{}).
		Select("type, COUNT(*) AS count, COALESCE(SUM(total_amount), 0) AS amount").
		Where("status = 2").
		Group("type").
		Scan(&stats)

	typeMap := map[int]string{
		1: "new",
		2: "renew",
		3: "upgrade",
		4: "reset_traffic",
	}

	result := make(map[string]interface{})
	for _, stat := range stats {
		name := typeMap[stat.Type]
		if name == "" {
			name = fmt.Sprintf("unknown_%d", stat.Type)
		}
		result[name] = map[string]interface{}{
			"count":  stat.Count,
			"amount": math.Round(stat.Amount*100) / 100,
		}
	}

	return result, nil
}

func (s *DashboardService) safePercent(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return math.Round(float64(part)/float64(total)*10000) / 100
}
