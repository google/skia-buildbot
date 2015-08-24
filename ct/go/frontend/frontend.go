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
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/webhook"
)

const (
	CTFE_V2        = false
	WEBAPP_ROOT    = "https://skia-tree-status.appspot.com/skia-telemetry/"
	WEBAPP_ROOT_V2 = "https://ct-staging.skia.org/"
)

var (
	// Webapp subparts.
	AdminTasksWebapp                         = WEBAPP_ROOT + "admin_tasks"
	UpdateAdminTasksWebapp                   = WEBAPP_ROOT + "update_admin_task"
	UpdateRecreatePageSetsTasksWebapp        = ""
	UpdateRecreateWebpageArchivesTasksWebapp = ""
	LuaTasksWebapp                           = WEBAPP_ROOT + "lua_script"
	UpdateLuaTasksWebapp                     = WEBAPP_ROOT + "update_lua_task"
	CaptureSKPsTasksWebapp                   = WEBAPP_ROOT
	UpdateCaptureSKPsTasksWebapp             = WEBAPP_ROOT + "update_telemetry_task"
	ChromiumPerfTasksWebapp                  = WEBAPP_ROOT + "chromium_try"
	UpdateChromiumPerfTasksWebapp            = WEBAPP_ROOT + "update_chromium_try_tasks"
	SkiaCorrectnessTasksWebapp               = WEBAPP_ROOT + "skia_try"
	UpdateSkiaCorrectnessTasksWebapp         = WEBAPP_ROOT + "update_skia_try_task"
	ChromiumBuildTasksWebapp                 = WEBAPP_ROOT + "chromium_builds"
	UpdateChromiumBuildTasksWebapp           = WEBAPP_ROOT + "update_chromium_build_tasks"
)

func init() {
	if CTFE_V2 {
		AdminTasksWebapp = WEBAPP_ROOT_V2 + ctfeutil.ADMIN_TASK_URI
		UpdateAdminTasksWebapp = ""
		UpdateRecreatePageSetsTasksWebapp = WEBAPP_ROOT_V2 + ctfeutil.UPDATE_RECREATE_PAGE_SETS_TASK_POST_URI
		UpdateRecreateWebpageArchivesTasksWebapp = WEBAPP_ROOT_V2 + ctfeutil.UPDATE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI
		LuaTasksWebapp = WEBAPP_ROOT_V2 + ctfeutil.LUA_SCRIPT_URI
		UpdateLuaTasksWebapp = WEBAPP_ROOT_V2 + ctfeutil.UPDATE_LUA_SCRIPT_TASK_POST_URI
		CaptureSKPsTasksWebapp = WEBAPP_ROOT_V2 + ctfeutil.CAPTURE_SKPS_URI
		UpdateCaptureSKPsTasksWebapp = WEBAPP_ROOT_V2 + ctfeutil.UPDATE_CAPTURE_SKPS_TASK_POST_URI
		ChromiumPerfTasksWebapp = WEBAPP_ROOT_V2 + ctfeutil.CHROMIUM_PERF_URI
		UpdateChromiumPerfTasksWebapp = WEBAPP_ROOT_V2 + ctfeutil.UPDATE_CHROMIUM_PERF_TASK_POST_URI
		SkiaCorrectnessTasksWebapp = ""
		UpdateSkiaCorrectnessTasksWebapp = ""
		ChromiumBuildTasksWebapp = WEBAPP_ROOT_V2 + ctfeutil.CHROMIUM_BUILD_URI
		UpdateChromiumBuildTasksWebapp = WEBAPP_ROOT_V2 + ctfeutil.UPDATE_CHROMIUM_BUILD_TASK_POST_URI
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

func UpdateWebappTaskV2(vars task_common.UpdateTaskVars) error {
	postUrl := WEBAPP_ROOT_V2 + vars.UriPath()
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
	if !CTFE_V2 {
		return nil
	}
	vars.GetUpdateTaskCommonVars().Id = id
	vars.GetUpdateTaskCommonVars().SetStarted()
	return UpdateWebappTaskV2(vars)
}
