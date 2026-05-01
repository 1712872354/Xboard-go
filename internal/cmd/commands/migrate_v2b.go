package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xboard/xboard/internal/cmd"
	"github.com/xboard/xboard/internal/config"
	"github.com/xboard/xboard/internal/database"
)

func init() {
	cmd.AddCommand(migrateV2bCmd)
}

var migrateV2bCmd = &cobra.Command{
	Use:   "migrate:v2b",
	Short: "从 V2board 迁移数据",
	Run: func(cobraCmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("无法加载配置: %v\n", err)
			return
		}
		db := database.InitDB(&cfg.Database)
		_ = db

		fmt.Println("========================================")
		fmt.Println("  V2board 数据迁移工具")
		fmt.Println("========================================")
		fmt.Println()
		fmt.Println("此功能将帮助您从 V2board 迁移数据到 Xboard。")
		fmt.Println("迁移前请确保：")
		fmt.Println("  1. 已备份原 V2board 数据库")
		fmt.Println("  2. 当前数据库为空或为 Xboard 格式")
		fmt.Println()
		fmt.Println("迁移过程包括：")
		fmt.Println("  - 用户数据迁移")
		fmt.Println("  - 套餐/计划数据迁移")
		fmt.Println("  - 订单数据迁移")
		fmt.Println("  - 服务器节点数据迁移")
		fmt.Println("  - 优惠券数据迁移")
		fmt.Println()
		fmt.Println("请手动执行迁移，具体步骤请参考文档:")
		fmt.Println("  https://github.com/xboard/xboard/wiki/migrate-from-v2board")
	},
}
