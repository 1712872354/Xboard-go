package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/xboard/xboard/internal/cmd"
	"github.com/xboard/xboard/internal/config"
	"github.com/xboard/xboard/internal/database"
	"github.com/xboard/xboard/internal/model"
)

func init() {
	cmd.AddCommand(cleanupOnlineCmd)
}

var cleanupOnlineCmd = &cobra.Command{
	Use:   "cleanup:online-status",
	Short: "清理过期在线数据",
	Run: func(cobraCmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("无法加载配置: %v\n", err)
			return
		}
		db := database.InitDB(&cfg.Database)
		model.DB = db

		fmt.Println("清理过期在线数据...")

		result := db.Model(&model.Server{}).
			Where("last_push_at > 0 AND last_push_at < ?", time.Now().Add(-10*time.Minute).Unix()).
			Update("online_count", 0)

		fmt.Printf("在线状态清理完成, 影响 %d 个节点\n", result.RowsAffected)
	},
}
