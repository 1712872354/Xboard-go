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
	flag.Parse()

	if *showVersion {
		fmt.Printf("Xboard Scheduler %s (build: %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("无法加载配置文件: %v", err)
	}

	// 初始化应用（仅使用数据库，不启动 HTTP）
	app := bootstrap.Initialize(cfg)

	// 启动调度器
	if app.Scheduler != nil {
		log.Println("Xboard 调度器已启动")
		app.Scheduler.Start()
	}

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("正在关闭调度器...")
	if app.Scheduler != nil {
		app.Scheduler.Stop()
	}
}
