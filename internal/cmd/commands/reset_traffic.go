package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/xboard/xboard/internal/cmd"
	"github.com/xboard/xboard/internal/config"
	"github.com/xboard/xboard/internal/database"
	"github.com/xboard/xboard/internal/model"
	"gorm.io/gorm"
)

func init() {
	cmd.AddCommand(resetTrafficCmd)
}

var resetTrafficCmd = &cobra.Command{
	Use:   "reset:traffic",
	Short: "重置用户流量",
	Run: func(cobraCmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("无法加载配置: %v\n", err)
			return
		}
		db := database.InitDB(&cfg.Database)
		model.DB = db

		today := time.Now().Day()

		var users []model.User
		if err := db.Where("traffic_reset_day = ? AND plan_id > 0", today).Find(&users).Error; err != nil {
			fmt.Printf("查询用户失败: %v\n", err)
			return
		}

		resetCount := 0
		for _, user := range users {
			before := user.U + user.D

			err := db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Model(&model.User{}).Where("id = ?", user.ID).
					Updates(map[string]interface{}{"u": 0, "d": 0}).Error; err != nil {
					return err
				}

				resetLog := model.TrafficResetLog{
					UserID: user.ID,
					Before: before,
					After:  0,
				}
				return tx.Create(&resetLog).Error
			})

			if err != nil {
				fmt.Printf("重置用户 %d 流量失败: %v\n", user.ID, err)
			} else {
				fmt.Printf("用户 %d 流量已重置: 之前=%d, 之后=0\n", user.ID, before)
				resetCount++
			}
		}

		fmt.Printf("共重置 %d 个用户的流量\n", resetCount)
	},
}
