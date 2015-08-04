/*
	Handlers, types, and functions common to all types of tasks.
*/

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/ct/go/db"
	api "go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/login"
	skutil "go.skia.org/infra/go/util"
)

const (
	// Default page size used for pagination.
	DEFAULT_PAGE_SIZE = 5

	// Maximum page size used for pagination.
	MAX_PAGE_SIZE = 100
)

var (
	taskTables = []string{
		db.TABLE_CHROMIUM_PERF_TASKS,
		db.TABLE_CAPTURE_SKPS_TASKS,
		db.TABLE_LUA_SCRIPT_TASKS,
		db.TABLE_CHROMIUM_BUILD_TASKS,
		db.TABLE_RECREATE_PAGE_SETS_TASKS,
		db.TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS,
	}

	runsHistoryTemplate  *template.Template = nil
	pendingTasksTemplate *template.Template = nil
)

type CommonCols struct {
	Id              int64         `db:"id"`
	TsAdded         sql.NullInt64 `db:"ts_added"`
	TsStarted       sql.NullInt64 `db:"ts_started"`
	TsCompleted     sql.NullInt64 `db:"ts_completed"`
	Username        string        `db:"username"`
	Failure         sql.NullBool  `db:"failure"`
	RepeatAfterDays sql.NullInt64 `db:"repeat_after_days"`
}

type Task interface {
	GetCommonCols() *CommonCols
	GetTaskName() string
	TableName() string
	// Returns a slice of the struct type.
	Select(query string, args ...interface{}) (interface{}, error)
}

func (dbrow *CommonCols) GetCommonCols() *CommonCols {
	return dbrow
}

// Takes the result of Task.Select and returns a slice of Tasks containing the same objects.
func asTaskSlice(selectResult interface{}) []Task {
	sliceValue := reflect.ValueOf(selectResult)
	len := sliceValue.Len()
	result := make([]Task, len)
	for i := 0; i < len; i++ {
		result[i] = sliceValue.Index(i).Addr().Interface().(Task)
	}
	return result
}

// Data included in all tasks; set by addTaskHandler.
type AddTaskCommonVars struct {
	Username        string
	TsAdded         string
	RepeatAfterDays string `json:"repeat_after_days"`
}

type AddTaskVars interface {
	GetAddTaskCommonVars() *AddTaskCommonVars
	IsAdminTask() bool
	GetInsertQueryAndBinds() (string, []interface{}, error)
}

func (vars *AddTaskCommonVars) GetAddTaskCommonVars() *AddTaskCommonVars {
	return vars
}

func (vars *AddTaskCommonVars) IsAdminTask() bool {
	return false
}

func addTaskHandler(w http.ResponseWriter, r *http.Request, task AddTaskVars) {
	if !userHasEditRights(r) {
		skutil.ReportError(w, r, fmt.Errorf("Must have google or chromium account to add tasks"), "")
		return
	}
	if task.IsAdminTask() && !userHasAdminRights(r) {
		skutil.ReportError(w, r, fmt.Errorf("Must be admin to add admin tasks; contact rmistry@"), "")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to add %T task", task))
		return
	}
	defer skutil.Close(r.Body)

	task.GetAddTaskCommonVars().Username = login.LoggedInAs(r)
	task.GetAddTaskCommonVars().TsAdded = api.GetCurrentTs()

	query, binds, err := task.GetInsertQueryAndBinds()
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to marshal %T task", task))
		return
	}
	if _, err = db.DB.Exec(query, binds...); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to insert %T task", task))
		return
	}
}

// Returns true if the string is non-empty, unless strconv.ParseBool parses the string as false.
func parseBoolFormValue(string string) bool {
	if string == "" {
		return false
	} else if val, err := strconv.ParseBool(string); val == false && err == nil {
		return false
	} else {
		return true
	}
}

func dbTaskQuery(prototype Task, username string, successful bool, includeCompleted bool, includeFutureRuns bool, countQuery bool, offset int, size int) (string, []interface{}) {
	args := []interface{}{}
	query := "SELECT "
	if countQuery {
		query += "COUNT(*)"
	} else {
		query += "*"
	}
	query += fmt.Sprintf(" FROM %s", prototype.TableName())
	clauses := []string{}
	if username != "" {
		clauses = append(clauses, "username=?")
		args = append(args, username)
	}
	if successful {
		clauses = append(clauses, "(ts_completed IS NOT NULL AND failure = 0)")
	}
	if !includeCompleted {
		clauses = append(clauses, "ts_completed IS NULL")
	}
	if includeFutureRuns {
		clauses = append(clauses, "(repeat_after_days != 0 AND ts_completed IS NOT NULL)")
	}
	if len(clauses) > 0 {
		query += " WHERE "
		query += strings.Join(clauses, " AND ")
	}
	if !countQuery {
		query += " ORDER BY id DESC LIMIT ?,?"
		args = append(args, offset, size)
	}
	return query, args
}

func getTasksHandler(prototype Task, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Filter by either username or not started yet.
	username := r.FormValue("username")
	successful := parseBoolFormValue(r.FormValue("successful"))
	includeCompleted := !parseBoolFormValue(r.FormValue("not_completed"))
	includeFutureRuns := parseBoolFormValue(r.FormValue("include_future_runs"))
	if successful && !includeCompleted {
		skutil.ReportError(w, r, fmt.Errorf("Inconsistent params: successful %v not_completed %v", r.FormValue("successful"), r.FormValue("not_completed")), "")
		return
	}
	offset, size, err := skutil.PaginationParams(r.URL.Query(), 0, DEFAULT_PAGE_SIZE, MAX_PAGE_SIZE)
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to get pagination params: %v", err))
		return
	}
	query, args := dbTaskQuery(prototype, username, successful, includeCompleted, includeFutureRuns, false, offset, size)
	glog.Infof("Running %s", query)
	data, err := prototype.Select(query, args...)
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to query %s tasks: %v", prototype.GetTaskName(), err))
		return
	}

	query, args = dbTaskQuery(prototype, username, successful, includeCompleted, includeFutureRuns, true, 0, 0)
	// Get the total count.
	glog.Infof("Running %s", query)
	countVal := []int{}
	if err := db.DB.Select(&countVal, query, args...); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to query %s tasks: %v", prototype.GetTaskName(), err))
		return
	}

	pagination := &skutil.ResponsePagination{
		Offset: offset,
		Size:   size,
		Total:  countVal[0],
	}
	type Permissions struct {
		DeleteAllowed bool
	}
	tasks := asTaskSlice(data)
	permissions := make([]Permissions, len(tasks))
	for i := 0; i < len(tasks); i++ {
		deleteAllowed, _ := canDeleteTask(tasks[i], r)
		permissions[i] = Permissions{DeleteAllowed: deleteAllowed}
	}
	jsonResponse := map[string]interface{}{
		"data":        data,
		"permissions": permissions,
		"pagination":  pagination,
	}
	if err := json.NewEncoder(w).Encode(jsonResponse); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

// Add methods for working with the database to api.UpdateTaskVars.
type UpdateTaskVars interface {
	api.UpdateTaskVars
	// Produces SQL query clauses and binds for fields not in api.UpdateTaskCommonVars. First return
	// value is a slice of strings like "results = ?". Second return value contains a value for
	// each "?" bind.
	GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error)
}

func getUpdateQueryAndBinds(vars UpdateTaskVars, tableName string) (string, []interface{}, error) {
	common := vars.GetUpdateTaskCommonVars()
	query := fmt.Sprintf("UPDATE %s SET ", tableName)
	clauses := []string{}
	args := []interface{}{}
	if common.TsStarted.Valid {
		clauses = append(clauses, "ts_started = ?")
		args = append(args, common.TsStarted.String)
	}
	if common.TsCompleted.Valid {
		clauses = append(clauses, "ts_completed = ?")
		args = append(args, common.TsCompleted.String)
	}
	if common.Failure.Valid {
		clauses = append(clauses, "failure = ?")
		args = append(args, common.Failure.Bool)
	}
	additionalClauses, additionalArgs, err := vars.GetUpdateExtraClausesAndBinds()
	if err != nil {
		return "", nil, err
	}
	clauses = append(clauses, additionalClauses...)
	args = append(args, additionalArgs...)
	if len(clauses) == 0 {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	query += strings.Join(clauses, ", ")
	query += " WHERE id = ?"
	args = append(args, common.Id)
	return query, args, nil
}

func updateTaskHandler(vars UpdateTaskVars, tableName string, w http.ResponseWriter, r *http.Request) {
	// TODO(benjaminwagner): authenticate
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewDecoder(r.Body).Decode(&vars); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to parse %T update", vars))
		return
	}
	defer skutil.Close(r.Body)

	query, binds, err := getUpdateQueryAndBinds(vars, tableName)
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to marshal %T update", vars))
		return
	}
	result, err := db.DB.Exec(query, binds...)
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to update using %T", vars))
		return
	}
	if rowsUpdated, _ := result.RowsAffected(); rowsUpdated != 1 {
		skutil.ReportError(w, r, fmt.Errorf("No rows updated. Likely invalid parameters."), "")
		return
	}
}

// Returns true if the given task can be deleted by the logged-in user; otherwise false and an error
// describing the problem.
func canDeleteTask(task Task, r *http.Request) (bool, error) {
	if !userHasAdminRights(r) {
		username := login.LoggedInAs(r)
		taskUser := task.GetCommonCols().Username
		if taskUser != username {
			return false, fmt.Errorf("Task is owned by %s but you are logged in as %s", taskUser, username)
		}
	}
	if task.GetCommonCols().TsStarted.Valid && !task.GetCommonCols().TsCompleted.Valid {
		return false, fmt.Errorf("Cannot delete currently running tasks.")
	}
	return true, nil
}

func deleteTaskHandler(prototype Task, w http.ResponseWriter, r *http.Request) {
	if !userHasEditRights(r) {
		skutil.ReportError(w, r, fmt.Errorf("Must have google or chromium account to delete tasks"), "")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	vars := struct{ Id int64 }{}
	if err := json.NewDecoder(r.Body).Decode(&vars); err != nil {
		skutil.ReportError(w, r, err, "Failed to parse delete request")
		return
	}
	defer skutil.Close(r.Body)
	requireUsernameMatch := !userHasAdminRights(r)
	username := login.LoggedInAs(r)
	// Put all conditions in delete request; only if the delete fails, do a select to determine the cause.
	deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE id = ? AND (ts_started IS NULL OR ts_completed IS NOT NULL)", prototype.TableName())
	binds := []interface{}{vars.Id}
	if requireUsernameMatch {
		deleteQuery += " AND username = ?"
		binds = append(binds, username)
	}
	result, err := db.DB.Exec(deleteQuery, binds...)
	if err != nil {
		skutil.ReportError(w, r, err, "Failed to delete")
		return
	}
	// Check result to ensure that the row was deleted.
	if rowsDeleted, _ := result.RowsAffected(); rowsDeleted == 1 {
		glog.Infof("%s task with ID %d deleted by %s", prototype.GetTaskName(), vars.Id, username)
		return
	}
	// The code below determines the reason that no rows were deleted.
	rowQuery := fmt.Sprintf("SELECT * FROM %s WHERE id = ?", prototype.TableName())
	data, err := prototype.Select(rowQuery, vars.Id)
	if err != nil {
		skutil.ReportError(w, r, err, "Unable to validate request.")
		return
	}
	tasks := asTaskSlice(data)
	if len(tasks) != 1 {
		// Row already deleted; return success.
		return
	}
	if ok, err := canDeleteTask(tasks[0], r); !ok {
		skutil.ReportError(w, r, err, "")
	} else {
		skutil.ReportError(w, r, fmt.Errorf("Failed to delete; reason unknown"), "")
		return
	}
}

func pageSetsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	pageSets := []map[string]string{}
	for pageSet := range util.PagesetTypeToInfo {
		pageSetObj := map[string]string{
			"key":         pageSet,
			"description": util.PagesetTypeToInfo[pageSet].Description,
		}
		pageSets = append(pageSets, pageSetObj)
	}

	if err := json.NewEncoder(w).Encode(pageSets); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

func runsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(runsHistoryTemplate, w, r)
}

func getAllPendingTasks() ([]Task, error) {
	tasks := []Task{}
	for _, tableName := range taskTables {
		var task Task
		query := fmt.Sprintf("SELECT * FROM %s WHERE ts_completed IS NULL ORDER BY ts_added LIMIT 1;", tableName)
		switch tableName {
		case db.TABLE_CHROMIUM_PERF_TASKS:
			task = &ChromiumPerfDBTask{}
		case db.TABLE_CAPTURE_SKPS_TASKS:
			task = &CaptureSkpsDBTask{}
		case db.TABLE_LUA_SCRIPT_TASKS:
			task = &LuaScriptDBTask{}
		case db.TABLE_CHROMIUM_BUILD_TASKS:
			task = &ChromiumBuildDBTask{}
		case db.TABLE_RECREATE_PAGE_SETS_TASKS:
			task = &RecreatePageSetsDBTask{}
		case db.TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS:
			task = &RecreateWebpageArchivesDBTask{}
		default:
			panic("Unknown table " + tableName)
		}

		if err := db.DB.Get(task, query); err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("Failed to query DB: %v", err)
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func getOldestPendingTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tasks, err := getAllPendingTasks()
	if err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to get all pending tasks: %v", err))
		return
	}

	var oldestTask Task
	for _, task := range tasks {
		if oldestTask == nil {
			oldestTask = task
		} else if oldestTask.GetCommonCols().TsAdded.Int64 <
			task.GetCommonCols().TsAdded.Int64 {
			oldestTask = task
		}
	}

	oldestTaskJsonRepr := map[string]Task{}
	if oldestTask != nil {
		oldestTaskJsonRepr[oldestTask.GetTaskName()] = oldestTask
	}
	if err := json.NewEncoder(w).Encode(oldestTaskJsonRepr); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

func pendingTasksView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(pendingTasksTemplate, w, r)
}
