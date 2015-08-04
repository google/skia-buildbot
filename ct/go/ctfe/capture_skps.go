/*
	Handlers and types specific to capturing SKP repositories.
*/

package main

import (
	"fmt"
	"net/http"
	"text/template"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/ct/go/db"
	api "go.skia.org/infra/ct/go/frontend"
)

var (
	captureSkpsTemplate           *template.Template = nil
	captureSkpRunsHistoryTemplate *template.Template = nil
)

type CaptureSkpsDBTask struct {
	CommonCols

	PageSets    string `db:"page_sets"`
	ChromiumRev string `db:"chromium_rev"`
	SkiaRev     string `db:"skia_rev"`
	Description string `db:"description"`
}

func (task CaptureSkpsDBTask) GetTaskName() string {
	return "CaptureSkps"
}

func (task CaptureSkpsDBTask) TableName() string {
	return db.TABLE_CAPTURE_SKPS_TASKS
}

func (task CaptureSkpsDBTask) Select(query string, args ...interface{}) (interface{}, error) {
	result := []CaptureSkpsDBTask{}
	err := db.DB.Select(&result, query, args...)
	return result, err
}

func captureSkpsView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(captureSkpsTemplate, w, r)
}

type AddCaptureSkpsTaskVars struct {
	AddTaskCommonVars

	PageSets      string              `json:"page_sets"`
	ChromiumBuild ChromiumBuildDBTask `json:"chromium_build"`
	Description   string              `json:"desc"`
}

func (task *AddCaptureSkpsTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.PageSets == "" ||
		task.ChromiumBuild.ChromiumRev == "" ||
		task.ChromiumBuild.SkiaRev == "" ||
		task.Description == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	if err := validateChromiumBuild(task.ChromiumBuild); err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("INSERT INTO %s (username,page_sets,chromium_rev,skia_rev,description,ts_added, repeat_after_days) VALUES (?,?,?,?,?,?,?);",
			db.TABLE_CAPTURE_SKPS_TASKS),
		[]interface{}{
			task.Username,
			task.PageSets,
			task.ChromiumBuild.ChromiumRev,
			task.ChromiumBuild.SkiaRev,
			task.Description,
			task.TsAdded,
			task.RepeatAfterDays,
		},
		nil
}

func addCaptureSkpsTaskHandler(w http.ResponseWriter, r *http.Request) {
	addTaskHandler(w, r, &AddCaptureSkpsTaskVars{})
}

func getCaptureSkpTasksHandler(w http.ResponseWriter, r *http.Request) {
	getTasksHandler(&CaptureSkpsDBTask{}, w, r)
}

// Validate that the given skpRepository exists in the DB.
func validateSkpRepository(skpRepository CaptureSkpsDBTask) error {
	rowCount := []int{}
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE page_sets = ? AND chromium_rev = ? AND skia_rev = ? AND ts_completed IS NOT NULL AND failure = 0", db.TABLE_CAPTURE_SKPS_TASKS)
	if err := db.DB.Select(&rowCount, query, skpRepository.PageSets, skpRepository.ChromiumRev, skpRepository.SkiaRev); err != nil || len(rowCount) < 1 || rowCount[0] == 0 {
		glog.Info(err)
		return fmt.Errorf("Unable to validate skp_repository parameter %v", skpRepository)
	}
	return nil
}

// Define api.CaptureSkpsUpdateVars in this package so we can add methods.
type CaptureSkpsUpdateVars struct {
	api.CaptureSkpsUpdateVars
}

func (task *CaptureSkpsUpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
	return nil, nil, nil
}

func updateCaptureSkpsTaskHandler(w http.ResponseWriter, r *http.Request) {
	updateTaskHandler(&CaptureSkpsUpdateVars{}, db.TABLE_CAPTURE_SKPS_TASKS, w, r)
}

func deleteCaptureSkpsTaskHandler(w http.ResponseWriter, r *http.Request) {
	deleteTaskHandler(&CaptureSkpsDBTask{}, w, r)
}

func captureSkpRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(captureSkpRunsHistoryTemplate, w, r)
}
