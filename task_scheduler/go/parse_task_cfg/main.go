package main

/*
	Convenience program for verification of task config files.
*/

import (
	"flag"
	"io/ioutil"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/specs"
)

var (
	cfgFile = flag.String("cfg_file", "", "Config file to parse.")
)

func main() {
	common.Init()

	b, err := ioutil.ReadFile(*cfgFile)
	if err != nil {
		sklog.Fatal(err)
	}
	cfg, err := specs.ParseTasksCfg(string(b))
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Task config:")
	for name, t := range cfg.Tasks {
		sklog.Infof("  %s: %v", name, t)
	}
}
