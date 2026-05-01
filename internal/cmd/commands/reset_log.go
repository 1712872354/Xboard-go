package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/xboard/xboard/internal/cmd"
	"github.com/xboard/xboard/internal/config"
	"github.com/xboard/xboard/internal/database"
	"github.com/xboard/xboard/internal/model"
)

func init() {
	cmd.AddCommand(resetLogCmd)
}

var resetLogCmd = &cobra.Command{
	Use:   "reset:log",
	Short: "清理旧日志",
	Run: func(cobraCmd *cobra.Command, args []string) {
		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Printf("无法加载配置: %v\n", err)
			return
		}
		db := database.InitDB(&cfg.Database)
		model.DB = db

		cutoff := time.Now().AddDate(0, 0, -30)
		fmt.Printf("清理 %s 之前的日志...\n", cutoff.Format("2006-01-02"))

		var deletedCount int64

		result := db.Where("recorded_at < ?", cutoff).Delete(&model.ServerLog{})
		deletedCount += result.RowsAffected

		result = db.Where("recorded_at < ?", cutoff).Delete(&model.Stat{})
		deletedCount += result.RowsAffected

		result = db.Where("recorded_at < ?", cutoff).Delete(&model.StatServer{})
		deletedCount += result.RowsAffected

		result = db.Where("recorded_at < ?", cutoff).Delete(&model.StatUser{})
		deletedCount += result.RowsAffected

		fmt.Printf("清理完成, 共删除 %d 条日志\n", deletedCount)
	},
}
