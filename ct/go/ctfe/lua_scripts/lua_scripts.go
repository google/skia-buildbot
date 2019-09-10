/*
	Handlers and types specific to running Lua scripts.
*/

package lua_scripts

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"text/template"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/email"
	skutil "go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
)

var (
	addTaskTemplate     *template.Template = nil
	runsHistoryTemplate *template.Template = nil
)

func ReloadTemplates(resourcesDir string) {
	addTaskTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/lua_scripts.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
	runsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/lua_script_runs_history.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
}

type DatastoreTask struct {
	task_common.CommonCols

	PageSets            string
	IsTestPageSet       bool
	ChromiumRev         string
	SkiaRev             string
	LuaScript           string `datastore:",noindex"`
	LuaAggregatorScript string `datastore:",noindex"`
	Description         string
	ScriptOutput        string
	AggregatedOutput    string
}

func (task DatastoreTask) GetTaskName() string {
	return "LuaScript"
}

func (task DatastoreTask) GetResultsLink() string {
	if task.AggregatedOutput != "" {
		return task.AggregatedOutput
	} else if task.ScriptOutput != "" {
		return task.ScriptOutput
	}
	return ""
}

func (task DatastoreTask) GetPopulatedAddTaskVars() (task_common.AddTaskVars, error) {
	taskVars := &AddTaskVars{}
	taskVars.Username = task.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(task.RepeatAfterDays, 10)

	taskVars.SkpRepository.ChromiumRev = task.ChromiumRev
	taskVars.SkpRepository.SkiaRev = task.SkiaRev
	taskVars.SkpRepository.PageSets = task.PageSets

	taskVars.LuaScript = task.LuaScript
	taskVars.LuaAggregatorScript = task.LuaAggregatorScript
	taskVars.Description = task.Description
	return taskVars, nil
}

func (task DatastoreTask) RunsOnGCEWorkers() bool {
	return true
}

func (task DatastoreTask) GetDatastoreKind() ds.Kind {
	return ds.LUA_SCRIPT_TASKS
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

func (task DatastoreTask) TriggerSwarmingTaskAndMail(ctx context.Context) error {
	runID := task_common.GetRunID(&task)
	luaScriptGSPath, err := ctutil.SavePatchToStorage(task.LuaScript)
	if err != nil {
		return err
	}
	luaAggregatorScriptGSPath := ""
	if task.LuaAggregatorScript != "" {
		luaAggregatorScriptGSPath, err = ctutil.SavePatchToStorage(task.LuaAggregatorScript)
		if err != nil {
			return err
		}
	}

	emails := task_common.GetEmailRecipients(task.Username, nil)
	isolateArgs := map[string]string{
		"PAGESET_TYPE":                  task.PageSets,
		"CHROMIUM_BUILD":                ctutil.ChromiumBuildDir(task.ChromiumRev, task.SkiaRev, ""),
		"RUN_ON_GCE":                    strconv.FormatBool(task.RunsOnGCEWorkers()),
		"RUN_ID":                        runID,
		"LUA_SCRIPT_GS_PATH":            luaScriptGSPath,
		"LUA_AGGREGATOR_SCRIPT_GS_PATH": luaAggregatorScriptGSPath,
	}

	sTaskID, err := ctutil.TriggerMasterScriptSwarmingTask(ctx, runID, "run_lua_on_workers", ctutil.RUN_LUA_MASTER_ISOLATE, task_common.ServiceAccountFile, ctutil.PLATFORM_LINUX, false, isolateArgs)
	if err != nil {
		return fmt.Errorf("Could not trigger master script for run_lua_on_workers with isolate args %v: %s", isolateArgs, err)
	}
	// Mark task as started in datastore.
	if err := task_common.UpdateTaskSetStarted(ctx, runID, sTaskID, &task); err != nil {
		return fmt.Errorf("Could not mark task as started in datastore: %s", err)
	}
	// Send start email.
	skutil.LogErr(ctfeutil.SendTaskStartEmail(task.DatastoreKey.ID, emails, "Lua script", runID, task.Description, ""))
	return nil
}

func (task DatastoreTask) SendCompletionEmail(ctx context.Context, completedSuccessfully bool) error {
	runID := task_common.GetRunID(&task)
	emails := task_common.GetEmailRecipients(task.Username, nil)
	emailSubject := fmt.Sprintf("Run lua script Cluster telemetry task has completed (#%d)", task.DatastoreKey.ID)
	failureHtml := ""
	viewActionMarkup := ""
	var err error

	if !completedSuccessfully {
		emailSubject += " with failures"
		failureHtml = ctfeutil.GetFailureEmailHtml(runID)
		if viewActionMarkup, err = email.GetViewActionMarkup(ctfeutil.GetSwarmingLogsLink(runID), "View Failure", "Direct link to the swarming logs"); err != nil {
			return fmt.Errorf("Failed to get view action markup: %s", err)
		}
	} else {
		if viewActionMarkup, err = email.GetViewActionMarkup(task.ScriptOutput, "View Results", "Direct link to the lua output"); err != nil {
			return fmt.Errorf("Failed to get view action markup: %s", err)
		}
	}
	scriptOutputHtml := ""
	if task.ScriptOutput != "" {
		scriptOutputHtml = fmt.Sprintf("The output of your script is available <a href='%s'>here</a>.<br/>\n", task.ScriptOutput)
	}
	aggregatorOutputHtml := ""
	if task.AggregatedOutput != "" {
		aggregatorOutputHtml = fmt.Sprintf("The aggregated output of your script is available <a href='%s'>here</a>.<br/>\n", task.AggregatedOutput)
	}
	bodyTemplate := `
	The Cluster telemetry queued task to run lua script on %s pageset has completed. %s.<br/>
	Run description: %s<br/>
	%s
	%s
	%s
	You can schedule more runs <a href="%s">here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, task.PageSets, ctfeutil.GetSwarmingLogsLink(runID), task.Description, failureHtml, scriptOutputHtml, aggregatorOutputHtml, task_common.WebappURL+ctfeutil.LUA_SCRIPT_URI)
	if err := ctfeutil.SendEmailWithMarkup(emails, emailSubject, emailBody, viewActionMarkup); err != nil {
		return fmt.Errorf("Error while sending email: %s", err)
	}

	return nil
}

func (task *DatastoreTask) SetCompleted(success bool) {
	if success {
		runID := task_common.GetRunID(task)
		task.ScriptOutput = ctutil.GetLuaOutputRemoteLink(runID)
		if task.LuaAggregatorScript != "" {
			task.AggregatedOutput = ctutil.GetLuaAggregatorOutputRemoteLink(runID)
		}
	}
	task.TsCompleted = ctutil.GetCurrentTsInt64()
	task.Failure = !success
	task.TaskDone = true
}

func addTaskView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(addTaskTemplate, w, r)
}

type AddTaskVars struct {
	task_common.AddTaskCommonVars

	SkpRepository       capture_skps.DatastoreTask `json:"skp_repository"`
	LuaScript           string                     `json:"lua_script"`
	LuaAggregatorScript string                     `json:"lua_aggregator_script"`
	Description         string                     `json:"desc"`
}

func (task *AddTaskVars) GetDatastoreKind() ds.Kind {
	return ds.LUA_SCRIPT_TASKS
}

func (task *AddTaskVars) GetPopulatedDatastoreTask(ctx context.Context) (task_common.Task, error) {
	if task.SkpRepository.PageSets == "" ||
		task.SkpRepository.ChromiumRev == "" ||
		task.SkpRepository.SkiaRev == "" ||
		task.LuaScript == "" ||
		task.Description == "" {
		return nil, fmt.Errorf("Invalid parameters")
	}
	if err := capture_skps.Validate(ctx, task.SkpRepository); err != nil {
		return nil, err
	}

	t := &DatastoreTask{
		PageSets:            task.SkpRepository.PageSets,
		IsTestPageSet:       task.SkpRepository.PageSets == ctutil.PAGESET_TYPE_DUMMY_1k,
		ChromiumRev:         task.SkpRepository.ChromiumRev,
		SkiaRev:             task.SkpRepository.SkiaRev,
		LuaScript:           task.LuaScript,
		LuaAggregatorScript: task.LuaAggregatorScript,
		Description:         task.Description,
	}
	return t, nil
}

func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.AddTaskHandler(w, r, &AddTaskVars{})
}

func getTasksHandler(w http.ResponseWriter, r *http.Request) {
	task_common.GetTasksHandler(&DatastoreTask{}, w, r)
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
	externalRouter.HandleFunc("/"+ctfeutil.LUA_SCRIPT_URI, addTaskView).Methods("GET")
	externalRouter.HandleFunc("/"+ctfeutil.LUA_SCRIPT_RUNS_URI, runsHistoryView).Methods("GET")

	externalRouter.HandleFunc("/"+ctfeutil.ADD_LUA_SCRIPT_TASK_POST_URI, addTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.GET_LUA_SCRIPT_TASKS_POST_URI, getTasksHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.DELETE_LUA_SCRIPT_TASK_POST_URI, deleteTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.REDO_LUA_SCRIPT_TASK_POST_URI, redoTaskHandler).Methods("POST")
}
