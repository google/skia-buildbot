package cli

import (
	"context"
	"fmt"
	"time"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"github.com/urfave/cli/v2"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.skia.org/infra/cabe/go/backends"
	cpb "go.skia.org/infra/cabe/go/proto"
	"go.skia.org/infra/cabe/go/replaybackends"
	"go.skia.org/infra/go/sklog"
	swarmingv2 "go.skia.org/infra/go/swarming/v2"
	"go.skia.org/infra/perf/go/perfresults"
)

const (
	pinpointSwarmingTagName = "pinpoint_job_id"
	rbeCASTTLDays           = 56
)

// flag names
const (
	pinpointJobIDFlagName = "pinpoint-job"
	replayFromZipFlagName = "replay-from-zip"
	recordToZipFlagName   = "record-to-zip"
	benchmarkFlagName     = "benchmark"
	workloadFlagName      = "workload"
)

type commonCmd struct {
	pinpointJobID string
	recordToZip   string
	replayFromZip string
	benchmark     string
	workloads     []string

	replayBackends *replaybackends.ReplayBackends

	swarmingClient swarmingv2.SwarmingV2Client
	rbeClients     map[string]*rbeclient.Client

	swarmingTaskReader backends.SwarmingTaskReader
	casResultReader    backends.CASResultReader
}

func (a *commonCmd) readCASResultFromRBEAPI(ctx context.Context, instance, digest string) (map[string]perfresults.PerfResults, error) {
	rbeClient, ok := a.rbeClients[instance]
	if !ok {
		return nil, fmt.Errorf("no RBE client for instance %s", instance)
	}

	return backends.FetchBenchmarkJSON(ctx, rbeClient, digest)
}

func (a *commonCmd) readSwarmingTasksFromAPI(ctx context.Context, pinpointJobID string) ([]*apipb.TaskRequestMetadataResponse, error) {
	tasksResp, err := swarmingv2.ListTaskRequestMetadataHelper(ctx, a.swarmingClient, &apipb.TasksWithPerfRequest{
		Start: timestamppb.New(time.Now().AddDate(0, 0, -rbeCASTTLDays)),
		State: apipb.StateQuery_QUERY_ALL,
		Tags:  []string{"pinpoint_job_id:" + pinpointJobID},
	})
	if err != nil {
		sklog.Fatalf("list task results: %v", err)
		return nil, err
	}
	return tasksResp, nil
}

func (cmd *commonCmd) dialBackends(ctx context.Context) error {
	if cmd.replayFromZip != "" {
		cmd.replayBackends = replaybackends.FromZipFile(cmd.replayFromZip, "blank")
		cmd.casResultReader = cmd.replayBackends.CASResultReader
		cmd.swarmingTaskReader = cmd.replayBackends.SwarmingTaskReader
		return nil
	}

	rbeClients, err := backends.DialRBECAS(ctx)
	if err != nil {
		sklog.Fatalf("dialing RBE-CAS backends: %v", err)
		return err
	}
	cmd.rbeClients = rbeClients

	swarmingClient, err := backends.DialSwarming(ctx)
	if err != nil {
		sklog.Fatalf("dialing swarming: %v", err)
		return err
	}
	cmd.swarmingClient = swarmingClient

	cmd.swarmingTaskReader = cmd.readSwarmingTasksFromAPI
	cmd.casResultReader = cmd.readCASResultFromRBEAPI

	if cmd.recordToZip != "" {
		cmd.replayBackends = replaybackends.ToZipFile(cmd.recordToZip, rbeClients, swarmingClient)
		cmd.casResultReader = cmd.replayBackends.CASResultReader
		cmd.swarmingTaskReader = cmd.replayBackends.SwarmingTaskReader
	}
	return nil
}

func (cmd *commonCmd) flags() []cli.Flag {
	pinpointJobIDFlag := &cli.StringFlag{
		Name:        pinpointJobIDFlagName,
		Value:       "",
		Usage:       "ID of the pinpoint job to check",
		Destination: &cmd.pinpointJobID,
	}
	replayFromZipFlag := &cli.StringFlag{
		Name:        replayFromZipFlagName,
		Value:       "",
		Usage:       "Zip file to replay data from",
		Destination: &cmd.replayFromZip,
		Action: func(ctx *cli.Context, v string) error {
			if cmd.recordToZip != "" {
				return fmt.Errorf("only one of -%s or -%s may be specified", replayFromZipFlagName, recordToZipFlagName)
			}
			return nil
		},
	}
	recordToZipFlag := &cli.StringFlag{
		Name:        recordToZipFlagName,
		Value:       "",
		Usage:       "Zip file to save replay data to",
		Destination: &cmd.recordToZip,
		Action: func(ctx *cli.Context, v string) error {
			if cmd.replayFromZip != "" {
				return fmt.Errorf("only one of -%s or -%s may be specified", replayFromZipFlagName, recordToZipFlagName)
			}
			return nil
		},
	}
	benchmarkFlag := &cli.StringFlag{
		Name:        benchmarkFlagName,
		Value:       "",
		Usage:       "name of benchmark to analyze",
		Destination: &cmd.benchmark,
	}
	workloadFlag := &cli.StringSliceFlag{
		Name:  workloadFlagName,
		Value: nil,
		Usage: "comma separated list of names of benchmark workloads to analyze",
		Action: func(ctx *cli.Context, v []string) error {
			if cmd.benchmark == "" {
				return fmt.Errorf("must specify -%s with -%s", benchmarkFlagName, workloadFlagName)
			}
			cmd.workloads = v
			return nil
		},
	}

	return []cli.Flag{pinpointJobIDFlag, replayFromZipFlag, recordToZipFlag, benchmarkFlag, workloadFlag}
}

func (cmd *commonCmd) experimentSpecFromFlags() *cpb.ExperimentSpec {
	if cmd.benchmark != "" {
		aSpec := &cpb.AnalysisSpec{
			Benchmark: []*cpb.Benchmark{
				{
					Name:     cmd.benchmark,
					Workload: cmd.workloads,
				},
			},
		}

		return &cpb.ExperimentSpec{
			Analysis: aSpec,
		}
	}
	return nil
}

func (cmd *commonCmd) cleanup(cliCtx *cli.Context) error {
	if cmd.replayBackends != nil {
		return cmd.replayBackends.Close()
	}
	return nil
}
