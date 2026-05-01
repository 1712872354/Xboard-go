package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xboard/xboard/internal/cmd"
	"github.com/xboard/xboard/internal/plugin"
)

func init() {
	cmd.AddCommand(hookListCmd)
}

var hookListCmd = &cobra.Command{
	Use:   "hook:list",
	Short: "列出已注册的钩子",
	Run: func(cobraCmd *cobra.Command, args []string) {
		hm := plugin.GetHookManager()

		hookNames := []string{
			"payment.notify.success",
			"order.create.before",
			"order.create.after",
			"order.open.before",
			"order.open.after",
			"user.register.after",
			"available_payment_methods",
			"ticket.create.after",
		}

		fmt.Println("已注册的钩子:")
		fmt.Println("========================================")

		totalListeners := 0
		totalFilters := 0

		for _, name := range hookNames {
			listenerCount := hm.ListenerCount(name)
			filterCount := hm.FilterCount(name)
			hasEvent := hm.HasEvent(name)
			hasFilter := hm.HasFilter(name)

			if listenerCount > 0 || filterCount > 0 {
				fmt.Printf("  %s\n", name)
				if hasEvent {
					fmt.Printf("    - 监听器: %d\n", listenerCount)
					totalListeners += listenerCount
				}
				if hasFilter {
					fmt.Printf("    - 过滤器: %d\n", filterCount)
					totalFilters += filterCount
				}
			}
		}

		fmt.Println("========================================")
		fmt.Printf("总计: %d 个监听器, %d 个过滤器\n", totalListeners, totalFilters)
	},
}
