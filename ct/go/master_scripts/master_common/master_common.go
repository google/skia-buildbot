/*
	Common initialization for master scripts.
*/

package master_common

import (
	"flag"

	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
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

	// Webapp URLs
	AdminTasksWebapp            string
	LuaTasksWebapp              string
	CaptureSKPsTasksWebapp      string
	MetricsAnalysisTasksWebapp  string
	ChromiumPerfTasksWebapp     string
	ChromiumAnalysisTasksWebapp string
	ChromiumBuildTasksWebapp    string
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

func initRest() {
	AdminTasksWebapp = *ctfeURL + ctfeutil.ADMIN_TASK_URI
	LuaTasksWebapp = *ctfeURL + ctfeutil.LUA_SCRIPT_URI
	CaptureSKPsTasksWebapp = *ctfeURL + ctfeutil.CAPTURE_SKPS_URI
	MetricsAnalysisTasksWebapp = *ctfeURL + ctfeutil.METRICS_ANALYSIS_URI
	ChromiumPerfTasksWebapp = *ctfeURL + ctfeutil.CHROMIUM_PERF_URI
	ChromiumAnalysisTasksWebapp = *ctfeURL + ctfeutil.CHROMIUM_ANALYSIS_URI
	ChromiumBuildTasksWebapp = *ctfeURL + ctfeutil.CHROMIUM_BUILD_URI

	// Initialize the datastore.
	if err := ds.Init(*projectName, *namespace); err != nil {
		sklog.Fatal(err)
	}
	if *Local {
		util.SetVarsForLocal()
	} else {
		// Initialize mailing library.
		if err := util.MailInit(); err != nil {
			sklog.Fatal(err)
		}
	}
}
