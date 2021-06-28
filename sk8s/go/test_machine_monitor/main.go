// A command-line application where each sub-command implements a get_* call in test_machine_monitor.py.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/revportforward"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine/targetconnect"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/machine/go/switchboard"
	"go.skia.org/infra/sk8s/go/test_machine_monitor/machine"
	"go.skia.org/infra/sk8s/go/test_machine_monitor/server"
	"go.skia.org/infra/sk8s/go/test_machine_monitor/swarming"
	"go.skia.org/infra/switchboard/go/kubeconfig"
)

// flags
var (
	configFlag     = flag.String("config", "", "The path to the configuration file.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	metadataURL    = flag.String("metadata_url", "http://metadata:8000/computeMetadata/v1/instance/service-accounts/default/token", "The URL of the metadata server that provides service account tokens.")
	port           = flag.String("port", ":11000", "HTTP service address (e.g., ':8000')")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	pythonExe      = flag.String("python_exe", "/usr/bin/python2.7", "Absolute path to Python.")
	startSwarming  = flag.Bool("start_swarming", false, "If true then start swarming_bot.zip.")
	username       = flag.String("username", "chrome-bot", "The username of the account that accepts SSH connections.")
	swarmingBotZip = flag.String("swarming_bot_zip", "/b/s/swarming_bot.zip", "Absolute path to where the swarming_bot.zip code should run from.")
)

func main() {
	common.InitWithMust(
		"test_machine_monitor",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	var instanceConfig config.InstanceConfig
	err := util.WithReadFile(*configFlag, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&instanceConfig)
	})
	if emulators.GetEmulatorHostEnvVar(emulators.PubSub) != "" {
		sklog.Fatal("Do not run with the pubsub emulator.")
	}
	if err != nil {
		sklog.Fatalf("Failed to open config file: %q: %s", *configFlag, err)
	}

	ctx := context.Background()
	machineState, err := machine.New(ctx, *local, instanceConfig, time.Now())
	if err != nil {
		sklog.Fatal("Failed to create machine: %s", err)
	}
	if err := machineState.Start(ctx); err != nil {
		sklog.Fatal("Failed to start machine: %s", err)
	}

	sklog.Infof("Starting the server.")
	machineSwarmingServer, err := server.New(machineState)
	if err != nil {
		sklog.Fatal(err)
	}
	go func() {
		sklog.Fatal(machineSwarmingServer.Start(*port))
	}()

	sklog.Infof("Starting connection to switchboard.")
	switchboardImpl, err := switchboard.New(ctx, *local, instanceConfig)
	if err != nil {
		sklog.Fatal(err)
	}
	rpf, err := revportforward.New(kubeconfig.Config, ":22", true /*useNcRev */)
	if err != nil {
		sklog.Fatal(err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatal(err)
	}
	connection := targetconnect.New(switchboardImpl, rpf, hostname, *username)
	go func() {
		err := connection.Start(ctx)
		if err != nil {
			sklog.Fatalf("Failed to maintain connection to switchboard: %s", err)
		}
	}()

	if *startSwarming {
		sklog.Infof("Starting swarming_bot.")
		bot, err := swarming.New(*pythonExe, *swarmingBotZip, *metadataURL)
		if err != nil {
			sklog.Fatal(err)
		}
		sklog.Fatal(bot.Launch(ctx))
	} else {
		select {}
	}
}
