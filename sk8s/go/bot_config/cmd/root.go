package cmd

import (
	"github.com/spf13/cobra"
	"go.skia.org/infra/go/sklog"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "bot_config",
	Short: "Determines the configuration of a bot.",
	Long:  `Each sub-command implements a get_* call in bot_config.py`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd.AddCommand(getDimensionsCmd)
	rootCmd.AddCommand(getSettingsCmd)
	rootCmd.AddCommand(getStateCmd)
	if err := rootCmd.Execute(); err != nil {
		sklog.Fatal(err)
	}
}
