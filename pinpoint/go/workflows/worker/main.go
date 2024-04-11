package main

import (
	"flag"
	"fmt"
	"os/user"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/internal"
	"go.skia.org/infra/temporal/go/metrics"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const appName = "pinpoint-worker"

var (
	hostPort  = flag.String("hostPort", "localhost:7233", "Host the worker connects to.")
	promPort  = flag.String("promPort", ":8000", "Prometheus port that it listens on.")
	namespace = flag.String("namespace", "default", "The namespace the worker registered to.")
	taskQueue = flag.String("taskQueue", "", "Task queue name registered to worker services.")
)

func main() {
	flag.Parse()

	common.InitWithMust(
		appName,
		common.PrometheusOpt(promPort),
	)

	if *taskQueue == "" {
		if u, err := user.Current(); err != nil {
			sklog.Fatalf("Unable to get the current user: %s", err)
		} else {
			*taskQueue = fmt.Sprintf("localhost.%s", u.Username)
		}
	}

	// The client and worker are heavyweight objects that should be created once per process.
	c, err := client.Dial(client.Options{
		MetricsHandler: metrics.NewMetricsHandler(map[string]string{}, nil),
		HostPort:       *hostPort,
		Namespace:      *namespace,
	})
	if err != nil {
		sklog.Fatalf("Unable to create client: %s", err)
	}
	defer c.Close()

	w := worker.New(c, *taskQueue, worker.Options{})

	bca := &internal.BuildChromeActivity{}
	w.RegisterActivity(bca)
	w.RegisterWorkflowWithOptions(internal.BuildChrome, workflow.RegisterOptions{Name: workflows.BuildChrome})

	rba := &internal.RunBenchmarkActivity{}
	w.RegisterActivity(rba)
	w.RegisterWorkflowWithOptions(internal.RunBenchmarkWorkflow, workflow.RegisterOptions{Name: workflows.RunBenchmark})

	w.RegisterActivity(internal.CollectValuesActivity)
	w.RegisterWorkflowWithOptions(internal.SingleCommitRunner, workflow.RegisterOptions{Name: workflows.SingleCommitRunner})

	w.RegisterActivity(internal.CompareActivity)
	w.RegisterActivity(internal.FindMidCommitActivity)
	w.RegisterActivity(internal.CheckCombinedCommitEqualActivity)
	w.RegisterActivity(internal.ReportStatusActivity)
	w.RegisterWorkflowWithOptions(internal.BisectWorkflow, workflow.RegisterOptions{Name: workflows.Bisect})

	w.RegisterActivity(internal.FindAvailableBotsActivity)
	w.RegisterWorkflowWithOptions(internal.PairwiseCommitsRunnerWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseCommitsRunner})

	w.RegisterActivity(internal.PostBugCommentActivity)
	w.RegisterWorkflow(internal.PostBugCommentWorkflow)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		sklog.Fatalf("Unable to start worker: %s", err)
	}
}
