/*
	Handlers and types specific to running Lua scripts.
*/

package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"path/filepath"
	"text/template"

	"go.skia.org/infra/ct/go/db"
	api "go.skia.org/infra/ct/go/frontend"
)

var (
	luaScriptsTemplate           *template.Template = nil
	luaScriptRunsHistoryTemplate *template.Template = nil
)

func reloadLuaScriptTemplates() {
	luaScriptsTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/lua_scripts.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))
	luaScriptRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/lua_script_runs_history.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))
}

type LuaScriptDBTask struct {
	CommonCols

	PageSets            string         `db:"page_sets"`
	ChromiumRev         string         `db:"chromium_rev"`
	SkiaRev             string         `db:"skia_rev"`
	LuaScript           string         `db:"lua_script"`
	LuaAggregatorScript string         `db:"lua_aggregator_script"`
	Description         string         `db:"description"`
	ScriptOutput        sql.NullString `db:"script_output"`
	AggregatedOutput    sql.NullString `db:"aggregated_output"`
}

func (task LuaScriptDBTask) GetTaskName() string {
	return "LuaScript"
}

func (task LuaScriptDBTask) TableName() string {
	return db.TABLE_LUA_SCRIPT_TASKS
}

func (task LuaScriptDBTask) Select(query string, args ...interface{}) (interface{}, error) {
	result := []LuaScriptDBTask{}
	err := db.DB.Select(&result, query, args...)
	return result, err
}

func luaScriptsView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(luaScriptsTemplate, w, r)
}

type AddLuaScriptTaskVars struct {
	AddTaskCommonVars

	SkpRepository       CaptureSkpsDBTask `json:"skp_repository"`
	LuaScript           string            `json:"lua_script"`
	LuaAggregatorScript string            `json:"lua_aggregator_script"`
	Description         string            `json:"desc"`
}

func (task *AddLuaScriptTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.SkpRepository.PageSets == "" ||
		task.SkpRepository.ChromiumRev == "" ||
		task.SkpRepository.SkiaRev == "" ||
		task.LuaScript == "" ||
		task.Description == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	if err := validateSkpRepository(task.SkpRepository); err != nil {
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

func addLuaScriptTaskHandler(w http.ResponseWriter, r *http.Request) {
	addTaskHandler(w, r, &AddLuaScriptTaskVars{})
}

func getLuaScriptTasksHandler(w http.ResponseWriter, r *http.Request) {
	getTasksHandler(&LuaScriptDBTask{}, w, r)
}

// Define api.LuaScriptUpdateVars in this package so we can add methods.
type LuaScriptUpdateVars struct {
	api.LuaScriptUpdateVars
}

func (task *LuaScriptUpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
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

func updateLuaScriptTaskHandler(w http.ResponseWriter, r *http.Request) {
	updateTaskHandler(&LuaScriptUpdateVars{}, db.TABLE_LUA_SCRIPT_TASKS, w, r)
}

func deleteLuaScriptTaskHandler(w http.ResponseWriter, r *http.Request) {
	deleteTaskHandler(&LuaScriptDBTask{}, w, r)
}

func luaScriptRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(luaScriptRunsHistoryTemplate, w, r)
}
