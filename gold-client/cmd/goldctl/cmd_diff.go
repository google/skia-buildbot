package main

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/gold-client/go/goldclient"
	"go.skia.org/infra/golden/go/types"
)

// diffEnv provides the environment for the diff command.
type diffEnv struct {
	inputFile      string
	deprecatedTest string
	grouping       string
	corpus         string
	instanceID     string
	outDir         string
	workDir        string
}

// getDiffCmd returns the definition of the diff command.
func getDiffCmd() *cobra.Command {
	env := &diffEnv{}
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compute the diff to the closest known image",
		Long: `
Downloads all images for a given test and compares them to the provided image.

Outputs the closest of these images and the diff to the given folder.
`,
		Run: env.runDiffCmd,
	}

	cmd.Flags().StringVar(&env.inputFile, "input", "", "path to file to diff")
	cmd.Flags().StringVar(&env.deprecatedTest, "test", "", "[deprecated] Test name. Clients should use grouping instead.")
	cmd.Flags().StringVar(&env.grouping, "grouping", "", "A comma separated list of keys and values that make up the grouping (not including the corpus). For example, 'name=myTest' or 'name=myTest,color_mode=GREY'")
	cmd.Flags().StringVar(&env.corpus, "corpus", "", "Corpus name. This will be combined with the grouping.")
	cmd.Flags().StringVar(&env.instanceID, "instance", "", "Instance (e.g. 'chrome', 'flutter')")
	cmd.Flags().StringVar(&env.outDir, "out-dir", "", "Work directory that will contain the output")
	cmd.Flags().StringVar(&env.workDir, fstrWorkDir, "", "Work directory for intermediate results")
	// Everything is required for this command.
	must(cmd.MarkFlagRequired(fstrWorkDir))
	must(cmd.MarkFlagRequired("input"))
	must(cmd.MarkFlagRequired("corpus"))
	must(cmd.MarkFlagRequired("out-dir"))
	must(cmd.MarkFlagRequired("instance"))

	return cmd
}

func (d *diffEnv) runDiffCmd(cmd *cobra.Command, _ []string) {
	ctx := cmd.Context()
	d.Diff(ctx)
}

// Diff executes the diff logic for comparing a given image against all that Gold knows.
func (d *diffEnv) Diff(ctx context.Context) {
	ctx = loadAuthenticatedClients(ctx, d.workDir)

	config := goldclient.GoldClientConfig{
		InstanceID: d.instanceID,
		WorkDir:    d.workDir,
	}
	if d.deprecatedTest == "" && d.grouping == "" {
		logErrf(ctx, "Must include either test or grouping")
		exitProcess(ctx, 1)
	}

	// overwrite any existing configs in this workdir.
	goldClient, err := goldclient.NewCloudClient(config)
	ifErrLogExit(ctx, err)

	grouping := paramtools.Params{
		types.CorpusField: d.corpus,
	}
	if d.grouping != "" {
		segments := strings.Split(d.grouping, ",")
		for _, seg := range segments {
			keyAndValue := strings.Split(seg, "=")
			if len(keyAndValue) != 2 {
				logErrf(ctx, "Invalid grouping param %q", seg)
				exitProcess(ctx, 1)
			}
			grouping[keyAndValue[0]] = keyAndValue[1]
		}
	} else {
		grouping[types.PrimaryKeyField] = d.deprecatedTest
	}

	err = goldClient.Diff(ctx, grouping, d.inputFile, d.outDir)
	ifErrLogExit(ctx, err)
	exitProcess(ctx, 0)
}
