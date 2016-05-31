/*
	Common initialization for master scripts.
*/

package master_common

import (
	"flag"

	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
)

var (
	Local         = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	localFrontend = flag.String("local_frontend", "http://localhost:8000/", "When local is true, base URL where CTFE is running.")
)

func Init() {
	common.Init()
	initRest()
}

func InitWithMetrics2(appName string, influxHost, influxUser, influxPassword, influxDatabase *string) {
	// Minor hack: pass true for local param to avoid attempting to read the influx* params from GCE metadata.
	alwaysUseGivenInfluxCredentials := true
	common.InitWithMetrics2(appName, influxHost, influxUser, influxPassword, influxDatabase, &alwaysUseGivenInfluxCredentials)
	initRest()
}

func initRest() {
	if *Local {
		frontend.InitForTesting(*localFrontend)
		util.SetVarsForLocal()
	} else {
		frontend.MustInit()
	}
}
