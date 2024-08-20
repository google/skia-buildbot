package main

import (
	"flag"
	"fmt"
	"os/user"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/temporal/go/metrics"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

const appName = "perf-upload-worker"

var (
	hostPort  = flag.String("host_port", "localhost:7233", "Host the worker connects to.")
	promPort  = flag.String("prom_port", ":8000", "Prometheus port that it listens on.")
	namespace = flag.String("namespace", "default", "The namespace the worker registered to.")
	taskQueue = flag.String("task_queue", "", "Task queue name registered to worker services.")
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

	err = w.Run(worker.InterruptCh())
	if err != nil {
		sklog.Fatalf("Unable to start worker: %s", err)
	}
}
