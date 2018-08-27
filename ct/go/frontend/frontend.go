// Functions and variables helping with communication with CT frontend.
package frontend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"go.skia.org/infra/ct/go/ctfe/pending_tasks"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	WEBAPP_ROOT          = "https://ct.skia.org/"
	INTERNAL_WEBAPP_ROOT = "http://ctfe:9000/"
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
	PixelDiffTasksWebapp                     string
	UpdatePixelDiffTasksWebapp               string
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
func MustInit(webapp_root, internal_webapp_root string) {
	WebappRoot = webapp_root
	AdminTasksWebapp = webapp_root + ctfeutil.ADMIN_TASK_URI
	LuaTasksWebapp = webapp_root + ctfeutil.LUA_SCRIPT_URI
	CaptureSKPsTasksWebapp = webapp_root + ctfeutil.CAPTURE_SKPS_URI
	PixelDiffTasksWebapp = webapp_root + ctfeutil.PIXEL_DIFF_URI
	MetricsAnalysisTasksWebapp = webapp_root + ctfeutil.METRICS_ANALYSIS_URI
	ChromiumPerfTasksWebapp = webapp_root + ctfeutil.CHROMIUM_PERF_URI
	ChromiumAnalysisTasksWebapp = webapp_root + ctfeutil.CHROMIUM_ANALYSIS_URI
	ChromiumBuildTasksWebapp = webapp_root + ctfeutil.CHROMIUM_BUILD_URI

	// URLs that are accessible only through internal ports.
	InternalWebappRoot = internal_webapp_root
	GetOldestPendingTaskWebapp = internal_webapp_root + ctfeutil.GET_OLDEST_PENDING_TASK_URI
	TerminateRunningTasksWebapp = internal_webapp_root + ctfeutil.TERMINATE_RUNNING_TASKS_URI
}

// Common functions

func GetOldestPendingTaskV2() (task_common.Task, error) {
	resp, err := httpClient.Get(GetOldestPendingTaskWebapp)
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		response, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s returned %d: %s", GetOldestPendingTaskWebapp, resp.StatusCode, response)
	}
	return pending_tasks.DecodeTask(resp.Body)
}

func TerminateRunningTasks() error {
	resp, err := httpClient.Post(TerminateRunningTasksWebapp, "application/json", nil)
	if err != nil {
		return fmt.Errorf("Could not terminate running tasks: %s", err)
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		response, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("POST %s returned %d: %s", TerminateRunningTasksWebapp, resp.StatusCode, response)
	}
	return nil
}

func UpdateWebappTaskV2(vars task_common.UpdateTaskVars) error {
	postUrl := InternalWebappRoot + vars.UriPath()
	sklog.Infof("Updating %v on %s", vars, postUrl)

	json, err := json.Marshal(vars)
	if err != nil {
		return fmt.Errorf("Failed to marshal %v: %s", vars, err)
	}
	resp, err := httpClient.Post(postUrl, "application/json", bytes.NewReader(json))
	if err != nil {
		return fmt.Errorf("Could not update webapp task: %s", err)
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		response, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("Could not update webapp task, response status code was %d: %s", resp.StatusCode, response)
	}
	return nil
}

func UpdateWebappTaskSetStarted(vars task_common.UpdateTaskVars, id int64, runID string) error {
	vars.GetUpdateTaskCommonVars().Id = id
	vars.GetUpdateTaskCommonVars().SetStarted(runID)
	return UpdateWebappTaskV2(vars)
}
