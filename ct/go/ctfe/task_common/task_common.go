/*
	Handlers, types, and functions common to all types of tasks.
*/

package task_common

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"go.skia.org/infra/ct/go/ct_autoscaler"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
)

const (
	// Default page size used for pagination.
	DEFAULT_PAGE_SIZE = 10

	// Maximum page size used for pagination.
	MAX_PAGE_SIZE = 100
)

var (
	httpClient       = httputils.NewTimeoutClient()
	datastoreIdMutex sync.Mutex

	// CT autoscaler.
	autoscaler ct_autoscaler.ICTAutoscaler

	// The location of the service account JSON file.
	ServiceAccountFile string

	WebappURL string
)

type CommonCols struct {
	DatastoreKey    *datastore.Key `datastore:"__key__"`
	TsAdded         int64
	TsStarted       int64
	TsCompleted     int64
	Username        string
	Failure         bool
	RepeatAfterDays int64
	SwarmingLogs    string
	TaskDone        bool
	SwarmingTaskID  string
}

type Task interface {
	GetCommonCols() *CommonCols
	RunsOnGCEWorkers() bool
	TriggerSwarmingTaskAndMail(ctx context.Context) error
	SendCompletionEmail(ctx context.Context, completedSuccessfully bool) error
	GetTaskName() string
	GetDatastoreKind() ds.Kind
	// Returns a slice of the struct type.
	Query(it *datastore.Iterator) (interface{}, error)
	// Returns the struct type.
	Get(c context.Context, key *datastore.Key) (Task, error)
	// Returns the corresponding UpdateTaskVars instance of this Task. The
	// returned instance is not populated.
	GetUpdateTaskVars() UpdateTaskVars
	// Returns the corresponding AddTaskVars instance of this Task. The returned
	// instance is populated.
	GetPopulatedAddTaskVars() (AddTaskVars, error)
	// Returns the results link for this task if it completed successfully and if
	// the task supports results links.
	GetResultsLink() string
}

func (dbrow *CommonCols) GetCommonCols() *CommonCols {
	return dbrow
}

// Takes the result of Task.Query and returns a slice of Tasks containing the same objects.
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

// Generates a unique ID for this task.
func GetRunID(task Task) string {
	return fmt.Sprintf("%s-%s-%d", strings.SplitN(task.GetCommonCols().Username, "@", 2)[0], task.GetTaskName(), task.GetCommonCols().DatastoreKey.ID)
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
	GetDatastoreKind() ds.Kind
	GetPopulatedDatastoreTask(ctx context.Context) (Task, error)
}

func (vars *AddTaskCommonVars) GetAddTaskCommonVars() *AddTaskCommonVars {
	return vars
}

func (vars *AddTaskCommonVars) IsAdminTask() bool {
	return false
}

func AddTaskHandler(w http.ResponseWriter, r *http.Request, task AddTaskVars) {
	if !ctfeutil.UserHasEditRights(r) {
		httputils.ReportError(w, r, nil, "Please login with google account to add tasks")
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

	if err := AddAndTriggerTask(r.Context(), task); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to insert or trigger %T task", task))
		return
	}
}

func AddAndTriggerTask(ctx context.Context, task AddTaskVars) error {
	datastoreTask, err := AddTaskToDatastore(ctx, task)
	if err != nil {
		return fmt.Errorf("Failed to insert %T task: %s", task, err)
	}
	if err := TriggerTaskOnSwarming(ctx, task, datastoreTask); err != nil {
		if err := UpdateTaskSetFailed(ctx, datastoreTask); err != nil {
			sklog.Error(err)
		}
		return fmt.Errorf("Failed to trigger on swarming %T task: %s", task, err)
	}
	return nil
}

func AddTaskToDatastore(ctx context.Context, task AddTaskVars) (Task, error) {
	datastoreTask, err := task.GetPopulatedDatastoreTask(ctx)
	if err != nil {
		return nil, fmt.Errorf("Could not get populated datastore task: %s", err)
	}

	// Create the key.
	id, err := GetNextId(ctx, task.GetDatastoreKind(), datastoreTask)
	if err != nil {
		return nil, fmt.Errorf("Could not get highest id for %s: %s", task.GetDatastoreKind(), err)
	}
	key := ds.NewKey(task.GetDatastoreKind())
	key.ID = id
	datastoreTask.GetCommonCols().DatastoreKey = key

	// Add the common columns to the task.
	tsAdded, err := strconv.ParseInt(task.GetAddTaskCommonVars().TsAdded, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s is not int64: %s", task.GetAddTaskCommonVars().TsAdded, err)
	}
	datastoreTask.GetCommonCols().TsAdded = tsAdded
	datastoreTask.GetCommonCols().Username = task.GetAddTaskCommonVars().Username
	repeatAfterDays, err := strconv.ParseInt(task.GetAddTaskCommonVars().RepeatAfterDays, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s is not int64: %s", task.GetAddTaskCommonVars().RepeatAfterDays, err)
	}
	datastoreTask.GetCommonCols().RepeatAfterDays = repeatAfterDays

	if _, err := ds.DS.Put(ctx, key, datastoreTask); err != nil {
		return nil, fmt.Errorf("Error putting task in datastore: %s", err)
	}
	return datastoreTask, nil
}

func TriggerTaskOnSwarming(ctx context.Context, task AddTaskVars, datastoreTask Task) error {
	if datastoreTask.RunsOnGCEWorkers() {
		taskId := fmt.Sprintf("%s.%d", datastoreTask.GetTaskName(), datastoreTask.GetCommonCols().DatastoreKey.ID)
		autoscaler.RegisterGCETask(taskId)
	}
	return datastoreTask.TriggerSwarmingTaskAndMail(ctx)
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

func DatastoreTaskQuery(ctx context.Context, prototype Task, params QueryParams) *datastore.Iterator {
	q := ds.NewQuery(prototype.GetDatastoreKind())
	if params.CountQuery {
		q = q.KeysOnly()
	}
	if params.Username != "" {
		q = q.Filter("Username =", params.Username)
	}
	if params.SuccessfulOnly {
		q = q.Filter("TaskDone =", true)
		q = q.Filter("Failure =", false)
	}
	if params.PendingOnly {
		q = q.Filter("TaskDone =", false)
	}
	if params.FutureRunsOnly {
		q = q.Filter("RepeatAfterDays >", 0)
		q = q.Order("RepeatAfterDays")
		q = q.Filter("TaskDone =", true)
	}
	if params.ExcludeDummyPageSets {
		q = q.Filter("IsTestPageSet =", false)
	}
	if !params.CountQuery {
		q = q.Order("-__key__")
		q = q.Limit(params.Size)
		q = q.Offset(params.Offset)
	}

	return ds.DS.Run(ctx, q)
}

func HasPageSetsColumn(prototype Task) bool {
	v := reflect.Indirect(reflect.ValueOf(prototype))
	if v.Kind() != reflect.Struct {
		return false
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Name == "PageSets" {
			return true
		}
	}
	return false
}

type ClusterTelemetryIDs struct {
	HighestID int64
}

func GetNextId(ctx context.Context, kind ds.Kind, task Task) (int64, error) {
	datastoreIdMutex.Lock()
	defer datastoreIdMutex.Unlock()

	// Hit the datastore to get the current highest ID.
	key := ds.NewKey(ds.CLUSTER_TELEMETRY_IDS)
	key.Name = string(kind)
	var nextId int64 = -1
	_, err := ds.DS.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		ids := ClusterTelemetryIDs{}
		if err := ds.DS.Get(ctx, key, &ids); err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}
		nextId = ids.HighestID + 1
		ids.HighestID = nextId
		_, err := ds.DS.Put(ctx, key, &ids)
		return err
	})
	return nextId, err
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
	it := DatastoreTaskQuery(r.Context(), prototype, params)
	data, err := prototype.Query(it)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to query %s tasks", prototype.GetTaskName()))
		return
	}

	params.CountQuery = true
	it = DatastoreTaskQuery(r.Context(), prototype, params)
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
		count++
	}

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
	ids := make([]int64, len(tasks))
	permissions := make([]Permissions, len(tasks))
	for i := 0; i < len(tasks); i++ {
		deleteAllowed, _ := canDeleteTask(tasks[i], r)
		redoAllowed, _ := canRedoTask(tasks[i], r)
		permissions[i] = Permissions{DeleteAllowed: deleteAllowed, RedoAllowed: redoAllowed}
		ids[i] = tasks[i].GetCommonCols().DatastoreKey.ID
	}
	jsonResponse := map[string]interface{}{
		"data":        data,
		"permissions": permissions,
		"pagination":  pagination,
		"ids":         ids,
	}
	if err := json.NewEncoder(w).Encode(jsonResponse); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode JSON")
		return
	}
}

// TODO(rmistry): Should be able to remove large chunks of the below. See:
// https://skia-review.googlesource.com/c/buildbot/+/238902/14/ct/go/ctfe/task_common/task_common.go#514

// Data included in all update requests.
type UpdateTaskCommonVars struct {
	Id                   int64
	TsStarted            string
	TsCompleted          string
	Failure              bool
	TaskDone             bool
	RepeatAfterDays      int64
	ClearRepeatAfterDays bool
	SwarmingLogs         string
	SwarmingTaskID       string
}

func (vars *UpdateTaskCommonVars) SetStarted(runID string) {
	vars.TsStarted = ctutil.GetCurrentTs()
	swarmingLogsLink := fmt.Sprintf(ctutil.SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, runID)
	vars.SwarmingLogs = swarmingLogsLink
}

func (vars *UpdateTaskCommonVars) GetUpdateTaskCommonVars() *UpdateTaskCommonVars {
	return vars
}

type UpdateTaskVars interface {
	GetTaskPrototype() Task
	GetUpdateTaskCommonVars() *UpdateTaskCommonVars
	// Adds CT task specific updates for fields not in UpdateTaskCommonVars.
	UpdateExtraFields(Task) error
	SetCompleted(success bool, t Task)
}

func updateDatastoreTask(vars UpdateTaskVars, task Task) error {
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
		if task.GetCommonCols().TsCompleted == 0 {
			return fmt.Errorf("TsCompleted must be set before TaskDone can be set to true")
		}
		task.GetCommonCols().TaskDone = common.TaskDone
	}
	if common.ClearRepeatAfterDays {
		task.GetCommonCols().RepeatAfterDays = 0
	} else if common.RepeatAfterDays != 0 {
		task.GetCommonCols().RepeatAfterDays = common.RepeatAfterDays
	}
	if common.SwarmingLogs != "" {
		task.GetCommonCols().SwarmingLogs = common.SwarmingLogs
	}
	if common.SwarmingTaskID != "" {
		task.GetCommonCols().SwarmingTaskID = common.SwarmingTaskID
	}
	if err := vars.UpdateExtraFields(task); err != nil {
		return err
	}
	return nil
}

func UpdateTaskHandler(vars UpdateTaskVars, prototype Task, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewDecoder(r.Body).Decode(&vars); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to parse %T update", vars))
		return
	}

	key := ds.NewKey(prototype.GetDatastoreKind())
	key.ID = vars.GetUpdateTaskCommonVars().Id
	task, err := prototype.Get(r.Context(), key)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to find %T task", vars))
		return
	}

	if err := UpdateTask(r.Context(), vars, task); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to update %T task", vars))
		return
	}
}

func UpdateTaskSetStarted(ctx context.Context, vars UpdateTaskVars, id int64, runID, swarmingTaskID string) error {
	vars.GetUpdateTaskCommonVars().Id = id
	vars.GetUpdateTaskCommonVars().SwarmingTaskID = swarmingTaskID
	vars.GetUpdateTaskCommonVars().SetStarted(runID)
	_, err := FindAndUpdateTask(ctx, vars)
	return err
}

func UpdateTaskSetFailed(ctx context.Context, task Task) error {
	updateVars := task.GetUpdateTaskVars()
	updateVars.GetUpdateTaskCommonVars().Id = task.GetCommonCols().DatastoreKey.ID
	updateVars.GetUpdateTaskCommonVars().TsStarted = ctutil.GetCurrentTs()
	updateVars.SetCompleted(false, task)
	return UpdateTask(ctx, updateVars, task)
}

func FindAndUpdateTask(ctx context.Context, vars UpdateTaskVars) (Task, error) {
	prototype := vars.GetTaskPrototype()
	key := ds.NewKey(prototype.GetDatastoreKind())
	key.ID = vars.GetUpdateTaskCommonVars().Id
	task, err := prototype.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("Failed to find task with Id %d: %s", key.ID, err)
	}

	if err := updateDatastoreTask(vars, task); err != nil {
		return nil, fmt.Errorf("Failed to marshal %T update: %v", vars, err)
	}

	if _, err := ds.DS.Put(ctx, task.GetCommonCols().DatastoreKey, task); err != nil {
		return nil, fmt.Errorf("Failed to update task %d in the datastore: %s", task.GetCommonCols().DatastoreKey.ID, err)
	}
	return task, nil
}

func UpdateTask(ctx context.Context, vars UpdateTaskVars, task Task) error {
	if err := updateDatastoreTask(vars, task); err != nil {
		return fmt.Errorf("Failed to marshal %T update: %v", vars, err)
	}

	if _, err := ds.DS.Put(ctx, task.GetCommonCols().DatastoreKey, task); err != nil {
		return fmt.Errorf("Failed to update task %d in the datastore: %s", task.GetCommonCols().DatastoreKey.ID, err)
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
		httputils.ReportError(w, r, nil, "Please login with google account to delete tasks")
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
	if err := ds.DS.Delete(r.Context(), key); err != nil {
		httputils.ReportError(w, r, err, "Failed to delete")
		return
	}
	sklog.Infof("%s task with ID %d deleted by %s", prototype.GetTaskName(), vars.Id, login.LoggedInAs(r))
}

func RedoTaskHandler(prototype Task, w http.ResponseWriter, r *http.Request) {
	if !ctfeutil.UserHasEditRights(r) {
		httputils.ReportError(w, r, nil, "Please login with google account to redo tasks")
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
	task, err := prototype.Get(r.Context(), key)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to find requested task")
		return
	}

	addTaskVars, err := task.GetPopulatedAddTaskVars()
	if err != nil {
		httputils.ReportError(w, r, err, "Could not GetPopulatedAddTaskVars")
	}
	// Replace the username with the new requester.
	addTaskVars.GetAddTaskCommonVars().Username = login.LoggedInAs(r)
	// Do not preserve repeat_after_days for retried tasks. Carrying over
	// repeat_after_days causes the same task to be unknowingly repeated.
	addTaskVars.GetAddTaskCommonVars().RepeatAfterDays = "0"
	if err := AddAndTriggerTask(r.Context(), addTaskVars); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to insert or trigger %T task", task))
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
	change, err := g.GetIssueProperties(context.TODO(), cl)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to get issue properties from Gerrit")
		return
	}

	// Check to see if the change has any open dependencies.
	activeDep, err := g.HasOpenDependency(context.TODO(), cl, len(change.Patchsets))
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
	isBinary, err := g.IsBinaryPatch(context.TODO(), cl, latestPatchsetID)
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
	patch, err = g.GetPatch(context.TODO(), cl, latestPatchsetID)
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
		"benchmarks": ctutil.SupportedBenchmarksToDoc,
		"platforms":  ctutil.SupportedPlatformsToDesc,
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

func taskPrioritiesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	data := map[string]interface{}{
		"task_priorities": ctutil.TaskPrioritiesToDesc,
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

func GetEmailRecipients(runOwner string, ccList []string) []string {
	emails := []string{runOwner}
	if ccList != nil {
		emails = append(emails, ccList...)
	}
	emails = append(emails, ctutil.CtAdmins...)
	return emails
}

func isAdminHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	data := map[string]interface{}{
		"isAdmin": ctfeutil.UserHasAdminRights(r),
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

func AddHandlers(externalRouter *mux.Router) {
	externalRouter.HandleFunc("/"+ctfeutil.PAGE_SETS_PARAMETERS_POST_URI, pageSetsHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.CL_DATA_POST_URI, getCLHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.BENCHMARKS_PLATFORMS_POST_URI, benchmarksPlatformsHandler).Methods("POST")
	externalRouter.HandleFunc("/"+ctfeutil.TASK_PRIORITIES_GET_URI, taskPrioritiesHandler).Methods("GET")
	externalRouter.HandleFunc("/"+ctfeutil.IS_ADMIN_GET_URI, isAdminHandler).Methods("GET")
}

func Init(ctx context.Context, local bool, ctfeURL, serviceAccountFileFlagVal string, getGCETasksCount func(ctx context.Context) (int, error)) error {
	WebappURL = ctfeURL
	ServiceAccountFile = serviceAccountFileFlagVal
	var err error
	autoscaler, err = ct_autoscaler.NewCTAutoscaler(ctx, local, getGCETasksCount)
	if err != nil {
		return fmt.Errorf("Could not instantiate the CT autoscaler: %s", err)
	}
	return err
}
