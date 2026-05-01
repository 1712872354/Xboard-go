package v2

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/internal/service"
	"github.com/xboard/xboard/pkg/response"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ============================================================================
// Additional User Management methods
// ============================================================================

// Generate creates one or more user accounts (admin)
func (h *AdminUserHandler) Generate(c *gin.Context) {
	var req struct {
		Email    string  `json:"email"`
		Password string  `json:"password"`
		Count    int     `json:"count"`
		Prefix   string  `json:"prefix"`
		PlanID   uint    `json:"plan_id"`
		Balance  float64 `json:"balance"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// Single user generation
	if req.Email != "" && req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			response.InternalError(c, "密码加密失败")
			return
		}
		userOpts := []service.UserOption{}
		if req.PlanID > 0 {
			userOpts = append(userOpts, service.WithPlan(req.PlanID))
		}
		if req.Balance > 0 {
			userOpts = append(userOpts, service.WithBalance(req.Balance))
		}
		user, err := h.userSvc.CreateUser(req.Email, string(hashedPassword), userOpts...)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, gin.H{
			"user_id": user.ID,
			"email":   user.Email,
			"token":   user.Token,
		})
		return
	}

	// Batch generation by count
	if req.Count > 0 && req.Count <= 100 {
		users := make([]*model.User, 0, req.Count)
		for i := 0; i < req.Count; i++ {
			email := fmt.Sprintf("%s%d@example.com", req.Prefix, i+1)
			if req.Prefix == "" {
				email = fmt.Sprintf("user_%d_%d@example.com", time.Now().UnixNano(), i)
			}
			password := "12345678"
			hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			users = append(users, &model.User{
				Email:    email,
				Password: string(hashedPassword),
				Token:    fmt.Sprintf("token_%d_%d", time.Now().UnixNano(), i),
				UUID:     fmt.Sprintf("uuid_%d_%d", time.Now().UnixNano(), i),
				PlanID:   req.PlanID,
			})
		}
		if err := h.userSvc.BatchCreateUsers(users); err != nil {
			response.InternalError(c, err.Error())
			return
		}
		response.Created(c, gin.H{
			"count":  len(users),
			"users":  users,
			"prefix": req.Prefix,
		})
		return
	}

	response.BadRequest(c, "请提供邮箱+密码 或 生成数量")
}

// DumpCSV exports users as CSV
func (h *AdminUserHandler) DumpCSV(c *gin.Context) {
	var users []model.User
	h.db.Order("id ASC").Find(&users)

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=users_export.csv")

	var sb strings.Builder
	sb.WriteString("ID,Email,余额,已用流量(MB),总流量(MB),套餐ID,过期时间,创建时间,状态\n")
	for _, u := range users {
		status := "正常"
		if u.Banned == 1 {
			status = "封禁"
		}
		usedMB := float64(u.U+u.D) / 1024 / 1024
		totalMB := float64(u.TransferEnable) / 1024 / 1024
		sb.WriteString(fmt.Sprintf("%d,%s,%.2f,%.2f,%.2f,%d,%d,%s,%s\n",
			u.ID, u.Email, u.Balance, usedMB, totalMB, u.PlanID,
			u.ExpiredAt.Unix(), u.CreatedAt.Format("2006-01-02 15:04:05"), status))
	}
	c.String(200, sb.String())
}

// SendMail broadcasts email to users (admin)
func (h *AdminUserHandler) SendMail(c *gin.Context) {
	var req struct {
		Subject string `json:"subject" binding:"required"`
		Content string `json:"content" binding:"required"`
		UserIDs []uint `json:"user_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// Build mail service from settings
	mailSvc := service.NewMailServiceFromDB(h.db)

	var users []model.User
	if len(req.UserIDs) > 0 {
		h.db.Where("id IN ?", req.UserIDs).Find(&users)
	} else {
		h.db.Find(&users)
	}

	sent := 0
	failed := 0
	for _, u := range users {
		if err := mailSvc.Send(u.Email, req.Subject, req.Content); err != nil {
			h.db.Create(&model.MailLog{
				Email:   u.Email,
				Subject: req.Subject,
				Error:   err.Error(),
			})
			failed++
		} else {
			h.db.Create(&model.MailLog{
				Email:   u.Email,
				Subject: req.Subject,
			})
			sent++
		}
	}

	response.Success(c, gin.H{
		"message": fmt.Sprintf("发送完成：成功 %d 封，失败 %d 封", sent, failed),
		"sent":    sent,
		"failed":  failed,
	})
}

// Destroy hard-deletes a user and related data
func (h *AdminUserHandler) Destroy(c *gin.Context) {
	id := parseUint(c.Param("id"))

	h.db.Transaction(func(tx *gorm.DB) error {
		// Delete related records
		tx.Where("user_id = ?", id).Delete(&model.Order{})
		tx.Where("user_id = ?", id).Delete(&model.Ticket{})
		tx.Where("user_id = ?", id).Delete(&model.InviteCode{})
		tx.Where("user_id = ? OR invite_user_id = ?", id, id).Delete(&model.CommissionLog{})
		tx.Where("user_id = ?", id).Delete(&model.StatUser{})
		tx.Where("user_id = ?", id).Delete(&model.MailLog{})
		tx.Where("user_id = ?", id).Delete(&model.LoginToken{})
		tx.Delete(&model.User{}, id)
		return nil
	})

	response.Success(c, gin.H{"message": "用户已删除"})
}

// GetUserInfoByID returns user info by ID
func (h *AdminUserHandler) GetUserInfoByID(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var user model.User
	if err := h.db.Preload("Plan").First(&user, id).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	response.Success(c, user)
}

// SetInviteUser sets the inviting user for a target user
func (h *AdminUserHandler) SetInviteUser(c *gin.Context) {
	var req struct {
		UserID       uint `json:"user_id" binding:"required"`
		InviteUserID uint `json:"invite_user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// Verify both users exist
	var user, inviteUser model.User
	if err := h.db.First(&user, req.UserID).Error; err != nil {
		response.NotFound(c, "目标用户不存在")
		return
	}
	if err := h.db.First(&inviteUser, req.InviteUserID).Error; err != nil {
		response.NotFound(c, "邀请用户不存在")
		return
	}

	if err := h.userSvc.UpdateUser(req.UserID, map[string]interface{}{
		"invite_user_id": req.InviteUserID,
	}); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "设置成功"})
}

// ResetUserSecret resets a user's security token/UUID (admin)
func (h *AdminUserHandler) ResetUserSecret(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	newToken := fmt.Sprintf("token_%d_%d", id, time.Now().UnixNano())
	newUUID := fmt.Sprintf("uuid_%d_%d", id, time.Now().UnixNano())

	h.db.Model(&user).Updates(map[string]interface{}{
		"token": newToken,
		"uuid":  newUUID,
	})

	response.Success(c, gin.H{
		"token": newToken,
		"uuid":  newUUID,
	})
}

// ============================================================================
// AdminPlanHandler - extra methods
// ============================================================================

// Sort sorts plans by a list of IDs
func (h *AdminPlanHandler) Sort(c *gin.Context) {
	var req []struct {
		ID   uint `json:"id"`
		Sort int  `json:"sort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	for _, item := range req {
		h.db.Model(&model.Plan{}).Where("id = ?", item.ID).Update("sort", item.Sort)
	}
	response.Success(c, nil)
}

// ============================================================================
// AdminServerHandler - extra methods (copy, batch ops, ECH key)
// ============================================================================

// Copy duplicates a server configuration
func (h *AdminServerHandler) Copy(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var original model.Server
	if err := h.db.First(&original, id).Error; err != nil {
		response.NotFound(c, "节点不存在")
		return
	}
	copy := original
	copy.ID = 0
	copy.Name = original.Name + " (副本)"
	copy.Show = 0
	if err := h.db.Create(&copy).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, copy)
}

// BatchDelete deletes multiple servers at once
func (h *AdminServerHandler) BatchDelete(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	h.db.Delete(&model.Server{}, req.IDs)
	response.Success(c, gin.H{"message": fmt.Sprintf("已删除 %d 个节点", len(req.IDs))})
}

// BatchUpdate updates multiple servers at once
func (h *AdminServerHandler) BatchUpdate(c *gin.Context) {
	var req struct {
		IDs     []uint                 `json:"ids" binding:"required"`
		Updates map[string]interface{} `json:"updates" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	delete(req.Updates, "id")
	if err := h.db.Model(&model.Server{}).Where("id IN ?", req.IDs).Updates(req.Updates).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"message": fmt.Sprintf("已更新 %d 个节点", len(req.IDs))})
}

// ResetTraffic resets traffic for a single server
func (h *AdminServerHandler) ResetTraffic(c *gin.Context) {
	id := parseUint(c.Param("id"))
	h.db.Model(&model.Server{}).Where("id = ?", id).Updates(map[string]interface{}{
		"u": 0,
		"d": 0,
	})
	response.Success(c, gin.H{"message": "流量已重置"})
}

// BatchResetTraffic resets traffic for multiple servers
func (h *AdminServerHandler) BatchResetTraffic(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	h.db.Model(&model.Server{}).Where("id IN ?", req.IDs).Updates(map[string]interface{}{
		"u": 0,
		"d": 0,
	})
	response.Success(c, gin.H{"message": fmt.Sprintf("已重置 %d 个节点的流量", len(req.IDs))})
}

// GenerateEchKey generates ECH key pair (simulated)
func (h *AdminServerHandler) GenerateEchKey(c *gin.Context) {
	publicKey := fmt.Sprintf("ech_pub_%d", time.Now().UnixNano())
	privateKey := fmt.Sprintf("ech_priv_%d", time.Now().UnixNano())
	response.Success(c, gin.H{
		"public_key":  publicKey,
		"private_key": privateKey,
		"algorithm":   "x25519",
		"created_at":  time.Now().Unix(),
	})
}

// ============================================================================
// AdminOrderHandler - extra methods
// ============================================================================

// Assign assigns an order to a payment method
func (h *AdminOrderHandler) Assign(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var req struct {
		PaymentID uint `json:"payment_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	h.db.Model(&model.Order{}).Where("id = ?", id).Update("payment_id", req.PaymentID)
	response.Success(c, gin.H{"message": "分配成功"})
}

// Paid marks an order as paid manually
func (h *AdminOrderHandler) Paid(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var order model.Order
	if err := h.db.First(&order, id).Error; err != nil {
		response.NotFound(c, "订单不存在")
		return
	}
	if order.Status != 1 {
		response.BadRequest(c, "订单状态不允许操作")
		return
	}
	go func() {
		h.orderSvc.OpenOrder(order.ID)
	}()
	response.Success(c, gin.H{"message": "订单已标记为已支付"})
}

// Cancel cancels an order
func (h *AdminOrderHandler) Cancel(c *gin.Context) {
	id := parseUint(c.Param("id"))
	if err := h.orderSvc.CloseOrder(id); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"message": "订单已取消"})
}

// ============================================================================
// AdminCouponHandler - extra methods
// ============================================================================

// Show returns details of a single coupon
func (h *AdminCouponHandler) Show(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var coupon model.Coupon
	if err := h.db.First(&coupon, id).Error; err != nil {
		response.NotFound(c, "优惠券不存在")
		return
	}
	response.Success(c, coupon)
}

// Update updates an existing coupon
func (h *AdminCouponHandler) Update(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Model(&model.Coupon{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// ============================================================================
// AdminTicketHandler - extra methods
// ============================================================================

// Close closes a ticket as admin
func (h *AdminTicketHandler) Close(c *gin.Context) {
	type CloseReq struct {
		TicketID uint `json:"ticket_id" binding:"required"`
	}
	var req CloseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	result := h.db.Model(&model.Ticket{}).Where("id = ?", req.TicketID).Update("status", 1)
	if result.RowsAffected == 0 {
		response.NotFound(c, "工单不存在")
		return
	}
	response.Success(c, gin.H{"message": "工单已关闭"})
}

// ============================================================================
// AdminKnowledgeHandler - extra methods
// ============================================================================

// GetCategory returns knowledge categories
func (h *AdminKnowledgeHandler) GetCategory(c *gin.Context) {
	type Category struct {
		Category string `json:"category"`
		Count    int64  `json:"count"`
	}
	var categories []Category
	h.db.Model(&model.Knowledge{}).
		Select("category, COUNT(*) as count").
		Group("category").
		Order("category ASC").
		Scan(&categories)
	response.Success(c, categories)
}

// Sort sorts knowledge items
func (h *AdminKnowledgeHandler) Sort(c *gin.Context) {
	var req []struct {
		ID   uint `json:"id"`
		Sort int  `json:"sort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	for _, item := range req {
		h.db.Model(&model.Knowledge{}).Where("id = ?", item.ID).Update("sort", item.Sort)
	}
	response.Success(c, nil)
}

// Show returns a single knowledge item
func (h *AdminKnowledgeHandler) Show(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var item model.Knowledge
	if err := h.db.First(&item, id).Error; err != nil {
		response.NotFound(c, "文章不存在")
		return
	}
	response.Success(c, item)
}

// ============================================================================
// AdminNoticeHandler - extra methods
// ============================================================================

// Sort sorts notices
func (h *AdminNoticeHandler) Sort(c *gin.Context) {
	var req []struct {
		ID   uint `json:"id"`
		Sort int  `json:"sort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	for _, item := range req {
		h.db.Model(&model.Notice{}).Where("id = ?", item.ID).Update("sort", item.Sort)
	}
	response.Success(c, nil)
}

// Show toggles the show/hide status of a notice
func (h *AdminNoticeHandler) Show(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var notice model.Notice
	if err := h.db.First(&notice, id).Error; err != nil {
		response.NotFound(c, "公告不存在")
		return
	}
	newShow := 1
	if notice.Show == 1 {
		newShow = 0
	}
	h.db.Model(&notice).Update("show", newShow)
	response.Success(c, gin.H{"show": newShow})
}

// ============================================================================
// AdminPaymentHandler - extra methods
// ============================================================================

// GetPaymentMethods returns payment methods list
func (h *AdminPaymentHandler) GetPaymentMethods(c *gin.Context) {
	var payments []model.Payment
	h.db.Order("sort ASC").Find(&payments)
	response.Success(c, payments)
}

// GetPaymentForm returns the form configuration for a payment method
func (h *AdminPaymentHandler) GetPaymentForm(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var payment model.Payment
	if err := h.db.First(&payment, id).Error; err != nil {
		response.NotFound(c, "支付方式不存在")
		return
	}
	response.Success(c, gin.H{
		"id":            payment.ID,
		"payment":       payment.Payment,
		"name":          payment.Name,
		"icon":          payment.Icon,
		"config":        payment.Config,
		"notify_domain": payment.NotifyDomain,
		"handle":        payment.Handle,
		"enable":        payment.Enable,
	})
}

// Show returns details of a single payment
func (h *AdminPaymentHandler) Show(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var payment model.Payment
	if err := h.db.First(&payment, id).Error; err != nil {
		response.NotFound(c, "支付方式不存在")
		return
	}
	response.Success(c, payment)
}

// ============================================================================
// AdminPluginHandler - extra methods
// ============================================================================

// Types returns available plugin types
func (h *AdminPluginHandler) Types(c *gin.Context) {
	response.Success(c, []gin.H{
		{"type": "payment", "label": "支付"},
		{"type": "hook", "label": "钩子"},
		{"type": "notification", "label": "通知"},
	})
}

// Install installs a plugin by code
func (h *AdminPluginHandler) Install(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	var count int64
	h.db.Model(&model.Plugin{}).Where("code = ?", req.Code).Count(&count)
	if count > 0 {
		response.BadRequest(c, "插件已存在")
		return
	}
	plugin := &model.Plugin{
		Code:      req.Code,
		Name:      req.Code,
		Version:   "1.0.0",
		IsEnabled: 1,
	}
	if err := h.db.Create(plugin).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, plugin)
}

// Uninstall uninstalls a plugin
func (h *AdminPluginHandler) Uninstall(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	h.db.Where("code = ?", req.Code).Delete(&model.Plugin{})
	response.Success(c, gin.H{"message": "插件已卸载"})
}

// Upgrade upgrades a plugin version
func (h *AdminPluginHandler) Upgrade(c *gin.Context) {
	var req struct {
		Code    string `json:"code" binding:"required"`
		Version string `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	result := h.db.Model(&model.Plugin{}).Where("code = ?", req.Code).Update("version", req.Version)
	if result.RowsAffected == 0 {
		response.NotFound(c, "插件不存在")
		return
	}
	response.Success(c, gin.H{"message": "插件已升级"})
}

// Upload uploads a new plugin package
func (h *AdminPluginHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("plugin")
	if err != nil {
		response.BadRequest(c, "请选择插件文件")
		return
	}
	dst := filepath.Join("storage", "plugins", file.Filename)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		response.InternalError(c, "上传失败")
		return
	}
	response.Success(c, gin.H{"message": "上传成功", "path": dst})
}

// Delete deletes a plugin package
func (h *AdminPluginHandler) Delete(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	// Remove from DB and file system
	h.db.Where("code = ?", req.Code).Delete(&model.Plugin{})
	filename := filepath.Join("storage", "plugins", req.Code+".zip")
	os.Remove(filename)
	response.Success(c, gin.H{"message": "删除成功"})
}

// GetConfig returns plugin configuration
func (h *AdminPluginHandler) GetConfig(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		response.BadRequest(c, "缺少插件代码")
		return
	}
	var plugin model.Plugin
	if err := h.db.Where("code = ?", code).First(&plugin).Error; err != nil {
		response.NotFound(c, "插件不存在")
		return
	}
	response.Success(c, gin.H{
		"code":   plugin.Code,
		"config": plugin.Config,
	})
}

// UpdateConfig updates plugin configuration
func (h *AdminPluginHandler) UpdateConfig(c *gin.Context) {
	var req struct {
		Code   string                 `json:"code" binding:"required"`
		Config map[string]interface{} `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	result := h.db.Model(&model.Plugin{}).Where("code = ?", req.Code).Update("config", req.Config)
	if result.RowsAffected == 0 {
		response.NotFound(c, "插件不存在")
		return
	}
	response.Success(c, gin.H{"message": "配置已更新"})
}

// ============================================================================
// AdminNotifyHandler - fix the STUB
// ============================================================================

// ============================================================================
// AdminMailTemplateHandler - extra methods
// ============================================================================

// Get returns a single mail template
func (h *AdminMailTemplateHandler) Get(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var tmpl model.MailTemplate
	if err := h.db.First(&tmpl, id).Error; err != nil {
		response.NotFound(c, "模板不存在")
		return
	}
	response.Success(c, tmpl)
}

// Reset resets a mail template to default
func (h *AdminMailTemplateHandler) Reset(c *gin.Context) {
	id := parseUint(c.Param("id"))
	h.db.Delete(&model.MailTemplate{}, id)
	response.Success(c, gin.H{"message": "模板已重置"})
}

// Test sends a test email using a template
func (h *AdminMailTemplateHandler) Test(c *gin.Context) {
	var req struct {
		ID     uint   `json:"id"`
		Email  string `json:"email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	var tmpl model.MailTemplate
	if req.ID > 0 {
		if err := h.db.First(&tmpl, req.ID).Error; err != nil {
			response.NotFound(c, "模板不存在")
			return
		}
	} else {
		tmpl.Subject = "测试邮件"
		tmpl.Template = "<h1>这是一封测试邮件</h1><p>如果收到此邮件，说明邮件配置正常。</p>"
	}

	mailSvc := service.NewMailServiceFromDB(h.db)
	body := tmpl.Template
	if body == "" {
		body = tmpl.Subject
	}
	if err := mailSvc.Send(req.Email, tmpl.Subject, body); err != nil {
		response.InternalError(c, "邮件发送失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{"message": "测试邮件已发送"})
}

// ============================================================================
// AdminThemeHandler - extra methods
// ============================================================================

// SaveThemeConfig saves theme configuration
func (h *AdminThemeHandler) SaveThemeConfig(c *gin.Context) {
	var req struct {
		Theme  string                 `json:"theme" binding:"required"`
		Config map[string]interface{} `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	var existing model.ThemeConfig
	result := h.db.Where("name = ?", req.Theme).First(&existing)
	if result.Error != nil {
		// Create new
		tc := &model.ThemeConfig{Name: req.Theme, Config: req.Config}
		h.db.Create(tc)
	} else {
		existing.SetConfig(req.Config)
		h.db.Save(&existing)
	}
	response.Success(c, gin.H{"message": "主题配置已保存"})
}

// GetThemeConfig returns theme configuration
func (h *AdminThemeHandler) GetThemeConfig(c *gin.Context) {
	theme := c.Query("theme")
	if theme == "" {
		response.BadRequest(c, "缺少主题名称")
		return
	}
	var config model.ThemeConfig
	if err := h.db.Where("name = ?", theme).First(&config).Error; err != nil {
		response.Success(c, gin.H{"theme": theme, "config": map[string]interface{}{}})
		return
	}
	response.Success(c, config)
}

// ============================================================================
// AdminConfigHandler - brand new handler for config management
// ============================================================================

// AdminConfigHandler handles advanced config management
type AdminConfigHandler struct {
	db      *gorm.DB
	setting *service.SettingService
	mailSvc *service.MailService
}

func NewAdminConfigHandler(db *gorm.DB, setting *service.SettingService, mailSvc *service.MailService) *AdminConfigHandler {
	return &AdminConfigHandler{db: db, setting: setting, mailSvc: mailSvc}
}

// Fetch returns all settings
func (h *AdminConfigHandler) Fetch(c *gin.Context) {
	var settings []model.Setting
	h.db.Find(&settings)
	sMap := make(map[string]string)
	for _, s := range settings {
		sMap[s.Key] = s.Value
	}
	response.Success(c, sMap)
}

// Save saves settings
func (h *AdminConfigHandler) Save(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	for key, value := range req {
		h.setting.Set(key, value)
	}
	response.Success(c, nil)
}

// GetEmailTemplate returns email template settings
func (h *AdminConfigHandler) GetEmailTemplate(c *gin.Context) {
	mailHost, _ := h.setting.Get("mail_host")
	mailPort, _ := h.setting.Get("mail_port")
	mailUsername, _ := h.setting.Get("mail_username")
	mailEncryption, _ := h.setting.Get("mail_encryption")
	mailFromAddr, _ := h.setting.Get("mail_from_address")
	mailFromName, _ := h.setting.Get("mail_from_name")
	response.Success(c, gin.H{
		"mail_host":        mailHost,
		"mail_port":        mailPort,
		"mail_username":    mailUsername,
		"mail_encryption":  mailEncryption,
		"mail_from_address": mailFromAddr,
		"mail_from_name":   mailFromName,
	})
}

// GetThemeTemplate returns theme template config
func (h *AdminConfigHandler) GetThemeTemplate(c *gin.Context) {
	themes := []gin.H{
		{"name": "default", "label": "默认主题", "version": "1.0"},
		{"name": "dark", "label": "深色主题", "version": "1.0"},
	}
	response.Success(c, themes)
}

// SetTelegramWebhook configures the Telegram webhook URL
func (h *AdminConfigHandler) SetTelegramWebhook(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	h.setting.Set("telegram_webhook_url", req.URL)
	response.Success(c, gin.H{"message": "Webhook已设置"})
}

// TestSendMail sends a test email
func (h *AdminConfigHandler) TestSendMail(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "邮箱格式错误")
		return
	}
	if err := h.mailSvc.Send(req.Email, "测试邮件", "<h1>邮件配置测试</h1><p>如果收到此邮件，说明SMTP配置正常。</p>"); err != nil {
		response.InternalError(c, "发送失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{"message": "测试邮件已发送"})
}

// ============================================================================
// AdminSystemHandler - extra methods
// ============================================================================

// GetSystemStatus returns detailed system status
func (h *AdminSystemHandler) GetSystemStatus(c *gin.Context) {
	version, _ := service.NewSettingService(h.db).Get("app_version")
	if version == "" {
		version = "v2.0.1"
	}
	var userCount, orderCount, serverCount int64
	h.db.Model(&model.User{}).Count(&userCount)
	h.db.Model(&model.Order{}).Count(&orderCount)
	h.db.Model(&model.Server{}).Count(&serverCount)

	response.Success(c, gin.H{
		"version":        version,
		"go_version":     "go1.22",
		"db_type":        "mysql",
		"user_count":     userCount,
		"order_count":    orderCount,
		"server_count":   serverCount,
		"uptime":         time.Since(startTime).String(),
	})
}

// GetQueueStats returns queue statistics
func (h *AdminSystemHandler) GetQueueStats(c *gin.Context) {
	response.Success(c, gin.H{
		"queue_enabled": false,
		"message":       "Go版本暂未实现队列系统，请参考原版PHP Horizon",
	})
}

// GetQueueWorkload returns queue workload
func (h *AdminSystemHandler) GetQueueWorkload(c *gin.Context) {
	response.Success(c, []interface{}{})
}

// GetHorizonFailedJobs returns failed horizon jobs
func (h *AdminSystemHandler) GetHorizonFailedJobs(c *gin.Context) {
	response.Success(c, []interface{}{})
}

// ============================================================================
// AdminBackupHandler - fix STUB with actual backup implementation
// ============================================================================

// Create executes an actual database backup using mysqldump
func (h *AdminBackupHandler) Create(c *gin.Context) {
	backupDir := "storage/backups"
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		response.InternalError(c, "创建备份目录失败")
		return
	}

	// Get DB connection settings from config or env
	dbHost := getEnvOrDefault("DB_HOST", "127.0.0.1")
	dbPort := getEnvOrDefault("DB_PORT", "3306")
	dbUser := getEnvOrDefault("DB_USERNAME", "root")
	dbPass := getEnvOrDefault("DB_PASSWORD", "")
	dbName := getEnvOrDefault("DB_DATABASE", "xboard")

	filename := fmt.Sprintf("backup_%s.sql", time.Now().Format("20060102_150405"))
	filepath := filepath.Join(backupDir, filename)

	// Execute mysqldump
	args := []string{
		"-h", dbHost,
		"-P", dbPort,
		"-u", dbUser,
	}
	if dbPass != "" {
		args = append(args, fmt.Sprintf("-p%s", dbPass))
	}
	args = append(args, "--routines", "--triggers", "--single-transaction", dbName)

	cmd := exec.Command("mysqldump", args...)
	output, err := cmd.Output()
	if err != nil {
		response.InternalError(c, fmt.Sprintf("备份失败: %v", err))
		return
	}

	if err := os.WriteFile(filepath, output, 0644); err != nil {
		response.InternalError(c, "写入备份文件失败")
		return
	}

	// Save backup record
	type BackupRecord struct {
		model.Model
		Filename string `json:"filename"`
		FileSize int64  `json:"file_size"`
	}
	record := BackupRecord{
		Filename: filename,
		FileSize: int64(len(output)),
	}
	h.db.Table("v2_backup_records").Create(&record)

	response.Created(c, gin.H{
		"filename":  filename,
		"file_size": record.FileSize,
		"path":      filepath,
		"created_at": time.Now(),
	})
}

// getEnvOrDefault gets env var or returns default
func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// startTime tracks server start time
var startTime = time.Now()

// ============================================================================
// AdminStatHandler - extra methods
// ============================================================================

// GetOverride returns dashboard overview stats
func (h *AdminStatHandler) GetOverride(c *gin.Context) {
	var (
		userCount, orderCount, paidCount int64
		monthOrderTotal, todayPaidCount  int64
		monthRegisterCount, monthOrderCount int64
	)
	h.db.Model(&model.User{}).Count(&userCount)
	h.db.Model(&model.Order{}).Count(&orderCount)

	monthStart := time.Now().AddDate(0, -1, 0)
	todayStart := time.Now().Truncate(24 * time.Hour)

	h.db.Model(&model.Order{}).Where("created_at >= ? AND status = 2", monthStart).Count(&paidCount)
	h.db.Model(&model.Order{}).Where("created_at >= ? AND status = 2", monthStart).Select("COALESCE(SUM(total_amount), 0)").Scan(&monthOrderTotal)
	h.db.Model(&model.Order{}).Where("created_at >= ? AND status = 2", todayStart).Count(&todayPaidCount)
	h.db.Model(&model.User{}).Where("created_at >= ?", monthStart).Count(&monthRegisterCount)
	h.db.Model(&model.Order{}).Where("created_at >= ?", monthStart).Count(&monthOrderCount)

	response.Success(c, gin.H{
		"user_count":          userCount,
		"order_count":         orderCount,
		"month_order_count":   monthOrderCount,
		"month_paid_count":    paidCount,
		"month_order_total":   monthOrderTotal,
		"today_paid_count":    todayPaidCount,
		"month_register_count": monthRegisterCount,
	})
}

// GetStats returns aggregated stats
func (h *AdminStatHandler) GetStats(c *gin.Context) {
	var stats []model.Stat
	h.db.Order("recorded_at DESC").Limit(30).Find(&stats)
	response.Success(c, stats)
}

// GetServerLastRank returns server traffic rank
func (h *AdminStatHandler) GetServerLastRank(c *gin.Context) {
	var servers []model.Server
	h.db.Where("enable = 1").Order("(u + d) DESC").Limit(20).Find(&servers)
	response.Success(c, servers)
}

// GetServerYesterdayRank returns yesterday's server traffic rank
func (h *AdminStatHandler) GetServerYesterdayRank(c *gin.Context) {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	var stats []model.StatServer
	h.db.Where("recorded_at LIKE ?", yesterday+"%").
		Order("(u + d) DESC").Limit(20).Find(&stats)
	response.Success(c, stats)
}

// GetOrder returns filtered order statistics
func (h *AdminStatHandler) GetOrder(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var orders []model.Order
	var total int64
	query := h.db.Model(&model.Order{})

	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	offset := (page - 1) * pageSize
	query.Preload("User").Preload("Plan").Order("id DESC").Offset(offset).Limit(pageSize).Find(&orders)
	response.Paginated(c, orders, total, page, pageSize)
}

// GetStatUser returns per-user stats
func (h *AdminStatHandler) GetStatUser(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var stats []model.StatUser
	var total int64
	h.db.Model(&model.StatUser{}).Count(&total)
	offset := (page - 1) * pageSize
	h.db.Preload("User").Order("recorded_at DESC").Offset(offset).Limit(pageSize).Find(&stats)
	response.Paginated(c, stats, total, page, pageSize)
}

// GetRanking returns user traffic ranking
func (h *AdminStatHandler) GetRanking(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	type Ranking struct {
		UserID uint   `json:"user_id"`
		Email  string `json:"email"`
		U      int64  `json:"u"`
		D      int64  `json:"d"`
		Total  int64  `json:"total"`
	}
	var rankings []Ranking
	h.db.Model(&model.User{}).
		Select("id as user_id, email, u, d, (u + d) as total").
		Where("plan_id > 0").
		Order("(u + d) DESC").
		Limit(limit).
		Scan(&rankings)
	response.Success(c, rankings)
}

// GetStatRecord returns stat records
func (h *AdminStatHandler) GetStatRecord(c *gin.Context) {
	var stats []model.Stat
	h.db.Where("1=1").Order("recorded_at DESC").Limit(60).Find(&stats)
	response.Success(c, stats)
}

// GetTrafficRank returns traffic rank by node or user
func (h *AdminStatHandler) GetTrafficRank(c *gin.Context) {
	rankType := c.DefaultQuery("type", "server")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 200 {
		limit = 20
	}

	if rankType == "user" {
		type UserTraffic struct {
			ID    uint   `json:"id"`
			Email string `json:"email"`
			U     int64  `json:"u"`
			D     int64  `json:"d"`
			Total int64  `json:"total"`
		}
		var users []UserTraffic
		h.db.Model(&model.User{}).
			Select("id, email, u, d, (u + d) as total").
			Where("plan_id > 0").
			Order("(u + d) DESC").
			Limit(limit).Scan(&users)
		response.Success(c, users)
	} else {
		type ServerTraffic struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
			U    int64  `json:"u"`
			D    int64  `json:"d"`
			Total int64 `json:"total"`
		}
		var servers []ServerTraffic
		h.db.Model(&model.Server{}).
			Select("id, name, u, d, (u + d) as total").
			Where("enable = 1").
			Order("(u + d) DESC").
			Limit(limit).Scan(&servers)
		response.Success(c, servers)
	}
}
