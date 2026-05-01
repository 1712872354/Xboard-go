package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/xboard/xboard/internal/cmd"
)

func init() {
	cmd.AddCommand(rollbackCmd)
}

var rollbackCmd = &cobra.Command{
	Use:   "xboard:rollback",
	Short: "回滚更新",
	Run: func(cobraCmd *cobra.Command, args []string) {
		execPath, err := os.Executable()
		if err != nil {
			fmt.Printf("获取程序路径失败: %v\n", err)
			return
		}

		backupPath := filepath.Join(filepath.Dir(execPath), "xboard.bak")
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			fmt.Println("未找到备份文件 xboard.bak，无法回滚")
			return
		}

		fmt.Println("正在回滚更新...")

		cmd := exec.Command("cp", backupPath, execPath)
		if err := cmd.Run(); err != nil {
			fmt.Printf("回滚失败: %v\n", err)
			return
		}

		fmt.Println("回滚完成，请重启程序")
	},
}
