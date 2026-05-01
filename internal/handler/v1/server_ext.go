package v1

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/pkg/response"
	"gorm.io/gorm"
)

// ============================================================================
// UniProxy — extended endpoints (alive, alivelist, status, push fix)
// ============================================================================

// Alive handles node liveness reporting
// Node sends heartbeat to indicate it is online
func (h *ServerHandler) Alive(c *gin.Context) {
	serverID := c.GetUint("server_id")
	if serverID == 0 {
		// Try from server object set by middleware
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

	// Update server last_active_at timestamp
	h.db.Model(&model.Server{}).Where("id = ?", serverID).
		Update("last_active_at", time.Now().Unix())

	// Mark node as online in cache (if Redis available)
	if rdb := getRedisClient(); rdb != nil {
		key := fmt.Sprintf("node_alive:%d", serverID)
		rdb.Set(c.Request.Context(), key, time.Now().Unix(), 60*time.Second)
	}

	response.Success(c, gin.H{"message": "ok"})
}

// AliveList returns list of online node IDs
func (h *ServerHandler) AliveList(c *gin.Context) {
	if rdb := getRedisClient(); rdb != nil {
		keys, err := rdb.Keys(c.Request.Context(), "node_alive:*").Result()
		if err == nil && len(keys) > 0 {
			var nodeIDs []uint
			for _, key := range keys {
				var id uint
				fmt.Sscanf(key, "node_alive:%d", &id)
				if id > 0 {
					nodeIDs = append(nodeIDs, id)
				}
			}
			response.Success(c, nodeIDs)
			return
		}
	}

	// Fallback: query servers with recent last_active_at (within 2 minutes)
	var servers []model.Server
	twoMinAgo := time.Now().Add(-2 * time.Minute).Unix()
	h.db.Where("last_active_at > ? AND enable = 1", twoMinAgo).
		Select("id").Find(&servers)

	var nodeIDs []uint
	for _, s := range servers {
		nodeIDs = append(nodeIDs, s.ID)
	}
	response.Success(c, nodeIDs)
}

// Status handles node status reporting (CPU, memory, load, etc.)
func (h *ServerHandler) Status(c *gin.Context) {
	serverID := c.GetUint("server_id")
	if serverID == 0 {
		if sv, ok := c.Get("server"); ok {
			if s, ok := sv.(*model.Server); ok {
				serverID = s.ID
			}
		}
	}

	var req struct {
		CPU     float64 `json:"cpu"`
		Mem     float64 `json:"mem"`
		Disk    float64 `json:"disk"`
		Load    float64 `json:"load"`
		Uptime  int64   `json:"uptime"`
		Version string  `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// Store status in cache
	if rdb := getRedisClient(); rdb != nil {
		key := fmt.Sprintf("node_status:%d", serverID)
		statusData := map[string]interface{}{
			"cpu":     req.CPU,
			"mem":     req.Mem,
			"disk":    req.Disk,
			"load":    req.Load,
			"uptime":  req.Uptime,
			"version": req.Version,
			"at":      time.Now().Unix(),
		}
		rdb.HSet(c.Request.Context(), key, statusData)
		rdb.Expire(c.Request.Context(), key, 5*time.Minute)
	}

	// Update server record
	h.db.Model(&model.Server{}).Where("id = ?", serverID).Updates(map[string]interface{}{
		"last_active_at": time.Now().Unix(),
	})

	response.Success(c, gin.H{"message": "ok"})
}

// PushTrafficFixed replaces the stub PushTraffic with actual flow processing
func (h *ServerHandler) PushTrafficFixed(c *gin.Context) {
	var req struct {
		ServerID uint            `json:"server_id"`
		Records  []TrafficRecordV1 `json:"records"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	if len(req.Records) == 0 {
		response.Success(c, gin.H{"message": "ok", "count": 0})
		return
	}

	processed := 0
	failed := 0

	for _, record := range req.Records {
		if record.UserID == 0 {
			continue
		}

		err := h.db.Transaction(func(tx *gorm.DB) error {
			var user model.User
			if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, record.UserID).Error; err != nil {
				return err
			}

			// Add traffic
			newU := user.U + record.U
			newD := user.D + record.D

			updates := map[string]interface{}{
				"u": newU,
				"d": newD,
			}

			// Check if traffic exceeded
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

	// Update server traffic stats
	if req.ServerID > 0 {
		var totalU, totalD int64
		for _, r := range req.Records {
			totalU += r.U
			totalD += r.D
		}
		h.db.Model(&model.Server{}).Where("id = ?", req.ServerID).
			Updates(map[string]interface{}{
				"u":              gorm.Expr("u + ?", totalU),
				"d":              gorm.Expr("d + ?", totalD),
				"last_active_at": time.Now().Unix(),
			})
	}

	response.Success(c, gin.H{
		"message":   "ok",
		"count":     len(req.Records),
		"processed": processed,
		"failed":    failed,
	})
}

// ============================================================================
// ShadowsocksTidalab endpoints
// ============================================================================

// SSTidalabUser returns user list for Shadowsocks Tidalab nodes
func (h *ServerHandler) SSTidalabUser(c *gin.Context) {
	var users []model.User
	h.db.Where("expired_at > ? AND plan_id > 0", time.Now().Unix()).Find(&users)
	response.Success(c, users)
}

// SSTidalabSubmit receives traffic data from Shadowsocks Tidalab nodes
func (h *ServerHandler) SSTidalabSubmit(c *gin.Context) {
	h.PushTrafficFixed(c)
}

// ============================================================================
// TrojanTidalab endpoints
// ============================================================================

// TrojanTidalabConfig returns node config for Trojan Tidalab
func (h *ServerHandler) TrojanTidalabConfig(c *gin.Context) {
	serverID := c.GetUint("server_id")
	if serverID == 0 {
		if sv, ok := c.Get("server"); ok {
			if s, ok := sv.(*model.Server); ok {
				serverID = s.ID
			}
		}
	}

	var server model.Server
	if err := h.db.First(&server, serverID).Error; err != nil {
		response.NotFound(c, "节点不存在")
		return
	}

	response.Success(c, gin.H{
		"server_id": server.ID,
		"name":      server.Name,
		"host":      server.Host,
		"port":      server.Port,
		"tls":       server.TLS == 1,
	})
}

// TrojanTidalabUser returns user list for Trojan Tidalab nodes
func (h *ServerHandler) TrojanTidalabUser(c *gin.Context) {
	var users []model.User
	h.db.Where("expired_at > ? AND plan_id > 0", time.Now().Unix()).Find(&users)
	response.Success(c, users)
}

// TrojanTidalabSubmit receives traffic data from Trojan Tidalab nodes
func (h *ServerHandler) TrojanTidalabSubmit(c *gin.Context) {
	h.PushTrafficFixed(c)
}
