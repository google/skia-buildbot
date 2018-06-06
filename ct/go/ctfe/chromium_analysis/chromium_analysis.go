/*
	Handlers and types specific to Chromium analysis tasks.
*/

package chromium_analysis

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
	"google.golang.org/api/iterator"

	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/httputils"
)

var (
	addTaskTemplate     *template.Template = nil
	runsHistoryTemplate *template.Template = nil

	httpClient = httputils.NewTimeoutClient()
)

func ReloadTemplates(resourcesDir string) {
	addTaskTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/chromium_analysis.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
	runsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/chromium_analysis_runs_history.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
}

type DatastoreTask struct {
	task_common.CommonCols

	Benchmark      string
	PageSets       string
	IsTestPageSet  bool
	CustomWebpages string
	BenchmarkArgs  string
	BrowserArgs    string
	Description    string
	ChromiumPatch  string `datastore:",noindex"`
	SkiaPatch      string `datastore:",noindex"`
	CatapultPatch  string `datastore:",noindex"`
	BenchmarkPatch string `datastore:",noindex"`
	V8Patch        string `datastore:",noindex"`
	RunInParallel  bool
	Platform       string
	RunOnGCE       bool
	RawOutput      string
	MatchStdoutTxt string
}

func (task DatastoreTask) GetTaskName() string {
	return "ChromiumAnalysis"
}

func (dbTask *DatastoreTask) GetPopulatedAddTaskVars() task_common.AddTaskVars {
	taskVars := &AddTaskVars{}
	taskVars.Username = dbTask.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(dbTask.RepeatAfterDays, 10)
	taskVars.Benchmark = dbTask.Benchmark
	taskVars.PageSets = dbTask.PageSets
	taskVars.CustomWebpages = dbTask.CustomWebpages
	taskVars.BenchmarkArgs = dbTask.BenchmarkArgs
	taskVars.BrowserArgs = dbTask.BrowserArgs
	taskVars.Description = dbTask.Description
	taskVars.ChromiumPatch = dbTask.ChromiumPatch
	taskVars.SkiaPatch = dbTask.SkiaPatch
	taskVars.CatapultPatch = dbTask.CatapultPatch
	taskVars.BenchmarkPatch = dbTask.BenchmarkPatch
	taskVars.V8Patch = dbTask.V8Patch
	taskVars.RunInParallel = dbTask.RunInParallel
	taskVars.Platform = dbTask.Platform
	taskVars.RunOnGCE = dbTask.RunOnGCE
	taskVars.MatchStdoutTxt = dbTask.MatchStdoutTxt
	return taskVars
}

func (task DatastoreTask) GetResultsLink() string {
	return task.RawOutput
}

func (task DatastoreTask) GetUpdateTaskVars() task_common.UpdateTaskVars {
	return &UpdateVars{}
}

func (task DatastoreTask) RunsOnGCEWorkers() bool {
	return task.RunOnGCE && task.Platform != ctutil.PLATFORM_ANDROID
}

func (task DatastoreTask) GetDatastoreKind() ds.Kind {
	return ds.CHROMIUM_ANALYSIS_TASKS
}

func (task DatastoreTask) Select(it *datastore.Iterator) (interface{}, error) {
	tasks := []*DatastoreTask{}
	for {
		t := &DatastoreTask{}
		k, err := it.Next(t)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of tasks: %s", err)
		}
		t.DatastoreId = k.ID
		tasks = append(tasks, t)
	}

	return tasks, nil
}

func (task DatastoreTask) Find(c context.Context, key *datastore.Key) (interface{}, error) {
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

	Benchmark      string `json:"benchmark"`
	PageSets       string `json:"page_sets"`
	CustomWebpages string `json:"custom_webpages"`
	BenchmarkArgs  string `json:"benchmark_args"`
	BrowserArgs    string `json:"browser_args"`
	Description    string `json:"desc"`
	ChromiumPatch  string `json:"chromium_patch"`
	SkiaPatch      string `json:"skia_patch"`
	CatapultPatch  string `json:"catapult_patch"`
	BenchmarkPatch string `json:"benchmark_patch"`
	V8Patch        string `json:"v8_patch"`
	RunInParallel  bool   `json:"run_in_parallel"`
	Platform       string `json:"platform"`
	RunOnGCE       bool   `json:"run_on_gce"`
	MatchStdoutTxt string `json:"match_stdout_txt"`
}

func (task *AddTaskVars) GetDatastoreKind() ds.Kind {
	return ds.CHROMIUM_ANALYSIS_TASKS
}

func (task *AddTaskVars) GetPopulatedDatastoreTask() (task_common.Task, error) {
	if task.Benchmark == "" ||
		task.PageSets == "" ||
		task.Platform == "" ||
		task.Description == "" {
		return nil, fmt.Errorf("Invalid parameters")
	}
	customWebpages, err := ctfeutil.GetQualifiedCustomWebpages(task.CustomWebpages, task.BenchmarkArgs)
	if err != nil {
		return nil, err
	}

	id, err := task_common.GetNextId(ds.CHROMIUM_ANALYSIS_TASKS, &DatastoreTask{})
	if err != nil {
		return nil, fmt.Errorf("Could not get highest id: %s", err)
	}

	t := &DatastoreTask{
		Benchmark:      task.Benchmark,
		PageSets:       task.PageSets,
		IsTestPageSet:  task.PageSets == ctutil.PAGESET_TYPE_DUMMY_1k,
		CustomWebpages: strings.Join(customWebpages, ","),
		BenchmarkArgs:  task.BenchmarkArgs,
		BrowserArgs:    task.BrowserArgs,
		Description:    task.Description,
		ChromiumPatch:  task.ChromiumPatch,
		SkiaPatch:      task.SkiaPatch,
		CatapultPatch:  task.CatapultPatch,
		BenchmarkPatch: task.BenchmarkPatch,
		V8Patch:        task.V8Patch,
		RunInParallel:  task.RunInParallel,
		Platform:       task.Platform,
		RunOnGCE:       task.RunOnGCE,
		MatchStdoutTxt: task.MatchStdoutTxt,
	}
	tsAdded, err := strconv.ParseInt(task.TsAdded, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s is not int64: %s", task.TsAdded, err)
	}
	t.TsAdded = tsAdded
	t.Username = task.Username
	t.Id = id
	repeatAfterDays, err := strconv.ParseInt(task.RepeatAfterDays, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s is not int64: %s", task.RepeatAfterDays, err)
	}
	t.RepeatAfterDays = repeatAfterDays
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

	RawOutput string
}

func (vars *UpdateVars) UriPath() string {
	return ctfeutil.UPDATE_CHROMIUM_ANALYSIS_TASK_POST_URI
}

func (task *UpdateVars) AddUpdatesToDatastoreTask(t task_common.Task) error {
	if task.RawOutput != "" {
		dbTask := t.(*DatastoreTask)
		dbTask.RawOutput = task.RawOutput
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

func AddHandlers(r *mux.Router) {
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.CHROMIUM_ANALYSIS_URI, "GET", addTaskView)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.CHROMIUM_ANALYSIS_RUNS_URI, "GET", runsHistoryView)

	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.ADD_CHROMIUM_ANALYSIS_TASK_POST_URI, "POST", addTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.GET_CHROMIUM_ANALYSIS_TASKS_POST_URI, "POST", getTasksHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.DELETE_CHROMIUM_ANALYSIS_TASK_POST_URI, "POST", deleteTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.REDO_CHROMIUM_ANALYSIS_TASK_POST_URI, "POST", redoTaskHandler)

	// Do not add force login handler for update methods. They use webhooks for authentication.
	r.HandleFunc("/"+ctfeutil.UPDATE_CHROMIUM_ANALYSIS_TASK_POST_URI, updateTaskHandler).Methods("POST")
}
