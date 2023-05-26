package cli

import (
	"context"
	"fmt"
	"time"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"github.com/urfave/cli/v2"
	swarmingapi "go.chromium.org/luci/common/api/swarming/swarming/v1"

	"go.skia.org/infra/cabe/go/backends"
	"go.skia.org/infra/cabe/go/perfresults"
	"go.skia.org/infra/cabe/go/replaybackends"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
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
)

type commonCmd struct {
	pinpointJobID string
	recordToZip   string
	replayFromZip string

	replayBackends *replaybackends.ReplayBackends

	swarmingClient swarming.ApiClient
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

func (a *commonCmd) readSwarmingTasksFromAPI(ctx context.Context, pinpointJobID string) ([]*swarmingapi.SwarmingRpcsTaskRequestMetadata, error) {
	tasksResp, err := a.swarmingClient.ListTasks(ctx, time.Now().AddDate(0, 0, -rbeCASTTLDays), time.Now(), []string{"pinpoint_job_id:" + pinpointJobID}, "")
	if err != nil {
		sklog.Fatalf("list task results: %v", err)
		return nil, err
	}
	return tasksResp, nil
}

func (cmd *commonCmd) dialBackends(ctx context.Context) error {
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

	if cmd.replayFromZip != "" {
		cmd.replayBackends = replaybackends.FromZipFile(cmd.replayFromZip, "blank")
		cmd.casResultReader = cmd.replayBackends.CASResultReader
		cmd.swarmingTaskReader = cmd.replayBackends.SwarmingTaskReader
	} else if cmd.recordToZip != "" {
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
	return []cli.Flag{pinpointJobIDFlag, replayFromZipFlag, recordToZipFlag}
}

func (cmd *commonCmd) cleanup(cliCtx *cli.Context) error {
	if cmd.replayBackends != nil {
		return cmd.replayBackends.Close()
	}
	return nil
}
