package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// alertCmd represents the alert command
var alertCmd = &cobra.Command{
	Use:   "alert",
	Short: "Run the regression detection process.",
	Long: `Continuously runs over all the configured alerts
and looks for regressions as new data arrives.
`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("alert called")
	},
}

func alertInit() {
	rootCmd.AddCommand(alertCmd)
}
