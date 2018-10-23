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
	RepeatRuns           int64
	RunInParallel        bool
	BenchmarkArgs        string
	BrowserArgsNoPatch   string
	BrowserArgsWithPatch string
	Description          string
	CustomWebpagesGSPath string
	ChromiumPatchGSPath  string
	BlinkPatchGSPath     string
	SkiaPatchGSPath      string
	CatapultPatchGSPath  string
	BenchmarkPatchGSPath string
	V8PatchGSPath        string
	Results              string
	NoPatchRawOutput     string
	WithPatchRawOutput   string
	CCList               []string
	TaskPriority         int
}

func (task DatastoreTask) GetTaskName() string {
	return "ChromiumPerf"
}

func (task DatastoreTask) GetPopulatedAddTaskVars() (task_common.AddTaskVars, error) {
	taskVars := &AddTaskVars{}
	taskVars.Username = task.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(task.RepeatAfterDays, 10)
	taskVars.Benchmark = task.Benchmark
	taskVars.Platform = task.Platform
	taskVars.PageSets = task.PageSets
	taskVars.RepeatRuns = strconv.FormatInt(task.RepeatRuns, 10)
	taskVars.RunInParallel = strconv.FormatBool(task.RunInParallel)
	taskVars.BenchmarkArgs = task.BenchmarkArgs
	taskVars.BrowserArgsNoPatch = task.BrowserArgsNoPatch
	taskVars.BrowserArgsWithPatch = task.BrowserArgsWithPatch
	taskVars.Description = task.Description
	taskVars.CCList = task.CCList
	taskVars.TaskPriority = strconv.Itoa(task.TaskPriority)

	var err error
	taskVars.CustomWebpages, err = ctutil.GetPatchFromStorage(task.CustomWebpagesGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", task.CustomWebpagesGSPath, err)
	}
	taskVars.ChromiumPatch, err = ctutil.GetPatchFromStorage(task.ChromiumPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", task.ChromiumPatchGSPath, err)
	}
	taskVars.BlinkPatch, err = ctutil.GetPatchFromStorage(task.BlinkPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", task.BlinkPatchGSPath, err)
	}
	taskVars.SkiaPatch, err = ctutil.GetPatchFromStorage(task.SkiaPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", task.SkiaPatchGSPath, err)
	}
	taskVars.CatapultPatch, err = ctutil.GetPatchFromStorage(task.CatapultPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", task.CatapultPatchGSPath, err)
	}
	taskVars.BenchmarkPatch, err = ctutil.GetPatchFromStorage(task.BenchmarkPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", task.BenchmarkPatchGSPath, err)
	}
	taskVars.V8Patch, err = ctutil.GetPatchFromStorage(task.V8PatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", task.V8PatchGSPath, err)
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

	Benchmark            string   `json:"benchmark"`
	Platform             string   `json:"platform"`
	PageSets             string   `json:"page_sets"`
	CustomWebpages       string   `json:"custom_webpages"`
	RepeatRuns           string   `json:"repeat_runs"`
	RunInParallel        string   `json:"run_in_parallel"`
	BenchmarkArgs        string   `json:"benchmark_args"`
	BrowserArgsNoPatch   string   `json:"browser_args_nopatch"`
	BrowserArgsWithPatch string   `json:"browser_args_withpatch"`
	Description          string   `json:"desc"`
	CCList               []string `json:"cc_list"`
	TaskPriority         string   `json:"task_priority"`

	ChromiumPatch  string `json:"chromium_patch"`
	BlinkPatch     string `json:"blink_patch"`
	SkiaPatch      string `json:"skia_patch"`
	CatapultPatch  string `json:"catapult_patch"`
	BenchmarkPatch string `json:"benchmark_patch"`
	V8Patch        string `json:"v8_patch"`
}

func (task *AddTaskVars) GetDatastoreKind() ds.Kind {
	return ds.CHROMIUM_PERF_TASKS
}

func (task *AddTaskVars) GetPopulatedDatastoreTask(ctx context.Context) (task_common.Task, error) {
	if task.Benchmark == "" ||
		task.Platform == "" ||
		task.PageSets == "" ||
		task.RepeatRuns == "" ||
		task.RunInParallel == "" ||
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

	t := &DatastoreTask{
		Benchmark:            task.Benchmark,
		Platform:             task.Platform,
		PageSets:             task.PageSets,
		IsTestPageSet:        task.PageSets == ctutil.PAGESET_TYPE_DUMMY_1k,
		BenchmarkArgs:        task.BenchmarkArgs,
		BrowserArgsNoPatch:   task.BrowserArgsNoPatch,
		BrowserArgsWithPatch: task.BrowserArgsWithPatch,
		Description:          task.Description,
		CCList:               task.CCList,

		CustomWebpagesGSPath: customWebpagesGSPath,
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
	taskPriority, err := strconv.Atoi(task.TaskPriority)
	if err != nil {
		return nil, fmt.Errorf("%s is not int: %s", task.TaskPriority, err)
	}
	t.TaskPriority = taskPriority
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

func (vars *UpdateVars) UpdateExtraFields(t task_common.Task) error {
	task := t.(*DatastoreTask)
	if vars.NoPatchRawOutput != "" {
		task.NoPatchRawOutput = vars.NoPatchRawOutput
	}
	if vars.WithPatchRawOutput != "" {
		task.WithPatchRawOutput = vars.WithPatchRawOutput
	}
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
	externalRouter.HandleFunc("/", addTaskView).Methods("GET")
	externalRouter.HandleFunc("/"+ctfeutil.CHROMIUM_PERF_URI, addTaskView).Methods("GET")
	externalRouter.HandleFunc("/"+ctfeutil.CHROMIUM_PERF_RUNS_URI, runsHistoryView).Methods("GET")

	externalRouter.HandleFunc("/"+ctfeutil.ADD_CHROMIUM_PERF_TASK_POST_URI, addTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.GET_CHROMIUM_PERF_TASKS_POST_URI, getTasksHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.DELETE_CHROMIUM_PERF_TASK_POST_URI, deleteTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.REDO_CHROMIUM_PERF_TASK_POST_URI, redoTaskHandler).Methods("POST")

	// Updating tasks is done via the internal router.
	internalRouter.HandleFunc("/"+ctfeutil.UPDATE_CHROMIUM_PERF_TASK_POST_URI, updateTaskHandler).Methods("POST")
}
