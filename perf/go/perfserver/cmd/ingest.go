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

var ingestFlags config.IngestFlags

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

		instanceConfig, err := config.InstanceConfigFromFile(ingestFlags.InstanceConfigFile)
		if err != nil {
			return err
		}

		metrics2.InitPrometheus(ingestFlags.PromPort)

		return process.Start(context.Background(), ingestFlags.Local, instanceConfig)
	},
}

func ingestInit() error {
	rootCmd.AddCommand(ingestCmd)
	ingestFlags.Register(ingestCmd.LocalFlags())

	if err := ingestCmd.MarkFlagRequired("config"); err != nil {
		return err
	}

	return nil
}
