package v2

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xboard/xboard/internal/database"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/pkg/response"
	"gorm.io/gorm"
)

// ============================================================================
// V2 ServerHandler — new node communication protocol
// ============================================================================

// V2ServerHandler handles V2 server protocol endpoints
type V2ServerHandler struct {
	db *gorm.DB
}

func NewV2ServerHandler(db *gorm.DB) *V2ServerHandler {
	return &V2ServerHandler{db: db}
}

// Handshake handles V2 node handshake
// Node authenticates with server_key, receives config and session token
func (h *V2ServerHandler) Handshake(c *gin.Context) {
	var req struct {
		ServerKey string `json:"server_key" binding:"required"`
		NodeID    uint   `json:"node_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// Find server by server_key
	var server model.Server
	query := h.db.Where("server_key = ?", req.ServerKey)
	if req.NodeID > 0 {
		query = query.Where("id = ?", req.NodeID)
	}
	if err := query.First(&server).Error; err != nil {
		response.Unauthorized(c, "节点认证失败")
		return
	}

	if server.Enable != 1 {
		response.Forbidden(c, "节点已停用")
		return
	}

	// Generate session token
	sessionToken := generateNodeSessionToken()

	// Store session in Redis
	if rdb := database.Redis; rdb != nil {
		key := fmt.Sprintf("node_session:%s", sessionToken)
		rdb.HSet(c.Request.Context(), key,
			"node_id", server.ID,
			"authenticated_at", time.Now().Unix(),
		)
		rdb.Expire(c.Request.Context(), key, 24*time.Hour)

		// Mark node online
		aliveKey := fmt.Sprintf("node_alive:%d", server.ID)
		rdb.Set(c.Request.Context(), aliveKey, time.Now().Unix(), 60*time.Second)
	}

	// Update server last_active_at
	h.db.Model(&server).Update("last_active_at", time.Now().Unix())

	response.Success(c, gin.H{
		"session_token": sessionToken,
		"node_id":       server.ID,
		"node_name":     server.Name,
		"config":        server,
		"expires_at":    time.Now().Add(24 * time.Hour).Unix(),
	})
}

// Report handles V2 node data reporting
// Node reports traffic, online users, system metrics
func (h *V2ServerHandler) Report(c *gin.Context) {
	var req struct {
		NodeID      uint              `json:"node_id" binding:"required"`
		TrafficData []TrafficRecordV2 `json:"traffic_data"`
		OnlineUsers int               `json:"online_users"`
		System      *SystemMetrics    `json:"system"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// Verify node exists
	var server model.Server
	if err := h.db.First(&server, req.NodeID).Error; err != nil {
		response.NotFound(c, "节点不存在")
		return
	}

	// Process traffic data
	processed := 0
	failed := 0
	if len(req.TrafficData) > 0 {
		for _, record := range req.TrafficData {
			if record.UserID == 0 {
				continue
			}

			err := h.db.Transaction(func(tx *gorm.DB) error {
				var user model.User
				if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, record.UserID).Error; err != nil {
					return err
				}

				newU := user.U + record.U
				newD := user.D + record.D
				updates := map[string]interface{}{
					"u": newU,
					"d": newD,
				}

				// Check traffic limit
				if user.TransferEnable > 0 && (newU+newD) > user.TransferEnable {
					updates["enable"] = 0
				}

				return tx.Model(&user).Updates(updates).Error
			})

			if err != nil {
				failed++
			} else {
				processed++
			}
		}
	}

	// Update server stats
	var totalU, totalD int64
	for _, r := range req.TrafficData {
		totalU += r.U
		totalD += r.D
	}
	h.db.Model(&server).Updates(map[string]interface{}{
		"u":              gorm.Expr("u + ?", totalU),
		"d":              gorm.Expr("d + ?", totalD),
		"last_active_at": time.Now().Unix(),
	})

	// Store system metrics in Redis
	if rdb := database.Redis; rdb != nil && req.System != nil {
		key := fmt.Sprintf("node_status:%d", req.NodeID)
		rdb.HSet(c.Request.Context(), key,
			"cpu", req.System.CPU,
			"mem", req.System.Mem,
			"disk", req.System.Disk,
			"load", req.System.Load,
			"uptime", req.System.Uptime,
			"online_users", req.OnlineUsers,
			"at", time.Now().Unix(),
		)
		rdb.Expire(c.Request.Context(), key, 5*time.Minute)
	}

	// Mark node alive
	if rdb := database.Redis; rdb != nil {
		aliveKey := fmt.Sprintf("node_alive:%d", req.NodeID)
		rdb.Set(c.Request.Context(), aliveKey, time.Now().Unix(), 60*time.Second)
	}

	response.Success(c, gin.H{
		"message":   "ok",
		"processed": processed,
		"failed":    failed,
	})
}

// ============================================================================
// V2 Machine endpoints
// ============================================================================

// V2MachineHandler handles machine-level operations
type V2MachineHandler struct {
	db *gorm.DB
}

func NewV2MachineHandler(db *gorm.DB) *V2MachineHandler {
	return &V2MachineHandler{db: db}
}

// Nodes returns all servers associated with a machine
func (h *V2MachineHandler) Nodes(c *gin.Context) {
	var req struct {
		MachineID uint `json:"machine_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	var servers []model.Server
	h.db.Where("machine_id = ?", req.MachineID).Find(&servers)
	response.Success(c, servers)
}

// Status handles machine-level status reporting
func (h *V2MachineHandler) Status(c *gin.Context) {
	var req struct {
		MachineID    uint  `json:"machine_id" binding:"required"`
		CPU          float64 `json:"cpu"`
		Mem          float64 `json:"mem"`
		Disk         float64 `json:"disk"`
		Load         float64 `json:"load"`
		NetworkIn    int64 `json:"network_in"`
		NetworkOut   int64 `json:"network_out"`
		Uptime       int64 `json:"uptime"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// Store machine status in Redis
	if rdb := database.Redis; rdb != nil {
		key := fmt.Sprintf("machine_status:%d", req.MachineID)
		rdb.HSet(c.Request.Context(), key,
			"cpu", req.CPU,
			"mem", req.Mem,
			"disk", req.Disk,
			"load", req.Load,
			"network_in", req.NetworkIn,
			"network_out", req.NetworkOut,
			"uptime", req.Uptime,
			"at", time.Now().Unix(),
		)
		rdb.Expire(c.Request.Context(), key, 5*time.Minute)
	}

	// Record load history
	h.db.Create(&model.ServerMachineLoadHistory{
		MachineID:  req.MachineID,
		CPU:        req.CPU,
		MemUsed:    int64(req.Mem),
		DiskUsed:   int64(req.Disk),
		NetInSpeed: req.NetworkIn,
		NetOutSpeed: req.NetworkOut,
		RecordedAt: time.Now(),
	})

	response.Success(c, gin.H{"message": "ok"})
}

// ============================================================================
// Types
// ============================================================================

// TrafficRecordV2 represents a V2 traffic record
type TrafficRecordV2 struct {
	UserID uint  `json:"user_id"`
	U      int64 `json:"u"`
	D      int64 `json:"d"`
}

// SystemMetrics represents node system metrics
type SystemMetrics struct {
	CPU    float64 `json:"cpu"`
	Mem    float64 `json:"mem"`
	Disk   float64 `json:"disk"`
	Load   float64 `json:"load"`
	Uptime int64   `json:"uptime"`
}

// generateNodeSessionToken generates a random session token for node
func generateNodeSessionToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
