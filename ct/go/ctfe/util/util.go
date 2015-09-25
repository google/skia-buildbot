/*
	Utility functions used by all of ctfe.
*/

package util

import (
	"fmt"
	"net/http"
	"strings"
	"text/template"

	"go.skia.org/infra/go/login"
	skutil "go.skia.org/infra/go/util"
)

// URIs for frontend handlers.
const (
	CHROMIUM_PERF_URI                       = "chromium_perf/"
	CHROMIUM_PERF_RUNS_URI                  = "chromium_perf_runs/"
	GET_CHROMIUM_PERF_RUN_STATUS_URI        = "get_chromium_perf_run_status"
	CHROMIUM_PERF_PARAMETERS_POST_URI       = "_/chromium_perf/"
	CHROMIUM_PERF_CL_DATA_POST_URI          = "_/cl_data"
	ADD_CHROMIUM_PERF_TASK_POST_URI         = "_/add_chromium_perf_task"
	GET_CHROMIUM_PERF_TASKS_POST_URI        = "_/get_chromium_perf_tasks"
	UPDATE_CHROMIUM_PERF_TASK_POST_URI      = "_/update_chromium_perf_task"
	WEBHOOK_ADD_CHROMIUM_PERF_TASK_POST_URI = "_/webhook_add_chromium_perf_task"
	DELETE_CHROMIUM_PERF_TASK_POST_URI      = "_/delete_chromium_perf_task"
	REDO_CHROMIUM_PERF_TASK_POST_URI        = "_/redo_chromium_perf_task"

	CAPTURE_SKPS_URI                  = "capture_skps/"
	CAPTURE_SKPS_RUNS_URI             = "capture_skp_runs/"
	ADD_CAPTURE_SKPS_TASK_POST_URI    = "_/add_capture_skps_task"
	GET_CAPTURE_SKPS_TASKS_POST_URI   = "_/get_capture_skp_tasks"
	UPDATE_CAPTURE_SKPS_TASK_POST_URI = "_/update_capture_skps_task"
	DELETE_CAPTURE_SKPS_TASK_POST_URI = "_/delete_capture_skps_task"
	REDO_CAPTURE_SKPS_TASK_POST_URI   = "_/redo_capture_skps_task"

	LUA_SCRIPT_URI                  = "lua_script/"
	LUA_SCRIPT_RUNS_URI             = "lua_script_runs/"
	ADD_LUA_SCRIPT_TASK_POST_URI    = "_/add_lua_script_task"
	GET_LUA_SCRIPT_TASKS_POST_URI   = "_/get_lua_script_tasks"
	UPDATE_LUA_SCRIPT_TASK_POST_URI = "_/update_lua_script_task"
	DELETE_LUA_SCRIPT_TASK_POST_URI = "_/delete_lua_script_task"
	REDO_LUA_SCRIPT_TASK_POST_URI   = "_/redo_lua_script_task"

	CHROMIUM_BUILD_URI                  = "chromium_builds/"
	CHROMIUM_BUILD_RUNS_URI             = "chromium_builds_runs/"
	CHROMIUM_REV_DATA_POST_URI          = "_/chromium_rev_data"
	SKIA_REV_DATA_POST_URI              = "_/skia_rev_data"
	ADD_CHROMIUM_BUILD_TASK_POST_URI    = "_/add_chromium_build_task"
	GET_CHROMIUM_BUILD_TASKS_POST_URI   = "_/get_chromium_build_tasks"
	UPDATE_CHROMIUM_BUILD_TASK_POST_URI = "_/update_chromium_build_task"
	DELETE_CHROMIUM_BUILD_TASK_POST_URI = "_/delete_chromium_build_task"
	REDO_CHROMIUM_BUILD_TASK_POST_URI   = "_/redo_chromium_build_task"

	ADMIN_TASK_URI = "admin_tasks/"

	RECREATE_PAGE_SETS_RUNS_URI             = "recreate_page_sets_runs/"
	ADD_RECREATE_PAGE_SETS_TASK_POST_URI    = "_/add_recreate_page_sets_task"
	GET_RECREATE_PAGE_SETS_TASKS_POST_URI   = "_/get_recreate_page_sets_tasks"
	UPDATE_RECREATE_PAGE_SETS_TASK_POST_URI = "_/update_recreate_page_sets_task"
	DELETE_RECREATE_PAGE_SETS_TASK_POST_URI = "_/delete_recreate_page_sets_task"
	REDO_RECREATE_PAGE_SETS_TASK_POST_URI   = "_/redo_recreate_page_sets_task"

	RECREATE_WEBPAGE_ARCHIVES_RUNS_URI             = "recreate_webpage_archives_runs/"
	ADD_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI    = "_/add_recreate_webpage_archives_task"
	GET_RECREATE_WEBPAGE_ARCHIVES_TASKS_POST_URI   = "_/get_recreate_webpage_archives_tasks"
	UPDATE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI = "_/update_recreate_webpage_archives_task"
	DELETE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI = "_/delete_recreate_webpage_archives_task"
	REDO_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI   = "_/redo_recreate_webpage_archives_task"

	RUNS_HISTORY_URI = "history/"

	PENDING_TASKS_URI           = "queue/"
	GET_OLDEST_PENDING_TASK_URI = "_/get_oldest_pending_task"

	PAGE_SETS_PARAMETERS_POST_URI = "_/page_sets/"
)

// Function to run before executing a template.
var PreExecuteTemplateHook = func() {}

func UserHasEditRights(r *http.Request) bool {
	return strings.HasSuffix(login.LoggedInAs(r), "@google.com") || strings.HasSuffix(login.LoggedInAs(r), "@chromium.org")
}

func UserHasAdminRights(r *http.Request) bool {
	// TODO(benjaminwagner): Add this list to GCE project level metadata and retrieve from there.
	admins := map[string]bool{
		"benjaminwagner@google.com": true,
		"borenet@google.com":        true,
		"jcgregorio@google.com":     true,
		"rmistry@google.com":        true,
		"stephana@google.com":       true,
	}
	return UserHasEditRights(r) && admins[login.LoggedInAs(r)]
}

func ExecuteSimpleTemplate(template *template.Template, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	PreExecuteTemplateHook()
	if err := template.Execute(w, struct{}{}); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
		return
	}
}

type LengthCheck struct {
	Name  string
	Value string
	Limit int
}

func CheckLengths(checks []LengthCheck) error {
	for _, check := range checks {
		if len(check.Value) > check.Limit {
			return fmt.Errorf("Value of %s is too long; limit %d bytes", check.Name, check.Limit)
		}
	}
	return nil
}
