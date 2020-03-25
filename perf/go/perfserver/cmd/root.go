package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog/glog_and_cloud"
	"go.skia.org/infra/perf/go/config"
)

var instanceConfigFile string
var instanceConfig *config.InstanceConfig
var promPort string
var local bool

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "perfserver",
	Short: "The main Perf application.",
	Long: `The main Perf application.

The different parts of Perf are run as sub-commands, for example
to run the ingestion process:

	perfserver ingest --config=instance_config.json ...

`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		glog_and_cloud.SetLogger(glog_and_cloud.NewStdErrCloudLogger(glog_and_cloud.SLogStderr))

		var err error
		instanceConfig, err = config.InstanceConfigFromFile(instanceConfigFile)
		if err != nil {
			return err
		}

		metrics2.InitPrometheus(promPort)

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := initSubCommands(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initSubCommands() error {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&instanceConfigFile, "config", "", "Instance config file. Must be supplied.")
	err := rootCmd.MarkPersistentFlagRequired("config")
	if err != nil {
		return err
	}

	rootCmd.PersistentFlags().StringVar(&promPort, "prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	rootCmd.PersistentFlags().BoolVar(&local, "local", false, "True if running locally and not in production.")

	alertInit()
	ingestInit()
	frontendInit()

	return nil
}
