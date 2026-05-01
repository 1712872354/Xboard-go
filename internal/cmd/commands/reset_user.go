package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xboard/xboard/internal/cmd"
	"github.com/xboard/xboard/internal/config"
	"github.com/xboard/xboard/internal/database"
	"github.com/xboard/xboard/internal/model"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	cmd.AddCommand(resetUserCmd)
}

var resetUserCmd = &cobra.Command{
	Use:   "reset:user",
	Short: "重置单个用户",
	Run: func(cobraCmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("无法加载配置: %v\n", err)
			return
		}
		db := database.InitDB(&cfg.Database)
		model.DB = db

		if len(args) < 1 {
			fmt.Println("用法: xboard reset:user <email>")
			return
		}

		email := args[0]

		var user model.User
		if err := db.Where("email = ?", email).First(&user).Error; err != nil {
			fmt.Printf("用户 %s 不存在\n", email)
			return
		}

		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
		updates := map[string]interface{}{
			"password":            string(hashedPassword),
			"plan_id":             0,
			"u":                   0,
			"d":                   0,
			"transfer_enable":     0,
			"balance":             0,
			"commission_balance":  0,
		}

		if err := db.Model(&user).Updates(updates).Error; err != nil {
			fmt.Printf("重置用户失败: %v\n", err)
			return
		}

		fmt.Printf("用户 %s 已重置 (密码: 123456)\n", email)
	},
}
