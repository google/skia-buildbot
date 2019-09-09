/*
	Common initialization for master scripts.
*/

package master_common

import (
	"flag"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
)

var (
	Local = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
)

func Init(appName string) {
	common.InitWithMust(appName)
	initRest()
}

func InitWithMetrics2(appName string, promPort *string) {
	common.InitWithMust(
		appName,
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	initRest()
}

// remove lots of things from here.
func initRest() {
	if *Local {
		util.SetVarsForLocal()
	}
}
