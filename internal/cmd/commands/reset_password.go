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
	cmd.AddCommand(resetPasswordCmd)
}

var resetPasswordCmd = &cobra.Command{
	Use:   "reset:password",
	Short: "重置用户密码",
	Run: func(cobraCmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("无法加载配置: %v\n", err)
			return
		}
		db := database.InitDB(&cfg.Database)
		model.DB = db

		if len(args) < 2 {
			fmt.Println("用法: xboard reset:password <email> <new_password>")
			return
		}

		email := args[0]
		newPassword := args[1]

		var user model.User
		if err := db.Where("email = ?", email).First(&user).Error; err != nil {
			fmt.Printf("用户 %s 不存在\n", email)
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			fmt.Printf("密码加密失败: %v\n", err)
			return
		}

		if err := db.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
			fmt.Printf("密码重置失败: %v\n", err)
			return
		}

		fmt.Printf("用户 %s 密码已重置\n", email)
	},
}
