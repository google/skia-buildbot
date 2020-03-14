package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// webuiCmd represents the webui command
var webuiCmd = &cobra.Command{
	Use:   "webui",
	Short: "The main web UI.",
	Long: `Runs the process that serves the web UI for Perf.
	`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("webui called")
	},
}

func webUIInit() {
	rootCmd.AddCommand(webuiCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// webuiCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// webuiCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
