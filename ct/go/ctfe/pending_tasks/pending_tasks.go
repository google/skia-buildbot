/*
	Handlers for retrieving pending tasks.
*/

package pending_tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"text/template"

	"github.com/gorilla/mux"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/ctfe/chromium_analysis"
	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/lua_scripts"
	"go.skia.org/infra/ct/go/ctfe/metrics_analysis"
	"go.skia.org/infra/ct/go/ctfe/pixel_diff"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	"go.skia.org/infra/ct/go/ctfe/task_types"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/httputils"
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
func GetOldestPendingTask(ctx context.Context) (task_common.Task, error) {
	var oldestTask task_common.Task
	for _, task := range task_types.Prototypes() {
		q := ds.NewQuery(task.GetDatastoreKind())
		q = q.Filter("TsStarted =", 0)
		q = q.Order("-__key__")
		q = q.Limit(1)
		it := ds.DS.Run(ctx, q)
		s, err := task.Query(it)
		if err != nil {
			return nil, fmt.Errorf("Failed to query datastore for oldest pending task: %s", err)
		}
		tasks := task_common.AsTaskSlice(s)
		if len(tasks) == 0 {
			continue
		}
		t := tasks[0]
		if oldestTask == nil {
			oldestTask = t
		} else if oldestTask.GetCommonCols().TsAdded > t.GetCommonCols().TsAdded {
			oldestTask = t
		}
	}

	return oldestTask, nil
}

// GetRunningTasks returns all running tasks from all task types.
func GetRunningTasks(ctx context.Context) ([]task_common.Task, error) {
	runningTasks := []task_common.Task{}
	for _, task := range task_types.Prototypes() {
		q := ds.NewQuery(task.GetDatastoreKind())
		q = q.Filter("TaskDone =", false)
		q = q.Order("-__key__")
		it := ds.DS.Run(ctx, q)
		s, err := task.Query(it)
		if err != nil {
			return nil, fmt.Errorf("Failed to query datastore for running tasks: %s", err)
		}
		tasks := task_common.AsTaskSlice(s)
		for _, t := range tasks {
			if t.GetCommonCols().TsStarted != 0 {
				runningTasks = append(runningTasks, t)
			}
		}
	}
	return runningTasks, nil
}

func TerminateRunningTasks(ctx context.Context) error {
	runningTasks, err := GetRunningTasks(ctx)
	if err != nil {
		return fmt.Errorf("Could not get list of running tasks: %s", err)
	}
	runningTasksOwners := []string{}
	for _, task := range runningTasks {
		updateVars := task.GetUpdateTaskVars()
		commonUpdateVars := updateVars.GetUpdateTaskCommonVars()
		commonUpdateVars.Id = task.GetCommonCols().DatastoreKey.ID
		commonUpdateVars.SetCompleted(false)
		if err := task_common.UpdateTask(ctx, updateVars, task); err != nil {
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
	CaptureSkps             *capture_skps.DatastoreTask
	ChromiumAnalysis        *chromium_analysis.DatastoreTask
	ChromiumBuild           *chromium_builds.DatastoreTask
	ChromiumPerf            *chromium_perf.DatastoreTask
	LuaScript               *lua_scripts.DatastoreTask
	MetricsAnalysis         *metrics_analysis.DatastoreTask
	PixelDiff               *pixel_diff.DatastoreTask
	RecreatePageSets        *admin_tasks.RecreatePageSetsDatastoreTask
	RecreateWebpageArchives *admin_tasks.RecreateWebpageArchivesDatastoreTask
}

// Writes JSON representation of oldestTask to taskJson. Returns an error if oldestTask's type is
// unknown, if there was an error encoding to JSON, or there is an error writing to taskJson. Does
// not close taskJson.
func EncodeTask(taskJson io.Writer, oldestTask task_common.Task) error {
	oldestTaskJsonRepr := oldestPendingTask{}
	switch task := oldestTask.(type) {
	case nil:
		// No fields set.
	case *admin_tasks.RecreatePageSetsDatastoreTask:
		oldestTaskJsonRepr.RecreatePageSets = task
	case *admin_tasks.RecreateWebpageArchivesDatastoreTask:
		oldestTaskJsonRepr.RecreateWebpageArchives = task
	case *capture_skps.DatastoreTask:
		oldestTaskJsonRepr.CaptureSkps = task
	case *chromium_analysis.DatastoreTask:
		oldestTaskJsonRepr.ChromiumAnalysis = task
	case *chromium_builds.DatastoreTask:
		oldestTaskJsonRepr.ChromiumBuild = task
	case *chromium_perf.DatastoreTask:
		oldestTaskJsonRepr.ChromiumPerf = task
	case *lua_scripts.DatastoreTask:
		oldestTaskJsonRepr.LuaScript = task
	case *metrics_analysis.DatastoreTask:
		oldestTaskJsonRepr.MetricsAnalysis = task
	case *pixel_diff.DatastoreTask:
		oldestTaskJsonRepr.PixelDiff = task
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
	case pending.CaptureSkps != nil:
		return pending.CaptureSkps, nil
	case pending.ChromiumAnalysis != nil:
		return pending.ChromiumAnalysis, nil
	case pending.ChromiumBuild != nil:
		return pending.ChromiumBuild, nil
	case pending.ChromiumPerf != nil:
		return pending.ChromiumPerf, nil
	case pending.LuaScript != nil:
		return pending.LuaScript, nil
	case pending.MetricsAnalysis != nil:
		return pending.MetricsAnalysis, nil
	case pending.PixelDiff != nil:
		return pending.PixelDiff, nil
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
	oldestTask, err := GetOldestPendingTask(r.Context())
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
	w.Header().Set("Content-Type", "application/json")
	if err := TerminateRunningTasks(r.Context()); err != nil {
		httputils.ReportError(w, r, err, "Failed to terminate running tasks")
		return
	}
}

// GetPendingTaskCount returns the total number of pending tasks of all types. On error, the first
// return value will be -1 and the second return value will be non-nil.
func GetPendingTaskCount(ctx context.Context) (int64, error) {
	var result int64 = 0
	params := task_common.QueryParams{
		PendingOnly: true,
		CountQuery:  true,
	}
	for _, prototype := range task_types.Prototypes() {
		it := task_common.DatastoreTaskQuery(ctx, prototype, params)
		var countVal int64 = 0
		for {
			var i int
			_, err := it.Next(i)
			if err == iterator.Done {
				break
			} else if err != nil {
				return -1, fmt.Errorf("Failed to query %s tasks for pending task count: %s", prototype.GetTaskName(), err)
			}
			countVal++
		}
		result += countVal
	}
	return result, nil
}

func pendingTasksView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(pendingTasksTemplate, w, r)
}

func AddHandlers(externalRouter, internalRouter *mux.Router) {
	// Runs history handlers.
	externalRouter.HandleFunc("/"+ctfeutil.RUNS_HISTORY_URI, runsHistoryView).Methods("GET")

	// Task Queue handlers.
	externalRouter.HandleFunc("/"+ctfeutil.PENDING_TASKS_URI, pendingTasksView).Methods("GET")

	// getOldestPendingTaskHandler and getTerminateRunningTasksHandler is done via the internal router.
	internalRouter.HandleFunc("/"+ctfeutil.GET_OLDEST_PENDING_TASK_URI, getOldestPendingTaskHandler).Methods("GET")
	internalRouter.HandleFunc("/"+ctfeutil.TERMINATE_RUNNING_TASKS_URI, getTerminateRunningTasksHandler).Methods("POST")
}
