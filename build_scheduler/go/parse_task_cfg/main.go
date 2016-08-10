package main

/*
	Convenience program for verification of task config files.
*/

import (
	"flag"
	"io/ioutil"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/build_scheduler/go/task_scheduler"
	"go.skia.org/infra/go/common"
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
	if _, err := task_scheduler.ParseTasksCfg(string(b)); err != nil {
		glog.Fatal(err)
	}
}
