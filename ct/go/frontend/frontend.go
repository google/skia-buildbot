// Functions and variables helping with communication with CT frontend.
package frontend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.skia.org/infra/ct/go/ctfe/pending_tasks"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	skutil "go.skia.org/infra/go/util"
	"go.skia.org/infra/go/webhook"
)

const (
	WEBAPP_ROOT_V2 = "https://ct.skia.org/"
)

var (
	WebappRoot string
	// Webapp subparts.
	AdminTasksWebapp                         string
	UpdateAdminTasksWebapp                   string
	UpdateRecreatePageSetsTasksWebapp        string
	UpdateRecreateWebpageArchivesTasksWebapp string
	LuaTasksWebapp                           string
	UpdateLuaTasksWebapp                     string
	CaptureSKPsTasksWebapp                   string
	UpdateCaptureSKPsTasksWebapp             string
	ChromiumPerfTasksWebapp                  string
	ChromiumAnalysisTasksWebapp              string
	UpdateChromiumPerfTasksWebapp            string
	ChromiumBuildTasksWebapp                 string
	UpdateChromiumBuildTasksWebapp           string
	GetOldestPendingTaskWebapp               string
)

var httpClient = httputils.NewTimeoutClient()

// Initializes *Webapp URLs above and sets up authentication credentials for UpdateWebappTaskV2.
func MustInit() {
	webhook.MustInitRequestSaltFromMetadata()
	initUrls(WEBAPP_ROOT_V2)
}

// Initializes *Webapp URLs above using webapp_root as the base URL (e.g. "http://localhost:8000/")
// and sets up test authentication credentials for UpdateWebappTaskV2.
func InitForTesting(webapp_root string) {
	webhook.InitRequestSaltForTesting()
	initUrls(webapp_root)
}

func initUrls(webapp_root string) {
	WebappRoot = webapp_root
	AdminTasksWebapp = webapp_root + ctfeutil.ADMIN_TASK_URI
	UpdateAdminTasksWebapp = ""
	UpdateRecreatePageSetsTasksWebapp = webapp_root + ctfeutil.UPDATE_RECREATE_PAGE_SETS_TASK_POST_URI
	UpdateRecreateWebpageArchivesTasksWebapp = webapp_root + ctfeutil.UPDATE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI
	LuaTasksWebapp = webapp_root + ctfeutil.LUA_SCRIPT_URI
	UpdateLuaTasksWebapp = webapp_root + ctfeutil.UPDATE_LUA_SCRIPT_TASK_POST_URI
	CaptureSKPsTasksWebapp = webapp_root + ctfeutil.CAPTURE_SKPS_URI
	UpdateCaptureSKPsTasksWebapp = webapp_root + ctfeutil.UPDATE_CAPTURE_SKPS_TASK_POST_URI
	ChromiumPerfTasksWebapp = webapp_root + ctfeutil.CHROMIUM_PERF_URI
	ChromiumAnalysisTasksWebapp = webapp_root + ctfeutil.CHROMIUM_ANALYSIS_URI
	UpdateChromiumPerfTasksWebapp = webapp_root + ctfeutil.UPDATE_CHROMIUM_PERF_TASK_POST_URI
	ChromiumBuildTasksWebapp = webapp_root + ctfeutil.CHROMIUM_BUILD_URI
	UpdateChromiumBuildTasksWebapp = webapp_root + ctfeutil.UPDATE_CHROMIUM_BUILD_TASK_POST_URI
	GetOldestPendingTaskWebapp = webapp_root + ctfeutil.GET_OLDEST_PENDING_TASK_URI
}

// Common functions

func GetOldestPendingTaskV2() (task_common.Task, error) {
	req, err := http.NewRequest("GET", GetOldestPendingTaskWebapp, nil)
	if err != nil {
		return nil, fmt.Errorf("Could not create HTTP request: %s", err)
	}
	hash, err := webhook.ComputeAuthHashBase64([]byte{})
	if err != nil {
		return nil, fmt.Errorf("Could not compute authentication hash: %s", err)
	}
	req.Header.Set(webhook.REQUEST_AUTH_HASH_HEADER, hash)
	client := httputils.NewTimeoutClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer skutil.Close(resp.Body)
	if resp.StatusCode != 200 {
		response, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s returned %d: %s", GetOldestPendingTaskWebapp, resp.StatusCode, response)
	}
	return pending_tasks.DecodeTask(resp.Body)
}

func UpdateWebappTaskV2(vars task_common.UpdateTaskVars) error {
	postUrl := WebappRoot + vars.UriPath()
	sklog.Infof("Updating %v on %s", vars, postUrl)

	json, err := json.Marshal(vars)
	if err != nil {
		return fmt.Errorf("Failed to marshal %v: %s", vars, err)
	}
	req, err := http.NewRequest("POST", postUrl, bytes.NewReader(json))
	if err != nil {
		return fmt.Errorf("Could not create HTTP request: %s", err)
	}
	hash, err := webhook.ComputeAuthHashBase64(json)
	if err != nil {
		return fmt.Errorf("Could not compute authentication hash: %s", err)
	}
	req.Header.Set(webhook.REQUEST_AUTH_HASH_HEADER, hash)
	client := httputils.NewTimeoutClient()
	resp, err := client.Do(req)
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

func UpdateWebappTaskSetStarted(vars task_common.UpdateTaskVars, id int64) error {
	vars.GetUpdateTaskCommonVars().Id = id
	vars.GetUpdateTaskCommonVars().SetStarted()
	return UpdateWebappTaskV2(vars)
}
