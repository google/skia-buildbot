// Functions and variables helping with communication with CT frontend.
package frontend

import (
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	"go.skia.org/infra/go/httputils"
)

var (
	WebappRoot         string
	InternalWebappRoot string
	// Webapp subparts.
	AdminTasksWebapp                         string
	UpdateRecreatePageSetsTasksWebapp        string
	UpdateRecreateWebpageArchivesTasksWebapp string
	LuaTasksWebapp                           string
	UpdateLuaTasksWebapp                     string
	CaptureSKPsTasksWebapp                   string
	UpdateCaptureSKPsTasksWebapp             string
	MetricsAnalysisTasksWebapp               string
	UpdateMetricsAnalysisTasksWebapp         string
	ChromiumPerfTasksWebapp                  string
	ChromiumAnalysisTasksWebapp              string
	UpdateChromiumPerfTasksWebapp            string
	ChromiumBuildTasksWebapp                 string
	UpdateChromiumBuildTasksWebapp           string
	GetOldestPendingTaskWebapp               string
	TerminateRunningTasksWebapp              string
)

var httpClient = httputils.NewTimeoutClient()

// Initializes *Webapp URLs above and sets up authentication credentials for UpdateWebappTaskV2.
func MustInit(webapp_root string) {
	WebappRoot = webapp_root
	AdminTasksWebapp = webapp_root + ctfeutil.ADMIN_TASK_URI
	LuaTasksWebapp = webapp_root + ctfeutil.LUA_SCRIPT_URI
	CaptureSKPsTasksWebapp = webapp_root + ctfeutil.CAPTURE_SKPS_URI
	MetricsAnalysisTasksWebapp = webapp_root + ctfeutil.METRICS_ANALYSIS_URI
	ChromiumPerfTasksWebapp = webapp_root + ctfeutil.CHROMIUM_PERF_URI
	ChromiumAnalysisTasksWebapp = webapp_root + ctfeutil.CHROMIUM_ANALYSIS_URI
	ChromiumBuildTasksWebapp = webapp_root + ctfeutil.CHROMIUM_BUILD_URI
}
