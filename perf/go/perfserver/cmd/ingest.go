package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"go.skia.org/infra/perf/go/ingest/process"
)

// ingestCmd represents the ingest command
var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Run the ingestion process.",
	Long: `Continuously imports files as they arrive from
the configured ingestion sources and populates the datastore
with that data.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return process.Start(context.Background(), instanceConfig)
	},
}

func ingestInit() {
	rootCmd.AddCommand(ingestCmd)
}
