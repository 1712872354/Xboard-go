package router

import (
	"github.com/gin-gonic/gin"
	"github.com/xboard/xboard/internal/auth"
	v1 "github.com/xboard/xboard/internal/handler/v1"
	v2 "github.com/xboard/xboard/internal/handler/v2"
	"github.com/xboard/xboard/internal/middleware"
	"github.com/xboard/xboard/internal/service"
	"gorm.io/gorm"
)

type Handlers struct {
	Passport  *v1.PassportHandler
	User      *v1.UserHandler
	Guest     *v1.GuestHandler
	Client    *v1.ClientHandler
	Server    *v1.ServerHandler
	Payment   *v1.PaymentHandler
	Order     *v1.OrderHandler
	App       *v1.AppHandler
	// V2 Admin
	AdminUser           *v2.AdminUserHandler
	AdminPlan           *v2.AdminPlanHandler
	AdminServer         *v2.AdminServerHandler
	AdminOrder          *v2.AdminOrderHandler
	AdminSetting        *v2.AdminSettingHandler
	AdminStat           *v2.AdminStatHandler
	AdminCoupon         *v2.AdminCouponHandler
	AdminTicket         *v2.AdminTicketHandler
	AdminPlugin         *v2.AdminPluginHandler
	AdminNotify         *v2.AdminNotifyHandler
	AdminKnowledge      *v2.AdminKnowledgeHandler
	AdminNotice         *v2.AdminNoticeHandler
	AdminServerGroup    *v2.AdminServerGroupHandler
	AdminServerRoute    *v2.AdminServerRouteHandler
	AdminPayment        *v2.AdminPaymentHandler
	AdminMailTemplate   *v2.AdminMailTemplateHandler
	AdminCommission     *v2.AdminCommissionHandler
	AdminAuditLog       *v2.AdminAuditLogHandler
	AdminTheme          *v2.AdminThemeHandler
	AdminBackup         *v2.AdminBackupHandler
	AdminOnlineUser     *v2.AdminOnlineUserHandler
	AdminSystem         *v2.AdminSystemHandler
	AdminConfig         *v2.AdminConfigHandler
	AdminMachine        *v2.AdminMachineHandler
	AdminGiftCard       *v2.AdminGiftCardHandler
	AdminTrafficReset   *v2.AdminTrafficResetHandler
	// V1 additional handlers
	PaymentCallback *v1.PaymentCallbackHandler
	Knowledge       *v1.KnowledgeHandler
	Coupon          *v1.CouponHandler
	GiftCard        *v1.GiftCardHandler
	Telegram        *v1.TelegramHandler
	Stat            *v1.StatHandler
	Invite          *v1.InviteHandler
	Plan            *v1.PlanHandler
	ServerV1        *v1.ServerV1Handler
	// V2 additional handlers
	V2Server   *v2.V2ServerHandler
	UserToken  *v2.UserTokenHandler
	TgWebhook  *v1.TelegramWebhookHandler
}

func NewHandlers(
	db *gorm.DB,
	authz *auth.JWTAuth,
	userSvc *service.UserService,
	orderSvc *service.OrderService,
	planSvc *service.PlanService,
	serverSvc *service.ServerService,
	settingSvc *service.SettingService,
	statSvc *service.StatisticsService,
	tokenSvc *service.TokenService,
	mailSvc *service.MailService,
) *Handlers {
	return &Handlers{
		Passport:  v1.NewPassportHandler(db, authz, userSvc, settingSvc, tokenSvc, mailSvc),
		User:      v1.NewUserHandler(db, userSvc, orderSvc),
		Guest:     v1.NewGuestHandler(db, settingSvc),
		Client:    v1.NewClientHandler(db),
		Server:    v1.NewServerHandler(db),
		Payment:   v1.NewPaymentHandler(db, settingSvc, orderSvc),
		Order:     v1.NewOrderHandler(db, orderSvc),
		App:       v1.NewAppHandler(db, settingSvc),
		AdminUser:        v2.NewAdminUserHandler(db, userSvc),
		AdminPlan:        v2.NewAdminPlanHandler(db, planSvc),
		AdminServer:      v2.NewAdminServerHandler(db, serverSvc),
		AdminOrder:       v2.NewAdminOrderHandler(db, orderSvc),
		AdminSetting:     v2.NewAdminSettingHandler(db, settingSvc),
		AdminStat:        v2.NewAdminStatHandler(statSvc, db),
		AdminCoupon:      v2.NewAdminCouponHandler(db),
		AdminTicket:      v2.NewAdminTicketHandler(db),
		AdminPlugin:      v2.NewAdminPluginHandler(db),
		AdminNotify:      v2.NewAdminNotifyHandler(),
		AdminKnowledge:   v2.NewAdminKnowledgeHandler(db),
		AdminNotice:      v2.NewAdminNoticeHandler(db),
		AdminServerGroup: v2.NewAdminServerGroupHandler(db),
		AdminServerRoute: v2.NewAdminServerRouteHandler(db),
		AdminPayment:     v2.NewAdminPaymentHandler(db),
		AdminMailTemplate: v2.NewAdminMailTemplateHandler(db),
		AdminCommission:  v2.NewAdminCommissionHandler(db),
		AdminAuditLog:    v2.NewAdminAuditLogHandler(db),
		AdminTheme:       v2.NewAdminThemeHandler(db),
		AdminBackup:      v2.NewAdminBackupHandler(db),
		AdminOnlineUser:  v2.NewAdminOnlineUserHandler(db),
		AdminSystem:      v2.NewAdminSystemHandler(db),
		AdminConfig:      v2.NewAdminConfigHandler(db, settingSvc, mailSvc),
		AdminMachine:     v2.NewAdminMachineHandler(db, serverSvc),
		AdminGiftCard:    v2.NewAdminGiftCardHandler(db),
		AdminTrafficReset: v2.NewAdminTrafficResetHandler(db),
		PaymentCallback:  v1.NewPaymentCallbackHandler(db, service.NewPaymentService(db, nil, orderSvc), orderSvc),
		Knowledge:        v1.NewKnowledgeHandler(db),
		Coupon:           v1.NewCouponHandler(db),
		GiftCard:         v1.NewGiftCardHandler(db),
		Telegram:         v1.NewTelegramHandler(db, settingSvc, userSvc),
		Stat:             v1.NewStatHandler(db),
		Invite:           v1.NewInviteHandler(db),
		Plan:             v1.NewPlanHandler(db),
		ServerV1:         v1.NewServerV1Handler(db),
		V2Server:         v2.NewV2ServerHandler(db),
		UserToken:        v2.NewUserTokenHandler(db),
		TgWebhook:        v1.NewTelegramWebhookHandler(db, "", 0),
	}
}

// SetupRouter registers ALL API routes matching the PHP original path conventions
func SetupRouter(
	r *gin.Engine,
	h *Handlers,
	authz *auth.JWTAuth,
	settingSvc *service.SettingService,
	wsHandler func(c *gin.Context),
) {
	apiV1 := r.Group("/api/v1")
	apiV2 := r.Group("/api/v2")

	// =================== V1 Guest (公开) ===================
	guest := apiV1.Group("/guest")
	{
		guest.GET("/plan/fetch", h.Guest.GetPlans)
		guest.GET("/comm/config", h.Guest.GetConfig)
		guest.GET("/paymentMethods", h.Guest.GetPaymentMethods)
		guest.GET("/notice/fetch", h.Guest.GetAlerts)
	}

	// =================== V1 Payment Notify (公开) ===================
	apiV1.POST("/guest/payment/notify/:method/:uuid", h.PaymentCallback.Handle)

	// =================== V1 Passport (认证) ===================
	passport := apiV1.Group("/passport")
	{
		passport.POST("/auth/login", h.Passport.Login)
		passport.POST("/auth/register", h.Passport.Register)
		passport.GET("/auth/token2Login", h.Passport.Token2Login)
		passport.POST("/auth/forget", h.Passport.Forget)
		passport.POST("/auth/resetPassword", h.Passport.ResetPassword)
		passport.POST("/auth/getQuickLoginUrl", middleware.JWTAuth(authz), h.Passport.GetQuickLoginURL)
		passport.POST("/auth/loginWithMailLink", h.Passport.LoginWithMailLink)
		passport.POST("/comm/sendEmailVerify", middleware.JWTAuth(authz), h.Passport.SendEmailVerify)
		passport.POST("/comm/pv", h.Passport.Pv)
	}

	// =================== V1 User (需登录) ===================
	user := apiV1.Group("/user").Use(middleware.JWTAuth(authz))
	{
		user.GET("/info", h.User.GetInfo)
		user.POST("/changePassword", h.User.ChangePassword)
		user.POST("/update", h.User.Update)
		user.GET("/resetSecurity", h.User.ResetSecurity)
		user.GET("/getSubscribe", h.User.GetSubscribe)
		user.GET("/getStat", h.User.GetStat)
		user.GET("/checkLogin", h.User.CheckLogin)
		user.POST("/transfer", h.User.Transfer)
		user.GET("/getQuickLoginUrl", h.User.GetQuickLoginURL)
		user.GET("/getActiveSession", h.User.GetActiveSession)
		user.POST("/removeActiveSession", h.User.RemoveActiveSession)
		user.GET("/plan/fetch", h.User.GetPlans)
		user.GET("/plan/detail/:id", h.User.GetPlanDetail)
		user.GET("/server", h.User.GetServers)
		user.GET("/server/fetch", h.User.GetServers)

		// Orders
		user.POST("/order/save", h.User.CreateOrder)
		user.POST("/order/checkout", h.Order.Checkout)
		user.GET("/order/check", h.Order.Check)
		user.GET("/order/detail", h.Order.Detail)
		user.GET("/order/fetch", h.User.GetOrders)
		user.GET("/order/getPaymentMethod", h.User.GetPaymentMethod)
		user.POST("/order/cancel", h.Order.CloseOrderByTradeNo)

		// Invite
		user.GET("/invite/fetch", h.User.GetInviteCodes)
		user.POST("/invite/save", h.User.SaveInviteCode)
		user.GET("/invite/details", h.User.InviteDetails)

		// Tickets
		user.GET("/ticket/fetch", h.User.GetTickets)
		user.POST("/ticket/save", h.User.CreateTicket)
		user.POST("/ticket/reply", h.User.ReplyTicket)
		user.POST("/ticket/close", h.User.CloseTicket)
		user.POST("/ticket/withdraw", h.User.WithdrawTicket)

		// Knowledge
		user.GET("/knowledge/fetch", h.User.GetKnowledge)
		user.GET("/knowledge/getCategory", h.User.GetKnowledge)

		// Coupon
		user.POST("/coupon/check", h.User.GetCouponCheck)

		// Gift Card
		user.POST("/gift-card/check", h.GiftCard.Check)
		user.POST("/gift-card/redeem", h.GiftCard.Redeem)

		// Telegram
		user.GET("/telegram/getBotInfo", h.Telegram.GetBotInfo)
		user.POST("/telegram/bind", h.Telegram.Bind)

		// Comm
		user.GET("/comm/config", h.Guest.GetConfig)

		// Notices
		user.GET("/notice/fetch", h.User.GetNotices)

		// Stat
		user.GET("/stat/getTrafficLog", h.Stat.GetUserStats)
	}

	// =================== V1 Server (节点通信) ===================
	server := apiV1.Group("/server")
	{
		server.GET("/:id/user", middleware.ServerAuth(), h.Server.GetUsers)
		server.POST("/:id/submit", middleware.ServerAuth(), h.Server.PushTraffic)
		server.GET("/:id/config", middleware.ServerAuth(), h.Server.GetConfig)

		// UniProxy extended endpoints
		server.GET("/:id/alive", middleware.ServerAuth(), h.Server.Alive)
		server.POST("/:id/alive", middleware.ServerAuth(), h.Server.Alive)
		server.GET("/alivelist", h.Server.AliveList)
		server.GET("/:id/status", middleware.ServerAuth(), h.Server.Status)

		// Shadowsocks/Tidalab/Trojan legacy endpoints
		server.GET("/shadowsocksTidalab/:id/user", middleware.ServerAuth(), h.Server.SSTidalabUser)
		server.POST("/shadowsocksTidalab/:id/submit", middleware.ServerAuth(), h.Server.SSTidalabSubmit)
		server.GET("/trojanTidalab/:id/user", middleware.ServerAuth(), h.Server.TrojanTidalabUser)
		server.POST("/trojanTidalab/:id/submit", middleware.ServerAuth(), h.Server.TrojanTidalabSubmit)
	}

	// =================== V2 Server (新协议) ===================
	serverV2 := apiV2.Group("/server")
	{
		serverV2.POST("/handshake", middleware.ServerV2Auth(), h.V2Server.Handshake)
		serverV2.POST("/report", middleware.ServerV2Auth(), h.V2Server.Report)
	}

	// =================== Telegram Webhook ===================
	apiV1.POST("/telegram/webhook/:token", h.TgWebhook.Handle)

	// =================== API Token (用户长令牌) ===================
	tokenGroup := apiV1.Group("/token").Use(middleware.JWTAuth(authz))
	{
		tokenGroup.GET("/fetch", h.UserToken.List)
		tokenGroup.POST("/save", h.UserToken.Create)
		tokenGroup.POST("/delete", h.UserToken.Delete)
		tokenGroup.POST("/deleteAll", h.UserToken.DeleteAll)
		tokenGroup.GET("/abilities", h.UserToken.Abilities)
	}

	// =================== 订阅路由 ===================
	r.GET("/s/:token", h.Client.Subscribe)
	r.GET("/api/v1/client/subscribe", h.Client.Subscribe)
	apiV1.GET("/client/app/getConfig", h.App.GetConfig)
	apiV1.GET("/client/app/getVersion", h.App.GetVersion)

	// =================== V2 Admin (管理后台) ===================
	admin := apiV2.Group("/admin").Use(middleware.JWTAuth(authz), middleware.AdminOnly())
	{
		// === User Management ===
		admin.GET("/user/fetch", h.AdminUser.Fetch)
		admin.GET("/user/info/:id", h.AdminUser.Get)
		admin.POST("/user/update/:id", h.AdminUser.Update)
		admin.POST("/user/ban/:id", h.AdminUser.ToggleBan)
		admin.POST("/user/generate", h.AdminUser.Generate)
		admin.POST("/user/dumpCSV", h.AdminUser.DumpCSV)
		admin.POST("/user/sendMail", h.AdminUser.SendMail)
		admin.POST("/user/destroy/:id", h.AdminUser.Destroy)
		admin.GET("/user/getUserInfoById", h.AdminUser.GetUserInfoByID)
		admin.POST("/user/resetSecret/:id", h.AdminUser.ResetUserSecret)
		admin.POST("/user/setInviteUser", h.AdminUser.SetInviteUser)
		admin.POST("/user/batch", h.AdminUser.Batch)

		// === Plan Management ===
		admin.GET("/plan/fetch", h.AdminPlan.List)
		admin.POST("/plan/save", h.AdminPlan.Create)
		admin.POST("/plan/update/:id", h.AdminPlan.Update)
		admin.POST("/plan/drop/:id", h.AdminPlan.Delete)
		admin.POST("/plan/sort", h.AdminPlan.Sort)

		// === Server Management ===
		admin.GET("/server/fetch", h.AdminServer.List)
		admin.POST("/server/save", h.AdminServer.Create)
		admin.POST("/server/update/:id", h.AdminServer.Update)
		admin.GET("/server/manage/getNodes", h.AdminServer.List)
		admin.POST("/server/manage/save", h.AdminServer.Create)
		admin.POST("/server/manage/update", h.AdminServer.Update)
		admin.POST("/server/manage/drop/:id", h.AdminServer.Delete)
		admin.POST("/server/manage/copy/:id", h.AdminServer.Copy)
		admin.POST("/server/manage/sort", h.AdminServer.Sort)
		admin.POST("/server/manage/batchDelete", h.AdminServer.BatchDelete)
		admin.POST("/server/manage/batchUpdate", h.AdminServer.BatchUpdate)
		admin.POST("/server/manage/resetTraffic/:id", h.AdminServer.ResetTraffic)
		admin.POST("/server/manage/batchResetTraffic", h.AdminServer.BatchResetTraffic)
		admin.GET("/server/manage/generateEchKey", h.AdminServer.GenerateEchKey)

		// === Server Group ===
		admin.GET("/server/group/fetch", h.AdminServerGroup.List)
		admin.POST("/server/group/save", h.AdminServerGroup.Create)
		admin.POST("/server/group/update/:id", h.AdminServerGroup.Update)
		admin.POST("/server/group/drop/:id", h.AdminServerGroup.Delete)

		// === Server Route ===
		admin.GET("/server/route/fetch", h.AdminServerRoute.List)
		admin.POST("/server/route/save", h.AdminServerRoute.Create)
		admin.POST("/server/route/update/:id", h.AdminServerRoute.Update)
		admin.POST("/server/route/drop/:id", h.AdminServerRoute.Delete)

		// === Machine Management ===
		admin.GET("/server/machine/fetch", h.AdminMachine.List)
		admin.POST("/server/machine/save", h.AdminMachine.Create)
		admin.POST("/server/machine/update/:id", h.AdminMachine.Update)
		admin.POST("/server/machine/drop/:id", h.AdminMachine.Delete)
		admin.POST("/server/machine/resetToken/:id", h.AdminMachine.ResetToken)
		admin.GET("/server/machine/getToken/:id", h.AdminMachine.GetToken)
		admin.GET("/server/machine/installCommand/:id", h.AdminMachine.InstallCommand)
		admin.GET("/server/machine/nodes/:id", h.AdminMachine.Nodes)
		admin.GET("/server/machine/history/:id", h.AdminMachine.History)

		// === Order Management ===
		admin.GET("/order/fetch", h.AdminOrder.Fetch)
		admin.GET("/order/detail/:id", h.AdminOrder.Detail)
		admin.POST("/order/update/:id", h.AdminOrder.Update)
		admin.POST("/order/assign/:id", h.AdminOrder.Assign)
		admin.POST("/order/paid/:id", h.AdminOrder.Paid)
		admin.POST("/order/cancel/:id", h.AdminOrder.Cancel)

		// === Coupon Management ===
		admin.GET("/coupon/fetch", h.AdminCoupon.List)
		admin.POST("/coupon/save", h.AdminCoupon.Create)
		admin.POST("/coupon/drop/:id", h.AdminCoupon.Delete)
		admin.POST("/coupon/show/:id", h.AdminCoupon.Show)
		admin.POST("/coupon/update/:id", h.AdminCoupon.Update)

		// === Ticket Management ===
		admin.GET("/ticket/fetch", h.AdminTicket.List)
		admin.POST("/ticket/reply", h.AdminTicket.Reply)
		admin.POST("/ticket/close", h.AdminTicket.Close)

		// === Setting ===
		admin.GET("/setting/all", h.AdminSetting.GetAll)
		admin.POST("/setting/update", h.AdminSetting.Update)

		// === Config Management ===
		admin.GET("/config/fetch", h.AdminConfig.Fetch)
		admin.POST("/config/save", h.AdminConfig.Save)
		admin.GET("/config/getEmailTemplate", h.AdminConfig.GetEmailTemplate)
		admin.GET("/config/getThemeTemplate", h.AdminConfig.GetThemeTemplate)
		admin.POST("/config/setTelegramWebhook", h.AdminConfig.SetTelegramWebhook)
		admin.POST("/config/testSendMail", h.AdminConfig.TestSendMail)

		// === Statistics ===
		admin.GET("/stat/dashboard", h.AdminStat.Dashboard)
		admin.GET("/stat/getOverride", h.AdminStat.GetOverride)
		admin.GET("/stat/getStats", h.AdminStat.GetStats)
		admin.GET("/stat/getServerLastRank", h.AdminStat.GetServerLastRank)
		admin.GET("/stat/getServerYesterdayRank", h.AdminStat.GetServerYesterdayRank)
		admin.GET("/stat/getOrder", h.AdminStat.GetOrder)
		admin.GET("/stat/getStatUser", h.AdminStat.GetStatUser)
		admin.GET("/stat/getRanking", h.AdminStat.GetRanking)
		admin.GET("/stat/getStatRecord", h.AdminStat.GetStatRecord)
		admin.GET("/stat/getTrafficRank", h.AdminStat.GetTrafficRank)

		// === Notify ===
		admin.GET("/notify/fetch", h.AdminNotify.List)

		// === Plugin Management ===
		admin.GET("/plugin/fetch", h.AdminPlugin.List)
		admin.POST("/plugin/toggle", h.AdminPlugin.Toggle)
		admin.GET("/plugin/types", h.AdminPlugin.Types)
		admin.POST("/plugin/install", h.AdminPlugin.Install)
		admin.POST("/plugin/uninstall", h.AdminPlugin.Uninstall)
		admin.POST("/plugin/upgrade", h.AdminPlugin.Upgrade)
		admin.POST("/plugin/upload", h.AdminPlugin.Upload)
		admin.POST("/plugin/delete", h.AdminPlugin.Delete)
		admin.GET("/plugin/config", h.AdminPlugin.GetConfig)
		admin.POST("/plugin/config", h.AdminPlugin.UpdateConfig)

		// === Knowledge ===
		admin.GET("/knowledge/fetch", h.AdminKnowledge.List)
		admin.POST("/knowledge/save", h.AdminKnowledge.Create)
		admin.POST("/knowledge/update/:id", h.AdminKnowledge.Update)
		admin.POST("/knowledge/show/:id", h.AdminKnowledge.Show)
		admin.POST("/knowledge/drop/:id", h.AdminKnowledge.Delete)
		admin.GET("/knowledge/getCategory", h.AdminKnowledge.GetCategory)
		admin.POST("/knowledge/sort", h.AdminKnowledge.Sort)

		// === Notice ===
		admin.GET("/notice/fetch", h.AdminNotice.List)
		admin.POST("/notice/save", h.AdminNotice.Create)
		admin.POST("/notice/update/:id", h.AdminNotice.Update)
		admin.POST("/notice/drop/:id", h.AdminNotice.Delete)
		admin.POST("/notice/show/:id", h.AdminNotice.Show)
		admin.POST("/notice/sort", h.AdminNotice.Sort)

		// === Payment ===
		admin.GET("/payment/fetch", h.AdminPayment.List)
		admin.POST("/payment/save", h.AdminPayment.Create)
		admin.POST("/payment/update/:id", h.AdminPayment.Update)
		admin.POST("/payment/drop/:id", h.AdminPayment.Delete)
		admin.POST("/payment/sort", h.AdminPayment.Sort)
		admin.GET("/payment/getPaymentMethods", h.AdminPayment.GetPaymentMethods)
		admin.POST("/payment/getPaymentForm/:id", h.AdminPayment.GetPaymentForm)
		admin.POST("/payment/show/:id", h.AdminPayment.Show)

		// === Mail Template ===
		admin.GET("/mail/template/fetch", h.AdminMailTemplate.List)
		admin.POST("/mail/template/save", h.AdminMailTemplate.Create)
		admin.POST("/mail/template/update/:id", h.AdminMailTemplate.Update)
		admin.GET("/mail/template/get/:id", h.AdminMailTemplate.Get)
		admin.POST("/mail/template/reset/:id", h.AdminMailTemplate.Reset)
		admin.POST("/mail/template/test", h.AdminMailTemplate.Test)

		// === Commission ===
		admin.GET("/commission/fetch", h.AdminCommission.List)

		// === Audit Log ===
		admin.GET("/log/audit/fetch", h.AdminAuditLog.List)

		// === Theme ===
		admin.GET("/theme/fetch", h.AdminTheme.List)
		admin.POST("/theme/switch", h.AdminTheme.Switch)
		admin.POST("/theme/upload", h.AdminTheme.Upload)
		admin.POST("/theme/delete", h.AdminTheme.Delete)
		admin.POST("/theme/saveThemeConfig", h.AdminTheme.SaveThemeConfig)
		admin.POST("/theme/getThemeConfig", h.AdminTheme.GetThemeConfig)

		// === Backup ===
		admin.POST("/backup/create", h.AdminBackup.Create)
		admin.GET("/backup/fetch", h.AdminBackup.List)

		// === Online User ===
		admin.GET("/user/online", h.AdminOnlineUser.List)

		// === System ===
		admin.GET("/system/info", h.AdminSystem.Info)
		admin.GET("/system/getSystemStatus", h.AdminSystem.GetSystemStatus)
		admin.GET("/system/getQueueStats", h.AdminSystem.GetQueueStats)
		admin.GET("/system/getQueueWorkload", h.AdminSystem.GetQueueWorkload)
		admin.GET("/system/getHorizonFailedJobs", h.AdminSystem.GetHorizonFailedJobs)

		// === Gift Card ===
		admin.GET("/gift-card/templates", h.AdminGiftCard.Templates)
		admin.POST("/gift-card/create-template", h.AdminGiftCard.CreateTemplate)
		admin.POST("/gift-card/update-template/:id", h.AdminGiftCard.UpdateTemplate)
		admin.POST("/gift-card/delete-template/:id", h.AdminGiftCard.DeleteTemplate)
		admin.POST("/gift-card/generate-codes", h.AdminGiftCard.GenerateCodes)
		admin.GET("/gift-card/codes", h.AdminGiftCard.Codes)
		admin.POST("/gift-card/toggle-code/:id", h.AdminGiftCard.ToggleCode)
		admin.GET("/gift-card/export-codes", h.AdminGiftCard.ExportCodes)
		admin.POST("/gift-card/update-code/:id", h.AdminGiftCard.UpdateCode)
		admin.POST("/gift-card/delete-code/:id", h.AdminGiftCard.DeleteCode)
		admin.GET("/gift-card/usages", h.AdminGiftCard.Usages)
		admin.GET("/gift-card/statistics", h.AdminGiftCard.Statistics)
		admin.GET("/gift-card/types", h.AdminGiftCard.Types)

		// === Traffic Reset ===
		admin.GET("/traffic-reset/logs", h.AdminTrafficReset.Logs)
		admin.GET("/traffic-reset/stats", h.AdminTrafficReset.Stats)
		admin.GET("/traffic-reset/user/:user_id/history", h.AdminTrafficReset.UserHistory)
		admin.POST("/traffic-reset/reset-user", h.AdminTrafficReset.ResetUser)
	}

	// =================== WebSocket ===================
	if wsHandler != nil {
		r.GET("/api/v1/server/ws", wsHandler)
	}
}
