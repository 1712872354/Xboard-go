package commands

import (
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/xboard/xboard/internal/cmd"
	"github.com/xboard/xboard/internal/config"
	"github.com/xboard/xboard/internal/database"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/internal/websocket"
)

func init() {
	cmd.AddCommand(websocketServerCmd)
}

var websocketServerCmd = &cobra.Command{
	Use:   "node:websocket",
	Short: "启动 WebSocket 服务",
	Run: func(cobraCmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("无法加载配置: %v\n", err)
			return
		}
		db := database.InitDB(&cfg.Database)
		model.DB = db

		var rdb = database.InitRedis(&cfg.Redis)

		wsServer := websocket.NewServer(cfg, db, rdb)
		wsServer.Start()
		defer wsServer.Stop()

		port := cfg.Server.Port
		if port == 0 {
			port = 8080
		}

		http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			wsServer.HandleConnection(w, r)
		})

		addr := fmt.Sprintf(":%d", port)
		log.Printf("WebSocket 服务启动在 %s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalf("服务启动失败: %v", err)
		}
	},
}
