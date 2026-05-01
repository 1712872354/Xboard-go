package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/xboard/xboard/internal/cmd"
	"github.com/xboard/xboard/internal/config"
	"github.com/xboard/xboard/internal/database"
	"github.com/xboard/xboard/internal/model"
	"github.com/xboard/xboard/internal/service"
)

func init() {
	cmd.AddCommand(checkOrderCmd)
}

var checkOrderCmd = &cobra.Command{
	Use:   "check:order",
	Short: "关闭超时未支付的订单",
	Run: func(cobraCmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("无法加载配置: %v\n", err)
			return
		}
		db := database.InitDB(&cfg.Database)
		model.DB = db

		orderSvc := service.NewOrderService(db, nil, nil, nil, nil)

		orders, err := orderSvc.GetPendingOrders(100)
		if err != nil {
			fmt.Printf("查询待处理订单失败: %v\n", err)
			return
		}

		closedCount := 0
		for _, order := range orders {
			if time.Since(order.CreatedAt) > 15*time.Minute {
				if err := orderSvc.CloseOrder(order.ID); err != nil {
					fmt.Printf("关闭订单 %d 失败: %v\n", order.ID, err)
				} else {
					closedCount++
				}
			}
		}

		fmt.Printf("关闭了 %d 个超时订单\n", closedCount)
	},
}
