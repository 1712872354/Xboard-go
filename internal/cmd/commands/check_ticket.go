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
	cmd.AddCommand(checkTicketCmd)
}

var checkTicketCmd = &cobra.Command{
	Use:   "check:ticket",
	Short: "检查未回复的工单",
	Run: func(cobraCmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("无法加载配置: %v\n", err)
			return
		}
		db := database.InitDB(&cfg.Database)
		model.DB = db

		var tickets []model.Ticket
		if err := db.Where("status = 0").Find(&tickets).Error; err != nil {
			fmt.Printf("查询工单失败: %v\n", err)
			return
		}

		unansweredCount := 0
		for _, ticket := range tickets {
			var lastMsg model.TicketMessage
			if err := db.Where("ticket_id = ?", ticket.ID).Order("id DESC").First(&lastMsg).Error; err != nil {
				continue
			}

			var msgUser model.User
			if err := db.First(&msgUser, lastMsg.UserID).Error; err != nil {
				continue
			}

			if msgUser.IsAdmin == 0 && time.Since(lastMsg.CreatedAt) > 24*time.Hour {
				fmt.Printf("工单 #%d (%s) 超过24小时未回复\n", ticket.ID, ticket.Subject)
				unansweredCount++
			}
		}

		if unansweredCount == 0 {
			fmt.Println("没有未回复的工单")
		} else {
			fmt.Printf("共发现 %d 个未回复的工单\n", unansweredCount)
		}
	},
}
