/*
	Common initialization for master scripts.
*/

package master_common

import (
	"flag"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
)

var (
	ctfeURL = flag.String("ctfe_url", "https://ct.skia.org/", "The CTFE frontend URL.")
	Local   = flag.Bool("local", false, "Running locally if true. As opposed to in production.")

	// Datastore params
	namespace   = flag.String("ds_namespace", "cluster-telemetry", "The Cloud Datastore namespace, such as 'cluster-telemetry'.")
	projectName = flag.String("ds_project_name", "skia-public", "The Google Cloud project name.")
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
	// Initialize the datastore.
	if err := ds.Init(*projectName, *namespace); err != nil {
		sklog.Fatal(err)
	}
	if *Local {
		util.SetVarsForLocal()
	}
}
