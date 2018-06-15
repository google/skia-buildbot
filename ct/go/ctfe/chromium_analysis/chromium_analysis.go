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

	Benchmark            string
	PageSets             string
	IsTestPageSet        bool
	BenchmarkArgs        string
	BrowserArgs          string
	Description          string
	CustomWebpagesGSPath string
	ChromiumPatchGSPath  string
	SkiaPatchGSPath      string
	CatapultPatchGSPath  string
	BenchmarkPatchGSPath string
	V8PatchGSPath        string
	RunInParallel        bool
	Platform             string
	RunOnGCE             bool
	RawOutput            string
	MatchStdoutTxt       string

	CustomWebpages string `datastore:"-"`
	ChromiumPatch  string `datastore:"-"`
	SkiaPatch      string `datastore:"-"`
	CatapultPatch  string `datastore:"-"`
	BenchmarkPatch string `datastore:"-"`
	V8Patch        string `datastore:"-"`
}

func getAllPatchesFromStorage(t *DatastoreTask) error {
	var err error
	t.CustomWebpages, err = ctutil.GetPatchFromStorage(t.CustomWebpagesGSPath)
	if err != nil {
		return fmt.Errorf("Could not read from %s: %s", t.CustomWebpagesGSPath, err)
	}
	t.ChromiumPatch, err = ctutil.GetPatchFromStorage(t.ChromiumPatchGSPath)
	if err != nil {
		return fmt.Errorf("Could not read from %s: %s", t.ChromiumPatchGSPath, err)
	}
	t.SkiaPatch, err = ctutil.GetPatchFromStorage(t.SkiaPatchGSPath)
	if err != nil {
		return fmt.Errorf("Could not read from %s: %s", t.SkiaPatchGSPath, err)
	}
	t.CatapultPatch, err = ctutil.GetPatchFromStorage(t.CatapultPatchGSPath)
	if err != nil {
		return fmt.Errorf("Could not read from %s: %s", t.CatapultPatchGSPath, err)
	}
	t.BenchmarkPatch, err = ctutil.GetPatchFromStorage(t.BenchmarkPatchGSPath)
	if err != nil {
		return fmt.Errorf("Could not read from %s: %s", t.BenchmarkPatchGSPath, err)
	}
	t.V8Patch, err = ctutil.GetPatchFromStorage(t.V8PatchGSPath)
	if err != nil {
		return fmt.Errorf("Could not read from %s: %s", t.V8PatchGSPath, err)
	}
	return nil
}

func (task DatastoreTask) GetTaskName() string {
	return "ChromiumAnalysis"
}

func (task *DatastoreTask) GetPopulatedAddTaskVars() (task_common.AddTaskVars, error) {
	taskVars := &AddTaskVars{}
	taskVars.Username = task.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(task.RepeatAfterDays, 10)
	taskVars.Benchmark = task.Benchmark
	taskVars.PageSets = task.PageSets
	taskVars.BenchmarkArgs = task.BenchmarkArgs
	taskVars.BrowserArgs = task.BrowserArgs
	taskVars.Description = task.Description

	taskVars.CustomWebpages = task.CustomWebpages
	taskVars.ChromiumPatch = task.ChromiumPatch
	taskVars.SkiaPatch = task.SkiaPatch
	taskVars.CatapultPatch = task.CatapultPatch
	taskVars.BenchmarkPatch = task.BenchmarkPatch
	taskVars.V8Patch = task.V8Patch

	taskVars.RunInParallel = task.RunInParallel
	taskVars.Platform = task.Platform
	taskVars.RunOnGCE = task.RunOnGCE
	taskVars.MatchStdoutTxt = task.MatchStdoutTxt
	return taskVars, nil
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
		if err := getAllPatchesFromStorage(t); err != nil {
			return nil, fmt.Errorf("Could not get all patches from storage: %s", err)
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
	if err := getAllPatchesFromStorage(t); err != nil {
		return nil, fmt.Errorf("Could not get all patches from storage: %s", err)
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

func (task *AddTaskVars) GetPopulatedDatastoreTask(ctx context.Context) (task_common.Task, error) {
	if task.Benchmark == "" ||
		task.PageSets == "" ||
		task.Platform == "" ||
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
	catapultPatchGSPath, err := ctutil.SavePatchToStorage(task.CatapultPatch)
	if err != nil {
		return nil, fmt.Errorf("Could not save catapult patch to storage: %s", err)
	}
	benchmarkPatchGSPath, err := ctutil.SavePatchToStorage(task.BenchmarkPatch)
	if err != nil {
		return nil, fmt.Errorf("Could not save benchmark patch to storage: %s", err)
	}
	v8PatchGSPath, err := ctutil.SavePatchToStorage(task.V8Patch)
	if err != nil {
		return nil, fmt.Errorf("Could not save v8 patch to storage: %s", err)
	}

	t := &DatastoreTask{
		Benchmark:     task.Benchmark,
		PageSets:      task.PageSets,
		IsTestPageSet: task.PageSets == ctutil.PAGESET_TYPE_DUMMY_1k,
		BenchmarkArgs: task.BenchmarkArgs,
		BrowserArgs:   task.BrowserArgs,
		Description:   task.Description,

		CustomWebpagesGSPath: customWebpagesGSPath,
		CustomWebpages:       customWebpages,

		ChromiumPatchGSPath: chromiumPatchGSPath,
		ChromiumPatch:       task.ChromiumPatch,

		SkiaPatchGSPath: skiaPatchGSPath,
		SkiaPatch:       task.SkiaPatch,

		CatapultPatchGSPath: catapultPatchGSPath,
		CatapultPatch:       task.CatapultPatch,

		BenchmarkPatchGSPath: benchmarkPatchGSPath,
		BenchmarkPatch:       task.BenchmarkPatch,

		V8PatchGSPath: v8PatchGSPath,
		V8Patch:       task.V8Patch,

		RunInParallel:  task.RunInParallel,
		Platform:       task.Platform,
		RunOnGCE:       task.RunOnGCE,
		MatchStdoutTxt: task.MatchStdoutTxt,
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

	RawOutput string
}

func (vars *UpdateVars) UriPath() string {
	return ctfeutil.UPDATE_CHROMIUM_ANALYSIS_TASK_POST_URI
}

func (vars *UpdateVars) UpdateExtraFields(t task_common.Task) error {
	task := t.(*DatastoreTask)
	if vars.RawOutput != "" {
		task.RawOutput = vars.RawOutput
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
