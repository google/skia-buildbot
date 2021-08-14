// powercycle_server_ansible is an application that watches the machine
// firestore database and powercycles test machines that need powercycling.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/fs"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/configs"
	"go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/skolo/go/powercycle"
	"go.skia.org/infra/skolo/sys"
)

var (
	// Flags.
	configFlag               = flag.String("config", "", "The name of the configuration file.")
	local                    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	powercycleConfigFilename = flag.String("powercycle_config", "", "The name of the config file for powercycle.Controller.")
	promPort                 = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
)

func main() {
	common.InitWithMust(
		"powercycle_server_ansible",
		common.PrometheusOpt(promPort),
		common.CloudLogging(local, "skia-public"),
	)
	ctx := context.Background()

	if *powercycleConfigFilename == "" {
		sklog.Fatal("--powercycle_config flag must be supplied.")
	}

	if *configFlag == "" {
		sklog.Fatal("--config flag must be supplied.")
	}

	// Construct store.
	var instanceConfig config.InstanceConfig
	b, err := fs.ReadFile(configs.Configs, *configFlag)
	if err != nil {
		sklog.Fatalf("Failed to read config file %q: %s", *configFlag, err)
	}
	err = json.Unmarshal(b, &instanceConfig)
	if err != nil {
		sklog.Fatal(err)
	}

	store, err := store.New(ctx, *local, instanceConfig)
	if err != nil {
		sklog.Fatalf("Failed to build store instance: %s", err)
	}

	// Construct powercycle controller.
	powerCycleConfigBytes, err := fs.ReadFile(sys.Sys, *powercycleConfigFilename)
	if err != nil {
		sklog.Fatalf("Failed to read config file %q: %s", *powercycleConfigFilename, err)
	}

	sklog.Info("Building powercycle.Controller from %q", *powercycleConfigFilename)
	powercycleController, err := powercycle.ControllerFromJSON5Bytes(ctx, powerCycleConfigBytes, true)
	if err != nil {
		sklog.Fatalf("Failed to instantiate powercycle.Controller: %s", err)
	}

	// Start a loop that does a firestore onsnapshot watcher that gets machine names
	// that need to be power-cycled.
	for machineID := range store.WatchForPowerCycle(ctx) {
		// TODO(jcgregorio) We should filter on rack and ignore devices not on the local rack.
		if err := powercycleController.PowerCycle(ctx, powercycle.DeviceID(machineID), 0); err != nil {
			sklog.Errorf("Failed to powercycle %q: %s", machineID, err)
		} else {
			sklog.Infof("Successfully powercycled: %q", machineID)
		}
	}
	sklog.Info("Exiting WatchForPowerCycle")
}
