/*
	Handlers for retrieving pending tasks.
*/

package pending_tasks

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	api "go.skia.org/infra/ct/go/frontend"
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

		if err := db.DB.Get(task, query); err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("Failed to query DB: %v", err)
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
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
		} else if oldestTask.GetCommonCols().TsAdded.Int64 <
			task.GetCommonCols().TsAdded.Int64 {
			oldestTask = task
		}
	}

	oldestTaskJsonRepr := map[string]task_common.Task{}
	if oldestTask != nil {
		oldestTaskJsonRepr[oldestTask.GetTaskName()] = oldestTask
	}
	if err := json.NewEncoder(w).Encode(oldestTaskJsonRepr); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

func pendingTasksView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(pendingTasksTemplate, w, r)
}

func AddHandlers(r *mux.Router) {
	// Runs history handlers.
	r.HandleFunc("/"+api.RUNS_HISTORY_URI, runsHistoryView).Methods("GET")

	// Task Queue handlers.
	r.HandleFunc("/"+api.PENDING_TASKS_URI, pendingTasksView).Methods("GET")
	r.HandleFunc("/"+api.GET_OLDEST_PENDING_TASK_URI, getOldestPendingTaskHandler).Methods("GET")
}
