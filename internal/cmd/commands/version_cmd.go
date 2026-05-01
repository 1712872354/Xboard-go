package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xboard/xboard/internal/cmd"
)

const cliVersion = "1.0.0"

func init() {
	cmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Run: func(cobraCmd *cobra.Command, args []string) {
		fmt.Printf("Xboard %s\n", cliVersion)
		fmt.Println("代理面板管理系统")
		fmt.Println("Build: " + cliVersion)
	},
}
