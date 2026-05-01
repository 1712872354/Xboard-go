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
	cmd.AddCommand(xboardStatisticsCmd)
}

var xboardStatisticsCmd = &cobra.Command{
	Use:   "xboard:statistics",
	Short: "运行每日统计计算",
	Run: func(cobraCmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("无法加载配置: %v\n", err)
			return
		}
		db := database.InitDB(&cfg.Database)
		model.DB = db

		fmt.Println("运行每日统计计算...")

		today := time.Now().Truncate(24 * time.Hour)
		tomorrow := today.Add(24 * time.Hour)

		var existing model.Stat
		result := db.Where("recorded_at >= ? AND recorded_at < ?", today, tomorrow).First(&existing)
		if result.RowsAffected > 0 {
			fmt.Println("今日统计已存在，跳过")
			return
		}

		var registerCount int64
		db.Model(&model.User{}).Where("created_at >= ? AND created_at < ?", today, tomorrow).Count(&registerCount)

		var tradeCount int64
		var tradeAmount float64
		db.Model(&model.Order{}).
			Where("status = ? AND paid_at >= ? AND paid_at < ?", 2, today, tomorrow).
			Count(&tradeCount)
		db.Model(&model.Order{}).
			Select("COALESCE(SUM(total_amount), 0)").
			Where("status = ? AND paid_at >= ? AND paid_at < ?", 2, today, tomorrow).
			Scan(&tradeAmount)

		var activeUserCount int64
		db.Model(&model.User{}).Where("expired_at > ?", time.Now()).Count(&activeUserCount)

		stat := model.Stat{
			RegisterCount: int(registerCount),
			TradeCount:    int(tradeCount),
			TradeAmount:   tradeAmount,
			PaidUserCount: int(activeUserCount),
			RecordedAt:    time.Now(),
		}

		if err := db.Create(&stat).Error; err != nil {
			fmt.Printf("创建统计记录失败: %v\n", err)
			return
		}

		fmt.Printf("统计完成: %d 注册, %d 订单, %.2f 收入, %d 活跃用户\n",
			registerCount, tradeCount, tradeAmount, activeUserCount)
	},
}
