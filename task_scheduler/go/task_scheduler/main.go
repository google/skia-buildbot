package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/net/context"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/webhook"
	"go.skia.org/infra/task_scheduler/go/blacklist"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"go.skia.org/infra/task_scheduler/go/db/recovery"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
	"go.skia.org/infra/task_scheduler/go/scheduling"
	"go.skia.org/infra/task_scheduler/go/tryjobs"
)

const (
	// APP_NAME is the name of this app.
	APP_NAME = "task_scheduler"
)

var (
	// "Constants"

	// PROJECT_REPO_MAPPING is a mapping of project names to repo URLs.
	PROJECT_REPO_MAPPING = map[string]string{
		"buildbot":      common.REPO_SKIA_INFRA,
		"internal_test": common.REPO_SKIA_INTERNAL_TEST,
		"skia":          common.REPO_SKIA,
		"skiabuildbot":  common.REPO_SKIA_INFRA,
	}

	// Task Scheduler instance.
	ts *scheduling.TaskScheduler

	// Task Scheduler database.
	tsDb db.BackupDBCloser

	// Git repo objects.
	repos repograph.Map

	// HTML templates.
	blacklistTemplate *template.Template = nil
	jobTemplate       *template.Template = nil
	mainTemplate      *template.Template = nil
	triggerTemplate   *template.Template = nil

	// Flags.
	host           = flag.String("host", "localhost", "HTTP service host")
	port           = flag.String("port", ":8000", "HTTP service port for the web server (e.g., ':8000')")
	dbPort         = flag.String("db_port", ":8008", "HTTP service port for the database RPC server (e.g., ':8008')")
	isolateServer  = flag.String("isolate_server", isolate.ISOLATE_SERVER_URL, "Which Isolate server to use.")
	local          = flag.Bool("local", false, "Whether we're running on a dev machine vs in production.")
	repoUrls       = common.NewMultiStringFlag("repo", nil, "Repositories for which to schedule tasks.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank, assumes you're running inside a checkout and will attempt to find the resources relative to this source file.")
	scoreDecay24Hr = flag.Float64("scoreDecay24Hr", 0.9, "Task candidate scores are penalized using linear time decay. This is the desired value after 24 hours. Setting it to 1.0 causes commits not to be prioritized according to commit time.")
	swarmingPools  = common.NewMultiStringFlag("pool", swarming.POOLS_PUBLIC, "Which Swarming pools to use.")
	swarmingServer = flag.String("swarming_server", swarming.SWARMING_SERVER, "Which Swarming server to use.")
	timePeriod     = flag.String("timeWindow", "4d", "Time period to use.")
	tryJobBucket   = flag.String("tryjob_bucket", tryjobs.BUCKET_PRIMARY, "Which Buildbucket bucket to use for try jobs.")
	commitWindow   = flag.Int("commitWindow", 10, "Minimum number of recent commits to keep in the timeWindow.")
	gsBucket       = flag.String("gsBucket", "skia-task-scheduler", "Name of Google Cloud Storage bucket to use for backups and recovery.")
	workdir        = flag.String("workdir", "workdir", "Working directory to use.")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")

	pubsubTopicName      = flag.String("pubsub_topic", scheduling.PUBSUB_TOPIC_SWARMING_TASKS, "Pub/Sub topic to use for Swarming tasks.")
	pubsubSubscriberName = flag.String("pubsub_subscriber", scheduling.PUBSUB_SUBSCRIBER_TASK_SCHEDULER, "Pub/Sub subscriber name.")
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
	jobTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/job.html"),
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
	_, t, c := ts.RecentSpecsAndCommits()
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
	j, _, c := ts.RecentSpecsAndCommits()
	page := struct {
		JobSpecs []string
		Commits  []string
	}{
		JobSpecs: j,
		Commits:  c,
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
	w.Header().Set("Access-Control-Allow-Origin", "https://status.skia.org")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Add("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	if r.Method == "OPTIONS" {
		return
	}
	if !login.IsGoogler(r) {
		errStr := "Cannot trigger tasks; user is not a logged-in Googler."
		httputils.ReportError(w, r, fmt.Errorf(errStr), errStr)
		return
	}

	var msg []struct {
		Name   string `json:"name"`
		Commit string `json:"commit"`
	}
	defer util.Close(r.Body)
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to decode request body: %s", err))
		return
	}
	ids := make([]string, 0, len(msg))
	for _, j := range msg {
		_, repoName, _, err := repos.FindCommit(j.Commit)
		if err != nil {
			httputils.ReportError(w, r, err, "Unable to find the given commit in any repo.")
			return
		}
		id, err := ts.TriggerJob(repoName, j.Commit, j.Name)
		if err != nil {
			httputils.ReportError(w, r, err, "Failed to trigger jobs.")
			return
		}
		ids = append(ids, id)
	}
	if err := json.NewEncoder(w).Encode(ids); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode response.")
		return
	}
}

func jsonJobHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id, ok := mux.Vars(r)["id"]
	if !ok {
		err := "Job ID is required."
		httputils.ReportError(w, r, fmt.Errorf(err), err)
		return
	}

	job, err := ts.GetJob(id)
	if err != nil {
		if err == db.ErrNotFound {
			http.Error(w, fmt.Sprintf("Unknown Job %q", id), 404)
			return
		}
		httputils.ReportError(w, r, err, "Error retrieving Job.")
		return
	}
	if err := json.NewEncoder(w).Encode(job); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode response.")
		return
	}
}

func jsonCancelJobHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !login.IsGoogler(r) {
		errStr := "Cannot cancel jobs; user is not a logged-in Googler."
		httputils.ReportError(w, r, fmt.Errorf(errStr), errStr)
		return
	}

	id, ok := mux.Vars(r)["id"]
	if !ok {
		err := "Job ID is required."
		httputils.ReportError(w, r, fmt.Errorf(err), err)
		return
	}

	job, err := ts.CancelJob(id)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to cancel job.")
		return
	}
	if err := json.NewEncoder(w).Encode(job); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode response.")
		return
	}
}

func jobHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}

	id, ok := mux.Vars(r)["id"]
	if !ok {
		err := "Job ID is required."
		httputils.ReportError(w, r, fmt.Errorf(err), err)
		return
	}

	page := struct {
		JobId          string
		SwarmingServer string
	}{
		JobId:          id,
		SwarmingServer: *swarmingServer,
	}
	if err := jobTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, r, err, "Failed to execute template.")
		return
	}
}

// jsonTaskHandler parses a Task as JSON from the request and calls
// TaskScheduler.ValidateAnd(Add|Update)Task, returning the updated Task as
// JSON.
func jsonTaskHandler(w http.ResponseWriter, r *http.Request) {
	data, err := webhook.AuthenticateRequest(r)
	if err != nil {
		if data == nil {
			httputils.ReportError(w, r, err, "Failed to read request")
			return
		}
		if !login.IsAdmin(r) {
			httputils.ReportError(w, r, err, "Failed authentication")
			return
		}
	}

	var task db.Task
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&task); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to decode request body: %s", err))
		return
	}

	if r.Method == http.MethodPost {
		if err := ts.ValidateAndAddTask(&task); err != nil {
			httputils.ReportError(w, r, err, fmt.Sprintf("Failed to add task: %s", err))
			return
		}
	} else {
		if err := ts.ValidateAndUpdateTask(&task); err != nil {
			httputils.ReportError(w, r, err, fmt.Sprintf("Failed to update task: %s", err))
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(task); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode response: %s", err))
		return
	}
}

// jsonJobSearchHandler allows for searching Jobs based on various parameters.
func jsonJobSearchHandler(w http.ResponseWriter, r *http.Request) {
	var params db.JobSearchParams
	if err := httputils.ParseFormValues(r, &params); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse request parameters.")
		return
	}
	jobs, err := db.SearchJobs(tsDb, &params)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to search for jobs.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(jobs); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode response: %s", err))
		return
	}
}

func googleVerificationHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte("google-site-verification: google2c59f97e1ced9fdc.html")); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to write response: %s", err))
		return
	}

}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/blacklist", blacklistHandler)
	r.HandleFunc("/job/{id}", jobHandler)
	r.HandleFunc("/trigger", triggerHandler)
	r.HandleFunc("/json/blacklist", jsonBlacklistHandler).Methods(http.MethodPost, http.MethodDelete)
	r.HandleFunc("/json/job/{id}", jsonJobHandler)
	r.HandleFunc("/json/job/{id}/cancel", jsonCancelJobHandler).Methods(http.MethodPost)
	r.HandleFunc("/json/jobs/search", jsonJobSearchHandler)
	r.HandleFunc("/json/task", jsonTaskHandler).Methods(http.MethodPost, http.MethodPut)
	r.HandleFunc("/json/trigger", jsonTriggerHandler).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/google2c59f97e1ced9fdc.html", googleVerificationHandler)
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)

	scheduling.RegisterPubSubServer(ts, r)

	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

// runDbServer listens on dbPort and responds to HTTP requests at path /db with
// RPC calls to taskDb. Does not return.
func runDbServer(taskDb db.RemoteDB) {
	r := mux.NewRouter()
	err := remote_db.RegisterServer(taskDb, r.PathPrefix("/db").Subrouter())
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Fatal(http.ListenAndServe(*dbPort, httputils.LoggingGzipRequestResponse(r)))
}

func main() {
	defer common.LogPanic()

	// Global init.
	common.InitWithMust(
		APP_NAME,
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)

	reloadTemplates()

	v, err := skiaversion.GetVersion()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Version %s, built at %s", v.Commit, v.Date)

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	// Parse the time period.
	period, err := human.ParseDuration(*timePeriod)
	if err != nil {
		sklog.Fatal(err)
	}

	// Get the absolute workdir.
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Authenticated HTTP client.
	oauthCacheFile := path.Join(wdAbs, "google_storage_token.data")
	httpClient, err := auth.NewClient(*local, oauthCacheFile, auth.SCOPE_READ_WRITE)
	if err != nil {
		sklog.Fatal(err)
	}

	// Initialize Isolate client.
	isolateServerUrl := *isolateServer
	if *local {
		isolateServerUrl = isolate.ISOLATE_SERVER_URL_FAKE
	}
	isolateClient, err := isolate.NewClient(wdAbs, isolateServerUrl)
	if err != nil {
		sklog.Fatal(err)
	}

	// Initialize the database.
	// TODO(benjaminwagner): Create a signal handler which closes the DB.
	tsDb, err = local_db.NewDB(local_db.DB_NAME, path.Join(wdAbs, local_db.DB_FILENAME))
	if err != nil {
		sklog.Fatal(err)
	}
	defer util.Close(tsDb)

	// Git repos.
	if *repoUrls == nil {
		*repoUrls = common.PUBLIC_REPOS
	}
	repos, err = repograph.NewMap(*repoUrls, wdAbs)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := repos.Update(); err != nil {
		sklog.Fatal(err)
	}

	// Initialize Swarming client.
	var swarm swarming.ApiClient
	if *local {
		swarmTestClient := swarming.NewTestClient()
		swarmTestClient.MockBots(mockSwarmingBotsForAllTasksForTesting(repos))
		go periodicallyUpdateMockTasksForTesting(swarmTestClient)
		swarm = swarmTestClient
	} else {
		tp := httputils.NewBackOffTransport().(*httputils.BackOffTransport)
		tp.Transport.Dial = func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, 3*time.Minute)
		}
		swarmClient, err := auth.NewClientWithTransport(*local, oauthCacheFile, "", tp, swarming.AUTH_SCOPE)
		if err != nil {
			sklog.Fatal(err)
		}
		swarm, err = swarming.NewApiClient(swarmClient, *swarmingServer)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Start DB backup.
	if *local && *gsBucket == "skia-task-scheduler" {
		sklog.Fatalf("Specify --gsBucket=dogben-test to run locally.")
	}
	// TODO(benjaminwagner): The storage client library already handles buffering
	// and retrying requests, so we may not want to use BackoffTransport for the
	// httpClient provided to NewDBBackup.
	b, err := recovery.NewDBBackup(ctx, *gsBucket, tsDb, local_db.DB_NAME, wdAbs, httpClient)
	if err != nil {
		sklog.Fatal(err)
	}

	// Find depot_tools.
	depotTools, err := depot_tools.Find()
	if err != nil {
		sklog.Fatal(err)
	}

	// Create and start the task scheduler.
	sklog.Infof("Creating task scheduler.")
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}
	if err := scheduling.InitPubSub(serverURL, *pubsubTopicName, *pubsubSubscriberName); err != nil {
		sklog.Fatal(err)
	}
	ts, err = scheduling.NewTaskScheduler(tsDb, period, *commitWindow, wdAbs, serverURL, repos, isolateClient, swarm, httpClient, *scoreDecay24Hr, tryjobs.API_URL_PROD, *tryJobBucket, PROJECT_REPO_MAPPING, *swarmingPools, *pubsubTopicName, depotTools)
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Created task scheduler. Starting loop.")
	ts.Start(ctx, b.Tick)

	// Start up the web server.
	login.SimpleInitMust(*port, *local)

	if *local {
		webhook.InitRequestSaltForTesting()
	} else {
		webhook.MustInitRequestSaltFromMetadata()
	}

	go runServer(serverURL)
	go runDbServer(tsDb)

	// Run indefinitely, responding to HTTP requests.
	select {}
}
