package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"go.skia.org/infra/go/git/gitinfo"
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
		source, err := builders.NewSourceFromConfig(ctx, instanceConfig, false)
		ch, err := source.Start(ctx)
		if err != nil {
			return err
		}

		parser := parser.New(instanceConfig)
		store, err := builders.NewTraceStoreFromConfig(ctx, false, instanceConfig)
		if err != nil {
			return err
		}

		sklog.Infof("Cloning repo %q into %q", instanceConfig.GitRepoConfig.URL, instanceConfig.GitRepoConfig.Dir)
		vcs, err := gitinfo.CloneOrUpdate(ctx, instanceConfig.GitRepoConfig.URL, instanceConfig.GitRepoConfig.Dir, false)
		if err != nil {
			return err
		}

		// TODO(jcgregorio) Add logging and metrics.

		sklog.Info("Waiting on files to process.")
		for f := range ch {
			sklog.Infof("%#v", f)
			params, values, gitHash, err := parser.Parse(f)
			if err != nil {
				continue
			}

			// Convert gitHash to commitNumber.
			index, err := vcs.IndexOf(ctx, gitHash)
			if err != nil {
				sklog.Error(err)
				continue
			}

			// Build paramset from params.
			ps := paramtools.NewParamSet()
			for _, p := range params {
				ps.AddParams(p)
			}

			if err := store.WriteTraces(types.CommitNumber(index), params, values, ps, f.Name, time.Now()); err != nil {
				sklog.Error(err)
			}
		}
		sklog.Infof("Exited while waiting on files. Should only happen on source_type=dir.")

		return nil
	},
}

func ingestInit() {
	rootCmd.AddCommand(ingestCmd)
}
