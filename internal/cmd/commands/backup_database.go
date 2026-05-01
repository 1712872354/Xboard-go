package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xboard/xboard/internal/cmd"
	"github.com/xboard/xboard/internal/config"
)

func init() {
	cmd.AddCommand(backupDatabaseCmd)
}

var backupDatabaseCmd = &cobra.Command{
	Use:   "backup:database",
	Short: "备份数据库 (MySQL dump)",
	Run: func(cobraCmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("无法加载配置: %v\n", err)
			return
		}

		if cfg.Database.Driver != "mysql" {
			fmt.Printf("仅支持 MySQL 数据库备份，当前驱动: %s\n", cfg.Database.Driver)
			return
		}

		backupDir := "backups"
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			fmt.Printf("创建备份目录失败: %v\n", err)
			return
		}

		filename := fmt.Sprintf("%s_%s.sql", cfg.Database.DBName, time.Now().Format("20060102_150405"))
		backupPath := backupDir + "/" + filename

		fmt.Printf("正在备份数据库 %s ...\n", cfg.Database.DBName)

		mysqldumpPath, err := exec.LookPath("mysqldump")
		if err != nil {
			fmt.Println("未找到 mysqldump，请确保已安装 MySQL 客户端")
			return
		}

		argsList := []string{
			"-h", cfg.Database.Host,
			"-P", fmt.Sprintf("%d", cfg.Database.Port),
			"-u", cfg.Database.Username,
			fmt.Sprintf("--password=%s", cfg.Database.Password),
			cfg.Database.DBName,
			"--routines",
			"--events",
			"--result-file", backupPath,
		}

		cmdExec := exec.Command(mysqldumpPath, argsList...)
		output, err := cmdExec.CombinedOutput()
		if err != nil {
			fmt.Printf("备份失败: %v\n%s\n", err, string(output))
			return
		}

		fileInfo, _ := os.Stat(backupPath)
		var fileSize string
		if fileInfo != nil {
			size := fileInfo.Size()
			if size > 1024*1024 {
				fileSize = fmt.Sprintf("%.2f MB", float64(size)/(1024*1024))
			} else if size > 1024 {
				fileSize = fmt.Sprintf("%.2f KB", float64(size)/1024)
			} else {
				fileSize = fmt.Sprintf("%d B", size)
			}
		}

		fmt.Printf("备份完成！\n")
		fmt.Printf("  文件: %s\n", backupPath)
		if fileSize != "" {
			fmt.Printf("  大小: %s\n", fileSize)
		}

		// 清理旧备份，保留最近 7 天的
		entries, err := os.ReadDir(backupDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
					info, _ := entry.Info()
					if info != nil && time.Since(info.ModTime()) > 7*24*time.Hour {
						os.Remove(backupDir + "/" + entry.Name())
					}
				}
			}
		}
	},
}
