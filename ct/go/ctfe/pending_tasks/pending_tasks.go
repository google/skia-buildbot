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
	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/lua_scripts"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	"go.skia.org/infra/ct/go/db"
	skutil "go.skia.org/infra/go/util"
)

var (
	taskTables = []string{
		db.TABLE_CHROMIUM_PERF_TASKS,
		db.TABLE_CAPTURE_SKPS_TASKS,
		db.TABLE_LUA_SCRIPT_TASKS,
		db.TABLE_CHROMIUM_BUILD_TASKS,
		db.TABLE_RECREATE_PAGE_SETS_TASKS,
		db.TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS,
	}

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

func getAllPendingTasks() ([]task_common.Task, error) {
	tasks := []task_common.Task{}
	for _, tableName := range taskTables {
		var task task_common.Task
		query := fmt.Sprintf("SELECT * FROM %s WHERE ts_completed IS NULL ORDER BY ts_added LIMIT 1;", tableName)
		switch tableName {
		case db.TABLE_CHROMIUM_PERF_TASKS:
			task = &chromium_perf.DBTask{}
		case db.TABLE_CAPTURE_SKPS_TASKS:
			task = &capture_skps.DBTask{}
		case db.TABLE_LUA_SCRIPT_TASKS:
			task = &lua_scripts.DBTask{}
		case db.TABLE_CHROMIUM_BUILD_TASKS:
			task = &chromium_builds.DBTask{}
		case db.TABLE_RECREATE_PAGE_SETS_TASKS:
			task = &admin_tasks.RecreatePageSetsDBTask{}
		case db.TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS:
			task = &admin_tasks.RecreateWebpageArchivesDBTask{}
		default:
			panic("Unknown table " + tableName)
		}

		if err := db.DB.Get(task, query); err == sql.ErrNoRows {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("Failed to query DB: %v", err)
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// Union of all task types, to be easily marshalled/unmarshalled to/from JSON. At most one field
// should be non-nil when serialized as JSON.
type oldestPendingTask struct {
	ChromiumPerf            *chromium_perf.DBTask
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
	case *chromium_perf.DBTask:
		oldestTaskJsonRepr.ChromiumPerf = task
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
	case pending.ChromiumPerf != nil:
		return pending.ChromiumPerf, nil
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
	w.Header().Set("Content-Type", "application/json")

	tasks, err := getAllPendingTasks()
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to get all pending tasks: %v", err))
		return
	}

	var oldestTask task_common.Task
	for _, task := range tasks {
		if oldestTask == nil {
			oldestTask = task
		} else if oldestTask.GetCommonCols().TsAdded.Int64 >
			task.GetCommonCols().TsAdded.Int64 {
			oldestTask = task
		}
	}

	if err := EncodeTask(w, oldestTask); err != nil {
		skutil.ReportError(w, r, err,
			fmt.Sprintf("Failed to encode JSON for %#v", oldestTask))
		return
	}
}

func pendingTasksView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(pendingTasksTemplate, w, r)
}

func AddHandlers(r *mux.Router) {
	// Runs history handlers.
	r.HandleFunc("/"+ctfeutil.RUNS_HISTORY_URI, runsHistoryView).Methods("GET")

	// Task Queue handlers.
	r.HandleFunc("/"+ctfeutil.PENDING_TASKS_URI, pendingTasksView).Methods("GET")
	r.HandleFunc("/"+ctfeutil.GET_OLDEST_PENDING_TASK_URI, getOldestPendingTaskHandler).Methods("GET")
}
