package cli

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/cabe/go/analyzer"
	"go.skia.org/infra/cabe/go/backends"
	"go.skia.org/infra/cabe/go/perfresults"
	"go.skia.org/infra/go/sklog"
)

const (
	pinpointSwarmingTagName = "pinpoint_job_id"
)

// Check runs diagnostic checks on an experiment.
func Check(ctx context.Context, pinpointJobID string) error {
	var casResultReader = func(c context.Context, casInstance, digest string) (map[string]perfresults.PerfResults, error) {
		rbeClients, err := backends.DialRBECAS(ctx)
		if err != nil {
			sklog.Fatalf("dialing RBE-CAS backends: %v", err)
			return nil, err
		}
		rbeClient := rbeClients[casInstance]
		return analyzer.FetchBenchmarkJSON(ctx, rbeClient, digest)
	}

	var swarmingTaskReader = func(ctx context.Context) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
		swarmingClient, err := backends.DialSwarming(ctx)
		if err != nil {
			sklog.Fatalf("dialing swarming: %v", err)
			return nil, err
		}
		tasksResp, err := swarmingClient.ListTasks(ctx, time.Now().AddDate(0, 0, -56), time.Now(), []string{pinpointSwarmingTagName + ":" + pinpointJobID}, "")
		if err != nil {
			sklog.Fatalf("list task results: %v", err)
			return nil, err
		}
		return tasksResp, nil
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

	for i, finding := range c.Findings() {
		fmt.Println(i, finding)
	}

	return nil
}
