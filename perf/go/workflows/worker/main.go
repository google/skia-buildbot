package main

import (
	"flag"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/workflows"
	"go.skia.org/infra/perf/go/workflows/internal"
	"go.skia.org/infra/temporal/go/metrics"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var (
	// Run the following command to portforward Temporal service so the client can connect to it.
	// kubectl port-forward service/temporal --address 0.0.0.0 -n temporal 7233:7233
	hostPort  = flag.String("hostPort", "localhost:7233", "Host the worker connects to.")
	namespace = flag.String("namespace", "default", "The namespace the worker registered to.")
	taskQueue = flag.String("taskQueue", "localhost.dev", "Task queue name registered to worker services.")
)

func main() {
	flag.Parse()

	// The client is a heavyweight object that should be created once per process.
	c, err := client.Dial(client.Options{
		MetricsHandler: metrics.NewMetricsHandler(map[string]string{}, nil),
		HostPort:       *hostPort,
		Namespace:      *namespace,
	})
	if err != nil {
		sklog.Errorf("Unable to create client", err)
		return
	}
	defer c.Close()
	w := worker.New(c, *taskQueue, worker.Options{})
	csa := &internal.CulpritServiceActivity{}
	w.RegisterActivity(csa)
	w.RegisterWorkflowWithOptions(internal.ProcessCulpritWorkflow, workflow.RegisterOptions{Name: workflows.ProcessCulprit})

	agsa := &internal.AnomalyGroupServiceActivity{}
	w.RegisterActivity(agsa)
	w.RegisterWorkflowWithOptions(internal.MaybeTriggerBisectionWorkflow, workflow.RegisterOptions{Name: workflows.MaybeTriggerBisection})

	gsa := &internal.GerritServiceActivity{}
	w.RegisterActivity(gsa)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		sklog.Fatalf("Unable to start worker: %s", err)
	}
}
