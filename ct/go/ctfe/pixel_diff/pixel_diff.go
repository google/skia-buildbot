/*
	Handlers and types specific to Pixel diff tasks.
*/

package pixel_diff

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/ds"
	"google.golang.org/api/iterator"
)

var (
	addTaskTemplate     *template.Template = nil
	runsHistoryTemplate *template.Template = nil
)

func ReloadTemplates(resourcesDir string) {
	addTaskTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/pixel_diff.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
	runsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/pixel_diff_runs_history.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
}

type DatastoreTask struct {
	task_common.CommonCols

	PageSets             string
	IsTestPageSet        bool
	BenchmarkArgs        string
	BrowserArgsNoPatch   string
	BrowserArgsWithPatch string
	Description          string
	CustomWebpagesGSPath string
	ChromiumPatchGSPath  string
	SkiaPatchGSPath      string
	Results              string
}

func (task DatastoreTask) GetTaskName() string {
	return "PixelDiff"
}

func (task DatastoreTask) GetPopulatedAddTaskVars() (task_common.AddTaskVars, error) {
	taskVars := &AddTaskVars{}
	taskVars.Username = task.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(task.RepeatAfterDays, 10)
	taskVars.PageSets = task.PageSets
	taskVars.BenchmarkArgs = task.BenchmarkArgs
	taskVars.BrowserArgsNoPatch = task.BrowserArgsNoPatch
	taskVars.BrowserArgsWithPatch = task.BrowserArgsWithPatch
	taskVars.Description = task.Description

	var err error
	taskVars.CustomWebpages, err = ctutil.GetPatchFromStorage(task.CustomWebpagesGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", task.CustomWebpagesGSPath, err)
	}
	taskVars.ChromiumPatch, err = ctutil.GetPatchFromStorage(task.ChromiumPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", task.ChromiumPatchGSPath, err)
	}
	taskVars.SkiaPatch, err = ctutil.GetPatchFromStorage(task.SkiaPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", task.SkiaPatchGSPath, err)
	}

	return taskVars, nil
}

func (task DatastoreTask) GetResultsLink() string {
	return task.Results
}

func (task DatastoreTask) GetUpdateTaskVars() task_common.UpdateTaskVars {
	return &UpdateVars{}
}

func (task DatastoreTask) RunsOnGCEWorkers() bool {
	return true
}

func (task DatastoreTask) GetDatastoreKind() ds.Kind {
	return ds.PIXEL_DIFF_TASKS
}

func (task DatastoreTask) Query(it *datastore.Iterator) (interface{}, error) {
	tasks := []*DatastoreTask{}
	for {
		t := &DatastoreTask{}
		_, err := it.Next(t)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of tasks: %s", err)
		}
		tasks = append(tasks, t)
	}

	return tasks, nil
}

func (task DatastoreTask) Get(c context.Context, key *datastore.Key) (task_common.Task, error) {
	t := &DatastoreTask{}
	if err := ds.DS.Get(c, key, t); err != nil {
		return nil, err
	}
	return t, nil
}

func addTaskView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(addTaskTemplate, w, r)
}

type AddTaskVars struct {
	task_common.AddTaskCommonVars

	PageSets             string `json:"page_sets"`
	CustomWebpages       string `json:"custom_webpages"`
	BenchmarkArgs        string `json:"benchmark_args"`
	BrowserArgsNoPatch   string `json:"browser_args_nopatch"`
	BrowserArgsWithPatch string `json:"browser_args_withpatch"`
	Description          string `json:"desc"`
	ChromiumPatch        string `json:"chromium_patch"`
	SkiaPatch            string `json:"skia_patch"`
}

func (task *AddTaskVars) GetDatastoreKind() ds.Kind {
	return ds.PIXEL_DIFF_TASKS
}

func (task *AddTaskVars) GetPopulatedDatastoreTask(ctx context.Context) (task_common.Task, error) {
	if task.PageSets == "" ||
		task.Description == "" {
		return nil, fmt.Errorf("Invalid parameters")
	}

	customWebpagesSlice, err := ctfeutil.GetQualifiedCustomWebpages(task.CustomWebpages, task.BenchmarkArgs)
	if err != nil {
		return nil, err
	}
	customWebpages := strings.Join(customWebpagesSlice, ",")
	customWebpagesGSPath, err := ctutil.SavePatchToStorage(customWebpages)
	if err != nil {
		return nil, fmt.Errorf("Could not save custom webpages to storage: %s", err)
	}

	chromiumPatchGSPath, err := ctutil.SavePatchToStorage(task.ChromiumPatch)
	if err != nil {
		return nil, fmt.Errorf("Could not save chromium patch to storage: %s", err)
	}
	skiaPatchGSPath, err := ctutil.SavePatchToStorage(task.SkiaPatch)
	if err != nil {
		return nil, fmt.Errorf("Could not save skia patch to storage: %s", err)
	}

	t := &DatastoreTask{
		PageSets:             task.PageSets,
		IsTestPageSet:        task.PageSets == ctutil.PAGESET_TYPE_DUMMY_1k,
		BenchmarkArgs:        task.BenchmarkArgs,
		BrowserArgsNoPatch:   task.BrowserArgsNoPatch,
		BrowserArgsWithPatch: task.BrowserArgsWithPatch,
		Description:          task.Description,

		CustomWebpagesGSPath: customWebpagesGSPath,
		ChromiumPatchGSPath:  chromiumPatchGSPath,
		SkiaPatchGSPath:      skiaPatchGSPath,
	}
	return t, nil
}

func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.AddTaskHandler(w, r, &AddTaskVars{})
}

func getTasksHandler(w http.ResponseWriter, r *http.Request) {
	task_common.GetTasksHandler(&DatastoreTask{}, w, r)
}

type UpdateVars struct {
	task_common.UpdateTaskCommonVars

	Results string
}

func (vars *UpdateVars) UriPath() string {
	return ctfeutil.UPDATE_PIXEL_DIFF_TASK_POST_URI
}

func (vars *UpdateVars) UpdateExtraFields(t task_common.Task) error {
	task := t.(*DatastoreTask)
	if vars.Results != "" {
		task.Results = vars.Results
	}
	return nil
}

func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.UpdateTaskHandler(&UpdateVars{}, &DatastoreTask{}, w, r)
}

func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.DeleteTaskHandler(&DatastoreTask{}, w, r)
}

func redoTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.RedoTaskHandler(&DatastoreTask{}, w, r)
}

func runsHistoryView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(runsHistoryTemplate, w, r)
}

func AddHandlers(externalRouter, internalRouter *mux.Router) {
	externalRouter.HandleFunc("/"+ctfeutil.PIXEL_DIFF_URI, addTaskView).Methods("GET")
	externalRouter.HandleFunc("/"+ctfeutil.PIXEL_DIFF_RUNS_URI, runsHistoryView).Methods("GET")

	externalRouter.HandleFunc("/"+ctfeutil.ADD_PIXEL_DIFF_TASK_POST_URI, addTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.GET_PIXEL_DIFF_TASKS_POST_URI, getTasksHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.DELETE_PIXEL_DIFF_TASK_POST_URI, deleteTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.REDO_PIXEL_DIFF_TASK_POST_URI, redoTaskHandler).Methods("POST")

	// Updating tasks is done via the internal router.
	internalRouter.HandleFunc("/"+ctfeutil.UPDATE_PIXEL_DIFF_TASK_POST_URI, updateTaskHandler).Methods("POST")
}
