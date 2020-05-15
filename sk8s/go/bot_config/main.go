// A command-line application where each sub-command implements a get_* call in bot_config.py.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/sk8s/go/bot_config/machine"
	"go.skia.org/infra/sk8s/go/bot_config/server"
	"go.skia.org/infra/sk8s/go/bot_config/swarming"
)

// flags
var (
	configFlag               = flag.String("config", "", "The path to the configuration file.")
	local                    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	metadataURL              = flag.String("metadata_url", "http://metadata:8000/computeMetadata/v1/instance/service-accounts/default/token", "The URL of the metadata server that provides service account tokens.")
	port                     = flag.String("port", ":11000", "HTTP service address (e.g., ':8000')")
	powercycleConfigFilename = flag.String("powercycle_config", "", "The name of the config file for powercycle.Controller.")
	promPort                 = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	pythonExe                = flag.String("python_exe", "/usr/bin/python2.7", "Absolute path to Python.")
	startSwarming            = flag.Bool("start_swarming", false, "If true then start swarming_bot.zip.")
	swarmingBotZip           = flag.String("swarming_bot_zip", "/b/s/swarming_bot.zip", "Absolute path to where the swarming_bot.zip code should run from.")
)

func main() {
	common.InitWithMust(
		"bot_config",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	var instanceConfig config.InstanceConfig
	err := util.WithReadFile(*configFlag, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&instanceConfig)
	})
	if os.Getenv("PUBSUB_EMULATOR_HOST") != "" {
		sklog.Fatal("Do not run with the pubsub emulator.")
	}
	if err != nil {
		sklog.Fatalf("Failed to open config file: %q: %s", *configFlag, err)
	}

	ctx := context.Background()
	m, err := machine.New(ctx, *local, instanceConfig, *powercycleConfigFilename)
	if err != nil {
		sklog.Fatal("Failed to create machine: %s", err)
	}
	if err := m.Start(ctx); err != nil {
		sklog.Fatal("Failed to start machine: %s", err)
	}

	sklog.Infof("Starting the server.")
	s, err := server.New(m)
	if err != nil {
		sklog.Fatal(err)
	}
	go func() {
		sklog.Fatal(s.Start(*port))
	}()

	if *startSwarming {
		sklog.Infof("Starting swarming_bot.")
		bot, err := swarming.New(*pythonExe, *swarmingBotZip, *metadataURL)
		if err != nil {
			sklog.Fatal(err)
		}
		sklog.Fatal(bot.Launch(context.Background()))
	} else {
		select {}
	}
}
