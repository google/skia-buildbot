/*
	Handlers and types specific to Metrics analysis tasks.
*/

package metrics_analysis

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"text/template"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/ct/go/ctfe/chromium_analysis"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/httputils"
)

var (
	addTaskTemplate     *template.Template = nil
	runsHistoryTemplate *template.Template = nil

	httpClient = httputils.NewTimeoutClient()
)

func ReloadTemplates(resourcesDir string) {
	addTaskTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/metrics_analysis.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
	runsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/metrics_analysis_runs_history.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
}

type DatastoreTask struct {
	task_common.CommonCols

	MetricName          string
	CustomTraces        string
	AnalysisTaskId      string
	AnalysisOutputLink  string
	BenchmarkArgs       string
	Description         string
	ChromiumPatchGSPath string
	CatapultPatchGSPath string
	RawOutput           string

	ChromiumPatch string `datastore:"-"`
	CatapultPatch string `datastore:"-"`
}

func (task DatastoreTask) GetTaskName() string {
	return "MetricsAnalysis"
}

func (task DatastoreTask) GetPopulatedAddTaskVars() (task_common.AddTaskVars, error) {
	taskVars := &AddTaskVars{}
	taskVars.Username = task.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(task.RepeatAfterDays, 10)
	taskVars.MetricName = task.MetricName
	taskVars.CustomTraces = task.CustomTraces
	taskVars.AnalysisTaskId = task.AnalysisTaskId
	taskVars.AnalysisOutputLink = task.AnalysisOutputLink
	taskVars.BenchmarkArgs = task.BenchmarkArgs
	taskVars.Description = task.Description

	var err error
	taskVars.ChromiumPatch, err = ctutil.GetPatchFromStorage(task.ChromiumPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", task.ChromiumPatchGSPath)
	}
	taskVars.CatapultPatch, err = ctutil.GetPatchFromStorage(task.CatapultPatchGSPath)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %s: %s", task.CatapultPatchGSPath)
	}

	return taskVars, nil
}

func (task DatastoreTask) GetResultsLink() string {
	return task.RawOutput
}

func (task DatastoreTask) GetUpdateTaskVars() task_common.UpdateTaskVars {
	return &UpdateVars{}
}

func (task DatastoreTask) RunsOnGCEWorkers() bool {
	return true
}

func (task DatastoreTask) GetDatastoreKind() ds.Kind {
	return ds.METRICS_ANALYSIS_TASKS
}

func (task DatastoreTask) Select(it *datastore.Iterator) (interface{}, error) {
	tasks := []*DatastoreTask{}
	for {
		t := &DatastoreTask{}
		k, err := it.Next(t)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of tasks: %s", err)
		}
		t.DatastoreId = k.ID
		t.ChromiumPatch, err = ctutil.GetPatchFromStorage(t.ChromiumPatchGSPath)
		if err != nil {
			return nil, fmt.Errorf("Could not read from %s: %s", t.ChromiumPatchGSPath)
		}
		t.CatapultPatch, err = ctutil.GetPatchFromStorage(t.CatapultPatchGSPath)
		if err != nil {
			return nil, fmt.Errorf("Could not read from %s: %s", t.CatapultPatchGSPath)
		}
		tasks = append(tasks, t)
	}

	return tasks, nil
}

func (task DatastoreTask) Find(c context.Context, key *datastore.Key) (interface{}, error) {
	t := &DatastoreTask{}
	if err := ds.DS.Get(c, key, t); err != nil {
		return nil, err
	}
	return t, nil
}

func addTaskView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(addTaskTemplate, w, r)
}

type AddTaskVars struct {
	task_common.AddTaskCommonVars

	MetricName         string `json:"metric_name"`
	CustomTraces       string `json:"custom_traces"`
	AnalysisTaskId     string `json:"analysis_task_id"`
	AnalysisOutputLink string `json:"analysis_output_link"`
	BenchmarkArgs      string `json:"benchmark_args"`
	Description        string `json:"desc"`
	ChromiumPatch      string `json:"chromium_patch"`
	CatapultPatch      string `json:"catapult_patch"`
}

func (task *AddTaskVars) GetDatastoreKind() ds.Kind {
	return ds.METRICS_ANALYSIS_TASKS
}

func (task *AddTaskVars) GetPopulatedDatastoreTask() (task_common.Task, error) {
	if task.MetricName == "" {
		return nil, fmt.Errorf("Must specify metric name")
	}
	if task.CustomTraces == "" && task.AnalysisTaskId == "" {
		return nil, fmt.Errorf("Must specify one of custom traces or analysis task id")
	}
	if task.Description == "" {
		return nil, fmt.Errorf("Must specify description")
	}

	// rmistry rmistry rmistry - fix this with a query. need to test it e2e as well!
	if task.AnalysisTaskId != "" && task.AnalysisTaskId != "0" {
		// Get analysis output link from analysis task id.
		outputLinks := []string{}
		q := ds.NewQuery(ds.CHROMIUM_ANALYSIS_TASKS).EventualConsistency()
		id, err := strconv.Atoi(task.AnalysisTaskId)
		if err != nil {
			return nil, fmt.Errorf("%s is not an int: %s", task.AnalysisTaskId, err)
		}
		q = q.Filter("Id =", id)
		it := ds.DS.Run(context.Background(), q)
		for {
			t := &chromium_analysis.DatastoreTask{}
			_, err := it.Next(t)
			if err == iterator.Done {
				break
			} else if err != nil {
				return nil, fmt.Errorf("Failed to retrieve list of tasks: %s", err)
			}
			outputLinks = append(outputLinks, t.RawOutput)
		}
		if len(outputLinks) != 1 {
			return nil, fmt.Errorf("Unable to find requested analysis task id.")
		}
		task.AnalysisOutputLink = outputLinks[0]
	}

	chromiumPatchGSPath, err := ctutil.SavePatchToStorage(task.ChromiumPatch)
	if err != nil {
		return nil, fmt.Errorf("Could not save chromium patch to storage: %s", err)
	}
	catapultPatchGSPath, err := ctutil.SavePatchToStorage(task.CatapultPatch)
	if err != nil {
		return nil, fmt.Errorf("Could not save catapult patch to storage: %s", err)
	}

	id, err := task_common.GetNextId(ds.METRICS_ANALYSIS_TASKS, &DatastoreTask{})
	if err != nil {
		return nil, fmt.Errorf("Could not get highest id: %s", err)
	}

	t := &DatastoreTask{
		MetricName:          task.MetricName,
		CustomTraces:        task.CustomTraces,
		AnalysisTaskId:      task.AnalysisTaskId,
		AnalysisOutputLink:  task.AnalysisOutputLink,
		BenchmarkArgs:       task.BenchmarkArgs,
		Description:         task.Description,
		ChromiumPatchGSPath: chromiumPatchGSPath,
		CatapultPatchGSPath: catapultPatchGSPath,
	}
	tsAdded, err := strconv.ParseInt(task.TsAdded, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s is not int64: %s", task.TsAdded, err)
	}
	t.TsAdded = tsAdded
	t.Username = task.Username
	t.Id = id
	repeatAfterDays, err := strconv.ParseInt(task.RepeatAfterDays, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s is not int64: %s", task.RepeatAfterDays, err)
	}
	t.RepeatAfterDays = repeatAfterDays
	return t, nil
}

func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.AddTaskHandler(w, r, &AddTaskVars{})
}

func getTasksHandler(w http.ResponseWriter, r *http.Request) {
	task_common.GetTasksHandler(&DatastoreTask{}, w, r)
}

type UpdateVars struct {
	task_common.UpdateTaskCommonVars

	RawOutput string
}

func (vars *UpdateVars) UriPath() string {
	return ctfeutil.UPDATE_METRICS_ANALYSIS_TASK_POST_URI
}

func (vars *UpdateVars) AddUpdatesToDatastoreTask(t task_common.Task) error {
	task := t.(*DatastoreTask)
	if vars.RawOutput != "" {
		task.RawOutput = vars.RawOutput
	}
	return nil
}

func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.UpdateTaskHandler(&UpdateVars{}, &DatastoreTask{}, w, r)
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

func AddHandlers(r *mux.Router) {
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.METRICS_ANALYSIS_URI, "GET", addTaskView)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.METRICS_ANALYSIS_RUNS_URI, "GET", runsHistoryView)

	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.ADD_METRICS_ANALYSIS_TASK_POST_URI, "POST", addTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.GET_METRICS_ANALYSIS_TASKS_POST_URI, "POST", getTasksHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.DELETE_METRICS_ANALYSIS_TASK_POST_URI, "POST", deleteTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.REDO_METRICS_ANALYSIS_TASK_POST_URI, "POST", redoTaskHandler)

	// Do not add force login handler for update methods. They use webhooks for authentication.
	r.HandleFunc("/"+ctfeutil.UPDATE_METRICS_ANALYSIS_TASK_POST_URI, updateTaskHandler).Methods("POST")
}
