package middleware

import (
	"context"
	"log"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xboard/xboard/internal/auth"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/pkg/response"
)

const (
	CtxUserID    = "user_id"
	CtxUser      = "user"
	CtxIsAdmin   = "is_admin"
	CtxTokenID   = "token_id"
)

// JWTAuth JWT 认证中间件
func JWTAuth(authz *auth.JWTAuth) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			response.Unauthorized(c, "未提供认证令牌")
			c.Abort()
			return
		}

		claims, err := authz.ValidateToken(token)
		if err != nil {
			response.Unauthorized(c, "令牌无效或已过期")
			c.Abort()
			return
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxIsAdmin, claims.IsAdmin)
		c.Set(CtxTokenID, claims.ID)
		c.Next()
	}
}

// OptionalAuth 可选认证 (不强制但解析 token)
func OptionalAuth(authz *auth.JWTAuth) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.Next()
			return
		}
		claims, err := authz.ValidateToken(token)
		if err == nil {
			c.Set(CtxUserID, claims.UserID)
			c.Set(CtxIsAdmin, claims.IsAdmin)
			c.Set(CtxTokenID, claims.ID)
		}
		c.Next()
	}
}

// AdminOnly 管理员权限中间件
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, _ := c.Get(CtxIsAdmin)
		if admin, ok := isAdmin.(bool); !ok || !admin {
			response.Forbidden(c, "无管理权限")
			c.Abort()
			return
		}
		c.Next()
	}
}

// StaffOnly 工作人员权限中间件
func StaffOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get(CtxUserID)
		uid, ok := userID.(uint)
		if !ok {
			response.Forbidden(c)
			c.Abort()
			return
		}

		var user model.User
		if err := model.DB.First(&user, uid).Error; err != nil {
			response.Forbidden(c)
			c.Abort()
			return
		}
		if user.Staff != 1 {
			response.Forbidden(c, "无工作人员权限")
			c.Abort()
			return
		}
		c.Set(CtxUser, &user)
		c.Next()
	}
}

// ServerAuth 服务节点认证中间件
func ServerAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			token = c.Query("token")
		}
		if token == "" {
			response.Unauthorized(c, "未提供节点令牌")
			c.Abort()
			return
		}

		nodeID := c.Param("id")
		if nodeID == "" {
			nodeID = c.Query("id")
		}

		var server model.Server
		if err := model.DB.Where("id = ? AND token = ?", nodeID, token).First(&server).Error; err != nil {
			response.Unauthorized(c, "节点认证失败")
			c.Abort()
			return
		}

		c.Set("server", &server)
		c.Next()
	}
}

// RateLimit 限流中间件
func RateLimit(handler func(c *gin.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if handler != nil && handler(c) {
			c.Abort()
			return
		}
		c.Next()
	}
}

// throttler 内存限流器
type throttler struct {
	mu       sync.Mutex
	visitors map[string]*visitor
}

type visitor struct {
	count     int
	firstSeen time.Time
}

// Throttle 节流中间件 - 基于 IP 和路由的速率限制
func Throttle(requests int, duration time.Duration) gin.HandlerFunc {
	t := &throttler{
		visitors: make(map[string]*visitor),
	}

	// 定期清理过期记录
	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()
		for range ticker.C {
			t.mu.Lock()
			now := time.Now()
			for key, v := range t.visitors {
				if now.Sub(v.firstSeen) > duration {
					delete(t.visitors, key)
				}
			}
			t.mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		key := c.ClientIP() + ":" + c.FullPath()
		t.mu.Lock()
		v, exists := t.visitors[key]
		if !exists {
			t.visitors[key] = &visitor{count: 1, firstSeen: time.Now()}
			t.mu.Unlock()
			c.Next()
			return
		}
		v.count++
		if v.count > requests {
			t.mu.Unlock()
			response.Error(c, http.StatusTooManyRequests, "请求过于频繁，请稍后重试")
			c.Abort()
			return
		}
		t.mu.Unlock()
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	// 优先从 Authorization header 提取
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	// 其次从 query 参数提取
	if token := c.Query("token"); token != "" {
		return token
	}
	return ""
}

// LogMiddleware 请求/响应日志记录中间件
func LogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		method := c.Request.Method
		path := c.Request.URL.Path
		ip := c.ClientIP()

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		log.Printf("[HTTP] %s %s %d %v %s", method, path, status, latency, ip)
	}
}

// SecurityHeaders 安全响应头中间件
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Next()
	}
}

// CorsMiddleware 可配置的 CORS 中间件
func CorsMiddleware(origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := false
		for _, o := range origins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}
		if allowed || len(origins) == 0 {
			if origin != "" {
				c.Header("Access-Control-Allow-Origin", origin)
			} else if len(origins) > 0 {
				c.Header("Access-Control-Allow-Origin", origins[0])
			} else {
				c.Header("Access-Control-Allow-Origin", "*")
			}
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")
			c.Header("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// RecoveryMiddleware panic 恢复中间件，记录堆栈并返回 500
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[PANIC] %v\n%s", err, stackTrace())
				response.InternalError(c, "服务器内部错误")
				c.Abort()
			}
		}()
		c.Next()
	}
}

// stackTrace 获取当前调用堆栈
func stackTrace() string {
	var buf [4096]byte
	n := runtime.Stack(buf[:], false)
	return string(buf[:n])
}

// ClientMiddleware 从 User-Agent 识别客户端类型
func ClientMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ua := c.GetHeader("User-Agent")
		client := detectClient(ua)
		c.Header("X-Client", client)
		c.Set("client", client)
		c.Next()
	}
}

// detectClient 检测客户端类型
func detectClient(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "clash"):
		return "clash"
	case strings.Contains(ua, "surge"):
		return "surge"
	case strings.Contains(ua, "quantumult%20x"):
		return "quantumultx"
	case strings.Contains(ua, "quantumult"):
		return "quantumult"
	case strings.Contains(ua, "shadowrocket"):
		return "shadowrocket"
	case strings.Contains(ua, "stash"):
		return "stash"
	case strings.Contains(ua, "sing-box"):
		return "sing-box"
	case strings.Contains(ua, "hiddify"):
		return "hiddify"
	case strings.Contains(ua, "v2ray") || strings.Contains(ua, "v2fly"):
		return "v2ray"
	case strings.Contains(ua, "nekoray"):
		return "nekoray"
	case strings.Contains(ua, "loon"):
		return "loon"
	case strings.Contains(ua, "egern"):
		return "egern"
	default:
		return "unknown"
	}
}

// UserMiddleware 从上下文加载完整用户对象
func UserMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get(CtxUserID)
		if !exists {
			c.Next()
			return
		}
		uid, ok := userID.(uint)
		if !ok {
			c.Next()
			return
		}
		var user model.User
		if err := model.DB.First(&user, uid).Error; err != nil {
			response.InternalError(c, "用户不存在")
			c.Abort()
			return
		}
		c.Set(CtxUser, &user)
		c.Next()
	}
}

// CheckBannedMiddleware 检查用户是否被封禁
func CheckBannedMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userVal, exists := c.Get(CtxUser)
		if !exists {
			c.Next()
			return
		}
		user, ok := userVal.(*model.User)
		if !ok {
			c.Next()
			return
		}
		if user.Banned == 1 {
			response.Forbidden(c, "账号已被封禁")
			c.Abort()
			return
		}
		c.Next()
	}
}

// ServerV2Auth V2 服务节点认证中间件（基于 server_key）
func ServerV2Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		serverKey := c.GetHeader("X-Server-Key")
		if serverKey == "" {
			serverKey = c.Query("server_key")
		}
		if serverKey == "" {
			response.Unauthorized(c, "未提供节点密钥")
			c.Abort()
			return
		}

		nodeID := c.Param("id")
		if nodeID == "" {
			nodeID = c.Query("id")
		}

		var server model.Server
		if err := model.DB.Where("id = ? AND server_key = ?", nodeID, serverKey).First(&server).Error; err != nil {
			response.Unauthorized(c, "节点认证失败")
			c.Abort()
			return
		}

		c.Set("server", &server)
		c.Next()
	}
}

// TimeoutMiddleware 请求超时中间件
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)

		done := make(chan struct{})
		go func() {
			c.Next()
			close(done)
		}()

		select {
		case <-done:
			return
		case <-ctx.Done():
			response.Error(c, http.StatusGatewayTimeout, "请求超时")
			c.Abort()
		}
	}
}

// MaintenanceMiddleware 维护模式中间件
func MaintenanceMiddleware(getSetting func(string) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		val := getSetting("maintenance_mode")
		if val == "1" {
			response.Error(c, http.StatusServiceUnavailable, "系统正在维护中，请稍后访问")
			c.Abort()
			return
		}
		c.Next()
	}
}
