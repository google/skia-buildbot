package main

import (
	"context"
	"flag"
	"fmt"
	"os/user"
	"time"

	"github.com/google/uuid"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/internal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

var (
	// Run the following command to portforward Temporal service so the client can connect to it.
	// kubectl port-forward service/temporal --address 0.0.0.0 -n temporal 7233:7233
	hostPort  = flag.String("hostPort", "localhost:7233", "Host the worker connects to.")
	namespace = flag.String("namespace", "default", "The namespace the worker registered to.")
	taskQueue = flag.String("taskQueue", "", "Task queue name registered to worker services.")

	// common flags
	project   = flag.String("project", "chromeperf", "GCP project of the target BQ table.")
	dataset   = flag.String("dataset", "experiments", "Dataset name of the target BQ table in the provided GCP project.")
	tableName = flag.String("tableName", "mac_raw_results", "GCP project of the target BQ table.")
	benchmark = flag.String("benchmark", "speedometer3", "Benchmark to test.")

	// startChromeExperiment flags
	commit     = flag.String("commit", "40106da1ba5cc86737a7aba072833b21a074a6b8", "Git commit hash to build Chrome.")
	bot        = flag.String("bot", "mac-m1_mini_2020-perf-pgo", "Target bot for the Chrome build.")
	iterations = flag.Int("iterations", 20, "The number of times to run a benchmark task.")

	// fetchExperimentResult flags
	workflowId = flag.String("workflowId", "", "The temporal workflow associated with Swarming runs.")

	// Run flags
	runExperiment = flag.Bool("runExperiment", false, "Flag to run experiment")
)

func defaultWorkflowOptions() client.StartWorkflowOptions {
	return client.StartWorkflowOptions{
		ID:        uuid.New().String(),
		TaskQueue: *taskQueue,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    30 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    1,
		},
	}
}

// startExperiment builds Chrome and triggers ths swarming tasks.
// The swarming task ids are exported to BQ.
// It's expected that collectResults will run after to collect the results and
// upload those to BQ.
func startChromeExperiment(c client.Client) error {
	ctx := context.Background()

	workflowUUID := uuid.New().String()
	req := &internal.TestAndExportParams{
		WorkflowID: workflowUUID,
		Benchmark:  *benchmark,
		Bot:        *bot,
		GitHash:    *commit,
		Iterations: *iterations,
		Project:    *project,
		Dataset:    *dataset,
		TableName:  *tableName,
	}

	_, err := c.ExecuteWorkflow(ctx, defaultWorkflowOptions(), workflows.TestAndExport, req)
	if err != nil {
		return skerr.Wrapf(err, "Failed to execute workflow")
	}

	return nil
}

func main() {
	flag.Parse()

	if *taskQueue == "" {
		if u, err := user.Current(); err != nil {
			sklog.Fatalf("Unable to get the current user: %s", err)
		} else {
			*taskQueue = fmt.Sprintf("localhost.%s", u.Username)
		}
	}

	// The client is a heavyweight object that should be created once per process.
	c, err := client.Dial(client.Options{
		HostPort:  *hostPort,
		Namespace: *namespace,
	})
	if err != nil {
		sklog.Errorf("Unable to create client", err)
		return
	}
	defer c.Close()

	if *runExperiment {
		err = startChromeExperiment(c)
		if err != nil {
			sklog.Errorf("Workflow failed: ", err)
		}
	}

	return
}
