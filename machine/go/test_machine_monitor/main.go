// A command-line application where each sub-command implements a get_* call in test_machine_monitor.py.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"time"

	"go.skia.org/infra/go/common"
	pubsubUtils "go.skia.org/infra/go/pubsub"
	"go.skia.org/infra/go/revportforward"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/configs"
	"go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machine/targetconnect"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/machine/go/switchboard"
	"go.skia.org/infra/machine/go/test_machine_monitor/machine"
	"go.skia.org/infra/machine/go/test_machine_monitor/server"
	"go.skia.org/infra/machine/go/test_machine_monitor/swarming"
	"go.skia.org/infra/switchboard/go/kubeconfig"
)

// flags
var (
	configFlag       = flag.String("config", "prod.json", "The name to the configuration file, such as prod.json or test.json, as found in machine/go/configs.")
	local            = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	metadataURL      = flag.String("metadata_url", "http://metadata:8000/computeMetadata/v1/instance/service-accounts/default/token", "The URL of the metadata server that provides service account tokens.")
	port             = flag.String("port", ":11000", "HTTP service address (e.g., ':8000')")
	promPort         = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	pythonExe        = flag.String("python_exe", "/usr/bin/python2.7", "Absolute path to Python.")
	startSwarming    = flag.Bool("start_swarming", false, "If true then start swarming_bot.zip.")
	startSwitchboard = flag.Bool("start_switchboard", false, "If true then establish a connection to skia-switchboard.")
	username         = flag.String("username", "chrome-bot", "The username of the account that accepts SSH connections.")
	swarmingBotZip   = flag.String("swarming_bot_zip", "/b/s/swarming_bot.zip", "Absolute path to where the swarming_bot.zip code should run from.")
)

var (
	// Version can be changed via -ldflags.
	// This should be reverted
	Version = "development"
)

func main() {
	common.InitWithMust(
		"test_machine_monitor",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
		common.CloudLogging(local, "skia-public"),
	)
	sklog.Infof("Version: %s", Version)
	var instanceConfig config.InstanceConfig
	b, err := fs.ReadFile(configs.Configs, *configFlag)
	if err != nil {
		sklog.Fatalf("Failed to read config file %q: %s", *configFlag, err)
	}
	err = json.Unmarshal(b, &instanceConfig)
	if err != nil {
		sklog.Fatal(err)
	}
	pubsubUtils.EnsureNotEmulator()
	if err != nil {
		sklog.Fatalf("Failed to open config file: %q: %s", *configFlag, err)
	}

	ctx := context.Background()
	machineState, err := machine.New(ctx, *local, instanceConfig, time.Now(), Version, *startSwarming)
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
	store, err := store.New(ctx, *local, instanceConfig)
	if err != nil {
		sklog.Fatal(err)
	}

	if *startSwitchboard {
		connection := targetconnect.New(switchboardImpl, rpf, store, hostname, *username)
		go func() {
			err := connection.Start(ctx)
			if err != nil {
				sklog.Fatalf("Failed to maintain connection to switchboard: %s", err)
			}
		}()
	}

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
