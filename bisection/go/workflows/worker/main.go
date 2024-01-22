package main

import (
	"flag"

	"go.skia.org/infra/bisection/go/workflows"
	"go.skia.org/infra/bisection/go/workflows/internal"
	"go.skia.org/infra/go/sklog"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var (
	hostPort  = flag.String("hostPort", "localhost:7233", "Host the worker connects to.")
	taskQueue = flag.String("taskQueue", "localhost.dev", "Task queue name registered to worker services.")
)

func main() {
	flag.Parse()

	// The client and worker are heavyweight objects that should be created once per process.
	c, err := client.Dial(client.Options{
		HostPort: *hostPort,
	})
	if err != nil {
		sklog.Fatalf("Unable to create client: %s", err)
	}
	defer c.Close()

	w := worker.New(c, *taskQueue, worker.Options{})

	w.RegisterWorkflowWithOptions(internal.BuildChrome, workflow.RegisterOptions{Name: workflows.BuildChrome})

	bca := &internal.BuildChromeActivity{}
	w.RegisterActivity(bca)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		sklog.Fatalf("Unable to start worker: %s", err)
	}
}
