package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/urfave/cli/v2"
	"google.golang.org/protobuf/encoding/prototext"

	"go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/cabe/go/analyzer"
	"go.skia.org/infra/cabe/go/backends"
	"go.skia.org/infra/cabe/go/perfresults"
	"go.skia.org/infra/cabe/go/replaybackends"
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
	}
}

// action runs diagnostic checks on an experiment.
func (cmd *checkCmd) action(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	rbeClients, err := backends.DialRBECAS(ctx)
	if err != nil {
		sklog.Fatalf("dialing RBE-CAS backends: %v", err)
		return err
	}

	swarmingClient, err := backends.DialSwarming(ctx)
	if err != nil {
		sklog.Fatalf("dialing swarming: %v", err)
		return err
	}

	var casResultReader = func(c context.Context, casInstance, digest string) (map[string]perfresults.PerfResults, error) {
		rbeClient := rbeClients[casInstance]
		return analyzer.FetchBenchmarkJSON(ctx, rbeClient, digest)
	}

	var swarmingTaskReader = func(ctx context.Context) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
		tasksResp, err := swarmingClient.ListTasks(ctx, time.Now().AddDate(0, 0, -56), time.Now(), []string{pinpointSwarmingTagName + ":" + cmd.pinpointJobID}, "")
		if err != nil {
			sklog.Fatalf("list task results: %v", err)
			return nil, err
		}
		return tasksResp, nil
	}

	if cmd.replayFromZip != "" {
		replayBackends := replaybackends.FromZipFile(cmd.replayFromZip, "blank")
		casResultReader = replayBackends.CASResultReader
		swarmingTaskReader = replayBackends.SwarmingTaskReader
	} else if cmd.recordToZip != "" {
		replayBackends := replaybackends.ToZipFile(cmd.recordToZip, cmd.pinpointJobID, rbeClients, swarmingClient)
		defer func() {
			if err := replayBackends.Close(); err != nil {
				sklog.Fatalf("closing replay backends: %v", err)
			}
		}()
		casResultReader = replayBackends.CASResultReader
		swarmingTaskReader = replayBackends.SwarmingTaskReader
	}

	var analyzerOpts = []analyzer.Options{
		analyzer.WithCASResultReader(casResultReader),
		analyzer.WithSwarmingTaskReader(swarmingTaskReader),
	}

	a := analyzer.New(analyzerOpts...)

	c := analyzer.NewChecker(analyzer.DefaultCheckerOpts...)
	if err := a.RunChecker(ctx, c); err != nil {
		sklog.Fatalf("run checker error: %v", err)
		return err
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
