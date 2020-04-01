// A command-line application where each sub-command implements a get_* call in bot_config.py.
package main

import (
	"flag"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/sk8s/go/bot_config/server"
)

// flags
var (
	port     = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

func main() {
	common.InitWithMust(
		"bot_config",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	server, err := server.NewServer()
	if err != nil {
		sklog.Fatal(err)
	}
	server.Start(*port)
}
