/*
	Handlers and types specific to Chromium builds.
*/

package chromium_builds

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
)

const (
	// URL returning the GIT commit hash of the last known good release of Chromium.
	CHROMIUM_LKGR_URL = "http://chromium-status.appspot.com/git-lkgr"
)

var (
	// Base URL of the Chromium GIT repository, to be followed by commit hash.
	CHROMIUM_GIT_REPO_URL = fmt.Sprintf(gitiles.CommitURL, common.REPO_CHROMIUM, "")
	// URL of a base64-encoded file that includes the GIT commit hash last known good release of Skia.
	CHROMIUM_DEPS_FILE = fmt.Sprintf(gitiles.DownloadURL, common.REPO_CHROMIUM, "master", "DEPS")
	// Base URL of the Skia GIT repository, to be followed by commit hash.
	SKIA_GIT_REPO_URL = fmt.Sprintf(gitiles.CommitURL, common.REPO_SKIA, "")

	addTaskTemplate     *template.Template = nil
	runsHistoryTemplate *template.Template = nil

	httpClient = httputils.NewTimeoutClient()
)

func ReloadTemplates(resourcesDir string) {
	addTaskTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/chromium_builds.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
	runsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/chromium_build_runs_history.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
}

type DatastoreTask struct {
	task_common.CommonCols

	ChromiumRev   string
	ChromiumRevTs int64
	SkiaRev       string
}

func (task DatastoreTask) GetTaskName() string {
	return "ChromiumBuild"
}

func (task DatastoreTask) GetDescription() string {
	// This task does not support descriptions.
	return ""
}

func (task DatastoreTask) GetResultsLink() string {
	return ""
}

func (task DatastoreTask) GetPopulatedAddTaskVars() (task_common.AddTaskVars, error) {
	taskVars := &AddTaskVars{}
	taskVars.Username = task.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(task.RepeatAfterDays, 10)

	taskVars.ChromiumRev = task.ChromiumRev
	taskVars.ChromiumRevTs = strconv.FormatInt(task.ChromiumRevTs, 10)
	taskVars.SkiaRev = task.SkiaRev
	return taskVars, nil
}

func (task DatastoreTask) RunsOnGCEWorkers() bool {
	// Unused for chromium_builds because it always runs on the GCE builders not
	// the workers or bare-metal machines.
	return false
}

func (task DatastoreTask) GetDatastoreKind() ds.Kind {
	return ds.CHROMIUM_BUILD_TASKS
}

func (task DatastoreTask) Query(it *datastore.Iterator) (interface{}, error) {
	tasks := []*DatastoreTask{}
	for {
		t := &DatastoreTask{}
		_, err := it.Next(t)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of tasks: %s", err)
		}
		tasks = append(tasks, t)
	}

	return tasks, nil
}

func (task DatastoreTask) Get(c context.Context, key *datastore.Key) (task_common.Task, error) {
	t := &DatastoreTask{}
	if err := ds.DS.Get(c, key, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (task DatastoreTask) TriggerSwarmingTaskAndMail(ctx context.Context) error {
	runID := task_common.GetRunID(&task)
	emails := task_common.GetEmailRecipients(task.Username, nil)
	isolateArgs := map[string]string{
		"TARGET_PLATFORM": ctutil.PLATFORM_LINUX,
		"CHROMIUM_HASH":   task.ChromiumRev,
		"SKIA_HASH":       task.SkiaRev,
	}

	sTaskID, err := ctutil.TriggerMasterScriptSwarmingTask(ctx, runID, "build_chromium", ctutil.BUILD_CHROMIUM_MASTER_ISOLATE, task_common.ServiceAccountFile, ctutil.PLATFORM_LINUX, false, isolateArgs)
	if err != nil {
		return fmt.Errorf("Could not trigger master script for build_chromium with isolate args %v: %s", isolateArgs, err)
	}
	// Mark task as started in datastore.
	if err := task_common.UpdateTaskSetStarted(ctx, runID, sTaskID, &task); err != nil {
		return fmt.Errorf("Could not mark task as started in datastore: %s", err)
	}
	// Send start email.
	skutil.LogErr(ctfeutil.SendTaskStartEmail(task.DatastoreKey.ID, emails, "Build chromium", runID, "", ""))
	return nil
}

func (task DatastoreTask) SendCompletionEmail(ctx context.Context, completedSuccessfully bool) error {
	runID := task_common.GetRunID(&task)
	emails := task_common.GetEmailRecipients(task.Username, nil)
	emailSubject := fmt.Sprintf("Chromium build task has completed (#%d)", task.DatastoreKey.ID)
	failureHtml := ""
	if !completedSuccessfully {
		emailSubject += " with failures"
		failureHtml = ctfeutil.GetFailureEmailHtml(runID)
	}
	bodyTemplate := `
	The Cluster telemetry queued task to create a new chromium build has completed. %s.<br/>
	%s
	You can schedule more runs <a href="%s">here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, ctfeutil.GetSwarmingLogsLink(runID), failureHtml, task_common.WebappURL+ctfeutil.CHROMIUM_BUILD_URI)
	if err := ctfeutil.SendEmail(emails, emailSubject, emailBody); err != nil {
		return fmt.Errorf("Error while sending email: %s", err)
	}
	return nil
}

func (task *DatastoreTask) SetCompleted(success bool) {
	task.TsCompleted = ctutil.GetCurrentTsInt64()
	task.Failure = !success
	task.TaskDone = true
}

func addTaskView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(addTaskTemplate, w, r)
}

type AddTaskVars struct {
	task_common.AddTaskCommonVars

	ChromiumRev   string `json:"chromium_rev"`
	ChromiumRevTs string `json:"chromium_rev_ts"`
	SkiaRev       string `json:"skia_rev"`
}

func (task *AddTaskVars) GetDatastoreKind() ds.Kind {
	return ds.CHROMIUM_BUILD_TASKS
}

func (task *AddTaskVars) GetPopulatedDatastoreTask(ctx context.Context) (task_common.Task, error) {
	if task.ChromiumRev == "" ||
		task.SkiaRev == "" ||
		task.ChromiumRevTs == "" {
		return nil, fmt.Errorf("Invalid parameters")
	}

	// Example timestamp format: "Wed Jul 15 13:42:19 2015"
	var chromiumRevTs int64
	if parsedTs, err := time.Parse(time.ANSIC, task.ChromiumRevTs); err != nil {
		// ChromiumRevTs is likely already in the expected format.
		chromiumRevTs, err = strconv.ParseInt(task.ChromiumRevTs, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s is not int64: %s", task.ChromiumRevTs, err)
		}
	} else {
		ts := parsedTs.UTC().Format("20060102150405")
		chromiumRevTs, err = strconv.ParseInt(ts, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s is not int64: %s", ts, err)
		}
	}

	t := &DatastoreTask{
		ChromiumRev:   task.ChromiumRev,
		SkiaRev:       task.SkiaRev,
		ChromiumRevTs: chromiumRevTs,
	}
	return t, nil
}

func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.AddTaskHandler(w, r, &AddTaskVars{})
}

func getRevDataHandler(getLkgr func() (string, error), gitRepoUrl string, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	revString := r.FormValue("rev")
	if revString == "" {
		httputils.ReportError(w, nil, "No revision specified", http.StatusInternalServerError)
		return
	}

	if strings.EqualFold(revString, "LKGR") {
		lkgr, err := getLkgr()
		if err != nil {
			httputils.ReportError(w, nil, "Unable to retrieve LKGR revision", http.StatusInternalServerError)
		}
		sklog.Infof("Retrieved LKGR commit hash: %s", lkgr)
		revString = lkgr
	}
	commitUrl := gitRepoUrl + revString + "?format=JSON"
	sklog.Infof("Reading revision detail from %s", commitUrl)
	resp, err := httpClient.Get(commitUrl)
	if err != nil {
		httputils.ReportError(w, err, "Unable to retrieve revision detail", http.StatusInternalServerError)
		return
	}
	defer skutil.Close(resp.Body)
	if resp.StatusCode == 404 {
		// Return successful empty response, since the user could be typing.
		if err := json.NewEncoder(w).Encode(map[string]interface{}{}); err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
			return
		}
		return
	}
	if resp.StatusCode != 200 {
		httputils.ReportError(w, nil, "Unable to retrieve revision detail", http.StatusInternalServerError)
		return
	}
	// Remove junk in the first line. https://code.google.com/p/gitiles/issues/detail?id=31
	bufBody := bufio.NewReader(resp.Body)
	if _, err = bufBody.ReadString('\n'); err != nil {
		httputils.ReportError(w, err, "Unable to retrieve revision detail", http.StatusInternalServerError)
		return
	}
	if _, err = io.Copy(w, bufBody); err != nil {
		httputils.ReportError(w, err, "Unable to retrieve revision detail", http.StatusInternalServerError)
		return
	}
}

// TODO(benjaminwagner): Seems to duplicate code in ct/go/util/chromium_builds.go.
func getChromiumLkgr() (string, error) {
	sklog.Infof("Reading Chromium LKGR from %s", CHROMIUM_LKGR_URL)
	resp, err := httpClient.Get(CHROMIUM_LKGR_URL)
	if err != nil {
		return "", err
	}
	defer skutil.Close(resp.Body)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return bytes.NewBuffer(body).String(), nil
}

func getChromiumRevDataHandler(w http.ResponseWriter, r *http.Request) {
	getRevDataHandler(getChromiumLkgr, CHROMIUM_GIT_REPO_URL, w, r)
}

var skiaRevisionRegexp = regexp.MustCompile("'skia_revision': '([0-9a-fA-F]{2,40})'")

func getSkiaLkgr() (string, error) {
	return buildskia.GetSkiaHash(httpClient)
}

func getSkiaRevDataHandler(w http.ResponseWriter, r *http.Request) {
	getRevDataHandler(getSkiaLkgr, SKIA_GIT_REPO_URL, w, r)
}

func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.DeleteTaskHandler(&DatastoreTask{}, w, r)
}

func redoTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.RedoTaskHandler(&DatastoreTask{}, w, r)
}

func runsHistoryView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(runsHistoryTemplate, w, r)
}

func getTasksHandler(w http.ResponseWriter, r *http.Request) {
	task_common.GetTasksHandler(&DatastoreTask{}, w, r)
}

// Validate that the given chromiumBuild exists in the Datastore.
func Validate(ctx context.Context, chromiumBuild DatastoreTask) error {
	q := ds.NewQuery(chromiumBuild.GetDatastoreKind())
	q = q.Filter("ChromiumRev =", chromiumBuild.ChromiumRev)
	q = q.Filter("SkiaRev =", chromiumBuild.SkiaRev)
	q = q.Filter("TaskDone =", true)
	q = q.Filter("Failure =", false)

	count, err := ds.DS.Count(ctx, q)
	if err != nil {
		sklog.Info(err)
		return fmt.Errorf("Error when validating chromium build %v: %s", chromiumBuild, err)
	}
	if count == 0 {
		return fmt.Errorf("Unable to validate chromium_build parameter %v", chromiumBuild)
	}
	return nil
}

func AddHandlers(externalRouter *mux.Router) {
	externalRouter.HandleFunc("/"+ctfeutil.CHROMIUM_BUILD_URI, addTaskView).Methods("GET")
	externalRouter.HandleFunc("/"+ctfeutil.CHROMIUM_BUILD_RUNS_URI, runsHistoryView).Methods("GET")

	externalRouter.HandleFunc("/"+ctfeutil.CHROMIUM_REV_DATA_POST_URI, getChromiumRevDataHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.SKIA_REV_DATA_POST_URI, getSkiaRevDataHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.ADD_CHROMIUM_BUILD_TASK_POST_URI, addTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.GET_CHROMIUM_BUILD_TASKS_POST_URI, getTasksHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.DELETE_CHROMIUM_BUILD_TASK_POST_URI, deleteTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.REDO_CHROMIUM_BUILD_TASK_POST_URI, redoTaskHandler).Methods("POST")
}
