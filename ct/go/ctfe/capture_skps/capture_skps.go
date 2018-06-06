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
	"google.golang.org/api/iterator"

	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/ds"
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

// Obviously change this to DatastoreTask.
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

func (task DatastoreTask) GetResultsLink() string {
	return ""
}

func (dbTask *DatastoreTask) GetPopulatedAddTaskVars() (task_common.AddTaskVars, error) {
	taskVars := &AddTaskVars{}
	taskVars.Username = dbTask.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(dbTask.RepeatAfterDays, 10)
	taskVars.PageSets = dbTask.PageSets
	taskVars.ChromiumBuild.ChromiumRev = dbTask.ChromiumRev
	taskVars.ChromiumBuild.SkiaRev = dbTask.SkiaRev
	taskVars.Description = dbTask.Description
	return taskVars, nil
}

func (task DatastoreTask) GetUpdateTaskVars() task_common.UpdateTaskVars {
	return &UpdateVars{}
}

func (task DatastoreTask) RunsOnGCEWorkers() bool {
	// Capture SKP tasks need to run on bare-metal machines because they have
	// the right font packages installed.
	return false
}

func (task DatastoreTask) GetDatastoreKind() ds.Kind {
	return ds.CAPTURE_SKPS_TASKS
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

// Think of where all to put noindex sutff. like datastore:",noindex"
type AddTaskVars struct {
	task_common.AddTaskCommonVars

	PageSets      string                        `json:"page_sets"`
	ChromiumBuild chromium_builds.DatastoreTask `json:"chromium_build"`
	Description   string                        `json:"desc"`
}

func (task *AddTaskVars) GetDatastoreKind() ds.Kind {
	return ds.CAPTURE_SKPS_TASKS
}

// Except ID ofcourse
func (task *AddTaskVars) GetPopulatedDatastoreTask() (task_common.Task, error) {

	if task.PageSets == "" ||
		task.ChromiumBuild.ChromiumRev == "" ||
		task.ChromiumBuild.SkiaRev == "" ||
		task.Description == "" {
		return nil, fmt.Errorf("Invalid parameters")
	}
	// TODO(rmistry): Uncomment this!
	//if err := chromium_builds.Validate(task.ChromiumBuild); err != nil {
	//	return nil, err
	//}

	id, err := task_common.GetNextId(ds.CAPTURE_SKPS_TASKS, &DatastoreTask{})
	if err != nil {
		return nil, fmt.Errorf("Could not get highest id: %s", err)
	}

	t := &DatastoreTask{
		PageSets:      task.PageSets,
		IsTestPageSet: task.PageSets == ctutil.PAGESET_TYPE_DUMMY_1k,
		ChromiumRev:   task.ChromiumBuild.ChromiumRev,
		SkiaRev:       task.ChromiumBuild.SkiaRev,
		Description:   task.Description,
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

//// Rename to get populated Datastore Task
//func (task *AddTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
//	if task.PageSets == "" ||
//		task.ChromiumBuild.ChromiumRev == "" ||
//		task.ChromiumBuild.SkiaRev == "" ||
//		task.Description == "" {
//		return "", nil, fmt.Errorf("Invalid parameters")
//	}
//	if err := chromium_builds.Validate(task.ChromiumBuild); err != nil {
//		return "", nil, err
//	}
//	if err := ctfeutil.CheckLengths([]ctfeutil.LengthCheck{
//		{Name: "page_sets", Value: task.PageSets, Limit: 100},
//		{Name: "desc", Value: task.Description, Limit: 255},
//	}); err != nil {
//		return "", nil, err
//	}
//	return fmt.Sprintf("INSERT INTO %s (username,page_sets,chromium_rev,skia_rev,description,ts_added, repeat_after_days) VALUES (?,?,?,?,?,?,?);",
//			db.TABLE_CAPTURE_SKPS_TASKS),
//		[]interface{}{
//			task.Username,
//			task.PageSets,
//			task.ChromiumBuild.ChromiumRev,
//			task.ChromiumBuild.SkiaRev,
//			task.Description,
//			task.TsAdded,
//			task.RepeatAfterDays,
//		},
//		nil
//}

func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.AddTaskHandler(w, r, &AddTaskVars{})
}

func getTasksHandler(w http.ResponseWriter, r *http.Request) {
	task_common.GetTasksHandler(&DatastoreTask{}, w, r)
}

// Validate that the given skpRepository exists in the DB.
// rmistry - where is this used? maybe from lua.. do it later!
func Validate(skpRepository DatastoreTask) error {
	//rowCount := []int{}
	//query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE page_sets = ? AND chromium_rev = ? AND skia_rev = ? AND ts_completed IS NOT NULL AND failure = 0", db.TABLE_CAPTURE_SKPS_TASKS)
	//if err := db.DB.Select(&rowCount, query, skpRepository.PageSets, skpRepository.ChromiumRev, skpRepository.SkiaRev); err != nil || len(rowCount) < 1 || rowCount[0] == 0 {
	//	sklog.Info(err)
	//	return fmt.Errorf("Unable to validate skp_repository parameter %v", skpRepository)
	//}
	return nil
}

type UpdateVars struct {
	task_common.UpdateTaskCommonVars
}

func (vars *UpdateVars) UriPath() string {
	return ctfeutil.UPDATE_CAPTURE_SKPS_TASK_POST_URI
}

func (task *UpdateVars) AddUpdatesToDatastoreTask(t task_common.Task) error {
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
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.CAPTURE_SKPS_URI, "GET", addTaskView)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.CAPTURE_SKPS_RUNS_URI, "GET", runsHistoryView)

	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.ADD_CAPTURE_SKPS_TASK_POST_URI, "POST", addTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.GET_CAPTURE_SKPS_TASKS_POST_URI, "POST", getTasksHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.DELETE_CAPTURE_SKPS_TASK_POST_URI, "POST", deleteTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.REDO_CAPTURE_SKPS_TASK_POST_URI, "POST", redoTaskHandler)

	// Do not add force login handler for update methods. They use webhooks for authentication.
	r.HandleFunc("/"+ctfeutil.UPDATE_CAPTURE_SKPS_TASK_POST_URI, updateTaskHandler).Methods("POST")
}
