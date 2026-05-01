package cron

import (
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/internal/service"
	"gorm.io/gorm"
)

// Scheduler manages all scheduled tasks
type Scheduler struct {
	db        *gorm.DB
	cron      *cron.Cron
	orderSvc  *service.OrderService
	userSvc   *service.UserService
	statSvc   *service.StatisticsService
	resetSvc  *service.TrafficResetService
	mailSvc   *service.MailService
	mu        sync.Mutex
	running   map[string]bool
}

func NewScheduler(
	db *gorm.DB,
	orderSvc *service.OrderService,
	userSvc *service.UserService,
	statSvc *service.StatisticsService,
	resetSvc *service.TrafficResetService,
	mailSvc *service.MailService,
) *Scheduler {
	return &Scheduler{
		db:       db,
		cron:     cron.New(cron.WithSeconds()),
		orderSvc: orderSvc,
		userSvc:  userSvc,
		statSvc:  statSvc,
		resetSvc: resetSvc,
		mailSvc:  mailSvc,
		running:  make(map[string]bool),
	}
}

// Start begins all scheduled tasks
func (s *Scheduler) Start() {
	// Daily statistics at 00:10
	s.addJob("0 10 0 * * *", "xboard:statistics", s.dailyStatistics)

	// Check orders every minute
	s.addJob("0 * * * * *", "check:order", s.checkOrders)

	// Check commission every minute
	s.addJob("0 * * * * *", "check:commission", s.checkCommission)

	// Check tickets every minute
	s.addJob("0 * * * * *", "check:ticket", s.checkTickets)

	// Check traffic exceeded every minute
	s.addJob("0 * * * * *", "check:traffic-exceeded", s.checkTrafficExceeded)

	// Reset traffic every minute
	s.addJob("0 * * * * *", "reset:traffic", s.resetTraffic)

	// Reset logs daily
	s.addJob("0 0 0 * * *", "reset:log", s.resetLogs)

	// Send reminder email at 11:30 daily
	s.addJob("0 30 11 * * *", "send:remindMail", s.sendRemindMail)

	// Cleanup online status every 5 minutes
	s.addJob("0 */5 * * * *", "cleanup:online-status", s.cleanupOnlineStatus)

	s.cron.Start()
	log.Println("scheduler started with 9 tasks")
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Println("scheduler stopped")
}

// addJob adds a cron job with concurrency control
func (s *Scheduler) addJob(spec, name string, handler func()) {
	s.cron.AddFunc(spec, func() {
		s.mu.Lock()
		if s.running[name] {
			s.mu.Unlock()
			return
		}
		s.running[name] = true
		s.mu.Unlock()

		defer func() {
			s.mu.Lock()
			delete(s.running, name)
			s.mu.Unlock()
			if r := recover(); r != nil {
				log.Printf("cron job %s panicked: %v", name, r)
			}
		}()

		handler()
	})
}

// dailyStatistics creates a daily stat record in v2_stat
func (s *Scheduler) dailyStatistics() {
	log.Println("running daily statistics...")

	today := time.Now().Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	// Check if today's record already exists
	var existing model.Stat
	result := s.db.Where("recorded_at >= ? AND recorded_at < ?", today, tomorrow).First(&existing)
	if result.RowsAffected > 0 {
		log.Println("daily statistics already recorded for today")
		return
	}

	// Count new registrations
	var registerCount int64
	s.db.Model(&model.User{}).Where("created_at >= ? AND created_at < ?", today, tomorrow).Count(&registerCount)

	// Count new orders and revenue
	var tradeCount int64
	var tradeAmount float64
	s.db.Model(&model.Order{}).
		Where("status = ? AND paid_at >= ? AND paid_at < ?", 2, today, tomorrow).
		Count(&tradeCount)
	s.db.Model(&model.Order{}).
		Select("COALESCE(SUM(total_amount), 0)").
		Where("status = ? AND paid_at >= ? AND paid_at < ?", 2, today, tomorrow).
		Scan(&tradeAmount)

	// Count active users
	var activeUserCount int64
	s.db.Model(&model.User{}).Where("expired_at > ?", time.Now()).Count(&activeUserCount)

	// Create daily stat record
	stat := model.Stat{
		RegisterCount: int(registerCount),
		TradeCount:    int(tradeCount),
		TradeAmount:   tradeAmount,
		PaidUserCount: int(activeUserCount),
		RecordedAt:    time.Now(),
	}

	if err := s.db.Create(&stat).Error; err != nil {
		log.Printf("failed to create daily stat: %v", err)
		return
	}

	log.Printf("daily statistics recorded: %d registrations, %d orders, %.2f revenue, %d active users",
		registerCount, tradeCount, tradeAmount, activeUserCount)
}

// checkOrders closes orders older than 15 minutes
func (s *Scheduler) checkOrders() {
	orders, err := s.orderSvc.GetPendingOrders(100)
	if err != nil {
		log.Printf("check orders error: %v", err)
		return
	}
	for _, order := range orders {
		if time.Since(order.CreatedAt) > 15*time.Minute {
			s.orderSvc.CloseOrder(order.ID)
		}
	}
}

// checkCommission processes pending commissions
func (s *Scheduler) checkCommission() {
	var orders []model.Order
	if err := s.db.Where("commission_status = 0 AND status = ? AND invite_user_id > 0", 2).
		Find(&orders).Error; err != nil {
		log.Printf("check commission query error: %v", err)
		return
	}

	for _, order := range orders {
		var inviter model.User
		if err := s.db.First(&inviter, order.InviteUserID).Error; err != nil {
			log.Printf("commission: inviter %d not found, skipping order %d", order.InviteUserID, order.ID)
			continue
		}

		commissionRate := inviter.CommissionRate
		if commissionRate <= 0 {
			continue
		}

		commissionAmount := order.TotalAmount * float64(commissionRate) / 100.0
		if commissionAmount <= 0 {
			continue
		}

		err := s.db.Transaction(func(tx *gorm.DB) error {
			// Mark order commission as processed
			if err := tx.Model(&model.Order{}).Where("id = ?", order.ID).
				Update("commission_status", 1).Error; err != nil {
				return err
			}

			// Insert commission log
			commissionLog := model.CommissionLog{
				InviteUserID: order.InviteUserID,
				OrderID:      order.ID,
				GetAmount:    commissionAmount,
				Status:       1,
			}
			if err := tx.Create(&commissionLog).Error; err != nil {
				return err
			}

			// Update user commission balance
			return tx.Model(&model.User{}).Where("id = ?", order.InviteUserID).
				Update("commission_balance", gorm.Expr("commission_balance + ?", commissionAmount)).Error
		})

		if err != nil {
			log.Printf("failed to process commission for order %d: %v", order.ID, err)
		}
	}
}

// checkTickets finds tickets with no admin reply for >24h and notifies admin
func (s *Scheduler) checkTickets() {
	var tickets []model.Ticket
	if err := s.db.Where("status = 0").Find(&tickets).Error; err != nil {
		log.Printf("check tickets error: %v", err)
		return
	}

	for _, ticket := range tickets {
		// Get the last message in the ticket
		var lastMsg model.TicketMessage
		if err := s.db.Where("ticket_id = ?", ticket.ID).Order("id DESC").First(&lastMsg).Error; err != nil {
			continue
		}

		// Check if the last message was from a non-admin user and is older than 24h
		var msgUser model.User
		if err := s.db.First(&msgUser, lastMsg.UserID).Error; err != nil {
			continue
		}

		if msgUser.IsAdmin == 0 && time.Since(lastMsg.CreatedAt) > 24*time.Hour {
			log.Printf("ticket #%d (%s) has no admin reply for >24h (last reply by user %d)",
				ticket.ID, ticket.Subject, msgUser.ID)
		}
	}
}

// checkTrafficExceeded suspends users who exceeded traffic limits
func (s *Scheduler) checkTrafficExceeded() {
	var users []model.User
	if err := s.db.Where("u + d > transfer_enable AND expired_at < ? AND plan_id > 0",
		time.Now()).Find(&users).Error; err != nil {
		log.Printf("check traffic exceeded error: %v", err)
		return
	}

	for _, user := range users {
		if err := s.db.Model(&user).Update("plan_id", 0).Error; err != nil {
			log.Printf("failed to suspend user %d: %v", user.ID, err)
			continue
		}
		log.Printf("user %d (%s) suspended for exceeding traffic limit", user.ID, user.Email)
	}
}

// resetTraffic resets traffic for users whose traffic_reset_day matches current day
func (s *Scheduler) resetTraffic() {
	today := time.Now().Day()

	var users []model.User
	if err := s.db.Where("traffic_reset_day = ? AND plan_id > 0", today).Find(&users).Error; err != nil {
		log.Printf("reset traffic query error: %v", err)
		return
	}

	for _, user := range users {
		before := user.U + user.D

		err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&model.User{}).Where("id = ?", user.ID).
				Updates(map[string]interface{}{"u": 0, "d": 0}).Error; err != nil {
				return err
			}

			resetLog := model.TrafficResetLog{
				UserID: user.ID,
				Before: before,
				After:  0,
			}
			return tx.Create(&resetLog).Error
		})

		if err != nil {
			log.Printf("failed to reset traffic for user %d: %v", user.ID, err)
		} else {
			log.Printf("traffic reset for user %d: before=%d, after=0", user.ID, before)
		}
	}
}

// resetLogs deletes old logs older than 30 days
func (s *Scheduler) resetLogs() {
	cutoff := time.Now().AddDate(0, 0, -30)
	log.Printf("cleaning up logs older than %s", cutoff.Format("2006-01-02"))

	s.db.Where("recorded_at < ?", cutoff).Delete(&model.ServerLog{})
	s.db.Where("recorded_at < ?", cutoff).Delete(&model.Stat{})
	s.db.Where("recorded_at < ?", cutoff).Delete(&model.StatServer{})
	s.db.Where("recorded_at < ?", cutoff).Delete(&model.StatUser{})

	log.Println("old logs cleaned up")
}

// sendRemindMail sends expiry reminder emails to users expiring in 1, 3, 7 days
func (s *Scheduler) sendRemindMail() {
	if s.mailSvc == nil {
		log.Println("mail service not available, skipping reminder email")
		return
	}

	now := time.Now()
	remindDays := []int{1, 3, 7}

	for _, days := range remindDays {
		targetDate := now.AddDate(0, 0, days)
		startOfDay := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		var users []model.User
		if err := s.db.Where("expired_at >= ? AND expired_at < ? AND plan_id > 0",
			startOfDay, endOfDay).Find(&users).Error; err != nil {
			log.Printf("find expiring users for %d days error: %v", days, err)
			continue
		}

		for _, user := range users {
			params := map[string]string{
				"email":      user.Email,
				"days":       time.Until(user.ExpiredAt).Round(time.Hour).String(),
				"expired_at": user.ExpiredAt.Format("2006-01-02 15:04:05"),
			}

			if err := s.mailSvc.SendWithTemplate(user.Email, "expire_remind", params); err != nil {
				log.Printf("failed to send reminder to %s (%d days): %v", user.Email, days, err)
			} else {
				log.Printf("reminder sent to %s (%d days until expiry)", user.Email, days)
			}
		}
	}
}

// cleanupOnlineStatus cleans up stale online status records
func (s *Scheduler) cleanupOnlineStatus() {
	log.Println("cleaning up online status...")
	// Reset old online counts for servers that haven't pushed traffic recently
	s.db.Model(&model.Server{}).
		Where("last_push_at > 0 AND last_push_at < ?", time.Now().Add(-10*time.Minute).Unix()).
		Update("online_count", 0)
	log.Println("online status cleanup completed")
}

// ContextKey for storing services in context
type ContextKey string

const (
	OrderServiceKey = ContextKey("order_service")
	UserServiceKey  = ContextKey("user_service")
)

// SchedulerService wraps the scheduler for dependency injection
type SchedulerService struct {
	*Scheduler
}

func NewSchedulerService(s *Scheduler) *SchedulerService {
	return &SchedulerService{Scheduler: s}
}
