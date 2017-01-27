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

func Init(appName string) {
	common.InitWithMust(appName, common.CloudLoggingOpt())
	initRest()
}

func InitWithMetrics2(appName string, influxHost, influxUser, influxPassword, influxDatabase *string) {
	common.InitWithCloudLogging(appName, influxHost, influxUser, influxPassword, influxDatabase, Local)
	initRest()
}

func initRest() {
	if *Local {
		frontend.InitForTesting(*localFrontend)
		util.SetVarsForLocal()
	} else {
		frontend.MustInit()
		util.MailInit()
	}
}
