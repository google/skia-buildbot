/*
	Handlers and types specific to running admin tasks, including recreating page sets and
	recreating webpage archives.
*/

package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"text/template"

	"go.skia.org/infra/ct/go/db"
	api "go.skia.org/infra/ct/go/frontend"
)

var (
	adminTasksTemplate                         *template.Template = nil
	recreatePageSetsRunsHistoryTemplate        *template.Template = nil
	recreateWebpageArchivesRunsHistoryTemplate *template.Template = nil
)

func reloadAdminTaskTemplates() {
	adminTasksTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/admin_tasks.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))
	recreatePageSetsRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/recreate_page_sets_runs_history.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))
	recreateWebpageArchivesRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/recreate_webpage_archives_runs_history.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))
}

type RecreatePageSetsDBTask struct {
	CommonCols

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
	CommonCols

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

func adminTasksView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(adminTasksTemplate, w, r)
}

type AdminTaskVars struct {
	AddTaskCommonVars
}

func (vars *AdminTaskVars) IsAdminTask() bool {
	return true
}

// Represents the parameters sent as JSON to the add_recreate_page_sets_task handler.
type AddRecreatePageSetsTaskVars struct {
	AdminTaskVars
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
	addTaskHandler(w, r, &AddRecreatePageSetsTaskVars{})
}

// Represents the parameters sent as JSON to the add_recreate_webpage_archives_task handler.
type AddRecreateWebpageArchivesTaskVars struct {
	AdminTaskVars
	PageSets      string              `json:"page_sets"`
	ChromiumBuild ChromiumBuildDBTask `json:"chromium_build"`
}

func (task *AddRecreateWebpageArchivesTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.PageSets == "" ||
		task.ChromiumBuild.ChromiumRev == "" ||
		task.ChromiumBuild.SkiaRev == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	if err := validateChromiumBuild(task.ChromiumBuild); err != nil {
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
	addTaskHandler(w, r, &AddRecreateWebpageArchivesTaskVars{})
}

// Define api.RecreatePageSetsUpdateVars in this package so we can add methods.
type RecreatePageSetsUpdateVars struct {
	api.RecreatePageSetsUpdateVars
}

func (task *RecreatePageSetsUpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
	return nil, nil, nil
}

func updateRecreatePageSetsTaskHandler(w http.ResponseWriter, r *http.Request) {
	updateTaskHandler(&RecreatePageSetsUpdateVars{}, db.TABLE_RECREATE_PAGE_SETS_TASKS, w, r)
}

// Define api.RecreateWebpageArchivesUpdateVars in this package so we can add methods.
type RecreateWebpageArchivesUpdateVars struct {
	api.RecreateWebpageArchivesUpdateVars
}

func (task *RecreateWebpageArchivesUpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
	return nil, nil, nil
}

func updateRecreateWebpageArchivesTaskHandler(w http.ResponseWriter, r *http.Request) {
	updateTaskHandler(&RecreateWebpageArchivesUpdateVars{}, db.TABLE_RECREATE_PAGE_SETS_TASKS, w, r)
}

func deleteRecreatePageSetsTaskHandler(w http.ResponseWriter, r *http.Request) {
	deleteTaskHandler(&RecreatePageSetsDBTask{}, w, r)
}

func deleteRecreateWebpageArchivesTaskHandler(w http.ResponseWriter, r *http.Request) {
	deleteTaskHandler(&RecreateWebpageArchivesDBTask{}, w, r)
}

func recreatePageSetsRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(recreatePageSetsRunsHistoryTemplate, w, r)
}

func recreateWebpageArchivesRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(recreateWebpageArchivesRunsHistoryTemplate, w, r)
}

func getRecreatePageSetsTasksHandler(w http.ResponseWriter, r *http.Request) {
	getTasksHandler(&RecreatePageSetsDBTask{}, w, r)
}

func getRecreateWebpageArchivesTasksHandler(w http.ResponseWriter, r *http.Request) {
	getTasksHandler(&RecreateWebpageArchivesDBTask{}, w, r)
}
