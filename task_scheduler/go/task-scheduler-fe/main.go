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

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/datastore"
	"cloud.google.com/go/pubsub"
	"github.com/go-chi/chi/v5"
	"github.com/rs/cors"
	"golang.org/x/oauth2/google"

	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/proxylogin"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	gs_pubsub "go.skia.org/infra/go/gitstore/pubsub"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	swarmingv2 "go.skia.org/infra/go/swarming/v2"
	"go.skia.org/infra/go/tracing"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/job_creation/buildbucket_taskbackend"
	"go.skia.org/infra/task_scheduler/go/rpc"
	"go.skia.org/infra/task_scheduler/go/skip_tasks"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

const (
	// APP_NAME is the name of this app.
	APP_NAME = "task-scheduler-fe"

	// PubSub subscriber ID used for GitStore.
	GITSTORE_SUBSCRIBER_ID = APP_NAME
)

var (
	// Tasks to skip.
	skipTasks *skip_tasks.DB

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
	buildbucketTarget = flag.String("buildbucket_target", "", "Target name used by Buildbucket to address this Task Scheduler.")
	commitWindow      = flag.Int("commitWindow", 10, "Minimum number of recent commits to keep in the timeWindow.")
	debugPort         = flag.String("debug_port", "", "HTTP service port for debugging using pprof")
	host              = flag.String("host", "localhost", "HTTP service host")
	port              = flag.String("port", ":8000", "HTTP service port for the web server (e.g., ':8000')")
	firestoreInstance = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	gitstoreTable     = flag.String("gitstore_bt_table", "git-repos2", "BigTable table used for GitStore.")
	local             = flag.Bool("local", false, "Whether we're running on a dev machine vs in production.")
	repoUrls          = common.NewMultiStringFlag("repo", nil, "Repositories for which to schedule tasks.")
	resourcesDir      = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank, assumes you're running inside a checkout and will attempt to find the resources relative to this source file.")
	swarmingServer    = flag.String("swarming_server", swarming.SWARMING_SERVER, "Which Swarming server to use.")
	timePeriod        = flag.String("timeWindow", "4d", "Time period to use for cache expiration.")
	tracingProject    = flag.String("tracing_project", "", "GCP project where traces should be uploaded.")
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

func jobHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}

	id := chi.URLParam(r, "id")
	if id == "" {
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

	jobId := chi.URLParam(r, "id")
	if jobId == "" {
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

	id := chi.URLParam(r, "id")
	if id == "" {
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

func runServer(serverURL string, srv, bbHandler http.Handler, plogin alogin.Login) {
	r := chi.NewRouter()
	r.HandleFunc("/", mainHandler)
	r.Handle("/dist/*", http.StripPrefix("/dist/", http.HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))))
	r.Handle(rpc.TaskSchedulerServicePathPrefix+"*", addCorsMiddleware(srv))
	r.HandleFunc("/skip_tasks", skipTasksHandler)
	r.HandleFunc("/job/{id}", jobHandler)
	r.HandleFunc("/job/{id}/timeline", jobTimelineHandler)
	r.HandleFunc("/jobs/search", jobSearchHandler)
	r.HandleFunc("/task/{id}", taskHandler)
	r.HandleFunc("/trigger", triggerHandler)
	r.HandleFunc("/google2c59f97e1ced9fdc.html", googleVerificationHandler)
	r.HandleFunc("/res/*", httputils.MakeResourceHandler(*resourcesDir))
	r.HandleFunc("/_/login/status", alogin.LoginStatusHandler(plogin))
	if bbHandler != nil {
		r.Handle("/prpc/*", alogin.ForceRole(bbHandler, plogin, roles.Buildbucket))
	}

	h := httputils.LoggingRequestResponse(r)
	h = httputils.XFrameOptionsDeny(h)
	h = alogin.StatusMiddleware(plogin)(h)
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
	)
	defer common.Defer()

	reloadTemplates()

	if *tracingProject != "" {
		if err := tracing.Initialize(0.1, *tracingProject, nil); err != nil {
			sklog.Fatalf("Could not set up tracing: %s", err)
		}
	}
	ctx, cancelFn := context.WithCancel(context.Background())
	cleanup.AtExit(cancelFn)

	// Set up token source and authenticated API clients.
	// TODO(borenet): Should we create a new service account with fewer
	// permissions?
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, pubsub.ScopePubSub, datastore.ScopeDatastore, bigtable.Scope, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatalf("Failed to create token source: %s", err)
	}

	// Initialize the database.
	tsDb, err := firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, tokenSource)
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
	repos := autoUpdateRepos.Map

	// Task Cfg Cache.
	taskCfgCache, err := task_cfg_cache.NewTaskCfgCache(ctx, repos, *btProject, *btInstance, tokenSource)
	if err != nil {
		sklog.Fatal(err)
	}
	period, err := human.ParseDuration(*timePeriod)
	if err != nil {
		sklog.Fatal(err)
	}
	w, err := window.New(ctx, period, *commitWindow, repos)
	if err != nil {
		sklog.Fatal(err)
	}
	// Periodically clean up the taskCfgCache.
	go util.RepeatCtx(ctx, 30*time.Minute, func(ctx context.Context) {
		if err := w.Update(ctx); err != nil {
			sklog.Errorf("Failed to update time window: %s", err)
			return
		}
		if err := taskCfgCache.Cleanup(ctx, now.Now(ctx).Sub(w.EarliestStart())); err != nil {
			sklog.Errorf("Failed to clean up task cfg cache: %s", err)
		}
	})

	// Initialize Swarming client.
	cfg := httputils.DefaultClientConfig().WithTokenSource(tokenSource).WithDialTimeout(time.Minute).With2xxOnly()
	httpClient := cfg.Client()
	swarm := swarmingv2.NewDefaultClient(httpClient, *swarmingServer)

	// Auto-update the git repos.
	if err := autoUpdateRepos.Start(ctx, GITSTORE_SUBSCRIBER_ID, tokenSource, 5*time.Minute, func(_ context.Context, _ string, _ *repograph.Graph, ack, _ func()) error {
		ack()
		return nil
	}); err != nil {
		sklog.Fatal(err)
	}
	plogin := proxylogin.NewWithDefaults()

	srv := rpc.NewTaskSchedulerServer(ctx, tsDb, repos, skipTasks, taskCfgCache, swarm, plogin)
	if err != nil {
		sklog.Fatal(err)
	}

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	// Initialize Buildbucket TaskBackend.
	var bbHandler http.Handler
	if *buildbucketTarget != "" {
		httpClient := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()
		bb2 := buildbucket.NewClient(httpClient)
		bbHandler = buildbucket_taskbackend.Handler(*buildbucketTarget, serverURL, common.PROJECT_REPO_MAPPING, tsDb, bb2)
	}

	go runServer(serverURL, srv, bbHandler, plogin)

	if *debugPort != "" {
		go httputils.ServePprof(*debugPort)
	}

	// Run indefinitely, responding to HTTP requests.
	select {}
}
