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

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// alertCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// alertCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
