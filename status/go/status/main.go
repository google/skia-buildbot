/*
	Provides roll-up statuses for Skia build/test/perf.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"

	"golang.org/x/net/context"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/influxdb_init"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/polling_status"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/status/go/capacity"
	"go.skia.org/infra/status/go/franken"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
)

const (
	DEFAULT_COMMITS_TO_LOAD = 50
	MAX_COMMITS_TO_LOAD     = 100
	SKIA_REPO               = "skia"
	INFRA_REPO              = "infra"
	// The from clause needs to be in double quotes and the where clauses need to be
	// in single quotes because InfluxDB is quite particular about these things.
	GOLD_STATUS_QUERY_TMPL = `select value from "gold.status.by-corpus" WHERE time > now() - 1h and host='skia-gold-prod' AND app='skiacorrectness' AND type='untriaged' AND corpus='%s' ORDER BY time DESC LIMIT 1`
	PERF_STATUS_QUERY      = `select value from "perf.clustering.untriaged" where time > now() - 1h and app='skiaperf' and host='skia-perf' order by time desc limit 1`

	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"
)

var (
	buildCache       *franken.BTCache              = nil
	buildDb          buildbot.DB                   = nil
	capacityClient   *capacity.CapacityClient      = nil
	capacityTemplate *template.Template            = nil
	commitsTemplate  *template.Template            = nil
	dbClient         *influxdb.Client              = nil
	goldGMStatus     *polling_status.PollingStatus = nil
	goldImageStatus  *polling_status.PollingStatus = nil
	goldSKPStatus    *polling_status.PollingStatus = nil
	perfStatus       *polling_status.PollingStatus = nil
	tasksPerCommit   *tasksPerCommitCache          = nil
)

// flags
var (
	host                        = flag.String("host", "localhost", "HTTP service host")
	port                        = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	useMetadata                 = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	testing                     = flag.Bool("testing", false, "Set to true for locally testing rules. No email will be sent.")
	workdir                     = flag.String("workdir", ".", "Directory to use for scratch work.")
	resourcesDir                = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	buildbotDbHost              = flag.String("buildbot_db_host", "skia-datahopper2:8000", "Where the Skia buildbot database is hosted.")
	taskSchedulerDbUrl          = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
	capacityRecalculateInterval = flag.Duration("capacity_recalculate_interval", 10*time.Minute, "How often to re-calculate capacity statistics.")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")

	repoMap = map[string]string{
		"skia":  common.REPO_SKIA,
		"infra": common.REPO_SKIA_INFRA,
	}
)

// StringIsInteresting returns true iff the string contains non-whitespace characters.
func StringIsInteresting(s string) bool {
	for _, c := range s {
		if !unicode.IsSpace(c) {
			return true
		}
	}
	return false
}

func reloadTemplates() {
	// Change the current working directory to two directories up from this source file so that we
	// can read templates and serve static (res/) files.

	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	commitsTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/commits.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
	capacityTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/capacity.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func Init() {
	reloadTemplates()
}

func userHasEditRights(r *http.Request) bool {
	return strings.HasSuffix(login.LoggedInAs(r), "@google.com")
}

func getIntParam(name string, r *http.Request) (*int, error) {
	raw, ok := r.URL.Query()[name]
	if !ok {
		return nil, nil
	}
	v64, err := strconv.ParseInt(raw[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("Invalid integer value for parameter %q", name)
	}
	v32 := int(v64)
	return &v32, nil
}

func getRepo(r *http.Request) string {
	repoName, _ := mux.Vars(r)["repo"]
	return repoMap[repoName]
}

// commitsJsonHandler writes information about a range of commits into the
// ResponseWriter. The information takes the form of a JSON-encoded CommitsData
// object.
func commitsJsonHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("commitsJsonHandler").Stop()
	w.Header().Set("Content-Type", "application/json")
	commitsToLoad := DEFAULT_COMMITS_TO_LOAD
	n, err := getIntParam("n", r)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Invalid parameter: %v", err))
		return
	}
	if n != nil {
		commitsToLoad = *n
	}
	// Prevent server overload.
	if commitsToLoad > MAX_COMMITS_TO_LOAD {
		commitsToLoad = MAX_COMMITS_TO_LOAD
	}
	if commitsToLoad < 0 {
		commitsToLoad = DEFAULT_COMMITS_TO_LOAD
	}
	repo := getRepo(r)
	rv, err := buildCache.GetLastN(repo, commitsToLoad, login.IsGoogler(r))
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to load commits from cache: %v", err))
		return
	}
	if err := json.NewEncoder(w).Encode(rv); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode response: %s", err))
		return
	}
}

func addBuildCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("addBuildCommentHandler").Stop()
	if !userHasEditRights(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	master, ok := mux.Vars(r)["master"]
	if !ok {
		httputils.ReportError(w, r, fmt.Errorf("No build master given!"), "No build master given!")
		return
	}
	builder, ok := mux.Vars(r)["builder"]
	if !ok {
		httputils.ReportError(w, r, fmt.Errorf("No builder given!"), "No builder given!")
		return
	}
	number, err := strconv.ParseInt(mux.Vars(r)["number"], 10, 64)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("No valid build number given: %v", err))
		return
	}

	comment := struct {
		Comment string `json:"comment"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to add comment: %v", err))
		return
	}
	defer util.Close(r.Body)
	c := buildbot.BuildComment{
		User:      login.LoggedInAs(r),
		Timestamp: time.Now().UTC(),
		Message:   comment.Comment,
	}
	if err := buildCache.AddBuildComment(master, builder, int(number), &c); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to add comment: %v", err))
		return
	}
}

func deleteBuildCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("deleteBuildCommentHandler").Stop()
	if !userHasEditRights(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	master, ok := mux.Vars(r)["master"]
	if !ok {
		httputils.ReportError(w, r, fmt.Errorf("No build master given!"), "No build master given!")
		return
	}
	builder, ok := mux.Vars(r)["builder"]
	if !ok {
		httputils.ReportError(w, r, fmt.Errorf("No builder given!"), "No builder given!")
		return
	}
	number, err := strconv.ParseInt(mux.Vars(r)["number"], 10, 64)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("No valid build number given: %v", err))
		return
	}
	commentId, err := strconv.ParseInt(mux.Vars(r)["commentId"], 10, 64)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Invalid comment id: %v", err))
		return
	}
	if err := buildCache.DeleteBuildComment(master, builder, int(number), commentId); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to delete comment: %v", err))
		return
	}
}

func addBuilderCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("addBuilderCommentHandler").Stop()
	if !userHasEditRights(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	builder := mux.Vars(r)["builder"]

	comment := struct {
		Comment       string `json:"comment"`
		Flaky         bool   `json:"flaky"`
		IgnoreFailure bool   `json:"ignoreFailure"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to add comment: %v", err))
		return
	}
	defer util.Close(r.Body)

	c := buildbot.BuilderComment{
		Builder:       builder,
		User:          login.LoggedInAs(r),
		Timestamp:     time.Now().UTC(),
		Flaky:         comment.Flaky,
		IgnoreFailure: comment.IgnoreFailure,
		Message:       comment.Comment,
	}
	if err := buildCache.AddBuilderComment(builder, &c); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to add builder comment: %v", err))
		return
	}
}

func deleteBuilderCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("deleteBuilderCommentHandler").Stop()
	if !userHasEditRights(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	builder := mux.Vars(r)["builder"]
	commentId, err := strconv.ParseInt(mux.Vars(r)["commentId"], 10, 32)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Invalid comment id: %v", err))
		return
	}
	if err := buildCache.DeleteBuilderComment(builder, commentId); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to delete comment: %v", err))
		return
	}
}

func addCommitCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("addCommitCommentHandler").Stop()
	if !userHasEditRights(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	repo := getRepo(r)
	commit := mux.Vars(r)["commit"]
	comment := struct {
		Comment       string `json:"comment"`
		IgnoreFailure bool   `json:"ignoreFailure"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to add comment: %v", err))
		return
	}
	defer util.Close(r.Body)

	c := buildbot.CommitComment{
		Commit:        commit,
		User:          login.LoggedInAs(r),
		Timestamp:     time.Now().UTC(),
		IgnoreFailure: comment.IgnoreFailure,
		Message:       comment.Comment,
	}
	if err := buildCache.AddCommitComment(repo, &c); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to add commit comment: %s", err))
		return
	}
}

func deleteCommitCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("deleteCommitCommentHandler").Stop()
	if !userHasEditRights(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	repo := getRepo(r)
	commit := mux.Vars(r)["commit"]
	commentId, err := strconv.ParseInt(mux.Vars(r)["commentId"], 10, 64)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Invalid comment id: %v", err))
		return
	}
	if err := buildCache.DeleteCommitComment(repo, commit, commentId); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to delete commit comment: %s", err))
		return
	}
}

type commitsTemplateData struct {
	Repo     string
	Title    string
	RepoBase string
}

func commitsHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("commitsHandler").Stop()
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}

	d := commitsTemplateData{
		Repo:     "skia",
		Title:    "Skia Status",
		RepoBase: "https://skia.googlesource.com/skia/+/",
	}

	if err := commitsTemplate.Execute(w, d); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
	}
}

func infraHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("infraHandler").Stop()
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}

	d := commitsTemplateData{
		Repo:     "infra",
		Title:    "Skia Infra Status",
		RepoBase: "https://skia.googlesource.com/buildbot/+/",
	}

	if err := commitsTemplate.Execute(w, d); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
	}
}

func capacityHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("capacityHandler").Stop()
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}

	if err := capacityTemplate.Execute(w, nil); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
	}
}

func capacityStatsHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("capacityStatsHandler").Stop()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(capacityClient.CapacityMetrics()); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode response: %s", err))
		return
	}
}

func perfJsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"alerts": perfStatus}); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to report Perf status: %v", err))
		return
	}
}

func goldJsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"gm": goldGMStatus,
		// Uncomment once we track SKP's again.
		// "skp":   goldSKPStatus,
		"image": goldImageStatus,
	}); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

// buildProgressHandler returns the number of finished builds at the given
// commit, compared to that of an older commit.
func buildProgressHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("buildProgressHandler").Stop()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get the number of finished tasks for the requested commit.
	hash := r.FormValue("commit")
	repo := getRepo(r)
	builds, err := buildCache.GetBuildsForCommit(repo, hash, login.IsGoogler(r))
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to get the number of finished builds."))
		return
	}
	finished := 0
	for _, b := range builds {
		if b.Finished {
			finished++
		}
	}
	tasksForCommit, err := tasksPerCommit.Get(db.RepoState{
		Repo:     repo,
		Revision: hash,
	})
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to get number of tasks at commit."))
		return
	}
	proportion := 1.0
	if tasksForCommit > 0 {
		proportion = float64(finished) / float64(tasksForCommit)
	}

	res := struct {
		Commit             string  `json:"commit"`
		FinishedTasks      int     `json:"finishedTasks"`
		FinishedProportion float64 `json:"finishedProportion"`
		TotalTasks         int     `json:"totalTasks"`
	}{
		Commit:             hash,
		FinishedTasks:      finished,
		FinishedProportion: proportion,
		TotalTasks:         tasksForCommit,
	}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON."))
		return
	}
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.HandleFunc("/", commitsHandler)
	r.HandleFunc("/infra", infraHandler)
	r.HandleFunc("/capacity", capacityHandler)
	r.HandleFunc("/capacity/json", capacityStatsHandler)
	r.HandleFunc("/json/goldStatus", goldJsonHandler)
	r.HandleFunc("/json/perfAlerts", perfJsonHandler)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/json/{repo}/buildProgress", buildProgressHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc(OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	builds := r.PathPrefix("/json/{repo}/builds/{master}/{builder}/{number:[0-9]+}").Subrouter()
	builds.HandleFunc("/comments", addBuildCommentHandler).Methods("POST")
	builds.HandleFunc("/comments/{commentId:[0-9]+}", deleteBuildCommentHandler).Methods("DELETE")
	builders := r.PathPrefix("/json/{repo}/builders/{builder}").Subrouter()
	builders.HandleFunc("/comments", addBuilderCommentHandler).Methods("POST")
	builders.HandleFunc("/comments/{commentId:[0-9]+}", deleteBuilderCommentHandler).Methods("DELETE")
	commits := r.PathPrefix("/json/{repo}/commits").Subrouter()
	commits.HandleFunc("/", commitsJsonHandler)
	commits.HandleFunc("/{commit:[a-f0-9]+}/comments", addCommitCommentHandler).Methods("POST")
	commits.HandleFunc("/{commit:[a-f0-9]+}/comments/{commentId:[0-9]+}", deleteCommitCommentHandler).Methods("DELETE")
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	defer common.LogPanic()
	// Setup flags.

	common.InitWithMetrics2("status", influxHost, influxUser, influxPassword, influxDatabase, testing)
	v, err := skiaversion.GetVersion()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Version %s, built at %s", v.Commit, v.Date)

	Init()
	if *testing {
		*useMetadata = false
	}
	serverURL := "https://" + *host
	if *testing {
		serverURL = "http://" + *host + *port
	}

	// Create buildbot remote DB.
	buildDb, err = buildbot.NewRemoteDB(*buildbotDbHost)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create remote Tasks DB.
	taskDb, err := remote_db.NewClient(*taskSchedulerDbUrl)
	if err != nil {
		sklog.Fatal(err)
	}

	// Setup InfluxDB client.
	dbClient, err = influxdb_init.NewClientFromParamsAndMetadata(*influxHost, *influxUser, *influxPassword, *influxDatabase, *testing)
	if err != nil {
		sklog.Fatal(err)
	}

	// By default use a set of credentials setup for localhost access.
	var cookieSalt = "notverysecret"
	var clientID = "31977622648-1873k0c1e5edaka4adpv1ppvhr5id3qm.apps.googleusercontent.com"
	var clientSecret = "cw0IosPu4yjaG2KWmppj2guj"
	var redirectURL = serverURL + OAUTH2_CALLBACK_PATH
	if *useMetadata {
		cookieSalt = metadata.Must(metadata.ProjectGet(metadata.COOKIESALT))
		clientID = metadata.Must(metadata.ProjectGet(metadata.CLIENT_ID))
		clientSecret = metadata.Must(metadata.ProjectGet(metadata.CLIENT_SECRET))
	}
	login.Init(clientID, clientSecret, redirectURL, cookieSalt, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST, false)

	// Check out source code.
	repos, err := repograph.NewMap([]string{common.REPO_SKIA, common.REPO_SKIA_INFRA}, path.Join(*workdir, "repos"))
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Info("Checkout complete")

	// Cache for buildProgressHandler.
	tasksPerCommit, err = newTasksPerCommitCache(*workdir, []string{common.REPO_SKIA, common.REPO_SKIA_INFRA}, 14*24*time.Hour, context.Background())
	if err != nil {
		sklog.Fatalf("Failed to create tasksPerCommitCache: %s", err)
	}

	// Create the build cache.
	bc, err := franken.NewBTCache(repos, buildDb, taskDb)
	if err != nil {
		sklog.Fatalf("Failed to create build cache: %s", err)
	}
	buildCache = bc

	capacityClient = capacity.New(tasksPerCommit.tcc, bc.GetTaskCache(), repos)
	capacityClient.StartLoading(*capacityRecalculateInterval)

	// Load Perf and Gold data in a loop.
	perfStatus = dbClient.Int64PollingStatus("skmetrics", PERF_STATUS_QUERY, time.Minute)
	goldGMStatus = dbClient.Int64PollingStatus(*influxDatabase, fmt.Sprintf(GOLD_STATUS_QUERY_TMPL, "gm"), time.Minute)
	goldImageStatus = dbClient.Int64PollingStatus(*influxDatabase, fmt.Sprintf(GOLD_STATUS_QUERY_TMPL, "image"), time.Minute)

	// Run the server.
	runServer(serverURL)
}
