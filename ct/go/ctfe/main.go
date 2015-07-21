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
		db.TABLE_CHROMIUM_BUILD_TASKS,
		db.TABLE_RECREATE_PAGE_SETS_TASKS,
		db.TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS,
	}

	chromiumPerfTemplate                       *template.Template = nil
	chromiumPerfRunsHistoryTemplate            *template.Template = nil
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
	Id          int64         `db:"id"`
	TsAdded     sql.NullInt64 `db:"ts_added"`
	TsStarted   sql.NullInt64 `db:"ts_started"`
	TsCompleted sql.NullInt64 `db:"ts_completed"`
	Username    string        `db:"username"`
	Failure     sql.NullBool  `db:"failure"`
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

	PageSets      string `db:"page_sets"`
	ChromiumBuild string `db:"chromium_build"`
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
		filepath.Join(*resourcesDir, "templates/drawer.html"),
	))
	chromiumPerfRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/chromium_perf_runs_history.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/drawer.html"),
	))

	chromiumBuildsTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/chromium_builds.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/drawer.html"),
	))
	chromiumBuildRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/chromium_build_runs_history.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/drawer.html"),
	))

	adminTasksTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/admin_tasks.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/drawer.html"),
	))
	recreatePageSetsRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/recreate_page_sets_runs_history.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/drawer.html"),
	))
	recreateWebpageArchivesRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/recreate_webpage_archives_runs_history.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/drawer.html"),
	))

	runsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/runs_history.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/drawer.html"),
	))

	pendingTasksTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/pending_tasks.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/drawer.html"),
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

func getCurrentTs() string {
	return time.Now().UTC().Format("20060102150405")
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
	Username string
	TsAdded  string
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
	task.GetAddTaskCommonVars().TsAdded = getCurrentTs()

	query, binds, err := task.GetInsertQueryAndBinds()
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to marshall %T task: %v", task, err))
		return
	}
	_, err = db.DB.Exec(query, binds...)
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to insert %T task: %v", task, err))
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
	return fmt.Sprintf("INSERT INTO %s (username,benchmark,platform,page_sets,repeat_runs,benchmark_args,browser_args_nopatch,browser_args_withpatch,description,chromium_patch,blink_patch,skia_patch,ts_added) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?);",
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
		},
		nil
}

func addChromiumPerfTaskHandler(w http.ResponseWriter, r *http.Request) {
	addTaskHandler(w, r, &ChromiumPerfVars{})
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

func dbTaskQuery(prototype Task, username string, successful bool, includeCompleted bool, countQuery bool, offset int, size int) (string, []interface{}) {
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
	if successful && !includeCompleted {
		skutil.ReportError(w, r, fmt.Errorf("Inconsistent params: successful %v not_completed %v", r.FormValue("successful"), r.FormValue("not_completed")), "")
		return
	}
	offset, size, err := skutil.PaginationParams(r.URL.Query(), 0, DEFAULT_PAGE_SIZE, MAX_PAGE_SIZE)
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to get pagination params: %v", err))
		return
	}
	query, args := dbTaskQuery(prototype, username, successful, includeCompleted, false, offset, size)
	glog.Infof("Running %s", query)
	data, err := prototype.Select(query, args...)
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to query %s tasks: %v", prototype.GetTaskName(), err))
		return
	}

	query, args = dbTaskQuery(prototype, username, successful, includeCompleted, true, 0, 0)
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

func getChromiumPerfTasksHandler(w http.ResponseWriter, r *http.Request) {
	getTasksHandler(&ChromiumPerfDBTask{}, w, r)
}

func chromiumPerfRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(chromiumPerfRunsHistoryTemplate, w, r)
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
	return fmt.Sprintf("INSERT INTO %s (username,chromium_rev,chromium_rev_ts,skia_rev,ts_added) VALUES (?,?,?,?,?);",
			db.TABLE_CHROMIUM_BUILD_TASKS),
		[]interface{}{
			task.Username,
			task.ChromiumRev,
			chromiumRevTs,
			task.SkiaRev,
			task.TsAdded,
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

func chromiumBuildRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(chromiumBuildRunsHistoryTemplate, w, r)
}

func getChromiumBuildTasksHandler(w http.ResponseWriter, r *http.Request) {
	getTasksHandler(&ChromiumBuildDBTask{}, w, r)
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
type RecreatePageSetsTaskHandlerVars struct {
	AdminTaskVars
	PageSets string `json:"page_sets"`
}

func (task *RecreatePageSetsTaskHandlerVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.PageSets == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	return fmt.Sprintf("INSERT INTO %s (username,page_sets,ts_added) VALUES (?,?,?);",
			db.TABLE_RECREATE_PAGE_SETS_TASKS),
		[]interface{}{
			task.Username,
			task.PageSets,
			task.TsAdded,
		},
		nil
}

func addRecreatePageSetsTaskHandler(w http.ResponseWriter, r *http.Request) {
	addTaskHandler(w, r, &RecreatePageSetsTaskHandlerVars{})
}

// Represents the parameters sent as JSON to the add_recreate_webpage_archives_task handler.
type RecreateWebpageArchivesTaskHandlerVars struct {
	AdminTaskVars
	PageSets      string `json:"page_sets"`
	ChromiumBuild string `json:"chromium_build"`
}

func (task *RecreateWebpageArchivesTaskHandlerVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.PageSets == "" ||
		task.ChromiumBuild == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	return fmt.Sprintf("INSERT INTO %s (username,page_sets,chromium_build,ts_added) VALUES (?,?,?,?);",
			db.TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS),
		[]interface{}{
			task.Username,
			task.PageSets,
			task.ChromiumBuild,
			task.TsAdded,
		},
		nil
}

func addRecreateWebpageArchivesTaskHandler(w http.ResponseWriter, r *http.Request) {
	addTaskHandler(w, r, &RecreateWebpageArchivesTaskHandlerVars{})
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
	r.HandleFunc("/chromium_perf/", chromiumPerfView).Methods("GET")
	r.HandleFunc("/chromium_perf_runs/", chromiumPerfRunsHistoryView).Methods("GET")
	r.HandleFunc("/_/chromium_perf/", chromiumPerfHandler).Methods("POST")
	r.HandleFunc("/_/add_chromium_perf_task", addChromiumPerfTaskHandler).Methods("POST")
	r.HandleFunc("/_/get_chromium_perf_tasks", getChromiumPerfTasksHandler).Methods("POST")

	// Chromium Build handlers.
	r.HandleFunc("/chromium_builds/", chromiumBuildsView).Methods("GET")
	r.HandleFunc("/chromium_builds_runs/", chromiumBuildRunsHistoryView).Methods("GET")
	r.HandleFunc("/_/chromium_rev_data", getChromiumRevDataHandler).Methods("POST")
	r.HandleFunc("/_/skia_rev_data", getSkiaRevDataHandler).Methods("POST")
	r.HandleFunc("/_/chromium_builds", getChromiumBuildTasksHandler).Methods("POST")
	r.HandleFunc("/_/add_chromium_build_task", addChromiumBuildTaskHandler).Methods("POST")
	r.HandleFunc("/_/get_chromium_build_tasks", getChromiumBuildTasksHandler).Methods("POST")

	// Admin Tasks handlers.
	r.HandleFunc("/admin_tasks/", adminTasksView).Methods("GET")
	r.HandleFunc("/recreate_page_sets_runs/", recreatePageSetsRunsHistoryView).Methods("GET")
	r.HandleFunc("/recreate_webpage_archives_runs/", recreateWebpageArchivesRunsHistoryView).Methods("GET")
	r.HandleFunc("/_/add_recreate_page_sets_task", addRecreatePageSetsTaskHandler).Methods("POST")
	r.HandleFunc("/_/add_recreate_webpage_archives_task", addRecreateWebpageArchivesTaskHandler).Methods("POST")
	r.HandleFunc("/_/get_recreate_page_sets_tasks", getRecreatePageSetsTasksHandler).Methods("POST")
	r.HandleFunc("/_/get_recreate_webpage_archives_tasks", getRecreateWebpageArchivesTasksHandler).Methods("POST")

	// Runs history handlers.
	r.HandleFunc("/history/", runsHistoryView).Methods("GET")

	// Task Queue handlers.
	r.HandleFunc("/queue/", pendingTasksView).Methods("GET")
	r.HandleFunc("/_/get_oldest_pending_task", getOldestPendingTaskHandler).Methods("GET")

	// Common handlers used by different pages.
	r.HandleFunc("/_/page_sets/", pageSetsHandler).Methods("POST")
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/login/", loginHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	http.Handle("/", skutil.LoggingGzipRequestResponse(r))
	glog.Infof("Ready to serve on %s", serverURL)
	glog.Fatal(http.ListenAndServe(*port, nil))
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

	runServer(serverURL)
}
