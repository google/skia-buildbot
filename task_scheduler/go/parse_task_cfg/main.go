package main

/*
	Convenience program for verification of task config files.
*/

import (
	"flag"
	"io/ioutil"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/task_scheduler/go/specs"
)

var (
	cfgFile = flag.String("cfg_file", "", "Config file to parse.")
)

func main() {
	common.Init()
	defer common.LogPanic()

	b, err := ioutil.ReadFile(*cfgFile)
	if err != nil {
		glog.Fatal(err)
	}
	cfg, err := specs.ParseTasksCfg(string(b))
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Task config:")
	for name, t := range cfg.Tasks {
		glog.Infof("  %s: %v", name, t)
	}
}
