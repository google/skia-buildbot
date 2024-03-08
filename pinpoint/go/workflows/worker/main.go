package main

import (
	"flag"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/internal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var (
	hostPort  = flag.String("hostPort", "localhost:7233", "Host the worker connects to.")
	namespace = flag.String("namespace", "default", "The namespace the worker registered to.")
	taskQueue = flag.String("taskQueue", "localhost.dev", "Task queue name registered to worker services.")
)

func main() {
	flag.Parse()

	// The client and worker are heavyweight objects that should be created once per process.
	c, err := client.Dial(client.Options{
		HostPort:  *hostPort,
		Namespace: *namespace,
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

	w.RegisterActivity(internal.ComparePerformanceActivity)
	w.RegisterActivity(internal.FindMidCommitActivity)
	w.RegisterWorkflowWithOptions(internal.BisectWorkflow, workflow.RegisterOptions{Name: workflows.Bisect})

	err = w.Run(worker.InterruptCh())
	if err != nil {
		sklog.Fatalf("Unable to start worker: %s", err)
	}
}
