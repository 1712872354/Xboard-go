package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xboard/xboard/internal/cmd"
	"github.com/xboard/xboard/internal/config"
	"github.com/xboard/xboard/internal/database"
	"github.com/xboard/xboard/internal/model"
	"gorm.io/gorm"
)

func init() {
	cmd.AddCommand(checkCommissionCmd)
}

var checkCommissionCmd = &cobra.Command{
	Use:   "check:commission",
	Short: "处理待结算佣金",
	Run: func(cobraCmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("无法加载配置: %v\n", err)
			return
		}
		db := database.InitDB(&cfg.Database)
		model.DB = db

		var orders []model.Order
		if err := db.Where("commission_status = 0 AND status = ? AND invite_user_id > 0", 2).
			Find(&orders).Error; err != nil {
			fmt.Printf("查询订单失败: %v\n", err)
			return
		}

		processedCount := 0
		for _, order := range orders {
			var inviter model.User
			if err := db.First(&inviter, order.InviteUserID).Error; err != nil {
				fmt.Printf("邀请人 %d 不存在, 跳过订单 %d\n", order.InviteUserID, order.ID)
				continue
			}

			commissionRate := inviter.CommissionRate
			if commissionRate <= 0 {
				continue
			}

			commissionAmount := order.TotalAmount * float64(commissionRate) / 100.0
			if commissionAmount <= 0 {
				continue
			}

			err := db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Model(&model.Order{}).Where("id = ?", order.ID).
					Update("commission_status", 1).Error; err != nil {
					return err
				}

				commissionLog := model.CommissionLog{
					InviteUserID: order.InviteUserID,
					OrderID:      order.ID,
					GetAmount:    commissionAmount,
					Status:       1,
				}
				if err := tx.Create(&commissionLog).Error; err != nil {
					return err
				}

				return tx.Model(&model.User{}).Where("id = ?", order.InviteUserID).
					Update("commission_balance", gorm.Expr("commission_balance + ?", commissionAmount)).Error
			})

			if err != nil {
				fmt.Printf("处理订单 %d 佣金失败: %v\n", order.ID, err)
			} else {
				processedCount++
			}
		}

		fmt.Printf("处理了 %d 笔佣金\n", processedCount)
	},
}
