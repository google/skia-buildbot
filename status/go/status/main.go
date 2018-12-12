/*
	Provides roll-up statuses for Skia build/test/perf.
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode"

	"cloud.google.com/go/bigtable"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/status/go/capacity"
	"go.skia.org/infra/status/go/incremental"
	"go.skia.org/infra/status/go/lkgr"
	task_driver_db "go.skia.org/infra/task_driver/go/db"
	bigtable_db "go.skia.org/infra/task_driver/go/db/bigtable"
	"go.skia.org/infra/task_driver/go/handlers"
	"go.skia.org/infra/task_driver/go/logs"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"go.skia.org/infra/task_scheduler/go/db/pubsub"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

const (
	DEFAULT_COMMITS_TO_LOAD = 35
	MAX_COMMITS_TO_LOAD     = 100
	SKIA_REPO               = "skia"
	INFRA_REPO              = "infra"

	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"
)

var (
	autorollMtx      sync.RWMutex
	autorollStatus   []byte                        = nil
	capacityClient   *capacity.CapacityClient      = nil
	capacityTemplate *template.Template            = nil
	commitsTemplate  *template.Template            = nil
	iCache           *incremental.IncrementalCache = nil
	lkgrObj          *lkgr.LKGR                    = nil
	taskDb           db.RemoteDB                   = nil
	taskDriverDb     task_driver_db.DB             = nil
	taskDriverLogs   *logs.LogsManager             = nil
	tasksPerCommit   *tasksPerCommitCache          = nil
	tCache           cache.TaskCache               = nil

	// AUTOROLLERS maps roller IDs to their human-friendly display names.
	AUTOROLLERS = map[string]string{
		"android-master-autoroll":   "Android",
		"skia-flutter-autoroll":     "Flutter",
		"skia-autoroll":             "Chrome",
		"google3-autoroll":          "Google3",
		"angle-skia-autoroll":       "ANGLE",
		"skcms-skia-autoroll":       "skcms",
		"swiftshader-skia-autoroll": "SwiftSh",
	}
)

// flags
var (
	capacityRecalculateInterval = flag.Duration("capacity_recalculate_interval", 10*time.Minute, "How often to re-calculate capacity statistics.")
	firestoreInstance           = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"prod\"")
	host                        = flag.String("host", "localhost", "HTTP service host")
	port                        = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	promPort                    = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	repoUrls                    = common.NewMultiStringFlag("repo", nil, "Repositories to query for status.")
	resourcesDir                = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	swarmingUrl                 = flag.String("swarming_url", "https://chromium-swarm.appspot.com", "URL of the Swarming server.")
	taskSchedulerDbUrl          = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
	taskSchedulerUrl            = flag.String("task_scheduler_url", "https://task-scheduler.skia.org", "URL of the Task Scheduler server.")
	testing                     = flag.Bool("testing", false, "Set to true for locally testing rules. No email will be sent.")
	useMetadata                 = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	workdir                     = flag.String("workdir", ".", "Directory to use for scratch work.")
	pubsubTopicTasks            = flag.String("pubsub_topic_tasks", pubsub.TOPIC_TASKS, "Pubsub topic for tasks.")
	pubsubTopicJobs             = flag.String("pubsub_topic_jobs", pubsub.TOPIC_JOBS, "Pubsub topic for jobs.")

	repos repograph.Map
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

func getIntParam(name string, r *http.Request) (*int64, error) {
	raw, ok := r.URL.Query()[name]
	if !ok {
		return nil, nil
	}
	v, err := strconv.ParseInt(raw[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Invalid integer value for parameter %q", name)
	}
	return &v, nil
}

// repoUrlToName returns a short repo nickname given a full repo URL.
func repoUrlToName(repoUrl string) string {
	// Special case: we like "infra" better than "buildbot".
	if repoUrl == common.REPO_SKIA_INFRA {
		return "infra"
	}
	return strings.TrimSuffix(path.Base(repoUrl), ".git")
}

// repoNameToUrl returns a full repo URL given a short nickname, or an error
// if no matching repo URL is found.
func repoNameToUrl(repoName string) (string, error) {
	// Special case: we like "infra" better than "buildbot".
	if repoName == "infra" {
		return common.REPO_SKIA_INFRA, nil
	}
	// Search the list of repos used by this server.
	for _, repoUrl := range *repoUrls {
		if repoUrlToName(repoUrl) == repoName {
			return repoUrl, nil
		}
	}
	return "", fmt.Errorf("No such repo.")
}

// getRepo returns a short repo nickname and a full repo URL based on the URL
// path of the given http.Request.
func getRepo(r *http.Request) (string, string, error) {
	repoPath, _ := mux.Vars(r)["repo"]
	repoUrl, err := repoNameToUrl(repoPath)
	if err != nil {
		return "", "", err
	}
	return repoUrlToName(repoUrl), repoUrl, nil
}

// getRepoNames returns the nicknames for all repos on this server.
func getRepoNames() []string {
	repoNames := make([]string, 0, len(*repoUrls))
	for _, repoUrl := range *repoUrls {
		repoNames = append(repoNames, repoUrlToName(repoUrl))
	}
	return repoNames
}

func commentsForRepoHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "application/json")
	_, repoUrl, err := getRepo(r)
	if err != nil {
		httputils.ReportError(w, r, err, err.Error())
		return
	}
	comments, err := taskDb.GetCommentsForRepos([]string{repoUrl}, time.Now().Add(-10000*time.Hour))
	if err != nil {
		httputils.ReportError(w, r, err, err.Error())
		return
	}
	if err := json.NewEncoder(w).Encode(comments); err != nil {
		sklog.Errorf("Failed to encode comments as JSON: %s", err)
	}
}

func incrementalJsonHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "application/json")
	_, repoUrl, err := getRepo(r)
	if err != nil {
		httputils.ReportError(w, r, err, err.Error())
		return
	}
	from, err := getIntParam("from", r)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Invalid parameter for \"from\": %s", err))
		return
	}
	to, err := getIntParam("to", r)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Invalid parameter for \"to\": %s", err))
		return
	}
	n, err := getIntParam("n", r)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Invalid parameter for \"n\": %s", err))
		return
	}
	numCommits := DEFAULT_COMMITS_TO_LOAD
	if n != nil {
		numCommits = int(*n)
		if numCommits > MAX_COMMITS_TO_LOAD {
			numCommits = MAX_COMMITS_TO_LOAD
		}
	}
	var update *incremental.Update
	if from != nil {
		fromTime := time.Unix(0, (*from)*util.MILLIS_TO_NANOS)
		if to != nil {
			toTime := time.Unix(0, (*to)*util.MILLIS_TO_NANOS)
			update, err = iCache.GetRange(repoUrl, fromTime, toTime, numCommits)
		} else {
			update, err = iCache.Get(repoUrl, fromTime, numCommits)
		}
	} else {
		update, err = iCache.GetAll(repoUrl, numCommits)
	}
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to retrieve updates: %s", err))
		return
	}
	if err := json.NewEncoder(w).Encode(update); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode response: %s", err))
		return
	}
}

func addTaskCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	defer util.Close(r.Body)
	if !userHasEditRights(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")

	id, ok := mux.Vars(r)["id"]
	if !ok {
		httputils.ReportError(w, r, fmt.Errorf("No task ID given!"), "No task ID given!")
		return
	}
	task, err := taskDb.GetTaskById(id)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to obtain task details.")
		return
	}

	comment := struct {
		Comment string `json:"comment"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to add comment: %s", err))
		return
	}
	c := types.TaskComment{
		Repo:      task.Repo,
		Revision:  task.Revision,
		Name:      task.Name,
		Timestamp: time.Now().UTC(),
		TaskId:    task.Id,
		User:      login.LoggedInAs(r),
		Message:   comment.Comment,
	}
	if err := taskDb.PutTaskComment(&c); err != nil {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Failed to add comment: %s", err))
		return
	}
	if err := iCache.Update(context.Background(), false); err != nil {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Failed to update cache: %s", err))
		return
	}
}

func deleteTaskCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if !userHasEditRights(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")

	id, ok := mux.Vars(r)["id"]
	if !ok {
		httputils.ReportError(w, r, fmt.Errorf("No task ID given!"), "No task ID given!")
		return
	}
	task, err := taskDb.GetTaskById(id)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to obtain task details.")
		return
	}
	timestamp, err := strconv.ParseInt(mux.Vars(r)["timestamp"], 10, 64)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Invalid comment id: %v", err))
		return
	}
	c := &types.TaskComment{
		Repo:      task.Repo,
		Revision:  task.Revision,
		Name:      task.Name,
		Timestamp: time.Unix(0, timestamp),
		TaskId:    task.Id,
	}

	if err := taskDb.DeleteTaskComment(c); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to delete comment: %v", err))
		return
	}
	if err := iCache.Update(context.Background(), false); err != nil {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Failed to update cache: %s", err))
		return
	}
}

func addTaskSpecCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if !userHasEditRights(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	taskSpec, ok := mux.Vars(r)["taskSpec"]
	if !ok {
		httputils.ReportError(w, r, nil, "No taskSpec provided!")
		return
	}
	_, repoUrl, err := getRepo(r)
	if err != nil {
		httputils.ReportError(w, r, err, err.Error())
		return
	}

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

	c := types.TaskSpecComment{
		Repo:          repoUrl,
		Name:          taskSpec,
		Timestamp:     time.Now().UTC(),
		User:          login.LoggedInAs(r),
		Flaky:         comment.Flaky,
		IgnoreFailure: comment.IgnoreFailure,
		Message:       comment.Comment,
	}
	if err := taskDb.PutTaskSpecComment(&c); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to add task spec comment: %v", err))
		return
	}
	if err := iCache.Update(context.Background(), false); err != nil {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Failed to update cache: %s", err))
		return
	}
}

func deleteTaskSpecCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if !userHasEditRights(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	taskSpec, ok := mux.Vars(r)["taskSpec"]
	if !ok {
		httputils.ReportError(w, r, nil, "No taskSpec provided!")
		return
	}
	_, repoUrl, err := getRepo(r)
	if err != nil {
		httputils.ReportError(w, r, err, err.Error())
		return
	}
	timestamp, err := strconv.ParseInt(mux.Vars(r)["timestamp"], 10, 64)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Invalid timestamp: %v", err))
		return
	}
	c := types.TaskSpecComment{
		Repo:      repoUrl,
		Name:      taskSpec,
		Timestamp: time.Unix(0, timestamp),
	}
	if err := taskDb.DeleteTaskSpecComment(&c); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to delete comment: %v", err))
		return
	}
	if err := iCache.Update(context.Background(), false); err != nil {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Failed to update cache: %s", err))
		return
	}
}

func addCommitCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if !userHasEditRights(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, repoUrl, err := getRepo(r)
	if err != nil {
		httputils.ReportError(w, r, err, err.Error())
		return
	}
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

	c := types.CommitComment{
		Repo:          repoUrl,
		Revision:      commit,
		Timestamp:     time.Now().UTC(),
		User:          login.LoggedInAs(r),
		IgnoreFailure: comment.IgnoreFailure,
		Message:       comment.Comment,
	}
	if err := taskDb.PutCommitComment(&c); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to add commit comment: %s", err))
		return
	}
	if err := iCache.Update(context.Background(), false); err != nil {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Failed to update cache: %s", err))
		return
	}
}

func deleteCommitCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if !userHasEditRights(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "User does not have edit rights.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, repoUrl, err := getRepo(r)
	if err != nil {
		httputils.ReportError(w, r, err, err.Error())
		return
	}
	commit := mux.Vars(r)["commit"]
	timestamp, err := strconv.ParseInt(mux.Vars(r)["timestamp"], 10, 64)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Invalid comment id: %v", err))
		return
	}
	c := types.CommitComment{
		Repo:      repoUrl,
		Revision:  commit,
		Timestamp: time.Unix(0, timestamp),
	}
	if err := taskDb.DeleteCommitComment(&c); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to delete commit comment: %s", err))
		return
	}
	if err := iCache.Update(context.Background(), false); err != nil {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Failed to update cache: %s", err))
		return
	}
}

type commitsTemplateData struct {
	Repo     string
	Title    string
	RepoBase string
	Repos    []string
}

func defaultRedirectHandler(w http.ResponseWriter, r *http.Request) {
	defaultRepo := repoUrlToName((*repoUrls)[0])
	http.Redirect(w, r, fmt.Sprintf("/repo/%s", defaultRepo), http.StatusFound)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "text/html")

	repoName, repoUrl, err := getRepo(r)
	if err != nil {
		httputils.ReportError(w, r, err, err.Error())
		return
	}

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}

	d := commitsTemplateData{
		Repo:     repoName,
		RepoBase: fmt.Sprintf("%s/+/", repoUrl),
		Repos:    getRepoNames(),
		Title:    fmt.Sprintf("Status: %s", repoName),
	}

	if err := commitsTemplate.Execute(w, d); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
	}
}

func capacityHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}

	page := struct {
		Repos []string
	}{
		Repos: getRepoNames(),
	}
	if err := capacityTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
	}
}

func capacityStatsHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(capacityClient.CapacityMetrics()); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode response: %s", err))
		return
	}
}

// buildProgressHandler returns the number of finished builds at the given
// commit, compared to that of an older commit.
func buildProgressHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get the number of finished tasks for the requested commit.
	hash := r.FormValue("commit")
	if !util.ValidateCommit(hash) {
		httputils.ReportError(w, r, nil, fmt.Sprintf("%q is not a valid commit hash.", hash))
		return
	}
	_, repoUrl, err := getRepo(r)
	if err != nil {
		httputils.ReportError(w, r, err, err.Error())
		return
	}
	tasks, err := tCache.GetTasksForCommits(repoUrl, []string{hash})
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to get the number of finished builds."))
		return
	}
	finished := 0
	for _, byCommit := range tasks {
		for _, t := range byCommit {
			if t.Done() {
				finished++
			}
		}
	}
	tasksForCommit, err := tasksPerCommit.Get(context.Background(), types.RepoState{
		Repo:     repoUrl,
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

func lkgrHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(lkgrObj.Get())); err != nil {
		httputils.ReportError(w, r, err, "Failed to write response.")
		return
	}
}

func autorollStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	autorollMtx.RLock()
	defer autorollMtx.RUnlock()
	if _, err := w.Write(autorollStatus); err != nil {
		httputils.ReportError(w, r, err, "Failed to write response.")
		return
	}
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.HandleFunc("/", defaultRedirectHandler)
	r.HandleFunc("/repo/{repo}", statusHandler)
	r.HandleFunc("/capacity", capacityHandler)
	r.HandleFunc("/capacity/json", capacityStatsHandler)
	r.HandleFunc("/json/autorollers", autorollStatusHandler)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/json/{repo}/buildProgress", buildProgressHandler)
	r.HandleFunc("/lkgr", lkgrHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc(OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	taskComments := r.PathPrefix("/json/tasks/{id}").Subrouter()
	taskComments.HandleFunc("/comments", addTaskCommentHandler).Methods("POST")
	taskComments.HandleFunc("/comments/{timestamp:[0-9]+}", deleteTaskCommentHandler).Methods("DELETE")
	taskSpecs := r.PathPrefix("/json/{repo}/taskSpecs/{taskSpec}").Subrouter()
	taskSpecs.HandleFunc("/comments", addTaskSpecCommentHandler).Methods("POST")
	taskSpecs.HandleFunc("/comments/{timestamp:[0-9]+}", deleteTaskSpecCommentHandler).Methods("DELETE")
	commits := r.PathPrefix("/json/{repo}/commits").Subrouter()
	commits.HandleFunc("/{commit:[a-f0-9]+}/comments", addCommitCommentHandler).Methods("POST")
	commits.HandleFunc("/{commit:[a-f0-9]+}/comments/{timestamp:[0-9]+}", deleteCommitCommentHandler).Methods("DELETE")
	r.HandleFunc("/json/{repo}/incremental", incrementalJsonHandler)
	r.HandleFunc("/json/{repo}/all_comments", commentsForRepoHandler)
	handlers.AddTaskDriverHandlers(r, taskDriverDb, taskDriverLogs)
	sklog.AddLogsRedirect(r)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	// Setup flags.
	common.InitWithMust(
		"status",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)

	skiaversion.MustLogVersion()

	Init()
	if *testing {
		*useMetadata = false
	}
	serverURL := "https://" + *host
	if *testing {
		serverURL = "http://" + *host + *port
	}
	ctx := context.Background()

	ts, err := auth.NewDefaultTokenSource(*testing, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT, bigtable.ReadonlyScope, pubsub.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	c := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Create LKGR object.
	lkgrObj, err = lkgr.New(ctx)
	if err != nil {
		sklog.Fatalf("Failed to create LKGR: %s", err)
	}
	lkgrObj.UpdateLoop(10*time.Minute, ctx)

	// Create remote Tasks DB.
	if *testing {
		taskDb, err = local_db.NewDB("status-testing", path.Join(*workdir, "status-testing.bdb"), nil, nil)
		if err != nil {
			sklog.Fatalf("Failed to create local task DB: %s", err)
		}
		defer util.Close(taskDb.(db.DBCloser))
	} else if *firestoreInstance != "" {
		label := *host
		modTasks, err := pubsub.NewModifiedTasks(*pubsubTopicTasks, label, ts)
		if err != nil {
			sklog.Fatal(err)
		}
		modJobs, err := pubsub.NewModifiedJobs(*pubsubTopicJobs, label, ts)
		if err != nil {
			sklog.Fatal(err)
		}
		taskDb, err = firestore.NewDB(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, ts, modTasks, modJobs)
		if err != nil {
			sklog.Fatalf("Failed to create Firestore DB client: %s", err)
		}
	} else {
		label := *host
		taskDb, err = remote_db.NewClient(*taskSchedulerDbUrl, *pubsubTopicTasks, *pubsubTopicJobs, label, ts)
		if err != nil {
			sklog.Fatalf("Failed to create remote task DB: %s", err)
		}
	}

	login.SimpleInitMust(*port, *testing)

	// Check out source code.
	reposDir := path.Join(*workdir, "repos")
	if err := os.MkdirAll(reposDir, os.ModePerm); err != nil {
		sklog.Fatalf("Failed to create repos dir: %s", err)
	}
	if *repoUrls == nil {
		*repoUrls = common.PUBLIC_REPOS
	}
	repos, err = repograph.NewMap(ctx, *repoUrls, reposDir)
	if err != nil {
		sklog.Fatalf("Failed to create repo map: %s", err)
	}
	if err := repos.Update(ctx); err != nil {
		sklog.Fatal(err)
	}
	sklog.Info("Checkout complete")

	// Cache for buildProgressHandler.
	tasksPerCommit, err = newTasksPerCommitCache(ctx, *workdir, []string{common.REPO_SKIA, common.REPO_SKIA_INFRA}, 14*24*time.Hour)
	if err != nil {
		sklog.Fatalf("Failed to create tasksPerCommitCache: %s", err)
	}

	// Create the IncrementalCache.
	w, err := window.New(time.Minute, MAX_COMMITS_TO_LOAD, repos)
	if err != nil {
		sklog.Fatalf("Failed to create time window: %s", err)
	}
	iCache, err = incremental.NewIncrementalCache(ctx, taskDb, w, repos, MAX_COMMITS_TO_LOAD, *swarmingUrl, *taskSchedulerUrl)
	if err != nil {
		sklog.Fatalf("Failed to create IncrementalCache: %s", err)
	}
	iCache.UpdateLoop(60*time.Second, ctx)

	// Create a regular task cache.
	tCache, err = cache.NewTaskCache(taskDb, w)
	if err != nil {
		sklog.Fatalf("Failed to create TaskCache: %s", err)
	}
	lvTaskCache := metrics2.NewLiveness("status_task_cache")
	go util.RepeatCtx(60*time.Second, ctx, func() {
		if err := tCache.Update(); err != nil {
			sklog.Errorf("Failed to update TaskCache: %s", err)
		} else {
			lvTaskCache.Reset()
		}
	})

	// Capacity stats.
	capacityClient = capacity.New(tasksPerCommit.tcc, tCache, repos)
	capacityClient.StartLoading(ctx, *capacityRecalculateInterval)

	// Periodically obtain the autoroller statuses.
	updateAutorollStatus := func() error {
		statuses := make(map[string]interface{}, len(AUTOROLLERS))
		for _, host := range []string{"https://autoroll.skia.org", "https://autoroll-internal.skia.org"} {
			url := host + "/json/all"
			resp, err := c.Get(url)
			if err != nil {
				return err
			}
			defer util.Close(resp.Body)
			var st map[string]map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
				return err
			}
			for name, s := range st {
				if friendlyName, ok := AUTOROLLERS[name]; ok {
					s["url"] = host + "/r/" + name
					statuses[friendlyName] = s
				}
			}
		}
		b, err := json.Marshal(statuses)
		if err != nil {
			return err
		}
		autorollMtx.Lock()
		defer autorollMtx.Unlock()
		autorollStatus = b
		return nil
	}
	if err := updateAutorollStatus(); err != nil {
		sklog.Fatal(err)
	}
	go util.RepeatCtx(60*time.Second, ctx, func() {
		if err := updateAutorollStatus(); err != nil {
			sklog.Errorf("Failed to update autoroll status: %s", err)
		}
	})

	// Create the TaskDriver DB.
	btProject := "skia-public"
	taskDriverDb, err = bigtable_db.NewBigTableDB(ctx, btProject, bigtable_db.BT_INSTANCE, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	taskDriverLogs, err = logs.NewLogsManager(ctx, btProject, logs.BT_INSTANCE, ts)
	if err != nil {
		sklog.Fatal(err)
	}

	// Run the server.
	runServer(serverURL)
}
