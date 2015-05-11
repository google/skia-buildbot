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
)

import (
	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
)

import (
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/status/go/commit_cache"
)

const (
	DEFAULT_COMMITS_TO_LOAD = 50
	SKIA_REPO               = "skia"
	INFRA_REPO              = "infra"
)

var (
	commitCaches         map[string]*commit_cache.CommitCache = nil
	buildbotDashTemplate *template.Template                   = nil
	commitsTemplate      *template.Template                   = nil
	infraTemplate        *template.Template                   = nil
	dbClient             *influxdb.Client                     = nil
	goldGMStatus         *influxdb.IntPollingStatus           = nil
	goldSKPStatus        *influxdb.IntPollingStatus           = nil
	goldImageStatus      *influxdb.IntPollingStatus           = nil
	perfStatus           *influxdb.PollingStatus              = nil
)

// flags
var (
	graphiteServer = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	host           = flag.String("host", "localhost", "HTTP service host")
	port           = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	useMetadata    = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	testing        = flag.Bool("testing", false, "Set to true for locally testing rules. No email will be sent.")
	workdir        = flag.String("workdir", ".", "Directory to use for scratch work.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
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
	infraTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/infra.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
	buildbotDashTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/buildbot_dash.html"),
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
		return nil, fmt.Errorf("Invalid value for parameter %q: %s -- %v", name, raw, err)
	}
	v32 := int(v64)
	return &v32, nil
}

func getCommitCache(w http.ResponseWriter, r *http.Request) (*commit_cache.CommitCache, error) {
	repo, _ := mux.Vars(r)["repo"]
	cache, ok := commitCaches[repo]
	if !ok {
		e := fmt.Sprintf("Unknown repo: %s", repo)
		err := fmt.Errorf(e)
		util.ReportError(w, r, err, e)
		return nil, err
	}
	return cache, nil
}

func commitsJsonHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("commitsJsonHandler").Stop()
	w.Header().Set("Content-Type", "application/json")
	cache, err := getCommitCache(w, r)
	if err != nil {
		return
	}
	commitsToLoad := DEFAULT_COMMITS_TO_LOAD
	n, err := getIntParam("n", r)
	if err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Invalid parameter: %v", err))
		return
	}
	if n != nil {
		commitsToLoad = *n
	}
	if err := cache.LastNAsJson(w, commitsToLoad); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to load commits from cache: %v", err))
		return
	}
}

func addBuildCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("addBuildCommentHandler").Stop()
	if !userHasEditRights(r) {
		util.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	cache, err := getCommitCache(w, r)
	if err != nil {
		return
	}
	buildId, err := strconv.ParseInt(mux.Vars(r)["buildId"], 10, 32)
	if err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Invalid build id: %v", err))
		return
	}
	comment := struct {
		Comment string `json:"comment"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to add comment: %v", err))
		return
	}
	defer util.Close(r.Body)
	c := buildbot.BuildComment{
		BuildId:   int(buildId),
		User:      login.LoggedInAs(r),
		Timestamp: float64(time.Now().UTC().Unix()),
		Message:   comment.Comment,
	}
	if err := cache.AddBuildComment(int(buildId), &c); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to add comment: %v", err))
		return
	}
}

func addBuilderStatusHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("addBuilderStatusHandler").Stop()
	if !userHasEditRights(r) {
		util.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	cache, err := getCommitCache(w, r)
	if err != nil {
		return
	}
	builder := mux.Vars(r)["builder"]

	status := struct {
		Comment       string `json:"comment"`
		Flaky         bool   `json:"flaky"`
		IgnoreFailure bool   `json:"ignoreFailure"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&status); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to add comment: %v", err))
		return
	}
	defer util.Close(r.Body)

	s := buildbot.BuilderStatus{
		Builder:       builder,
		User:          login.LoggedInAs(r),
		Timestamp:     float64(time.Now().UTC().Unix()),
		Flaky:         status.Flaky,
		IgnoreFailure: status.IgnoreFailure,
		Message:       status.Comment,
	}
	if err := cache.SetBuilderStatus(builder, &s); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to set builder status: %v", err))
		return
	}
}

func addCommitCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("addCommitCommentHandler").Stop()
	if !userHasEditRights(r) {
		util.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	commit := mux.Vars(r)["commit"]
	comment := struct {
		Comment string `json:"comment"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to add comment: %v", err))
		return
	}
	defer util.Close(r.Body)

	c := buildbot.CommitComment{
		Commit:    commit,
		User:      login.LoggedInAs(r),
		Timestamp: float64(time.Now().UTC().Unix()),
		Message:   comment.Comment,
	}
	if _, err := c.InsertIntoDB(); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to add commit comment: %v", err))
		return
	}
}

func commitsHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("commitsHandler").Stop()
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}

	if err := commitsTemplate.Execute(w, struct{}{}); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
	}
}

func infraHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("infraHandler").Stop()
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}

	if err := infraTemplate.Execute(w, struct{}{}); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
	}
}

func buildsJsonHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("buildsHandler").Stop()
	w.Header().Set("Content-Type", "application/json")

	start, err := getIntParam("start", r)
	if err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Invalid value for parameter \"start\": %v", err))
		return
	}
	end, err := getIntParam("end", r)
	if err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Invalid value for parameter \"end\": %v", err))
		return
	}

	var startTime time.Time
	var endTime time.Time

	if end == nil {
		endTime = time.Now()
	} else {
		endTime = time.Unix(int64(*end), 0)
	}

	if start == nil {
		startTime = endTime.AddDate(0, 0, -1)
	} else {
		startTime = time.Unix(int64(*start), 0)
	}

	builds, err := buildbot.GetBuildsFromDateRange(startTime, endTime)
	if err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to load builds: %v", err))
		return
	}

	defer timer.New("buildsHandler_encode").Stop()
	if err := json.NewEncoder(w).Encode(builds); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

func buildbotDashHandler(w http.ResponseWriter, r *http.Request) {
	defer timer.New("buildbotDashHandler").Stop()
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}

	if err := buildbotDashTemplate.Execute(w, struct{}{}); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
		return
	}
}

func perfJsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := perfStatus.WriteJson(w); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to report Perf status: %v", err))
		return
	}
}

func goldJsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := map[string]int{
		"gm":    goldGMStatus.Get(),
		"skp":   goldSKPStatus.Get(),
		"image": goldImageStatus.Get(),
	}
	if err := json.NewEncoder(w).Encode(status); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(util.MakeResourceHandler(*resourcesDir))
	r.HandleFunc("/", commitsHandler)
	r.HandleFunc("/json/builds", buildsJsonHandler)
	builds := r.PathPrefix("/json/{repo}/builds/{buildId:[0-9]+}").Subrouter()
	builds.HandleFunc("/comments", addBuildCommentHandler).Methods("POST")
	r.HandleFunc("/buildbots", buildbotDashHandler)
	r.HandleFunc("/infra", infraHandler)
	builders := r.PathPrefix("/json/{repo}/builders/{builder}").Subrouter()
	builders.HandleFunc("/status", addBuilderStatusHandler).Methods("POST")
	commits := r.PathPrefix("/json/{repo}/commits").Subrouter()
	commits.HandleFunc("/", commitsJsonHandler)
	commits.HandleFunc("/{commit:[a-f0-9]+}/comments", addCommitCommentHandler).Methods("POST")
	r.HandleFunc("/json/perfAlerts", perfJsonHandler)
	r.HandleFunc("/json/goldStatus", goldJsonHandler)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	http.Handle("/", util.LoggingGzipRequestResponse(r))
	glog.Infof("Ready to serve on %s", serverURL)
	glog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	// Setup flags.
	database.SetupFlags(buildbot.PROD_DB_HOST, buildbot.PROD_DB_PORT, database.USER_RW, buildbot.PROD_DB_NAME)
	influxdb.SetupFlags()

	common.InitWithMetrics("status", graphiteServer)
	v, err := skiaversion.GetVersion()
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Version %s, built at %s", v.Commit, v.Date)

	Init()
	if *testing {
		*useMetadata = false
	}
	serverURL := "https://" + *host
	if *testing {
		serverURL = "http://" + *host + *port
	}

	// Setup InfluxDB client.
	dbClient, err = influxdb.NewClientFromFlagsAndMetadata(*testing)
	if err != nil {
		glog.Fatal(err)
	}

	// By default use a set of credentials setup for localhost access.
	var cookieSalt = "notverysecret"
	var clientID = "31977622648-1873k0c1e5edaka4adpv1ppvhr5id3qm.apps.googleusercontent.com"
	var clientSecret = "cw0IosPu4yjaG2KWmppj2guj"
	var redirectURL = serverURL + "/oauth2callback/"
	if *useMetadata {
		cookieSalt = metadata.Must(metadata.ProjectGet(metadata.COOKIESALT))
		clientID = metadata.Must(metadata.ProjectGet(metadata.CLIENT_ID))
		clientSecret = metadata.Must(metadata.ProjectGet(metadata.CLIENT_SECRET))
	}
	login.Init(clientID, clientSecret, redirectURL, cookieSalt, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST, false)

	// Check out source code.
	skiaRepo, err := gitinfo.CloneOrUpdate("https://skia.googlesource.com/skia.git", path.Join(*workdir, "skia"), true)
	if err != nil {
		glog.Fatalf("Failed to check out Skia: %v", err)
	}

	infraRepo, err := gitinfo.CloneOrUpdate("https://skia.googlesource.com/buildbot.git", path.Join(*workdir, "infra"), true)
	if err != nil {
		glog.Fatalf("Failed to checkout Infra: %v", err)
	}

	glog.Info("CloneOrUpdate complete")

	// Initialize the buildbot database.
	conf, err := database.ConfigFromFlagsAndMetadata(*testing, buildbot.MigrationSteps())
	if err != nil {
		glog.Fatal(err)
	}
	if err := buildbot.InitDB(conf); err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Database config: %s", conf.MySQLString)

	// Create the commit caches.
	commitCaches = map[string]*commit_cache.CommitCache{}
	skiaCache, err := commit_cache.New(skiaRepo, path.Join(*workdir, "commit_cache.gob"), DEFAULT_COMMITS_TO_LOAD)
	if err != nil {
		glog.Fatalf("Failed to create commit cache: %v", err)
	}
	commitCaches[SKIA_REPO] = skiaCache

	infraCache, err := commit_cache.New(infraRepo, path.Join(*workdir, "commit_cache_infra.gob"), DEFAULT_COMMITS_TO_LOAD)
	if err != nil {
		glog.Fatalf("Failed to create commit cache: %v", err)
	}
	commitCaches[INFRA_REPO] = infraCache
	glog.Info("commit_cache complete")

	// Load Perf and Gold data in a loop.
	var perfRes struct {
		Value int `json:"alerts" influxdb:"value"`
	}
	perfStatus, err = influxdb.NewPollingStatus(&perfRes, "select value from /skiaperf.skia-perf.alerting.new.value/ limit 1", dbClient)
	if err != nil {
		glog.Fatalf("Failed to create polling Perf status: %v", err)
	}
	goldGMStatus, err = influxdb.NewIntPollingStatus("select value from /skia-gold-prod.skiacorrectness.skia-gold-prod.gold.untriaged.by_corpus.gm.value/ limit 1", dbClient)
	if err != nil {
		glog.Fatalf("Failed to create polling Gold status: %v", err)
	}
	goldSKPStatus, err = influxdb.NewIntPollingStatus("select value from /skia-gold-prod.skiacorrectness.skia-gold-prod.gold.untriaged.by_corpus.skp.value/ limit 1", dbClient)
	if err != nil {
		glog.Fatalf("Failed to create polling Gold status: %v", err)
	}
	goldImageStatus, err = influxdb.NewIntPollingStatus("select value from /skia-gold-prod.skiacorrectness.skia-gold-prod.gold.untriaged.by_corpus.image.value/ limit 1", dbClient)
	if err != nil {
		glog.Fatalf("Failed to create polling Gold status: %v", err)
	}

	runServer(serverURL)
}
