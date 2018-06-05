/*
	Handlers and types specific to running admin tasks, including recreating page sets and
	recreating webpage archives.
*/

package admin_tasks

import (
	"context"
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
)

var (
	addTaskTemplate                            *template.Template = nil
	recreatePageSetsRunsHistoryTemplate        *template.Template = nil
	recreateWebpageArchivesRunsHistoryTemplate *template.Template = nil
)

func ReloadTemplates(resourcesDir string) {
	addTaskTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/admin_tasks.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
	recreatePageSetsRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/recreate_page_sets_runs_history.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
	recreateWebpageArchivesRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(resourcesDir, "templates/recreate_webpage_archives_runs_history.html"),
		filepath.Join(resourcesDir, "templates/header.html"),
		filepath.Join(resourcesDir, "templates/titlebar.html"),
	))
}

type RecreatePageSetsDBTask struct {
	task_common.CommonCols

	PageSets string `db:"page_sets"`
}

func (task RecreatePageSetsDBTask) GetTaskName() string {
	return "RecreatePageSets"
}

func (dbTask RecreatePageSetsDBTask) GetPopulatedAddTaskVars() task_common.AddTaskVars {
	taskVars := &AddRecreatePageSetsTaskVars{}
	taskVars.Username = dbTask.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(dbTask.RepeatAfterDays, 10)

	taskVars.PageSets = dbTask.PageSets
	return taskVars
}

func (task RecreatePageSetsDBTask) GetUpdateTaskVars() task_common.UpdateTaskVars {
	return &RecreatePageSetsUpdateVars{}
}

func (task RecreatePageSetsDBTask) RunsOnGCEWorkers() bool {
	return true
}

func (task RecreatePageSetsDBTask) GetDatastoreKind() ds.Kind {
	return ds.RECREATE_PAGESETS_TASKS
}

func (task RecreatePageSetsDBTask) GetResultsLink() string {
	return ""
}

func (task RecreatePageSetsDBTask) Select(it *datastore.Iterator) (interface{}, error) {
	return nil, nil
	//result := []RecreatePageSetsDBTask{}
	//err := db.DB.Select(&result, query, args...)
	//return result, err
}

func (task RecreatePageSetsDBTask) Find(c context.Context, key *datastore.Key) (interface{}, error) {
	t := &RecreatePageSetsDBTask{}
	if err := ds.DS.Get(c, key, t); err != nil {
		return nil, err
	}
	return t, nil
}

type RecreateWebpageArchivesDBTask struct {
	task_common.CommonCols

	PageSets    string `db:"page_sets"`
	ChromiumRev string `db:"chromium_rev"`
	SkiaRev     string `db:"skia_rev"`
}

func (task RecreateWebpageArchivesDBTask) GetTaskName() string {
	return "RecreateWebpageArchives"
}

func (task RecreateWebpageArchivesDBTask) GetResultsLink() string {
	return ""
}

func (dbTask RecreateWebpageArchivesDBTask) GetPopulatedAddTaskVars() task_common.AddTaskVars {
	taskVars := &AddRecreateWebpageArchivesTaskVars{}
	taskVars.Username = dbTask.Username
	taskVars.TsAdded = ctutil.GetCurrentTs()
	taskVars.RepeatAfterDays = strconv.FormatInt(dbTask.RepeatAfterDays, 10)

	taskVars.PageSets = dbTask.PageSets
	taskVars.ChromiumBuild.ChromiumRev = dbTask.ChromiumRev
	taskVars.ChromiumBuild.SkiaRev = dbTask.SkiaRev
	return taskVars
}

func (task RecreateWebpageArchivesDBTask) GetUpdateTaskVars() task_common.UpdateTaskVars {
	return &RecreateWebpageArchivesUpdateVars{}
}

func (task RecreateWebpageArchivesDBTask) RunsOnGCEWorkers() bool {
	return true
}

func (task RecreateWebpageArchivesDBTask) GetDatastoreKind() ds.Kind {
	return ds.RECREATE_WEBPAGE_ARCHIVES_TASKS
}

func (task RecreateWebpageArchivesDBTask) Select(it *datastore.Iterator) (interface{}, error) {
	return nil, nil
	//result := []RecreateWebpageArchivesDBTask{}
	//err := db.DB.Select(&result, query, args...)
	//return result, err
}

func (task RecreateWebpageArchivesDBTask) Find(c context.Context, key *datastore.Key) (interface{}, error) {
	t := &RecreateWebpageArchivesDBTask{}
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
}

func (vars *AddTaskVars) IsAdminTask() bool {
	return true
}

// Represents the parameters sent as JSON to the add_recreate_page_sets_task handler.
type AddRecreatePageSetsTaskVars struct {
	AddTaskVars
	PageSets string `json:"page_sets"`
}

func (task *AddRecreatePageSetsTaskVars) GetPopulatedDatastoreTask() (task_common.Task, error) {
	return nil, nil
}

//func (task *AddRecreatePageSetsTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
//	if task.PageSets == "" {
//		return "", nil, fmt.Errorf("Invalid parameters")
//	}
//	if err := ctfeutil.CheckLengths([]ctfeutil.LengthCheck{{Name: "page_sets", Value: task.PageSets, Limit: 100}}); err != nil {
//		return "", nil, err
//	}
//	return fmt.Sprintf("INSERT INTO %s (username,page_sets,ts_added,repeat_after_days) VALUES (?,?,?,?);",
//			db.TABLE_RECREATE_PAGE_SETS_TASKS),
//		[]interface{}{
//			task.Username,
//			task.PageSets,
//			task.TsAdded,
//			task.RepeatAfterDays,
//		},
//		nil
//}

func addRecreatePageSetsTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.AddTaskHandler(w, r, &AddRecreatePageSetsTaskVars{})
}

// Represents the parameters sent as JSON to the add_recreate_webpage_archives_task handler.
type AddRecreateWebpageArchivesTaskVars struct {
	AddTaskVars
	PageSets      string                 `json:"page_sets"`
	ChromiumBuild chromium_builds.DBTask `json:"chromium_build"`
}

func (task *AddRecreateWebpageArchivesTaskVars) GetPopulatedDatastoreTask() (task_common.Task, error) {
	return nil, nil
}

//func (task *AddRecreateWebpageArchivesTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
//	if task.PageSets == "" ||
//		task.ChromiumBuild.ChromiumRev == "" ||
//		task.ChromiumBuild.SkiaRev == "" {
//		return "", nil, fmt.Errorf("Invalid parameters")
//	}
//	if err := chromium_builds.Validate(task.ChromiumBuild); err != nil {
//		return "", nil, err
//	}
//	if err := ctfeutil.CheckLengths([]ctfeutil.LengthCheck{{Name: "page_sets", Value: task.PageSets, Limit: 100}}); err != nil {
//		return "", nil, err
//	}
//	return fmt.Sprintf("INSERT INTO %s (username,page_sets,chromium_rev,skia_rev,ts_added,repeat_after_days) VALUES (?,?,?,?,?,?);",
//			db.TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS),
//		[]interface{}{
//			task.Username,
//			task.PageSets,
//			task.ChromiumBuild.ChromiumRev,
//			task.ChromiumBuild.SkiaRev,
//			task.TsAdded,
//			task.RepeatAfterDays,
//		},
//		nil
//}

func addRecreateWebpageArchivesTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.AddTaskHandler(w, r, &AddRecreateWebpageArchivesTaskVars{})
}

type RecreatePageSetsUpdateVars struct {
	task_common.UpdateTaskCommonVars
}

func (vars *RecreatePageSetsUpdateVars) UriPath() string {
	return ctfeutil.UPDATE_RECREATE_PAGE_SETS_TASK_POST_URI
}

func (task *RecreatePageSetsUpdateVars) AddUpdatesToDBTask(t task_common.Task) error {
	return nil
}

func updateRecreatePageSetsTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.UpdateTaskHandler(&RecreatePageSetsUpdateVars{}, &RecreatePageSetsDBTask{}, w, r)
}

type RecreateWebpageArchivesUpdateVars struct {
	task_common.UpdateTaskCommonVars
}

func (vars *RecreateWebpageArchivesUpdateVars) UriPath() string {
	return ctfeutil.UPDATE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI
}

func (task *RecreateWebpageArchivesUpdateVars) AddUpdatesToDBTask(t task_common.Task) error {
	return nil
}

func updateRecreateWebpageArchivesTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.UpdateTaskHandler(&RecreateWebpageArchivesUpdateVars{}, &RecreateWebpageArchivesDBTask{}, w, r)
}

func deleteRecreatePageSetsTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.DeleteTaskHandler(&RecreatePageSetsDBTask{}, w, r)
}

func deleteRecreateWebpageArchivesTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.DeleteTaskHandler(&RecreateWebpageArchivesDBTask{}, w, r)
}

func redoRecreatePageSetsTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.RedoTaskHandler(&RecreatePageSetsDBTask{}, w, r)
}

func redoRecreateWebpageArchivesTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.RedoTaskHandler(&RecreateWebpageArchivesDBTask{}, w, r)
}

func recreatePageSetsRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(recreatePageSetsRunsHistoryTemplate, w, r)
}

func recreateWebpageArchivesRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	ctfeutil.ExecuteSimpleTemplate(recreateWebpageArchivesRunsHistoryTemplate, w, r)
}

func getRecreatePageSetsTasksHandler(w http.ResponseWriter, r *http.Request) {
	task_common.GetTasksHandler(&RecreatePageSetsDBTask{}, w, r)
}

func getRecreateWebpageArchivesTasksHandler(w http.ResponseWriter, r *http.Request) {
	task_common.GetTasksHandler(&RecreateWebpageArchivesDBTask{}, w, r)
}

func AddHandlers(r *mux.Router) {
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.ADMIN_TASK_URI, "GET", addTaskView)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.RECREATE_PAGE_SETS_RUNS_URI, "GET", recreatePageSetsRunsHistoryView)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.RECREATE_WEBPAGE_ARCHIVES_RUNS_URI, "GET", recreateWebpageArchivesRunsHistoryView)

	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.ADD_RECREATE_PAGE_SETS_TASK_POST_URI, "POST", addRecreatePageSetsTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.ADD_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, "POST", addRecreateWebpageArchivesTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.GET_RECREATE_PAGE_SETS_TASKS_POST_URI, "POST", getRecreatePageSetsTasksHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.GET_RECREATE_WEBPAGE_ARCHIVES_TASKS_POST_URI, "POST", getRecreateWebpageArchivesTasksHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.DELETE_RECREATE_PAGE_SETS_TASK_POST_URI, "POST", deleteRecreatePageSetsTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.DELETE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, "POST", deleteRecreateWebpageArchivesTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.REDO_RECREATE_PAGE_SETS_TASK_POST_URI, "POST", redoRecreatePageSetsTaskHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.REDO_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, "POST", redoRecreateWebpageArchivesTaskHandler)

	// Do not add force login handler for update methods. They use webhooks for authentication.
	r.HandleFunc("/"+ctfeutil.UPDATE_RECREATE_PAGE_SETS_TASK_POST_URI, updateRecreatePageSetsTaskHandler).Methods("POST")
	r.HandleFunc("/"+ctfeutil.UPDATE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, updateRecreateWebpageArchivesTaskHandler).Methods("POST")
}
