package cmd

import (
	"context"
	"os"

	"github.com/jcgregorio/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
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
the configured ingestion sources and populates the TraceStore
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

		cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
			sklog.Infof("Flags: --%s=%v", f.Name, f.Value)
		})

		return process.Start(context.Background(), ingestFlags.Local, ingestFlags.NumParallelIngesters, instanceConfig)
	},
}

func ingestInit() error {
	rootCmd.AddCommand(ingestCmd)
	ingestFlags.Register(ingestCmd.Flags())
	return nil
}
