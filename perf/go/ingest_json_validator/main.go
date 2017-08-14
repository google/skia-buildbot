package main

import (
	"flag"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/sklog"
)

var (
	configFilename = flag.String("config_filename", "default.json5", "Configuration file in TOML format.")
)

func main() {
	common.Init()
	config, err := sharedconfig.ConfigFromJson5File(*configFilename)
	if err != nil {
		sklog.Fatalf("Unable to read config file %s. Got error: %s", *configFilename, err)
	}
	if len(config.Ingesters) == 0 {
		sklog.Fatalf("No ingesters configured: %s", *configFilename)
	}
}
