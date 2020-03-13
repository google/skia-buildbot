package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"go.skia.org/infra/go/sklog"
)

var instanceConfigFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "perfserver",
	Short: "The main Perf application.",
	Long: `The main Perf application.

The different parts of Perf are run as sub-commands, for example
to run the ingestion process:

	perfserver ingest --config=instance_config.json ...

`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	initSubCommands()
	if err := rootCmd.Execute(); err != nil {
		sklog.Error(err)
		os.Exit(1)
	}
}

func initSubCommands() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&instanceConfigFile, "config", "", "Instance config file. Must be supplied.")
	rootCmd.MarkPersistentFlagRequired("config")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	alertInit()
	webUIInit()
	ingestInit()
}
