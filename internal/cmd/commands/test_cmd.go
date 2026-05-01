package commands

import (
	"fmt"
	"net"
	"time"

	"github.com/spf13/cobra"
	"github.com/xboard/xboard/internal/cmd"
	"github.com/xboard/xboard/internal/config"
	"github.com/xboard/xboard/internal/database"
	"github.com/xboard/xboard/internal/model"
)

func init() {
	cmd.AddCommand(testCmd)
}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "测试数据库连接和配置",
	Run: func(cobraCmd *cobra.Command, args []string) {
		fmt.Println("========================================")
		fmt.Println("  Xboard 配置测试")
		fmt.Println("========================================")

		// 测试配置加载
		fmt.Println("[1/3] 加载配置文件...")
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("  ✗ 失败: %v\n", err)
			return
		}
		fmt.Println("  ✓ 成功")

		// 测试数据库连接
		fmt.Println("[2/3] 测试数据库连接...")
		db := database.InitDB(&cfg.Database)
		model.DB = db

		sqlDB, err := db.DB()
		if err != nil {
			fmt.Printf("  ✗ 获取数据库实例失败: %v\n", err)
			return
		}

		if err := sqlDB.Ping(); err != nil {
			fmt.Printf("  ✗ 数据库连接失败: %v\n", err)
			return
		}
		fmt.Println("  ✓ 数据库连接正常")

		// 测试网络连接
		fmt.Println("[3/3] 测试网络连接...")
		if cfg.App.URL != "" {
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:443", cfg.App.URL), 5*time.Second)
			if err == nil {
				conn.Close()
				fmt.Printf("  ✓ 站点 %s 可达\n", cfg.App.URL)
			}
		}
		fmt.Println()

		fmt.Println("========================================")
		fmt.Println("  测试全部通过！")
		fmt.Println("========================================")
	},
}
