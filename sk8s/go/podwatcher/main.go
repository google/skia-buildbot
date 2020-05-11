// podwatcher is an application that monitors pods in a k8s cluster running swarming tasks.
//
// store.WatchForDeletablePods returns a channel that will produce the name of a
// k8s for every pod that becomes eligible for deletion. Note that since these
// k8s pods are from a daemonset the pod will automatically be restarted, but
// with the latest image. This is because the spec for the rpi-warming pods is
//
//     spec:
//       updateStrategy:
//         type: OnDelete
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/sk8s/go/podwatcher/deleter"
)

var (
	// Flags.
	configFlag = flag.String("config", "", "The path to the configuration file.")
	local      = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promPort   = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
)

func main() {
	common.InitWithMust("podwatcher", common.PrometheusOpt(promPort))
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

	successfulUpdates := metrics2.GetCounter("podreader_successful_update")
	failedUpdates := metrics2.GetCounter("podreader_failed_update")

	for podname := range store.WatchForDeletablePods(ctx) {
		if err := deleter.Delete(ctx, podname); err != nil {
			failedUpdates.Inc(1)
			sklog.Errorf("Failed to update pod by deleting it: %s", err)
			continue
		}
		sklog.Infof("Deleted: %q", podname)
		successfulUpdates.Inc(1)
	}
}
