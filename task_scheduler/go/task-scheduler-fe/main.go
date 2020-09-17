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
	// Change the current working directory to two directories up from this source file so that we
	// can read templates and serve static (res/) files.
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	skipTasksTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/skip_tasks.html"),
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

func jsonSkipTasksHandler(w http.ResponseWriter, r *http.Request) {
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
		if err := skipTasks.RemoveRule(msg.Id); err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Failed to delete skip rule: %s", err), http.StatusInternalServerError)
			return
		}
	} else if r.Method == http.MethodPost {
		var rule skip_tasks.Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Failed to decode request body: %s", err), http.StatusInternalServerError)
			return
		}
		defer util.Close(r.Body)
		rule.AddedBy = login.LoggedInAs(r)
		if len(rule.Commits) == 2 {
			rangeRule, err := skip_tasks.NewCommitRangeRule(context.Background(), rule.Name, rule.AddedBy, rule.Description, rule.TaskSpecPatterns, rule.Commits[0], rule.Commits[1], repos)
			if err != nil {
				httputils.ReportError(w, err, fmt.Sprintf("Failed to create commit range rule: %s", err), http.StatusInternalServerError)
				return
			}
			rule = *rangeRule
		}
		if err := skipTasks.AddRule(&rule, repos); err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Failed to add skip rule: %s", err), http.StatusInternalServerError)
			return
		}
	}
	resp := &struct {
		Rules []*skip_tasks.Rule `json:"rules"`
	}{
		Rules: skipTasks.GetRules(),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode response: %s", err), http.StatusInternalServerError)
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
		httputils.ReportError(w, err, fmt.Sprintf("Failed to decode request body: %s", err), http.StatusInternalServerError)
		return
	}
	jobs := make([]*types.Job, 0, len(msg))
	for _, j := range msg {
		_, repoName, _, err := repos.FindCommit(j.Commit)
		if err != nil {
			httputils.ReportError(w, err, "Unable to find the given commit in any repo.", http.StatusInternalServerError)
			return
		}
		job, err := makeJob(r.Context(), repoName, j.Commit, j.Name)
		if err != nil {
			httputils.ReportError(w, err, "Failed to trigger jobs.", http.StatusInternalServerError)
			return
		}
		jobs = append(jobs, job)
	}
	if err := tsDb.PutJobsInChunks(jobs); err != nil {
		httputils.ReportError(w, err, "Failed to insert jobs.", http.StatusInternalServerError)
		return
	}
	ids := make([]string, 0, len(jobs))
	for _, job := range jobs {
		ids = append(ids, job.Id)
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

	// Retrieve the Job from the DB.
	job, err := tsDb.GetJobById(id)
	if err != nil {
		if err == db.ErrNotFound {
			http.Error(w, "Unknown Job", 404)
			return
		}
		httputils.ReportError(w, err, "Error retrieving Job.", http.StatusInternalServerError)
		return
	}
	if job == nil {
		http.Error(w, "Unknown Job", 404)
		return
	}

	// Retrieve the task specs, so that we can include the task dimensions
	// in the results.
	cfg, err := taskCfgCache.Get(r.Context(), job.RepoState)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve Tasks cfg", http.StatusInternalServerError)
		return
	}
	dimsByTask := make(map[string][]string, len(job.Dependencies))
	for taskName := range job.Dependencies {
		taskSpec, ok := cfg.Tasks[taskName]
		if !ok {
			httputils.ReportError(w, fmt.Errorf("Job %s (%s) points to unknown task %q at repo state: %+v", job.Id, job.Name, taskName, job.RepoState), "Job points to unknown task", http.StatusInternalServerError)
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

	job, err := tsDb.GetJobById(id)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve job.", http.StatusInternalServerError)
		return
	}
	if job.Done() {
		err := fmt.Errorf("Job %s is already finished with status %s", id, job.Status)
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}
	job.Finished = time.Now()
	job.Status = types.JOB_STATUS_CANCELED
	if err := tsDb.PutJob(job); err != nil {
		httputils.ReportError(w, err, "Failed to insert Job", http.StatusInternalServerError)
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

	task, err := tsDb.GetTaskById(id)
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
// TODO(borenet): Re-enable this if/when candidates have their own DB.
/*func jsonTaskCandidateSearchHandler(w http.ResponseWriter, r *http.Request) {
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
}*/

func googleVerificationHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte("google-site-verification: google2c59f97e1ced9fdc.html")); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to write response: %s", err), http.StatusInternalServerError)
		return
	}
}

func runServer(serverURL string, srv http.Handler) {
	r := mux.NewRouter()
	r.HandleFunc("/", httputils.OriginTrial(mainHandler, *local))
	r.HandleFunc("/skip_tasks", httputils.OriginTrial(skipTasksHandler, *local))
	r.HandleFunc("/job/{id}", httputils.OriginTrial(jobHandler, *local))
	r.HandleFunc("/job/{id}/timeline", httputils.OriginTrial(jobTimelineHandler, *local))
	r.HandleFunc("/jobs/search", httputils.OriginTrial(jobSearchHandler, *local))
	r.HandleFunc("/task/{id}", httputils.OriginTrial(taskHandler, *local))
	r.HandleFunc("/trigger", httputils.OriginTrial(triggerHandler, *local))
	r.HandleFunc("/json/skip_tasks", login.RestrictEditorFn(jsonSkipTasksHandler)).Methods(http.MethodPost, http.MethodDelete)
	r.HandleFunc("/json/job/{id}", jsonJobHandler)
	r.HandleFunc("/json/job/{id}/cancel", login.RestrictEditorFn(jsonCancelJobHandler)).Methods(http.MethodPost)
	r.HandleFunc("/json/jobs/search", jsonJobSearchHandler)
	r.HandleFunc("/json/task/{id}", jsonGetTaskHandler)
	// TODO(borenet): Re-enable this if/when candidates have their own DB.
	//r.HandleFunc("/json/taskCandidates/search", jsonTaskCandidateSearchHandler)
	r.HandleFunc("/json/tasks/search", jsonTaskSearchHandler)
	r.HandleFunc("/json/trigger", login.RestrictEditorFn(jsonTriggerHandler)).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/google2c59f97e1ced9fdc.html", googleVerificationHandler)
	r.PathPrefix(rpc.TaskSchedulerServicePathPrefix).Handler(srv)
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)

	h := httputils.LoggingRequestResponse(r)
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
	srv := rpc.NewTaskSchedulerServer(ctx, tsDb, repos, skipTasks, taskCfgCache, viewAllow, editAllow, adminAllow)
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
