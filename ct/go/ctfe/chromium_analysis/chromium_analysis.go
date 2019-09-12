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
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/httputils"
	"google.golang.org/api/iterator"
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
	ChromiumHash         string
	CCList               []string
	TaskPriority         int
	GroupName            string
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

	taskVars.RunInParallel = task.RunInParallel
	taskVars.Platform = task.Platform
	taskVars.RunOnGCE = task.RunOnGCE
	taskVars.MatchStdoutTxt = task.MatchStdoutTxt
	taskVars.ChromiumHash = task.ChromiumHash
	taskVars.CCList = task.CCList
	taskVars.TaskPriority = strconv.Itoa(task.TaskPriority)
	taskVars.GroupName = task.GroupName
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

func (task DatastoreTask) TriggerSwarmingTask(ctx context.Context) error {
	runID := task_common.GetRunID(&task)
	emails := []string{task.Username}
	emails = append(emails, task.CCList...)
	isolateArgs := map[string]string{
		"EMAILS":                      strings.Join(emails, ","),
		"DESCRIPTION":                 task.Description,
		"TASK_ID":                     strconv.FormatInt(task.DatastoreKey.ID, 10),
		"PAGESET_TYPE":                task.PageSets,
		"BENCHMARK":                   task.Benchmark,
		"BENCHMARK_ARGS":              task.BenchmarkArgs,
		"BROWSER_EXTRA_ARGS":          task.BrowserArgs,
		"RUN_IN_PARALLEL":             strconv.FormatBool(task.RunInParallel),
		"TARGET_PLATFORM":             task.Platform,
		"RUN_ON_GCE":                  strconv.FormatBool(task.RunsOnGCEWorkers()),
		"MATCH_STDOUT_TXT":            task.MatchStdoutTxt,
		"CHROMIUM_HASH":               task.ChromiumHash,
		"RUN_ID":                      runID,
		"TASK_PRIORITY":               strconv.Itoa(task.TaskPriority),
		"GROUP_NAME":                  task.GroupName,
		"CHROMIUM_PATCH_GS_PATH":      task.ChromiumPatchGSPath,
		"SKIA_PATCH_GS_PATH":          task.SkiaPatchGSPath,
		"V8_PATCH_GS_PATH":            task.V8PatchGSPath,
		"CATAPULT_PATCH_GS_PATH":      task.CatapultPatchGSPath,
		"CUSTOM_WEBPAGES_CSV_GS_PATH": task.CustomWebpagesGSPath,
		"DS_NAMESPACE":                task_common.DsNamespace,
		"DS_PROJECT_NAME":             task_common.DsProjectName,
	}

	if err := ctutil.TriggerMasterScriptSwarmingTask(ctx, runID, "run_chromium_analysis_on_workers", ctutil.CHROMIUM_ANALYSIS_MASTER_ISOLATE, task_common.ServiceAccountFile, task.Platform, false, isolateArgs); err != nil {
		return fmt.Errorf("Could not trigger master script for run_chromium_analysis_on_workers with isolate args %v: %s", isolateArgs, err)
	}
	return nil
}

func addTaskView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(addTaskTemplate, w, r)
}

type AddTaskVars struct {
	task_common.AddTaskCommonVars

	Benchmark      string   `json:"benchmark"`
	PageSets       string   `json:"page_sets"`
	CustomWebpages string   `json:"custom_webpages"`
	BenchmarkArgs  string   `json:"benchmark_args"`
	BrowserArgs    string   `json:"browser_args"`
	Description    string   `json:"desc"`
	ChromiumPatch  string   `json:"chromium_patch"`
	SkiaPatch      string   `json:"skia_patch"`
	CatapultPatch  string   `json:"catapult_patch"`
	BenchmarkPatch string   `json:"benchmark_patch"`
	V8Patch        string   `json:"v8_patch"`
	RunInParallel  bool     `json:"run_in_parallel"`
	Platform       string   `json:"platform"`
	RunOnGCE       bool     `json:"run_on_gce"`
	MatchStdoutTxt string   `json:"match_stdout_txt"`
	ChromiumHash   string   `json:"chromium_hash"`
	CCList         []string `json:"cc_list"`
	TaskPriority   string   `json:"task_priority"`
	GroupName      string   `json:"group_name"`
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
	if task.GroupName != "" && len(task.GroupName) >= ctfeutil.MAX_GROUPNAME_LEN {
		return nil, fmt.Errorf("Please limit group names to less than %d characters", ctfeutil.MAX_GROUPNAME_LEN)
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
		ChromiumPatchGSPath:  chromiumPatchGSPath,
		SkiaPatchGSPath:      skiaPatchGSPath,
		CatapultPatchGSPath:  catapultPatchGSPath,
		BenchmarkPatchGSPath: benchmarkPatchGSPath,
		V8PatchGSPath:        v8PatchGSPath,

		RunInParallel:  task.RunInParallel,
		Platform:       task.Platform,
		RunOnGCE:       task.RunOnGCE,
		MatchStdoutTxt: task.MatchStdoutTxt,
		ChromiumHash:   task.ChromiumHash,
		CCList:         task.CCList,
		GroupName:      task.GroupName,
	}
	taskPriority, err := strconv.Atoi(task.TaskPriority)
	if err != nil {
		return nil, fmt.Errorf("%s is not int: %s", task.TaskPriority, err)
	}
	if taskPriority == 0 {
		// This should only happen for repeating tasks that were created before
		// support for task priorities was added to CT.
		// Triggering tasks with 0 priority fails in swarming with
		// "priority 0 can only be used for terminate request"
		// Override it to the medium priority.
		taskPriority = ctutil.TASKS_PRIORITY_MEDIUM
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

	RawOutput string
}

func (task *UpdateVars) GetTaskPrototype() task_common.Task {
	return &DatastoreTask{}
}

func (vars *UpdateVars) UpdateExtraFields(t task_common.Task) error {
	task := t.(*DatastoreTask)
	if vars.RawOutput != "" {
		task.RawOutput = vars.RawOutput
	}
	return nil
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

func AddHandlers(externalRouter *mux.Router) {
	externalRouter.HandleFunc("/"+ctfeutil.CHROMIUM_ANALYSIS_URI, addTaskView).Methods("GET")
	externalRouter.HandleFunc("/"+ctfeutil.CHROMIUM_ANALYSIS_RUNS_URI, runsHistoryView).Methods("GET")

	externalRouter.HandleFunc("/"+ctfeutil.ADD_CHROMIUM_ANALYSIS_TASK_POST_URI, addTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.GET_CHROMIUM_ANALYSIS_TASKS_POST_URI, getTasksHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.DELETE_CHROMIUM_ANALYSIS_TASK_POST_URI, deleteTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.REDO_CHROMIUM_ANALYSIS_TASK_POST_URI, redoTaskHandler).Methods("POST")
}
