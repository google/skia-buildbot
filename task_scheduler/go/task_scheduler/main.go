package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path"
	"path/filepath"
	"runtime"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/blacklist"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
	"go.skia.org/infra/task_scheduler/go/scheduling"
)

const (
	// APP_NAME is the name of this app.
	APP_NAME = "task_scheduler"

	// DB_NAME is the name of the database.
	DB_NAME = "task_scheduler_db"

	// DB_FILENAME is the name of the file in which the database is stored.
	DB_FILENAME = "task_scheduler.bdb"
)

var (
	// "Constants"

	// REPOS are the repositories to query.
	REPOS = []string{
		common.REPO_SKIA,
		common.REPO_SKIA_INFRA,
	}

	// Task Scheduler instance.
	ts *scheduling.TaskScheduler

	// Git repo objects.
	repos *gitinfo.RepoMap

	// HTML templates.
	blacklistTemplate *template.Template = nil
	mainTemplate      *template.Template = nil
	triggerTemplate   *template.Template = nil

	// Flags.
	host           = flag.String("host", "localhost", "HTTP service host")
	port           = flag.String("port", ":8000", "HTTP service port for the web server (e.g., ':8000')")
	dbPort         = flag.String("db_port", ":8008", "HTTP service port for the database RPC server (e.g., ':8008')")
	local          = flag.Bool("local", false, "Whether we're running on a dev machine vs in production.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank, assumes you're running inside a checkout and will attempt to find the resources relative to this source file.")
	scoreDecay24Hr = flag.Float64("scoreDecay24Hr", 0.9, "Task candidate scores are penalized using linear time decay. This is the desired value after 24 hours. Setting it to 1.0 causes commits not to be prioritized according to commit time.")
	timePeriod     = flag.String("timePeriod", "4d", "Time period to use.")
	workdir        = flag.String("workdir", "workdir", "Working directory to use.")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
)

func reloadTemplates() {
	// Change the current working directory to two directories up from this source file so that we
	// can read templates and serve static (res/) files.
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	blacklistTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/blacklist.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/footer.html"),
	))
	mainTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/main.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/footer.html"),
	))
	triggerTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/trigger.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/footer.html"),
	))
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}
	if err := mainTemplate.Execute(w, ts.Status()); err != nil {
		httputils.ReportError(w, r, err, "Failed to execute template.")
		return
	}
}

func blacklistHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}
	t, c := ts.RecentTaskSpecsAndCommits()
	rulesMap := ts.GetBlacklist().Rules
	rules := make([]*blacklist.Rule, 0, len(rulesMap))
	for _, r := range rulesMap {
		rules = append(rules, r)
	}
	enc, err := json.Marshal(&struct {
		Commits   []string
		Rules     []*blacklist.Rule
		TaskSpecs []string
	}{
		Commits:   c,
		Rules:     rules,
		TaskSpecs: t,
	})
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to encode JSON.")
		return
	}
	if err := blacklistTemplate.Execute(w, struct {
		Data string
	}{
		Data: string(enc),
	}); err != nil {
		httputils.ReportError(w, r, err, "Failed to execute template.")
		return
	}
}

func triggerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}
	t, c := ts.RecentTaskSpecsAndCommits()
	page := struct {
		TaskSpecs []string
		Commits   []string
	}{
		TaskSpecs: t,
		Commits:   c,
	}
	if err := triggerTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, r, err, "Failed to execute template.")
		return
	}
}

func jsonBlacklistHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !login.IsGoogler(r) {
		errStr := "Cannot modify the blacklist; user is not a logged-in Googler."
		httputils.ReportError(w, r, fmt.Errorf(errStr), errStr)
		return
	}

	if r.Method == http.MethodDelete {
		var msg struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			httputils.ReportError(w, r, err, fmt.Sprintf("Failed to decode request body: %s", err))
			return
		}
		defer util.Close(r.Body)
		if err := ts.GetBlacklist().RemoveRule(msg.Name); err != nil {
			httputils.ReportError(w, r, err, fmt.Sprintf("Failed to delete blacklist rule: %s", err))
			return
		}
	} else if r.Method == http.MethodPost {
		var rule blacklist.Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			httputils.ReportError(w, r, err, fmt.Sprintf("Failed to decode request body: %s", err))
			return
		}
		defer util.Close(r.Body)
		rule.AddedBy = login.LoggedInAs(r)
		if len(rule.Commits) == 2 {
			rangeRule, err := blacklist.NewCommitRangeRule(rule.Name, rule.AddedBy, rule.Description, rule.TaskSpecPatterns, rule.Commits[0], rule.Commits[1], repos)
			if err != nil {
				httputils.ReportError(w, r, err, fmt.Sprintf("Failed to create commit range rule: %s", err))
				return
			}
			rule = *rangeRule
		}
		if err := ts.GetBlacklist().AddRule(&rule, repos); err != nil {
			httputils.ReportError(w, r, err, fmt.Sprintf("Failed to add blacklist rule: %s", err))
			return
		}
	}
	if err := json.NewEncoder(w).Encode(ts.GetBlacklist()); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode response: %s", err))
		return
	}
}

func jsonTriggerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !login.IsGoogler(r) {
		errStr := "Cannot trigger tasks; user is not a logged-in Googler."
		httputils.ReportError(w, r, fmt.Errorf(errStr), errStr)
		return
	}

	var msg struct {
		Repo      string   `json:"repo"`
		TaskSpecs []string `json:"task_specs"`
		Commit    string   `json:"commit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to decode request body: %s", err))
		return
	}
	defer util.Close(r.Body)
	for _, t := range msg.TaskSpecs {
		if err := ts.Trigger(t, msg.Repo, msg.Commit); err != nil {
			httputils.ReportError(w, r, err, "Failed to trigger tasks.")
			return
		}
	}
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/blacklist", blacklistHandler)
	r.HandleFunc("/trigger", triggerHandler)
	r.HandleFunc("/json/blacklist", jsonBlacklistHandler).Methods(http.MethodPost, http.MethodDelete)
	r.HandleFunc("/json/trigger", jsonTriggerHandler).Methods(http.MethodPost)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)

	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	glog.Infof("Ready to serve on %s", serverURL)
	glog.Fatal(http.ListenAndServe(*port, nil))
}

// runDbServer listens on dbPort and responds to HTTP requests at path /db with
// RPC calls to taskDb. Does not return.
func runDbServer(taskDb db.RemoteDB) {
	r := mux.NewRouter()
	err := remote_db.RegisterServer(taskDb, r.PathPrefix("/db").Subrouter())
	if err != nil {
		glog.Fatal(err)
	}
	glog.Fatal(http.ListenAndServe(*dbPort, httputils.LoggingGzipRequestResponse(r)))
}

func main() {
	defer common.LogPanic()

	// Global init.
	common.InitWithMetrics2(APP_NAME, influxHost, influxUser, influxPassword, influxDatabase, local)

	reloadTemplates()

	v, err := skiaversion.GetVersion()
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Version %s, built at %s", v.Commit, v.Date)

	// Parse the time period.
	period, err := human.ParseDuration(*timePeriod)
	if err != nil {
		glog.Fatal(err)
	}

	// Authenticated HTTP client.
	oauthCacheFile := path.Join(*workdir, "google_storage_token.data")
	httpClient, err := auth.NewClient(*local, oauthCacheFile, swarming.AUTH_SCOPE)
	if err != nil {
		glog.Fatal(err)
	}

	// Initialize Isolate client.
	isolateClient, err := isolate.NewClient(*workdir)
	if err != nil {
		glog.Fatal(err)
	}
	if *local {
		isolateClient.ServerUrl = isolate.FAKE_SERVER_URL
	}

	// Initialize the database.
	// TODO(benjaminwagner): Create a signal handler which closes the DB.
	d, err := local_db.NewDB(DB_NAME, path.Join(*workdir, DB_FILENAME))
	if err != nil {
		glog.Fatal(err)
	}
	defer util.Close(d)

	// Git repos.
	repos = gitinfo.NewRepoMap(*workdir)
	for _, r := range REPOS {
		if _, err := repos.Repo(r); err != nil {
			glog.Fatal(err)
		}
	}

	// Initialize Swarming client.
	var swarm swarming.ApiClient
	if *local {
		swarmTestClient := swarming.NewTestClient()
		swarmTestClient.MockBots(mockSwarmingBotsForAllTasksForTesting(repos))
		go periodicallyUpdateMockTasksForTesting(swarmTestClient)
		swarm = swarmTestClient
	} else {
		swarm, err = swarming.NewApiClient(httpClient)
		if err != nil {
			glog.Fatal(err)
		}
	}

	// Create and start the task scheduler.
	glog.Infof("Creating task scheduler.")
	ts, err = scheduling.NewTaskScheduler(d, period, *workdir, REPOS, isolateClient, swarm, *scoreDecay24Hr)
	if err != nil {
		glog.Fatal(err)
	}

	glog.Infof("Created task scheduler. Starting loop.")
	ts.Start()

	// Start up the web server.
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	var redirectURL = serverURL + "/oauth2callback/"
	if err := login.InitFromMetadataOrJSON(redirectURL, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST); err != nil {
		glog.Fatal(err)
	}

	go runServer(serverURL)
	go runDbServer(d)

	// Run indefinitely, responding to HTTP requests.
	select {}
}
