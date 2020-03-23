package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/ingest/parser"
	"go.skia.org/infra/perf/go/types"
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
		ctx := context.Background()

		// Metrics.
		filesReceived := metrics2.GetCounter("perfserver_ingest_files_received")
		failedToParse := metrics2.GetCounter("perfserver_ingest_failed_to_parse")
		badGitHash := metrics2.GetCounter("perfserver_ingest_bad_githash")
		failedToWrite := metrics2.GetCounter("perfserver_ingest_failed_to_write")
		successfulWrite := metrics2.GetCounter("perfserver_ingest_successful_write")

		// New file.Source.
		source, err := builders.NewSourceFromConfig(ctx, instanceConfig, false)
		ch, err := source.Start(ctx)
		if err != nil {
			return err
		}

		// New Parser.
		parser := parser.New(instanceConfig)

		// New TraceStore.
		store, err := builders.NewTraceStoreFromConfig(ctx, false, instanceConfig)
		if err != nil {
			return err
		}

		// New gitinfo.GitInfo.
		sklog.Infof("Cloning repo %q into %q", instanceConfig.GitRepoConfig.URL, instanceConfig.GitRepoConfig.Dir)
		vcs, err := gitinfo.CloneOrUpdate(ctx, instanceConfig.GitRepoConfig.URL, instanceConfig.GitRepoConfig.Dir, false)
		if err != nil {
			return err
		}

		sklog.Info("Waiting on files to process.")
		for f := range ch {
			sklog.Infof("Ingest received: %v", f)
			filesReceived.Inc(1)

			// Parse the file.
			params, values, gitHash, err := parser.Parse(f)
			if err != nil {
				sklog.Errorf("Failed to parse %v: %s", f, err)
				failedToParse.Inc(1)
				continue
			}

			// Convert gitHash to commitNumber.
			index, err := vcs.IndexOf(ctx, gitHash)
			if err != nil {
				badGitHash.Inc(1)
				sklog.Error("Failed to find gitHash %v: %s", f, err)
				continue
			}

			// Build paramset from params.
			ps := paramtools.NewParamSet()
			for _, p := range params {
				ps.AddParams(p)
			}

			// Write data to the trace store.
			if err := store.WriteTraces(types.CommitNumber(index), params, values, ps, f.Name, time.Now()); err != nil {
				failedToWrite.Inc(1)
				sklog.Error("Failed to write %v: %s", f, err)
			}
			successfulWrite.Inc(1)
		}
		sklog.Infof("Exited while waiting on files. Should only happen on source_type=dir.")

		return nil
	},
}

func ingestInit() {
	rootCmd.AddCommand(ingestCmd)
}
