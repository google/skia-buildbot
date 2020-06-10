/*
	Handlers and types specific to capturing SKP repositories.
*/

package capture_skps

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"text/template"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/swarming"
	skutil "go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
)

var (
	addTaskTemplate     *template.Template = nil
	runsHistoryTemplate *template.Template = nil
)

func ReloadTemplates(resourcesDir string) {
	addTaskTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/capture_skps.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
	runsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/capture_skp_runs_history.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
}

type DatastoreTask struct {
	task_common.CommonCols

	PageSets      string
	IsTestPageSet bool
	ChromiumRev   string
	SkiaRev       string
	Description   string
}

func (task DatastoreTask) GetTaskName() string {
	return "CaptureSkps"
}

func (task DatastoreTask) GetDescription() string {
	return task.Description
}

func (task DatastoreTask) GetResultsLink() string {
	return ""
}

func (task *DatastoreTask) GetPopulatedAddTaskVars() (task_common.AddTaskVars, error) {
	taskVars := &AddTaskVars{}
	taskVars.Username = task.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(task.RepeatAfterDays, 10)
	taskVars.PageSets = task.PageSets
	taskVars.ChromiumBuild.ChromiumRev = task.ChromiumRev
	taskVars.ChromiumBuild.SkiaRev = task.SkiaRev
	taskVars.Description = task.Description
	return taskVars, nil
}

func (task DatastoreTask) RunsOnGCEWorkers() bool {
	// TODO(rmistry): Figure out which font packages to install on the GCE
	// instances if any missing packages become an issue.
	return true
}

func (task DatastoreTask) GetDatastoreKind() ds.Kind {
	return ds.CAPTURE_SKPS_TASKS
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

func (task DatastoreTask) TriggerSwarmingTaskAndMail(ctx context.Context, swarmingClient swarming.ApiClient) error {
	runID := task_common.GetRunID(&task)
	emails := task_common.GetEmailRecipients(task.Username, nil)
	cmd := []string{
		"cipd_bin_packages/luci-auth",
		"context",
		"--",
		"bin/capture_skps_on_workers",
		"-logtostderr",
		"--pageset_type=" + task.PageSets,
		"--chromium_build=" + ctutil.ChromiumBuildDir(task.ChromiumRev, task.SkiaRev, ""),
		"--target_platform=" + ctutil.PLATFORM_LINUX,
		"--run_on_gce=" + strconv.FormatBool(task.RunsOnGCEWorkers()),
		"--run_id=" + runID,
	}

	sTaskID, err := ctutil.TriggerMasterScriptSwarmingTask(ctx, runID, "capture_skps_on_workers", ctutil.CAPTURE_SKPS_MASTER_ISOLATE, task_common.ServiceAccountFile, ctutil.PLATFORM_LINUX, false, cmd, swarmingClient)
	if err != nil {
		return fmt.Errorf("Could not trigger master script for capture_skps_on_workers with cmd %v: %s", cmd, err)
	}
	// Mark task as started in datastore.
	if err := task_common.UpdateTaskSetStarted(ctx, runID, sTaskID, &task); err != nil {
		return fmt.Errorf("Could not mark task as started in datastore: %s", err)
	}
	// Send start email.
	skutil.LogErr(ctfeutil.SendTaskStartEmail(task.DatastoreKey.ID, emails, "Capture SKPs", runID, task.Description, ""))
	return nil
}

func (task DatastoreTask) SendCompletionEmail(ctx context.Context, completedSuccessfully bool) error {
	runID := task_common.GetRunID(&task)
	emails := task_common.GetEmailRecipients(task.Username, nil)
	emailSubject := fmt.Sprintf("Capture SKPs cluster telemetry task has completed (#%d)", task.DatastoreKey.ID)
	failureHtml := ""
	if !completedSuccessfully {
		emailSubject += " with failures"
		failureHtml = ctfeutil.GetFailureEmailHtml(runID)
	}
	bodyTemplate := `
	The Capture SKPs task on %s pageset has completed. %s.<br/>
	Run description: %s<br/>
	%s
	You can schedule more runs <a href="%s">here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, task.PageSets, ctfeutil.GetSwarmingLogsLink(runID), task.Description, failureHtml, task_common.WebappURL+ctfeutil.CAPTURE_SKPS_URI)
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

	PageSets      string                        `json:"page_sets"`
	ChromiumBuild chromium_builds.DatastoreTask `json:"chromium_build"`
	Description   string                        `json:"desc"`
}

func (task *AddTaskVars) GetDatastoreKind() ds.Kind {
	return ds.CAPTURE_SKPS_TASKS
}

func (task *AddTaskVars) GetPopulatedDatastoreTask(ctx context.Context) (task_common.Task, error) {

	if task.PageSets == "" ||
		task.ChromiumBuild.ChromiumRev == "" ||
		task.ChromiumBuild.SkiaRev == "" ||
		task.Description == "" {
		return nil, fmt.Errorf("Invalid parameters")
	}
	if err := chromium_builds.Validate(ctx, task.ChromiumBuild); err != nil {
		return nil, err
	}

	t := &DatastoreTask{
		PageSets:      task.PageSets,
		IsTestPageSet: task.PageSets == ctutil.PAGESET_TYPE_DUMMY_1k || task.PageSets == ctutil.PAGESET_TYPE_MOBILE_DUMMY_1k,
		ChromiumRev:   task.ChromiumBuild.ChromiumRev,
		SkiaRev:       task.ChromiumBuild.SkiaRev,
		Description:   task.Description,
	}
	return t, nil
}

func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.AddTaskHandler(w, r, &AddTaskVars{})
}

func getTasksHandler(w http.ResponseWriter, r *http.Request) {
	task_common.GetTasksHandler(&DatastoreTask{}, w, r)
}

// Validate that the given skpRepository exists in the Datastore.
func Validate(ctx context.Context, skpRepository DatastoreTask) error {
	q := ds.NewQuery(skpRepository.GetDatastoreKind())
	q = q.Filter("PageSets =", skpRepository.PageSets)
	q = q.Filter("ChromiumRev =", skpRepository.ChromiumRev)
	q = q.Filter("SkiaRev =", skpRepository.SkiaRev)
	q = q.Filter("TaskDone =", true)
	q = q.Filter("Failure=", false)

	count, err := ds.DS.Count(ctx, q)
	if err != nil {
		return fmt.Errorf("Error when validating skp repository %v: %s", skpRepository, err)
	}
	if count == 0 {
		return fmt.Errorf("Unable to validate skp repository parameter %v", skpRepository)
	}

	return nil
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

func AddHandlers(externalRouter *mux.Router) {
	externalRouter.HandleFunc("/"+ctfeutil.CAPTURE_SKPS_URI, addTaskView).Methods("GET")
	externalRouter.HandleFunc("/"+ctfeutil.CAPTURE_SKPS_RUNS_URI, runsHistoryView).Methods("GET")

	externalRouter.HandleFunc("/"+ctfeutil.ADD_CAPTURE_SKPS_TASK_POST_URI, addTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.GET_CAPTURE_SKPS_TASKS_POST_URI, getTasksHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.DELETE_CAPTURE_SKPS_TASK_POST_URI, deleteTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.REDO_CAPTURE_SKPS_TASK_POST_URI, redoTaskHandler).Methods("POST")
}
