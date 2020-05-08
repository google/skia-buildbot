// pod-reaper is an application that deletes pods as directed.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/sk8s/go/podreaper/deleter"
)

var (
	// Flags.
	configFlag     = flag.String("config", "", "The path to the configuration file.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	repeatInterval = flag.Duration("repeat_interval", 15*time.Second, "How often to check for pods to reap.")
)

var (
	zero = int64(0) // Needed by metav1.DeleteOptions.
)

func main() {
	common.InitWithMust("podreaper", common.PrometheusOpt(promPort))
	ctx := context.Background()

	var instanceConfig config.InstanceConfig
	err := util.WithReadFile(*configFlag, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&instanceConfig)
	})
	if err != nil {
		sklog.Fatalf("Failed to load config from %q", *configFlag, err)
	}
	store, err := store.New(ctx, *local, instanceConfig)
	if err != nil {
		sklog.Fatalf("Failed to build store instance: %s", err)
	}

	deleter, err := deleter.New()
	if err != nil {
		sklog.Fatalf("Failed to build deleter: %s", err)
	}

	successfulDeletes := metrics2.NewCounter("podreader_successful_deletes")
	failedDeletes := metrics2.NewCounter("podreader_failed_deletes")

	for podname := range store.WatchForDeletablePods(ctx) {
		if err := deleter.Delete(podname); err != nil {
			failedDeletes.Inc(1)
			sklog.Errorf("Failed to delete pod: %s", err)
		}
		successfulDeletes.Inc(1)
	}
}
