// A command-line application where each sub-command implements a get_* call in bot_config.py
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/fs"

	"go.skia.org/infra/go/common"
	pubsubUtils "go.skia.org/infra/go/pubsub"
	"go.skia.org/infra/go/recentschannel"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/configs"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/machine/go/test_machine_monitor/foundrybotcustodian"
	"go.skia.org/infra/machine/go/test_machine_monitor/machine"
	"go.skia.org/infra/machine/go/test_machine_monitor/server"
	"go.skia.org/infra/machine/go/test_machine_monitor/swarming"
)

const (
	// Make the triggerInterrogation channel buffered so we don't lag responding
	// to HTTP requests from the Swarming bot.
	interrogationChannelSize = 10
)

// flags
var (
	configFlag         = flag.String("config", "prod.json", "The name to the configuration file, such as prod.json or test.json, as found in machine/go/configs.")
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	machineServerHost  = flag.String("machine_server", "https://machines.skia.org", "A URL with the scheme and domain name of the machine hosting the machine server API.")
	metadataURL        = flag.String("metadata_url", "http://metadata:8000/computeMetadata/v1/instance/service-accounts/default/token", "The URL of the metadata server that provides service account tokens.")
	port               = flag.String("port", ":11000", "HTTP service address (e.g., ':8000')")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	pythonExe          = flag.String("python_exe", "", "Absolute path to Python.")
	startFoundryBot    = flag.Bool("start_foundry_bot", false, "Start the Foundry Bot daemon (if not in maintenance mode), which listens for and runs Bazel RBE jobs.")
	foundryBotInstance = flag.String("foundry_bot_instance", "projects/skia-rbe/instances/default_instance", "Path to GCP instance under which RBE tasks should run")
	foundryBotPath     = flag.String("foundry_bot_path", "/usr/local/bin/bot.1", "Path to Foundry Bot executable")
	startSwarming      = flag.Bool("start_swarming", false, "Start swarming_bot.zip.")
	swarmingBotZip     = flag.String("swarming_bot_zip", "", "Absolute path to where the swarming_bot.zip code should run from.")
	username           = flag.String("username", "chrome-bot", "The username of the account that accepts SSH connections.")
)

var (
	// Version can be changed via -ldflags.
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

	// Wait until we hear the up-to-date maintenance mode from machineserver
	// before launching Foundry Bot.
	wantFoundryBotUpCh := recentschannel.New[bool](1)
	reportAvailability := func(m *machine.Machine) {
		wantFoundryBotUpCh.Send(m.IsAvailable())
	}

	ctx := context.Background()
	triggerInterrogationCh := make(chan bool, interrogationChannelSize)
	machineState, err := machine.New(ctx, *local, instanceConfig, Version, *startSwarming, *machineServerHost, *startFoundryBot, reportAvailability, triggerInterrogationCh)
	if err != nil {
		sklog.Fatal("Failed to create machine: %s", err)
	}
	if err := machineState.Start(ctx); err != nil {
		sklog.Fatal("Failed to start machine: %s", err)
	}

	sklog.Infof("Starting the server.")
	machineSwarmingServer, err := server.New(machineState, triggerInterrogationCh)
	if err != nil {
		sklog.Fatal(err)
	}
	go func() {
		sklog.Fatal(machineSwarmingServer.Start(*port))
	}()

	if *startFoundryBot {
		err := foundrybotcustodian.Start(ctx, *foundryBotPath, *foundryBotInstance, wantFoundryBotUpCh)
		if err != nil {
			sklog.Fatalf("Failed to start Foundry Bot Custodian: %s", err)
		}
	}

	if *startSwarming {
		if *pythonExe == "" {
			sklog.Fatalf("Flag --python_exe is required when --start_swarming is true.")
		}
		if *swarmingBotZip == "" {
			sklog.Fatalf("Flag --swarming_bot_zip is required when --start_swarming is true.")
		}
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
