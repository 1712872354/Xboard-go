package bootstrap

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/xboard/xboard/internal/auth"
	"github.com/xboard/xboard/internal/config"
	"github.com/xboard/xboard/internal/cron"
	"github.com/xboard/xboard/internal/database"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/internal/plugin"
	"github.com/xboard/xboard/internal/queue"
	"github.com/xboard/xboard/internal/router"
	"github.com/xboard/xboard/internal/service"
	"github.com/xboard/xboard/internal/websocket"
	"github.com/xboard/xboard/pkg/response"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type App struct {
	Config        *config.Config
	DB            *gorm.DB
	RDB           *redis.Client
	Auth          *auth.JWTAuth
	Services      *Services
	Handlers      *router.Handlers
	PluginManager *plugin.Manager
	WebSocket     *websocket.Server
	Scheduler     *cron.Scheduler
	QueueEngine   *queue.Engine
	QueuePool     *queue.WorkerPool
	engine        *gin.Engine
}

type Services struct {
	UserSvc    *service.UserService
	OrderSvc   *service.OrderService
	PlanSvc    *service.PlanService
	ServerSvc  *service.ServerService
	PaymentSvc *service.PaymentService
	CouponSvc  *service.CouponService
	SettingSvc *service.SettingService
	StatSvc    *service.StatisticsService
	MailSvc    interface{}
}

func (s *Services) Setting() *service.SettingService { return s.SettingSvc }

func Initialize(cfg *config.Config) *App {
	db := database.InitDB(&cfg.Database)
	model.DB = db

	var rdb *redis.Client
	if cfg.Cache.Driver == "redis" {
		rdb = database.InitRedis(&cfg.Redis)
	}

	// 每次启动都自动迁移，确保表结构最新
	runMigrations(db)

	jwtAuth := auth.NewJWTAuth(cfg.App.Key,
		auth.WithIssuer(cfg.App.Name),
		auth.WithExpiresIn(24*time.Hour),
	)

	settingSvc := service.NewSettingService(db)
	planSvc := service.NewPlanService(db)
	userSvc := service.NewUserService(db, settingSvc)
	couponSvc := service.NewCouponService(db)
	serverSvc := service.NewServerService(db)
	statSvc := service.NewStatisticsService(db)
	orderSvc := service.NewOrderService(db, userSvc, planSvc, couponSvc, plugin.GetHookManager())
	resetSvc := service.NewTrafficResetService(db)
	commissionSvc := service.NewCommissionService(db)

	pluginStore := plugin.NewGORMPluginStore(db)
	pluginCtx := &plugin.Context{
		DB:      db,
		Setting: settingSvc,
	}
	pluginMgr := plugin.NewManager(pluginStore, pluginCtx)
	plugin.InitHookManager()

	paymentSvc := service.NewPaymentService(db, pluginMgr, orderSvc)
	tokenSvc := service.NewTokenService(db)
	mailSvc := service.NewMailServiceFromDB(db)

	services := &Services{
		UserSvc:    userSvc,
		OrderSvc:   orderSvc,
		PlanSvc:    planSvc,
		ServerSvc:  serverSvc,
		PaymentSvc: paymentSvc,
		CouponSvc:  couponSvc,
		SettingSvc: settingSvc,
		StatSvc:    statSvc,
		MailSvc:    mailSvc,
	}

	_ = commissionSvc // used by payment flow via event bus

	// =================== 初始化队列系统 ===================
	qEngine := queue.NewEngine(db, rdb)
	handlerReg := queue.NewHandlerRegistry()
	queue.RegisterAllHandlers(handlerReg, db)
	qPool := queue.NewWorkerPool(qEngine, handlerReg,
		queue.WithMinWorkers(2),
		queue.WithMaxWorkers(20),
	)
	qPool.Start()
	log.Printf("queue: system initialized (engine=redis+mysql, workers=2~20)")

	wsServer := websocket.NewServer(cfg, db, rdb)
	wsServer.Start()

	handlers := router.NewHandlers(db, jwtAuth, userSvc, orderSvc, planSvc, serverSvc, settingSvc, statSvc, tokenSvc, mailSvc)

	engine := setupGin(cfg, handlers, jwtAuth, wsServer, qEngine)
	scheduler := cron.NewScheduler(db, orderSvc, userSvc, statSvc, resetSvc, mailSvc)

	return &App{
		Config:        cfg,
		DB:            db,
		RDB:           rdb,
		Auth:          jwtAuth,
		Services:      services,
		Handlers:      handlers,
		PluginManager: pluginMgr,
		WebSocket:     wsServer,
		Scheduler:     scheduler,
		QueueEngine:   qEngine,
		QueuePool:     qPool,
		engine:        engine,
	}
}

func setupGin(cfg *config.Config, handlers *router.Handlers,
	jwtAuth *auth.JWTAuth, wsServer *websocket.Server, qEngine *queue.Engine) *gin.Engine {

	switch cfg.Server.Mode {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// =================== 队列监控 API 端点 ===================
	admin := r.Group("/api/v2/admin")
	admin.Use(func(c *gin.Context) { c.Next() }) // no auth for monitoring in dev
	{
		admin.GET("/queue/stats", func(c *gin.Context) {
			ctx, cancel := contextWithTimeout(5 * time.Second)
			defer cancel()
			stats, err := qEngine.Stats(ctx)
			if err != nil {
				response.InternalError(c, "获取队列统计失败")
				return
			}
			response.Success(c, stats)
		})

		admin.GET("/queue/metrics", func(c *gin.Context) {
			response.Success(c, qEngine.Metrics().Snapshot())
		})

		admin.POST("/queue/recover", func(c *gin.Context) {
			ctx, cancel := contextWithTimeout(30 * time.Second)
			defer cancel()
			count, err := qEngine.Recover(ctx)
			if err != nil {
				response.InternalError(c, "恢复失败: "+err.Error())
				return
			}
			response.Success(c, gin.H{"recovered": count})
		})
	}

	// Static files
	r.Static("/theme", "./web/theme")
	r.Static("/assets", "./web/assets")
	r.StaticFile("/favicon.ico", "./web/favicon.ico")

	// SPA catch-all
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if len(path) >= 4 && path[:4] == "/api" {
			c.JSON(404, gin.H{"success": false, "message": "接口不存在"})
			return
		}
		if path == "/api/v1/server/ws" {
			c.JSON(404, gin.H{"success": false, "message": "WebSocket 未启用"})
			return
		}
		adminPrefix := "/" + cfg.App.SecurePath
		if strings.HasPrefix(path, adminPrefix) {
			c.File("./web/admin.html")
		} else {
			c.File("./web/index.html")
		}
	})

	wsHandler := func(c *gin.Context) {
		wsServer.HandleConnection(c.Writer, c.Request)
	}
	router.SetupRouter(r, handlers, jwtAuth, nil, wsHandler)

	return r
}

func (app *App) Run() error {
	addr := fmt.Sprintf(":%d", app.Config.Server.Port)
	log.Printf("Xboard %s 启动在 %s", app.Config.App.Version, addr)

	if app.Scheduler != nil {
		app.Scheduler.Start()
	}

	return app.engine.Run(addr)
}

func (app *App) Shutdown() {
	log.Println("关闭服务...")
	if app.QueuePool != nil {
		log.Println("停止队列工作池...")
		app.QueuePool.Stop()
	}
	if app.QueueEngine != nil {
		app.QueueEngine.Close()
	}
	if app.Scheduler != nil {
		app.Scheduler.Stop()
	}
	if app.WebSocket != nil {
		app.WebSocket.Stop()
	}
}

func SeedDefaultAdmin(db *gorm.DB) {
	var count int64
	db.Model(&model.User{}).Where("is_admin = 1").Count(&count)
	if count > 0 {
		return
	}

	pwd, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	db.Create(&model.User{
		Email:    "admin@xboard.dev",
		Password: string(pwd),
		IsAdmin:  1,
		Staff:    1,
	})
	log.Println("默认管理员: admin@xboard.dev / admin123")
}

func shouldMigrate() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--migrate" || arg == "-m" {
			return true
		}
	}
	return false
}

// convertTimestampColumns 将 PHP 版遗留的整型时间戳 (created_at/updated_at) 转换为 datetime
func convertTimestampColumns(db *gorm.DB) {
	var tables []string
	db.Raw("SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME LIKE 'v2_%'").Scan(&tables)

	for _, table := range tables {
		var cols []struct {
			ColumnName string
			DataType   string
		}
		db.Raw("SELECT COLUMN_NAME, DATA_TYPE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME IN ('created_at','updated_at') AND DATA_TYPE IN ('int','bigint')", table).Scan(&cols)

		for _, col := range cols {
			sql := fmt.Sprintf("UPDATE `%s` SET `%s` = FROM_UNIXTIME(`%s`) WHERE `%s` > 0 AND `%s` < 4102444800 AND CHAR_LENGTH(`%s`) < 11", table, col.ColumnName, col.ColumnName, col.ColumnName, col.ColumnName, col.ColumnName)
			if err := db.Exec(sql).Error; err != nil {
				log.Printf("  转换 %s.%s 失败: %v (可忽略)", table, col.ColumnName, err)
			} else {
				log.Printf("  ✓ 转换 %s.%s 整型时间戳为 datetime", table, col.ColumnName)
			}
		}
	}
}

func runMigrations(db *gorm.DB) {
	log.Println("运行数据库迁移...")

	// 兼容 PHP 版遗留数据：将整型时间戳转为 datetime
	convertTimestampColumns(db)

	// Auto-migrate all models in dependency order
	models := []interface{}{
		&model.User{},
		&model.Plan{},
		&model.Order{},
		&model.Payment{},
		&model.Server{},
		&model.ServerGroup{},
		&model.ServerGroupRelation{},
		&model.ServerRoute{},
		&model.ServerLog{},
		&model.ServerStat{},
		&model.ServerMachine{},
		&model.ServerMachineLoadHistory{},
		&model.Setting{},
		&model.Coupon{},
		&model.Ticket{},
		&model.TicketMessage{},
		&model.Knowledge{},
		&model.Notice{},
		&model.InviteCode{},
		&model.CommissionLog{},
		&model.MailLog{},
		&model.MailTemplate{},
		&model.LoginToken{},
		&model.APIToken{},
		&model.Plugin{},
		&model.Stat{},
		&model.StatServer{},
		&model.StatUser{},
		&model.SubscribeTemplate{},
		&model.ThemeConfig{},
		&model.AdminAuditLog{},
		&model.GiftCardTemplate{},
		&model.GiftCardCode{},
		&model.GiftCardUsage{},
		&model.TrafficResetLog{},
		&queue.QueueJobRecord{},
	}

	if err := db.AutoMigrate(models...); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	log.Printf("迁移完成: %d 个表", len(models))
}

func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}
