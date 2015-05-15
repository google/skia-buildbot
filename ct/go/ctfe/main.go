/*
	The Cluster Telemetry Frontend.
*/

package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"path/filepath"
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
	"go.skia.org/infra/go/timer"
	skutil "go.skia.org/infra/go/util"
)

const (
	// Default page size used for pagination.
	DEFAULT_PAGE_SIZE = 5

	// Maximum page size used for pagination.
	MAX_PAGE_SIZE = 100
)

var (
	taskTables = []string{
		db.TABLE_CHROMIUM_PERF_TASKS,
	}

	chromiumPerfTemplate            *template.Template = nil
	chromiumPerfRunsHistoryTemplate *template.Template = nil
	runsHistoryTemplate             *template.Template = nil
	pendingTasksTemplate            *template.Template = nil

	dbClient *influxdb.Client = nil
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

// ChromiumPerfVars is the type used by the Chromium Perf pages.
type ChromiumPerfVars struct {
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
	// Username does not have a JSON directive because it is calculated using the login module.
	Username string
	// TsAdded does not have a JSON directive because the current time is used.
	TsAdded string
}

type CommonCols struct {
	Id          int64          `db:"id"`
	TsAdded     sql.NullInt64  `db:"ts_added"`
	TsStarted   sql.NullInt64  `db:"ts_started"`
	TsCompleted sql.NullInt64  `db:"ts_completed"`
	Results     sql.NullString `db:"results"`
}

type Task interface {
	GetAddedTimestamp() int64
	GetTaskName() string
}

type ChromiumPerfDBTask struct {
	CommonCols

	Username             string         `db:"username"`
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
	Failure              sql.NullBool   `db:"failure"`
	NoPatchRawOutput     sql.NullString `db:"nopatch_raw_output"`
	WithPatchRawOutput   sql.NullString `db:"withpatch_raw_output"`
}

func (task ChromiumPerfDBTask) GetAddedTimestamp() int64 {
	return task.CommonCols.TsAdded.Int64
}

func (task ChromiumPerfDBTask) GetTaskName() string {
	return "ChromiumPerf"
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
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
	chromiumPerfRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/chromium_perf_runs_history.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))

	runsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/runs_history.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))

	pendingTasksTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/pending_tasks.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func Init() {
	reloadTemplates()
}

func userHasEditRights(r *http.Request) bool {
	return strings.HasSuffix(login.LoggedInAs(r), "@google.com") || strings.HasSuffix(login.LoggedInAs(r), "@chromium.org")
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

func chromiumPerfView(w http.ResponseWriter, r *http.Request) {
	defer timer.New("chromiumPerfView").Stop()
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in local mode.
	if *local {
		reloadTemplates()
	}

	if err := chromiumPerfTemplate.Execute(w, struct{}{}); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
		return
	}
}

func chromiumPerfHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("chromiumPerfHandler").Stop()
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
	defer timer.New("pageSetsHandler").Stop()
	w.Header().Set("Content-Type", "application/json")

	pageSetsToDesc := map[string]string{}
	for pageSet := range util.PagesetTypeToInfo {
		pageSetsToDesc[pageSet] = util.PagesetTypeToInfo[pageSet].Description
	}

	if err := json.NewEncoder(w).Encode(pageSetsToDesc); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

func addChromiumPerfTaskHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("addChromiumPerfTaskHandler").Stop()
	if !userHasEditRights(r) {
		skutil.ReportError(w, r, fmt.Errorf("Please login."), "")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	chromiumPerfVars := &ChromiumPerfVars{}
	if err := json.NewDecoder(r.Body).Decode(&chromiumPerfVars); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to add chromium perf task: %v", err))
		return
	}
	defer skutil.Close(r.Body)

	chromiumPerfVars.Username = login.LoggedInAs(r)
	chromiumPerfVars.TsAdded = time.Now().UTC().Format("20060102150405")

	_, err := db.DB.Exec(
		fmt.Sprintf("INSERT INTO %s (username,benchmark,platform,page_sets,repeat_runs,benchmark_args,browser_args_nopatch,browser_args_withpatch,description,chromium_patch,blink_patch,skia_patch,ts_added) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?);",
			db.TABLE_CHROMIUM_PERF_TASKS),
		chromiumPerfVars.Username,
		chromiumPerfVars.Benchmark,
		chromiumPerfVars.Platform,
		chromiumPerfVars.PageSets,
		chromiumPerfVars.RepeatRuns,
		chromiumPerfVars.BenchmarkArgs,
		chromiumPerfVars.BrowserArgsNoPatch,
		chromiumPerfVars.BrowserArgsWithPatch,
		chromiumPerfVars.Description,
		chromiumPerfVars.ChromiumPatch,
		chromiumPerfVars.BlinkPatch,
		chromiumPerfVars.SkiaPatch,
		chromiumPerfVars.TsAdded)
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to insert chromium perf task: %v", err))
		return
	}
}

func getChromiumPerfTasksHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("getChromiumPerfTasksHandler").Stop()
	w.Header().Set("Content-Type", "application/json")

	chromiumPerfTasks := []ChromiumPerfDBTask{}

	// Filter by either username or not started yet.
	username := r.FormValue("username")
	notCompleted := r.FormValue("not_completed")
	offset, size, err := skutil.PaginationParams(r.URL.Query(), 0, DEFAULT_PAGE_SIZE, MAX_PAGE_SIZE)
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to get pagination params: %v", err))
		return
	}
	args := []interface{}{}
	query := fmt.Sprintf("SELECT * FROM %s", db.TABLE_CHROMIUM_PERF_TASKS)
	if username != "" {
		query += " WHERE username=?"
		args = append(args, username)
	} else if notCompleted != "" {
		query += " WHERE ts_completed IS NULL"
	}
	query += " ORDER BY id DESC LIMIT ?,?"
	args = append(args, offset, size)
	glog.Infof("Running %s", query)
	if err := db.DB.Select(&chromiumPerfTasks, query, args...); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to query chromium perf tasks: %v", err))
		return
	}

	// Get the total count.
	countArgs := []interface{}{}
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", db.TABLE_CHROMIUM_PERF_TASKS)
	if username != "" {
		countQuery += " WHERE username=?"
		countArgs = append(countArgs, username)
	}
	glog.Infof("Running %s", countQuery)
	countVal := []int{}
	if err := db.DB.Select(&countVal, countQuery, countArgs...); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to query chromium perf tasks: %v", err))
		return
	}

	pagination := &skutil.ResponsePagination{
		Offset: offset,
		Size:   size,
		Total:  countVal[0],
	}
	jsonResponse := map[string]interface{}{
		"data":       chromiumPerfTasks,
		"pagination": pagination,
	}
	if err := json.NewEncoder(w).Encode(jsonResponse); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

func chromiumPerfRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	defer timer.New("chromiumPerfRunsHistoryView").Stop()
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in local mode.
	if *local {
		reloadTemplates()
	}

	if err := chromiumPerfRunsHistoryTemplate.Execute(w, struct{}{}); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
		return
	}
}

func runsHistoryView(w http.ResponseWriter, r *http.Request) {
	defer timer.New("runsHistoryView").Stop()
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in local mode.
	if *local {
		reloadTemplates()
	}

	if err := runsHistoryTemplate.Execute(w, struct{}{}); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
		return
	}
}

func getAllPendingTasks() ([]Task, error) {
	tasks := []Task{}
	for _, tableName := range taskTables {
		var task Task
		query := fmt.Sprintf("SELECT * FROM %s WHERE ts_completed IS NULL ORDER BY ts_added LIMIT 1;", tableName)
		if tableName == db.TABLE_CHROMIUM_PERF_TASKS {
			task = &ChromiumPerfDBTask{}
		}

		if err := db.DB.Get(task, query); err != nil {
			return nil, fmt.Errorf("Failed to query DB: %v", err)
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func getOldestPendingTaskHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("getOldestPendingTaskHandler").Stop()
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
	defer timer.New("pendingTasksView").Stop()
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in local mode.
	if *local {
		reloadTemplates()
	}

	if err := pendingTasksTemplate.Execute(w, struct{}{}); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
		return
	}
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

	// Runs history handlers.
	r.HandleFunc("/history/", runsHistoryView).Methods("GET")

	// Task Queue handlers.
	r.HandleFunc("/queue/", pendingTasksView).Methods("GET")
	r.HandleFunc("/_/get_oldest_pending_task", getOldestPendingTaskHandler).Methods("GET")

	// Common handlers used by different pages.
	r.HandleFunc("/_/page_sets/", pageSetsHandler).Methods("POST")
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
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
