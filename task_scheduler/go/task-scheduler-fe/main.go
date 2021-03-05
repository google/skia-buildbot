package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/cors"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/datastore"
	"cloud.google.com/go/pubsub"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	gs_pubsub "go.skia.org/infra/go/gitstore/pubsub"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/rpc"
	"go.skia.org/infra/task_scheduler/go/skip_tasks"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	// APP_NAME is the name of this app.
	APP_NAME = "task-scheduler-fe"

	// PubSub subscriber ID used for GitStore.
	GITSTORE_SUBSCRIBER_ID = APP_NAME
)

var (
	// Task Scheduler database.
	tsDb db.DBCloser

	// Tasks to skip.
	skipTasks *skip_tasks.DB

	// Git repo objects.
	repos repograph.Map

	// Swarming API client.
	swarm swarming.ApiClient

	// Task cfg cache.
	taskCfgCache *task_cfg_cache.TaskCfgCache

	// HTML templates.
	skipTasksTemplate   *template.Template = nil
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
	firestoreInstance = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	gitstoreTable     = flag.String("gitstore_bt_table", "git-repos2", "BigTable table used for GitStore.")
	local             = flag.Bool("local", false, "Whether we're running on a dev machine vs in production.")
	repoUrls          = common.NewMultiStringFlag("repo", nil, "Repositories for which to schedule tasks.")
	resourcesDir      = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank, assumes you're running inside a checkout and will attempt to find the resources relative to this source file.")
	swarmingServer    = flag.String("swarming_server", swarming.SWARMING_SERVER, "Which Swarming server to use.")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

func reloadTemplates() {
	if *resourcesDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			sklog.Fatal(err)
		}
		*resourcesDir = filepath.Join(filepath.Dir(wd), "dist")
	}
	jobTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "job.html"),
	))
	jobSearchTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "job_search.html"),
	))
	jobTimelineTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "job_timeline.html"),
	))
	mainTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "index.html"),
	))
	skipTasksTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "skip_tasks.html"),
	))
	taskTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "task.html"),
	))
	triggerTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "job_trigger.html"),
	))
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}
	if err := mainTemplate.Execute(w, nil); err != nil {
		httputils.ReportError(w, err, "Failed to execute template.", http.StatusInternalServerError)
		return
	}
}

func skipTasksHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}
	rules := skipTasks.GetRules()
	enc, err := json.Marshal(&struct {
		Rules []*skip_tasks.Rule `json:"rules"`
	}{
		Rules: rules,
	})
	if err != nil {
		httputils.ReportError(w, err, "Failed to encode JSON.", http.StatusInternalServerError)
		return
	}
	if err := skipTasksTemplate.Execute(w, struct {
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

// makeJob creates a Job for the given repo, revision, and name.
func makeJob(ctx context.Context, repo, revision, jobName string) (*types.Job, error) {
	j, err := taskCfgCache.MakeJob(ctx, types.RepoState{
		Repo:     repo,
		Revision: revision,
	}, jobName)
	if err != nil {
		return nil, err
	}
	j.Requested = j.Created
	j.IsForce = true
	sklog.Infof("Created manually-triggered Job %q", j.Id)
	return j, nil
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
		Servers:  []string{gerrit.GerritSkiaURL},
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
	if err := jobTimelineTemplate.Execute(w, struct {
		JobId string
	}{
		JobId: jobId,
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

func googleVerificationHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte("google-site-verification: google2c59f97e1ced9fdc.html")); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to write response: %s", err), http.StatusInternalServerError)
		return
	}
}

// addCorsMiddleware wraps the specified HTTP handler with a handler that applies the
// CORS specification on the request, and adds relevant CORS headers as necessary.
// This is needed for some handlers that do not have this middleware. Eg: the twirp
// handler (https://github.com/twitchtv/twirp/issues/210).
func addCorsMiddleware(handler http.Handler) http.Handler {
	corsWrapper := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		Debug:          true,
	})
	return corsWrapper.Handler(handler)
}

func runServer(serverURL string, srv http.Handler) {
	r := mux.NewRouter()
	r.HandleFunc("/", mainHandler)
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))))
	r.PathPrefix(rpc.TaskSchedulerServicePathPrefix).Handler(addCorsMiddleware(srv))
	r.HandleFunc("/skip_tasks", skipTasksHandler)
	r.HandleFunc("/job/{id}", jobHandler)
	r.HandleFunc("/job/{id}/timeline", jobTimelineHandler)
	r.HandleFunc("/jobs/search", jobSearchHandler)
	r.HandleFunc("/task/{id}", taskHandler)
	r.HandleFunc("/trigger", triggerHandler)
	r.HandleFunc("/google2c59f97e1ced9fdc.html", googleVerificationHandler)
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)

	h := httputils.LoggingRequestResponse(r)
	h = httputils.XFrameOptionsDeny(h)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}
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

	ctx, cancelFn := context.WithCancel(context.Background())
	cleanup.AtExit(cancelFn)

	// Set up token source and authenticated API clients.
	// TODO(borenet): Should we create a new service account with fewer
	// permissions?
	tokenSource, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL, pubsub.ScopePubSub, datastore.ScopeDatastore, bigtable.Scope, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatalf("Failed to create token source: %s", err)
	}

	// Initialize the database.
	tsDb, err = firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, tokenSource)
	if err != nil {
		sklog.Fatalf("Failed to create Firestore DB client: %s", err)
	}
	cleanup.AtExit(func() {
		util.Close(tsDb)
	})

	// Skip tasks DB.
	skipTasks, err = skip_tasks.NewWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, tokenSource)
	if err != nil {
		sklog.Fatal(err)
	}
	skipTasks.AutoUpdate(ctx)

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

	// Task Cfg Cache.
	taskCfgCache, err = task_cfg_cache.NewTaskCfgCache(ctx, repos, *btProject, *btInstance, tokenSource)
	if err != nil {
		sklog.Fatal(err)
	}

	// Initialize Swarming client.
	cfg := httputils.DefaultClientConfig().WithTokenSource(tokenSource).WithDialTimeout(time.Minute).With2xxOnly()
	cfg.RequestTimeout = time.Minute
	swarm, err = swarming.NewApiClient(cfg.Client(), *swarmingServer)
	if err != nil {
		sklog.Fatal(err)
	}

	// Auto-update the git repos.
	if err := autoUpdateRepos.Start(ctx, GITSTORE_SUBSCRIBER_ID, tokenSource, 5*time.Minute, func(_ context.Context, _ string, _ *repograph.Graph, ack, _ func()) error {
		ack()
		return nil
	}); err != nil {
		sklog.Fatal(err)
	}

	var viewAllow allowed.Allow = nil
	editAllow := allowed.Googlers()
	adminAllow := allowed.Googlers()
	srv := rpc.NewTaskSchedulerServer(ctx, tsDb, repos, skipTasks, taskCfgCache, swarm, viewAllow, editAllow, adminAllow)
	if err != nil {
		sklog.Fatal(err)
	}

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}
	login.InitWithAllow(serverURL+login.DEFAULT_OAUTH2_CALLBACK, adminAllow, editAllow, viewAllow)

	// Start up the web server.
	login.SimpleInitMust(*port, *local)

	go runServer(serverURL, srv)

	// Run indefinitely, responding to HTTP requests.
	select {}
}
