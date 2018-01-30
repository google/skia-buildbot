/*
	Handlers, types, and functions common to all types of tasks.
*/

package task_common

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/sklog"

	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	"go.skia.org/infra/ct/go/db"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	skutil "go.skia.org/infra/go/util"
	"go.skia.org/infra/go/webhook"
)

const (
	// Default page size used for pagination.
	DEFAULT_PAGE_SIZE = 10

	// Maximum page size used for pagination.
	MAX_PAGE_SIZE = 100
)

var (
	httpClient = httputils.NewTimeoutClient()
)

type CommonCols struct {
	Id              int64          `db:"id"`
	TsAdded         sql.NullInt64  `db:"ts_added"`
	TsStarted       sql.NullInt64  `db:"ts_started"`
	TsCompleted     sql.NullInt64  `db:"ts_completed"`
	Username        string         `db:"username"`
	Failure         sql.NullBool   `db:"failure"`
	RepeatAfterDays int64          `db:"repeat_after_days"`
	SwarmingLogs    sql.NullString `db:"swarming_logs"`
}

type Task interface {
	GetCommonCols() *CommonCols
	GetTaskName() string
	TableName() string
	// Returns a slice of the struct type.
	Select(query string, args ...interface{}) (interface{}, error)
	// Returns the corresponding UpdateTaskVars instance of this Task. The
	// returned instance is not populated.
	GetUpdateTaskVars() UpdateTaskVars
	// Returns the corresponding AddTaskVars instance of this Task. The returned
	// instance is populated.
	GetPopulatedAddTaskVars() AddTaskVars
	// Returns the results link for this task if it completed successfully and if
	// the task supports results links.
	GetResultsLink() string
}

func (dbrow *CommonCols) GetCommonCols() *CommonCols {
	return dbrow
}

// Takes the result of Task.Select and returns a slice of Tasks containing the same objects.
func AsTaskSlice(selectResult interface{}) []Task {
	sliceValue := reflect.ValueOf(selectResult)
	sliceLen := sliceValue.Len()
	result := make([]Task, sliceLen)
	for i := 0; i < sliceLen; i++ {
		result[i] = sliceValue.Index(i).Addr().Interface().(Task)
	}
	return result
}

// Data included in all tasks; set by AddTaskHandler.
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

func AddTaskHandler(w http.ResponseWriter, r *http.Request, task AddTaskVars) {
	if !ctfeutil.UserHasEditRights(r) {
		httputils.ReportError(w, r, nil, "Please login with google or chromium account to add tasks")
		return
	}
	if task.IsAdminTask() && !ctfeutil.UserHasAdminRights(r) {
		httputils.ReportError(w, r, nil, "Must be admin to add admin tasks; contact rmistry@")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to add %T task", task))
		return
	}
	defer skutil.Close(r.Body)

	task.GetAddTaskCommonVars().Username = login.LoggedInAs(r)
	task.GetAddTaskCommonVars().TsAdded = ctutil.GetCurrentTs()
	if len(task.GetAddTaskCommonVars().Username) > 255 {
		httputils.ReportError(w, r, nil, "Username is too long, limit 255 bytes")
		return
	}

	if _, err := AddTask(task); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to insert %T task: %s", task, err))
		return
	}
}

// Returns the ID of the inserted task if the operation was successful.
func AddTask(task AddTaskVars) (int64, error) {
	query, binds, err := task.GetInsertQueryAndBinds()
	if err != nil {
		return -1, fmt.Errorf("Failed to marshal %T task: %v", task, err)
	}
	result, err := db.DB.Exec(query, binds...)
	if err != nil {
		return -1, fmt.Errorf("Failed to insert %T task: %v", task, err)
	}
	return result.LastInsertId()
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

type QueryParams struct {
	// If non-empty, limits to only tasks with the given username.
	Username string
	// Include only tasks that have completed successfully.
	SuccessfulOnly bool
	// Include only tasks that are not yet completed.
	PendingOnly bool
	// Include only completed tasks that are scheduled to repeat.
	FutureRunsOnly bool
	// Exclude tasks where page_sets is PAGESET_TYPE_DUMMY_1k.
	ExcludeDummyPageSets bool
	// If true, SELECT COUNT(*). If false, SELECT * and include ORDER BY and LIMIT clauses.
	CountQuery bool
	// First term of LIMIT clause; ignored if countQuery is true.
	Offset int
	// Second term of LIMIT clause; ignored if countQuery is true.
	Size int
}

func DBTaskQuery(prototype Task, params QueryParams) (string, []interface{}) {
	args := []interface{}{}
	query := "SELECT "
	if params.CountQuery {
		query += "COUNT(*)"
	} else {
		query += "*"
	}
	query += fmt.Sprintf(" FROM %s", prototype.TableName())
	clauses := []string{}
	if params.Username != "" {
		clauses = append(clauses, "username=?")
		args = append(args, params.Username)
	}
	if params.SuccessfulOnly {
		clauses = append(clauses, "(ts_completed IS NOT NULL AND failure = 0)")
	}
	if params.PendingOnly {
		clauses = append(clauses, "ts_completed IS NULL")
	}
	if params.FutureRunsOnly {
		clauses = append(clauses, "(repeat_after_days != 0 AND ts_completed IS NOT NULL)")
	}
	if params.ExcludeDummyPageSets {
		clauses = append(clauses, fmt.Sprintf("page_sets != '%s'", ctutil.PAGESET_TYPE_DUMMY_1k))
	}
	if len(clauses) > 0 {
		query += " WHERE "
		query += strings.Join(clauses, " AND ")
	}
	if !params.CountQuery {
		query += " ORDER BY id DESC LIMIT ?,?"
		args = append(args, params.Offset, params.Size)
	}
	return query, args
}

func HasPageSetsColumn(prototype Task) bool {
	v := reflect.Indirect(reflect.ValueOf(prototype))
	if v.Kind() != reflect.Struct {
		return false
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if strings.Contains(string(f.Tag), `db:"page_sets"`) {
			return true
		}
	}
	return false
}

func GetTasksHandler(prototype Task, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	params := QueryParams{}
	if parseBoolFormValue(r.FormValue("filter_by_logged_in_user")) {
		params.Username = login.LoggedInAs(r)
	}
	params.SuccessfulOnly = parseBoolFormValue(r.FormValue("successful"))
	params.PendingOnly = parseBoolFormValue(r.FormValue("not_completed"))
	params.FutureRunsOnly = parseBoolFormValue(r.FormValue("include_future_runs"))
	params.ExcludeDummyPageSets = parseBoolFormValue(r.FormValue("exclude_dummy_page_sets"))
	if params.SuccessfulOnly && params.PendingOnly {
		httputils.ReportError(w, r, fmt.Errorf("Inconsistent params: successful %v not_completed %v", r.FormValue("successful"), r.FormValue("not_completed")), "Inconsistent params")
		return
	}
	if params.ExcludeDummyPageSets && !HasPageSetsColumn(prototype) {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Task %s does not use page sets and thus cannot exclude dummy page sets.", prototype.GetTaskName()))
		return
	}
	offset, size, err := httputils.PaginationParams(r.URL.Query(), 0, DEFAULT_PAGE_SIZE, MAX_PAGE_SIZE)
	if err == nil {
		params.Offset, params.Size = offset, size
	} else {
		httputils.ReportError(w, r, err, "Failed to get pagination params")
		return
	}
	params.CountQuery = false
	query, args := DBTaskQuery(prototype, params)
	sklog.Infof("Running %s", query)
	data, err := prototype.Select(query, args...)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to query %s tasks", prototype.GetTaskName()))
		return
	}

	params.CountQuery = true
	query, args = DBTaskQuery(prototype, params)
	// Get the total count.
	sklog.Infof("Running %s", query)
	countVal := []int{}
	if err := db.DB.Select(&countVal, query, args...); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to query %s tasks", prototype.GetTaskName()))
		return
	}

	pagination := &httputils.ResponsePagination{
		Offset: offset,
		Size:   size,
		Total:  countVal[0],
	}
	type Permissions struct {
		DeleteAllowed bool
		RedoAllowed   bool
	}
	tasks := AsTaskSlice(data)
	permissions := make([]Permissions, len(tasks))
	for i := 0; i < len(tasks); i++ {
		deleteAllowed, _ := canDeleteTask(tasks[i], r)
		redoAllowed, _ := canRedoTask(tasks[i], r)
		permissions[i] = Permissions{DeleteAllowed: deleteAllowed, RedoAllowed: redoAllowed}
	}
	jsonResponse := map[string]interface{}{
		"data":        data,
		"permissions": permissions,
		"pagination":  pagination,
	}
	if err := json.NewEncoder(w).Encode(jsonResponse); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode JSON")
		return
	}
}

// Data included in all update requests.
type UpdateTaskCommonVars struct {
	Id              int64
	TsStarted       sql.NullString
	TsCompleted     sql.NullString
	Failure         sql.NullBool
	RepeatAfterDays sql.NullInt64
	SwarmingLogs    sql.NullString
}

func (vars *UpdateTaskCommonVars) SetStarted(runID string) {
	vars.TsStarted = sql.NullString{String: ctutil.GetCurrentTs(), Valid: true}
	swarmingLogsLink := fmt.Sprintf(ctutil.SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, runID)
	vars.SwarmingLogs = sql.NullString{String: swarmingLogsLink, Valid: true}
}

func (vars *UpdateTaskCommonVars) SetCompleted(success bool) {
	vars.TsCompleted = sql.NullString{String: ctutil.GetCurrentTs(), Valid: true}
	vars.Failure = sql.NullBool{Bool: !success, Valid: true}
}

func (vars *UpdateTaskCommonVars) ClearRepeatAfterDays() {
	vars.RepeatAfterDays = sql.NullInt64{Int64: 0, Valid: true}
}

func (vars *UpdateTaskCommonVars) GetUpdateTaskCommonVars() *UpdateTaskCommonVars {
	return vars
}

type UpdateTaskVars interface {
	GetUpdateTaskCommonVars() *UpdateTaskCommonVars
	UriPath() string
	// Produces SQL query clauses and binds for fields not in UpdateTaskCommonVars. First return
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
	if common.RepeatAfterDays.Valid {
		clauses = append(clauses, "repeat_after_days = ?")
		args = append(args, common.RepeatAfterDays)
	}
	if common.SwarmingLogs.Valid {
		clauses = append(clauses, "swarming_logs = ?")
		args = append(args, common.SwarmingLogs.String)
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

func UpdateTaskHandler(vars UpdateTaskVars, tableName string, w http.ResponseWriter, r *http.Request) {
	data, err := webhook.AuthenticateRequest(r)
	if err != nil {
		if data == nil {
			httputils.ReportError(w, r, err, "Failed to read update request")
			return
		}
		if !ctfeutil.UserHasAdminRights(r) {
			httputils.ReportError(w, r, err, "Failed authentication")
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.Unmarshal(data, &vars); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to parse %T update", vars))
		return
	}
	defer skutil.Close(r.Body)

	if err := UpdateTask(vars, tableName); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to update %T task", vars))
		return
	}
}

func UpdateTask(vars UpdateTaskVars, tableName string) error {
	query, binds, err := getUpdateQueryAndBinds(vars, tableName)
	if err != nil {
		return fmt.Errorf("Failed to marshal %T update: %v", vars, err)
	}
	result, err := db.DB.Exec(query, binds...)
	if err != nil {
		return fmt.Errorf("Failed to update using %T: %v", vars, err)
	}
	if rowsUpdated, _ := result.RowsAffected(); rowsUpdated != 1 {
		return fmt.Errorf("No rows updated. Likely invalid parameters.")
	}
	return nil
}

// Returns true if the given task can be deleted by the logged-in user; otherwise false and an error
// describing the problem.
func canDeleteTask(task Task, r *http.Request) (bool, error) {
	if !ctfeutil.UserHasAdminRights(r) {
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

// Returns true if the given task can be re-added by the logged-in user; otherwise false and an
// error describing the problem.
func canRedoTask(task Task, r *http.Request) (bool, error) {
	if !task.GetCommonCols().TsCompleted.Valid {
		return false, fmt.Errorf("Cannot redo pending tasks.")
	}
	return true, nil
}

func DeleteTaskHandler(prototype Task, w http.ResponseWriter, r *http.Request) {
	if !ctfeutil.UserHasEditRights(r) {
		httputils.ReportError(w, r, nil, "Please login with google or chromium account to delete tasks")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	vars := struct{ Id int64 }{}
	if err := json.NewDecoder(r.Body).Decode(&vars); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse delete request")
		return
	}
	defer skutil.Close(r.Body)
	requireUsernameMatch := !ctfeutil.UserHasAdminRights(r)
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
		httputils.ReportError(w, r, err, "Failed to delete")
		return
	}
	// Check result to ensure that the row was deleted.
	if rowsDeleted, _ := result.RowsAffected(); rowsDeleted == 1 {
		sklog.Infof("%s task with ID %d deleted by %s", prototype.GetTaskName(), vars.Id, username)
		return
	}
	// The code below determines the reason that no rows were deleted.
	rowQuery := fmt.Sprintf("SELECT * FROM %s WHERE id = ?", prototype.TableName())
	data, err := prototype.Select(rowQuery, vars.Id)
	if err != nil {
		httputils.ReportError(w, r, err, "Unable to validate request.")
		return
	}
	tasks := AsTaskSlice(data)
	if len(tasks) != 1 {
		// Row already deleted; return success.
		return
	}
	if ok, err := canDeleteTask(tasks[0], r); !ok {
		httputils.ReportError(w, r, err, "Do not have permission to delete task")
	} else {
		httputils.ReportError(w, r, nil, "Failed to delete; reason unknown")
		return
	}
}

func RedoTaskHandler(prototype Task, w http.ResponseWriter, r *http.Request) {
	if !ctfeutil.UserHasEditRights(r) {
		httputils.ReportError(w, r, nil, "Please login with google or chromium account to redo tasks")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	vars := struct{ Id int64 }{}
	if err := json.NewDecoder(r.Body).Decode(&vars); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse redo request")
		return
	}
	defer skutil.Close(r.Body)

	rowQuery := fmt.Sprintf("SELECT * FROM %s WHERE id = ? AND ts_completed IS NOT NULL", prototype.TableName())
	binds := []interface{}{vars.Id}
	data, err := prototype.Select(rowQuery, binds...)
	if err != nil {
		httputils.ReportError(w, r, err, "Unable to find requested task.")
		return
	}
	tasks := AsTaskSlice(data)
	if len(tasks) != 1 {
		httputils.ReportError(w, r, err, "Unable to find requested task.")
		return
	}

	addTaskVars := tasks[0].GetPopulatedAddTaskVars()
	// Replace the username with the new requester.
	addTaskVars.GetAddTaskCommonVars().Username = login.LoggedInAs(r)
	// Do not preserve repeat_after_days for retried tasks. Carrying over
	// repeat_after_days causes the same task to be unknowingly repeated.
	addTaskVars.GetAddTaskCommonVars().RepeatAfterDays = "0"
	if _, err := AddTask(addTaskVars); err != nil {
		httputils.ReportError(w, r, err, "Could not redo the task.")
		return
	}
}

type PageSet struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

// ByPageSetDesc implements sort.Interface to order PageSets by their descriptions.
type ByPageSetDesc []PageSet

func (p ByPageSetDesc) Len() int           { return len(p) }
func (p ByPageSetDesc) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p ByPageSetDesc) Less(i, j int) bool { return p[i].Description < p[j].Description }

func pageSetsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	pageSets := []PageSet{}
	for pageSet := range ctutil.PagesetTypeToInfo {
		p := PageSet{
			Key:         pageSet,
			Description: ctutil.PagesetTypeToInfo[pageSet].Description,
		}
		pageSets = append(pageSets, p)
	}
	sort.Sort(ByPageSetDesc(pageSets))
	if err := json.NewEncoder(w).Encode(pageSets); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

var gerritURLRegexp = regexp.MustCompile("^(https?://(?:[a-z]+)-review\\.googlesource\\.com)/c/(?:.+/)?(\\d{3,})/?$")

type clDetail struct {
	Issue         int64  `json:"issue"`
	Subject       string `json:"subject"`
	Modified      string `json:"modified"`
	Project       string `json:"project"`
	Patchsets     []int  `json:"patchsets"`
	CodereviewURL string
}

func gatherCLData(detail clDetail, patch string) (map[string]string, error) {
	clData := map[string]string{}
	clData["cl"] = strconv.FormatInt(detail.Issue, 10)
	clData["subject"] = detail.Subject
	clData["url"] = detail.CodereviewURL
	modifiedTime, err := time.Parse("2006-01-02 15:04:05.999999", detail.Modified)
	if err != nil {
		sklog.Errorf("Unable to parse modified time for CL %d; input '%s', got %v", detail.Issue, detail.Modified, err)
		clData["modified"] = ""
	} else {
		clData["modified"] = modifiedTime.UTC().Format(ctutil.TS_FORMAT)
	}
	clData["chromium_patch"] = ""
	clData["skia_patch"] = ""
	clData["v8_patch"] = ""
	clData["catapult_patch"] = ""
	switch detail.Project {
	case "chromium", "chromium/src":
		clData["chromium_patch"] = patch
	case "skia":
		clData["skia_patch"] = patch
	case "v8/v8":
		clData["v8_patch"] = patch
	case "catapult":
		clData["catapult_patch"] = patch
	default:
		sklog.Errorf("CL project is %s; only chromium, skia, v8, catapult are supported.", detail.Project)
	}
	return clData, nil
}

func getCLHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	clURLString := r.FormValue("cl")

	var detail clDetail
	var patch string
	var err error
	// See if it is a Gerrit URL.
	matches := gerritURLRegexp.FindStringSubmatch(clURLString)
	if len(matches) < 3 || matches[1] == "" || matches[2] == "" {
		// Return successful empty response, since the user could still be typing.
		if err := json.NewEncoder(w).Encode(map[string]interface{}{}); err != nil {
			httputils.ReportError(w, r, err, "Failed to encode JSON")
		}
		return
	}
	crURL := matches[1]
	clString := matches[2]
	g, err := gerrit.NewGerrit(crURL, "", httpClient)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to talk to Gerrit")
		return
	}
	cl, err := strconv.ParseInt(clString, 10, 32)
	if err != nil {
		httputils.ReportError(w, r, err, "Invalid Gerrit CL number")
		return
	}
	change, err := g.GetIssueProperties(cl)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to get issue properties from Gerrit")
		return
	}

	// Check to see if the change has any open dependencies.
	activeDep, err := g.HasOpenDependency(cl, len(change.Patchsets))
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to get related changes from Gerrit")
		return
	}
	if activeDep {
		httputils.ReportError(w, r, err, fmt.Sprintf("This CL has an open dependency. Please squash your changes into a single CL."))
		return
	}

	latestPatchsetID := strconv.Itoa(len(change.Patchsets))
	detail = clDetail{
		Issue:         cl,
		Subject:       change.Subject,
		Modified:      change.UpdatedString,
		Project:       change.Project,
		CodereviewURL: fmt.Sprintf("%s/c/%d/%s", crURL, cl, latestPatchsetID),
	}
	patch, err = g.GetPatch(cl, latestPatchsetID)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to download patch from Gerrit")
		return
	}

	clData, err := gatherCLData(detail, patch)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to get CL data")
		return
	}
	if err = json.NewEncoder(w).Encode(clData); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode JSON")
		return
	}
}

func benchmarksPlatformsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	data := map[string]interface{}{
		"benchmarks": ctutil.SupportedBenchmarks,
		"platforms":  ctutil.SupportedPlatformsToDesc,
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

func AddHandlers(r *mux.Router) {
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.PAGE_SETS_PARAMETERS_POST_URI, "POST", pageSetsHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.CL_DATA_POST_URI, "POST", getCLHandler)
	ctfeutil.AddForceLoginHandler(r, "/"+ctfeutil.BENCHMARKS_PLATFORMS_POST_URI, "POST", benchmarksPlatformsHandler)
}
