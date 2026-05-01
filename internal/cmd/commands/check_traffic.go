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
	cmd.AddCommand(checkTrafficCmd)
}

var checkTrafficCmd = &cobra.Command{
	Use:   "check:traffic-exceeded",
	Short: "暂停超出流量限制的用户",
	Run: func(cobraCmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("无法加载配置: %v\n", err)
			return
		}
		db := database.InitDB(&cfg.Database)
		model.DB = db

		var users []model.User
		if err := db.Where("u + d > transfer_enable AND expired_at < ? AND plan_id > 0",
			time.Now()).Find(&users).Error; err != nil {
			fmt.Printf("查询用户失败: %v\n", err)
			return
		}

		suspendedCount := 0
		for _, user := range users {
			if err := db.Model(&user).Update("plan_id", 0).Error; err != nil {
				fmt.Printf("暂停用户 %d 失败: %v\n", user.ID, err)
				continue
			}
			fmt.Printf("用户 %d (%s) 因超量暂停\n", user.ID, user.Email)
			suspendedCount++
		}

		fmt.Printf("共暂停 %d 个用户\n", suspendedCount)
	},
}
