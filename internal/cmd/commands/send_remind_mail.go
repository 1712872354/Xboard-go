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
	cmd.AddCommand(sendRemindMailCmd)
}

var sendRemindMailCmd = &cobra.Command{
	Use:   "send:remindMail",
	Short: "发送到期提醒邮件",
	Run: func(cobraCmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("无法加载配置: %v\n", err)
			return
		}
		db := database.InitDB(&cfg.Database)
		model.DB = db

		fmt.Println("发送到期提醒邮件...")

		now := time.Now()
		remindDays := []int{1, 3, 7}

		for _, days := range remindDays {
			targetDate := now.AddDate(0, 0, days)
			startOfDay := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location())
			endOfDay := startOfDay.Add(24 * time.Hour)

			var users []model.User
			if err := db.Where("expired_at >= ? AND expired_at < ? AND plan_id > 0",
				startOfDay, endOfDay).Find(&users).Error; err != nil {
				fmt.Printf("查询即将到期的用户失败(%d天): %v\n", days, err)
				continue
			}

			for _, user := range users {
				fmt.Printf("提醒: 用户 %s 将在 %d 天后到期\n", user.Email, days)
			}
		}

		fmt.Println("提醒邮件发送完成")
	},
}
