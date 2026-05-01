package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xboard/xboard/internal/cmd"
)

func init() {
	cmd.AddCommand(installCmd)
}

var installCmd = &cobra.Command{
	Use:   "xboard:install",
	Short: "安装向导 - 创建配置文件并初始化数据库",
	Run: func(cobraCmd *cobra.Command, args []string) {
		reader := bufio.NewReader(os.Stdin)

		fmt.Println("========================================")
		fmt.Println("  Xboard 安装向导")
		fmt.Println("========================================")
		fmt.Println()

		// 检查配置文件是否已存在
		if _, err := os.Stat("config.yaml"); err == nil {
			fmt.Print("config.yaml 已存在，是否覆盖? (y/N): ")
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("安装已取消")
				return
			}
		}

		// 数据库配置
		fmt.Println("--- 数据库配置 ---")
		dbDriver := prompt(reader, "数据库驱动 (mysql/sqlite)", "mysql")
		dbHost := prompt(reader, "数据库主机", "127.0.0.1")
		dbPort := prompt(reader, "数据库端口", "3306")
		dbName := prompt(reader, "数据库名", "xboard")
		dbUser := prompt(reader, "数据库用户", "root")
		dbPass := prompt(reader, "数据库密码", "")
		tablePrefix := prompt(reader, "数据表前缀", "v2_")

		// 应用配置
		fmt.Println("--- 应用配置 ---")
		appKey := prompt(reader, "应用密钥", "xboard_secret_key_change_me")
		appURL := prompt(reader, "站点地址", "http://localhost:8080")

		// 生成配置文件
		configContent := fmt.Sprintf(`server:
  mode: debug
  port: 8080

app:
  name: Xboard
  version: 1.0.0
  key: %s
  url: %s
  secure_path: /admin
  subscribe_path: /s

database:
  driver: %s
  host: %s
  port: %s
  dbname: %s
  username: %s
  password: %s
  charset: utf8mb4
  collation: utf8mb4_unicode_ci
  table_prefix: %s

cache:
  driver: file

log:
  level: debug
  output: stdout
`, appKey, appURL, dbDriver, dbHost, dbPort, dbName, dbUser, dbPass, tablePrefix)

		if err := os.WriteFile("config.yaml", []byte(configContent), 0644); err != nil {
			fmt.Printf("写入配置文件失败: %v\n", err)
			return
		}

		fmt.Println("配置文件 config.yaml 已创建")
		fmt.Println()
		fmt.Println("安装完成！请运行 xboard server 启动服务")
	},
}

func prompt(reader *bufio.Reader, label, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", label, defaultValue)
	} else {
		fmt.Printf("%s: ", label)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}
