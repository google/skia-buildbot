/*
	The Cluster Telemetry Frontend.
*/

package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"

	"go.skia.org/infra/ct/go/db"
	api "go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	skutil "go.skia.org/infra/go/util"
)

const (
	// Default page size used for pagination.
	DEFAULT_PAGE_SIZE = 5

	// Maximum page size used for pagination.
	MAX_PAGE_SIZE = 100

	// URL returning the GIT commit hash of the last known good release of Chromium.
	CHROMIUM_LKGR_URL = "http://chromium-status.appspot.com/git-lkgr"
	// Base URL of the Chromium GIT repository, to be followed by commit hash.
	CHROMIUM_GIT_REPO_URL = "https://chromium.googlesource.com/chromium/src.git/+/"
	// URL of a base64-encoded file that includes the GIT commit hash last known good release of Skia.
	CHROMIUM_DEPS_FILE = "https://chromium.googlesource.com/chromium/src/+/master/DEPS?format=TEXT"
	// Base URL of the Skia GIT repository, to be followed by commit hash.
	SKIA_GIT_REPO_URL = "https://skia.googlesource.com/skia/+/"
)

var (
	taskTables = []string{
		db.TABLE_CHROMIUM_PERF_TASKS,
		db.TABLE_CAPTURE_SKPS_TASKS,
		db.TABLE_LUA_SCRIPT_TASKS,
		db.TABLE_CHROMIUM_BUILD_TASKS,
		db.TABLE_RECREATE_PAGE_SETS_TASKS,
		db.TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS,
	}

	chromiumPerfTemplate                       *template.Template = nil
	chromiumPerfRunsHistoryTemplate            *template.Template = nil
	captureSkpsTemplate                        *template.Template = nil
	captureSkpRunsHistoryTemplate              *template.Template = nil
	luaScriptsTemplate                         *template.Template = nil
	luaScriptRunsHistoryTemplate               *template.Template = nil
	chromiumBuildsTemplate                     *template.Template = nil
	chromiumBuildRunsHistoryTemplate           *template.Template = nil
	adminTasksTemplate                         *template.Template = nil
	recreatePageSetsRunsHistoryTemplate        *template.Template = nil
	recreateWebpageArchivesRunsHistoryTemplate *template.Template = nil
	runsHistoryTemplate                        *template.Template = nil
	pendingTasksTemplate                       *template.Template = nil

	dbClient   *influxdb.Client = nil
	httpClient                  = skutil.NewTimeoutClient()
)

// flags
var (
	graphiteServer = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	host           = flag.String("host", "localhost", "HTTP service host")
	port           = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	workdir        = flag.String("workdir", ".", "Directory to use for scratch work.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

type CommonCols struct {
	Id              int64         `db:"id"`
	TsAdded         sql.NullInt64 `db:"ts_added"`
	TsStarted       sql.NullInt64 `db:"ts_started"`
	TsCompleted     sql.NullInt64 `db:"ts_completed"`
	Username        string        `db:"username"`
	Failure         sql.NullBool  `db:"failure"`
	RepeatAfterDays sql.NullInt64 `db:"repeat_after_days"`
}

type Task interface {
	GetAddedTimestamp() int64
	GetTaskName() string
	TableName() string
	// Returns a slice of the struct type.
	Select(query string, args ...interface{}) (interface{}, error)
}

func (dbrow *CommonCols) GetAddedTimestamp() int64 {
	return dbrow.TsAdded.Int64
}

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

type CaptureSkpsDBTask struct {
	CommonCols

	PageSets    string `db:"page_sets"`
	ChromiumRev string `db:"chromium_rev"`
	SkiaRev     string `db:"skia_rev"`
	Description string `db:"description"`
}

func (task CaptureSkpsDBTask) GetTaskName() string {
	return "CaptureSkps"
}

func (task CaptureSkpsDBTask) TableName() string {
	return db.TABLE_CAPTURE_SKPS_TASKS
}

func (task CaptureSkpsDBTask) Select(query string, args ...interface{}) (interface{}, error) {
	result := []CaptureSkpsDBTask{}
	err := db.DB.Select(&result, query, args...)
	return result, err
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

type ChromiumBuildDBTask struct {
	CommonCols

	ChromiumRev   string        `db:"chromium_rev"`
	ChromiumRevTs sql.NullInt64 `db:"chromium_rev_ts"`
	SkiaRev       string        `db:"skia_rev"`
}

func (task ChromiumBuildDBTask) GetTaskName() string {
	return "ChromiumBuild"
}

func (task ChromiumBuildDBTask) TableName() string {
	return db.TABLE_CHROMIUM_BUILD_TASKS
}

func (task ChromiumBuildDBTask) Select(query string, args ...interface{}) (interface{}, error) {
	result := []ChromiumBuildDBTask{}
	err := db.DB.Select(&result, query, args...)
	return result, err
}

type RecreatePageSetsDBTask struct {
	CommonCols

	PageSets string `db:"page_sets"`
}

func (task RecreatePageSetsDBTask) GetTaskName() string {
	return "RecreatePageSets"
}

func (task RecreatePageSetsDBTask) TableName() string {
	return db.TABLE_RECREATE_PAGE_SETS_TASKS
}

func (task RecreatePageSetsDBTask) Select(query string, args ...interface{}) (interface{}, error) {
	result := []RecreatePageSetsDBTask{}
	err := db.DB.Select(&result, query, args...)
	return result, err
}

type RecreateWebpageArchivesDBTask struct {
	CommonCols

	PageSets    string `db:"page_sets"`
	ChromiumRev string `db:"chromium_rev"`
	SkiaRev     string `db:"skia_rev"`
}

func (task RecreateWebpageArchivesDBTask) GetTaskName() string {
	return "RecreateWebpageArchives"
}

func (task RecreateWebpageArchivesDBTask) TableName() string {
	return db.TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS
}

func (task RecreateWebpageArchivesDBTask) Select(query string, args ...interface{}) (interface{}, error) {
	result := []RecreateWebpageArchivesDBTask{}
	err := db.DB.Select(&result, query, args...)
	return result, err
}

func reloadTemplates() {
	if *resourcesDir == "" {
		// If resourcesDir is not specified then consider the directory two directories up from this
		// source file as the resourcesDir.
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	chromiumPerfTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/chromium_perf.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))
	chromiumPerfRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/chromium_perf_runs_history.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))

	captureSkpsTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/capture_skps.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))
	captureSkpRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/capture_skp_runs_history.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))

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

	chromiumBuildsTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/chromium_builds.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))
	chromiumBuildRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/chromium_build_runs_history.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))

	adminTasksTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/admin_tasks.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))
	recreatePageSetsRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/recreate_page_sets_runs_history.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))
	recreateWebpageArchivesRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/recreate_webpage_archives_runs_history.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))

	runsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/runs_history.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))

	pendingTasksTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/pending_tasks.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))
}

func Init() {
	reloadTemplates()
}

func userHasEditRights(r *http.Request) bool {
	return strings.HasSuffix(login.LoggedInAs(r), "@google.com") || strings.HasSuffix(login.LoggedInAs(r), "@chromium.org")
}

func userHasAdminRights(r *http.Request) bool {
	// TODO(benjaminwagner): Add this list to GCE project level metadata and retrieve from there.
	admins := map[string]bool{
		"benjaminwagner@google.com": true,
		"borenet@google.com":        true,
		"jcgregorio@google.com":     true,
		"rmistry@google.com":        true,
		"stephana@google.com":       true,
	}
	return userHasEditRights(r) && admins[login.LoggedInAs(r)]
}

func getIntParam(name string, r *http.Request) (*int, error) {
	raw, ok := r.URL.Query()[name]
	if !ok {
		return nil, nil
	}
	v64, err := strconv.ParseInt(raw[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("Invalid value for parameter %q: %s -- %v", name, raw, err)
	}
	v32 := int(v64)
	return &v32, nil
}

func executeSimpleTemplate(template *template.Template, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in local mode.
	if *local {
		reloadTemplates()
	}

	if err := template.Execute(w, struct{}{}); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
		return
	}
}

// Data included in all tasks; set by addTaskHandler.
type AddTaskCommonVars struct {
	Username        string
	TsAdded         string
	RepeatAfterDays string `json:"repeat_after_days"`
}

type AddTaskVars interface {
	GetAddTaskCommonVars() *AddTaskCommonVars
	IsAdminTask() bool
	GetInsertQueryAndBinds() (string, []interface{}, error)
}

func (vars *AddTaskCommonVars) GetAddTaskCommonVars() *AddTaskCommonVars {
	return vars
}

func (vars *AddTaskCommonVars) IsAdminTask() bool {
	return false
}

func addTaskHandler(w http.ResponseWriter, r *http.Request, task AddTaskVars) {
	if !userHasEditRights(r) {
		skutil.ReportError(w, r, fmt.Errorf("Must have google or chromium account to add tasks"), "")
		return
	}
	if task.IsAdminTask() && !userHasAdminRights(r) {
		skutil.ReportError(w, r, fmt.Errorf("Must be admin to add admin tasks; contact rmistry@"), "")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to add %T task: %v", task, err))
		return
	}
	defer skutil.Close(r.Body)

	task.GetAddTaskCommonVars().Username = login.LoggedInAs(r)
	task.GetAddTaskCommonVars().TsAdded = api.GetCurrentTs()

	query, binds, err := task.GetInsertQueryAndBinds()
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to marshal %T task: %v", task, err))
		return
	}
	if _, err = db.DB.Exec(query, binds...); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to insert %T task: %v", task, err))
		return
	}
}

// Returns true if the string is non-empty, unless strconv.ParseBool parses the string as false.
func parseBoolFormValue(string string) bool {
	if string == "" {
		return false
	} else if val, err := strconv.ParseBool(string); val == false && err == nil {
		return false
	} else {
		return true
	}
}

func dbTaskQuery(prototype Task, username string, successful bool, includeCompleted bool, includeFutureRuns bool, countQuery bool, offset int, size int) (string, []interface{}) {
	args := []interface{}{}
	query := "SELECT "
	if countQuery {
		query += "COUNT(*)"
	} else {
		query += "*"
	}
	query += fmt.Sprintf(" FROM %s", prototype.TableName())
	clauses := []string{}
	if username != "" {
		clauses = append(clauses, "username=?")
		args = append(args, username)
	}
	if successful {
		clauses = append(clauses, "(ts_completed IS NOT NULL AND failure = 0)")
	}
	if !includeCompleted {
		clauses = append(clauses, "ts_completed IS NULL")
	}
	if includeFutureRuns {
		clauses = append(clauses, "(repeat_after_days != 0 AND ts_completed IS NOT NULL)")
	}
	if len(clauses) > 0 {
		query += " WHERE "
		query += strings.Join(clauses, " AND ")
	}
	if !countQuery {
		query += " ORDER BY id DESC LIMIT ?,?"
		args = append(args, offset, size)
	}
	return query, args
}

func getTasksHandler(prototype Task, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Filter by either username or not started yet.
	username := r.FormValue("username")
	successful := parseBoolFormValue(r.FormValue("successful"))
	includeCompleted := !parseBoolFormValue(r.FormValue("not_completed"))
	includeFutureRuns := parseBoolFormValue(r.FormValue("include_future_runs"))
	if successful && !includeCompleted {
		skutil.ReportError(w, r, fmt.Errorf("Inconsistent params: successful %v not_completed %v", r.FormValue("successful"), r.FormValue("not_completed")), "")
		return
	}
	offset, size, err := skutil.PaginationParams(r.URL.Query(), 0, DEFAULT_PAGE_SIZE, MAX_PAGE_SIZE)
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to get pagination params: %v", err))
		return
	}
	query, args := dbTaskQuery(prototype, username, successful, includeCompleted, includeFutureRuns, false, offset, size)
	glog.Infof("Running %s", query)
	data, err := prototype.Select(query, args...)
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to query %s tasks: %v", prototype.GetTaskName(), err))
		return
	}

	query, args = dbTaskQuery(prototype, username, successful, includeCompleted, includeFutureRuns, true, 0, 0)
	// Get the total count.
	glog.Infof("Running %s", query)
	countVal := []int{}
	if err := db.DB.Select(&countVal, query, args...); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to query %s tasks: %v", prototype.GetTaskName(), err))
		return
	}

	pagination := &skutil.ResponsePagination{
		Offset: offset,
		Size:   size,
		Total:  countVal[0],
	}
	jsonResponse := map[string]interface{}{
		"data":       data,
		"pagination": pagination,
	}
	if err := json.NewEncoder(w).Encode(jsonResponse); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

// Add methods for working with the database to api.UpdateTaskVars.
type UpdateTaskVars interface {
	api.UpdateTaskVars
	// Produces SQL query clauses and binds for fields not in api.UpdateTaskCommonVars. First return
	// value is a slice of strings like "results = ?". Second return value contains a value for
	// each "?" bind.
	GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error)
}

func getUpdateQueryAndBinds(vars UpdateTaskVars, tableName string) (string, []interface{}, error) {
	common := vars.GetUpdateTaskCommonVars()
	query := fmt.Sprintf("UPDATE %s SET ", tableName)
	clauses := []string{}
	args := []interface{}{}
	if common.TsStarted.Valid {
		clauses = append(clauses, "ts_started = ?")
		args = append(args, common.TsStarted.String)
	}
	if common.TsCompleted.Valid {
		clauses = append(clauses, "ts_completed = ?")
		args = append(args, common.TsCompleted.String)
	}
	if common.Failure.Valid {
		clauses = append(clauses, "failure = ?")
		args = append(args, common.Failure.Bool)
	}
	additionalClauses, additionalArgs, err := vars.GetUpdateExtraClausesAndBinds()
	if err != nil {
		return "", nil, err
	}
	clauses = append(clauses, additionalClauses...)
	args = append(args, additionalArgs...)
	if len(clauses) == 0 {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	query += strings.Join(clauses, ", ")
	query += " WHERE id = ?"
	args = append(args, common.Id)
	return query, args, nil
}

func updateTaskHandler(vars UpdateTaskVars, tableName string, w http.ResponseWriter, r *http.Request) {
	// TODO(benjaminwagner): authenticate
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewDecoder(r.Body).Decode(&vars); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to parse %T update: %v", vars, err))
		return
	}
	defer skutil.Close(r.Body)

	query, binds, err := getUpdateQueryAndBinds(vars, tableName)
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to marshal %T update: %v", vars, err))
		return
	}
	_, err = db.DB.Exec(query, binds...)
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to update using %T: %v", vars, err))
		return
	}
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

func pageSetsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	pageSets := []map[string]string{}
	for pageSet := range util.PagesetTypeToInfo {
		pageSetObj := map[string]string{
			"key":         pageSet,
			"description": util.PagesetTypeToInfo[pageSet].Description,
		}
		pageSets = append(pageSets, pageSetObj)
	}

	if err := json.NewEncoder(w).Encode(pageSets); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

// ChromiumPerfVars is the type used by the Chromium Perf pages.
type ChromiumPerfVars struct {
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

func (task *ChromiumPerfVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
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
	addTaskHandler(w, r, &ChromiumPerfVars{})
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

func chromiumPerfRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(chromiumPerfRunsHistoryTemplate, w, r)
}

func captureSkpsView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(captureSkpsTemplate, w, r)
}

type AddCaptureSkpsTaskVars struct {
	AddTaskCommonVars

	PageSets      string              `json:"page_sets"`
	ChromiumBuild ChromiumBuildDBTask `json:"chromium_build"`
	Description   string              `json:"desc"`
}

func (task *AddCaptureSkpsTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.PageSets == "" ||
		task.ChromiumBuild.ChromiumRev == "" ||
		task.ChromiumBuild.SkiaRev == "" ||
		task.Description == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	if err := validateChromiumBuild(task.ChromiumBuild); err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("INSERT INTO %s (username,page_sets,chromium_rev,skia_rev,description,ts_added, repeat_after_days) VALUES (?,?,?,?,?,?,?);",
			db.TABLE_CAPTURE_SKPS_TASKS),
		[]interface{}{
			task.Username,
			task.PageSets,
			task.ChromiumBuild.ChromiumRev,
			task.ChromiumBuild.SkiaRev,
			task.Description,
			task.TsAdded,
			task.RepeatAfterDays,
		},
		nil
}

func addCaptureSkpsTaskHandler(w http.ResponseWriter, r *http.Request) {
	addTaskHandler(w, r, &AddCaptureSkpsTaskVars{})
}

func getCaptureSkpTasksHandler(w http.ResponseWriter, r *http.Request) {
	getTasksHandler(&CaptureSkpsDBTask{}, w, r)
}

// Validate that the given skpRepository exists in the DB.
func validateSkpRepository(skpRepository CaptureSkpsDBTask) error {
	rowCount := []int{}
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE page_sets = ? AND chromium_rev = ? AND skia_rev = ? AND ts_completed IS NOT NULL AND failure = 0", db.TABLE_CAPTURE_SKPS_TASKS)
	if err := db.DB.Select(&rowCount, query, skpRepository.PageSets, skpRepository.ChromiumRev, skpRepository.SkiaRev); err != nil || len(rowCount) < 1 || rowCount[0] == 0 {
		glog.Info(err)
		return fmt.Errorf("Unable to validate skp_repository parameter %v", skpRepository)
	}
	return nil
}

// Define api.CaptureSkpsUpdateVars in this package so we can add methods.
type CaptureSkpsUpdateVars struct {
	api.CaptureSkpsUpdateVars
}

func (task *CaptureSkpsUpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
	return nil, nil, nil
}

func updateCaptureSkpsTaskHandler(w http.ResponseWriter, r *http.Request) {
	updateTaskHandler(&CaptureSkpsUpdateVars{}, db.TABLE_CAPTURE_SKPS_TASKS, w, r)
}

func captureSkpRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(captureSkpRunsHistoryTemplate, w, r)
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

func luaScriptRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(luaScriptRunsHistoryTemplate, w, r)
}

func chromiumBuildsView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(chromiumBuildsTemplate, w, r)
}

type AddChromiumBuildTaskVars struct {
	AddTaskCommonVars

	ChromiumRev   string `json:"chromium_rev"`
	ChromiumRevTs string `json:"chromium_rev_ts"`
	SkiaRev       string `json:"skia_rev"`
}

func (task *AddChromiumBuildTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.ChromiumRev == "" ||
		task.SkiaRev == "" ||
		task.ChromiumRevTs == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	// Example timestamp format: "Wed Jul 15 13:42:19 2015"
	parsedTs, err := time.Parse(time.ANSIC, task.ChromiumRevTs)
	if err != nil {
		return "", nil, err
	}
	chromiumRevTs := parsedTs.UTC().Format("20060102150405")
	return fmt.Sprintf("INSERT INTO %s (username,chromium_rev,chromium_rev_ts,skia_rev,ts_added,repeat_after_days) VALUES (?,?,?,?,?,?);",
			db.TABLE_CHROMIUM_BUILD_TASKS),
		[]interface{}{
			task.Username,
			task.ChromiumRev,
			chromiumRevTs,
			task.SkiaRev,
			task.TsAdded,
			task.RepeatAfterDays,
		},
		nil
}

func addChromiumBuildTaskHandler(w http.ResponseWriter, r *http.Request) {
	addTaskHandler(w, r, &AddChromiumBuildTaskVars{})
}

func getRevDataHandler(getLkgr func() (string, error), gitRepoUrl string, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	revString := r.FormValue("rev")
	if revString == "" {
		skutil.ReportError(w, r, fmt.Errorf("No revision specified"), "")
		return
	}

	if strings.EqualFold(revString, "LKGR") {
		lkgr, err := getLkgr()
		if err != nil {
			skutil.ReportError(w, r, fmt.Errorf("Unable to retrieve LKGR revision"), "")
		}
		glog.Infof("Retrieved LKGR commit hash: %s", lkgr)
		revString = lkgr
	}
	commitUrl := gitRepoUrl + revString + "?format=JSON"
	glog.Infof("Reading revision detail from %s", commitUrl)
	resp, err := httpClient.Get(commitUrl)
	if err != nil {
		skutil.ReportError(w, r, err, "Unable to retrieve revision detail")
		return
	}
	defer skutil.Close(resp.Body)
	if resp.StatusCode == 404 {
		// Return successful empty response, since the user could be typing.
		if err := json.NewEncoder(w).Encode(map[string]interface{}{}); err != nil {
			skutil.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
			return
		}
		return
	}
	if resp.StatusCode != 200 {
		skutil.ReportError(w, r, fmt.Errorf("Unable to retrieve revision detail"), "")
		return
	}
	// Remove junk in the first line. https://code.google.com/p/gitiles/issues/detail?id=31
	bufBody := bufio.NewReader(resp.Body)
	if _, err = bufBody.ReadString('\n'); err != nil {
		skutil.ReportError(w, r, err, "Unable to retrieve revision detail")
		return
	}
	if _, err = io.Copy(w, bufBody); err != nil {
		skutil.ReportError(w, r, err, "Unable to retrieve revision detail")
		return
	}
}

func getChromiumLkgr() (string, error) {
	glog.Infof("Reading Chromium LKGR from %s", CHROMIUM_LKGR_URL)
	resp, err := httpClient.Get(CHROMIUM_LKGR_URL)
	if err != nil {
		return "", err
	}
	defer skutil.Close(resp.Body)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return bytes.NewBuffer(body).String(), nil
}

func getChromiumRevDataHandler(w http.ResponseWriter, r *http.Request) {
	getRevDataHandler(getChromiumLkgr, CHROMIUM_GIT_REPO_URL, w, r)
}

var skiaRevisionRegexp = regexp.MustCompile("'skia_revision': '([0-9a-fA-F]{2,40})'")

// Copied from https://github.com/google/skia-buildbot/blob/016cce36f0cd487c9586b013979705e49dd76f8e/appengine_scripts/skia-tree-status/status.py#L178
// to work around 403 error.
func getSkiaLkgr() (string, error) {
	glog.Infof("Reading Skia LKGR from %s", CHROMIUM_DEPS_FILE)
	resp, err := httpClient.Get(CHROMIUM_DEPS_FILE)
	if err != nil {
		return "", err
	}
	defer skutil.Close(resp.Body)
	decodedBody, err := ioutil.ReadAll(base64.NewDecoder(base64.StdEncoding, resp.Body))
	if err != nil {
		return "", err
	}
	regexpMatches := skiaRevisionRegexp.FindSubmatch(decodedBody)
	if regexpMatches == nil || len(regexpMatches) < 2 || len(regexpMatches[1]) == 0 {
		return "", fmt.Errorf("Could not find skia_revision in %s", CHROMIUM_DEPS_FILE)
	}
	return bytes.NewBuffer(regexpMatches[1]).String(), nil
}

func getSkiaRevDataHandler(w http.ResponseWriter, r *http.Request) {
	getRevDataHandler(getSkiaLkgr, SKIA_GIT_REPO_URL, w, r)
}

// Define api.ChromiumBuildUpdateVars in this package so we can add methods.
type ChromiumBuildUpdateVars struct {
	api.ChromiumBuildUpdateVars
}

func (task *ChromiumBuildUpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
	return nil, nil, nil
}

func updateChromiumBuildTaskHandler(w http.ResponseWriter, r *http.Request) {
	updateTaskHandler(&ChromiumBuildUpdateVars{}, db.TABLE_CHROMIUM_BUILD_TASKS, w, r)
}

func chromiumBuildRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(chromiumBuildRunsHistoryTemplate, w, r)
}

func getChromiumBuildTasksHandler(w http.ResponseWriter, r *http.Request) {
	getTasksHandler(&ChromiumBuildDBTask{}, w, r)
}

// Validate that the given chromiumBuild exists in the DB.
func validateChromiumBuild(chromiumBuild ChromiumBuildDBTask) error {
	buildCount := []int{}
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE chromium_rev = ? AND skia_rev = ? AND ts_completed IS NOT NULL AND failure = 0", db.TABLE_CHROMIUM_BUILD_TASKS)
	if err := db.DB.Select(&buildCount, query, chromiumBuild.ChromiumRev, chromiumBuild.SkiaRev); err != nil || len(buildCount) < 1 || buildCount[0] == 0 {
		glog.Info(err)
		return fmt.Errorf("Unable to validate chromium_build parameter %v", chromiumBuild)
	}
	return nil
}

func adminTasksView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(adminTasksTemplate, w, r)
}

type AdminTaskVars struct {
	AddTaskCommonVars
}

func (vars *AdminTaskVars) IsAdminTask() bool {
	return true
}

// Represents the parameters sent as JSON to the add_recreate_page_sets_task handler.
type AddRecreatePageSetsTaskVars struct {
	AdminTaskVars
	PageSets string `json:"page_sets"`
}

func (task *AddRecreatePageSetsTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.PageSets == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	return fmt.Sprintf("INSERT INTO %s (username,page_sets,ts_added,repeat_after_days) VALUES (?,?,?,?);",
			db.TABLE_RECREATE_PAGE_SETS_TASKS),
		[]interface{}{
			task.Username,
			task.PageSets,
			task.TsAdded,
			task.RepeatAfterDays,
		},
		nil
}

func addRecreatePageSetsTaskHandler(w http.ResponseWriter, r *http.Request) {
	addTaskHandler(w, r, &AddRecreatePageSetsTaskVars{})
}

// Represents the parameters sent as JSON to the add_recreate_webpage_archives_task handler.
type AddRecreateWebpageArchivesTaskVars struct {
	AdminTaskVars
	PageSets      string              `json:"page_sets"`
	ChromiumBuild ChromiumBuildDBTask `json:"chromium_build"`
}

func (task *AddRecreateWebpageArchivesTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.PageSets == "" ||
		task.ChromiumBuild.ChromiumRev == "" ||
		task.ChromiumBuild.SkiaRev == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	if err := validateChromiumBuild(task.ChromiumBuild); err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("INSERT INTO %s (username,page_sets,chromium_rev,skia_rev,ts_added,repeat_after_days) VALUES (?,?,?,?,?,?);",
			db.TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS),
		[]interface{}{
			task.Username,
			task.PageSets,
			task.ChromiumBuild.ChromiumRev,
			task.ChromiumBuild.SkiaRev,
			task.TsAdded,
			task.RepeatAfterDays,
		},
		nil
}

func addRecreateWebpageArchivesTaskHandler(w http.ResponseWriter, r *http.Request) {
	addTaskHandler(w, r, &AddRecreateWebpageArchivesTaskVars{})
}

// Define api.RecreatePageSetsUpdateVars in this package so we can add methods.
type RecreatePageSetsUpdateVars struct {
	api.RecreatePageSetsUpdateVars
}

func (task *RecreatePageSetsUpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
	return nil, nil, nil
}

func updateRecreatePageSetsTaskHandler(w http.ResponseWriter, r *http.Request) {
	updateTaskHandler(&RecreatePageSetsUpdateVars{}, db.TABLE_RECREATE_PAGE_SETS_TASKS, w, r)
}

// Define api.RecreateWebpageArchivesUpdateVars in this package so we can add methods.
type RecreateWebpageArchivesUpdateVars struct {
	api.RecreateWebpageArchivesUpdateVars
}

func (task *RecreateWebpageArchivesUpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
	return nil, nil, nil
}

func updateRecreateWebpageArchivesTaskHandler(w http.ResponseWriter, r *http.Request) {
	updateTaskHandler(&RecreateWebpageArchivesUpdateVars{}, db.TABLE_RECREATE_PAGE_SETS_TASKS, w, r)
}

func recreatePageSetsRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(recreatePageSetsRunsHistoryTemplate, w, r)
}

func recreateWebpageArchivesRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(recreateWebpageArchivesRunsHistoryTemplate, w, r)
}

func getRecreatePageSetsTasksHandler(w http.ResponseWriter, r *http.Request) {
	getTasksHandler(&RecreatePageSetsDBTask{}, w, r)
}

func getRecreateWebpageArchivesTasksHandler(w http.ResponseWriter, r *http.Request) {
	getTasksHandler(&RecreateWebpageArchivesDBTask{}, w, r)
}

func runsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(runsHistoryTemplate, w, r)
}

func getAllPendingTasks() ([]Task, error) {
	tasks := []Task{}
	for _, tableName := range taskTables {
		var task Task
		query := fmt.Sprintf("SELECT * FROM %s WHERE ts_completed IS NULL ORDER BY ts_added LIMIT 1;", tableName)
		switch tableName {
		case db.TABLE_CHROMIUM_PERF_TASKS:
			task = &ChromiumPerfDBTask{}
		case db.TABLE_CAPTURE_SKPS_TASKS:
			task = &CaptureSkpsDBTask{}
		case db.TABLE_LUA_SCRIPT_TASKS:
			task = &LuaScriptDBTask{}
		case db.TABLE_CHROMIUM_BUILD_TASKS:
			task = &ChromiumBuildDBTask{}
		case db.TABLE_RECREATE_PAGE_SETS_TASKS:
			task = &RecreatePageSetsDBTask{}
		case db.TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS:
			task = &RecreateWebpageArchivesDBTask{}
		default:
			panic("Unknown table " + tableName)
		}

		if err := db.DB.Get(task, query); err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("Failed to query DB: %v", err)
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func getOldestPendingTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tasks, err := getAllPendingTasks()
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to get all pending tasks: %v", err))
		return
	}

	var oldestTask Task
	for _, task := range tasks {
		if oldestTask == nil {
			oldestTask = task
		} else if oldestTask.GetAddedTimestamp() < task.GetAddedTimestamp() {
			oldestTask = task
		}
	}

	oldestTaskJsonRepr := map[string]Task{}
	if oldestTask != nil {
		oldestTaskJsonRepr[oldestTask.GetTaskName()] = oldestTask
	}
	if err := json.NewEncoder(w).Encode(oldestTaskJsonRepr); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

func pendingTasksView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(pendingTasksTemplate, w, r)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, login.LoginURL(w, r), http.StatusFound)
	return
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(skutil.MakeResourceHandler(*resourcesDir))

	// Chromium Perf handlers.
	r.HandleFunc("/", chromiumPerfView).Methods("GET")
	r.HandleFunc("/"+api.CHROMIUM_PERF_URI, chromiumPerfView).Methods("GET")
	r.HandleFunc("/"+api.CHROMIUM_PERF_RUNS_URI, chromiumPerfRunsHistoryView).Methods("GET")
	r.HandleFunc("/"+api.CHROMIUM_PERF_PARAMETERS_POST_URI, chromiumPerfHandler).Methods("POST")
	r.HandleFunc("/"+api.ADD_CHROMIUM_PERF_TASK_POST_URI, addChromiumPerfTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.GET_CHROMIUM_PERF_TASKS_POST_URI, getChromiumPerfTasksHandler).Methods("POST")
	r.HandleFunc("/"+api.UPDATE_CHROMIUM_PERF_TASK_POST_URI, updateChromiumPerfTaskHandler).Methods("POST")

	// Capture SKPs handlers.
	r.HandleFunc("/"+api.CAPTURE_SKPS_URI, captureSkpsView).Methods("GET")
	r.HandleFunc("/"+api.CAPTURE_SKPS_RUNS_URI, captureSkpRunsHistoryView).Methods("GET")
	r.HandleFunc("/"+api.ADD_CAPTURE_SKPS_TASK_POST_URI, addCaptureSkpsTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.GET_CAPTURE_SKPS_TASKS_POST_URI, getCaptureSkpTasksHandler).Methods("POST")
	r.HandleFunc("/"+api.UPDATE_CAPTURE_SKPS_TASK_POST_URI, updateCaptureSkpsTaskHandler).Methods("POST")

	// Lua Script handlers.
	r.HandleFunc("/"+api.LUA_SCRIPT_URI, luaScriptsView).Methods("GET")
	r.HandleFunc("/"+api.LUA_SCRIPT_RUNS_URI, luaScriptRunsHistoryView).Methods("GET")
	r.HandleFunc("/"+api.ADD_LUA_SCRIPT_TASK_POST_URI, addLuaScriptTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.GET_LUA_SCRIPT_TASKS_POST_URI, getLuaScriptTasksHandler).Methods("POST")
	r.HandleFunc("/"+api.UPDATE_LUA_SCRIPT_TASK_POST_URI, updateLuaScriptTaskHandler).Methods("POST")

	// Chromium Build handlers.
	r.HandleFunc("/"+api.CHROMIUM_BUILD_URI, chromiumBuildsView).Methods("GET")
	r.HandleFunc("/"+api.CHROMIUM_BUILD_RUNS_URI, chromiumBuildRunsHistoryView).Methods("GET")
	r.HandleFunc("/"+api.CHROMIUM_REV_DATA_POST_URI, getChromiumRevDataHandler).Methods("POST")
	r.HandleFunc("/"+api.SKIA_REV_DATA_POST_URI, getSkiaRevDataHandler).Methods("POST")
	r.HandleFunc("/"+api.ADD_CHROMIUM_BUILD_TASK_POST_URI, addChromiumBuildTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.GET_CHROMIUM_BUILD_TASKS_POST_URI, getChromiumBuildTasksHandler).Methods("POST")
	r.HandleFunc("/"+api.UPDATE_CHROMIUM_BUILD_TASK_POST_URI, updateChromiumBuildTaskHandler).Methods("POST")

	// Admin Tasks handlers.
	r.HandleFunc("/"+api.ADMIN_TASK_URI, adminTasksView).Methods("GET")
	r.HandleFunc("/"+api.RECREATE_PAGE_SETS_RUNS_URI, recreatePageSetsRunsHistoryView).Methods("GET")
	r.HandleFunc("/"+api.RECREATE_WEBPAGE_ARCHIVES_RUNS_URI, recreateWebpageArchivesRunsHistoryView).Methods("GET")
	r.HandleFunc("/"+api.ADD_RECREATE_PAGE_SETS_TASK_POST_URI, addRecreatePageSetsTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.ADD_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, addRecreateWebpageArchivesTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.GET_RECREATE_PAGE_SETS_TASKS_POST_URI, getRecreatePageSetsTasksHandler).Methods("POST")
	r.HandleFunc("/"+api.GET_RECREATE_WEBPAGE_ARCHIVES_TASKS_POST_URI, getRecreateWebpageArchivesTasksHandler).Methods("POST")
	r.HandleFunc("/"+api.UPDATE_RECREATE_PAGE_SETS_TASK_POST_URI, updateRecreatePageSetsTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.UPDATE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, updateRecreateWebpageArchivesTaskHandler).Methods("POST")

	// Runs history handlers.
	r.HandleFunc("/"+api.RUNS_HISTORY_URI, runsHistoryView).Methods("GET")

	// Task Queue handlers.
	r.HandleFunc("/"+api.PENDING_TASKS_URI, pendingTasksView).Methods("GET")
	r.HandleFunc("/"+api.GET_OLDEST_PENDING_TASK_URI, getOldestPendingTaskHandler).Methods("GET")

	// Common handlers used by different pages.
	r.HandleFunc("/"+api.PAGE_SETS_PARAMETERS_POST_URI, pageSetsHandler).Methods("POST")
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/login/", loginHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	http.Handle("/", skutil.LoggingGzipRequestResponse(r))
	glog.Infof("Ready to serve on %s", serverURL)
	glog.Fatal(http.ListenAndServe(*port, nil))
}

// repeatedTasksScheduler looks for all tasks that contain repeat_after_days
// set to > 0 and schedules them when the specified time comes.
func repeatedTasksScheduler() {
	for _ = range time.Tick(5 * time.Minute) {
		glog.Info("Checking for repeated tasks that need to be scheduled..")

		// TODO(rmistry): Complete this implementation.
		// This function needs to do the following:
		// 1. Look for tasks that need to be scheduled in the next 5 minutes.
		// 2. Loop over these tasks.
		//   2.1 Update the task and set repeat_after_days to 0.
		//   2.2 Schedule the task again and set repeat_after_days to what it
		//       originally was.
	}
}

func main() {
	// Setup flags.
	dbConf := db.DBConfigFromFlags()
	influxdb.SetupFlags()

	common.InitWithMetrics("ctfe", graphiteServer)
	v, err := skiaversion.GetVersion()
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Version %s, built at %s", v.Commit, v.Date)

	Init()
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	// Setup InfluxDB client.
	dbClient, err = influxdb.NewClientFromFlagsAndMetadata(*local)
	if err != nil {
		glog.Fatal(err)
	}

	// By default use a set of credentials setup for localhost access.
	var cookieSalt = "notverysecret"
	var clientID = "31977622648-1873k0c1e5edaka4adpv1ppvhr5id3qm.apps.googleusercontent.com"
	var clientSecret = "cw0IosPu4yjaG2KWmppj2guj"
	var redirectURL = serverURL + "/oauth2callback/"
	if !*local {
		cookieSalt = metadata.Must(metadata.ProjectGet(metadata.COOKIESALT))
		clientID = metadata.Must(metadata.ProjectGet(metadata.CLIENT_ID))
		clientSecret = metadata.Must(metadata.ProjectGet(metadata.CLIENT_SECRET))
	}
	login.Init(clientID, clientSecret, redirectURL, cookieSalt, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST, *local)

	glog.Info("CloneOrUpdate complete")

	// Initialize the ctfe database.
	if !*local {
		if err := dbConf.GetPasswordFromMetadata(); err != nil {
			glog.Fatal(err)
		}
	}
	if err := dbConf.InitDB(); err != nil {
		glog.Fatal(err)
	}

	// Start the repeated tasks scheduler.
	go repeatedTasksScheduler()

	runServer(serverURL)
}
