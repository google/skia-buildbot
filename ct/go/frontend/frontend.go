// Functions and variables helping with communication with CT frontend.
package frontend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/ct/go/ctfe/pending_tasks"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/util"
	skutil "go.skia.org/infra/go/util"
	"go.skia.org/infra/go/webhook"
)

const (
	WEBAPP_ROOT    = "https://skia-tree-status.appspot.com/skia-telemetry/"
	WEBAPP_ROOT_V2 = "https://ct-staging.skia.org/"
)

var (
	// Allow tests to set CtfeV2 and WebappRoot to test V2 code with a mock server.
	CtfeV2     = false
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
	UpdateChromiumPerfTasksWebapp            string
	SkiaCorrectnessTasksWebapp               string
	UpdateSkiaCorrectnessTasksWebapp         string
	ChromiumBuildTasksWebapp                 string
	UpdateChromiumBuildTasksWebapp           string
	GetOldestPendingTaskWebapp               string
)

var httpClient = skutil.NewTimeoutClient()

// Initializes *Webapp URLs above and sets up authentication credentials for UpdateWebappTaskV2.
func MustInit() {
	if CtfeV2 {
		err := webhook.InitRequestSaltFromFile(ctutil.WebhookRequestSaltPath)
		if err != nil {
			glog.Fatalf("Could not read salt from %s. %s Error was: %v",
				ctutil.WebhookRequestSaltPath, ctutil.WEBHOOK_SALT_MSG, err)
		}
		initUrls(WEBAPP_ROOT_V2)
	} else {
		initUrls(WEBAPP_ROOT)
	}
}

// Initializes *Webapp URLs above using webapp_root as the base URL (e.g. "http://localhost:8000/")
// and sets up test authentication credentials for UpdateWebappTaskV2.
func InitForTesting(webapp_root string) {
	webhook.InitRequestSaltForTesting()
	initUrls(webapp_root)
}

func initUrls(webapp_root string) {
	WebappRoot = webapp_root
	if CtfeV2 {
		AdminTasksWebapp = webapp_root + ctfeutil.ADMIN_TASK_URI
		UpdateAdminTasksWebapp = ""
		UpdateRecreatePageSetsTasksWebapp = webapp_root + ctfeutil.UPDATE_RECREATE_PAGE_SETS_TASK_POST_URI
		UpdateRecreateWebpageArchivesTasksWebapp = webapp_root + ctfeutil.UPDATE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI
		LuaTasksWebapp = webapp_root + ctfeutil.LUA_SCRIPT_URI
		UpdateLuaTasksWebapp = webapp_root + ctfeutil.UPDATE_LUA_SCRIPT_TASK_POST_URI
		CaptureSKPsTasksWebapp = webapp_root + ctfeutil.CAPTURE_SKPS_URI
		UpdateCaptureSKPsTasksWebapp = webapp_root + ctfeutil.UPDATE_CAPTURE_SKPS_TASK_POST_URI
		ChromiumPerfTasksWebapp = webapp_root + ctfeutil.CHROMIUM_PERF_URI
		UpdateChromiumPerfTasksWebapp = webapp_root + ctfeutil.UPDATE_CHROMIUM_PERF_TASK_POST_URI
		SkiaCorrectnessTasksWebapp = ""
		UpdateSkiaCorrectnessTasksWebapp = ""
		ChromiumBuildTasksWebapp = webapp_root + ctfeutil.CHROMIUM_BUILD_URI
		UpdateChromiumBuildTasksWebapp = webapp_root + ctfeutil.UPDATE_CHROMIUM_BUILD_TASK_POST_URI
		GetOldestPendingTaskWebapp = webapp_root + ctfeutil.GET_OLDEST_PENDING_TASK_URI
	} else {
		AdminTasksWebapp = webapp_root + "admin_tasks"
		UpdateAdminTasksWebapp = webapp_root + "update_admin_task"
		UpdateRecreatePageSetsTasksWebapp = ""
		UpdateRecreateWebpageArchivesTasksWebapp = ""
		LuaTasksWebapp = webapp_root + "lua_script"
		UpdateLuaTasksWebapp = webapp_root + "update_lua_task"
		CaptureSKPsTasksWebapp = webapp_root
		UpdateCaptureSKPsTasksWebapp = webapp_root + "update_telemetry_task"
		ChromiumPerfTasksWebapp = webapp_root + "chromium_try"
		UpdateChromiumPerfTasksWebapp = webapp_root + "update_chromium_try_tasks"
		SkiaCorrectnessTasksWebapp = webapp_root + "skia_try"
		UpdateSkiaCorrectnessTasksWebapp = webapp_root + "update_skia_try_task"
		ChromiumBuildTasksWebapp = webapp_root + "chromium_builds"
		UpdateChromiumBuildTasksWebapp = webapp_root + "update_chromium_build_tasks"
		GetOldestPendingTaskWebapp = webapp_root + "get_oldest_pending_task"
	}
}

func UpdateWebappTask(gaeTaskID int64, webappURL string, extraData map[string]string) error {
	glog.Infof("Updating %s on %s with %s", gaeTaskID, webappURL, extraData)
	pwdBytes, err := ioutil.ReadFile(ctutil.WebappPasswordPath)
	if err != nil {
		return fmt.Errorf("Could not read the webapp password file: %s", err)
	}
	pwd := strings.TrimSpace(string(pwdBytes))
	postData := url.Values{}
	postData.Set("key", strconv.FormatInt(gaeTaskID, 10))
	postData.Add("password", pwd)
	for k, v := range extraData {
		postData.Add(k, v)
	}
	req, err := http.NewRequest("POST", webappURL, strings.NewReader(postData.Encode()))
	if err != nil {
		return fmt.Errorf("Could not create HTTP request: %s", err)
	}
	client := util.NewTimeoutClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Could not update webapp task: %s", err)
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("Could not update webapp task, response status code was %d: %s", resp.StatusCode, err)
	}
	return nil
}

// Common functions

func GetOldestPendingTaskV2() (task_common.Task, error) {
	resp, err := httpClient.Get(GetOldestPendingTaskWebapp)
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
	glog.Infof("Updating %v on %s", vars, postUrl)

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
	client := util.NewTimeoutClient()
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
	if !CtfeV2 {
		return nil
	}
	vars.GetUpdateTaskCommonVars().Id = id
	vars.GetUpdateTaskCommonVars().SetStarted()
	return UpdateWebappTaskV2(vars)
}
