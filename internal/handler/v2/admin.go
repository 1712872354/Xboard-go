package v2

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/internal/service"
	"github.com/xboard/xboard/pkg/response"
	"gorm.io/gorm"
)

// AdminUserHandler handles admin user management
type AdminUserHandler struct {
	db      *gorm.DB
	userSvc *service.UserService
}

func NewAdminUserHandler(db *gorm.DB, userSvc *service.UserService) *AdminUserHandler {
	return &AdminUserHandler{db: db, userSvc: userSvc}
}

func (h *AdminUserHandler) List(c *gin.Context) {
	var users []model.User
	var total int64
	h.db.Model(&model.User{}).Count(&total)
	h.db.Preload("Plan").Order("id DESC").Offset(0).Limit(20).Find(&users)
	response.Paginated(c, users, total, 1, 20)
}

func (h *AdminUserHandler) Get(c *gin.Context) {
	id := c.Param("id")
	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	response.Success(c, user)
}

func (h *AdminUserHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.userSvc.UpdateUser(parseUint(id), updates); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminUserHandler) ToggleBan(c *gin.Context) {
	id := c.Param("id")
	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	newStatus := 1
	if user.Banned == 1 {
		newStatus = 0
	}
	h.db.Model(&user).Update("banned", newStatus)
	response.Success(c, gin.H{"banned": newStatus})
}

func (h *AdminUserHandler) Fetch(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	search := c.Query("search")
	banned := c.Query("banned")
	planID := c.Query("plan_id")
	isAdmin := c.Query("is_admin")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var users []model.User
	var total int64
	query := h.db.Model(&model.User{})

	if search != "" {
		query = query.Where("email LIKE ?", "%"+search+"%")
	}
	if banned != "" {
		query = query.Where("banned = ?", banned)
	}
	if planID != "" {
		query = query.Where("plan_id = ?", planID)
	}
	if isAdmin != "" {
		query = query.Where("is_admin = ?", isAdmin)
	}

	query.Count(&total)
	offset := (page - 1) * pageSize
	query.Preload("Plan").Order("id DESC").Offset(offset).Limit(pageSize).Find(&users)
	response.Paginated(c, users, total, page, pageSize)
}

func (h *AdminUserHandler) Batch(c *gin.Context) {
	var req struct {
		Action string `json:"action" binding:"required"`
		IDs    []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	switch req.Action {
	case "delete":
		h.db.Delete(&model.User{}, req.IDs)
	case "ban":
		h.db.Model(&model.User{}).Where("id IN ?", req.IDs).Update("banned", 1)
	case "unban":
		h.db.Model(&model.User{}).Where("id IN ?", req.IDs).Update("banned", 0)
	default:
		response.BadRequest(c, "不支持的操作")
		return
	}
	response.Success(c, nil)
}

// AdminPlanHandler handles plan management
type AdminPlanHandler struct {
	db      *gorm.DB
	planSvc *service.PlanService
}

func NewAdminPlanHandler(db *gorm.DB, planSvc *service.PlanService) *AdminPlanHandler {
	return &AdminPlanHandler{db: db, planSvc: planSvc}
}

func (h *AdminPlanHandler) List(c *gin.Context) {
	plans, err := h.planSvc.GetAllPlans()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, plans)
}

func (h *AdminPlanHandler) Create(c *gin.Context) {
	var plan model.Plan
	if err := c.ShouldBindJSON(&plan); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.planSvc.CreatePlan(&plan); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, plan)
}

func (h *AdminPlanHandler) Update(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.planSvc.UpdatePlan(id, updates); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminPlanHandler) Delete(c *gin.Context) {
	id := parseUint(c.Param("id"))
	if err := h.planSvc.DeletePlan(id); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.NoContent(c)
}

// AdminServerHandler handles server management
type AdminServerHandler struct {
	db        *gorm.DB
	serverSvc *service.ServerService
}

func NewAdminServerHandler(db *gorm.DB, serverSvc *service.ServerService) *AdminServerHandler {
	return &AdminServerHandler{db: db, serverSvc: serverSvc}
}

func (h *AdminServerHandler) List(c *gin.Context) {
	var servers []model.Server
	h.db.Order("sort ASC").Find(&servers)
	response.Success(c, servers)
}

func (h *AdminServerHandler) Create(c *gin.Context) {
	var server model.Server
	if err := c.ShouldBindJSON(&server); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Create(&server).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, server)
}

func (h *AdminServerHandler) Update(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Model(&model.Server{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminServerHandler) Delete(c *gin.Context) {
	id := parseUint(c.Param("id"))
	if err := h.db.Delete(&model.Server{}, id).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.NoContent(c)
}

func (h *AdminServerHandler) GetConfig(c *gin.Context) {
	id := parseUint(c.Param("id"))
	server, err := h.serverSvc.GetServerByID(id)
	if err != nil {
		response.NotFound(c)
		return
	}
	response.Success(c, server)
}

func (h *AdminServerHandler) Sort(c *gin.Context) {
	var req []struct {
		ID   uint `json:"id"`
		Sort int  `json:"sort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	for _, item := range req {
		h.db.Model(&model.Server{}).Where("id = ?", item.ID).Update("sort", item.Sort)
	}
	response.Success(c, nil)
}

// AdminOrderHandler handles order management
type AdminOrderHandler struct {
	db       *gorm.DB
	orderSvc *service.OrderService
}

func NewAdminOrderHandler(db *gorm.DB, orderSvc *service.OrderService) *AdminOrderHandler {
	return &AdminOrderHandler{db: db, orderSvc: orderSvc}
}

func (h *AdminOrderHandler) List(c *gin.Context) {
	var orders []model.Order
	var total int64
	h.db.Model(&model.Order{}).Count(&total)
	h.db.Preload("User").Preload("Plan").Order("id DESC").Offset(0).Limit(20).Find(&orders)
	response.Paginated(c, orders, total, 1, 20)
}

func (h *AdminOrderHandler) Detail(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var order model.Order
	if err := h.db.Preload("User").Preload("Plan").Preload("Payment").First(&order, id).Error; err != nil {
		response.NotFound(c, "订单不存在")
		return
	}
	response.Success(c, order)
}

func (h *AdminOrderHandler) Update(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Model(&model.Order{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminOrderHandler) Fetch(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	userID := c.Query("user_id")
	planID := c.Query("plan_id")
	tradeNo := c.Query("trade_no")

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
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if planID != "" {
		query = query.Where("plan_id = ?", planID)
	}
	if tradeNo != "" {
		query = query.Where("trade_no LIKE ?", "%"+tradeNo+"%")
	}

	query.Count(&total)
	offset := (page - 1) * pageSize
	query.Preload("User").Preload("Plan").Preload("Payment").Order("id DESC").Offset(offset).Limit(pageSize).Find(&orders)
	response.Paginated(c, orders, total, page, pageSize)
}

// AdminSettingHandler handles system settings
type AdminSettingHandler struct {
	db      *gorm.DB
	setting *service.SettingService
}

func NewAdminSettingHandler(db *gorm.DB, setting *service.SettingService) *AdminSettingHandler {
	return &AdminSettingHandler{db: db, setting: setting}
}

func (h *AdminSettingHandler) GetAll(c *gin.Context) {
	var settings []model.Setting
	h.db.Find(&settings)
	sMap := make(map[string]string)
	for _, s := range settings {
		sMap[s.Key] = s.Value
	}
	response.Success(c, sMap)
}

func (h *AdminSettingHandler) Update(c *gin.Context) {
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

// AdminStatHandler handles statistics
type AdminStatHandler struct {
	statSvc *service.StatisticsService
	db      *gorm.DB
}

func NewAdminStatHandler(statSvc *service.StatisticsService, db *gorm.DB) *AdminStatHandler {
	return &AdminStatHandler{statSvc: statSvc, db: db}
}

func (h *AdminStatHandler) Dashboard(c *gin.Context) {
	stats, err := h.statSvc.GetDashboardStats()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, stats)
}

// AdminCouponHandler handles coupon management
type AdminCouponHandler struct {
	db *gorm.DB
}

func NewAdminCouponHandler(db *gorm.DB) *AdminCouponHandler {
	return &AdminCouponHandler{db: db}
}

func (h *AdminCouponHandler) List(c *gin.Context) {
	var coupons []model.Coupon
	h.db.Order("id DESC").Find(&coupons)
	response.Success(c, coupons)
}

func (h *AdminCouponHandler) Create(c *gin.Context) {
	var coupon model.Coupon
	if err := c.ShouldBindJSON(&coupon); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Create(&coupon).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, coupon)
}

func (h *AdminCouponHandler) Delete(c *gin.Context) {
	id := parseUint(c.Param("id"))
	h.db.Delete(&model.Coupon{}, id)
	response.NoContent(c)
}

// AdminTicketHandler handles ticket management
type AdminTicketHandler struct {
	db *gorm.DB
}

func NewAdminTicketHandler(db *gorm.DB) *AdminTicketHandler {
	return &AdminTicketHandler{db: db}
}

func (h *AdminTicketHandler) List(c *gin.Context) {
	var tickets []model.Ticket
	h.db.Preload("User").Preload("Messages").Order("id DESC").Find(&tickets)
	response.Success(c, tickets)
}

func (h *AdminTicketHandler) Reply(c *gin.Context) {
	type ReplyReq struct {
		TicketID uint   `json:"ticket_id"`
		Message  string `json:"message" binding:"required"`
	}
	var req ReplyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	msg := &model.TicketMessage{
		TicketID: req.TicketID,
		Message:  req.Message,
	}
	h.db.Create(msg)
	response.Created(c, msg)
}

// AdminPluginHandler handles plugin management
type AdminPluginHandler struct {
	db *gorm.DB
}

func NewAdminPluginHandler(db *gorm.DB) *AdminPluginHandler {
	return &AdminPluginHandler{db: db}
}

func (h *AdminPluginHandler) List(c *gin.Context) {
	var plugins []model.Plugin
	h.db.Find(&plugins)
	response.Success(c, plugins)
}

func (h *AdminPluginHandler) Toggle(c *gin.Context) {
	type ToggleReq struct {
		Code      string `json:"code"`
		IsEnabled bool   `json:"is_enabled"`
	}
	var req ToggleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	enabled := 0
	if req.IsEnabled {
		enabled = 1
	}
	h.db.Model(&model.Plugin{}).Where("code = ?", req.Code).Update("is_enabled", enabled)
	response.Success(c, nil)
}

// AdminNotifyHandler handles notification management
type AdminNotifyHandler struct{}

func NewAdminNotifyHandler() *AdminNotifyHandler {
	return &AdminNotifyHandler{}
}

func (h *AdminNotifyHandler) List(c *gin.Context) {
	response.Success(c, []interface{}{})
}

func parseUint(s string) uint {
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

// AdminKnowledgeHandler handles knowledge base management
type AdminKnowledgeHandler struct {
	db *gorm.DB
}

func NewAdminKnowledgeHandler(db *gorm.DB) *AdminKnowledgeHandler {
	return &AdminKnowledgeHandler{db: db}
}

func (h *AdminKnowledgeHandler) List(c *gin.Context) {
	var items []model.Knowledge
	h.db.Order("sort ASC, id DESC").Find(&items)
	response.Success(c, items)
}

func (h *AdminKnowledgeHandler) Create(c *gin.Context) {
	var item model.Knowledge
	if err := c.ShouldBindJSON(&item); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Create(&item).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, item)
}

func (h *AdminKnowledgeHandler) Update(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Model(&model.Knowledge{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminKnowledgeHandler) Delete(c *gin.Context) {
	id := parseUint(c.Param("id"))
	if err := h.db.Delete(&model.Knowledge{}, id).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.NoContent(c)
}

// AdminNoticeHandler handles notice/announcement management
type AdminNoticeHandler struct {
	db *gorm.DB
}

func NewAdminNoticeHandler(db *gorm.DB) *AdminNoticeHandler {
	return &AdminNoticeHandler{db: db}
}

func (h *AdminNoticeHandler) List(c *gin.Context) {
	var items []model.Notice
	h.db.Order("sort ASC, id DESC").Find(&items)
	response.Success(c, items)
}

func (h *AdminNoticeHandler) Create(c *gin.Context) {
	var item model.Notice
	if err := c.ShouldBindJSON(&item); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Create(&item).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, item)
}

func (h *AdminNoticeHandler) Update(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Model(&model.Notice{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminNoticeHandler) Delete(c *gin.Context) {
	id := parseUint(c.Param("id"))
	if err := h.db.Delete(&model.Notice{}, id).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.NoContent(c)
}

// AdminServerGroupHandler handles server group management
type AdminServerGroupHandler struct {
	db *gorm.DB
}

func NewAdminServerGroupHandler(db *gorm.DB) *AdminServerGroupHandler {
	return &AdminServerGroupHandler{db: db}
}

func (h *AdminServerGroupHandler) List(c *gin.Context) {
	var items []model.ServerGroup
	h.db.Preload("Servers").Order("id DESC").Find(&items)
	response.Success(c, items)
}

func (h *AdminServerGroupHandler) Create(c *gin.Context) {
	var item model.ServerGroup
	if err := c.ShouldBindJSON(&item); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Create(&item).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, item)
}

func (h *AdminServerGroupHandler) Update(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Model(&model.ServerGroup{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminServerGroupHandler) Delete(c *gin.Context) {
	id := parseUint(c.Param("id"))
	if err := h.db.Delete(&model.ServerGroup{}, id).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.NoContent(c)
}

// AdminServerRouteHandler handles server route management
type AdminServerRouteHandler struct {
	db *gorm.DB
}

func NewAdminServerRouteHandler(db *gorm.DB) *AdminServerRouteHandler {
	return &AdminServerRouteHandler{db: db}
}

func (h *AdminServerRouteHandler) List(c *gin.Context) {
	var items []model.ServerRoute
	h.db.Order("id DESC").Find(&items)
	response.Success(c, items)
}

func (h *AdminServerRouteHandler) Create(c *gin.Context) {
	var item model.ServerRoute
	if err := c.ShouldBindJSON(&item); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Create(&item).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, item)
}

func (h *AdminServerRouteHandler) Update(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Model(&model.ServerRoute{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminServerRouteHandler) Delete(c *gin.Context) {
	id := parseUint(c.Param("id"))
	if err := h.db.Delete(&model.ServerRoute{}, id).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.NoContent(c)
}

// AdminPaymentHandler handles payment method management
type AdminPaymentHandler struct {
	db *gorm.DB
}

func NewAdminPaymentHandler(db *gorm.DB) *AdminPaymentHandler {
	return &AdminPaymentHandler{db: db}
}

func (h *AdminPaymentHandler) List(c *gin.Context) {
	var items []model.Payment
	h.db.Order("sort ASC, id DESC").Find(&items)
	response.Success(c, items)
}

func (h *AdminPaymentHandler) Create(c *gin.Context) {
	var item model.Payment
	if err := c.ShouldBindJSON(&item); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Create(&item).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, item)
}

func (h *AdminPaymentHandler) Update(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Model(&model.Payment{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminPaymentHandler) Delete(c *gin.Context) {
	id := parseUint(c.Param("id"))
	if err := h.db.Delete(&model.Payment{}, id).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.NoContent(c)
}

func (h *AdminPaymentHandler) Sort(c *gin.Context) {
	var req []struct {
		ID   uint `json:"id"`
		Sort int  `json:"sort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	for _, item := range req {
		h.db.Model(&model.Payment{}).Where("id = ?", item.ID).Update("sort", item.Sort)
	}
	response.Success(c, nil)
}

// AdminMailTemplateHandler handles mail template management
type AdminMailTemplateHandler struct {
	db *gorm.DB
}

func NewAdminMailTemplateHandler(db *gorm.DB) *AdminMailTemplateHandler {
	return &AdminMailTemplateHandler{db: db}
}

func (h *AdminMailTemplateHandler) List(c *gin.Context) {
	var items []model.MailTemplate
	h.db.Order("id DESC").Find(&items)
	response.Success(c, items)
}

func (h *AdminMailTemplateHandler) Create(c *gin.Context) {
	var item model.MailTemplate
	if err := c.ShouldBindJSON(&item); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Create(&item).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, item)
}

func (h *AdminMailTemplateHandler) Update(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Model(&model.MailTemplate{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminMailTemplateHandler) Delete(c *gin.Context) {
	id := parseUint(c.Param("id"))
	if err := h.db.Delete(&model.MailTemplate{}, id).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.NoContent(c)
}

// AdminCommissionHandler handles commission log viewing
type AdminCommissionHandler struct {
	db *gorm.DB
}

func NewAdminCommissionHandler(db *gorm.DB) *AdminCommissionHandler {
	return &AdminCommissionHandler{db: db}
}

func (h *AdminCommissionHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var items []model.CommissionLog
	var total int64
	query := h.db.Model(&model.CommissionLog{})

	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	offset := (page - 1) * pageSize
	query.Preload("InviteUser").Preload("Order").Order("id DESC").Offset(offset).Limit(pageSize).Find(&items)
	response.Paginated(c, items, total, page, pageSize)
}

// AdminAuditLogHandler handles admin audit log viewing
type AdminAuditLogHandler struct {
	db *gorm.DB
}

func NewAdminAuditLogHandler(db *gorm.DB) *AdminAuditLogHandler {
	return &AdminAuditLogHandler{db: db}
}

func (h *AdminAuditLogHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	action := c.Query("action")
	userID := c.Query("user_id")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var items []model.AdminAuditLog
	var total int64
	query := h.db.Model(&model.AdminAuditLog{})

	if action != "" {
		query = query.Where("action = ?", action)
	}
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	query.Count(&total)
	offset := (page - 1) * pageSize
	query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&items)
	response.Paginated(c, items, total, page, pageSize)
}

// AdminThemeHandler handles theme management
type AdminThemeHandler struct {
	db *gorm.DB
}

func NewAdminThemeHandler(db *gorm.DB) *AdminThemeHandler {
	return &AdminThemeHandler{db: db}
}

func (h *AdminThemeHandler) List(c *gin.Context) {
	themeDir := "theme"
	entries, err := os.ReadDir(themeDir)
	if err != nil {
		response.Success(c, []interface{}{})
		return
	}

	var themes []gin.H
	for _, entry := range entries {
		if entry.IsDir() {
			themes = append(themes, gin.H{
				"name": entry.Name(),
			})
		}
	}

	if themes == nil {
		themes = []gin.H{}
	}
	response.Success(c, themes)
}

func (h *AdminThemeHandler) Switch(c *gin.Context) {
	var req struct {
		Theme string `json:"theme" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	themeDir := filepath.Join("theme", req.Theme)
	if _, err := os.Stat(themeDir); os.IsNotExist(err) {
		response.BadRequest(c, "主题不存在")
		return
	}

	// Store active theme in settings
	h.db.Model(&model.Setting{}).Where("`key` = ?", "theme").Update("value", req.Theme)
	response.Success(c, nil)
}

func (h *AdminThemeHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "请上传文件")
		return
	}

	themeDir := filepath.Join("theme", file.Filename)
	if err := c.SaveUploadedFile(file, themeDir); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"name": file.Filename})
}

func (h *AdminThemeHandler) Delete(c *gin.Context) {
	var req struct {
		Theme string `json:"theme" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	themeDir := filepath.Join("theme", req.Theme)
	if err := os.RemoveAll(themeDir); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.NoContent(c)
}

// AdminBackupHandler handles database backup
type AdminBackupHandler struct {
	db *gorm.DB
}

func NewAdminBackupHandler(db *gorm.DB) *AdminBackupHandler {
	return &AdminBackupHandler{db: db}
}

func (h *AdminBackupHandler) List(c *gin.Context) {
	backupDir := "storage/backups"
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		response.Success(c, []interface{}{})
		return
	}

	var backups []gin.H
	for _, entry := range entries {
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			backups = append(backups, gin.H{
				"name":       entry.Name(),
				"size":       info.Size(),
				"created_at": info.ModTime(),
			})
		}
	}

	if backups == nil {
		backups = []gin.H{}
	}
	response.Success(c, backups)
}

// AdminOnlineUserHandler handles online user monitoring
type AdminOnlineUserHandler struct {
	db *gorm.DB
}

func NewAdminOnlineUserHandler(db *gorm.DB) *AdminOnlineUserHandler {
	return &AdminOnlineUserHandler{db: db}
}

func (h *AdminOnlineUserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var users []model.User
	var total int64

	since := time.Now().Add(-15 * time.Minute)
	query := h.db.Model(&model.User{}).Where("last_login_at > ?", since)
	query.Count(&total)

	offset := (page - 1) * pageSize
	query.Order("last_login_at DESC").Offset(offset).Limit(pageSize).Find(&users)
	response.Paginated(c, users, total, page, pageSize)
}

// AdminSystemHandler handles system information
type AdminSystemHandler struct {
	db *gorm.DB
}

func NewAdminSystemHandler(db *gorm.DB) *AdminSystemHandler {
	return &AdminSystemHandler{db: db}
}

func (h *AdminSystemHandler) Info(c *gin.Context) {
	dbName := h.db.Migrator().CurrentDatabase()

	// Count users
	var userCount int64
	h.db.Model(&model.User{}).Count(&userCount)

	// Count orders
	var orderCount int64
	h.db.Model(&model.Order{}).Count(&orderCount)

	// Count servers
	var serverCount int64
	h.db.Model(&model.Server{}).Count(&serverCount)

	response.Success(c, gin.H{
		"db_name":      dbName,
		"user_count":   userCount,
		"order_count":  orderCount,
		"server_count": serverCount,
		"go_version":   "1.21",
		"time":         time.Now(),
	})
}
