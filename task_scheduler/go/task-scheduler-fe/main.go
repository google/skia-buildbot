package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/datastore"
	"cloud.google.com/go/pubsub"
	"github.com/gorilla/mux"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
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
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/blacklist"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	// APP_NAME is the name of this app.
	APP_NAME = "task_scheduler"

	// PubSub subscriber ID used for GitStore.
	GITSTORE_SUBSCRIBER_ID = "task-scheduler-fe"
)

var (
	// Task Scheduler database.
	tsDb db.DBCloser

	// Task Scheduler blacklist.
	bl *blacklist.Blacklist

	// Git repo objects.
	repos repograph.Map

	// Swarming API client.
	swarm swarming.ApiClient

	// Task cfg cache.
	taskCfgCache *task_cfg_cache.TaskCfgCache

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
	firestoreInstance = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	gitstoreTable     = flag.String("gitstore_bt_table", "git-repos2", "BigTable table used for GitStore.")
	local             = flag.Bool("local", false, "Whether we're running on a dev machine vs in production.")
	repoUrls          = common.NewMultiStringFlag("repo", nil, "Repositories for which to schedule tasks.")
	resourcesDir      = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank, assumes you're running inside a checkout and will attempt to find the resources relative to this source file.")
	swarmingServer    = flag.String("swarming_server", swarming.SWARMING_SERVER, "Which Swarming server to use.")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
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
	if err := mainTemplate.Execute(w, struct {
		Data string
	}{
		Data: "",
	}); err != nil {
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
	rules := bl.GetRules()
	enc, err := json.Marshal(&struct {
		Rules []*blacklist.Rule `json:"rules"`
	}{
		Rules: rules,
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
	page := struct{}{}
	if err := triggerTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, r, err, "Failed to execute template.")
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
			httputils.ReportError(w, r, err, fmt.Sprintf("Failed to decode request body: %s", err))
			return
		}
		defer util.Close(r.Body)
		if err := bl.RemoveRule(msg.Id); err != nil {
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
			rangeRule, err := blacklist.NewCommitRangeRule(context.Background(), rule.Name, rule.AddedBy, rule.Description, rule.TaskSpecPatterns, rule.Commits[0], rule.Commits[1], repos)
			if err != nil {
				httputils.ReportError(w, r, err, fmt.Sprintf("Failed to create commit range rule: %s", err))
				return
			}
			rule = *rangeRule
		}
		if err := bl.AddRule(&rule, repos); err != nil {
			httputils.ReportError(w, r, err, fmt.Sprintf("Failed to add blacklist rule: %s", err))
			return
		}
	}
	resp := &struct {
		Rules []*blacklist.Rule `json:"rules"`
	}{
		Rules: bl.GetRules(),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode response: %s", err))
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
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to decode request body: %s", err))
		return
	}
	jobs := make([]*types.Job, 0, len(msg))
	for _, j := range msg {
		_, repoName, _, err := repos.FindCommit(j.Commit)
		if err != nil {
			httputils.ReportError(w, r, err, "Unable to find the given commit in any repo.")
			return
		}
		job, err := makeJob(r.Context(), repoName, j.Commit, j.Name)
		if err != nil {
			httputils.ReportError(w, r, err, "Failed to trigger jobs.")
			return
		}
		jobs = append(jobs, job)
	}
	if err := tsDb.PutJobs(jobs); err != nil {
		httputils.ReportError(w, r, err, "Failed to insert jobs.")
		return
	}
	ids := make([]string, 0, len(jobs))
	for _, job := range jobs {
		ids = append(ids, job.Id)
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
		httputils.ReportError(w, r, nil, "Job ID is required.")
		return
	}

	// Retrieve the Job from the DB.
	job, err := tsDb.GetJobById(id)
	if err != nil {
		if err == db.ErrNotFound {
			http.Error(w, "Unknown Job", 404)
			return
		}
		httputils.ReportError(w, r, err, "Error retrieving Job.")
		return
	}

	// Retrieve the task specs, so that we can include the task dimensions
	// in the results.
	cfg, err := taskCfgCache.Get(r.Context(), job.RepoState)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve Tasks cfg")
		return
	}
	dimsByTask := make(map[string][]string, len(job.Dependencies))
	for taskName := range job.Dependencies {
		taskSpec, ok := cfg.Tasks[taskName]
		if !ok {
			httputils.ReportError(w, r, fmt.Errorf("Job %s (%s) points to unknown task %q at repo state: %+v", job.Id, job.Name, taskName, job.RepoState), "Failed to retrieve Tasks cfg")
			return
		}
		dimsByTask[taskName] = taskSpec.Dimensions
	}

	// Encode the response.
	if err := json.NewEncoder(w).Encode(struct {
		*types.Job
		TaskDimensions map[string][]string `json:"taskDimensions"`
	}{
		Job:            job,
		TaskDimensions: dimsByTask,
	}); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode response.")
		return
	}
}

func jsonCancelJobHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id, ok := mux.Vars(r)["id"]
	if !ok {
		httputils.ReportError(w, r, nil, "Job ID is required.")
		return
	}

	job, err := tsDb.GetJobById(id)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve job.")
		return
	}
	if job.Done() {
		err := fmt.Errorf("Job %s is already finished with status %s", id, job.Status)
		httputils.ReportError(w, r, err, err.Error())
		return
	}
	job.Finished = time.Now()
	job.Status = types.JOB_STATUS_CANCELED
	if err := tsDb.PutJob(job); err != nil {
		httputils.ReportError(w, r, err, "Failed to insert Job")
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
		httputils.ReportError(w, r, nil, "Job ID is required.")
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
		httputils.ReportError(w, r, err, "Failed to execute template.")
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
		httputils.ReportError(w, r, nil, "Job ID is required.")
		return
	}

	job, err := tsDb.GetJobById(jobId)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve Job.")
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
				httputils.ReportError(w, r, err, "Failed to retrieve Task.")
				return
			}
			swarmingTask, err := swarm.GetTask(task.SwarmingTaskId, true)
			if err != nil {
				httputils.ReportError(w, r, err, "Failed to retrieve Swarming task.")
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
		httputils.ReportError(w, r, err, "Failed to encode JSON.")
		return
	}
	if err := jobTimelineTemplate.Execute(w, struct {
		Data string
	}{
		Data: string(enc),
	}); err != nil {
		httputils.ReportError(w, r, err, "Failed to execute template.")
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
		httputils.ReportError(w, r, nil, "Task ID is required.")
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
		httputils.ReportError(w, r, err, "Failed to execute template.")
		return
	}
}

func jsonGetTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id, ok := mux.Vars(r)["id"]
	if !ok {
		httputils.ReportError(w, r, nil, "Task ID is required.")
		return
	}

	task, err := tsDb.GetTaskById(id)
	if err != nil {
		if err == db.ErrNotFound {
			http.Error(w, "Unknown Task", 404)
			return
		}
		httputils.ReportError(w, r, err, "Error retrieving Job.")
		return
	}
	if err := json.NewEncoder(w).Encode(task); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode response.")
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

// jsonTaskSearchHandler allows searching for Tasks based on various parameters.
func jsonTaskSearchHandler(w http.ResponseWriter, r *http.Request) {
	var params db.TaskSearchParams
	if err := httputils.ParseFormValues(r, &params); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse request parameters.")
		return
	}
	tasks, err := db.SearchTasks(tsDb, &params)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to search for tasks.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode response: %s", err))
		return
	}
}

// jsonTaskCandidateSearchHandler allows for searching task candidates based on
// their TaskKey.
// TODO(borenet): Re-enable this if/when candidates have their own DB.
/*func jsonTaskCandidateSearchHandler(w http.ResponseWriter, r *http.Request) {
	var params scheduling.TaskCandidateSearchTerms
	if err := httputils.ParseFormValues(r, &params); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse request parameters.")
		return
	}
	candidates := ts.SearchQueue(&params)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(candidates); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode response: %s", err))
		return
	}
}*/

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
	r.HandleFunc("/job/{id}/timeline", jobTimelineHandler)
	r.HandleFunc("/jobs/search", jobSearchHandler)
	r.HandleFunc("/task/{id}", taskHandler)
	r.HandleFunc("/trigger", triggerHandler)
	r.HandleFunc("/json/blacklist", login.RestrictEditorFn(jsonBlacklistHandler)).Methods(http.MethodPost, http.MethodDelete)
	r.HandleFunc("/json/job/{id}", jsonJobHandler)
	r.HandleFunc("/json/job/{id}/cancel", login.RestrictEditorFn(jsonCancelJobHandler)).Methods(http.MethodPost)
	r.HandleFunc("/json/jobs/search", jsonJobSearchHandler)
	r.HandleFunc("/json/task/{id}", jsonGetTaskHandler)
	// TODO(borenet): Re-enable this if/when candidates have their own DB.
	//r.HandleFunc("/json/taskCandidates/search", jsonTaskCandidateSearchHandler)
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

	// Set up token source and authenticated API clients.
	tokenSource, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT, auth.SCOPE_READ_WRITE, pubsub.ScopePubSub, datastore.ScopeDatastore, bigtable.Scope, swarming.AUTH_SCOPE)
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

	// Start up the web server.
	login.SimpleInitMust(*port, *local)

	go runServer(serverURL)

	// Run indefinitely, responding to HTTP requests.
	select {}
}
