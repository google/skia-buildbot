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
	"github.com/fiorix/go-web/autogzip"
	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
)

import (
	"skia.googlesource.com/buildbot.git/go/buildbot"
	"skia.googlesource.com/buildbot.git/go/common"
	"skia.googlesource.com/buildbot.git/go/database"
	"skia.googlesource.com/buildbot.git/go/gitinfo"
	"skia.googlesource.com/buildbot.git/go/influxdb"
	"skia.googlesource.com/buildbot.git/go/login"
	"skia.googlesource.com/buildbot.git/go/metadata"
	"skia.googlesource.com/buildbot.git/go/skiaversion"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/status/go/commit_cache"
)

const (
	DEFAULT_COMMITS_TO_LOAD = 50
)

var (
	gitInfo         *gitinfo.GitInfo          = nil
	commitCache     *commit_cache.CommitCache = nil
	commitsTemplate *template.Template        = nil
	dbClient        *influxdb.Client          = nil
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

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", string(300))
		fileServer.ServeHTTP(w, r)
	}
}

func commitsJsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Case 1: Requesting specific commit range by index.
	startIdx, err := getIntParam("start", r)
	if err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Invalid parameter: %v", err))
		return
	}
	if startIdx != nil {
		endIdx := commitCache.NumCommits()
		end, err := getIntParam("end", r)
		if err != nil {
			util.ReportError(w, r, err, fmt.Sprintf("Invalid parameter: %v", err))
			return
		}
		if end != nil {
			endIdx = *end
		}
		if err := commitCache.RangeAsJson(w, *startIdx, endIdx); err != nil {
			util.ReportError(w, r, err, fmt.Sprintf("Failed to load commit range from cache: %v", err))
			return
		}
		return
	}
	// Case 2: Requesting N (or the default number) commits.
	commitsToLoad := DEFAULT_COMMITS_TO_LOAD
	n, err := getIntParam("n", r)
	if err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Invalid parameter: %v", err))
		return
	}
	if n != nil {
		commitsToLoad = *n
	}
	if err := commitCache.LastNAsJson(w, commitsToLoad); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to load commits from cache: %v", err))
		return
	}
}

func addBuildCommentHandler(w http.ResponseWriter, r *http.Request) {
	if !userHasEditRights(r) {
		util.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
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
	defer r.Body.Close()
	c := buildbot.BuildComment{
		BuildId:   int(buildId),
		User:      login.LoggedInAs(r),
		Timestamp: float64(time.Now().UTC().Unix()),
		Message:   comment.Comment,
	}
	cache := commitCache.BuildCache()
	build, err := cache.Get(int(buildId))
	if err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to add comment: %v", err))
		return
	}
	build.Comments = append(build.Comments, &c)
	if err := build.ReplaceIntoDB(); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to add comment: %v", err))
		return
	}
	if err := cache.Update(); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to add comment: %v", err))
		return
	}
}

func addBuilderStatusHandler(w http.ResponseWriter, r *http.Request) {
	if !userHasEditRights(r) {
		util.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
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
	defer r.Body.Close()

	s := buildbot.BuilderStatus{
		Builder:       builder,
		User:          login.LoggedInAs(r),
		Timestamp:     float64(time.Now().UTC().Unix()),
		Flaky:         status.Flaky,
		IgnoreFailure: status.IgnoreFailure,
		Message:       status.Comment,
	}
	if _, err := s.InsertIntoDB(); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to add builder status: %v", err))
		return
	}
	if err := commitCache.BuildCache().Update(); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to refresh cache after adding status: %v", err))
		return
	}
}

func addCommitCommentHandler(w http.ResponseWriter, r *http.Request) {
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
	defer r.Body.Close()

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
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}

	if err := commitsTemplate.Execute(w, struct{}{}); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

func makeHandler(f func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path
		if r.URL.RawQuery != "" {
			url += "?" + r.URL.RawQuery
		}
		glog.Infof("%s: %s from %s", r.Method, url, r.RemoteAddr)
		f(w, r)
	}
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(makeHandler(autogzip.HandleFunc(makeResourceHandler())))
	r.HandleFunc("/", makeHandler(commitsHandler))
	builds := r.PathPrefix("/json/builds/{buildId:[0-9]+}").Subrouter()
	builds.HandleFunc("/comments", makeHandler(addBuildCommentHandler)).Methods("POST")
	builders := r.PathPrefix("/json/builders/{builder}").Subrouter()
	builders.HandleFunc("/status", makeHandler(addBuilderStatusHandler)).Methods("POST")
	commits := r.PathPrefix("/json/commits").Subrouter()
	commits.HandleFunc("/", makeHandler(commitsJsonHandler))
	commits.HandleFunc("/{commit:[a-f0-9]+}/comments", makeHandler(addCommitCommentHandler)).Methods("POST")
	r.HandleFunc("/json/version", makeHandler(skiaversion.JsonHandler))
	r.HandleFunc("/oauth2callback/", makeHandler(login.OAuth2CallbackHandler))
	r.HandleFunc("/logout/", makeHandler(login.LogoutHandler))
	r.HandleFunc("/loginstatus/", makeHandler(login.StatusHandler))
	http.Handle("/", r)
	glog.Infof("Ready to serve on %s", serverURL)
	glog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	// Setup flags.
	database.SetupFlags(buildbot.PROD_DB_HOST, buildbot.PROD_DB_PORT, database.USER_RW, buildbot.PROD_DB_NAME)
	influxdb.SetupFlags()

	common.InitWithMetrics("status", graphiteServer)
	v := skiaversion.GetVersion()
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
	var err error
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
	login.Init(clientID, clientSecret, redirectURL, cookieSalt)

	gitInfo, err = gitinfo.CloneOrUpdate("https://skia.googlesource.com/skia.git", path.Join(*workdir, "skia"), true)
	if err != nil {
		glog.Fatalf("Failed to check out Skia: %v", err)
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

	// Create the commit cache.
	commitCache, err = commit_cache.New(gitInfo, path.Join(*workdir, "commit_cache.gob"), DEFAULT_COMMITS_TO_LOAD)
	if err != nil {
		glog.Fatalf("Failed to create commit cache: %v", err)
	}
	glog.Info("commit_cache complete")
	runServer(serverURL)
}
