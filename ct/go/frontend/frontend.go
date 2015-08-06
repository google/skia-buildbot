// Functions and variables helping with communication with CT frontend.
package frontend

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/util"
)

const (
	CTFE_V2        = false
	WEBAPP_ROOT    = "https://skia-tree-status.appspot.com/skia-telemetry/"
	WEBAPP_ROOT_V2 = "https://ct-staging.skia.org/"
)

// URIs for frontend handlers.
const (
	CHROMIUM_PERF_URI                  = "chromium_perf/"
	CHROMIUM_PERF_RUNS_URI             = "chromium_perf_runs/"
	CHROMIUM_PERF_PARAMETERS_POST_URI  = "_/chromium_perf/"
	ADD_CHROMIUM_PERF_TASK_POST_URI    = "_/add_chromium_perf_task"
	GET_CHROMIUM_PERF_TASKS_POST_URI   = "_/get_chromium_perf_tasks"
	UPDATE_CHROMIUM_PERF_TASK_POST_URI = "_/update_chromium_perf_task"
	DELETE_CHROMIUM_PERF_TASK_POST_URI = "_/delete_chromium_perf_task"

	CAPTURE_SKPS_URI                  = "capture_skps/"
	CAPTURE_SKPS_RUNS_URI             = "capture_skp_runs/"
	ADD_CAPTURE_SKPS_TASK_POST_URI    = "_/add_capture_skps_task"
	GET_CAPTURE_SKPS_TASKS_POST_URI   = "_/get_capture_skp_tasks"
	UPDATE_CAPTURE_SKPS_TASK_POST_URI = "_/update_capture_skps_task"
	DELETE_CAPTURE_SKPS_TASK_POST_URI = "_/delete_capture_skps_task"

	LUA_SCRIPT_URI                  = "lua_script/"
	LUA_SCRIPT_RUNS_URI             = "lua_script_runs/"
	ADD_LUA_SCRIPT_TASK_POST_URI    = "_/add_lua_script_task"
	GET_LUA_SCRIPT_TASKS_POST_URI   = "_/get_lua_script_tasks"
	UPDATE_LUA_SCRIPT_TASK_POST_URI = "_/update_lua_script_task"
	DELETE_LUA_SCRIPT_TASK_POST_URI = "_/delete_lua_script_task"

	CHROMIUM_BUILD_URI                  = "chromium_builds/"
	CHROMIUM_BUILD_RUNS_URI             = "chromium_builds_runs/"
	CHROMIUM_REV_DATA_POST_URI          = "_/chromium_rev_data"
	SKIA_REV_DATA_POST_URI              = "_/skia_rev_data"
	ADD_CHROMIUM_BUILD_TASK_POST_URI    = "_/add_chromium_build_task"
	GET_CHROMIUM_BUILD_TASKS_POST_URI   = "_/get_chromium_build_tasks"
	UPDATE_CHROMIUM_BUILD_TASK_POST_URI = "_/update_chromium_build_task"
	DELETE_CHROMIUM_BUILD_TASK_POST_URI = "_/delete_chromium_build_task"

	ADMIN_TASK_URI = "admin_tasks/"

	RECREATE_PAGE_SETS_RUNS_URI             = "recreate_page_sets_runs/"
	ADD_RECREATE_PAGE_SETS_TASK_POST_URI    = "_/add_recreate_page_sets_task"
	GET_RECREATE_PAGE_SETS_TASKS_POST_URI   = "_/get_recreate_page_sets_tasks"
	UPDATE_RECREATE_PAGE_SETS_TASK_POST_URI = "_/update_recreate_page_sets_task"
	DELETE_RECREATE_PAGE_SETS_TASK_POST_URI = "_/delete_recreate_page_sets_task"

	RECREATE_WEBPAGE_ARCHIVES_RUNS_URI             = "recreate_webpage_archives_runs/"
	ADD_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI    = "_/add_recreate_webpage_archives_task"
	GET_RECREATE_WEBPAGE_ARCHIVES_TASKS_POST_URI   = "_/get_recreate_webpage_archives_tasks"
	UPDATE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI = "_/update_recreate_webpage_archives_task"
	DELETE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI = "_/delete_recreate_webpage_archives_task"

	RUNS_HISTORY_URI = "history/"

	PENDING_TASKS_URI           = "queue/"
	GET_OLDEST_PENDING_TASK_URI = "_/get_oldest_pending_task"

	PAGE_SETS_PARAMETERS_POST_URI = "_/page_sets/"

	TS_FORMAT = "20060102150405"
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
		AdminTasksWebapp = WEBAPP_ROOT_V2 + ADMIN_TASK_URI
		UpdateAdminTasksWebapp = ""
		UpdateRecreatePageSetsTasksWebapp = WEBAPP_ROOT_V2 + UPDATE_RECREATE_PAGE_SETS_TASK_POST_URI
		UpdateRecreateWebpageArchivesTasksWebapp = WEBAPP_ROOT_V2 + UPDATE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI
		LuaTasksWebapp = WEBAPP_ROOT_V2 + LUA_SCRIPT_URI
		UpdateLuaTasksWebapp = WEBAPP_ROOT_V2 + UPDATE_LUA_SCRIPT_TASK_POST_URI
		CaptureSKPsTasksWebapp = WEBAPP_ROOT_V2 + CAPTURE_SKPS_URI
		UpdateCaptureSKPsTasksWebapp = WEBAPP_ROOT_V2 + UPDATE_CAPTURE_SKPS_TASK_POST_URI
		ChromiumPerfTasksWebapp = WEBAPP_ROOT_V2 + CHROMIUM_PERF_URI
		UpdateChromiumPerfTasksWebapp = WEBAPP_ROOT_V2 + UPDATE_CHROMIUM_PERF_TASK_POST_URI
		SkiaCorrectnessTasksWebapp = ""
		UpdateSkiaCorrectnessTasksWebapp = ""
		ChromiumBuildTasksWebapp = WEBAPP_ROOT_V2 + CHROMIUM_BUILD_URI
		UpdateChromiumBuildTasksWebapp = WEBAPP_ROOT_V2 + UPDATE_CHROMIUM_BUILD_TASK_POST_URI
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

func GetTimeFromTs(formattedTime string) time.Time {
	t, _ := time.Parse(TS_FORMAT, formattedTime)
	return t
}

func GetCurrentTs() string {
	return time.Now().UTC().Format(TS_FORMAT)
}

// Data included in all update requests.
type UpdateTaskCommonVars struct {
	Id              int64
	TsStarted       sql.NullString
	TsCompleted     sql.NullString
	Failure         sql.NullBool
	RepeatAfterDays sql.NullInt64
}

func (vars *UpdateTaskCommonVars) SetStarted() {
	vars.TsStarted = sql.NullString{String: GetCurrentTs(), Valid: true}
}

func (vars *UpdateTaskCommonVars) SetCompleted(success bool) {
	vars.TsCompleted = sql.NullString{String: GetCurrentTs(), Valid: true}
	vars.Failure = sql.NullBool{Bool: !success, Valid: true}
}

func (vars *UpdateTaskCommonVars) ClearRepeatAfterDays() {
	vars.RepeatAfterDays = sql.NullInt64{Int64: 0, Valid: true}
}

type UpdateTaskVars interface {
	GetUpdateTaskCommonVars() *UpdateTaskCommonVars
	Uri() string
}

func (vars *UpdateTaskCommonVars) GetUpdateTaskCommonVars() *UpdateTaskCommonVars {
	return vars
}

type ChromiumPerfUpdateVars struct {
	UpdateTaskCommonVars

	Results            sql.NullString
	NoPatchRawOutput   sql.NullString
	WithPatchRawOutput sql.NullString
}

func (vars *ChromiumPerfUpdateVars) Uri() string {
	return UpdateChromiumPerfTasksWebapp
}

type CaptureSkpsUpdateVars struct {
	UpdateTaskCommonVars
}

func (vars *CaptureSkpsUpdateVars) Uri() string {
	return UpdateCaptureSKPsTasksWebapp
}

type LuaScriptUpdateVars struct {
	UpdateTaskCommonVars
	ScriptOutput     sql.NullString `db:"script_output"`
	AggregatedOutput sql.NullString `db:"aggregated_output"`
}

func (vars *LuaScriptUpdateVars) Uri() string {
	return UpdateLuaTasksWebapp
}

type ChromiumBuildUpdateVars struct {
	UpdateTaskCommonVars
}

func (vars *ChromiumBuildUpdateVars) Uri() string {
	return UpdateChromiumBuildTasksWebapp
}

type RecreatePageSetsUpdateVars struct {
	UpdateTaskCommonVars
}

func (vars *RecreatePageSetsUpdateVars) Uri() string {
	return UpdateRecreatePageSetsTasksWebapp
}

type RecreateWebpageArchivesUpdateVars struct {
	UpdateTaskCommonVars
}

func (vars *RecreateWebpageArchivesUpdateVars) Uri() string {
	return UpdateRecreateWebpageArchivesTasksWebapp
}

func UpdateWebappTaskV2(vars UpdateTaskVars) error {
	glog.Infof("Updating %v on %s", vars, vars.Uri())

	json, err := json.Marshal(vars)
	if err != nil {
		return fmt.Errorf("Failed to marshal %v: %s", vars, err)
	}
	req, err := http.NewRequest("POST", vars.Uri(), bytes.NewReader(json))
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

func UpdateWebappTaskSetStarted(vars UpdateTaskVars, id int64) error {
	if !CTFE_V2 {
		return nil
	}
	vars.GetUpdateTaskCommonVars().Id = id
	vars.GetUpdateTaskCommonVars().SetStarted()
	return UpdateWebappTaskV2(vars)
}
