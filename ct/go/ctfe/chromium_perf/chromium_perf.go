/*
	Handlers and types specific to Chromium perf tasks.
*/

package chromium_perf

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"text/template"

	"github.com/gorilla/mux"

	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	"go.skia.org/infra/ct/go/db"
	api "go.skia.org/infra/ct/go/frontend"
	ctutil "go.skia.org/infra/ct/go/util"
	skutil "go.skia.org/infra/go/util"
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

type DBTask struct {
	task_common.CommonCols

	Benchmark            string         `db:"benchmark"`
	Platform             string         `db:"platform"`
	PageSets             string         `db:"page_sets"`
	RepeatRuns           int64          `db:"repeat_runs"`
	BenchmarkArgs        string         `db:"benchmark_args"`
	BrowserArgsNoPatch   string         `db:"browser_args_nopatch"`
	BrowserArgsWithPatch string         `db:"browser_args_withpatch"`
	Description          string         `db:"description"`
	ChromiumPatch        string         `db:"chromium_patch"`
	BlinkPatch           string         `db:"blink_patch"`
	SkiaPatch            string         `db:"skia_patch"`
	Results              sql.NullString `db:"results"`
	NoPatchRawOutput     sql.NullString `db:"nopatch_raw_output"`
	WithPatchRawOutput   sql.NullString `db:"withpatch_raw_output"`
}

func (task DBTask) GetTaskName() string {
	return "ChromiumPerf"
}

func (task DBTask) TableName() string {
	return db.TABLE_CHROMIUM_PERF_TASKS
}

func (task DBTask) Select(query string, args ...interface{}) (interface{}, error) {
	result := []DBTask{}
	err := db.DB.Select(&result, query, args...)
	return result, err
}

func addTaskView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(addTaskTemplate, w, r)
}

func parametersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	data := map[string]interface{}{
		"benchmarks": ctutil.SupportedBenchmarks,
		"platforms":  ctutil.SupportedPlatformsToDesc,
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

type AddTaskVars struct {
	task_common.AddTaskCommonVars

	Benchmark            string `json:"benchmark"`
	Platform             string `json:"platform"`
	PageSets             string `json:"page_sets"`
	RepeatRuns           string `json:"repeat_runs"`
	BenchmarkArgs        string `json:"benchmark_args"`
	BrowserArgsNoPatch   string `json:"browser_args_nopatch"`
	BrowserArgsWithPatch string `json:"browser_args_withpatch"`
	Description          string `json:"desc"`
	ChromiumPatch        string `json:"chromium_patch"`
	BlinkPatch           string `json:"blink_patch"`
	SkiaPatch            string `json:"skia_patch"`
}

func (task *AddTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.Benchmark == "" ||
		task.Platform == "" ||
		task.PageSets == "" ||
		task.RepeatRuns == "" ||
		task.Description == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	return fmt.Sprintf("INSERT INTO %s (username,benchmark,platform,page_sets,repeat_runs,benchmark_args,browser_args_nopatch,browser_args_withpatch,description,chromium_patch,blink_patch,skia_patch,ts_added,repeat_after_days) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?);",
			db.TABLE_CHROMIUM_PERF_TASKS),
		[]interface{}{
			task.Username,
			task.Benchmark,
			task.Platform,
			task.PageSets,
			task.RepeatRuns,
			task.BenchmarkArgs,
			task.BrowserArgsNoPatch,
			task.BrowserArgsWithPatch,
			task.Description,
			task.ChromiumPatch,
			task.BlinkPatch,
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

// Define api.ChromiumPerfUpdateVars in this package so we can add methods.
type UpdateVars struct {
	api.ChromiumPerfUpdateVars
}

func (task *UpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
	clauses := []string{}
	args := []interface{}{}
	if task.Results.Valid {
		clauses = append(clauses, "results = ?")
		args = append(args, task.Results.String)
	}
	if task.NoPatchRawOutput.Valid {
		clauses = append(clauses, "nopatch_raw_output = ?")
		args = append(args, task.NoPatchRawOutput.String)
	}
	if task.WithPatchRawOutput.Valid {
		clauses = append(clauses, "withpatch_raw_output = ?")
		args = append(args, task.WithPatchRawOutput.String)
	}
	return clauses, args, nil
}

func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.UpdateTaskHandler(&UpdateVars{}, db.TABLE_CHROMIUM_PERF_TASKS, w, r)
}

func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.DeleteTaskHandler(&DBTask{}, w, r)
}

func runsHistoryView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(runsHistoryTemplate, w, r)
}

func AddHandlers(r *mux.Router) {
	r.HandleFunc("/", addTaskView).Methods("GET")
	r.HandleFunc("/"+api.CHROMIUM_PERF_URI, addTaskView).Methods("GET")
	r.HandleFunc("/"+api.CHROMIUM_PERF_RUNS_URI, runsHistoryView).Methods("GET")
	r.HandleFunc("/"+api.CHROMIUM_PERF_PARAMETERS_POST_URI, parametersHandler).Methods("POST")
	r.HandleFunc("/"+api.ADD_CHROMIUM_PERF_TASK_POST_URI, addTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.GET_CHROMIUM_PERF_TASKS_POST_URI, getTasksHandler).Methods("POST")
	r.HandleFunc("/"+api.UPDATE_CHROMIUM_PERF_TASK_POST_URI, updateTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.DELETE_CHROMIUM_PERF_TASK_POST_URI, deleteTaskHandler).Methods("POST")
}
