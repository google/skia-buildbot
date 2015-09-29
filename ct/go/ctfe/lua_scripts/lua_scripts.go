/*
	Handlers and types specific to running Lua scripts.
*/

package lua_scripts

import (
	"database/sql"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/gorilla/mux"

	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	"go.skia.org/infra/ct/go/db"
	ctutil "go.skia.org/infra/ct/go/util"
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

type DBTask struct {
	task_common.CommonCols

	PageSets            string         `db:"page_sets"`
	ChromiumRev         string         `db:"chromium_rev"`
	SkiaRev             string         `db:"skia_rev"`
	LuaScript           string         `db:"lua_script"`
	LuaAggregatorScript string         `db:"lua_aggregator_script"`
	Description         string         `db:"description"`
	ScriptOutput        sql.NullString `db:"script_output"`
	AggregatedOutput    sql.NullString `db:"aggregated_output"`
}

func (task DBTask) GetTaskName() string {
	return "LuaScript"
}

func (task DBTask) GetResultsLink() string {
	if task.AggregatedOutput.Valid && task.AggregatedOutput.String != "" {
		return task.AggregatedOutput.String
	} else if task.ScriptOutput.Valid {
		return task.ScriptOutput.String
	}
	return ""
}

func (dbTask DBTask) GetPopulatedAddTaskVars() task_common.AddTaskVars {
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

func (task DBTask) GetUpdateTaskVars() task_common.UpdateTaskVars {
	return &UpdateVars{}
}

func (task DBTask) TableName() string {
	return db.TABLE_LUA_SCRIPT_TASKS
}

func (task DBTask) Select(query string, args ...interface{}) (interface{}, error) {
	result := []DBTask{}
	err := db.DB.Select(&result, query, args...)
	return result, err
}

func addTaskView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(addTaskTemplate, w, r)
}

type AddTaskVars struct {
	task_common.AddTaskCommonVars

	SkpRepository       capture_skps.DBTask `json:"skp_repository"`
	LuaScript           string              `json:"lua_script"`
	LuaAggregatorScript string              `json:"lua_aggregator_script"`
	Description         string              `json:"desc"`
}

func (task *AddTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.SkpRepository.PageSets == "" ||
		task.SkpRepository.ChromiumRev == "" ||
		task.SkpRepository.SkiaRev == "" ||
		task.LuaScript == "" ||
		task.Description == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	if err := capture_skps.Validate(task.SkpRepository); err != nil {
		return "", nil, err
	}
	if err := ctfeutil.CheckLengths([]ctfeutil.LengthCheck{
		{"lua_script", task.LuaScript, db.TEXT_MAX_LENGTH},
		{"lua_aggregator_script", task.LuaAggregatorScript, db.TEXT_MAX_LENGTH},
		{"description", task.Description, 255},
	}); err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("INSERT INTO %s (username,page_sets,chromium_rev,skia_rev,lua_script,lua_aggregator_script,description,ts_added,repeat_after_days) VALUES (?,?,?,?,?,?,?,?,?);",
			db.TABLE_LUA_SCRIPT_TASKS),
		[]interface{}{
			task.Username,
			task.SkpRepository.PageSets,
			task.SkpRepository.ChromiumRev,
			task.SkpRepository.SkiaRev,
			task.LuaScript,
			task.LuaAggregatorScript,
			task.Description,
			task.TsAdded,
			task.RepeatAfterDays,
		},
		nil
}

func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.AddTaskHandler(w, r, &AddTaskVars{})
}

func getTasksHandler(w http.ResponseWriter, r *http.Request) {
	task_common.GetTasksHandler(&DBTask{}, w, r)
}

type UpdateVars struct {
	task_common.UpdateTaskCommonVars
	ScriptOutput     sql.NullString `db:"script_output"`
	AggregatedOutput sql.NullString `db:"aggregated_output"`
}

func (vars *UpdateVars) UriPath() string {
	return ctfeutil.UPDATE_LUA_SCRIPT_TASK_POST_URI
}

func (task *UpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
	if err := ctfeutil.CheckLengths([]ctfeutil.LengthCheck{
		{"ScriptOutput", task.ScriptOutput.String, 255},
		{"AggregatedOutput", task.AggregatedOutput.String, 255},
	}); err != nil {
		return nil, nil, err
	}
	clauses := []string{}
	args := []interface{}{}
	if task.ScriptOutput.Valid {
		clauses = append(clauses, "script_output = ?")
		args = append(args, task.ScriptOutput.String)
	}
	if task.AggregatedOutput.Valid {
		clauses = append(clauses, "aggregated_output = ?")
		args = append(args, task.AggregatedOutput.String)
	}
	return clauses, args, nil
}

func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.UpdateTaskHandler(&UpdateVars{}, db.TABLE_LUA_SCRIPT_TASKS, w, r)
}

func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.DeleteTaskHandler(&DBTask{}, w, r)
}

func redoTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.RedoTaskHandler(&DBTask{}, w, r)
}

func runsHistoryView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(runsHistoryTemplate, w, r)
}

func AddHandlers(r *mux.Router) {
	r.HandleFunc("/"+ctfeutil.LUA_SCRIPT_URI, addTaskView).Methods("GET")
	r.HandleFunc("/"+ctfeutil.LUA_SCRIPT_RUNS_URI, runsHistoryView).Methods("GET")
	r.HandleFunc("/"+ctfeutil.ADD_LUA_SCRIPT_TASK_POST_URI, addTaskHandler).Methods("POST")
	r.HandleFunc("/"+ctfeutil.GET_LUA_SCRIPT_TASKS_POST_URI, getTasksHandler).Methods("POST")
	r.HandleFunc("/"+ctfeutil.UPDATE_LUA_SCRIPT_TASK_POST_URI, updateTaskHandler).Methods("POST")
	r.HandleFunc("/"+ctfeutil.DELETE_LUA_SCRIPT_TASK_POST_URI, deleteTaskHandler).Methods("POST")
	r.HandleFunc("/"+ctfeutil.REDO_LUA_SCRIPT_TASK_POST_URI, redoTaskHandler).Methods("POST")
}
