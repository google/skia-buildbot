package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"
	"google.golang.org/protobuf/encoding/prototext"

	"go.skia.org/infra/cabe/go/analyzer"
	"go.skia.org/infra/go/sklog"
)

// checkCmd holds the flag values and any internal state necessary for
// executing the `check` subcommand.
type checkCmd struct {
	commonCmd
}

// CheckCommand returns a [*cli.Command] for running cabe's analysis precondition checker.
func CheckCommand() *cli.Command {
	cmd := &checkCmd{}
	return &cli.Command{
		Name:        "check",
		Description: "check runs some diagnostic checks on perf experiment jobs.",
		Usage:       "cabe check --pinpoint-job <pinpoint-job>",
		Flags:       cmd.flags(),
		Action:      cmd.action,
		After:       cmd.cleanup,
	}
}

// action runs diagnostic checks on an experiment.
func (cmd *checkCmd) action(cliCtx *cli.Context) error {
	ctx := cliCtx.Context
	if err := cmd.dialBackends(ctx); err != nil {
		return err
	}

	var analyzerOpts = []analyzer.Options{
		analyzer.WithCASResultReader(cmd.casResultReader),
		analyzer.WithSwarmingTaskReader(cmd.swarmingTaskReader),
		analyzer.WithExperimentSpec(cmd.experimentSpecFromFlags()),
	}

	a := analyzer.New(cmd.pinpointJobID, analyzerOpts...)

	c := analyzer.NewChecker(analyzer.DefaultCheckerOpts...)
	if err := a.RunChecker(ctx, c); err != nil {
		sklog.Errorf("run checker error: %v", err)
	}

	exSpec := a.ExperimentSpec()
	if exSpec != nil {
		txt := prototext.MarshalOptions{
			Multiline: true,
			Indent:    "  ",
		}.Format(exSpec)
		fmt.Printf("ExperimentSpec:\n%s\n", txt)
	}

	findings := c.Findings()
	if len(findings) == 0 {
		fmt.Printf("Checker returned no findings.\n")
		return nil
	}
	fmt.Printf("Checker returned %d findings\n", len(findings))
	for i, finding := range c.Findings() {
		fmt.Println(i, finding)
	}

	return nil
}
