package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/deploy"
	gce_autoscaler "go.skia.org/infra/go/gce/autoscaler"
	"go.skia.org/infra/go/gce/swarming/instance_types"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/autoscaler"
	"go.skia.org/infra/task_scheduler/go/candidate_stats"
	"go.skia.org/infra/task_scheduler/go/db/pubsub"
)

var (
	deployment     = deploy.Flag("deployment")
	dev            = flag.Bool("dev", false, "Whether or not the bots connect to chromium-swarm-dev.")
	instances      = flag.String("instances", "", "Which instances to create/delete, eg. \"2,3-10,22\"")
	instanceType   = flag.String("type", "", fmt.Sprintf("Type of instance; one of: %v", instance_types.VALID_INSTANCE_TYPES))
	internal       = flag.Bool("internal", false, "Whether or not the bots are internal.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	name           = flag.String("name", "", "Name to uniquely identify this autoscaler.")
	port           = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	project        = flag.String("project", "", "GCE project in which to scale instances.")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	swarmingServer = flag.String("swarming_server", "", "Address of the Swarming server.")
	workdir        = flag.String("workdir", ".", "Working directory.")
)

func main() {
	common.InitWithMust(
		"task-scheduler-autoscaler",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	defer common.Defer()
	skiaversion.MustLogVersion()

	// Validation.
	if !util.In(*instanceType, instance_types.VALID_INSTANCE_TYPES) {
		sklog.Fatalf("--type must be one of %v", instance_types.VALID_INSTANCE_TYPES)
	}

	// Get the absolute workdir.
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Setup.
	_, filename, _, _ := runtime.Caller(0)
	checkoutRoot := path.Dir(path.Dir(path.Dir(path.Dir(filename))))
	ctx := context.Background()

	getInstance, err := instance_types.GetInstanceConstructor(ctx, *instanceType, checkoutRoot, wdAbs, *internal, *dev)
	if err != nil {
		sklog.Fatal(err)
	}
	project, zone := instance_types.GetProjectAndZone(*instanceType, *internal)

	ts, err := auth.NewDefaultTokenSource(*local, pubsub.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	statsClient, err := candidate_stats.NewClient(ctx, ts, *deployment)
	if err != nil {
		sklog.Fatal(err)
	}
	instanceSet, err := gce_autoscaler.GetInstanceSet(*instances, getInstance)
	if err != nil {
		sklog.Fatal(err)
	}
	as, err := autoscaler.New(project, zone, *swarmingServer, *name, ts, instanceSet, time.Now())
	if err != nil {
		sklog.Fatal(err)
	}

	// Start receiving stats from Task Scheduler.
	lv := metrics2.NewLiveness("last_successful_task_scheduler_autoscale", map[string]string{
		"scaler": *name,
	})
	if err := statsClient.Receive(ctx, *name, func(ctx context.Context, stats []*candidate_stats.CandidateStats) {
		count := 0
		scalerDims := as.Dimensions()
		for _, s := range stats {
			match := true
			for _, dim := range s.Dimensions {
				if !scalerDims[dim] {
					match = false
					break
				}
			}
			if match {
				count += s.Count
			}
		}
		if err := as.Autoscale(count, time.Now()); err == nil {
			lv.Reset()
		} else {
			sklog.Errorf("Failed to autoscale: %s", err)
		}
	}); err != nil {
		sklog.Fatal(err)
	}
}
