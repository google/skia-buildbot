/*
	Handlers and types specific to Metrics analysis tasks.
*/

package metrics_analysis

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"text/template"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"go.skia.org/infra/ct/go/ctfe/chromium_analysis"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/swarming"
	skutil "go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
)

var (
	addTaskTemplate     *template.Template = nil
	runsHistoryTemplate *template.Template = nil

	httpClient = httputils.NewTimeoutClient()
)

func ReloadTemplates(resourcesDir string) {
	addTaskTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "dist", "metrics_analysis.html"),
	))
	runsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "dist", "metrics_analysis_runs.html"),
	))
}

type MetricsAnalysisDatastoreTask struct {
	task_common.CommonCols

	MetricName          string   `json:"metric_name"`
	AnalysisTaskId      string   `json:"analysis_task_id"`
	AnalysisOutputLink  string   `json:"analysis_output_link"`
	BenchmarkArgs       string   `json:"benchmark_args"`
	Description         string   `json:"description"`
	CustomTracesGSPath  string   `json:"custom_traces_gspath"`
	ChromiumPatchGSPath string   `json:"chromium_patch_gspath"`
	CatapultPatchGSPath string   `json:"catapult_patch_gspath"`
	RawOutput           string   `json:"raw_output"`
	ValueColumnName     string   `json:"value_column_name"`
	CCList              []string `json:"cc_list"`
	TaskPriority        int      `json:"task_priority"`
}

func (task MetricsAnalysisDatastoreTask) GetTaskName() string {
	return "MetricsAnalysis"
}

func (task MetricsAnalysisDatastoreTask) GetDescription() string {
	return task.Description
}

func (task MetricsAnalysisDatastoreTask) GetPopulatedAddTaskVars() (task_common.AddTaskVars, error) {
	taskVars := &MetricsAnalysisAddTaskVars{}
	taskVars.Username = task.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(task.RepeatAfterDays, 10)
	taskVars.MetricName = task.MetricName
	taskVars.AnalysisTaskId = task.AnalysisTaskId
	taskVars.AnalysisOutputLink = task.AnalysisOutputLink
	taskVars.ValueColumnName = task.ValueColumnName
	taskVars.BenchmarkArgs = task.BenchmarkArgs
	taskVars.Description = task.Description
	taskVars.CCList = task.CCList
	taskVars.TaskPriority = strconv.Itoa(task.TaskPriority)

	var err error
	taskVars.CustomTraces, err = ctutil.GetPatchFromStorage(task.CustomTracesGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", task.CustomTracesGSPath, err)
	}
	taskVars.ChromiumPatch, err = ctutil.GetPatchFromStorage(task.ChromiumPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", task.ChromiumPatchGSPath, err)
	}
	taskVars.CatapultPatch, err = ctutil.GetPatchFromStorage(task.CatapultPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", task.CatapultPatchGSPath, err)
	}

	return taskVars, nil
}

func (task MetricsAnalysisDatastoreTask) GetResultsLink() string {
	return task.RawOutput
}

func (task MetricsAnalysisDatastoreTask) RunsOnGCEWorkers() bool {
	return true
}

func (task MetricsAnalysisDatastoreTask) GetDatastoreKind() ds.Kind {
	return ds.METRICS_ANALYSIS_TASKS
}

func (task MetricsAnalysisDatastoreTask) Query(it *datastore.Iterator) (interface{}, error) {
	tasks := []*MetricsAnalysisDatastoreTask{}
	for {
		t := &MetricsAnalysisDatastoreTask{}
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

func (task MetricsAnalysisDatastoreTask) Get(c context.Context, key *datastore.Key) (task_common.Task, error) {
	t := &MetricsAnalysisDatastoreTask{}
	if err := ds.DS.Get(c, key, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (task MetricsAnalysisDatastoreTask) TriggerSwarmingTaskAndMail(ctx context.Context, swarmingClient swarming.ApiClient) error {
	runID := task_common.GetRunID(&task)
	emails := task_common.GetEmailRecipients(task.Username, task.CCList)
	cmd := []string{
		"cipd_bin_packages/luci-auth",
		"context",
		"--",
		"bin/metrics_analysis_on_workers",
		"-logtostderr",
		"--metric_name=" + task.MetricName,
		"--analysis_output_link=" + task.AnalysisOutputLink,
		"--benchmark_extra_args=" + task.BenchmarkArgs,
		"--value_column_name=" + task.ValueColumnName,
		"--run_id=" + runID,
		"--task_priority=" + strconv.Itoa(task.TaskPriority),
		"--chromium_patch_gs_path=" + task.ChromiumPatchGSPath,
		"--catapult_patch_gs_path=" + task.CatapultPatchGSPath,
		"--custom_traces_csv_gs_path=" + task.CustomTracesGSPath,
	}

	sTaskID, err := ctutil.TriggerMasterScriptSwarmingTask(ctx, runID, "metrics_analysis_on_workers", ctutil.METRICS_ANALYSIS_MASTER_ISOLATE, task_common.ServiceAccountFile, ctutil.PLATFORM_LINUX, false, cmd, swarmingClient)
	if err != nil {
		return fmt.Errorf("Could not trigger master script for metrics_analysis_on_workers with cmd %v: %s", cmd, err)
	}
	// Mark task as started in datastore.
	if err := task_common.UpdateTaskSetStarted(ctx, runID, sTaskID, &task); err != nil {
		return fmt.Errorf("Could not mark task as started in datastore: %s", err)
	}
	// Send start email.
	skutil.LogErr(ctfeutil.SendTaskStartEmail(task.DatastoreKey.ID, emails, "Metrics analysis", runID, task.Description, ""))
	return nil
}

func (task MetricsAnalysisDatastoreTask) SendCompletionEmail(ctx context.Context, completedSuccessfully bool) error {
	runID := task_common.GetRunID(&task)
	emails := task_common.GetEmailRecipients(task.Username, task.CCList)
	emailSubject := fmt.Sprintf("Metrics analysis cluster telemetry task has completed (#%d)", task.DatastoreKey.ID)
	failureHtml := ""
	viewActionMarkup := ""
	var err error

	if completedSuccessfully {
		if viewActionMarkup, err = email.GetViewActionMarkup(task.RawOutput, "View Results", "Direct link to the CSV results"); err != nil {
			return fmt.Errorf("Failed to get view action markup: %s", err)
		}
	} else {
		emailSubject += " with failures"
		failureHtml = ctfeutil.GetFailureEmailHtml(runID)
		if viewActionMarkup, err = email.GetViewActionMarkup(fmt.Sprintf(ctutil.SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, runID), "View Failure", "Direct link to the swarming logs"); err != nil {
			return fmt.Errorf("Failed to get view action markup: %s", err)
		}
	}

	bodyTemplate := `
	The metrics analysis task has completed. %s.<br/>
	Run description: %s<br/>
	%s
	The CSV output is <a href='%s'>here</a>.<br/>
	The patch(es) you specified are here:
	<a href='%s'>chromium</a>/<a href='%s'>catapult</a>
	<br/>
	Traces used for this run are <a href='%s'>here</a>.
	<br/><br/>
	You can schedule more runs <a href='%s'>here</a>.
	<br/><br/>
	Thanks!
	`
	chromiumPatchLink := ctutil.GCS_HTTP_LINK + path.Join(ctutil.GCSBucketName, task.ChromiumPatchGSPath)
	catapultPatchLink := ctutil.GCS_HTTP_LINK + path.Join(ctutil.GCSBucketName, task.CatapultPatchGSPath)
	tracesLink := ctutil.GCS_HTTP_LINK + path.Join(ctutil.GCSBucketName, task.CustomTracesGSPath)
	emailBody := fmt.Sprintf(bodyTemplate, ctfeutil.GetSwarmingLogsLink(runID), task.Description, failureHtml, task.RawOutput, chromiumPatchLink, catapultPatchLink, tracesLink, task_common.WebappURL+ctfeutil.METRICS_ANALYSIS_URI)
	if err := ctfeutil.SendEmailWithMarkup(emails, emailSubject, emailBody, viewActionMarkup); err != nil {
		return fmt.Errorf("Error while sending email: %s", err)
	}
	return nil
}

func (task *MetricsAnalysisDatastoreTask) SetCompleted(success bool) {
	if success {
		runID := task_common.GetRunID(task)
		task.RawOutput = ctutil.GetMetricsAnalysisOutputLink(runID)
	}
	task.TsCompleted = ctutil.GetCurrentTsInt64()
	task.Failure = !success
	task.TaskDone = true
}

func addTaskView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(addTaskTemplate, w, r)
}

type MetricsAnalysisAddTaskVars struct {
	task_common.AddTaskCommonVars

	MetricName         string   `json:"metric_name"`
	CustomTraces       string   `json:"custom_traces"`
	AnalysisTaskId     string   `json:"analysis_task_id"`
	AnalysisOutputLink string   `json:"analysis_output_link"`
	BenchmarkArgs      string   `json:"benchmark_args"`
	Description        string   `json:"desc"`
	ChromiumPatch      string   `json:"chromium_patch"`
	CatapultPatch      string   `json:"catapult_patch"`
	ValueColumnName    string   `json:"value_column_name"`
	CCList             []string `json:"cc_list"`
	TaskPriority       string   `json:"task_priority"`
}

func (task *MetricsAnalysisAddTaskVars) GetDatastoreKind() ds.Kind {
	return ds.METRICS_ANALYSIS_TASKS
}

func (task *MetricsAnalysisAddTaskVars) GetPopulatedDatastoreTask(ctx context.Context) (task_common.Task, error) {
	if task.MetricName == "" {
		return nil, fmt.Errorf("Must specify metric name")
	}
	if task.CustomTraces == "" && task.AnalysisTaskId == "" {
		return nil, fmt.Errorf("Must specify one of custom traces or analysis task id")
	}
	if task.Description == "" {
		return nil, fmt.Errorf("Must specify description")
	}

	if task.AnalysisTaskId != "" && task.AnalysisTaskId != "0" {
		// Get analysis output link from analysis task id.
		key := ds.NewKey(ds.CHROMIUM_ANALYSIS_TASKS)
		id, err := strconv.ParseInt(task.AnalysisTaskId, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s is not an int64: %s", task.AnalysisTaskId, err)
		}
		key.ID = id
		analysisTask := &chromium_analysis.ChromiumAnalysisDatastoreTask{}
		if err := ds.DS.Get(ctx, key, analysisTask); err != nil {
			return nil, fmt.Errorf("Unable to find requested analysis task id.")
		}
		task.AnalysisOutputLink = analysisTask.RawOutput
	}

	customTracesGSPath, err := ctutil.SavePatchToStorage(task.CustomTraces)
	if err != nil {
		return nil, fmt.Errorf("Could not save custom traces to storage: %s", err)
	}
	chromiumPatchGSPath, err := ctutil.SavePatchToStorage(task.ChromiumPatch)
	if err != nil {
		return nil, fmt.Errorf("Could not save chromium patch to storage: %s", err)
	}
	catapultPatchGSPath, err := ctutil.SavePatchToStorage(task.CatapultPatch)
	if err != nil {
		return nil, fmt.Errorf("Could not save catapult patch to storage: %s", err)
	}

	t := &MetricsAnalysisDatastoreTask{
		MetricName:         task.MetricName,
		AnalysisTaskId:     task.AnalysisTaskId,
		AnalysisOutputLink: task.AnalysisOutputLink,
		ValueColumnName:    task.ValueColumnName,
		BenchmarkArgs:      task.BenchmarkArgs,
		Description:        task.Description,
		CCList:             task.CCList,

		CustomTracesGSPath:  customTracesGSPath,
		ChromiumPatchGSPath: chromiumPatchGSPath,
		CatapultPatchGSPath: catapultPatchGSPath,
	}
	taskPriority, err := strconv.Atoi(task.TaskPriority)
	if err != nil {
		return nil, fmt.Errorf("%s is not int: %s", task.TaskPriority, err)
	}
	if taskPriority == 0 {
		// This should only happen for repeating tasks that were created before
		// support for task priorities was added to CT.
		// Triggering tasks with 0 priority fails in swarming with
		// "priority 0 can only be used for terminate request"
		// Override it to the medium priority.
		taskPriority = ctutil.TASKS_PRIORITY_MEDIUM
	}
	t.TaskPriority = taskPriority
	return t, nil
}

func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.AddTaskHandler(w, r, &MetricsAnalysisAddTaskVars{})
}

func getTasksHandler(w http.ResponseWriter, r *http.Request) {
	task_common.GetTasksHandler(&MetricsAnalysisDatastoreTask{}, w, r)
}

func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.DeleteTaskHandler(&MetricsAnalysisDatastoreTask{}, w, r)
}

func redoTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.RedoTaskHandler(&MetricsAnalysisDatastoreTask{}, w, r)
}

func runsHistoryView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(runsHistoryTemplate, w, r)
}

func AddHandlers(externalRouter *mux.Router) {
	externalRouter.HandleFunc("/"+ctfeutil.METRICS_ANALYSIS_URI, addTaskView).Methods("GET")
	externalRouter.HandleFunc("/"+ctfeutil.METRICS_ANALYSIS_RUNS_URI, runsHistoryView).Methods("GET")

	externalRouter.HandleFunc("/"+ctfeutil.ADD_METRICS_ANALYSIS_TASK_POST_URI, addTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.GET_METRICS_ANALYSIS_TASKS_POST_URI, getTasksHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.DELETE_METRICS_ANALYSIS_TASK_POST_URI, deleteTaskHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.REDO_METRICS_ANALYSIS_TASK_POST_URI, redoTaskHandler).Methods("POST")
}
