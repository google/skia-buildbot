/*
	Handlers, types, and functions common to all types of tasks.
*/

package task_common

import (
	"context"
	"sync"
	//"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"google.golang.org/api/iterator"

	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
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
	httpClient      = httputils.NewTimeoutClient()
	kindToHighestID = map[ds.Kind]int64{}
	idMutex         sync.Mutex
)

type CommonCols struct {
	Id              int64  `db:"id"`
	DatastoreId     int64  `datastore:"-"`
	TsAdded         int64  `db:"ts_added"`
	TsStarted       int64  `db:"ts_started"`
	TsCompleted     int64  `db:"ts_completed"`
	Username        string `db:"username"`
	Failure         bool   `db:"failure"`
	RepeatAfterDays int64  `db:"repeat_after_days"`
	SwarmingLogs    string `db:"swarming_logs"`
	TaskDone        bool

	//Id              int64          `db:"id" json:"id"`
	//TsAdded         sql.NullInt64  `db:"ts_added" json:"ts_added"`
	//TsStarted       sql.NullInt64  `db:"ts_started" json:"ts_started"`
	//TsCompleted     sql.NullInt64  `db:"ts_completed" json:"ts_completed"`
	//Username        string         `db:"username" json:"username"`
	//Failure         sql.NullBool   `db:"failure" json:"failure"`
	//RepeatAfterDays int64          `db:"repeat_after_days" json:"repeat_after_days"`
	//SwarmingLogs    sql.NullString `db:"swarming_logs" json:"swarming_logs"`
}

type Task interface {
	GetCommonCols() *CommonCols
	GetTaskName() string
	GetDatastoreKind() ds.Kind
	// Returns a slice of the struct type.
	Select(it *datastore.Iterator) (interface{}, error)
	// Returns the struct type.
	Find(c context.Context, key *datastore.Key) (interface{}, error)
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

//Id              int64  `db:"id"`
//DatastoreId     int64  `datastore:"-"`
//TsAdded         int64  `db:"ts_added"`
//TsStarted       int64  `db:"ts_started"`
//TsCompleted     int64  `db:"ts_completed"`
//Username        string `db:"username"`
//Failure         bool   `db:"failure"`
//RepeatAfterDays int64  `db:"repeat_after_days"`
//SwarmingLogs    string `db:"swarming_logs"`
//TaskDone        bool

// Takes the result of Task.Select and returns a slice of Tasks containing the same objects.
func AsTaskSlice(selectResult interface{}) []Task {
	if selectResult == nil {
		return []Task{}
	}
	sliceValue := reflect.ValueOf(selectResult)
	sliceLen := sliceValue.Len()
	result := make([]Task, sliceLen)
	for i := 0; i < sliceLen; i++ {
		result[i] = sliceValue.Index(i).Interface().(Task)
	}
	return result
}

// Takes the result of Task.Find and returns a Task.
func AsTask(findResult interface{}) Task {
	if findResult == nil {
		return nil
	}
	val := reflect.ValueOf(findResult)
	return val.Interface().(Task)
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
	GetPopulatedDatastoreTask() (Task, error)
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
// TODO(rmistry): Make this take in a context and a ds KIND.
func AddTask(task AddTaskVars) (int64, error) {
	key := ds.NewKey(ds.CAPTURE_SKPS_TASKS)

	datastoreTask, err := task.GetPopulatedDatastoreTask()
	ret, err := ds.DS.Put(context.Background(), key, datastoreTask)
	if err != nil {
		return -1, fmt.Errorf("Error putting task in datastore: %s", err)
	}
	return ret.ID, nil
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

// take in context here?
func DatastoreTaskQuery(prototype Task, params QueryParams) *datastore.Iterator {
	q := ds.NewQuery(prototype.GetDatastoreKind()).EventualConsistency()
	// what do I do about count?
	if params.CountQuery {
		q = q.KeysOnly()
	}
	if params.Username != "" {
		sklog.Infof("Adding filter for Username = %s", params.Username)
		q = q.Filter("Username =", params.Username)
	}
	if params.SuccessfulOnly { // Only for SKP reositories and Build repositories I believe.
		sklog.Info("Adding filter for TaskDone=true AND Failure=false")
		q = q.Filter("TaskDone =", true)
		q = q.Filter("Failure =", false)
		//clauses = append(clauses, "(ts_completed IS NOT NULL AND failure = 0)")
	}
	if params.PendingOnly {
		sklog.Info("Adding filter for TaskDone = false")
		q = q.Filter("TaskDone =", false)
		//clauses = append(clauses, "ts_completed IS NULL")
	}
	if params.FutureRunsOnly {
		sklog.Info("Adding filter for RepeatAfterDays > 0 AND Done = true")
		q = q.Filter("RepeatAfterDays >", 0)
		q = q.Order("RepeatAfterDays")
		q = q.Filter("TaskDone =", true)
		//clauses = append(clauses, "(repeat_after_days != 0 AND ts_completed IS NOT NULL)")
	}
	if params.ExcludeDummyPageSets {
		q = q.Filter("IsTestPageSet =", false)
		//clauses = append(clauses, fmt.Sprintf("page_sets != '%s'", ctutil.PAGESET_TYPE_DUMMY_1k))
	}
	//if len(clauses) > 0 {
	//	query += " WHERE "
	//	query += strings.Join(clauses, " AND ")
	//}
	if !params.CountQuery {
		sklog.Infof("Adding order by id and limit %d and offset %d", params.Size, params.Offset)
		q = q.Order("-Id")
		q = q.Limit(params.Size)
		q = q.Offset(params.Offset)
	}
	//if !params.CountQuery {
	//	query += " ORDER BY id DESC LIMIT ?,?"
	//	args = append(args, params.Offset, params.Size)
	//}

	return ds.DS.Run(context.TODO(), q)
	//return ds.DS.Run(context.TODO(), q)
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

// rmistry: THIS IS WRONG AND NEEDS TO BE FIXED! get the higest ID from the datastore.
func GetNextId(kind ds.Kind) (int64, error) {
	idMutex.Lock()
	defer idMutex.Unlock()
	if val, ok := kindToHighestID[kind]; ok {
		// If it exists in memory then return the current highest ID.
		return val + 1, nil
	} else {
		// Does not exist in memory, hit the datastore.
		q := ds.NewQuery(kind).EventualConsistency()
		//q = q.KeysOnly()
		val, err := ds.DS.Count(context.Background(), q)
		if err != nil {
			return -1, err
		}
		nextId := int64(val + 1)
		kindToHighestID[kind] = nextId
		return nextId, nil
	}
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
	params.CountQuery = false // DOES THIS REALLY EVER NEED TO BE TRUE?
	// WHY DOES THIS DO 2 QUERIES? that makes no sense...

	// rmistry
	// q := datastore.NewQuery("Person").Filter("Height <=", maxHeight)

	it := DatastoreTaskQuery(prototype, params)
	//query, args := DBTaskQuery(prototype, params)
	//sklog.Infof("Running %s", query)
	data, err := prototype.Select(it)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to query %s tasks", prototype.GetTaskName()))
		return
	}

	//fmt.Println("NAME NAME NAME NAME")
	//fmt.Println(prototype.GetTaskName())
	params.CountQuery = true
	it = DatastoreTaskQuery(prototype, params)
	//query, args = DBTaskQuery(prototype, params)
	// Get the total count.
	//sklog.Infof("Running %s", query)
	//countVal := []int{}
	count := 0
	for {
		var i int
		_, err := it.Next(i)
		if err == iterator.Done {
			break
		} else if err != nil {
			httputils.ReportError(w, r, err, fmt.Sprintf("Failed to query %s tasks", prototype.GetTaskName()))
			return
		}
		//t.Id = k.ID // NEEDED????????????????????
		//t.DatastoreId = k.ID
		//countVal = append(countVal, k)
		count++
		//tasks = append(tasks, t)
	}
	//if err := db.DB.Select(&countVal, query, args...); err != nil {
	//	httputils.ReportError(w, r, err, fmt.Sprintf("Failed to query %s tasks", prototype.GetTaskName()))
	//	return
	//}

	pagination := &httputils.ResponsePagination{
		Offset: offset,
		Size:   size,
		Total:  count,
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
	TsStarted       string
	TsCompleted     string
	Failure         bool
	TaskDone        bool
	RepeatAfterDays int64
	SwarmingLogs    string
}

func (vars *UpdateTaskCommonVars) SetStarted(runID string) {
	vars.TsStarted = ctutil.GetCurrentTs()
	swarmingLogsLink := fmt.Sprintf(ctutil.SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, runID)
	vars.SwarmingLogs = swarmingLogsLink
}

func (vars *UpdateTaskCommonVars) SetCompleted(success bool) {
	vars.TsCompleted = ctutil.GetCurrentTs()
	vars.Failure = !success
	vars.TaskDone = true
}

func (vars *UpdateTaskCommonVars) ClearRepeatAfterDays() {
	vars.RepeatAfterDays = 0
}

func (vars *UpdateTaskCommonVars) GetUpdateTaskCommonVars() *UpdateTaskCommonVars {
	return vars
}

type UpdateTaskVars interface {
	GetUpdateTaskCommonVars() *UpdateTaskCommonVars
	UriPath() string
	// Adds CT task specific updates for fields not in UpdateTaskCommonVars.
	AddUpdatesToDBTask(Task) error
}

func updateDBTask(vars UpdateTaskVars, task Task) error {
	//  ds.DS.Put(context.Background(), k, t)

	common := vars.GetUpdateTaskCommonVars()

	if common.TsStarted != "" {
		tsStarted, err := strconv.ParseInt(common.TsStarted, 10, 64)
		if err != nil {
			return fmt.Errorf("Invalid TsStarted %s: %s", common.TsStarted, err)
		}
		task.GetCommonCols().TsStarted = tsStarted
	}
	if common.TsCompleted != "" {
		tsCompleted, err := strconv.ParseInt(common.TsCompleted, 10, 64)
		if err != nil {
			return fmt.Errorf("Invalid TsCompleted %s: %s", common.TsCompleted, err)
		}
		task.GetCommonCols().TsCompleted = tsCompleted
	}
	if common.Failure {
		task.GetCommonCols().Failure = common.Failure
	}
	if common.TaskDone {
		task.GetCommonCols().TaskDone = common.TaskDone
	}
	if common.RepeatAfterDays != 0 {
		task.GetCommonCols().RepeatAfterDays = common.RepeatAfterDays
	}
	if common.SwarmingLogs != "" {
		task.GetCommonCols().SwarmingLogs = common.SwarmingLogs
	}
	if err := vars.AddUpdatesToDBTask(task); err != nil {
		return err
	}
	return nil
}

// HERE HERE HERE
// rmistry: Replace tableName with DatastoreKind.... right???
// rmistry: Where do I get the datastore Id from?  do find task with id and return the whole task and then update whatever you need to update in it and then put it.
func UpdateTaskHandler(vars UpdateTaskVars, prototype Task, w http.ResponseWriter, r *http.Request) {
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

	q := ds.NewQuery(prototype.GetDatastoreKind()).EventualConsistency()
	q = q.Filter("Id =", vars.GetUpdateTaskCommonVars().Id)
	it := ds.DS.Run(context.Background(), q)
	s, err := prototype.Select(it)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to find %T task", vars))
		return
	}
	tasks := AsTaskSlice(s)

	fmt.Println("==========================================================")
	fmt.Println("==========================================================")
	fmt.Println("==========================================================")
	fmt.Println("==========================================================")
	fmt.Println(tasks)
	fmt.Println(len(tasks))
	fmt.Println(tasks[0])

	//key := ds.NewKey(kind)
	//key.ID = vars.Id
	//data, err := prototype.Find(context.Background(), key)
	//if err != nil {
	//	httputils.ReportError(w, r, err, "Failed to find requested task")
	//	return
	//}
	//task := AsTask(data)

	fmt.Println("BEFORE BEFORE BEFORE BEFORE BEFORE")
	fmt.Println(tasks[0])
	fmt.Println("UPDATES UPDATES")
	fmt.Println(vars)
	if err := UpdateTask(vars, tasks[0]); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to update %T task", vars))
		return
	}
}

func UpdateTask(vars UpdateTaskVars, task Task) error {
	// Remove DBTask stuff here to just task..
	if err := updateDBTask(vars, task); err != nil {
		return fmt.Errorf("Failed to marshal %T update: %v", vars, err)
	}

	fmt.Println("AFTER AFTER AFTER AFTER")
	fmt.Println(task)

	key := ds.NewKey(task.GetDatastoreKind())
	key.ID = task.GetCommonCols().DatastoreId
	fmt.Println("KEY KEY KEY KEY")
	fmt.Println(key.ID)
	task.GetCommonCols().Failure = true
	if _, err := ds.DS.Put(context.Background(), key, task); err != nil {
		return fmt.Errorf("Failed to update task %d in the datastore: %s", task.GetCommonCols().Id, err)
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
	if task.GetCommonCols().TsStarted != 0 && task.GetCommonCols().TsCompleted == 0 {
		return false, fmt.Errorf("Cannot delete currently running tasks.")
	}
	return true, nil
}

// Returns true if the given task can be re-added by the logged-in user; otherwise false and an
// error describing the problem.
func canRedoTask(task Task, r *http.Request) (bool, error) {
	if !task.GetCommonCols().TaskDone {
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

	key := ds.NewKey(prototype.GetDatastoreKind())
	key.ID = vars.Id
	if err := ds.DS.Delete(context.Background(), key); err != nil {
		httputils.ReportError(w, r, err, "Failed to delete")
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

	key := ds.NewKey(prototype.GetDatastoreKind())
	key.ID = vars.Id
	data, err := prototype.Find(context.Background(), key)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to find requested task")
		return
	}
	task := AsTask(data)

	//ds.DS.Run(context.TODO(), q)
	//ds.DS.Get(context.Background(), key, &protype)
	//prototype.Select()

	//q := ds.NewQuery(prototype.GetDatastoreKind()).EventualConsistency()
	////q.Filter("__key__ =", key)
	//q.Filter("__key__ =", fmt.Sprintf("Key(%s, %d)", prototype.GetDatastoreKind(), vars.Id))
	//it := ds.DS.Run(context.TODO(), q)
	//data, err := prototype.Select(it)
	//if err != nil {
	//	httputils.ReportError(w, r, err, "Unable to find requested task.")
	//	return
	//}
	//tasks := AsTaskSlice(data)
	//if len(tasks) != 1 {
	//	fmt.Println("ERROR ERROR ERROR ERROR!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	//	fmt.Println(data)
	//	fmt.Println(vars.Id)
	//	fmt.Println(tasks[0].GetCommonCols().Id)
	//	fmt.Println(tasks[0].GetCommonCols().DatastoreId)
	//	fmt.Println(tasks[1].GetCommonCols().Id)
	//	fmt.Println(tasks[1].GetCommonCols().DatastoreId)
	//	httputils.ReportError(w, r, nil, "Unable to find requested task.")
	//	return
	//}

	addTaskVars := task.GetPopulatedAddTaskVars()
	// Replace the username with the new requester.
	addTaskVars.GetAddTaskCommonVars().Username = login.LoggedInAs(r)
	// Do not preserve repeat_after_days for retried tasks. Carrying over
	// repeat_after_days causes the same task to be unknowingly repeated.
	addTaskVars.GetAddTaskCommonVars().RepeatAfterDays = "0"
	if _, err := AddTask(addTaskVars); err != nil {
		httputils.ReportError(w, r, err, "Could not redo the task.")
		return
	}

	// rmistry: FIX!!!!!!!
	return
	//rowQuery := fmt.Sprintf("SELECT * FROM %s WHERE id = ? AND ts_completed IS NOT NULL", prototype.TableName())
	//binds := []interface{}{vars.Id}
	//data, err := prototype.Select(rowQuery, binds...)
	//if err != nil {
	//	httputils.ReportError(w, r, err, "Unable to find requested task.")
	//	return
	//}
	//tasks := AsTaskSlice(data)
	//if len(tasks) != 1 {
	//	httputils.ReportError(w, r, err, "Unable to find requested task.")
	//	return
	//}

	//addTaskVars := tasks[0].GetPopulatedAddTaskVars()
	//// Replace the username with the new requester.
	//addTaskVars.GetAddTaskCommonVars().Username = login.LoggedInAs(r)
	//// Do not preserve repeat_after_days for retried tasks. Carrying over
	//// repeat_after_days causes the same task to be unknowingly repeated.
	//addTaskVars.GetAddTaskCommonVars().RepeatAfterDays = "0"
	//if _, err := AddTask(addTaskVars); err != nil {
	//	httputils.ReportError(w, r, err, "Could not redo the task.")
	//	return
	//}
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

var gerritURLRegexp = regexp.MustCompile("^(https?://(?:[a-z]+)-review\\.googlesource\\.com)/(?:#/)?c/(?:.+/)?(\\d{3,})/?$")

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

	// Check to see if the change has a binary file.
	latestPatchsetID := strconv.Itoa(len(change.Patchsets))
	isBinary, err := g.IsBinaryPatch(cl, latestPatchsetID)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to get list of files from Gerrit")
		return
	}
	if isBinary {
		httputils.ReportError(w, r, err, fmt.Sprintf("CT cannot get a full index for binary files via the Gerrit API. Details in skbug.com/7302."))
		return
	}

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
