/*
	Handlers and types specific to running admin tasks, including recreating page sets and
	recreating webpage archives.
*/

package admin_tasks

import (
	"fmt"
	"net/http"
	"path/filepath"
	"text/template"

	"github.com/gorilla/mux"

	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	"go.skia.org/infra/ct/go/db"
	api "go.skia.org/infra/ct/go/frontend"
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

func (task RecreatePageSetsDBTask) TableName() string {
	return db.TABLE_RECREATE_PAGE_SETS_TASKS
}

func (task RecreatePageSetsDBTask) Select(query string, args ...interface{}) (interface{}, error) {
	result := []RecreatePageSetsDBTask{}
	err := db.DB.Select(&result, query, args...)
	return result, err
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

func (task RecreateWebpageArchivesDBTask) TableName() string {
	return db.TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS
}

func (task RecreateWebpageArchivesDBTask) Select(query string, args ...interface{}) (interface{}, error) {
	result := []RecreateWebpageArchivesDBTask{}
	err := db.DB.Select(&result, query, args...)
	return result, err
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

func (task *AddRecreatePageSetsTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.PageSets == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	return fmt.Sprintf("INSERT INTO %s (username,page_sets,ts_added,repeat_after_days) VALUES (?,?,?,?);",
			db.TABLE_RECREATE_PAGE_SETS_TASKS),
		[]interface{}{
			task.Username,
			task.PageSets,
			task.TsAdded,
			task.RepeatAfterDays,
		},
		nil
}

func addRecreatePageSetsTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.AddTaskHandler(w, r, &AddRecreatePageSetsTaskVars{})
}

// Represents the parameters sent as JSON to the add_recreate_webpage_archives_task handler.
type AddRecreateWebpageArchivesTaskVars struct {
	AddTaskVars
	PageSets      string                 `json:"page_sets"`
	ChromiumBuild chromium_builds.DBTask `json:"chromium_build"`
}

func (task *AddRecreateWebpageArchivesTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.PageSets == "" ||
		task.ChromiumBuild.ChromiumRev == "" ||
		task.ChromiumBuild.SkiaRev == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	if err := chromium_builds.Validate(task.ChromiumBuild); err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("INSERT INTO %s (username,page_sets,chromium_rev,skia_rev,ts_added,repeat_after_days) VALUES (?,?,?,?,?,?);",
			db.TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS),
		[]interface{}{
			task.Username,
			task.PageSets,
			task.ChromiumBuild.ChromiumRev,
			task.ChromiumBuild.SkiaRev,
			task.TsAdded,
			task.RepeatAfterDays,
		},
		nil
}

func addRecreateWebpageArchivesTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.AddTaskHandler(w, r, &AddRecreateWebpageArchivesTaskVars{})
}

// Define api.RecreatePageSetsUpdateVars in this package so we can add methods.
type RecreatePageSetsUpdateVars struct {
	api.RecreatePageSetsUpdateVars
}

func (task *RecreatePageSetsUpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
	return nil, nil, nil
}

func updateRecreatePageSetsTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.UpdateTaskHandler(&RecreatePageSetsUpdateVars{}, db.TABLE_RECREATE_PAGE_SETS_TASKS, w, r)
}

// Define api.RecreateWebpageArchivesUpdateVars in this package so we can add methods.
type RecreateWebpageArchivesUpdateVars struct {
	api.RecreateWebpageArchivesUpdateVars
}

func (task *RecreateWebpageArchivesUpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
	return nil, nil, nil
}

func updateRecreateWebpageArchivesTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.UpdateTaskHandler(&RecreateWebpageArchivesUpdateVars{}, db.TABLE_RECREATE_PAGE_SETS_TASKS, w, r)
}

func deleteRecreatePageSetsTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.DeleteTaskHandler(&RecreatePageSetsDBTask{}, w, r)
}

func deleteRecreateWebpageArchivesTaskHandler(w http.ResponseWriter, r *http.Request) {
	task_common.DeleteTaskHandler(&RecreateWebpageArchivesDBTask{}, w, r)
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
	r.HandleFunc("/"+api.ADMIN_TASK_URI, addTaskView).Methods("GET")
	r.HandleFunc("/"+api.RECREATE_PAGE_SETS_RUNS_URI, recreatePageSetsRunsHistoryView).Methods("GET")
	r.HandleFunc("/"+api.RECREATE_WEBPAGE_ARCHIVES_RUNS_URI, recreateWebpageArchivesRunsHistoryView).Methods("GET")
	r.HandleFunc("/"+api.ADD_RECREATE_PAGE_SETS_TASK_POST_URI, addRecreatePageSetsTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.ADD_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, addRecreateWebpageArchivesTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.GET_RECREATE_PAGE_SETS_TASKS_POST_URI, getRecreatePageSetsTasksHandler).Methods("POST")
	r.HandleFunc("/"+api.GET_RECREATE_WEBPAGE_ARCHIVES_TASKS_POST_URI, getRecreateWebpageArchivesTasksHandler).Methods("POST")
	r.HandleFunc("/"+api.UPDATE_RECREATE_PAGE_SETS_TASK_POST_URI, updateRecreatePageSetsTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.UPDATE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, updateRecreateWebpageArchivesTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.DELETE_RECREATE_PAGE_SETS_TASK_POST_URI, deleteRecreatePageSetsTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.DELETE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, deleteRecreateWebpageArchivesTaskHandler).Methods("POST")
}
