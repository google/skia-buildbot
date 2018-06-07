/*
	Handlers and types specific to Chromium perf tasks.
*/

package chromium_perf

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
)

var (
	addTaskTemplate     *template.Template = nil
	runsHistoryTemplate *template.Template = nil
)

func ReloadTemplates(resourcesDir string) {
	addTaskTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/chromium_perf.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
	runsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/chromium_perf_runs_history.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
}

type DatastoreTask struct {
	task_common.CommonCols

	Benchmark            string
	Platform             string
	PageSets             string
	IsTestPageSet        bool
	CustomWebpages       string
	RepeatRuns           int64
	RunInParallel        bool
	BenchmarkArgs        string
	BrowserArgsNoPatch   string
	BrowserArgsWithPatch string
	Description          string
	ChromiumPatchGSPath  string
	BlinkPatchGSPath     string
	SkiaPatchGSPath      string
	CatapultPatchGSPath  string
	BenchmarkPatchGSPath string
	V8PatchGSPath        string
	Results              string
	NoPatchRawOutput     string
	WithPatchRawOutput   string

	ChromiumPatch  string `datastore:"-"`
	BlinkPatch     string `datastore:"-"`
	SkiaPatch      string `datastore:"-"`
	CatapultPatch  string `datastore:"-"`
	BenchmarkPatch string `datastore:"-"`
	V8Patch        string `datastore:"-"`
}

func (task DatastoreTask) GetTaskName() string {
	return "ChromiumPerf"
}

func (dbTask DatastoreTask) GetPopulatedAddTaskVars() (task_common.AddTaskVars, error) {
	taskVars := &AddTaskVars{}
	taskVars.Username = dbTask.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(dbTask.RepeatAfterDays, 10)
	taskVars.Benchmark = dbTask.Benchmark
	taskVars.Platform = dbTask.Platform
	taskVars.PageSets = dbTask.PageSets
	taskVars.CustomWebpages = dbTask.CustomWebpages
	taskVars.RepeatRuns = strconv.FormatInt(dbTask.RepeatRuns, 10)
	taskVars.RunInParallel = strconv.FormatBool(dbTask.RunInParallel)
	taskVars.BenchmarkArgs = dbTask.BenchmarkArgs
	taskVars.BrowserArgsNoPatch = dbTask.BrowserArgsNoPatch
	taskVars.BrowserArgsWithPatch = dbTask.BrowserArgsWithPatch
	taskVars.Description = dbTask.Description

	var err error
	taskVars.ChromiumPatch, err = ctutil.GetPatchFromStorage(dbTask.ChromiumPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", dbTask.ChromiumPatchGSPath)
	}
	taskVars.BlinkPatch, err = ctutil.GetPatchFromStorage(dbTask.BlinkPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", dbTask.BlinkPatchGSPath)
	}
	taskVars.SkiaPatch, err = ctutil.GetPatchFromStorage(dbTask.SkiaPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", dbTask.SkiaPatchGSPath)
	}
	taskVars.CatapultPatch, err = ctutil.GetPatchFromStorage(dbTask.CatapultPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", dbTask.CatapultPatchGSPath)
	}
	taskVars.BenchmarkPatch, err = ctutil.GetPatchFromStorage(dbTask.BenchmarkPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", dbTask.BenchmarkPatchGSPath)
	}
	taskVars.V8Patch, err = ctutil.GetPatchFromStorage(dbTask.V8PatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", dbTask.V8PatchGSPath)
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
	// Perf tasks should always run on bare-metal machines.
	return false
}

func (task DatastoreTask) GetDatastoreKind() ds.Kind {
	return ds.CHROMIUM_PERF_TASKS
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
		t.ChromiumPatch, err = ctutil.GetPatchFromStorage(t.ChromiumPatchGSPath)
		if err != nil {
			return nil, fmt.Errorf("Could not read from %s: %s", t.ChromiumPatchGSPath)
		}
		t.BlinkPatch, err = ctutil.GetPatchFromStorage(t.BlinkPatchGSPath)
		if err != nil {
			return nil, fmt.Errorf("Could not read from %s: %s", t.BlinkPatchGSPath)
		}
		t.SkiaPatch, err = ctutil.GetPatchFromStorage(t.SkiaPatchGSPath)
		if err != nil {
			return nil, fmt.Errorf("Could not read from %s: %s", t.SkiaPatchGSPath)
		}
		t.CatapultPatch, err = ctutil.GetPatchFromStorage(t.CatapultPatchGSPath)
		if err != nil {
			return nil, fmt.Errorf("Could not read from %s: %s", t.CatapultPatchGSPath)
		}
		t.BenchmarkPatch, err = ctutil.GetPatchFromStorage(t.BenchmarkPatchGSPath)
		if err != nil {
			return nil, fmt.Errorf("Could not read from %s: %s", t.BenchmarkPatchGSPath)
		}
		t.V8Patch, err = ctutil.GetPatchFromStorage(t.V8PatchGSPath)
		if err != nil {
			return nil, fmt.Errorf("Could not read from %s: %s", t.V8PatchGSPath)
		}
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

	Benchmark            string `json:"benchmark"`
	Platform             string `json:"platform"`
	PageSets             string `json:"page_sets"`
	CustomWebpages       string `json:"custom_webpages"`
	RepeatRuns           string `json:"repeat_runs"`
	RunInParallel        string `json:"run_in_parallel"`
	BenchmarkArgs        string `json:"benchmark_args"`
	BrowserArgsNoPatch   string `json:"browser_args_nopatch"`
	BrowserArgsWithPatch string `json:"browser_args_withpatch"`
	Description          string `json:"desc"`
	ChromiumPatch        string `json:"chromium_patch"`
	BlinkPatch           string `json:"blink_patch"`
	SkiaPatch            string `json:"skia_patch"`
	CatapultPatch        string `json:"catapult_patch"`
	BenchmarkPatch       string `json:"benchmark_patch"`
	V8Patch              string `json:"v8_patch"`
}

func (task *AddTaskVars) GetDatastoreKind() ds.Kind {
	return ds.CHROMIUM_PERF_TASKS
}

func (task *AddTaskVars) GetPopulatedDatastoreTask() (task_common.Task, error) {
	if task.Benchmark == "" ||
		task.Platform == "" ||
		task.PageSets == "" ||
		task.RepeatRuns == "" ||
		task.RunInParallel == "" ||
		task.Description == "" {
		return nil, fmt.Errorf("Invalid parameters")
	}

	customWebpages, err := ctfeutil.GetQualifiedCustomWebpages(task.CustomWebpages, task.BenchmarkArgs)
	if err != nil {
		return nil, err
	}

	chromiumPatchGSPath, err := ctutil.SavePatchToStorage(task.ChromiumPatch)
	if err != nil {
		return nil, fmt.Errorf("Could not save chromium patch to storage: %s", err)
	}
	blinkPatchGSPath, err := ctutil.SavePatchToStorage(task.BlinkPatch)
	if err != nil {
		return nil, fmt.Errorf("Could not save blink patch to storage: %s", err)
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

	id, err := task_common.GetNextId(ds.CHROMIUM_PERF_TASKS, &DatastoreTask{})
	if err != nil {
		return nil, fmt.Errorf("Could not get highest id: %s", err)
	}

	t := &DatastoreTask{
		Benchmark:            task.Benchmark,
		Platform:             task.Platform,
		PageSets:             task.PageSets,
		IsTestPageSet:        task.PageSets == ctutil.PAGESET_TYPE_DUMMY_1k,
		CustomWebpages:       strings.Join(customWebpages, ","),
		BenchmarkArgs:        task.BenchmarkArgs,
		BrowserArgsNoPatch:   task.BrowserArgsNoPatch,
		BrowserArgsWithPatch: task.BrowserArgsWithPatch,
		Description:          task.Description,
		ChromiumPatchGSPath:  chromiumPatchGSPath,
		BlinkPatchGSPath:     blinkPatchGSPath,
		SkiaPatchGSPath:      skiaPatchGSPath,
		CatapultPatchGSPath:  catapultPatchGSPath,
		BenchmarkPatchGSPath: benchmarkPatchGSPath,
		V8PatchGSPath:        v8PatchGSPath,
	}
	runInParallel, err := strconv.ParseBool(task.RunInParallel)
	if err != nil {
		return nil, fmt.Errorf("%s is not bool: %s", task.RunInParallel, err)
	}
	t.RunInParallel = runInParallel
	repeatRuns, err := strconv.ParseInt(task.RepeatRuns, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s is not int64: %s", task.RepeatRuns, err)
	}
	t.RepeatRuns = repeatRuns
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

	Results            string
	NoPatchRawOutput   string
	WithPatchRawOutput string
}

func (vars *UpdateVars) UriPath() string {
	return ctfeutil.UPDATE_CHROMIUM_PERF_TASK_POST_URI
}

func (task *UpdateVars) AddUpdatesToDatastoreTask(t task_common.Task) error {
	dbTask := t.(*DatastoreTask)
	if task.NoPatchRawOutput != "" {
		dbTask.NoPatchRawOutput = task.NoPatchRawOutput
	}
	if task.WithPatchRawOutput != "" {
		dbTask.WithPatchRawOutput = task.WithPatchRawOutput
	}
	if task.Results != "" {
		dbTask.Results = task.Results
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
	ctfeutil.AddForceLoginHandler(r, "/", "GET", addTaskView)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.CHROMIUM_PERF_URI, "GET", addTaskView)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.CHROMIUM_PERF_RUNS_URI, "GET", runsHistoryView)

	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.ADD_CHROMIUM_PERF_TASK_POST_URI, "POST", addTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.GET_CHROMIUM_PERF_TASKS_POST_URI, "POST", getTasksHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.DELETE_CHROMIUM_PERF_TASK_POST_URI, "POST", deleteTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.REDO_CHROMIUM_PERF_TASK_POST_URI, "POST", redoTaskHandler)

	// Do not add force login handler for update methods. They use webhooks for authentication.
	r.HandleFunc("/"+ctfeutil.UPDATE_CHROMIUM_PERF_TASK_POST_URI, updateTaskHandler).Methods("POST")
}
