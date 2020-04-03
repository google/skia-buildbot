// A command-line application where each sub-command implements a get_* call in bot_config.py.
package main

import (
	"context"
	"flag"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/sk8s/go/bot_config/server"
	"go.skia.org/infra/sk8s/go/bot_config/swarming"
)

// flags
var (
	metadataURL    = flag.String("metadata_url", "http://metadata:8000/computeMetadata/v1/instance/service-accounts/default/token", "The URL of the metadata server that provides service account tokens.")
	port           = flag.String("port", ":11000", "HTTP service address (e.g., ':8000')")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	pythonExe      = flag.String("python_exe", "/usr/bin/python2.7", "Absolute path to Python.")
	startSwarming  = flag.Bool("start_swarming", false, "If true then start swarming_bot.zip.")
	swarmingBotZip = flag.String("swarming_bot_zip", "/b/s/swarming_bot.zip", "Absolute path to where the swarming_bot.zip code should run from.")
)

func main() {
	common.InitWithMust(
		"bot_config",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	sklog.Infof("Starting the server.")
	s, err := server.New()
	if err != nil {
		sklog.Fatal(err)
	}
	go func() {
		sklog.Fatal(s.Start(*port))
	}()

	if *startSwarming {
		sklog.Infof("Starting swarming_bot.")
		bot := swarming.New(*pythonExe, *swarmingBotZip, *metadataURL)
		sklog.Fatal(bot.Launch(context.Background()))
	} else {
		select {}
	}
}
