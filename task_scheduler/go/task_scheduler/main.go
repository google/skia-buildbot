package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/datastore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	gs_pubsub "go.skia.org/infra/go/gitstore/pubsub"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/periodic"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/blacklist"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/scheduling"
	"go.skia.org/infra/task_scheduler/go/testutils"
	"go.skia.org/infra/task_scheduler/go/tryjobs"
	"go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const (
	// APP_NAME is the name of this app.
	APP_NAME = "task_scheduler"

	PUBSUB_SUBSCRIBER_TASK_SCHEDULER          = "task-scheduler"
	PUBSUB_SUBSCRIBER_TASK_SCHEDULER_INTERNAL = "task-scheduler-internal"

	// PubSub subscriber ID used for GitStore.
	GITSTORE_SUBSCRIBER_ID = "task-scheduler"
)

var (
	// Task Scheduler instance.
	ts *scheduling.TaskScheduler

	// Task Scheduler database.
	tsDb db.DBCloser

	// Task Scheduler blacklist.
	bl *blacklist.Blacklist

	// Git repo objects.
	repos repograph.Map

	// Swarming API client.
	swarm swarming.ApiClient

	// HTML templates.
	blacklistTemplate   *template.Template = nil
	jobTemplate         *template.Template = nil
	jobSearchTemplate   *template.Template = nil
	jobTimelineTemplate *template.Template = nil
	mainTemplate        *template.Template = nil
	taskTemplate        *template.Template = nil
	triggerTemplate     *template.Template = nil

	// Flags.
	btInstance        = flag.String("bigtable_instance", "", "BigTable instance to use.")
	btProject         = flag.String("bigtable_project", "", "GCE project to use for BigTable.")
	host              = flag.String("host", "localhost", "HTTP service host")
	port              = flag.String("port", ":8000", "HTTP service port for the web server (e.g., ':8000')")
	dbPort            = flag.String("db_port", ":8008", "HTTP service port for the database RPC server (e.g., ':8008')")
	disableTryjobs    = flag.Bool("disable_try_jobs", false, "If set, no try jobs will be picked up.")
	firestoreInstance = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	gitstoreTable     = flag.String("gitstore_bt_table", "git-repos2", "BigTable table used for GitStore.")
	isolateServer     = flag.String("isolate_server", isolate.ISOLATE_SERVER_URL, "Which Isolate server to use.")
	local             = flag.Bool("local", false, "Whether we're running on a dev machine vs in production.")
	repoUrls          = common.NewMultiStringFlag("repo", nil, "Repositories for which to schedule tasks.")
	recipesCfgFile    = flag.String("recipes_cfg", "", "Path to the recipes.cfg file.")
	resourcesDir      = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank, assumes you're running inside a checkout and will attempt to find the resources relative to this source file.")
	scoreDecay24Hr    = flag.Float64("scoreDecay24Hr", 0.9, "Task candidate scores are penalized using linear time decay. This is the desired value after 24 hours. Setting it to 1.0 causes commits not to be prioritized according to commit time.")
	swarmingPools     = common.NewMultiStringFlag("pool", nil, "Which Swarming pools to use.")
	swarmingServer    = flag.String("swarming_server", swarming.SWARMING_SERVER, "Which Swarming server to use.")
	timePeriod        = flag.String("timeWindow", "4d", "Time period to use.")
	tryJobBucket      = flag.String("tryjob_bucket", tryjobs.BUCKET_PRIMARY, "Which Buildbucket bucket to use for try jobs.")
	commitWindow      = flag.Int("commitWindow", 10, "Minimum number of recent commits to keep in the timeWindow.")
	diagnosticsBucket = flag.String("diagnostics_bucket", "skia-task-scheduler-diagnostics", "Name of Google Cloud Storage bucket to use for diagnostics data.")
	workdir           = flag.String("workdir", "workdir", "Working directory to use.")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")

	pubsubTopicName      = flag.String("pubsub_topic", swarming.PUBSUB_TOPIC_SWARMING_TASKS, "Pub/Sub topic to use for Swarming tasks.")
	pubsubSubscriberName = flag.String("pubsub_subscriber", PUBSUB_SUBSCRIBER_TASK_SCHEDULER, "Pub/Sub subscriber name.")
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
	jobSearchTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/job_search.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/footer.html"),
	))
	jobTimelineTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/job_timeline.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/footer.html"),
	))
	mainTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/main.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/footer.html"),
	))
	taskTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/task.html"),
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
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}
	enc, err := json.Marshal(ts.Status())
	if err != nil {
		httputils.ReportError(w, err, "Failed to encode JSON.", http.StatusInternalServerError)
		return
	}
	if err := mainTemplate.Execute(w, struct {
		Data string
	}{
		Data: string(enc),
	}); err != nil {
		httputils.ReportError(w, err, "Failed to execute template.", http.StatusInternalServerError)
		return
	}
}

func blacklistHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}
	rules := ts.GetBlacklist().GetRules()
	enc, err := json.Marshal(&struct {
		Rules []*blacklist.Rule `json:"rules"`
	}{
		Rules: rules,
	})
	if err != nil {
		httputils.ReportError(w, err, "Failed to encode JSON.", http.StatusInternalServerError)
		return
	}
	if err := blacklistTemplate.Execute(w, struct {
		Data string
	}{
		Data: string(enc),
	}); err != nil {
		httputils.ReportError(w, err, "Failed to execute template.", http.StatusInternalServerError)
		return
	}
}

func triggerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}
	page := struct{}{}
	if err := triggerTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, err, "Failed to execute template.", http.StatusInternalServerError)
		return
	}
}

func jsonBlacklistHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodDelete {
		var msg struct {
			Id string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Failed to decode request body: %s", err), http.StatusInternalServerError)
			return
		}
		defer util.Close(r.Body)
		if err := ts.GetBlacklist().RemoveRule(msg.Id); err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Failed to delete blacklist rule: %s", err), http.StatusInternalServerError)
			return
		}
	} else if r.Method == http.MethodPost {
		var rule blacklist.Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Failed to decode request body: %s", err), http.StatusInternalServerError)
			return
		}
		defer util.Close(r.Body)
		rule.AddedBy = login.LoggedInAs(r)
		if len(rule.Commits) == 2 {
			rangeRule, err := blacklist.NewCommitRangeRule(context.Background(), rule.Name, rule.AddedBy, rule.Description, rule.TaskSpecPatterns, rule.Commits[0], rule.Commits[1], repos)
			if err != nil {
				httputils.ReportError(w, err, fmt.Sprintf("Failed to create commit range rule: %s", err), http.StatusInternalServerError)
				return
			}
			rule = *rangeRule
		}
		if err := ts.GetBlacklist().AddRule(&rule, repos); err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Failed to add blacklist rule: %s", err), http.StatusInternalServerError)
			return
		}
	}
	resp := &struct {
		Rules []*blacklist.Rule `json:"rules"`
	}{
		Rules: ts.GetBlacklist().GetRules(),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode response: %s", err), http.StatusInternalServerError)
		return
	}
}

func jsonTriggerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "OPTIONS" {
		return
	}
	var msg []struct {
		Name   string `json:"name"`
		Commit string `json:"commit"`
	}
	defer util.Close(r.Body)
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to decode request body: %s", err), http.StatusInternalServerError)
		return
	}
	ids := make([]string, 0, len(msg))
	for _, j := range msg {
		_, repoName, _, err := repos.FindCommit(j.Commit)
		if err != nil {
			httputils.ReportError(w, err, "Unable to find the given commit in any repo.", http.StatusInternalServerError)
			return
		}
		id, err := ts.TriggerJob(context.Background(), repoName, j.Commit, j.Name)
		if err != nil {
			httputils.ReportError(w, err, "Failed to trigger jobs.", http.StatusInternalServerError)
			return
		}
		ids = append(ids, id)
	}
	if err := json.NewEncoder(w).Encode(ids); err != nil {
		httputils.ReportError(w, err, "Failed to encode response.", http.StatusInternalServerError)
		return
	}
}

func jsonJobHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id, ok := mux.Vars(r)["id"]
	if !ok {
		httputils.ReportError(w, nil, "Job ID is required.", http.StatusInternalServerError)
		return
	}

	job, dimsByTask, err := ts.GetJob(r.Context(), id)
	if err != nil {
		if err == db.ErrNotFound {
			http.Error(w, "Unknown Job", 404)
			return
		}
		httputils.ReportError(w, err, "Error retrieving Job.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(struct {
		*types.Job
		TaskDimensions map[string][]string `json:"taskDimensions"`
	}{
		Job:            job,
		TaskDimensions: dimsByTask,
	}); err != nil {
		httputils.ReportError(w, err, "Failed to encode response.", http.StatusInternalServerError)
		return
	}
}

func jsonCancelJobHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id, ok := mux.Vars(r)["id"]
	if !ok {
		httputils.ReportError(w, nil, "Job ID is required.", http.StatusInternalServerError)
		return
	}

	job, err := ts.CancelJob(id)
	if err != nil {
		httputils.ReportError(w, err, "Failed to cancel job.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(job); err != nil {
		httputils.ReportError(w, err, "Failed to encode response.", http.StatusInternalServerError)
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
		httputils.ReportError(w, nil, "Job ID is required.", http.StatusInternalServerError)
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
		httputils.ReportError(w, err, "Failed to execute template.", http.StatusInternalServerError)
		return
	}
}

func jobSearchHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}

	page := struct {
		Repos    []string          `json:"recent_repos"`
		Servers  []string          `json:"recent_servers"`
		Statuses []types.JobStatus `json:"valid_statuses"`
	}{
		Repos:    *repoUrls,
		Servers:  []string{gerrit.GERRIT_SKIA_URL},
		Statuses: types.VALID_JOB_STATUSES,
	}
	if err := jobSearchTemplate.Execute(w, &page); err != nil {
		httputils.ReportError(w, err, "Failed to execute template.", http.StatusInternalServerError)
		return
	}
}

func jobTimelineHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}

	jobId, ok := mux.Vars(r)["id"]
	if !ok {
		httputils.ReportError(w, nil, "Job ID is required.", http.StatusInternalServerError)
		return
	}

	job, err := tsDb.GetJobById(jobId)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve Job.", http.StatusInternalServerError)
		return
	}
	type unifiedTask struct {
		*types.Task
		Swarming *swarming_api.SwarmingRpcsTaskResult `json:"swarming"`
	}
	var tasks = make([]*unifiedTask, 0, len(job.Tasks))
	for _, summaries := range job.Tasks {
		for _, t := range summaries {
			task, err := tsDb.GetTaskById(t.Id)
			if err != nil {
				httputils.ReportError(w, err, "Failed to retrieve Task.", http.StatusInternalServerError)
				return
			}
			swarmingTask, err := swarm.GetTask(task.SwarmingTaskId, true)
			if err != nil {
				httputils.ReportError(w, err, "Failed to retrieve Swarming task.", http.StatusInternalServerError)
				return
			}
			tasks = append(tasks, &unifiedTask{
				Task:     task,
				Swarming: swarmingTask,
			})
		}
	}
	enc, err := json.Marshal(&struct {
		Job    *types.Job     `json:"job"`
		Tasks  []*unifiedTask `json:"tasks"`
		Epochs []time.Time    `json:"epochs"`
	}{
		Job:    job,
		Tasks:  tasks,
		Epochs: []time.Time{}, // TODO(borenet): Record tick timestamps.
	})
	if err != nil {
		httputils.ReportError(w, err, "Failed to encode JSON.", http.StatusInternalServerError)
		return
	}
	if err := jobTimelineTemplate.Execute(w, struct {
		Data string
	}{
		Data: string(enc),
	}); err != nil {
		httputils.ReportError(w, err, "Failed to execute template.", http.StatusInternalServerError)
		return
	}
}

func taskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}

	id, ok := mux.Vars(r)["id"]
	if !ok {
		httputils.ReportError(w, nil, "Task ID is required.", http.StatusInternalServerError)
		return
	}

	page := struct {
		TaskId         string
		SwarmingServer string
	}{
		TaskId:         id,
		SwarmingServer: *swarmingServer,
	}
	if err := taskTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, err, "Failed to execute template.", http.StatusInternalServerError)
		return
	}
}

func jsonGetTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id, ok := mux.Vars(r)["id"]
	if !ok {
		httputils.ReportError(w, nil, "Task ID is required.", http.StatusInternalServerError)
		return
	}

	task, err := ts.GetTask(id)
	if err != nil {
		if err == db.ErrNotFound {
			http.Error(w, "Unknown Task", 404)
			return
		}
		httputils.ReportError(w, err, "Error retrieving Job.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(task); err != nil {
		httputils.ReportError(w, err, "Failed to encode response.", http.StatusInternalServerError)
		return
	}
}

// jsonJobSearchHandler allows for searching Jobs based on various parameters.
func jsonJobSearchHandler(w http.ResponseWriter, r *http.Request) {
	var params db.JobSearchParams
	if err := httputils.ParseFormValues(r, &params); err != nil {
		httputils.ReportError(w, err, "Failed to parse request parameters.", http.StatusInternalServerError)
		return
	}
	jobs, err := db.SearchJobs(tsDb, &params)
	if err != nil {
		httputils.ReportError(w, err, "Failed to search for jobs.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(jobs); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode response: %s", err), http.StatusInternalServerError)
		return
	}
}

// jsonTaskSearchHandler allows searching for Tasks based on various parameters.
func jsonTaskSearchHandler(w http.ResponseWriter, r *http.Request) {
	var params db.TaskSearchParams
	if err := httputils.ParseFormValues(r, &params); err != nil {
		httputils.ReportError(w, err, "Failed to parse request parameters.", http.StatusInternalServerError)
		return
	}
	tasks, err := db.SearchTasks(tsDb, &params)
	if err != nil {
		httputils.ReportError(w, err, "Failed to search for tasks.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode response: %s", err), http.StatusInternalServerError)
		return
	}
}

// jsonTaskCandidateSearchHandler allows for searching task candidates based on
// their TaskKey.
func jsonTaskCandidateSearchHandler(w http.ResponseWriter, r *http.Request) {
	var params scheduling.TaskCandidateSearchTerms
	if err := httputils.ParseFormValues(r, &params); err != nil {
		httputils.ReportError(w, err, "Failed to parse request parameters.", http.StatusInternalServerError)
		return
	}
	candidates := ts.SearchQueue(&params)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(candidates); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode response: %s", err), http.StatusInternalServerError)
		return
	}
}

func googleVerificationHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte("google-site-verification: google2c59f97e1ced9fdc.html")); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to write response: %s", err), http.StatusInternalServerError)
		return
	}
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/blacklist", blacklistHandler)
	r.HandleFunc("/job/{id}", jobHandler)
	r.HandleFunc("/job/{id}/timeline", jobTimelineHandler)
	r.HandleFunc("/jobs/search", jobSearchHandler)
	r.HandleFunc("/task/{id}", taskHandler)
	r.HandleFunc("/trigger", triggerHandler)
	r.HandleFunc("/json/blacklist", login.RestrictEditorFn(jsonBlacklistHandler)).Methods(http.MethodPost, http.MethodDelete)
	r.HandleFunc("/json/job/{id}", jsonJobHandler)
	r.HandleFunc("/json/job/{id}/cancel", login.RestrictEditorFn(jsonCancelJobHandler)).Methods(http.MethodPost)
	r.HandleFunc("/json/jobs/search", jsonJobSearchHandler)
	r.HandleFunc("/json/task/{id}", jsonGetTaskHandler)
	r.HandleFunc("/json/taskCandidates/search", jsonTaskCandidateSearchHandler)
	r.HandleFunc("/json/tasks/search", jsonTaskSearchHandler)
	r.HandleFunc("/json/trigger", login.RestrictEditorFn(jsonTriggerHandler)).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/google2c59f97e1ced9fdc.html", googleVerificationHandler)
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)

	sklog.AddLogsRedirect(r)
	h := httputils.LoggingGzipRequestResponse(r)
	h = httputils.HealthzAndHTTPS(h)
	http.Handle("/", h)
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {

	// Global init.
	common.InitWithMust(
		APP_NAME,
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	defer common.Defer()

	reloadTemplates()

	skiaversion.MustLogVersion()

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}
	login.InitWithAllow(serverURL+login.DEFAULT_OAUTH2_CALLBACK, allowed.Googlers(), allowed.Googlers(), nil)

	ctx, cancelFn := context.WithCancel(context.Background())
	cleanup.AtExit(cancelFn)

	// Get the absolute workdir.
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Set up token source and authenticated API clients.
	isolateServerUrl := *isolateServer
	if *local {
		isolateServerUrl = isolate.ISOLATE_SERVER_URL_FAKE
	}
	var isolateClient *isolate.Client
	var tokenSource oauth2.TokenSource
	gitcookiesPath := "/tmp/.gitcookies"
	tokenSource, err = auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT, auth.SCOPE_READ_WRITE, pubsub.ScopePubSub, datastore.ScopeDatastore, bigtable.Scope, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatalf("Failed to create token source: %s", err)
	}
	isolateClient, err = isolate.NewClientWithServiceAccount(wdAbs, isolateServerUrl, os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if err != nil {
		sklog.Fatal(err)
	}
	if _, err := gitauth.New(tokenSource, gitcookiesPath, true, ""); err != nil {
		sklog.Fatalf("Failed to create git cookie updater: %s", err)
	}

	// Authenticated HTTP client.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(tokenSource).With2xxOnly().Client()

	// Gerrit API client.
	gerrit, err := gerrit.NewGerrit(gerrit.GERRIT_SKIA_URL, gitcookiesPath, nil)
	if err != nil {
		sklog.Fatal(err)
	}

	// Initialize the database.
	tsDb, err = firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, tokenSource)
	if err != nil {
		sklog.Fatalf("Failed to create Firestore DB client: %s", err)
	}
	cleanup.AtExit(func() {
		util.Close(tsDb)
	})

	// Blacklist DB.
	bl, err = blacklist.NewWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, tokenSource)
	if err != nil {
		sklog.Fatal(err)
	}

	// Git repos.
	if *repoUrls == nil {
		sklog.Fatal("--repo is required.")
	}
	btConf := &bt_gitstore.BTConfig{
		ProjectID:  *btProject,
		InstanceID: *btInstance,
		TableID:    *gitstoreTable,
		AppProfile: "task-scheduler",
	}
	autoUpdateRepos, err := gs_pubsub.NewAutoUpdateMap(ctx, *repoUrls, btConf)
	if err != nil {
		sklog.Fatal(err)
	}
	repos = autoUpdateRepos.Map

	// Initialize Swarming client.
	if *local {
		wd := filepath.Join(wdAbs, "local_git_repos")
		gitRepos := make(map[string]*git.Repo, len(repos))
		for url := range repos {
			r, err := git.NewRepo(ctx, url, wd)
			if err != nil {
				sklog.Fatal(err)
			}
			gitRepos[url] = r
		}
		swarmTestClient := testutils.NewTestClient()
		swarmTestClient.MockBots(testutils.MockSwarmingBotsForAllTasksForTesting(ctx, gitRepos))
		go testutils.PeriodicallyUpdateMockTasksForTesting(swarmTestClient)
		swarm = swarmTestClient
	} else {
		cfg := httputils.DefaultClientConfig().WithTokenSource(tokenSource).WithDialTimeout(time.Minute).With2xxOnly()
		cfg.RequestTimeout = time.Minute
		swarm, err = swarming.NewApiClient(cfg.Client(), *swarmingServer)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	storageClient, err := storage.NewClient(ctx, option.WithTokenSource(tokenSource))
	diagClient := gcsclient.New(storageClient, *diagnosticsBucket)
	diagInstance := *firestoreInstance

	// Find depot_tools.
	if *recipesCfgFile == "" {
		*recipesCfgFile = path.Join(wdAbs, "recipes.cfg")
	}
	depotTools, err := depot_tools.Sync(ctx, wdAbs, *recipesCfgFile)
	if err != nil {
		sklog.Fatal(err)
	}

	// Parse the time period.
	period, err := human.ParseDuration(*timePeriod)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create and start the task scheduler.
	sklog.Infof("Creating task scheduler.")
	ts, err = scheduling.NewTaskScheduler(ctx, tsDb, bl, period, *commitWindow, wdAbs, serverURL, repos, isolateClient, swarm, httpClient, *scoreDecay24Hr, tryjobs.API_URL_PROD, *tryJobBucket, common.PROJECT_REPO_MAPPING, *swarmingPools, *pubsubTopicName, depotTools, gerrit, *btProject, *btInstance, tokenSource, diagClient, diagInstance)
	if err != nil {
		sklog.Fatal(err)
	}
	cleanup.AtExit(func() {
		util.LogErr(ts.Close())
	})
	if err := swarming.InitPubSub(*pubsubTopicName, *pubsubSubscriberName, ts.HandleSwarmingPubSub); err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Created task scheduler. Starting loop.")
	ts.Start(ctx, !*disableTryjobs, func() {})
	if err := autoUpdateRepos.Start(ctx, GITSTORE_SUBSCRIBER_ID, tokenSource, 5*time.Minute, ts.HandleRepoUpdate); err != nil {
		sklog.Fatal(err)
	}

	// Set up periodic triggers.
	if err := periodic.Listen(ctx, fmt.Sprintf("task-scheduler-%s", *firestoreInstance), tokenSource, func(ctx context.Context, name, id string) bool {
		if err := ts.MaybeTriggerPeriodicJobs(ctx, name); err != nil {
			sklog.Errorf("Failed to trigger periodic jobs; will retry later: %s", err)
			return false // We will retry later.
		}
		return true
	}); err != nil {
		sklog.Fatal(err)
	}

	// Start up the web server.
	login.SimpleInitMust(*port, *local)

	go runServer(serverURL)

	// Run indefinitely, responding to HTTP requests.
	select {}
}
