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
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/datastore"
	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.skia.org/infra/autoroll/go/status"
	autoroll_status "go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/status/go/capacity"
	"go.skia.org/infra/status/go/incremental"
	"go.skia.org/infra/status/go/lkgr"
	"go.skia.org/infra/status/go/rpc"
	task_driver_db "go.skia.org/infra/task_driver/go/db"
	bigtable_db "go.skia.org/infra/task_driver/go/db/bigtable"
	"go.skia.org/infra/task_driver/go/handlers"
	"go.skia.org/infra/task_driver/go/logs"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
	"google.golang.org/api/option"
)

const (
	APPNAME = "status"

	// The chrome infra auth group to use for restricting admin rights.
	AUTH_GROUP_ADMIN_RIGHTS = "google/skia-root@google.com"
	// The chrome infra auth group to use for restricting edit rights.
	AUTH_GROUP_EDIT_RIGHTS = "google/skia-staff@google.com"

	DEFAULT_COMMITS_TO_LOAD = 35
	MAX_COMMITS_TO_LOAD     = 100
	SKIA_REPO               = "skia"
	INFRA_REPO              = "infra"
)

var (
	autorollMtx         sync.RWMutex
	autorollStatus      []byte                             = nil
	autorollStatusTwirp *rpc.GetAutorollerStatusesResponse = nil
	capacityClient      *capacity.CapacityClientImpl       = nil
	capacityTemplate    *template.Template                 = nil
	commitsTemplate     *template.Template                 = nil
	iCache              *incremental.IncrementalCacheImpl  = nil
	lkgrObj             *lkgr.LKGR                         = nil
	taskDb              db.RemoteDB                        = nil
	taskDriverDb        task_driver_db.DB                  = nil
	taskDriverLogs      *logs.LogsManager                  = nil
	tasksPerCommit      *tasksPerCommitCache               = nil
	tCache              cache.TaskCache                    = nil

	// AUTOROLLERS maps autoroll frontend host to maps of roller IDs to
	// their human-friendly display names.
	AUTOROLLERS = map[string]map[string]string{
		"autoroll.skia.org": {
			"skia-flutter-autoroll":     "Flutter",
			"skia-autoroll":             "Chrome",
			"angle-skia-autoroll":       "ANGLE",
			"skcms-skia-autoroll":       "skcms",
			"swiftshader-skia-autoroll": "SwiftSh",
		},
		"skia-autoroll.corp.goog": {
			"android-master-autoroll": "Android",
			"google3-autoroll":        "Google3",
		},
	}
)

// flags
var (
	chromeInfraAuthJWT = flag.String("chrome_infra_auth_jwt", "/var/secrets/skia-public-auth/key.json", "The JWT key for the service account that has access to chrome infra auth.")
	// TODO(borenet): Combine btInstance and firestoreInstance.
	btInstance                  = flag.String("bigtable_instance", "", "BigTable instance to use.")
	btProject                   = flag.String("bigtable_project", "", "GCE project to use for BigTable.")
	capacityRecalculateInterval = flag.Duration("capacity_recalculate_interval", 10*time.Minute, "How often to re-calculate capacity statistics.")
	firestoreInstance           = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	gitstoreTable               = flag.String("gitstore_bt_table", "git-repos2", "BigTable table used for GitStore.")
	host                        = flag.String("host", "localhost", "HTTP service host")
	port                        = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	promPort                    = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	repoUrls                    = common.NewMultiStringFlag("repo", nil, "Repositories to query for status.")
	resourcesDir                = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	swarmingUrl                 = flag.String("swarming_url", "https://chromium-swarm.appspot.com", "URL of the Swarming server.")
	taskLogsUrlTemplate         = flag.String("task_logs_url_template", "https://ci.chromium.org/raw/build/logs.chromium.org/skia/{{TaskID}}/+/annotations", "Template URL for direct link to logs, with {{TaskID}} as placeholder.")
	taskSchedulerUrl            = flag.String("task_scheduler_url", "https://task-scheduler.skia.org", "URL of the Task Scheduler server.")
	testing                     = flag.Bool("testing", false, "Set to true for locally testing rules. No email will be sent.")
	treeStatusBaseUrl           = flag.String("tree_status_base_url", "https://tree-status.skia.org", "Repo specific tree status URLs will be created using this base url. Eg: https://tree-status.skia.org or https://skia-tree-status.corp.goog")

	podId string
	repos repograph.Map
	// Repos and associated templates for creating links to their commits.
	repoURLsByName map[string]string
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
		filepath.Join(*resourcesDir, "dist", "status.html"),
	))
	capacityTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "dist", "capacity.html"),
	))
}

func Init() {
	reloadTemplates()
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

func getStringParam(name string, r *http.Request) string {
	raw, ok := r.URL.Query()[name]
	if !ok {
		return ""
	}
	return raw[0]
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

// Same as above, for new WIP Twirp server.
// TODO(westont): Refactor once Twirp server is in use.
func getRepoTwirp(repo string) (string, string, error) {
	repoURL, err := repoNameToUrl(repo)
	if err != nil {
		return "", "", err
	}
	return repoUrlToName(repoURL), repoURL, nil
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
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}
	comments, err := taskDb.GetCommentsForRepos([]string{repoUrl}, time.Now().Add(-10000*time.Hour))
	if err != nil {
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
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
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}
	from, err := getIntParam("from", r)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Invalid parameter for \"from\": %s", err), http.StatusInternalServerError)
		return
	}
	to, err := getIntParam("to", r)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Invalid parameter for \"to\": %s", err), http.StatusInternalServerError)
		return
	}
	n, err := getIntParam("n", r)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Invalid parameter for \"n\": %s", err), http.StatusInternalServerError)
		return
	}
	expectPodId := getStringParam("pod", r)
	numCommits := DEFAULT_COMMITS_TO_LOAD
	if n != nil {
		numCommits = int(*n)
		if numCommits > MAX_COMMITS_TO_LOAD {
			numCommits = MAX_COMMITS_TO_LOAD
		}
	}
	update := struct {
		*incremental.Update
		Pod string `json:"pod"`
	}{
		Pod: podId,
	}
	if (expectPodId != "" && expectPodId != podId) || from == nil {
		update.Update, err = iCache.GetAll(repoUrl, numCommits)
	} else {
		fromTime := time.Unix(0, (*from)*int64(time.Millisecond))
		if to != nil {
			toTime := time.Unix(0, (*to)*int64(time.Millisecond))
			update.Update, err = iCache.GetRange(repoUrl, fromTime, toTime, numCommits)
		} else {
			update.Update, err = iCache.Get(repoUrl, fromTime, numCommits)
		}
	}
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to retrieve updates: %s", err), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(update); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode response: %s", err), http.StatusInternalServerError)
		return
	}
}

func addTaskCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	defer util.Close(r.Body)
	w.Header().Set("Content-Type", "application/json")

	id, ok := mux.Vars(r)["id"]
	if !ok {
		httputils.ReportError(w, fmt.Errorf("No task ID given!"), "No task ID given!", http.StatusInternalServerError)
		return
	}
	task, err := taskDb.GetTaskById(id)
	if err != nil {
		httputils.ReportError(w, err, "Failed to obtain task details.", http.StatusInternalServerError)
		return
	}

	comment := struct {
		Comment string `json:"comment"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to add comment: %s", err), http.StatusInternalServerError)
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
		httputils.ReportError(w, nil, fmt.Sprintf("Failed to add comment: %s", err), http.StatusInternalServerError)
		return
	}
	if err := iCache.Update(context.Background(), false); err != nil {
		httputils.ReportError(w, nil, fmt.Sprintf("Failed to update cache: %s", err), http.StatusInternalServerError)
		return
	}
}

func deleteTaskCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "application/json")

	id, ok := mux.Vars(r)["id"]
	if !ok {
		httputils.ReportError(w, fmt.Errorf("No task ID given!"), "No task ID given!", http.StatusInternalServerError)
		return
	}
	task, err := taskDb.GetTaskById(id)
	if err != nil {
		httputils.ReportError(w, err, "Failed to obtain task details.", http.StatusInternalServerError)
		return
	}
	timestamp, err := strconv.ParseInt(mux.Vars(r)["timestamp"], 10, 64)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Invalid comment id: %v", err), http.StatusInternalServerError)
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
		httputils.ReportError(w, err, fmt.Sprintf("Failed to delete comment: %v", err), http.StatusInternalServerError)
		return
	}
	if err := iCache.Update(context.Background(), false); err != nil {
		httputils.ReportError(w, nil, fmt.Sprintf("Failed to update cache: %s", err), http.StatusInternalServerError)
		return
	}
}

func addTaskSpecCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "application/json")
	taskSpec, ok := mux.Vars(r)["taskSpec"]
	if !ok {
		httputils.ReportError(w, nil, "No taskSpec provided!", http.StatusInternalServerError)
		return
	}
	_, repoUrl, err := getRepo(r)
	if err != nil {
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}

	comment := struct {
		Comment       string `json:"comment"`
		Flaky         bool   `json:"flaky"`
		IgnoreFailure bool   `json:"ignoreFailure"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to add comment: %v", err), http.StatusInternalServerError)
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
		httputils.ReportError(w, err, fmt.Sprintf("Failed to add task spec comment: %v", err), http.StatusInternalServerError)
		return
	}
	if err := iCache.Update(context.Background(), false); err != nil {
		httputils.ReportError(w, nil, fmt.Sprintf("Failed to update cache: %s", err), http.StatusInternalServerError)
		return
	}
}

func deleteTaskSpecCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "application/json")
	taskSpec, ok := mux.Vars(r)["taskSpec"]
	if !ok {
		httputils.ReportError(w, nil, "No taskSpec provided!", http.StatusInternalServerError)
		return
	}
	_, repoUrl, err := getRepo(r)
	if err != nil {
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}
	timestamp, err := strconv.ParseInt(mux.Vars(r)["timestamp"], 10, 64)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Invalid timestamp: %v", err), http.StatusInternalServerError)
		return
	}
	c := types.TaskSpecComment{
		Repo:      repoUrl,
		Name:      taskSpec,
		Timestamp: time.Unix(0, timestamp),
	}
	if err := taskDb.DeleteTaskSpecComment(&c); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to delete comment: %v", err), http.StatusInternalServerError)
		return
	}
	if err := iCache.Update(context.Background(), false); err != nil {
		httputils.ReportError(w, nil, fmt.Sprintf("Failed to update cache: %s", err), http.StatusInternalServerError)
		return
	}
}

func addCommitCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "application/json")
	_, repoUrl, err := getRepo(r)
	if err != nil {
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}
	commit := mux.Vars(r)["commit"]
	comment := struct {
		Comment       string `json:"comment"`
		IgnoreFailure bool   `json:"ignoreFailure"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to add comment: %v", err), http.StatusInternalServerError)
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
		httputils.ReportError(w, err, fmt.Sprintf("Failed to add commit comment: %s", err), http.StatusInternalServerError)
		return
	}
	if err := iCache.Update(context.Background(), false); err != nil {
		httputils.ReportError(w, nil, fmt.Sprintf("Failed to update cache: %s", err), http.StatusInternalServerError)
		return
	}
}

func deleteCommitCommentHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "application/json")
	_, repoUrl, err := getRepo(r)
	if err != nil {
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}
	commit := mux.Vars(r)["commit"]
	timestamp, err := strconv.ParseInt(mux.Vars(r)["timestamp"], 10, 64)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Invalid comment id: %v", err), http.StatusInternalServerError)
		return
	}
	c := types.CommitComment{
		Repo:      repoUrl,
		Revision:  commit,
		Timestamp: time.Unix(0, timestamp),
	}
	if err := taskDb.DeleteCommitComment(&c); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to delete commit comment: %s", err), http.StatusInternalServerError)
		return
	}
	if err := iCache.Update(context.Background(), false); err != nil {
		httputils.ReportError(w, nil, fmt.Sprintf("Failed to update cache: %s", err), http.StatusInternalServerError)
		return
	}
}

type commitsTemplateData struct {
	Repo     string
	Title    string
	RepoBase string
	Repos    []string
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "text/html")

	defaultRepo := repoUrlToName((*repoUrls)[0])

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}

	d := struct {
		Title             string
		SwarmingURL       string
		TreeStatusBaseURL string
		LogsURLTemplate   string
		TaskSchedulerURL  string
		DefaultRepo       string
		// Repo name to repo URL.
		Repos map[string]string
	}{
		Title:             fmt.Sprintf("Status: %s", defaultRepo),
		SwarmingURL:       *swarmingUrl,
		TreeStatusBaseURL: *treeStatusBaseUrl,
		LogsURLTemplate:   *taskLogsUrlTemplate,
		TaskSchedulerURL:  *taskSchedulerUrl,
		DefaultRepo:       defaultRepo,
		Repos:             repoURLsByName,
	}

	if err := commitsTemplate.Execute(w, d); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to expand template: %v", err), http.StatusInternalServerError)
	}
}

func capacityHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "text/html")

	defaultRepo := repoUrlToName((*repoUrls)[0])

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}

	d := struct {
		Title            string
		SwarmingURL      string
		LogsURLTemplate  string
		TaskSchedulerURL string
		DefaultRepo      string
		// Repo name to repo URL.
		Repos map[string]string
	}{
		Title:            "Capacity Statistics for Skia Bots",
		SwarmingURL:      *swarmingUrl,
		LogsURLTemplate:  *taskLogsUrlTemplate,
		TaskSchedulerURL: *taskSchedulerUrl,
		DefaultRepo:      defaultRepo,
		Repos:            repoURLsByName,
	}

	if err := capacityTemplate.Execute(w, d); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to expand template: %v", err), http.StatusInternalServerError)
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
		httputils.ReportError(w, nil, fmt.Sprintf("%q is not a valid commit hash.", hash), http.StatusInternalServerError)
		return
	}
	_, repoUrl, err := getRepo(r)
	if err != nil {
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}
	tasks, err := tCache.GetTasksForCommits(repoUrl, []string{hash})
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to get the number of finished builds."), http.StatusInternalServerError)
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
		httputils.ReportError(w, err, fmt.Sprintf("Failed to get number of tasks at commit."), http.StatusInternalServerError)
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
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode JSON."), http.StatusInternalServerError)
		return
	}
}

func lkgrHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(lkgrObj.Get())); err != nil {
		httputils.ReportError(w, err, "Failed to write response.", http.StatusInternalServerError)
		return
	}
}

func autorollStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	autorollMtx.RLock()
	defer autorollMtx.RUnlock()
	if _, err := w.Write(autorollStatus); err != nil {
		httputils.ReportError(w, err, "Failed to write response.", http.StatusInternalServerError)
		return
	}
}

func getAutorollerStatusesTwirp() *rpc.GetAutorollerStatusesResponse {
	autorollMtx.RLock()
	defer autorollMtx.RUnlock()
	return autorollStatusTwirp
}

func runServer(serverURL string, srv http.Handler) {
	topLevelRouter := mux.NewRouter()
	topLevelRouter.Use(login.RestrictViewer)
	topLevelRouter.Use(login.SessionMiddleware)
	// Our 'main' router doesn't include the Twirp server, since it would double gzip responses.
	topLevelRouter.PathPrefix(rpc.StatusServicePathPrefix).Handler(httputils.LoggingRequestResponse(srv))
	r := topLevelRouter.NewRoute().Subrouter()
	r.Use(httputils.LoggingGzipRequestResponse)
	r.HandleFunc("/", httputils.CorsHandler(defaultHandler))
	r.HandleFunc("/capacity", capacityHandler)
	r.HandleFunc("/lkgr", lkgrHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	r.PathPrefix("/dist/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	handlers.AddTaskDriverHandlers(r, taskDriverDb, taskDriverLogs)
	var h http.Handler = topLevelRouter
	if !*testing {
		h = httputils.HealthzAndHTTPS(topLevelRouter)
	}
	h = httputils.XFrameOptionsDeny(h)
	http.Handle("/", h)
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

type autoRollStatus struct {
	autoroll_status.AutoRollMiniStatus
	Url string `json:"url"`
}

func main() {
	// Setup flags.
	common.InitWithMust(
		APPNAME,
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	Init()
	serverURL := "https://" + *host
	if *testing {
		serverURL = "http://" + *host + *port
	}
	ctx := context.Background()

	podId = os.Getenv("POD_ID")
	if podId == "" {
		sklog.Error("POD_ID not defined; falling back to UUID.")
		podId = uuid.New().String()
	}

	repoURLsByName = make(map[string]string)
	for _, repoURL := range *repoUrls {
		repoURLsByName[repoUrlToName(repoURL)] = fmt.Sprintf(gitiles.CommitURL, repoURL, "")
	}

	ts, err := auth.NewDefaultTokenSource(*testing, auth.ScopeUserinfoEmail, auth.ScopeGerrit, bigtable.Scope, pubsub.ScopePubSub, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create LKGR object.
	lkgrObj, err = lkgr.New(ctx)
	if err != nil {
		sklog.Fatalf("Failed to create LKGR: %s", err)
	}
	lkgrObj.UpdateLoop(10*time.Minute, ctx)

	// Create remote Tasks DB.
	taskDb, err = firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, ts)
	if err != nil {
		sklog.Fatalf("Failed to create Firestore DB client: %s", err)
	}

	criaTs, err := auth.NewJWTServiceAccountTokenSource("", *chromeInfraAuthJWT, auth.ScopeUserinfoEmail)
	if err != nil {
		sklog.Fatal(err)
	}
	criaClient := httputils.DefaultClientConfig().WithTokenSource(criaTs).With2xxOnly().Client()
	adminAllowed, err := allowed.NewAllowedFromChromeInfraAuth(criaClient, AUTH_GROUP_ADMIN_RIGHTS)
	if err != nil {
		sklog.Fatal(err)
	}
	editAllowed, err := allowed.NewAllowedFromChromeInfraAuth(criaClient, AUTH_GROUP_EDIT_RIGHTS)
	if err != nil {
		sklog.Fatal(err)
	}
	login.InitWithAllow(serverURL+login.DEFAULT_OAUTH2_CALLBACK, adminAllowed, editAllowed, nil)

	// Check out source code.
	if *repoUrls == nil {
		sklog.Fatal("At least one --repo is required.")
	}
	btConf := &bt_gitstore.BTConfig{
		ProjectID:  *btProject,
		InstanceID: *btInstance,
		TableID:    *gitstoreTable,
		AppProfile: APPNAME,
	}
	repos, err = bt_gitstore.NewBTGitStoreMap(ctx, *repoUrls, btConf)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Info("Checkout complete")

	// Cache for buildProgressHandler.
	tasksPerCommit, err = newTasksPerCommitCache(ctx, repos, 14*24*time.Hour, *btProject, *btInstance, ts)
	if err != nil {
		sklog.Fatalf("Failed to create tasksPerCommitCache: %s", err)
	}

	// Create the IncrementalCacheImpl.
	w, err := window.New(time.Minute, MAX_COMMITS_TO_LOAD, repos)
	if err != nil {
		sklog.Fatalf("Failed to create time window: %s", err)
	}
	iCache, err = incremental.NewIncrementalCacheImpl(ctx, taskDb, w, repos, MAX_COMMITS_TO_LOAD, *swarmingUrl, *taskSchedulerUrl)
	if err != nil {
		sklog.Fatalf("Failed to create IncrementalCacheImpl: %s", err)
	}
	iCache.UpdateLoop(ctx, 60*time.Second)

	// Create a regular task cache.
	tCache, err = cache.NewTaskCache(ctx, taskDb, w, nil)
	if err != nil {
		sklog.Fatalf("Failed to create TaskCache: %s", err)
	}
	lvTaskCache := metrics2.NewLiveness("status_task_cache")
	go util.RepeatCtx(ctx, 60*time.Second, func(ctx context.Context) {
		if err := tCache.Update(ctx); err != nil {
			sklog.Errorf("Failed to update TaskCache: %s", err)
		} else {
			lvTaskCache.Reset()
		}
	})

	// Capacity stats.
	capacityClient = capacity.New(tasksPerCommit.tcc, tCache, repos)
	capacityClient.StartLoading(ctx, *capacityRecalculateInterval)

	// Periodically obtain the autoroller statuses.
	if err := ds.InitWithOpt(common.PROJECT_ID, ds.AUTOROLL_NS, option.WithTokenSource(ts)); err != nil {
		sklog.Fatalf("Failed to initialize datastore: %s", err)
	}
	autorollStatusDB := status.NewDatastoreDB()
	updateAutorollStatus := func(ctx context.Context) error {
		statuses := map[string]autoRollStatus{}
		statusesTwirp := []*rpc.AutorollerStatus{}
		for host, subMap := range AUTOROLLERS {
			for roller, friendlyName := range subMap {
				s, err := autorollStatusDB.Get(ctx, roller)
				if err != nil {
					return err
				}
				miniStatus := s.AutoRollMiniStatus
				url := fmt.Sprintf("https://%s/r/%s", host, roller)
				statuses[friendlyName] = autoRollStatus{
					AutoRollMiniStatus: miniStatus,
					Url:                url,
				}
				statusesTwirp = append(statusesTwirp,
					&rpc.AutorollerStatus{
						Name:           friendlyName,
						CurrentRollRev: miniStatus.CurrentRollRev,
						LastRollRev:    miniStatus.LastRollRev,
						Mode:           miniStatus.Mode,
						NumBehind:      int32(miniStatus.NumNotRolledCommits),
						NumFailed:      int32(miniStatus.NumFailedRolls),
						Url:            url})
			}
		}
		b, err := json.Marshal(statuses)
		if err != nil {
			return err
		}
		sort.Slice(statusesTwirp, func(i, j int) bool {
			return statusesTwirp[i].Name < statusesTwirp[j].Name
		})
		autorollMtx.Lock()
		defer autorollMtx.Unlock()
		autorollStatus = b
		autorollStatusTwirp = &rpc.GetAutorollerStatusesResponse{Rollers: statusesTwirp}
		return nil
	}
	if err := updateAutorollStatus(ctx); err != nil {
		sklog.Fatal(err)
	}
	go util.RepeatCtx(ctx, 60*time.Second, func(ctx context.Context) {
		if err := updateAutorollStatus(ctx); err != nil {
			sklog.Errorf("Failed to update autoroll status: %s", err)
		}
	})

	// Create the TaskDriver DB.
	taskDriverBtInstance := "staging" // Task Drivers aren't in prod yet.
	taskDriverDb, err = bigtable_db.NewBigTableDB(ctx, *btProject, taskDriverBtInstance, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	taskDriverLogs, err = logs.NewLogsManager(ctx, *btProject, taskDriverBtInstance, ts)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create Twirp Server.
	twirpServer := rpc.NewStatusServer(iCache, taskDb, capacityClient, getAutorollerStatusesTwirp, getRepoTwirp, MAX_COMMITS_TO_LOAD, DEFAULT_COMMITS_TO_LOAD, podId)

	// Run the server.
	runServer(serverURL, twirpServer)
}
