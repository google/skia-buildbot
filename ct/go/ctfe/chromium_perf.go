/*
	Handlers and types specific to Chromium perf tasks.
*/

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"

	"go.skia.org/infra/ct/go/db"
	api "go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/util"
	skutil "go.skia.org/infra/go/util"
)

var (
	chromiumPerfTemplate            *template.Template = nil
	chromiumPerfRunsHistoryTemplate *template.Template = nil
)

type ChromiumPerfDBTask struct {
	CommonCols

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

func (task ChromiumPerfDBTask) GetTaskName() string {
	return "ChromiumPerf"
}

func (task ChromiumPerfDBTask) TableName() string {
	return db.TABLE_CHROMIUM_PERF_TASKS
}

func (task ChromiumPerfDBTask) Select(query string, args ...interface{}) (interface{}, error) {
	result := []ChromiumPerfDBTask{}
	err := db.DB.Select(&result, query, args...)
	return result, err
}

func chromiumPerfView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(chromiumPerfTemplate, w, r)
}

func chromiumPerfHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	data := map[string]interface{}{
		"benchmarks": util.SupportedBenchmarks,
		"platforms":  util.SupportedPlatformsToDesc,
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

type AddChromiumPerfTaskVars struct {
	AddTaskCommonVars

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

func (task *AddChromiumPerfTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
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

func addChromiumPerfTaskHandler(w http.ResponseWriter, r *http.Request) {
	addTaskHandler(w, r, &AddChromiumPerfTaskVars{})
}

func getChromiumPerfTasksHandler(w http.ResponseWriter, r *http.Request) {
	getTasksHandler(&ChromiumPerfDBTask{}, w, r)
}

// Define api.ChromiumPerfUpdateVars in this package so we can add methods.
type ChromiumPerfUpdateVars struct {
	api.ChromiumPerfUpdateVars
}

func (task *ChromiumPerfUpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
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

func updateChromiumPerfTaskHandler(w http.ResponseWriter, r *http.Request) {
	updateTaskHandler(&ChromiumPerfUpdateVars{}, db.TABLE_CHROMIUM_PERF_TASKS, w, r)
}

func deleteChromiumPerfTaskHandler(w http.ResponseWriter, r *http.Request) {
	deleteTaskHandler(&ChromiumPerfDBTask{}, w, r)
}

func chromiumPerfRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(chromiumPerfRunsHistoryTemplate, w, r)
}
