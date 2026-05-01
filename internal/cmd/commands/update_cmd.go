package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"github.com/xboard/xboard/internal/cmd"
)

const (
	updateCurrentVersion = "1.0.0"
	updateCheckURL       = "https://api.github.com/repos/xboard/xboard/releases/latest"
)

type githubRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	HTMLURL    string `json:"html_url"`
	Prerelease bool   `json:"prerelease"`
}

func init() {
	cmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:   "xboard:update",
	Short: "从 GitHub 更新程序",
	Run: func(cobraCmd *cobra.Command, args []string) {
		fmt.Println("检查更新...")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(updateCheckURL)
		if err != nil {
			fmt.Printf("检查更新失败: %v\n", err)
			return
		}
		defer resp.Body.Close()

		var release githubRelease
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			fmt.Printf("解析更新信息失败: %v\n", err)
			return
		}

		fmt.Printf("当前版本: %s\n", updateCurrentVersion)
		fmt.Printf("最新版本: %s\n", release.TagName)
		fmt.Printf("发布时间: %s\n", release.Name)
		fmt.Printf("更新地址: %s\n", release.HTMLURL)

		if release.TagName != "" && release.TagName != "v"+updateCurrentVersion && release.TagName != updateCurrentVersion {
			fmt.Println("发现新版本，请手动下载更新:")
			fmt.Printf("  %s\n", release.HTMLURL)
		} else {
			fmt.Println("当前已是最新版本")
		}
	},
}
