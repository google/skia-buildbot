package main

/*
	Convenience program for verification of task config files.
*/

import (
	"flag"
	"io/ioutil"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/task_scheduler/go/scheduling"
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
	if _, err := scheduling.ParseTasksCfg(string(b)); err != nil {
		glog.Fatal(err)
	}
}
