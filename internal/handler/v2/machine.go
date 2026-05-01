package v2

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/internal/service"
	"github.com/xboard/xboard/pkg/response"
	"gorm.io/gorm"
)

// AdminMachineHandler handles server machine management
type AdminMachineHandler struct {
	db       *gorm.DB
	serverSvc *service.ServerService
}

func NewAdminMachineHandler(db *gorm.DB, serverSvc *service.ServerService) *AdminMachineHandler {
	return &AdminMachineHandler{db: db, serverSvc: serverSvc}
}

// List returns all machines with pagination
func (h *AdminMachineHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var machines []model.ServerMachine
	var total int64
	h.db.Model(&model.ServerMachine{}).Count(&total)
	offset := (page - 1) * pageSize
	h.db.Order("sort ASC, id DESC").Offset(offset).Limit(pageSize).Find(&machines)

	// Enrich with node count
	type result struct {
		model.ServerMachine
		NodeCount int64 `json:"node_count"`
	}
	var enriched []result
	for _, m := range machines {
		var nodeCount int64
		h.db.Model(&model.Server{}).Where("machine_id = ?", m.ID).Count(&nodeCount)
		enriched = append(enriched, result{
			ServerMachine: m,
			NodeCount:     nodeCount,
		})
	}

	response.Paginated(c, enriched, total, page, pageSize)
}

// Create adds a new machine
func (h *AdminMachineHandler) Create(c *gin.Context) {
	var machine model.ServerMachine
	if err := c.ShouldBindJSON(&machine); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	machine.Token = generateMachineToken()
	if err := h.db.Create(&machine).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, machine)
}

// Update updates a machine
func (h *AdminMachineHandler) Update(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.db.Model(&model.ServerMachine{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// Delete removes a machine
func (h *AdminMachineHandler) Delete(c *gin.Context) {
	id := parseUint(c.Param("id"))
	h.db.Delete(&model.ServerMachine{}, id)
	response.Success(c, gin.H{"message": "机器已删除"})
}

// ResetToken resets the machine token
func (h *AdminMachineHandler) ResetToken(c *gin.Context) {
	id := parseUint(c.Param("id"))
	newToken := generateMachineToken()
	result := h.db.Model(&model.ServerMachine{}).Where("id = ?", id).Update("token", newToken)
	if result.RowsAffected == 0 {
		response.NotFound(c, "机器不存在")
		return
	}
	response.Success(c, gin.H{"token": newToken})
}

// GetToken returns the machine token
func (h *AdminMachineHandler) GetToken(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var machine model.ServerMachine
	if err := h.db.First(&machine, id).Error; err != nil {
		response.NotFound(c, "机器不存在")
		return
	}
	response.Success(c, gin.H{"token": machine.Token})
}

// InstallCommand returns the install command for the machine
func (h *AdminMachineHandler) InstallCommand(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var machine model.ServerMachine
	if err := h.db.First(&machine, id).Error; err != nil {
		response.NotFound(c, "机器不存在")
		return
	}
	serverURL := "https://example.com"
	command := fmt.Sprintf(
		`curl -sL %s/install.sh | bash -s -- --token=%s --name="%s"`,
		serverURL, machine.Token, machine.Name,
	)
	response.Success(c, gin.H{"command": command})
}

// Nodes returns all servers associated with this machine
func (h *AdminMachineHandler) Nodes(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var servers []model.Server
	h.db.Where("machine_id = ?", id).Find(&servers)
	response.Success(c, servers)
}

// History returns machine load history
func (h *AdminMachineHandler) History(c *gin.Context) {
	id := parseUint(c.Param("id"))
	var history []model.ServerMachineLoadHistory
	h.db.Where("machine_id = ?", id).Order("recorded_at DESC").Limit(50).Find(&history)
	response.Success(c, history)
}

// generateMachineToken generates a random machine token
func generateMachineToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
