package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/xboard/xboard/internal/bootstrap"
	"github.com/xboard/xboard/internal/config"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	showVersion := flag.Bool("version", false, "show version")
	migrateOnly := flag.Bool("migrate", false, "run migrations and exit")
	seedDB := flag.Bool("seed", false, "seed default data")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Xboard %s (build: %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("无法加载配置文件 %s: %v", *configPath, err)
	}

	// 初始化应用
	app := bootstrap.Initialize(cfg)

	// 只运行迁移
	if *migrateOnly {
		log.Println("数据库迁移完成")
		return
	}

	// 初始化默认数据
	if *seedDB {
		bootstrap.SeedDefaultAdmin(app.DB)
		log.Println("种子数据创建完成")
	}

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("正在关闭服务...")
		app.Shutdown()
		os.Exit(0)
	}()

	// 启动服务
	if err := app.Run(); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
