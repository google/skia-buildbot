package cmd

import (
	"context"
	"os"

	"github.com/jcgregorio/logger"
	"github.com/spf13/cobra"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog/glog_and_cloud"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ingest/process"
)

type IngestFlags struct {
	instanceConfigFile string
	promPort           string
	local              bool
}

var ingestFlags IngestFlags

// ingestCmd represents the ingest command
var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Run the ingestion process.",
	Long: `Continuously imports files as they arrive from
the configured ingestion sources and populates the datastore
with that data.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Log to stdout.
		glog_and_cloud.SetLogger(
			glog_and_cloud.NewSLogCloudLogger(logger.NewFromOptions(&logger.Options{
				SyncWriter: os.Stdout,
			})),
		)

		instanceConfig, err := config.InstanceConfigFromFile(ingestFlags.instanceConfigFile)
		if err != nil {
			return err
		}

		metrics2.InitPrometheus(ingestFlags.promPort)

		return process.Start(context.Background(), ingestFlags.local, instanceConfig)
	},
}

func ingestInit() error {
	rootCmd.AddCommand(ingestCmd)

	ingestCmd.LocalFlags().StringVar(&ingestFlags.instanceConfigFile, "config", "", "Instance config file. Must be supplied.")
	if err := ingestCmd.MarkFlagRequired("config"); err != nil {
		return err
	}

	ingestCmd.LocalFlags().StringVar(&ingestFlags.promPort, "prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	ingestCmd.LocalFlags().BoolVar(&ingestFlags.local, "local", false, "True if running locally and not in production.")
	return nil
}
