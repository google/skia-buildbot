/*
	Handlers and types specific to running admin tasks, including recreating page sets and
	recreating webpage archives.
*/

package admin_tasks

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
	addTaskTemplate                            *template.Template = nil
	recreatePageSetsRunsHistoryTemplate        *template.Template = nil
	recreateWebpageArchivesRunsHistoryTemplate *template.Template = nil
)

func ReloadTemplates(resourcesDir string) {
	addTaskTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "dist", "admin_tasks.html"),
	))
	recreatePageSetsRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "dist", "recreate_page_sets_runs.html"),
	))
	recreateWebpageArchivesRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/recreate_webpage_archives_runs_history.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
}

type RecreatePageSetsDatastoreTask struct {
	task_common.CommonCols

	PageSets      string
	IsTestPageSet bool
}

func (task RecreatePageSetsDatastoreTask) GetTaskName() string {
	return "RecreatePageSets"
}

func (task RecreatePageSetsDatastoreTask) GetDescription() string {
	// This task does not support descriptions.
	return ""
}

func (task RecreatePageSetsDatastoreTask) GetPopulatedAddTaskVars() (task_common.AddTaskVars, error) {
	taskVars := &AddRecreatePageSetsTaskVars{}
	taskVars.Username = task.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(task.RepeatAfterDays, 10)

	taskVars.PageSets = task.PageSets
	return taskVars, nil
}

func (task RecreatePageSetsDatastoreTask) RunsOnGCEWorkers() bool {
	return true
}

func (task RecreatePageSetsDatastoreTask) GetDatastoreKind() ds.Kind {
	return ds.RECREATE_PAGESETS_TASKS
}

func (task RecreatePageSetsDatastoreTask) GetResultsLink() string {
	return ""
}

func (task RecreatePageSetsDatastoreTask) Query(it *datastore.Iterator) (interface{}, error) {
	tasks := []*RecreatePageSetsDatastoreTask{}
	for {
		t := &RecreatePageSetsDatastoreTask{}
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

func (task RecreatePageSetsDatastoreTask) Get(c context.Context, key *datastore.Key) (task_common.Task, error) {
	t := &RecreatePageSetsDatastoreTask{}
	if err := ds.DS.Get(c, key, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (task RecreatePageSetsDatastoreTask) TriggerSwarmingTaskAndMail(ctx context.Context, swarmingClient swarming.ApiClient) error {
	runID := task_common.GetRunID(&task)
	emails := task_common.GetEmailRecipients(task.Username, nil)
	cmd := []string{
		"cipd_bin_packages/luci-auth",
		"context",
		"--",
		"bin/create_pagesets_on_workers",
		"-logtostderr",
		"--run_on_gce=" + strconv.FormatBool(task.RunsOnGCEWorkers()),
		"--run_id=" + runID,
		"--pageset_type=" + task.PageSets,
	}

	sTaskID, err := ctutil.TriggerMasterScriptSwarmingTask(ctx, runID, "create_pagesets_on_workers", ctutil.CREATE_PAGESETS_MASTER_ISOLATE, task_common.ServiceAccountFile, ctutil.PLATFORM_LINUX, false, cmd, swarmingClient)
	if err != nil {
		return fmt.Errorf("Could not trigger master script for create_pagesets_on_workers with cmd %v: %s", cmd, err)
	}
	// Mark task as started in datastore.
	if err := task_common.UpdateTaskSetStarted(ctx, runID, sTaskID, &task); err != nil {
		return fmt.Errorf("Could not mark task as started in datastore: %s", err)
	}
	// Send start email.
	skutil.LogErr(ctfeutil.SendTaskStartEmail(task.DatastoreKey.ID, emails, "Creating pagesets", runID, "", ""))
	return nil
}

func (task RecreatePageSetsDatastoreTask) SendCompletionEmail(ctx context.Context, completedSuccessfully bool) error {
	runID := task_common.GetRunID(&task)
	emails := task_common.GetEmailRecipients(task.Username, nil)
	emailSubject := fmt.Sprintf("Create pagesets Cluster telemetry task has completed (#%d)", task.DatastoreKey.ID)
	failureHtml := ""
	if !completedSuccessfully {
		emailSubject += " with failures"
		failureHtml = ctfeutil.GetFailureEmailHtml(runID)
	}
	bodyTemplate := `
	The Cluster telemetry queued task to create %s pagesets has completed. %s.<br/>
	%s
	You can schedule more runs <a href="%s">here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, task.PageSets, ctfeutil.GetSwarmingLogsLink(runID), failureHtml, task_common.WebappURL+ctfeutil.ADMIN_TASK_URI)
	if err := ctfeutil.SendEmail(emails, emailSubject, emailBody); err != nil {
		return fmt.Errorf("Error while sending email: %s", err)
	}
	return nil
}

func (task *RecreatePageSetsDatastoreTask) SetCompleted(success bool) {
	task.TsCompleted = ctutil.GetCurrentTsInt64()
	task.Failure = !success
	task.TaskDone = true
}

func addRecreateWebpageArchivesTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.AddTaskHandler(w, r, &AddRecreateWebpageArchivesTaskVars{})
}

type RecreateWebpageArchivesDatastoreTask struct {
	task_common.CommonCols

	PageSets      string
	IsTestPageSet bool
	ChromiumRev   string
	SkiaRev       string
}

func (task RecreateWebpageArchivesDatastoreTask) GetTaskName() string {
	return "RecreateWebpageArchives"
}

func (task RecreateWebpageArchivesDatastoreTask) GetDescription() string {
	// This task does not support descriptions.
	return ""
}

func (task RecreateWebpageArchivesDatastoreTask) GetResultsLink() string {
	return ""
}

func (task RecreateWebpageArchivesDatastoreTask) GetPopulatedAddTaskVars() (task_common.AddTaskVars, error) {
	taskVars := &AddRecreateWebpageArchivesTaskVars{}
	taskVars.Username = task.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(task.RepeatAfterDays, 10)

	taskVars.PageSets = task.PageSets
	taskVars.ChromiumBuild.ChromiumRev = task.ChromiumRev
	taskVars.ChromiumBuild.SkiaRev = task.SkiaRev
	return taskVars, nil
}

func (task RecreateWebpageArchivesDatastoreTask) RunsOnGCEWorkers() bool {
	return true
}

func (task RecreateWebpageArchivesDatastoreTask) GetDatastoreKind() ds.Kind {
	return ds.RECREATE_WEBPAGE_ARCHIVES_TASKS
}

func (task RecreateWebpageArchivesDatastoreTask) Query(it *datastore.Iterator) (interface{}, error) {
	tasks := []*RecreateWebpageArchivesDatastoreTask{}
	for {
		t := &RecreateWebpageArchivesDatastoreTask{}
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

func (task RecreateWebpageArchivesDatastoreTask) Get(c context.Context, key *datastore.Key) (task_common.Task, error) {
	t := &RecreateWebpageArchivesDatastoreTask{}
	if err := ds.DS.Get(c, key, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (task RecreateWebpageArchivesDatastoreTask) TriggerSwarmingTaskAndMail(ctx context.Context, swarmingClient swarming.ApiClient) error {
	runID := task_common.GetRunID(&task)
	emails := task_common.GetEmailRecipients(task.Username, nil)
	cmd := []string{
		"cipd_bin_packages/luci-auth",
		"context",
		"--",
		"bin/capture_archives_on_workers",
		"-logtostderr",
		"--run_on_gce=" + strconv.FormatBool(task.RunsOnGCEWorkers()),
		"--run_id=" + runID,
		"--pageset_type=" + task.PageSets,
	}

	sTaskID, err := ctutil.TriggerMasterScriptSwarmingTask(ctx, runID, "capture_archives_on_workers", ctutil.CAPTURE_ARCHIVES_MASTER_ISOLATE, task_common.ServiceAccountFile, ctutil.PLATFORM_LINUX, false, cmd, swarmingClient)
	if err != nil {
		return fmt.Errorf("Could not trigger master script for capture_archives_on_workers with cmd %v: %s", cmd, err)
	}
	// Mark task as started in datastore.
	if err := task_common.UpdateTaskSetStarted(ctx, runID, sTaskID, &task); err != nil {
		return fmt.Errorf("Could not mark task as started in datastore: %s", err)
	}
	// Send start email.
	skutil.LogErr(ctfeutil.SendTaskStartEmail(task.DatastoreKey.ID, emails, "Capture archives", runID, "", ""))
	return nil
}

func (task RecreateWebpageArchivesDatastoreTask) SendCompletionEmail(ctx context.Context, completedSuccessfully bool) error {
	runID := task_common.GetRunID(&task)
	emails := task_common.GetEmailRecipients(task.Username, nil)
	emailSubject := fmt.Sprintf("Capture archives Cluster telemetry task has completed (#%d)", task.DatastoreKey.ID)
	failureHtml := ""
	if !completedSuccessfully {
		emailSubject += " with failures"
		failureHtml = ctfeutil.GetFailureEmailHtml(runID)
	}
	bodyTemplate := `
	The Cluster telemetry queued task to capture archives of %s pagesets has completed. %s.<br/>
	%s
	You can schedule more runs <a href="%s">here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, task.PageSets, ctfeutil.GetSwarmingLogsLink(runID), failureHtml, task_common.WebappURL+ctfeutil.ADMIN_TASK_URI)
	if err := ctfeutil.SendEmail(emails, emailSubject, emailBody); err != nil {
		return fmt.Errorf("Error while sending email: %s", err)
	}
	return nil
}

func (task *RecreateWebpageArchivesDatastoreTask) SetCompleted(success bool) {
	task.TsCompleted = ctutil.GetCurrentTsInt64()
	task.Failure = !success
	task.TaskDone = true
}

func addTaskView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(addTaskTemplate, w, r)
}

type AddTaskVars struct {
	task_common.AddTaskCommonVars
}

func (vars *AddTaskVars) IsAdminTask() bool {
	return true
}

// Represents the parameters sent as JSON to the add_recreate_page_sets_task handler.
type AddRecreatePageSetsTaskVars struct {
	AddTaskVars
	PageSets string `json:"page_sets"`
}

func (task *AddRecreatePageSetsTaskVars) GetDatastoreKind() ds.Kind {
	return ds.RECREATE_PAGESETS_TASKS
}

func (task *AddRecreatePageSetsTaskVars) GetPopulatedDatastoreTask(ctx context.Context) (task_common.Task, error) {
	if task.PageSets == "" {
		return nil, fmt.Errorf("Invalid parameters")
	}

	t := &RecreatePageSetsDatastoreTask{
		PageSets:      task.PageSets,
		IsTestPageSet: task.PageSets == ctutil.PAGESET_TYPE_DUMMY_1k || task.PageSets == ctutil.PAGESET_TYPE_MOBILE_DUMMY_1k,
	}
	return t, nil
}

func addRecreatePageSetsTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.AddTaskHandler(w, r, &AddRecreatePageSetsTaskVars{})
}

// Represents the parameters sent as JSON to the add_recreate_webpage_archives_task handler.
type AddRecreateWebpageArchivesTaskVars struct {
	AddTaskVars
	PageSets      string                        `json:"page_sets"`
	ChromiumBuild chromium_builds.DatastoreTask `json:"chromium_build"`
}

func (task *AddRecreateWebpageArchivesTaskVars) GetDatastoreKind() ds.Kind {
	return ds.RECREATE_WEBPAGE_ARCHIVES_TASKS
}

func (task *AddRecreateWebpageArchivesTaskVars) GetPopulatedDatastoreTask(ctx context.Context) (task_common.Task, error) {
	if task.PageSets == "" ||
		task.ChromiumBuild.ChromiumRev == "" ||
		task.ChromiumBuild.SkiaRev == "" {
		return nil, fmt.Errorf("Invalid parameters")
	}
	if err := chromium_builds.Validate(ctx, task.ChromiumBuild); err != nil {
		return nil, err
	}

	t := &RecreateWebpageArchivesDatastoreTask{
		PageSets:      task.PageSets,
		IsTestPageSet: task.PageSets == ctutil.PAGESET_TYPE_DUMMY_1k || task.PageSets == ctutil.PAGESET_TYPE_MOBILE_DUMMY_1k,
		ChromiumRev:   task.ChromiumBuild.ChromiumRev,
		SkiaRev:       task.ChromiumBuild.SkiaRev,
	}
	return t, nil
}

func deleteRecreatePageSetsTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.DeleteTaskHandler(&RecreatePageSetsDatastoreTask{}, w, r)
}

func deleteRecreateWebpageArchivesTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.DeleteTaskHandler(&RecreateWebpageArchivesDatastoreTask{}, w, r)
}

func redoRecreatePageSetsTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.RedoTaskHandler(&RecreatePageSetsDatastoreTask{}, w, r)
}

func redoRecreateWebpageArchivesTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.RedoTaskHandler(&RecreateWebpageArchivesDatastoreTask{}, w, r)
}

func recreatePageSetsRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(recreatePageSetsRunsHistoryTemplate, w, r)
}

func recreateWebpageArchivesRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(recreateWebpageArchivesRunsHistoryTemplate, w, r)
}

func getRecreatePageSetsTasksHandler(w http.ResponseWriter, r *http.Request) {
	task_common.GetTasksHandler(&RecreatePageSetsDatastoreTask{}, w, r)
}

func getRecreateWebpageArchivesTasksHandler(w http.ResponseWriter, r *http.Request) {
	task_common.GetTasksHandler(&RecreateWebpageArchivesDatastoreTask{}, w, r)
}

func AddHandlers(externalRouter *mux.Router) {
	externalRouter.HandleFunc("/"+ctfeutil.ADMIN_TASK_URI, addTaskView).Methods("GET")
	externalRouter.HandleFunc("/"+ctfeutil.RECREATE_PAGE_SETS_RUNS_URI, recreatePageSetsRunsHistoryView).Methods("GET")
	externalRouter.HandleFunc("/"+ctfeutil.RECREATE_WEBPAGE_ARCHIVES_RUNS_URI, recreateWebpageArchivesRunsHistoryView).Methods("GET")

	externalRouter.HandleFunc("/"+ctfeutil.ADD_RECREATE_PAGE_SETS_TASK_POST_URI, addRecreatePageSetsTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.ADD_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, addRecreateWebpageArchivesTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.GET_RECREATE_PAGE_SETS_TASKS_POST_URI, getRecreatePageSetsTasksHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.GET_RECREATE_WEBPAGE_ARCHIVES_TASKS_POST_URI, getRecreateWebpageArchivesTasksHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.DELETE_RECREATE_PAGE_SETS_TASK_POST_URI, deleteRecreatePageSetsTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.DELETE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, deleteRecreateWebpageArchivesTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.REDO_RECREATE_PAGE_SETS_TASK_POST_URI, redoRecreatePageSetsTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.REDO_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, redoRecreateWebpageArchivesTaskHandler).Methods("POST")
}
