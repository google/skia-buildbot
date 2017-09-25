/*
	Handlers for retrieving pending tasks.
*/

package pending_tasks

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"text/template"

	"github.com/gorilla/mux"

	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/ctfe/chromium_analysis"
	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/lua_scripts"
	"go.skia.org/infra/ct/go/ctfe/pixel_diff"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	"go.skia.org/infra/ct/go/ctfe/task_types"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	"go.skia.org/infra/ct/go/db"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/webhook"
)

var (
	runsHistoryTemplate  *template.Template = nil
	pendingTasksTemplate *template.Template = nil
)

func ReloadTemplates(resourcesDir string) {
	runsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/runs_history.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))

	pendingTasksTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/pending_tasks.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
}

func runsHistoryView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(runsHistoryTemplate, w, r)
}

// GetOldestPendingTask returns the oldest pending task of any type.
func GetOldestPendingTask() (task_common.Task, error) {
	var oldestTask task_common.Task
	for _, task := range task_types.Prototypes() {
		query := fmt.Sprintf("SELECT * FROM %s WHERE ts_started IS NULL ORDER BY ts_added LIMIT 1;", task.TableName())
		if err := db.DB.Get(task, query); err == sql.ErrNoRows {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("Failed to query DB: %v", err)
		}
		if oldestTask == nil {
			oldestTask = task
		} else if oldestTask.GetCommonCols().TsAdded.Int64 > task.GetCommonCols().TsAdded.Int64 {
			oldestTask = task
		}
	}
	return oldestTask, nil
}

// GetRunningTasks returns all running tasks from all task types.
func GetRunningTasks() ([]task_common.Task, error) {
	runningTasks := []task_common.Task{}
	for _, task := range task_types.Prototypes() {
		query := fmt.Sprintf("SELECT * FROM %s WHERE ts_started IS NOT NULL AND ts_completed IS NULL ORDER BY ts_added;", task.TableName())
		data, err := task.Select(query)
		if err != nil {
			return nil, fmt.Errorf("Failed to query DB: %v", err)
		}
		runningTasks = append(runningTasks, task_common.AsTaskSlice(data)...)
	}
	return runningTasks, nil
}

func TerminateRunningTasks() error {
	runningTasks, err := GetRunningTasks()
	if err != nil {
		return fmt.Errorf("Could not get list of running tasks: %s", err)
	}
	runningTasksOwners := []string{}
	for _, task := range runningTasks {
		updateVars := task.GetUpdateTaskVars()
		commonUpdateVars := updateVars.GetUpdateTaskCommonVars()
		commonUpdateVars.Id = task.GetCommonCols().Id
		commonUpdateVars.SetCompleted(false)
		if err := task_common.UpdateTask(updateVars, task.TableName()); err != nil {
			return fmt.Errorf("Failed to update %T task: %s", updateVars, err)
		}
		runningTasksOwners = append(runningTasksOwners, task.GetCommonCols().Username)
	}
	// Email all owners + admins.
	if len(runningTasksOwners) > 0 {
		emailRecipients := append(runningTasksOwners, ctutil.CtAdmins...)
		if err := ctutil.SendTasksTerminatedEmail(emailRecipients); err != nil {
			return fmt.Errorf("Failed to send task termination email: %s", err)
		}
	}

	return nil
}

// Union of all task types, to be easily marshalled/unmarshalled to/from JSON. At most one field
// should be non-nil when serialized as JSON.
type oldestPendingTask struct {
	ChromiumAnalysis        *chromium_analysis.DBTask
	ChromiumPerf            *chromium_perf.DBTask
	PixelDiff               *pixel_diff.DBTask
	CaptureSkps             *capture_skps.DBTask
	LuaScript               *lua_scripts.DBTask
	ChromiumBuild           *chromium_builds.DBTask
	RecreatePageSets        *admin_tasks.RecreatePageSetsDBTask
	RecreateWebpageArchives *admin_tasks.RecreateWebpageArchivesDBTask
}

// Writes JSON representation of oldestTask to taskJson. Returns an error if oldestTask's type is
// unknown, if there was an error encoding to JSON, or there is an error writing to taskJson. Does
// not close taskJson.
func EncodeTask(taskJson io.Writer, oldestTask task_common.Task) error {
	oldestTaskJsonRepr := oldestPendingTask{}
	switch task := oldestTask.(type) {
	case nil:
		// No fields set.
	case *chromium_analysis.DBTask:
		oldestTaskJsonRepr.ChromiumAnalysis = task
	case *chromium_perf.DBTask:
		oldestTaskJsonRepr.ChromiumPerf = task
	case *pixel_diff.DBTask:
		oldestTaskJsonRepr.PixelDiff = task
	case *capture_skps.DBTask:
		oldestTaskJsonRepr.CaptureSkps = task
	case *lua_scripts.DBTask:
		oldestTaskJsonRepr.LuaScript = task
	case *chromium_builds.DBTask:
		oldestTaskJsonRepr.ChromiumBuild = task
	case *admin_tasks.RecreatePageSetsDBTask:
		oldestTaskJsonRepr.RecreatePageSets = task
	case *admin_tasks.RecreateWebpageArchivesDBTask:
		oldestTaskJsonRepr.RecreateWebpageArchives = task
	default:
		return fmt.Errorf("Missing case for %T", oldestTask)
	}
	return json.NewEncoder(taskJson).Encode(oldestTaskJsonRepr)
}

// Reads JSON response from ctfeutil.GET_OLDEST_PENDING_TASK_URI and returns either the Task decoded
// from the response or nil if there are no pending tasks. Returns an error if there is a problem
// decoding the JSON. Does not close taskJson.
func DecodeTask(taskJson io.Reader) (task_common.Task, error) {
	pending := oldestPendingTask{}
	if err := json.NewDecoder(taskJson).Decode(&pending); err != nil {
		return nil, err
	}
	switch {
	case pending.ChromiumAnalysis != nil:
		return pending.ChromiumAnalysis, nil
	case pending.ChromiumPerf != nil:
		return pending.ChromiumPerf, nil
	case pending.PixelDiff != nil:
		return pending.PixelDiff, nil
	case pending.CaptureSkps != nil:
		return pending.CaptureSkps, nil
	case pending.LuaScript != nil:
		return pending.LuaScript, nil
	case pending.ChromiumBuild != nil:
		return pending.ChromiumBuild, nil
	case pending.RecreatePageSets != nil:
		return pending.RecreatePageSets, nil
	case pending.RecreateWebpageArchives != nil:
		return pending.RecreateWebpageArchives, nil
	default:
		return nil, nil
	}
}

func getOldestPendingTaskHandler(w http.ResponseWriter, r *http.Request) {
	data, err := webhook.AuthenticateRequest(r)
	if err != nil {
		if data == nil {
			httputils.ReportError(w, r, err, "Failed to read update request")
			return
		}
		if !ctfeutil.UserHasAdminRights(r) {
			httputils.ReportError(w, r, err, "Failed authentication")
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")

	oldestTask, err := GetOldestPendingTask()
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to get oldest pending task")
		return
	}

	if err := EncodeTask(w, oldestTask); err != nil {
		httputils.ReportError(w, r, err,
			fmt.Sprintf("Failed to encode JSON for %#v", oldestTask))
		return
	}
}

func getTerminateRunningTasksHandler(w http.ResponseWriter, r *http.Request) {
	data, err := webhook.AuthenticateRequest(r)
	if err != nil {
		if data == nil {
			httputils.ReportError(w, r, err, "Failed to read update request")
			return
		}
		if !ctfeutil.UserHasAdminRights(r) {
			httputils.ReportError(w, r, err, "Failed authentication")
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")

	if err := TerminateRunningTasks(); err != nil {
		httputils.ReportError(w, r, err, "Failed to terminate running tasks")
		return
	}
}

// GetPendingTaskCount returns the total number of pending tasks of all types. On error, the first
// return value will be -1 and the second return value will be non-nil.
func GetPendingTaskCount() (int64, error) {
	var result int64 = 0
	params := task_common.QueryParams{
		PendingOnly: true,
		CountQuery:  true,
	}
	for _, prototype := range task_types.Prototypes() {
		query, args := task_common.DBTaskQuery(prototype, params)
		var countVal int64 = 0
		if err := db.DB.Get(&countVal, query, args...); err != nil {
			return -1, err
		}
		result += countVal
	}
	return result, nil
}

func pendingTasksView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(pendingTasksTemplate, w, r)
}

func AddHandlers(r *mux.Router) {
	// Runs history handlers.
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.RUNS_HISTORY_URI, "GET", runsHistoryView)

	// Task Queue handlers.
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.PENDING_TASKS_URI, "GET", pendingTasksView)

	// Do not add force login handler for getOldestPendingTaskHandler and
	// getTerminateRunningTasksHandler, they use webhooks for authentication.
	r.HandleFunc("/"+ctfeutil.GET_OLDEST_PENDING_TASK_URI, getOldestPendingTaskHandler).Methods("GET")
	r.HandleFunc("/"+ctfeutil.TERMINATE_RUNNING_TASKS_URI, getTerminateRunningTasksHandler).Methods("POST")
}
