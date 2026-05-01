package v2

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/pkg/response"
	"gorm.io/gorm"
)

// AdminGiftCardHandler handles gift card management in admin panel
type AdminGiftCardHandler struct {
	db *gorm.DB
}

func NewAdminGiftCardHandler(db *gorm.DB) *AdminGiftCardHandler {
	return &AdminGiftCardHandler{db: db}
}

// Templates returns all gift card templates with pagination
func (h *AdminGiftCardHandler) Templates(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var templates []model.GiftCardTemplate
	var total int64
	h.db.Model(&model.GiftCardTemplate{}).Count(&total)
	offset := (page - 1) * pageSize
	h.db.Order("sort ASC, id DESC").Offset(offset).Limit(pageSize).Find(&templates)
	response.Paginated(c, templates, total, page, pageSize)
}

// CreateTemplate creates a new gift card template
func (h *AdminGiftCardHandler) CreateTemplate(c *gin.Context) {
	var template model.GiftCardTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Create(&template).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, template)
}

// UpdateTemplate updates a gift card template
func (h *AdminGiftCardHandler) UpdateTemplate(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Model(&model.GiftCardTemplate{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// DeleteTemplate deletes a gift card template
func (h *AdminGiftCardHandler) DeleteTemplate(c *gin.Context) {
	id := parseUint(c.Param("id"))
	h.db.Delete(&model.GiftCardTemplate{}, id)
	response.Success(c, gin.H{"message": "模板已删除"})
}

// GenerateCodes generates gift card codes from a template
func (h *AdminGiftCardHandler) GenerateCodes(c *gin.Context) {
	var req struct {
		TemplateID uint `json:"template_id" binding:"required"`
		Count      int  `json:"count" binding:"required,min=1,max=1000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	var template model.GiftCardTemplate
	if err := h.db.First(&template, req.TemplateID).Error; err != nil {
		response.NotFound(c, "模板不存在")
		return
	}

	codes := make([]model.GiftCardCode, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		codeStr := fmt.Sprintf("GC_%d_%d_%d", req.TemplateID, i, time.Now().UnixNano())
		codes = append(codes, model.GiftCardCode{
			TemplateID: req.TemplateID,
			Code:       codeStr,
		})
	}

	if err := h.db.Create(&codes).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, gin.H{
		"count": len(codes),
		"codes": codes,
	})
}

// Codes returns all gift card codes with pagination
func (h *AdminGiftCardHandler) Codes(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	templateID := c.Query("template_id")
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var codes []model.GiftCardCode
	var total int64
	query := h.db.Model(&model.GiftCardCode{})

	if templateID != "" {
		query = query.Where("template_id = ?", templateID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	offset := (page - 1) * pageSize
	query.Preload("Template").Order("id DESC").Offset(offset).Limit(pageSize).Find(&codes)
	response.Paginated(c, codes, total, page, pageSize)
}

// ToggleCode toggles the status of a gift card code
func (h *AdminGiftCardHandler) ToggleCode(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var code model.GiftCardCode
	if err := h.db.First(&code, id).Error; err != nil {
		response.NotFound(c, "兑换码不存在")
		return
	}
	newStatus := 1
	if code.Status == 1 {
		newStatus = 0
	}
	h.db.Model(&code).Update("status", newStatus)
	response.Success(c, gin.H{"status": newStatus})
}

// ExportCodes exports codes as CSV
func (h *AdminGiftCardHandler) ExportCodes(c *gin.Context) {
	templateID := c.Query("template_id")

	var codes []model.GiftCardCode
	query := h.db.Model(&model.GiftCardCode{})
	if templateID != "" {
		query = query.Where("template_id = ?", templateID)
	}
	query.Find(&codes)

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=gift_codes.csv")
	var csvData string
	csvData = "Code,TemplateID,Status,CreatedAt\n"
	for _, code := range codes {
		status := "active"
		if code.Status == 1 {
			status = "used"
		}
		csvData += fmt.Sprintf("%s,%d,%s,%s\n", code.Code, code.TemplateID, status, code.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	c.String(200, csvData)
}

// UpdateCode updates a gift card code
func (h *AdminGiftCardHandler) UpdateCode(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	delete(updates, "id")
	if err := h.db.Model(&model.GiftCardCode{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// DeleteCode deletes a gift card code
func (h *AdminGiftCardHandler) DeleteCode(c *gin.Context) {
	id := parseUint(c.Param("id"))
	h.db.Delete(&model.GiftCardCode{}, id)
	response.Success(c, gin.H{"message": "兑换码已删除"})
}

// Usages returns gift card usage records
func (h *AdminGiftCardHandler) Usages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var usages []model.GiftCardUsage
	var total int64
	h.db.Model(&model.GiftCardUsage{}).Count(&total)
	offset := (page - 1) * pageSize
	h.db.Preload("GiftCard").Preload("Order").Order("id DESC").Offset(offset).Limit(pageSize).Find(&usages)
	response.Paginated(c, usages, total, page, pageSize)
}

// Statistics returns gift card statistics
func (h *AdminGiftCardHandler) Statistics(c *gin.Context) {
	var totalTemplates, totalCodes, usedCodes, totalUsage int64
	h.db.Model(&model.GiftCardTemplate{}).Count(&totalTemplates)
	h.db.Model(&model.GiftCardCode{}).Count(&totalCodes)
	h.db.Model(&model.GiftCardCode{}).Where("status = 1").Count(&usedCodes)
	h.db.Model(&model.GiftCardUsage{}).Count(&totalUsage)

	response.Success(c, gin.H{
		"total_templates": totalTemplates,
		"total_codes":     totalCodes,
		"used_codes":      usedCodes,
		"total_usages":    totalUsage,
	})
}

// Types returns gift card types
func (h *AdminGiftCardHandler) Types(c *gin.Context) {
	types := []gin.H{
		{"type": 1, "label": "通用礼品卡"},
		{"type": 2, "label": "套餐礼品卡"},
		{"type": 3, "label": "盲盒礼品卡"},
	}
	response.Success(c, types)
}
