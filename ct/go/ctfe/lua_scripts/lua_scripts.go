/*
	Handlers and types specific to running Lua scripts.
*/

package lua_scripts

import (
	"context"
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

	PageSets            string `db:"page_sets"`
	ChromiumRev         string `db:"chromium_rev"`
	SkiaRev             string `db:"skia_rev"`
	LuaScript           string `db:"lua_script"`
	LuaAggregatorScript string `db:"lua_aggregator_script"`
	Description         string `db:"description"`
	ScriptOutput        string `db:"script_output"`
	AggregatedOutput    string `db:"aggregated_output"`
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

func (dbTask DatastoreTask) GetPopulatedAddTaskVars() task_common.AddTaskVars {
	taskVars := &AddTaskVars{}
	taskVars.Username = dbTask.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(dbTask.RepeatAfterDays, 10)

	taskVars.SkpRepository.ChromiumRev = dbTask.ChromiumRev
	taskVars.SkpRepository.SkiaRev = dbTask.SkiaRev
	taskVars.SkpRepository.PageSets = dbTask.PageSets

	taskVars.LuaScript = dbTask.LuaScript
	taskVars.LuaAggregatorScript = dbTask.LuaAggregatorScript
	taskVars.Description = dbTask.Description
	return taskVars
}

func (task DatastoreTask) GetUpdateTaskVars() task_common.UpdateTaskVars {
	return &UpdateVars{}
}

func (task DatastoreTask) RunsOnGCEWorkers() bool {
	return true
}

func (task DatastoreTask) GetDatastoreKind() ds.Kind {
	return ds.LUA_SCRIPT_TASKS
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

	SkpRepository       capture_skps.DatastoreTask `json:"skp_repository"`
	LuaScript           string                     `json:"lua_script"`
	LuaAggregatorScript string                     `json:"lua_aggregator_script"`
	Description         string                     `json:"desc"`
}

func (task *AddTaskVars) GetDatastoreKind() ds.Kind {
	return ds.CAPTURE_SKPS_TASKS
}

func (task *AddTaskVars) GetPopulatedDatastoreTask() (task_common.Task, error) {
	return nil, nil
}

//func (task *AddTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
//	if task.SkpRepository.PageSets == "" ||
//		task.SkpRepository.ChromiumRev == "" ||
//		task.SkpRepository.SkiaRev == "" ||
//		task.LuaScript == "" ||
//		task.Description == "" {
//		return "", nil, fmt.Errorf("Invalid parameters")
//	}
//	if err := capture_skps.Validate(task.SkpRepository); err != nil {
//		return "", nil, err
//	}
//	if err := ctfeutil.CheckLengths([]ctfeutil.LengthCheck{
//		{Name: "lua_script", Value: task.LuaScript, Limit: db.TEXT_MAX_LENGTH},
//		{Name: "lua_aggregator_script", Value: task.LuaAggregatorScript, Limit: db.TEXT_MAX_LENGTH},
//		{Name: "description", Value: task.Description, Limit: 255},
//	}); err != nil {
//		return "", nil, err
//	}
//	return fmt.Sprintf("INSERT INTO %s (username,page_sets,chromium_rev,skia_rev,lua_script,lua_aggregator_script,description,ts_added,repeat_after_days) VALUES (?,?,?,?,?,?,?,?,?);",
//			db.TABLE_LUA_SCRIPT_TASKS),
//		[]interface{}{
//			task.Username,
//			task.SkpRepository.PageSets,
//			task.SkpRepository.ChromiumRev,
//			task.SkpRepository.SkiaRev,
//			task.LuaScript,
//			task.LuaAggregatorScript,
//			task.Description,
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
	ScriptOutput     string `db:"script_output"`
	AggregatedOutput string `db:"aggregated_output"`
}

func (vars *UpdateVars) UriPath() string {
	return ctfeutil.UPDATE_LUA_SCRIPT_TASK_POST_URI
}

func (task *UpdateVars) AddUpdatesToDatastoreTask(t task_common.Task) error {
	dbTask := t.(*DatastoreTask)
	if task.ScriptOutput != "" {
		dbTask.ScriptOutput = task.ScriptOutput
	}
	if task.AggregatedOutput != "" {
		dbTask.AggregatedOutput = task.AggregatedOutput
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
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.LUA_SCRIPT_URI, "GET", addTaskView)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.LUA_SCRIPT_RUNS_URI, "GET", runsHistoryView)

	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.ADD_LUA_SCRIPT_TASK_POST_URI, "POST", addTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.GET_LUA_SCRIPT_TASKS_POST_URI, "POST", getTasksHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.DELETE_LUA_SCRIPT_TASK_POST_URI, "POST", deleteTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.REDO_LUA_SCRIPT_TASK_POST_URI, "POST", redoTaskHandler)

	// Do not add force login handler for update methods. They use webhooks for authentication.
	r.HandleFunc("/"+ctfeutil.UPDATE_LUA_SCRIPT_TASK_POST_URI, updateTaskHandler).Methods("POST")
}
