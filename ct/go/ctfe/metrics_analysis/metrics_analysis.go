/*
	Handlers and types specific to Metrics analysis tasks.
*/

package metrics_analysis

import (
	"database/sql"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/gorilla/mux"

	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	"go.skia.org/infra/ct/go/db"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/httputils"
)

var (
	addTaskTemplate     *template.Template = nil
	runsHistoryTemplate *template.Template = nil

	httpClient = httputils.NewTimeoutClient()
)

func ReloadTemplates(resourcesDir string) {
	addTaskTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/metrics_analysis.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
	runsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/metrics_analysis_runs_history.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
}

type DBTask struct {
	task_common.CommonCols

	MetricName         string         `db:"metric_name"`
	CustomTraces       string         `db:"custom_traces"`
	AnalysisTaskId     string         `db:"analysis_task_id"`
	AnalysisOutputLink string         `db:"analysis_output_link"`
	BenchmarkArgs      string         `db:"benchmark_args"`
	Description        string         `db:"description"`
	ChromiumPatch      string         `db:"chromium_patch"`
	CatapultPatch      string         `db:"catapult_patch"`
	RawOutput          sql.NullString `db:"raw_output"`
}

func (task DBTask) GetTaskName() string {
	return "MetricsAnalysis"
}

func (dbTask DBTask) GetPopulatedAddTaskVars() task_common.AddTaskVars {
	taskVars := &AddTaskVars{}
	taskVars.Username = dbTask.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(dbTask.RepeatAfterDays, 10)
	taskVars.MetricName = dbTask.MetricName
	taskVars.CustomTraces = dbTask.CustomTraces
	taskVars.AnalysisTaskId = dbTask.AnalysisTaskId
	taskVars.AnalysisOutputLink = dbTask.AnalysisOutputLink
	taskVars.BenchmarkArgs = dbTask.BenchmarkArgs
	taskVars.Description = dbTask.Description
	taskVars.ChromiumPatch = dbTask.ChromiumPatch
	taskVars.CatapultPatch = dbTask.CatapultPatch
	return taskVars
}

func (task DBTask) GetResultsLink() string {
	if task.RawOutput.Valid {
		return task.RawOutput.String
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
	return db.TABLE_METRICS_ANALYSIS_TASKS
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

	MetricName         string `json:"metric_name"`
	CustomTraces       string `json:"custom_traces"`
	AnalysisTaskId     string `json:"analysis_task_id"`
	AnalysisOutputLink string `json:"analysis_output_link"`
	BenchmarkArgs      string `json:"benchmark_args"`
	Description        string `json:"desc"`
	ChromiumPatch      string `json:"chromium_patch"`
	CatapultPatch      string `json:"catapult_patch"`
}

func (task *AddTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.MetricName == "" {
		return "", nil, fmt.Errorf("Must specify metric name")
	}
	if task.CustomTraces == "" && task.AnalysisTaskId == "" {
		return "", nil, fmt.Errorf("Must specify one of custom traces or analysis task id")
	}
	if task.Description == "" {
		return "", nil, fmt.Errorf("Must specify description")
	}
	if err := ctfeutil.CheckLengths([]ctfeutil.LengthCheck{
		{Name: "metric_name", Value: task.MetricName, Limit: 255},
		{Name: "benchmark_args", Value: task.BenchmarkArgs, Limit: 255},
		{Name: "desc", Value: task.Description, Limit: 255},
		{Name: "custom_traces", Value: task.CustomTraces, Limit: db.LONG_TEXT_MAX_LENGTH},
		{Name: "chromium_patch", Value: task.ChromiumPatch, Limit: db.LONG_TEXT_MAX_LENGTH},
		{Name: "catapult_patch", Value: task.CatapultPatch, Limit: db.LONG_TEXT_MAX_LENGTH},
	}); err != nil {
		return "", nil, err
	}
	if task.AnalysisTaskId != "" {
		// Get analysis output link from analysis task id.
		outputLinks := []string{}
		query := fmt.Sprintf("SELECT raw_output FROM %s WHERE id = ?", db.TABLE_CHROMIUM_ANALYSIS_TASKS)
		if err := db.DB.Select(&outputLinks, query, task.AnalysisTaskId); err != nil || len(outputLinks) < 1 {
			return "", nil, fmt.Errorf("Unable to validate analysis task id parameter %v", task.AnalysisTaskId)
		}
		if len(outputLinks) != 1 {
			return "", nil, fmt.Errorf("Unable to find requested analysis task id.")
		}
		task.AnalysisOutputLink = outputLinks[0]
	}

	return fmt.Sprintf("INSERT INTO %s (username,metric_name,custom_traces,analysis_task_id,analysis_output_link,benchmark_args,description,chromium_patch,catapult_patch,ts_added,repeat_after_days) VALUES (?,?,?,?,?,?,?,?,?,?,?);",
			db.TABLE_METRICS_ANALYSIS_TASKS),
		[]interface{}{
			task.Username,
			task.MetricName,
			task.CustomTraces,
			task.AnalysisTaskId,
			task.AnalysisOutputLink,
			task.BenchmarkArgs,
			task.Description,
			task.ChromiumPatch,
			task.CatapultPatch,
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

	RawOutput sql.NullString
}

func (vars *UpdateVars) UriPath() string {
	return ctfeutil.UPDATE_METRICS_ANALYSIS_TASK_POST_URI
}

func (task *UpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
	if err := ctfeutil.CheckLengths([]ctfeutil.LengthCheck{
		{Name: "RawOutput", Value: task.RawOutput.String, Limit: 255},
	}); err != nil {
		return nil, nil, err
	}
	clauses := []string{}
	args := []interface{}{}
	if task.RawOutput.Valid {
		clauses = append(clauses, "raw_output = ?")
		args = append(args, task.RawOutput.String)
	}
	return clauses, args, nil
}

func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.UpdateTaskHandler(&UpdateVars{}, db.TABLE_METRICS_ANALYSIS_TASKS, w, r)
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
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.METRICS_ANALYSIS_URI, "GET", addTaskView)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.METRICS_ANALYSIS_RUNS_URI, "GET", runsHistoryView)

	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.ADD_METRICS_ANALYSIS_TASK_POST_URI, "POST", addTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.GET_METRICS_ANALYSIS_TASKS_POST_URI, "POST", getTasksHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.DELETE_METRICS_ANALYSIS_TASK_POST_URI, "POST", deleteTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.REDO_METRICS_ANALYSIS_TASK_POST_URI, "POST", redoTaskHandler)

	// Do not add force login handler for update methods. They use webhooks for authentication.
	r.HandleFunc("/"+ctfeutil.UPDATE_METRICS_ANALYSIS_TASK_POST_URI, updateTaskHandler).Methods("POST")
}
