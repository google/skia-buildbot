package main

import (
	"context"
	"flag"
	"os"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/workflows"
	"go.skia.org/infra/perf/go/workflows/internal"
	"go.skia.org/infra/pinpoint/go/pinpoint"
	"go.skia.org/infra/temporal/go/metrics"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var (
	// Run the following command to portforward Temporal service so the client can connect to it.
	// kubectl port-forward service/temporal --address 0.0.0.0 -n temporal 7233:7233
	hostPort  = flag.String("hostPort", "localhost:7233", "Host the worker connects to.")
	promPort  = flag.String("promPort", ":8000", "Prometheus port that it listens on.")
	namespace = flag.String("namespace", "default", "The namespace the worker registered to.")
	taskQueue = flag.String(
		"taskQueue",
		"localhost.dev",
		"Task queue name registered to worker services.",
	)
	useMockPinpoint = flag.Bool(
		"useMockPinpoint",
		false,
		"Use a local mock pinpoint client instead of live API integration.",
	)
	local = flag.Bool(
		"local",
		false,
		"Whether running in local dev/demo mode. Allows insecure gRPC connections to local backend services.",
	)
)

func main() {
	flag.Parse()
	common.InitWithMust(
		"grouping-worker",
		common.PrometheusOpt(promPort),
	)

	// The client is a heavyweight object that should be created once per process.
	c, err := client.Dial(client.Options{
		MetricsHandler: metrics.NewMetricsHandler(map[string]string{}, nil),
		HostPort:       *hostPort,
		Namespace:      *namespace,
	})
	if err != nil {
		sklog.Fatalf("Unable to create client: %s", err)
	}
	defer c.Close()
	workerOpts := worker.Options{}
	if buildID := os.Getenv("TEMPORAL_WORKER_BUILD_ID"); buildID != "" {
		workerOpts.BuildID = buildID
		workerOpts.UseBuildIDForVersioning = true
		if deploymentName := os.Getenv("TEMPORAL_DEPLOYMENT_NAME"); deploymentName != "" {
			workerOpts.DeploymentOptions = worker.DeploymentOptions{
				DeploymentSeriesName:      deploymentName,
				DefaultVersioningBehavior: workflow.VersioningBehaviorAutoUpgrade,
			}
		}
	}
	w := worker.New(c, *taskQueue, workerOpts)
	csa := internal.NewCulpritServiceActivity(*local)
	w.RegisterActivity(csa)
	w.RegisterWorkflowWithOptions(
		internal.ProcessCulpritWorkflow,
		workflow.RegisterOptions{Name: workflows.ProcessCulprit},
	)

	if *useMockPinpoint {
		if !devMode {
			sklog.Fatalf("Mock Pinpoint Client is only available in development builds. Recompile with -tags dev.")
		}
		registerMockActivities(w)
	} else {
		pinpointClient, err := pinpoint.New(context.Background())
		if err != nil {
			sklog.Fatalf("Unable to create pinpoint client: %s", err)
		}
		w.RegisterActivity(pinpointClient)
	}

	agsa := internal.NewAnomalyGroupServiceActivity(*local)
	bsa := internal.NewAutobisectionServiceActivity(*local)

	w.RegisterActivity(agsa)
	w.RegisterActivity(bsa)

	w.RegisterWorkflowWithOptions(
		internal.MaybeTriggerBisectionWorkflow,
		workflow.RegisterOptions{Name: workflows.MaybeTriggerBisection},
	)

	gsa := &internal.GerritServiceActivity{}
	w.RegisterActivity(gsa)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		sklog.Fatalf("Unable to start worker: %s", err)
	}
}
