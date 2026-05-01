package v1

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xboard/xboard/internal/auth"
	"github.com/xboard/xboard/internal/event"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/internal/protocol"
	"github.com/xboard/xboard/internal/service"
	"github.com/xboard/xboard/pkg/response"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ============================================================================
// PassportHandler handles authentication
// ============================================================================

type PassportHandler struct {
	db          *gorm.DB
	authz       *auth.JWTAuth
	userSvc     *service.UserService
	setting     *service.SettingService
	tokenSvc    *service.TokenService
	mailSvc     *service.MailService
	captchaSvc  *service.CaptchaService
	loginSecSvc *service.LoginSecurityService
}

func NewPassportHandler(db *gorm.DB, authz *auth.JWTAuth, userSvc *service.UserService, setting *service.SettingService, tokenSvc *service.TokenService, mailSvc *service.MailService) *PassportHandler {
	return &PassportHandler{
		db:          db,
		authz:       authz,
		userSvc:     userSvc,
		setting:     setting,
		tokenSvc:    tokenSvc,
		mailSvc:     mailSvc,
		captchaSvc:  service.NewCaptchaService(db),
		loginSecSvc: service.NewLoginSecurityService(db),
	}
}

func (h *PassportHandler) Login(c *gin.Context) {
	var req struct {
		Email        string `json:"email" binding:"required,email"`
		Password     string `json:"password" binding:"required,min:8"`
		CaptchaToken string `json:"captcha_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "邮箱或密码格式错误")
		return
	}

	// Captcha verification
	captchaEnable, _ := h.setting.GetBool("captcha_enable")
	if captchaEnable && h.captchaSvc != nil {
		if req.CaptchaToken == "" {
			response.BadRequest(c, "请完成验证码验证")
			return
		}
		if err := h.captchaSvc.Verify(req.CaptchaToken, c.ClientIP()); err != nil {
			response.BadRequest(c, "验证码验证失败: "+err.Error())
			return
		}
	}

	// Check if account is temporarily locked due to too many failed attempts
	if h.loginSecSvc != nil {
		locked, msg := h.loginSecSvc.IsLoginLocked(req.Email)
		if locked {
			response.Forbidden(c, msg)
			return
		}
	}

	var user model.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		response.Unauthorized(c, "邮箱或密码错误")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		// Record failed login attempt
		if h.loginSecSvc != nil {
			h.loginSecSvc.RecordFailedLogin(req.Email)
		}
		response.Unauthorized(c, "邮箱或密码错误")
		return
	}

	// Clear failed login attempts on success
	if h.loginSecSvc != nil {
		h.loginSecSvc.ClearFailedLogins(req.Email)
	}

	if user.Banned == 1 {
		response.Forbidden(c, "账号已被禁用")
		return
	}

	token, err := h.authz.GenerateToken(user.ID, user.Email, user.IsAdmin == 1, user.Staff == 1, 1)
	if err != nil {
		response.InternalError(c, "生成令牌失败")
		return
	}

	response.Success(c, gin.H{
		"token":      token,
		"user_id":    user.ID,
		"email":      user.Email,
		"is_admin":   user.IsAdmin,
		"is_staff":   user.Staff == 1,
		"token_type": "Bearer",
	})
}

func (h *PassportHandler) Register(c *gin.Context) {
	stopRegister, _ := h.setting.GetBool("stop_register")
	if stopRegister {
		response.Forbidden(c, "注册已关闭")
		return
	}

	// IP rate limiting
	if h.loginSecSvc != nil {
		if allowed, msg := h.loginSecSvc.CheckRegisterRate(c.ClientIP()); !allowed {
			response.Error(c, http.StatusTooManyRequests, msg)
			return
		}
	}

	var req struct {
		Email        string `json:"email" binding:"required,email"`
		Password     string `json:"password" binding:"required,min:8"`
		InviteCode   string `json:"invite_code"`
		CaptchaToken string `json:"captcha_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数格式错误")
		return
	}

	// Captcha verification
	captchaEnable, _ := h.setting.GetBool("captcha_enable")
	if captchaEnable && h.captchaSvc != nil {
		if req.CaptchaToken == "" {
			response.BadRequest(c, "请完成验证码验证")
			return
		}
		if err := h.captchaSvc.Verify(req.CaptchaToken, c.ClientIP()); err != nil {
			response.BadRequest(c, "验证码验证失败: "+err.Error())
			return
		}
	}

	// Email whitelist check
	emailWhitelistEnable, _ := h.setting.GetBool("email_whitelist_enable")
	if emailWhitelistEnable {
		var suffixes string
		if v, err := h.setting.Get("email_whitelist_suffixes"); err == nil && v != "" {
			suffixes = v
		}
		if suffixes != "" {
			allowed := false
			for _, s := range strings.Split(suffixes, ",") {
				s = strings.TrimSpace(s)
				if s != "" && strings.HasSuffix(req.Email, s) {
					allowed = true
					break
				}
			}
			if !allowed {
				response.BadRequest(c, "该邮箱域名不在白名单中")
				return
			}
		}
	}

	inviteForce, _ := h.setting.GetBool("invite_force")
	if inviteForce && req.InviteCode == "" {
		response.BadRequest(c, "需要邀请码")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		response.InternalError(c, "密码加密失败")
		return
	}

	user, err := h.userSvc.CreateUser(req.Email, string(hashedPassword))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.InviteCode != "" {
		var invite model.InviteCode
		if err := h.db.Where("code = ? AND status = 0", req.InviteCode).First(&invite).Error; err == nil {
			h.db.Model(&model.InviteCode{}).Where("id = ?", invite.ID).Update("status", 1)
			h.db.Model(&model.User{}).Where("id = ?", user.ID).Update("invite_user_id", invite.UserID)
		}
	}

	// Fire event
	event.GetBus().Publish(event.Event{Type: event.EventUserRegistered, Data: map[string]interface{}{"user_id": user.ID, "email": user.Email}})

	// Record registration IP
	if h.loginSecSvc != nil {
		h.loginSecSvc.RecordRegister(c.ClientIP())
	}

	response.Created(c, gin.H{"user_id": user.ID, "email": user.Email})
}

func (h *PassportHandler) Logout(c *gin.Context) {
	response.Success(c, gin.H{"message": "已退出登录"})
}

// LoginWithMailLink sends a magic-link login email
func (h *PassportHandler) LoginWithMailLink(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "邮箱格式错误")
		return
	}

	user, err := h.userSvc.GetUserByEmail(req.Email)
	if err != nil || user == nil {
		// Don't reveal whether the email exists
		response.Success(c, gin.H{"message": "验证邮件已发送"})
		return
	}

	// Invalidate any existing unused magic link tokens for this user
	h.tokenSvc.InvalidateUserTokens(user.ID, model.TokenTypeMagicLink)

	// Generate new one-time token
	loginToken, err := h.tokenSvc.GenerateToken(user.ID, req.Email, model.TokenTypeMagicLink)
	if err != nil {
		response.InternalError(c, "生成令牌失败")
		return
	}

	// Send magic link email
	appName, _ := h.setting.Get("app_name")
	appURL, _ := h.setting.Get("app_url")
	if appName == "" {
		appName = "Xboard"
	}
	if appURL == "" {
		appURL = "https://example.com"
	}
	magicLink := fmt.Sprintf("%s/api/v1/passport/auth/token2Login?token=%s", appURL, loginToken.Token)
	subject := fmt.Sprintf("[%s] 一键登录链接", appName)
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px;">
	<h2>%s - 一键登录</h2>
	<p>您好，</p>
	<p>请点击下方链接一键登录（链接30分钟内有效）：</p>
	<p><a href="%s" style="display: inline-block; background: #4F46E5; color: #fff; padding: 12px 24px; border-radius: 6px; text-decoration: none;">立即登录</a></p>
	<p>如果按钮无法点击，请复制以下链接到浏览器：</p>
	<p style="color: #666; word-break: break-all;">%s</p>
	<p>如果不是您本人操作，请忽略此邮件。</p>
</body>
</html>`, appName, magicLink, magicLink)

	if err := h.mailSvc.Send(req.Email, subject, body); err != nil {
		// Log error but don't expose to user
		h.db.Create(&model.MailLog{
			Email:        req.Email,
			Subject:      subject,
			TemplateName: "login_magic_link",
			Error:        err.Error(),
		})
		response.InternalError(c, "邮件发送失败")
		return
	}

	h.db.Create(&model.MailLog{
		Email:        req.Email,
		Subject:      subject,
		TemplateName: "login_magic_link",
	})

	response.Success(c, gin.H{"message": "验证邮件已发送"})
}

// Token2Login exchanges a one-time token for a JWT token
func (h *PassportHandler) Token2Login(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		var req struct {
			Token string `json:"token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "参数错误")
			return
		}
		token = req.Token
	}

	// Validate the one-time magic link token
	loginToken, err := h.tokenSvc.ValidateToken(token, model.TokenTypeMagicLink)
	if err != nil {
		response.BadRequest(c, "链接无效或已过期")
		return
	}

	user, err := h.userSvc.GetUserByEmail(loginToken.Email)
	if err != nil || user == nil {
		response.NotFound(c, "用户不存在")
		return
	}

	if user.Banned == 1 {
		response.Forbidden(c, "账号已被禁用")
		return
	}

	jwtToken, err := h.authz.GenerateToken(user.ID, user.Email, user.IsAdmin == 1, user.Staff == 1, 1)
	if err != nil {
		response.InternalError(c, "生成令牌失败")
		return
	}

	response.Success(c, gin.H{
		"token":      jwtToken,
		"user_id":    user.ID,
		"email":      user.Email,
		"is_admin":   user.IsAdmin,
		"is_staff":   user.Staff == 1,
		"token_type": "Bearer",
	})
}

// GetQuickLoginURL returns a quick-login URL for the current user
func (h *PassportHandler) GetQuickLoginURL(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	url := fmt.Sprintf("/api/v1/passport/auth/quick?token=%s", user.Token)
	response.Success(c, gin.H{"url": url})
}

// Forget handles password reset request
func (h *PassportHandler) Forget(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "邮箱格式错误")
		return
	}

	user, err := h.userSvc.GetUserByEmail(req.Email)
	if err != nil || user == nil {
		response.Success(c, gin.H{"message": "密码重置邮件已发送"})
		return
	}

	// Invalidate any existing reset tokens
	h.tokenSvc.InvalidateUserTokens(user.ID, model.TokenTypePasswordReset)

	// Generate reset token
	resetToken, err := h.tokenSvc.GenerateToken(user.ID, req.Email, model.TokenTypePasswordReset)
	if err != nil {
		response.InternalError(c, "生成重置令牌失败")
		return
	}

	// Send password reset email
	appName, _ := h.setting.Get("app_name")
	appURL, _ := h.setting.Get("app_url")
	if appName == "" {
		appName = "Xboard"
	}
	if appURL == "" {
		appURL = "https://example.com"
	}
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", appURL, resetToken.Token)
	subject := fmt.Sprintf("[%s] 密码重置", appName)
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px;">
	<h2>%s - 密码重置</h2>
	<p>您好，</p>
	<p>请点击下方链接重置您的密码（链接30分钟内有效）：</p>
	<p><a href="%s" style="display: inline-block; background: #EF4444; color: #fff; padding: 12px 24px; border-radius: 6px; text-decoration: none;">重置密码</a></p>
	<p>如果按钮无法点击，请复制以下链接到浏览器：</p>
	<p style="color: #666; word-break: break-all;">%s</p>
	<p>如果您没有请求重置密码，请忽略此邮件。</p>
</body>
</html>`, appName, resetLink, resetLink)

	if err := h.mailSvc.Send(req.Email, subject, body); err != nil {
		h.db.Create(&model.MailLog{
			Email:        req.Email,
			Subject:      subject,
			TemplateName: "password_reset",
			Error:        err.Error(),
		})
		response.InternalError(c, "邮件发送失败")
		return
	}

	h.db.Create(&model.MailLog{
		Email:        req.Email,
		Subject:      subject,
		TemplateName: "password_reset",
	})

	response.Success(c, gin.H{"message": "密码重置邮件已发送"})
}

// ResetPassword confirms password reset with a token
func (h *PassportHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min:8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// Validate the reset token
	loginToken, err := h.tokenSvc.ValidateToken(req.Token, model.TokenTypePasswordReset)
	if err != nil {
		response.BadRequest(c, "重置链接无效或已过期")
		return
	}

	// Update password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		response.InternalError(c, "密码加密失败")
		return
	}

	if err := h.userSvc.UpdateUser(loginToken.UserID, map[string]interface{}{
		"password": string(hashedPassword),
	}); err != nil {
		response.InternalError(c, "密码重置失败")
		return
	}

	// Invalidate all remaining tokens for this user
	h.tokenSvc.InvalidateUserTokens(loginToken.UserID, model.TokenTypePasswordReset)

	response.Success(c, gin.H{"message": "密码重置成功"})
}

// SendEmailVerify sends email verification code
func (h *PassportHandler) SendEmailVerify(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "邮箱格式错误")
		return
	}

	// Verify the user exists and the email matches
	userID, exists := c.Get("user_id")
	if exists {
		uid, _ := userID.(uint)
		var user model.User
		if err := h.db.First(&user, uid).Error; err != nil || user.Email != req.Email {
			response.BadRequest(c, "邮箱与当前用户不匹配")
			return
		}
		// Invalidate old tokens
		h.tokenSvc.InvalidateUserTokens(uid, model.TokenTypeEmailVerify)

		// Generate verification code (6-digit)
		code := h.tokenSvc.GenerateRandomCode(6)

		// Store a token with the code
		loginToken := &model.LoginToken{
			Token:     code,
			UserID:    uid,
			TokenType: model.TokenTypeEmailVerify,
			Email:     req.Email,
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}
		if err := h.db.Create(loginToken).Error; err != nil {
			response.InternalError(c, "生成验证码失败")
			return
		}

		// Send verification email
		appName, _ := h.setting.Get("app_name")
		if appName == "" {
			appName = "Xboard"
		}
		subject := fmt.Sprintf("[%s] 邮箱验证码", appName)
		body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px;">
	<h2>%s - 邮箱验证</h2>
	<p>您的验证码为：</p>
	<p style="font-size: 32px; font-weight: bold; color: #4F46E5; letter-spacing: 8px; text-align: center;">%s</p>
	<p>验证码有效期为10分钟，请勿泄露给他人。</p>
</body>
</html>`, appName, code)

		if err := h.mailSvc.Send(req.Email, subject, body); err != nil {
			h.db.Create(&model.MailLog{
				Email:        req.Email,
				Subject:      subject,
				TemplateName: "email_verify",
				Error:        err.Error(),
			})
			response.InternalError(c, "邮件发送失败")
			return
		}

		h.db.Create(&model.MailLog{
			Email:        req.Email,
			Subject:      subject,
			TemplateName: "email_verify",
		})
	} else {
		// For non-authenticated users (registration verification)
		// Check if email already exists
		existingUser, _ := h.userSvc.GetUserByEmail(req.Email)
		if existingUser != nil {
			response.BadRequest(c, "该邮箱已被注册")
			return
		}
	}

	response.Success(c, gin.H{"message": "验证邮件已发送"})
}

// Pv handles page view tracking
func (h *PassportHandler) Pv(c *gin.Context) {
	response.Success(c, gin.H{"message": "ok"})
}

// ============================================================================
// UserHandler handles authenticated user endpoints
// ============================================================================

type UserHandler struct {
	db          *gorm.DB
	userSvc     *service.UserService
	orderSvc    *service.OrderService
	planSvc     *service.PlanService
	serverSvc   *service.ServerService
	ticketSvc   *service.TicketService
	paymentSvc  *service.PaymentService
}

func NewUserHandler(db *gorm.DB, userSvc *service.UserService, orderSvc *service.OrderService) *UserHandler {
	return &UserHandler{
		db:        db,
		userSvc:   userSvc,
		orderSvc:  orderSvc,
		planSvc:   service.NewPlanService(db),
		serverSvc: service.NewServerService(db),
		ticketSvc: service.NewTicketService(db),
		paymentSvc: service.NewPaymentService(db, nil, orderSvc),
	}
}

func (h *UserHandler) GetInfo(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)
	var user model.User
	if err := h.db.Preload("Plan").First(&user, uid).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	response.Success(c, gin.H{
		"id":                 user.ID,
		"email":              user.Email,
		"plan_id":            user.PlanID,
		"plan":               user.Plan,
		"balance":            user.Balance,
		"transfer_enable":    user.TransferEnable,
		"u":                  user.U,
		"d":                  user.D,
		"expired_at":         user.ExpiredAt.Unix(),
		"banned":             user.Banned,
		"is_admin":           user.IsAdmin,
		"is_staff":           user.Staff == 1,
		"invite_user_id":     user.InviteUserID,
		"commission_balance": user.CommissionBalance,
		"device_limit":       user.DeviceLimit,
		"speed_limit":        user.SpeedLimit,
		"subscribe_url":      user.SubscribeURL,
		"last_login_at":      user.LastLoginAt,
		"created_at":         user.CreatedAt,
	})
}

// GetPlans returns available plans for the user
func (h *UserHandler) GetPlans(c *gin.Context) {
	var plans []model.Plan
	h.db.Where("show = 1 AND enable = 1").Order("sort ASC").Find(&plans)
	response.Success(c, plans)
}

// GetServers returns available servers for the user
func (h *UserHandler) GetServers(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)

	var user model.User
	if err := h.db.First(&user, uid).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	var servers []model.Server
	query := h.db.Where("enable = 1").Order("sort ASC")

	if user.GroupID > 0 {
		query = query.Joins("JOIN v2_server_group_relation ON v2_server.id = v2_server_group_relation.server_id").
			Where("v2_server_group_relation.server_group_id = ?", user.GroupID)
	} else {
		query = query.Where("show = 1")
	}

	query.Find(&servers)
	response.Success(c, servers)
}

// GetPlanDetail returns detail of a single plan
func (h *UserHandler) GetPlanDetail(c *gin.Context) {
	planID := c.Param("id")
	var plan model.Plan
	if err := h.db.Where("id = ? AND show = 1 AND enable = 1", planID).First(&plan).Error; err != nil {
		response.NotFound(c, "套餐不存在")
		return
	}
	response.Success(c, plan)
}

// GetOrders retrieves user's orders
func (h *UserHandler) GetOrders(c *gin.Context) {
	userID, _ := c.Get("user_id")
	orders, _, err := h.orderSvc.GetUserOrders(userID.(uint), 0, 1, 50)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, orders)
}

// CreateOrder creates a new order
func (h *UserHandler) CreateOrder(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		PlanID     uint   `json:"plan_id" binding:"required"`
		Cycle      string `json:"cycle" binding:"required"`
		CouponCode string `json:"coupon_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	order, err := h.orderSvc.CreateOrder(userID.(uint), req.PlanID, req.Cycle, req.CouponCode)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, order)
}

// OrderDetail returns detail of a single order
func (h *UserHandler) OrderDetail(c *gin.Context) {
	userID, _ := c.Get("user_id")
	orderID := c.Param("id")
	var order model.Order
	if err := h.db.Preload("Plan").Preload("Payment").Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
		response.NotFound(c, "订单不存在")
		return
	}
	response.Success(c, order)
}

// OrderCheck checks the status of an order
func (h *UserHandler) OrderCheck(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		TradeNo string `json:"trade_no" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	var order model.Order
	if err := h.db.Where("trade_no = ? AND user_id = ?", req.TradeNo, userID).First(&order).Error; err != nil {
		response.NotFound(c, "订单不存在")
		return
	}
	response.Success(c, gin.H{
		"status":   order.Status,
		"trade_no": order.TradeNo,
	})
}

// GetPaymentMethod returns available payment methods
func (h *UserHandler) GetPaymentMethod(c *gin.Context) {
	var payments []model.Payment
	h.db.Where("enable = 1").Order("sort ASC").Find(&payments)
	response.Success(c, payments)
}

// GetSubscribe returns subscription info/url
func (h *UserHandler) GetSubscribe(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	response.Success(c, gin.H{
		"subscribe_url": user.SubscribeURL,
		"token":         user.Token,
		"uuid":          user.UUID,
		"plan_id":       user.PlanID,
		"expired_at":    user.ExpiredAt.Unix(),
	})
}

// GetStat returns user traffic statistics
func (h *UserHandler) GetStat(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)

	var user model.User
	if err := h.db.First(&user, uid).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	var stats []model.StatUser
	h.db.Where("user_id = ?", uid).Order("recorded_at DESC").Limit(30).Find(&stats)

	response.Success(c, gin.H{
		"u":               user.U,
		"d":               user.D,
		"transfer_enable": user.TransferEnable,
		"used":            user.U + user.D,
		"daily_stats":     stats,
	})
}

// ChangePassword changes user password
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min:8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数格式错误")
		return
	}

	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		response.BadRequest(c, "原密码错误")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		response.InternalError(c, "密码加密失败")
		return
	}

	h.db.Model(&user).Update("password", string(hashedPassword))
	response.Success(c, gin.H{"message": "密码修改成功"})
}

// Update updates user profile
func (h *UserHandler) Update(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		Remarks              string `json:"remarks"`
		SubscribeStatus      *int   `json:"subscribe_status"`
		DeviceLimit          *int   `json:"device_limit"`
		SpeedLimit           *int   `json:"speed_limit"`
		CommissionDisplay    *int   `json:"commission_display"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	updates := make(map[string]interface{})
	if req.Remarks != "" {
		updates["remarks"] = req.Remarks
	}
	if req.SubscribeStatus != nil {
		updates["subscribe_status"] = *req.SubscribeStatus
	}
	if req.DeviceLimit != nil {
		updates["device_limit"] = *req.DeviceLimit
	}
	if req.SpeedLimit != nil {
		updates["speed_limit"] = *req.SpeedLimit
	}
	if req.CommissionDisplay != nil {
		updates["commission_display"] = *req.CommissionDisplay
	}

	if len(updates) > 0 {
		if err := h.userSvc.UpdateUser(userID.(uint), updates); err != nil {
			response.InternalError(c, "更新失败")
			return
		}
	}
	response.Success(c, gin.H{"message": "更新成功"})
}

// ResetSecurity resets user security token/uuid
func (h *UserHandler) ResetSecurity(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)

	var user model.User
	if err := h.db.First(&user, uid).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	newToken := fmt.Sprintf("token_%d_%d", uid, time.Now().UnixNano())
	newUUID := fmt.Sprintf("uuid_%d_%d", uid, time.Now().UnixNano())

	h.db.Model(&user).Updates(map[string]interface{}{
		"token": newToken,
		"uuid":  newUUID,
	})

	response.Success(c, gin.H{
		"token": newToken,
		"uuid":  newUUID,
	})
}

// Transfer transfers balance to another user
func (h *UserHandler) Transfer(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		Email  string  `json:"email" binding:"required,email"`
		Amount float64 `json:"amount" binding:"required,min:0.01"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	var target model.User
	if err := h.db.Where("email = ?", req.Email).First(&target).Error; err != nil {
		response.NotFound(c, "目标用户不存在")
		return
	}

	if target.ID == userID {
		response.BadRequest(c, "不能转账给自己")
		return
	}

	if err := h.userSvc.AddBalance(userID.(uint), -req.Amount); err != nil {
		response.BadRequest(c, "余额不足")
		return
	}

	h.userSvc.AddBalance(target.ID, req.Amount)
	response.Success(c, gin.H{"message": "转账成功"})
}

// GetActiveSession returns user's active login sessions
// Sessions are tracked via JWT token versions in Redis
func (h *UserHandler) GetActiveSession(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)

	deviceSvc := service.NewDeviceStateService(h.db)
	devices, err := deviceSvc.GetDevices(uid)
	if err != nil {
		response.InternalError(c, "获取会话失败")
		return
	}

	sessions := make([]gin.H, 0, len(devices))
	for i, d := range devices {
		sessions = append(sessions, gin.H{
			"id":         fmt.Sprintf("session_%d_%d", d.NodeID, i),
			"node_id":    d.NodeID,
			"ip":         d.IP,
			"last_seen":  d.LastSeen,
		})
	}

	response.Success(c, sessions)
}

// RemoveActiveSession removes a specific device session
func (h *UserHandler) RemoveActiveSession(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	deviceSvc := service.NewDeviceStateService(h.db)

	// Parse session_id format: "session_{nodeID}_{index}"
	var nodeID uint
	var idx int
	if n, _ := fmt.Sscanf(req.SessionID, "session_%d_%d", &nodeID, &idx); n >= 1 {
		// Remove specific device by nodeID
		devices, _ := deviceSvc.GetDevices(uid)
		for _, d := range devices {
			if d.NodeID == nodeID {
				deviceSvc.RemoveDevice(uid, d.NodeID, d.IP)
			}
		}
	} else {
		// Fallback: invalidate all tokens by incrementing token version
		h.db.Model(&model.User{}).Where("id = ?", uid).
			Update("token_version", gorm.Expr("token_version + 1"))
	}

	response.Success(c, gin.H{"message": "会话已移除"})
}

// CheckLogin checks if the login token is valid and returns email
func (h *UserHandler) CheckLogin(c *gin.Context) {
	userID, _ := c.Get("user_id")
	email, _ := c.Get("email")
	response.Success(c, gin.H{
		"valid": true,
		"email": email,
		"user_id": userID,
	})
}

// GetKnowledge returns knowledge base list
func (h *UserHandler) GetKnowledge(c *gin.Context) {
	var knowledge []model.Knowledge
	h.db.Where("show = 1").Order("sort ASC").Find(&knowledge)
	response.Success(c, knowledge)
}

// GetCouponCheck validates a coupon code
func (h *UserHandler) GetCouponCheck(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		CouponCode string `json:"coupon_code" binding:"required"`
		PlanID     uint   `json:"plan_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	coupon, err := service.NewCouponService(h.db).ValidateCoupon(req.CouponCode, userID.(uint), req.PlanID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, coupon)
}

// SaveInviteCode creates new invite codes
func (h *UserHandler) SaveInviteCode(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		Count int `json:"count" binding:"required,min:1,max:100"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	codes := make([]model.InviteCode, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		code := fmt.Sprintf("INVITE_%d_%d_%d", userID, time.Now().UnixNano(), i)
		codes = append(codes, model.InviteCode{
			UserID: userID.(uint),
			Code:   code,
			Status: 0,
		})
	}

	if len(codes) > 0 {
		h.db.Create(&codes)
	}
	response.Success(c, codes)
}

// InviteDetails returns invite statistics/details
func (h *UserHandler) InviteDetails(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)

	var invitedUsers []model.User
	h.db.Where("invite_user_id = ?", uid).Find(&invitedUsers)

	var commissionLogs []model.CommissionLog
	h.db.Where("invite_user_id = ?", uid).Preload("Order").Order("id DESC").Find(&commissionLogs)

	totalCommission := 0.0
	for _, log := range commissionLogs {
		totalCommission += log.GetAmount
	}

	response.Success(c, gin.H{
		"invited_count":     len(invitedUsers),
		"invited_users":     invitedUsers,
		"commission_logs":   commissionLogs,
		"total_commission":  totalCommission,
	})
}

// CreateTicket creates a support ticket
func (h *UserHandler) CreateTicket(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		Subject string `json:"subject" binding:"required"`
		Message string `json:"message" binding:"required"`
		Level   int    `json:"level"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	ticket, err := h.ticketSvc.CreateTicket(userID.(uint), req.Subject, req.Message, req.Level)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, ticket)
}

// ReplyTicket replies to an existing ticket
func (h *UserHandler) ReplyTicket(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		TicketID uint   `json:"ticket_id" binding:"required"`
		Message  string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	var ticket model.Ticket
	if err := h.db.Where("id = ? AND user_id = ?", req.TicketID, userID).First(&ticket).Error; err != nil {
		response.NotFound(c, "工单不存在")
		return
	}

	msg := model.TicketMessage{
		TicketID: req.TicketID,
		UserID:   userID.(uint),
		Message:  req.Message,
	}
	if err := h.db.Create(&msg).Error; err != nil {
		response.InternalError(c, "回复失败")
		return
	}
	response.Success(c, msg)
}

// CloseTicket closes a support ticket
func (h *UserHandler) CloseTicket(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		TicketID uint `json:"ticket_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	result := h.db.Model(&model.Ticket{}).
		Where("id = ? AND user_id = ?", req.TicketID, userID).
		Update("status", 1)
	if result.RowsAffected == 0 {
		response.NotFound(c, "工单不存在")
		return
	}
	response.Success(c, gin.H{"message": "工单已关闭"})
}

// WithdrawTicket withdraws (reopens) a closed ticket
func (h *UserHandler) WithdrawTicket(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		TicketID uint `json:"ticket_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	result := h.db.Model(&model.Ticket{}).
		Where("id = ? AND user_id = ? AND status = 1", req.TicketID, userID).
		Update("status", 0)
	if result.RowsAffected == 0 {
		response.NotFound(c, "工单不存在或未关闭")
		return
	}
	response.Success(c, gin.H{"message": "工单已重新开启"})
}

// GetTickets returns user's tickets
func (h *UserHandler) GetTickets(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var tickets []model.Ticket
	h.db.Where("user_id = ?", userID).Preload("Messages").Order("id DESC").Find(&tickets)
	response.Success(c, tickets)
}

// GetInviteCodes returns user's invite codes
func (h *UserHandler) GetInviteCodes(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var codes []model.InviteCode
	h.db.Where("user_id = ?", userID).Find(&codes)
	response.Success(c, codes)
}

// GetNotices returns system notices
func (h *UserHandler) GetNotices(c *gin.Context) {
	var notices []model.Notice
	h.db.Where("show = 1").Order("sort ASC").Find(&notices)
	response.Success(c, notices)
}

// GetConfig returns user-specific config (e.g. stripe key)
func (h *UserHandler) GetConfig(c *gin.Context) {
	stripeKey, _ := service.NewSettingService(h.db).Get("stripe_publishable_key")
	response.Success(c, gin.H{
		"stripe_publishable_key": stripeKey,
	})
}

// ============================================================================
// GuestHandler handles public/guest endpoints
// ============================================================================

type GuestHandler struct {
	db      *gorm.DB
	setting *service.SettingService
}

func NewGuestHandler(db *gorm.DB, setting *service.SettingService) *GuestHandler {
	return &GuestHandler{db: db, setting: setting}
}

func (h *GuestHandler) GetPlans(c *gin.Context) {
	var plans []model.Plan
	h.db.Where("show = 1 AND enable = 1").Order("sort ASC").Find(&plans)
	response.Success(c, plans)
}

func (h *GuestHandler) GetAlerts(c *gin.Context) {
	response.Success(c, []interface{}{})
}

func (h *GuestHandler) GetPaymentMethods(c *gin.Context) {
	var payments []model.Payment
	h.db.Where("enable = 1").Order("sort ASC").Find(&payments)
	response.Success(c, payments)
}

// GetConfig returns public/guest config
func (h *GuestHandler) GetConfig(c *gin.Context) {
	appName, _ := h.setting.Get("app_name")
	appDescription, _ := h.setting.Get("app_description")
	appURL, _ := h.setting.Get("app_url")
	stopRegister, _ := h.setting.GetBool("stop_register")
	inviteForce, _ := h.setting.GetBool("invite_force")

	response.Success(c, gin.H{
		"app_name":        appName,
		"app_description": appDescription,
		"app_url":         appURL,
		"stop_register":   stopRegister,
		"invite_force":    inviteForce,
	})
}

// ============================================================================
// ClientHandler handles subscription endpoints
// ============================================================================

type ClientHandler struct {
	db *gorm.DB
}

func NewClientHandler(db *gorm.DB) *ClientHandler {
	return &ClientHandler{db: db}
}

func (h *ClientHandler) Subscribe(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		token = c.Query("token")
	}
	if token == "" {
		c.String(http.StatusBadRequest, "invalid token")
		return
	}

	var user model.User
	if err := h.db.Where("token = ?", token).First(&user).Error; err != nil {
		c.String(http.StatusNotFound, "user not found")
		return
	}

	ua := c.GetHeader("User-Agent")
	flag := detectClientFlag(ua)

	var servers []model.Server
	h.db.Where("show = 1 AND enable = 1").Find(&servers)

	clientInfo := &protocol.ClientInfo{
		ID:             user.ID,
		UUID:           user.UUID,
		SpeedLimit:     user.SpeedLimit,
		DeviceLimit:    user.DeviceLimit,
		Email:          user.Email,
		TransferEnable: user.TransferEnable,
		U:              user.U,
		D:              user.D,
		ExpiredAt:      user.ExpiredAt.Unix(),
	}

	proto := protocol.MatchProtocolFromGlobal(flag)
	if proto == nil {
		c.String(http.StatusOK, "ss://"+token+"@"+flag)
		return
	}

	result := ""
	for _, sv := range servers {
		sc := &protocol.ServerConfig{
			ID:            sv.ID,
			Name:          sv.Name,
			Host:          sv.Host,
			Port:          sv.Port,
			ServerPort:    sv.ServerPort,
			PortRange:     sv.PortRange,
			ServerKey:     sv.ServerKey,
			Cipher:        sv.CIPHER,
			Rate:          parseFloat(sv.Rate),
			Network:       sv.Network,
			Protocol:      sv.Protocol,
			TLS:           sv.TLS == 1,
			TLSProvider:   sv.TLSProvider,
			TLSHost:       sv.TLSHost,
			Reality:       sv.Reality == 1,
			Flow:          sv.Flow,
			TrafficUsed:   sv.TrafficUsed,
			TrafficLimit:  sv.TrafficLimit,
		}
		cfg := proto.GenerateConfig(clientInfo, sc)
		line, _ := formatConfigLine(cfg)
		result += line + "\n"
	}

	c.String(http.StatusOK, result)
}

func detectClientFlag(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "clash.meta"), strings.Contains(ua, "clash-verge"), strings.Contains(ua, "clashmeta"):
		return "clash.meta"
	case strings.Contains(ua, "clash"):
		return "clash"
	case strings.Contains(ua, "surge"):
		return "surge"
	case strings.Contains(ua, "quantumult%20x"), strings.Contains(ua, "quantumultx"):
		return "quantumult%20x"
	case strings.Contains(ua, "quantumult"):
		return "quantumult"
	case strings.Contains(ua, "loon"):
		return "loon"
	case strings.Contains(ua, "shadowrocket"), strings.Contains(ua, "rocket"):
		return "shadowrocket"
	case strings.Contains(ua, "stash"):
		return "stash"
	case strings.Contains(ua, "singbox"), strings.Contains(ua, "sing-box"):
		return "sing-box"
	case strings.Contains(ua, "surfboard"):
		return "surfboard"
	case strings.Contains(ua, "v2rayu"), strings.Contains(ua, "v2rayng"):
		return "v2rayng"
	case strings.Contains(ua, "passwall"):
		return "passwall"
	default:
		return "general"
	}
}

func formatConfigLine(cfg interface{}) (string, error) {
	switch v := cfg.(type) {
	case string:
		return v, nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// ============================================================================
// ServerHandler handles node/server communication
// ============================================================================

type ServerHandler struct {
	db *gorm.DB
}

func NewServerHandler(db *gorm.DB) *ServerHandler {
	return &ServerHandler{db: db}
}

func (h *ServerHandler) GetUsers(c *gin.Context) {
	serverID := c.GetUint("server_id")

	// Get server info for group filtering
	var server model.Server
	if serverID > 0 {
		h.db.First(&server, serverID)
	}

	// Query active users with plans
	var users []model.User
	query := h.db.Where("expired_at > ? AND plan_id > 0 AND enabled = 1", time.Now())

	// If server belongs to a group, filter by that group's plans via server_group_relation
	if server.GroupID > 0 {
		var groupServerIDs []uint
		h.db.Model(&model.ServerGroupRelation{}).Where("server_group_id = ?", server.GroupID).Pluck("server_id", &groupServerIDs)
		// All servers in this group share the same user set, no additional filtering needed
	}

	query.Find(&users)

	// Build response with only necessary fields for the node
	type UserForNode struct {
		ID             uint      `json:"id"`
		UUID           string    `json:"uuid"`
		SpeedLimit     int       `json:"speed_limit"`
		TransferEnable int64     `json:"transfer_enable"`
		U              int64     `json:"u"`
		D              int64     `json:"d"`
		ExpiredAt      time.Time `json:"expired_at"`
		PlanID         uint      `json:"plan_id"`
	}

	result := make([]UserForNode, 0, len(users))
	for _, u := range users {
		result = append(result, UserForNode{
			ID:             u.ID,
			UUID:           u.UUID,
			SpeedLimit:     u.SpeedLimit,
			TransferEnable: u.TransferEnable,
			U:              u.U,
			D:              u.D,
			ExpiredAt:      u.ExpiredAt,
			PlanID:         u.PlanID,
		})
	}

	response.Success(c, result)
}

func (h *ServerHandler) PushTraffic(c *gin.Context) {
	var req struct {
		ServerID uint            `json:"server_id"`
		Records  []TrafficRecord `json:"records"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	if len(req.Records) == 0 {
		response.Success(c, gin.H{"message": "ok", "count": 0})
		return
	}

	// Process each traffic record
	for _, rec := range req.Records {
		if rec.UserID == 0 {
			continue
		}

		// Update user traffic in transaction
		err := h.db.Transaction(func(tx *gorm.DB) error {
			var user model.User
			if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, rec.UserID).Error; err != nil {
				return err // skip non-existent users
			}

			// Accumulate traffic
			if err := tx.Model(&user).Updates(map[string]interface{}{
				"u": user.U + rec.U,
				"d": user.D + rec.D,
			}).Error; err != nil {
				return err
			}

			// Check if traffic exceeded
			totalUsed := user.U + rec.U + user.D + rec.D
			if user.TransferEnable > 0 && totalUsed >= user.TransferEnable {
				tx.Model(&user).Update("enabled", 0)
			}

			// Log to stat_user
			statRecord := model.StatUser{
				UserID:     rec.UserID,
				U:          rec.U,
				D:          rec.D,
				RecordedAt: time.Now(),
			}
			tx.Create(&statRecord)

			return nil
		})
			if err != nil {
				continue // skip failed records
			}
	}

	// Update server last_active_at
	if req.ServerID > 0 {
		h.db.Model(&model.Server{}).Where("id = ?", req.ServerID).
			Update("last_active_at", time.Now().Unix())
	}

	response.Success(c, gin.H{"message": "ok", "count": len(req.Records)})
}

type TrafficRecord struct {
	UserID uint  `json:"user_id"`
	U      int64 `json:"u"`
	D      int64 `json:"d"`
}

func (h *ServerHandler) GetConfig(c *gin.Context) {
	serverID := c.GetUint("server_id")
	if serverID == 0 {
		if sv, ok := c.Get("server"); ok {
			if s, ok := sv.(*model.Server); ok {
				serverID = s.ID
			}
		}
	}
	if serverID == 0 {
		response.BadRequest(c, "无法识别节点")
		return
	}

	var server model.Server
	if err := h.db.First(&server, serverID).Error; err != nil {
		response.NotFound(c, "节点不存在")
		return
	}

	response.Success(c, gin.H{
		"id":             server.ID,
		"name":           server.Name,
		"network":        server.Network,
		"protocol":       server.Protocol,
		"host":           server.Host,
		"port":           server.Port,
		"server_port":    server.ServerPort,
		"cipher":         server.CIPHER,
		"rate":           server.Rate,
		"tls":            server.TLS,
		"tls_config":     server.GetTLSConfig(),
		"reality":        server.Reality,
		"reality_config": server.GetRealityConfig(),
		"flow":           server.Flow,
		"dns":            server.DNS,
		"dns_json":       server.DNSJSON,
		"custom_config":  server.CustomConfig,
		"port_range":     server.PortRange,
		"traffic_ratio":  server.TrafficRatio,
		"parent_id":      server.ParentID,
		"route_ids":      server.RouteIDs,
		"group_id":       server.GroupID,
	})
}

// ============================================================================
// PaymentHandler handles payment callbacks
// ============================================================================

type PaymentHandler struct {
	db       *gorm.DB
	setting  *service.SettingService
	orderSvc *service.OrderService
}

func NewPaymentHandler(db *gorm.DB, setting *service.SettingService, orderSvc *service.OrderService) *PaymentHandler {
	return &PaymentHandler{db: db, setting: setting, orderSvc: orderSvc}
}

func (h *PaymentHandler) Notify(c *gin.Context) {
	handle := c.Param("handle")
	if handle == "" {
		c.String(http.StatusBadRequest, "invalid payment handle")
		return
	}

	// Collect all request data (query + form + JSON body)
	rawData := make(map[string]interface{})

	// Query params
	for k, v := range c.Request.URL.Query() {
		if len(v) > 0 {
			rawData[k] = v[0]
		}
	}

	// Form / JSON body
	contentType := c.GetHeader("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var jsonBody map[string]interface{}
		if err := c.ShouldBindJSON(&jsonBody); err == nil {
			for k, v := range jsonBody {
				rawData[k] = v
			}
		}
	} else {
		c.Request.ParseForm()
		for k, v := range c.Request.PostForm {
			if len(v) > 0 {
				rawData[k] = v[0]
			}
		}
	}

	paymentSvc := service.NewPaymentService(h.db, nil, h.orderSvc)
	notification, err := paymentSvc.HandleNotify(handle, rawData)
	if err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}

	if notification != nil && notification.Status == "success" {
		c.String(http.StatusOK, "success")
	} else {
		c.String(http.StatusOK, "fail")
	}
}

// ============================================================================
// OrderHandler handles order/payment flow from client side
// ============================================================================

type OrderHandler struct {
	db         *gorm.DB
	orderSvc   *service.OrderService
	paymentSvc *service.PaymentService
}

func NewOrderHandler(db *gorm.DB, orderSvc *service.OrderService) *OrderHandler {
	return &OrderHandler{db: db, orderSvc: orderSvc}
}

func (h *OrderHandler) Pay(c *gin.Context) {
	orderIDStr := c.Param("id")
	paymentIDStr := c.Param("payment")
	if orderIDStr == "" || paymentIDStr == "" {
		response.BadRequest(c, "参数缺失")
		return
	}

	var orderID, paymentID uint
	fmt.Sscanf(orderIDStr, "%d", &orderID)
	fmt.Sscanf(paymentIDStr, "%d", &paymentID)
	if orderID == 0 || paymentID == 0 {
		response.BadRequest(c, "参数格式错误")
		return
	}

	paymentSvc := h.paymentSvc
	if paymentSvc == nil {
		paymentSvc = service.NewPaymentService(h.db, nil, h.orderSvc)
	}

	result, err := paymentSvc.PayOrder(orderID, paymentID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{
		"type":            result.Type,
		"trade_no":        result.TradeNo,
		"redirect_url":    result.RedirectURL,
		"pay_url":         result.PayURL,
		"qr_code":         result.QRCode,
		"gateway_order_id": result.GatewayOrderID,
	})
}

func (h *OrderHandler) Cancel(c *gin.Context) {
	id := parseOrderID(c.Param("id"))
	if err := h.orderSvc.CloseOrder(id); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// Checkout handles payment checkout for an order
func (h *OrderHandler) Checkout(c *gin.Context) {
	tradeNo := c.PostForm("trade_no")
	method := c.PostForm("method")
	if tradeNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}
	var order model.Order
	if err := h.db.Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
		response.BadRequest(c, "订单不存在")
		return
	}

	if order.Status != 0 {
		response.BadRequest(c, "订单状态不允许支付")
		return
	}

	// Find payment method by handle
	var payment model.Payment
	if method != "" {
		if err := h.db.Where("handle = ? AND enable = 1", method).First(&payment).Error; err != nil {
			response.BadRequest(c, "支付方式不可用")
			return
		}
	} else {
		// Use first enabled payment
		if err := h.db.Where("enable = 1").Order("id ASC").First(&payment).Error; err != nil {
			response.BadRequest(c, "无可用支付方式")
			return
		}
	}

	paymentSvc := h.paymentSvc
	if paymentSvc == nil {
		paymentSvc = service.NewPaymentService(h.db, nil, h.orderSvc)
	}

	result, err := paymentSvc.PayOrder(order.ID, payment.ID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{
		"type":            result.Type,
		"trade_no":        result.TradeNo,
		"redirect_url":    result.RedirectURL,
		"pay_url":         result.PayURL,
		"qr_code":         result.QRCode,
		"gateway_order_id": result.GatewayOrderID,
	})
}

// Check checks the status of an order by trade_no
func (h *OrderHandler) Check(c *gin.Context) {
	tradeNo := c.Query("trade_no")
	if tradeNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}
	var order model.Order
	if err := h.db.Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
		response.BadRequest(c, "订单不存在")
		return
	}
	response.Success(c, gin.H{
		"status":   order.Status,
		"trade_no": order.TradeNo,
	})
}

// Detail returns order details by trade_no
func (h *OrderHandler) Detail(c *gin.Context) {
	tradeNo := c.Query("trade_no")
	if tradeNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}
	var order model.Order
	if err := h.db.Where("trade_no = ?", tradeNo).Preload("Plan").First(&order).Error; err != nil {
		response.BadRequest(c, "订单不存在")
		return
	}
	response.Success(c, order)
}

// CloseOrderByTradeNo cancels an order by trade_no
func (h *OrderHandler) CloseOrderByTradeNo(c *gin.Context) {
	tradeNo := c.PostForm("trade_no")
	if tradeNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}
	var order model.Order
	if err := h.db.Where("trade_no = ? AND status = ?", tradeNo, 1).First(&order).Error; err != nil {
		response.BadRequest(c, "订单不存在或已处理")
		return
	}
	if err := h.orderSvc.CloseOrder(order.ID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// GetQuickLoginURL returns a quick login URL
func (h *UserHandler) GetQuickLoginURL(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var user model.User
	if err := h.db.First(&user, userID.(uint)).Error; err != nil {
		response.NotFound(c)
		return
	}
	response.Success(c, gin.H{"url": "/s/" + user.Token})
}

// ============================================================================
// AppHandler handles client app config/version
// ============================================================================

type AppHandler struct {
	db      *gorm.DB
	setting *service.SettingService
}

func NewAppHandler(db *gorm.DB, setting *service.SettingService) *AppHandler {
	return &AppHandler{db: db, setting: setting}
}

func (h *AppHandler) GetConfig(c *gin.Context) {
	forceUpdate, _ := h.setting.GetBool("force_update")
	version, _ := h.setting.Get("app_version")
	updateURL, _ := h.setting.Get("update_url")

	response.Success(c, gin.H{
		"force_update": forceUpdate,
		"version":      version,
		"update_url":   updateURL,
	})
}

func (h *AppHandler) GetVersion(c *gin.Context) {
	version, _ := h.setting.Get("app_version")
	build, _ := h.setting.Get("app_build")

	response.Success(c, gin.H{
		"version": version,
		"build":   build,
	})
}

// ============================================================================
// PaymentCallbackHandler handles payment gateway callbacks
// ============================================================================

type PaymentCallbackHandler struct {
	db        *gorm.DB
	paymentSvc *service.PaymentService
	orderSvc   *service.OrderService
}

func NewPaymentCallbackHandler(db *gorm.DB, paymentSvc *service.PaymentService, orderSvc *service.OrderService) *PaymentCallbackHandler {
	return &PaymentCallbackHandler{db: db, paymentSvc: paymentSvc, orderSvc: orderSvc}
}

func (h *PaymentCallbackHandler) Handle(c *gin.Context) {
	handle := c.Param("handle")
	if handle == "" {
		response.BadRequest(c, "invalid payment handle")
		return
	}

	// Collect all raw data from request
	rawData := make(map[string]interface{})
	if err := c.ShouldBindJSON(&rawData); err == nil {
		// JSON body
	} else {
		for k, v := range c.Request.PostForm {
			if len(v) > 0 {
				rawData[k] = v[0]
			}
		}
	}

	notification, err := h.paymentSvc.HandleNotify(handle, rawData)
	if err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}

	c.String(http.StatusOK, notification.Status)
}

// ============================================================================
// KnowledgeHandler handles knowledge base access for V1 users
// ============================================================================

type KnowledgeHandler struct {
	db *gorm.DB
}

func NewKnowledgeHandler(db *gorm.DB) *KnowledgeHandler {
	return &KnowledgeHandler{db: db}
}

func (h *KnowledgeHandler) List(c *gin.Context) {
	var knowledge []model.Knowledge
	h.db.Where("show = 1").Order("sort ASC").Find(&knowledge)
	response.Success(c, knowledge)
}

func (h *KnowledgeHandler) Detail(c *gin.Context) {
	id := c.Param("id")
	var knowledge model.Knowledge
	if err := h.db.Where("id = ? AND show = 1", id).First(&knowledge).Error; err != nil {
		response.NotFound(c, "文章不存在")
		return
	}
	response.Success(c, knowledge)
}

// ============================================================================
// CouponHandler handles coupon validation for V1 users
// ============================================================================

type CouponHandler struct {
	db *gorm.DB
}

func NewCouponHandler(db *gorm.DB) *CouponHandler {
	return &CouponHandler{db: db}
}

func (h *CouponHandler) Check(c *gin.Context) {
	var req struct {
		CouponCode string `json:"coupon_code" binding:"required"`
		PlanID     uint   `json:"plan_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// Since we don't have user context in guest context, skip user validation
	couponSvc := service.NewCouponService(h.db)
	coupon, err := couponSvc.ValidateCoupon(req.CouponCode, 0, req.PlanID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, coupon)
}

// ============================================================================
// GiftCardHandler handles gift card operations for V1 users
// ============================================================================

type GiftCardHandler struct {
	db *gorm.DB
}

func NewGiftCardHandler(db *gorm.DB) *GiftCardHandler {
	return &GiftCardHandler{db: db}
}

func (h *GiftCardHandler) Check(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	var card model.GiftCardCode
	if err := h.db.Where("code = ? AND status = 0", req.Code).Preload("Template").First(&card).Error; err != nil {
		response.BadRequest(c, "礼品卡无效或已使用")
		return
	}
	response.Success(c, card)
}

func (h *GiftCardHandler) Redeem(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	var card model.GiftCardCode
	if err := h.db.Where("code = ? AND status = 0", req.Code).Preload("Template").First(&card).Error; err != nil {
		response.BadRequest(c, "礼品卡无效或已使用")
		return
	}

	value := card.Template.Value
	h.db.Transaction(func(tx *gorm.DB) error {
		tx.Model(&card).Updates(map[string]interface{}{
			"status": 1,
			"used_by": userID,
			"used_at": time.Now(),
		})
		tx.Model(&model.User{}).Where("id = ?", userID).
			Update("balance", gorm.Expr("balance + ?", value))
		return nil
	})

	response.Success(c, gin.H{"message": "礼品卡兑换成功", "value": value})
}

// ============================================================================
// TelegramHandler handles Telegram bot integration
// ============================================================================

type TelegramHandler struct {
	db       *gorm.DB
	setting  *service.SettingService
	userSvc  *service.UserService
}

func NewTelegramHandler(db *gorm.DB, setting *service.SettingService, userSvc *service.UserService) *TelegramHandler {
	return &TelegramHandler{db: db, setting: setting, userSvc: userSvc}
}

func (h *TelegramHandler) GetBotInfo(c *gin.Context) {
	botToken, _ := h.setting.Get("telegram_bot_token")
	botUsername, _ := h.setting.Get("telegram_bot_username")
	response.Success(c, gin.H{
		"bot_token":    botToken,
		"bot_username": botUsername,
	})
}

func (h *TelegramHandler) Bind(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)
	var req struct {
		TelegramID int64 `json:"telegram_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// Check if Telegram ID is already bound to another user
	var count int64
	h.db.Model(&model.User{}).Where("telegram_id = ? AND id != ?", req.TelegramID, uid).Count(&count)
	if count > 0 {
		response.BadRequest(c, "该Telegram账号已被其他用户绑定")
		return
	}

	if err := h.userSvc.UpdateUser(uid, map[string]interface{}{
		"telegram_id": req.TelegramID,
	}); err != nil {
		response.InternalError(c, "绑定失败")
		return
	}

	response.Success(c, gin.H{"message": "绑定成功"})
}

// ============================================================================
// StatHandler handles user statistics for V1
// ============================================================================

type StatHandler struct {
	db *gorm.DB
}

func NewStatHandler(db *gorm.DB) *StatHandler {
	return &StatHandler{db: db}
}

func (h *StatHandler) GetUserStats(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)

	var user model.User
	if err := h.db.First(&user, uid).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	var stats []model.StatUser
	h.db.Where("user_id = ?", uid).Order("recorded_at DESC").Limit(30).Find(&stats)

	response.Success(c, gin.H{
		"u":               user.U,
		"d":               user.D,
		"transfer_enable": user.TransferEnable,
		"used":            user.U + user.D,
		"daily_stats":     stats,
	})
}

// ============================================================================
// InviteHandler handles invite operations for V1 users
// ============================================================================

type InviteHandler struct {
	db *gorm.DB
}

func NewInviteHandler(db *gorm.DB) *InviteHandler {
	return &InviteHandler{db: db}
}

func (h *InviteHandler) List(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var codes []model.InviteCode
	h.db.Where("user_id = ?", userID).Find(&codes)
	response.Success(c, codes)
}

func (h *InviteHandler) Details(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)

	var invited []model.User
	h.db.Where("invite_user_id = ?", uid).Find(&invited)

	var logs []model.CommissionLog
	h.db.Where("invite_user_id = ?", uid).Preload("Order").Order("id DESC").Limit(50).Find(&logs)

	totalCommission := 0.0
	for _, l := range logs {
		totalCommission += l.GetAmount
	}

	response.Success(c, gin.H{
		"invited_count":    len(invited),
		"invited_users":    invited,
		"commission_logs":  logs,
		"total_commission": totalCommission,
	})
}

// ============================================================================
// PlanHandler handles plan browsing for V1 users
// ============================================================================

type PlanHandler struct {
	db *gorm.DB
}

func NewPlanHandler(db *gorm.DB) *PlanHandler {
	return &PlanHandler{db: db}
}

func (h *PlanHandler) List(c *gin.Context) {
	var plans []model.Plan
	h.db.Where("show = 1 AND enable = 1").Order("sort ASC").Find(&plans)
	response.Success(c, plans)
}

func (h *PlanHandler) Detail(c *gin.Context) {
	id := c.Param("id")
	var plan model.Plan
	if err := h.db.Where("id = ? AND show = 1 AND enable = 1", id).First(&plan).Error; err != nil {
		response.NotFound(c, "套餐不存在")
		return
	}
	response.Success(c, plan)
}

// ============================================================================
// ServerV1Handler handles server listing for V1 users
// ============================================================================

type ServerV1Handler struct {
	db *gorm.DB
}

func NewServerV1Handler(db *gorm.DB) *ServerV1Handler {
	return &ServerV1Handler{db: db}
}

func (h *ServerV1Handler) List(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	var servers []model.Server
	query := h.db.Where("enable = 1").Order("sort ASC")

	if user.GroupID > 0 {
		query = query.Joins("JOIN v2_server_group_relation ON v2_server.id = v2_server_group_relation.server_id").
			Where("v2_server_group_relation.server_group_id = ?", user.GroupID)
	} else {
		query = query.Where("show = 1")
	}

	query.Find(&servers)
	response.Success(c, servers)
}

func (h *ServerV1Handler) Detail(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")

	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	var server model.Server
	query := h.db.Where("id = ? AND enable = 1", id)

	if user.GroupID > 0 {
		query = query.Joins("JOIN v2_server_group_relation ON v2_server.id = v2_server_group_relation.server_id").
			Where("v2_server_group_relation.server_group_id = ?", user.GroupID)
	}

	if err := query.First(&server).Error; err != nil {
		response.NotFound(c, "节点不存在")
		return
	}
	response.Success(c, server)
}

// ============================================================================
// Utility functions
// ============================================================================

func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func parseOrderID(s string) uint {
	var id uint
	for _, c := range s {
		if c >= '0' && c <= '9' {
			id = id*10 + uint(c-'0')
		} else {
			break
		}
	}
	return id
}
