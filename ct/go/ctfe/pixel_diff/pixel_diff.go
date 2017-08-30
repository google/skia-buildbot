/*
	Handlers and types specific to Pixel diff tasks.
*/

package pixel_diff

import (
	"database/sql"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/gorilla/mux"

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
		filepath.Join(resourcesDir, "templates/pixel_diff.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
	runsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/pixel_diff_runs_history.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
}

type DBTask struct {
	task_common.CommonCols

	PageSets             string         `db:"page_sets"`
	CustomWebpages       string         `db:"custom_webpages"`
	BenchmarkArgs        string         `db:"benchmark_args"`
	BrowserArgsNoPatch   string         `db:"browser_args_nopatch"`
	BrowserArgsWithPatch string         `db:"browser_args_withpatch"`
	Description          string         `db:"description"`
	ChromiumPatch        string         `db:"chromium_patch"`
	SkiaPatch            string         `db:"skia_patch"`
	Results              sql.NullString `db:"results"`
}

func (task DBTask) GetTaskName() string {
	return "PixelDiff"
}

func (dbTask DBTask) GetPopulatedAddTaskVars() task_common.AddTaskVars {
	taskVars := &AddTaskVars{}
	taskVars.Username = dbTask.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(dbTask.RepeatAfterDays, 10)
	taskVars.PageSets = dbTask.PageSets
	taskVars.CustomWebpages = dbTask.CustomWebpages
	taskVars.BenchmarkArgs = dbTask.BenchmarkArgs
	taskVars.BrowserArgsNoPatch = dbTask.BrowserArgsNoPatch
	taskVars.BrowserArgsWithPatch = dbTask.BrowserArgsWithPatch
	taskVars.Description = dbTask.Description
	taskVars.ChromiumPatch = dbTask.ChromiumPatch
	taskVars.SkiaPatch = dbTask.SkiaPatch
	return taskVars
}

func (task DBTask) GetResultsLink() string {
	if task.Results.Valid {
		return task.Results.String
	} else {
		return ""
	}
}

func (task DBTask) GetUpdateTaskVars() task_common.UpdateTaskVars {
	return &UpdateVars{}
}

func (task DBTask) RunsOnGCEWorkers() bool {
	return true
}

func (task DBTask) TableName() string {
	return db.TABLE_PIXEL_DIFF_TASKS
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

	PageSets             string `json:"page_sets"`
	CustomWebpages       string `json:"custom_webpages"`
	BenchmarkArgs        string `json:"benchmark_args"`
	BrowserArgsNoPatch   string `json:"browser_args_nopatch"`
	BrowserArgsWithPatch string `json:"browser_args_withpatch"`
	Description          string `json:"desc"`
	ChromiumPatch        string `json:"chromium_patch"`
	SkiaPatch            string `json:"skia_patch"`
}

func (task *AddTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.PageSets == "" ||
		task.Description == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	customWebpages, err := ctfeutil.GetQualifiedCustomWebpages(task.CustomWebpages, task.BenchmarkArgs)
	if err != nil {
		return "", nil, err
	}
	if err := ctfeutil.CheckLengths([]ctfeutil.LengthCheck{
		{Name: "page_sets", Value: task.PageSets, Limit: 100},
		{Name: "benchmark_args", Value: task.BenchmarkArgs, Limit: 255},
		{Name: "browser_args_nopatch", Value: task.BrowserArgsNoPatch, Limit: 255},
		{Name: "browser_args_withpatch", Value: task.BrowserArgsWithPatch, Limit: 255},
		{Name: "desc", Value: task.Description, Limit: 255},
		{Name: "custom_webpages", Value: strings.Join(customWebpages, ","), Limit: db.LONG_TEXT_MAX_LENGTH},
		{Name: "chromium_patch", Value: task.ChromiumPatch, Limit: db.LONG_TEXT_MAX_LENGTH},
		{Name: "skia_patch", Value: task.SkiaPatch, Limit: db.LONG_TEXT_MAX_LENGTH},
	}); err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("INSERT INTO %s (username,page_sets,custom_webpages,benchmark_args,browser_args_nopatch,browser_args_withpatch,description,chromium_patch,skia_patch,ts_added,repeat_after_days) VALUES (?,?,?,?,?,?,?,?,?,?,?);",
			db.TABLE_PIXEL_DIFF_TASKS),
		[]interface{}{
			task.Username,
			task.PageSets,
			strings.Join(customWebpages, ","),
			task.BenchmarkArgs,
			task.BrowserArgsNoPatch,
			task.BrowserArgsWithPatch,
			task.Description,
			task.ChromiumPatch,
			task.SkiaPatch,
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

	Results sql.NullString
}

func (vars *UpdateVars) UriPath() string {
	return ctfeutil.UPDATE_PIXEL_DIFF_TASK_POST_URI
}

func (task *UpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
	if err := ctfeutil.CheckLengths([]ctfeutil.LengthCheck{
		{Name: "Results", Value: task.Results.String, Limit: 255},
	}); err != nil {
		return nil, nil, err
	}
	clauses := []string{}
	args := []interface{}{}
	if task.Results.Valid {
		clauses = append(clauses, "results = ?")
		args = append(args, task.Results.String)
	}
	return clauses, args, nil
}

func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.UpdateTaskHandler(&UpdateVars{}, db.TABLE_PIXEL_DIFF_TASKS, w, r)
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
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.PIXEL_DIFF_URI, "GET", addTaskView)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.PIXEL_DIFF_RUNS_URI, "GET", runsHistoryView)

	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.ADD_PIXEL_DIFF_TASK_POST_URI, "POST", addTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.GET_PIXEL_DIFF_TASKS_POST_URI, "POST", getTasksHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.DELETE_PIXEL_DIFF_TASK_POST_URI, "POST", deleteTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.REDO_PIXEL_DIFF_TASK_POST_URI, "POST", redoTaskHandler)

	// Do not add force login handler for update methods. They use webhooks for authentication.
	r.HandleFunc("/"+ctfeutil.UPDATE_PIXEL_DIFF_TASK_POST_URI, updateTaskHandler).Methods("POST")
}
