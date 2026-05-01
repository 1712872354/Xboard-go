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
	cmd.AddCommand(clearUserCmd)
}

var clearUserCmd = &cobra.Command{
	Use:   "clear:user",
	Short: "清理过期用户",
	Run: func(cobraCmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("无法加载配置: %v\n", err)
			return
		}
		db := database.InitDB(&cfg.Database)
		model.DB = db

		fmt.Println("清理过期用户...")

		var count int64
		db.Model(&model.User{}).Where("plan_id > 0 AND expired_at < ?", time.Now()).Count(&count)

		result := db.Model(&model.User{}).
			Where("plan_id > 0 AND expired_at < ?", time.Now()).
			Updates(map[string]interface{}{
				"plan_id":         0,
				"u":               0,
				"d":               0,
				"transfer_enable": 0,
			})

		fmt.Printf("清理完成, 共处理 %d 个过期用户 (影响 %d 行)\n", count, result.RowsAffected)
	},
}
