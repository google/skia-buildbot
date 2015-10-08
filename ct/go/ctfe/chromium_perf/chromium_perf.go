/*
	Handlers and types specific to Chromium perf tasks.
*/

package chromium_perf

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"

	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	"go.skia.org/infra/ct/go/db"
	ctutil "go.skia.org/infra/ct/go/util"
	skutil "go.skia.org/infra/go/util"
	"go.skia.org/infra/go/webhook"
)

var (
	addTaskTemplate     *template.Template = nil
	runsHistoryTemplate *template.Template = nil

	httpClient = skutil.NewTimeoutClient()
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
	RunInParallel        bool           `db:"run_in_parallel"`
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

func (dbTask DBTask) GetPopulatedAddTaskVars() task_common.AddTaskVars {
	taskVars := &AddTaskVars{}
	taskVars.Username = dbTask.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(dbTask.RepeatAfterDays, 10)
	taskVars.Benchmark = dbTask.Benchmark
	taskVars.Platform = dbTask.Platform
	taskVars.PageSets = dbTask.PageSets
	taskVars.RepeatRuns = strconv.FormatInt(dbTask.RepeatRuns, 10)
	taskVars.RunInParallel = strconv.FormatBool(dbTask.RunInParallel)
	taskVars.BenchmarkArgs = dbTask.BenchmarkArgs
	taskVars.BrowserArgsNoPatch = dbTask.BrowserArgsNoPatch
	taskVars.BrowserArgsWithPatch = dbTask.BrowserArgsWithPatch
	taskVars.Description = dbTask.Description
	taskVars.ChromiumPatch = dbTask.ChromiumPatch
	taskVars.BlinkPatch = dbTask.BlinkPatch
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

var clURLRegexp = regexp.MustCompile("^(?:https?://codereview\\.chromium\\.org/)?(\\d{3,})/?$")

type clDetail struct {
	Issue     int    `json:"issue"`
	Subject   string `json:"subject"`
	Modified  string `json:"modified"`
	Project   string `json:"project"`
	Patchsets []int  `json:"patchsets"`
}

func getCLDetail(clURLString string) (clDetail, error) {
	if clURLString == "" {
		return clDetail{}, fmt.Errorf("No CL specified")
	}

	matches := clURLRegexp.FindStringSubmatch(clURLString)
	if len(matches) < 2 || matches[1] == "" {
		// Don't return error, since user could still be typing.
		return clDetail{}, nil
	}
	clString := matches[1]
	detailJsonUrl := "https://codereview.chromium.org/api/" + clString
	glog.Infof("Reading CL detail from %s", detailJsonUrl)
	detailResp, err := httpClient.Get(detailJsonUrl)
	if err != nil {
		return clDetail{}, fmt.Errorf("Unable to retrieve CL detail: %v", err)
	}
	defer skutil.Close(detailResp.Body)
	if detailResp.StatusCode == 404 {
		// Don't return error, since user could still be typing.
		return clDetail{}, nil
	}
	if detailResp.StatusCode != 200 {
		return clDetail{}, fmt.Errorf("Unable to retrieve CL detail; status code %d", detailResp.StatusCode)
	}
	detail := clDetail{}
	err = json.NewDecoder(detailResp.Body).Decode(&detail)
	return detail, err
}

func getCLPatch(detail clDetail, patchsetID int) (string, error) {
	if len(detail.Patchsets) == 0 {
		return "", fmt.Errorf("CL has no patchsets")
	}
	if patchsetID <= 0 {
		// If no valid patchsetID has been specified then use the last patchset.
		patchsetID = detail.Patchsets[len(detail.Patchsets)-1]
	}
	patchUrl := fmt.Sprintf("https://codereview.chromium.org/download/issue%d_%d.diff", detail.Issue, patchsetID)
	glog.Infof("Downloading CL patch from %s", patchUrl)
	patchResp, err := httpClient.Get(patchUrl)
	if err != nil {
		return "", fmt.Errorf("Unable to retrieve CL patch: %v", err)
	}
	defer skutil.Close(patchResp.Body)
	if patchResp.StatusCode != 200 {
		return "", fmt.Errorf("Unable to retrieve CL patch; status code %d", patchResp.StatusCode)
	}
	if patchResp.ContentLength > db.TEXT_MAX_LENGTH {
		return "", fmt.Errorf("Patch is too large; length is %d bytes.", patchResp.ContentLength)
	}
	patchBytes, err := ioutil.ReadAll(patchResp.Body)
	if err != nil {
		return "", fmt.Errorf("Unable to retrieve CL patch: %v", err)
	}
	// Double-check length in case ContentLength was -1.
	if len(patchBytes) > db.TEXT_MAX_LENGTH {
		return "", fmt.Errorf("Patch is too large; length is %d bytes.", len(patchBytes))
	}
	return string(patchBytes), nil
}

func gatherCLData(detail clDetail, patch string) (map[string]string, error) {
	clData := map[string]string{}
	clData["cl"] = strconv.Itoa(detail.Issue)
	clData["patchset"] = strconv.Itoa(detail.Patchsets[len(detail.Patchsets)-1])
	clData["subject"] = detail.Subject
	modifiedTime, err := time.Parse("2006-01-02 15:04:05.999999", detail.Modified)
	if err != nil {
		glog.Errorf("Unable to parse modified time for CL %d; input '%s', got %v", detail.Issue, detail.Modified, err)
		clData["modified"] = ""
	} else {
		clData["modified"] = modifiedTime.UTC().Format(ctutil.TS_FORMAT)
	}
	clData["chromium_patch"] = ""
	clData["skia_patch"] = ""
	switch detail.Project {
	case "chromium":
		clData["chromium_patch"] = patch
	case "skia":
		clData["skia_patch"] = patch
	default:
		return nil, fmt.Errorf("CL project is %s; only chromium and skia are supported.", detail.Project)
	}
	return clData, nil
}

func getCLHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	detail, err := getCLDetail(r.FormValue("cl"))
	if err != nil {
		skutil.ReportError(w, r, err, "")
		return
	}
	if detail.Issue == 0 {
		// Return successful empty response, since the user could still be typing.
		if err := json.NewEncoder(w).Encode(map[string]interface{}{}); err != nil {
			skutil.ReportError(w, r, err, "Failed to encode JSON")
		}
		return
	}
	patch, err := getCLPatch(detail, 0)
	if err != nil {
		skutil.ReportError(w, r, err, "")
		return
	}
	clData, err := gatherCLData(detail, patch)
	if err != nil {
		skutil.ReportError(w, r, err, "")
		return
	}
	if err = json.NewEncoder(w).Encode(clData); err != nil {
		skutil.ReportError(w, r, err, "")
		return
	}
}

type AddTaskVars struct {
	task_common.AddTaskCommonVars

	Benchmark            string `json:"benchmark"`
	Platform             string `json:"platform"`
	PageSets             string `json:"page_sets"`
	RepeatRuns           string `json:"repeat_runs"`
	RunInParallel        string `json:"run_in_parallel"`
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
		task.RunInParallel == "" ||
		task.Description == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	if err := ctfeutil.CheckLengths([]ctfeutil.LengthCheck{
		{"benchmark", task.Benchmark, 100},
		{"platform", task.Platform, 100},
		{"page_sets", task.PageSets, 100},
		{"benchmark_args", task.BenchmarkArgs, 255},
		{"browser_args_nopatch", task.BrowserArgsNoPatch, 255},
		{"browser_args_withpatch", task.BrowserArgsWithPatch, 255},
		{"desc", task.Description, 255},
		{"chromium_patch", task.ChromiumPatch, db.TEXT_MAX_LENGTH},
		{"blink_patch", task.BlinkPatch, db.TEXT_MAX_LENGTH},
		{"skia_patch", task.SkiaPatch, db.TEXT_MAX_LENGTH},
	}); err != nil {
		return "", nil, err
	}
	runInParallel := 0
	if task.RunInParallel == "True" {
		runInParallel = 1
	}
	return fmt.Sprintf("INSERT INTO %s (username,benchmark,platform,page_sets,repeat_runs,run_in_parallel, benchmark_args,browser_args_nopatch,browser_args_withpatch,description,chromium_patch,blink_patch,skia_patch,ts_added,repeat_after_days) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);",
			db.TABLE_CHROMIUM_PERF_TASKS),
		[]interface{}{
			task.Username,
			task.Benchmark,
			task.Platform,
			task.PageSets,
			task.RepeatRuns,
			runInParallel,
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

type UpdateVars struct {
	task_common.UpdateTaskCommonVars

	Results            sql.NullString
	NoPatchRawOutput   sql.NullString
	WithPatchRawOutput sql.NullString
}

func (vars *UpdateVars) UriPath() string {
	return ctfeutil.UPDATE_CHROMIUM_PERF_TASK_POST_URI
}

func (task *UpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
	if err := ctfeutil.CheckLengths([]ctfeutil.LengthCheck{
		{"NoPatchRawOutput", task.NoPatchRawOutput.String, 255},
		{"WithPatchRawOutput", task.WithPatchRawOutput.String, 255},
		{"Results", task.Results.String, 255},
	}); err != nil {
		return nil, nil, err
	}
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

type TrybotTask struct {
	Issue      string      `json:"issue"`
	PatchsetID string      `json:"patchset"`
	TaskVars   AddTaskVars `json:"task"`
}

func addTrybotTaskHandler(w http.ResponseWriter, r *http.Request) {
	data, err := webhook.AuthenticateRequest(r)
	if err != nil {
		if data == nil {
			skutil.ReportError(w, r, err, "Failed to read add request")
			return
		}
		if !ctfeutil.UserHasAdminRights(r) {
			skutil.ReportError(w, r, err, "Failed authentication")
			return
		}
	}

	trybotTask := TrybotTask{}
	if err := json.Unmarshal(data, &trybotTask); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to add %v trybot task", trybotTask))
		return
	}

	task := &trybotTask.TaskVars
	// Add patch data to the task.
	detail, err := getCLDetail(trybotTask.Issue)
	if err != nil {
		skutil.ReportError(w, r, err, "")
		return
	}
	patchsetID, err := strconv.Atoi(trybotTask.PatchsetID)
	if err != nil {
		skutil.ReportError(w, r, err, "")
		return
	}
	patch, err := getCLPatch(detail, patchsetID)
	if err != nil {
		skutil.ReportError(w, r, err, "")
		return
	}
	clData, err := gatherCLData(detail, patch)
	if err != nil {
		skutil.ReportError(w, r, err, "")
		return
	}

	task.Description = fmt.Sprintf("Trybot run for http://codereview.chromium.org/%s#ps%s", clData["cl"], clData["patchset"])
	if val, ok := clData["chromium_patch"]; ok {
		task.ChromiumPatch = val
	}
	if val, ok := clData["skia_patch"]; ok {
		task.SkiaPatch = val
	}

	task.GetAddTaskCommonVars().TsAdded = ctutil.GetCurrentTs()

	taskID, err := task_common.AddTask(task)
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to insert %T task", task))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	jsonResponse := map[string]interface{}{
		"taskID": taskID,
	}
	if err := json.NewEncoder(w).Encode(jsonResponse); err != nil {
		skutil.ReportError(w, r, err, "Failed to encode JSON")
		return
	}
}

func getTaskStatusHandler(w http.ResponseWriter, r *http.Request) {
	task_common.GetTaskStatusHandler(&DBTask{}, w, r)
}

func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.UpdateTaskHandler(&UpdateVars{}, db.TABLE_CHROMIUM_PERF_TASKS, w, r)
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
	r.HandleFunc("/", addTaskView).Methods("GET")
	r.HandleFunc("/"+ctfeutil.CHROMIUM_PERF_URI, addTaskView).Methods("GET")
	r.HandleFunc("/"+ctfeutil.CHROMIUM_PERF_RUNS_URI, runsHistoryView).Methods("GET")
	r.HandleFunc("/"+ctfeutil.GET_CHROMIUM_PERF_RUN_STATUS_URI, getTaskStatusHandler).Methods("GET")
	r.HandleFunc("/"+ctfeutil.CHROMIUM_PERF_PARAMETERS_POST_URI, parametersHandler).Methods("POST")
	r.HandleFunc("/"+ctfeutil.CHROMIUM_PERF_CL_DATA_POST_URI, getCLHandler).Methods("POST")
	r.HandleFunc("/"+ctfeutil.ADD_CHROMIUM_PERF_TASK_POST_URI, addTaskHandler).Methods("POST")
	r.HandleFunc("/"+ctfeutil.GET_CHROMIUM_PERF_TASKS_POST_URI, getTasksHandler).Methods("POST")
	r.HandleFunc("/"+ctfeutil.UPDATE_CHROMIUM_PERF_TASK_POST_URI, updateTaskHandler).Methods("POST")
	r.HandleFunc("/"+ctfeutil.WEBHOOK_ADD_CHROMIUM_PERF_TASK_POST_URI, addTrybotTaskHandler).Methods("POST")
	r.HandleFunc("/"+ctfeutil.DELETE_CHROMIUM_PERF_TASK_POST_URI, deleteTaskHandler).Methods("POST")
	r.HandleFunc("/"+ctfeutil.REDO_CHROMIUM_PERF_TASK_POST_URI, redoTaskHandler).Methods("POST")
}
