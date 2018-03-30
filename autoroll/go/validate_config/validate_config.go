package main

/*
	Read and validate an AutoRoll config file.
*/

import (
	"encoding/json"
	"flag"
	"io"

	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	config = flag.String("config", "", "Config file to validate.")
)

func main() {
	common.Init()

	if *config == "" {
		sklog.Fatal("--config is required.")
	}

	var cfg roller.AutoRollerConfig
	if err := util.WithReadFile(*config, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&cfg)
	}); err != nil {
		sklog.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		sklog.Fatal(err)
	}
	for _, n := range cfg.Notifiers {
		sklog.Infof("%+v", n)
	}
}
