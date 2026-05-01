package v2

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/pkg/response"
	"gorm.io/gorm"
)

// AdminTrafficResetHandler handles traffic reset management
type AdminTrafficResetHandler struct {
	db *gorm.DB
}

func NewAdminTrafficResetHandler(db *gorm.DB) *AdminTrafficResetHandler {
	return &AdminTrafficResetHandler{db: db}
}

// Logs returns traffic reset log records
func (h *AdminTrafficResetHandler) Logs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var logs []model.TrafficResetLog
	var total int64
	h.db.Model(&model.TrafficResetLog{}).Count(&total)
	offset := (page - 1) * pageSize
	h.db.Preload("User").Order("id DESC").Offset(offset).Limit(pageSize).Find(&logs)
	response.Paginated(c, logs, total, page, pageSize)
}

// Stats returns traffic reset statistics
func (h *AdminTrafficResetHandler) Stats(c *gin.Context) {
	var totalResets int64
	var todayResets int64
	var uniqueUsers int64

	h.db.Model(&model.TrafficResetLog{}).Count(&totalResets)
	todayStart := time.Now().Truncate(24 * time.Hour)
	h.db.Model(&model.TrafficResetLog{}).Where("created_at >= ?", todayStart).Count(&todayResets)
	h.db.Model(&model.TrafficResetLog{}).Select("COUNT(DISTINCT user_id)").Scan(&uniqueUsers)

	response.Success(c, gin.H{
		"total_resets": totalResets,
		"today_resets": todayResets,
		"unique_users": uniqueUsers,
	})
}

// UserHistory returns reset history for a specific user
func (h *AdminTrafficResetHandler) UserHistory(c *gin.Context) {
	userID := parseUint(c.Param("user_id"))
	var logs []model.TrafficResetLog
	h.db.Where("user_id = ?", userID).Order("id DESC").Limit(50).Find(&logs)
	response.Success(c, logs)
}

// ResetUser manually resets traffic for a user
func (h *AdminTrafficResetHandler) ResetUser(c *gin.Context) {
	var req struct {
		UserID uint `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	var user model.User
	if err := h.db.First(&user, req.UserID).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	h.db.Transaction(func(tx *gorm.DB) error {
		tx.Model(&user).Updates(map[string]interface{}{
			"u": 0,
			"d": 0,
		})
		beforeTraffic := user.U + user.D
		tx.Create(&model.TrafficResetLog{
			UserID:     req.UserID,
			RecordedAt: time.Now(),
			Before:     beforeTraffic,
			After:      0,
		})
		return nil
	})

	response.Success(c, gin.H{"message": "用户流量已重置"})
}
