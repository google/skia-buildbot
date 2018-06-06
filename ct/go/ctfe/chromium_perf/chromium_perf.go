/*
	Handlers and types specific to Chromium perf tasks.
*/

package chromium_perf

import (
	"context"
	"net/http"
	"path/filepath"
	"strconv"
	"text/template"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"

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

	Benchmark            string `db:"benchmark"`
	Platform             string `db:"platform"`
	PageSets             string `db:"page_sets"`
	CustomWebpages       string `db:"custom_webpages"`
	RepeatRuns           int64  `db:"repeat_runs"`
	RunInParallel        bool   `db:"run_in_parallel"`
	BenchmarkArgs        string `db:"benchmark_args"`
	BrowserArgsNoPatch   string `db:"browser_args_nopatch"`
	BrowserArgsWithPatch string `db:"browser_args_withpatch"`
	Description          string `db:"description"`
	ChromiumPatch        string `db:"chromium_patch"`
	BlinkPatch           string `db:"blink_patch"`
	SkiaPatch            string `db:"skia_patch"`
	CatapultPatch        string `db:"catapult_patch"`
	BenchmarkPatch       string `db:"benchmark_patch"`
	V8Patch              string `db:"v8_patch"`
	Results              string `db:"results"`
	NoPatchRawOutput     string `db:"nopatch_raw_output"`
	WithPatchRawOutput   string `db:"withpatch_raw_output"`
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

	taskVars.ChromiumPatch = dbTask.ChromiumPatch
	taskVars.BlinkPatch = dbTask.BlinkPatch
	taskVars.SkiaPatch = dbTask.SkiaPatch
	taskVars.CatapultPatch = dbTask.CatapultPatch
	taskVars.BenchmarkPatch = dbTask.BenchmarkPatch
	taskVars.V8Patch = dbTask.V8Patch
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
	//result := []DatastoreTask{}
	//err := db.DB.Select(&result, query, args...)
	//return result, err
	return nil, nil
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
	return ds.CAPTURE_SKPS_TASKS
}

func (task *AddTaskVars) GetPopulatedDatastoreTask() (task_common.Task, error) {
	return nil, nil
}

//func (task *AddTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
//	if task.Benchmark == "" ||
//		task.Platform == "" ||
//		task.PageSets == "" ||
//		task.RepeatRuns == "" ||
//		task.RunInParallel == "" ||
//		task.Description == "" {
//		return "", nil, fmt.Errorf("Invalid parameters")
//	}
//	customWebpages, err := ctfeutil.GetQualifiedCustomWebpages(task.CustomWebpages, task.BenchmarkArgs)
//	if err != nil {
//		return "", nil, err
//	}
//	if err := ctfeutil.CheckLengths([]ctfeutil.LengthCheck{
//		{Name: "benchmark", Value: task.Benchmark, Limit: 100},
//		{Name: "platform", Value: task.Platform, Limit: 100},
//		{Name: "page_sets", Value: task.PageSets, Limit: 100},
//		{Name: "benchmark_args", Value: task.BenchmarkArgs, Limit: 255},
//		{Name: "browser_args_nopatch", Value: task.BrowserArgsNoPatch, Limit: 255},
//		{Name: "browser_args_withpatch", Value: task.BrowserArgsWithPatch, Limit: 255},
//		{Name: "desc", Value: task.Description, Limit: 255},
//		{Name: "custom_webpages", Value: strings.Join(customWebpages, ","), Limit: db.LONG_TEXT_MAX_LENGTH},
//		{Name: "chromium_patch", Value: task.ChromiumPatch, Limit: db.LONG_TEXT_MAX_LENGTH},
//		{Name: "blink_patch", Value: task.BlinkPatch, Limit: db.LONG_TEXT_MAX_LENGTH},
//		{Name: "skia_patch", Value: task.SkiaPatch, Limit: db.LONG_TEXT_MAX_LENGTH},
//		{Name: "catapult_patch", Value: task.CatapultPatch, Limit: db.LONG_TEXT_MAX_LENGTH},
//		{Name: "benchmark_patch", Value: task.BenchmarkPatch, Limit: db.LONG_TEXT_MAX_LENGTH},
//		{Name: "v8_patch", Value: task.V8Patch, Limit: db.LONG_TEXT_MAX_LENGTH},
//	}); err != nil {
//		return "", nil, err
//	}
//	runInParallel := 0
//	if strings.EqualFold(task.RunInParallel, "True") {
//		runInParallel = 1
//	}
//	return fmt.Sprintf("INSERT INTO %s (username,benchmark,platform,page_sets,custom_webpages,repeat_runs,run_in_parallel, benchmark_args,browser_args_nopatch,browser_args_withpatch,description,chromium_patch,blink_patch,skia_patch,catapult_patch,benchmark_patch,v8_patch,ts_added,repeat_after_days) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);",
//			db.TABLE_CHROMIUM_PERF_TASKS),
//		[]interface{}{
//			task.Username,
//			task.Benchmark,
//			task.Platform,
//			task.PageSets,
//			strings.Join(customWebpages, ","),
//			task.RepeatRuns,
//			runInParallel,
//			task.BenchmarkArgs,
//			task.BrowserArgsNoPatch,
//			task.BrowserArgsWithPatch,
//			task.Description,
//			task.ChromiumPatch,
//			task.BlinkPatch,
//			task.SkiaPatch,
//			task.CatapultPatch,
//			task.BenchmarkPatch,
//			task.V8Patch,
//			task.TsAdded,
//			task.RepeatAfterDays,
//		},
//		nil
//}

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
