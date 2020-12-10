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
	swarmingapi "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/ct/go/ct_autoscaler"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	skutil "go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
)

const (
	// Default page size used for pagination.
	DEFAULT_PAGE_SIZE = 10

	// Maximum page size used for pagination.
	MAX_PAGE_SIZE = 100

	CANCEL_SWARMING_TASKS_WORKER_POOL_SIZE = 100
)

var (
	httpClient       *http.Client
	datastoreIdMutex sync.Mutex

	// CT autoscaler.
	autoscaler ct_autoscaler.ICTAutoscaler

	// The location of the service account JSON file.
	ServiceAccountFile string

	// Will be used to construct task specific URLs in emails. Will have a trailing "/".
	WebappURL string

	swarm swarming.ApiClient
)

type CommonCols struct {
	DatastoreKey    *datastore.Key `json:"-" datastore:"__key__"`
	TsAdded         int64          `json:"ts_added"`
	TsStarted       int64          `json:"ts_started"`
	TsCompleted     int64          `json:"ts_completed"`
	Username        string         `json:"username"`
	Failure         bool           `json:"failure"`
	RepeatAfterDays int64          `json:"repeat_after_days"`
	SwarmingLogs    string         `json:"swarming_logs"`
	TaskDone        bool           `json:"task_done"`
	SwarmingTaskID  string         `json:"swarming_task_id"`

	Id         int    `json:"id" datastore:"-"`
	CanRedo    bool   `json:"can_redo" datastore:"-"`
	CanDelete  bool   `json:"can_delete" datastore:"-"`
	FutureDate bool   `json:"future_date" datastore:"-"`
	TaskType   string `json:"task_type" datastore:"-"`
	GetURL     string `json:"get_url" datastore:"-"`
	DeleteURL  string `json:"delete_url" datastore:"-"`
}

type Task interface {
	GetCommonCols() *CommonCols
	RunsOnGCEWorkers() bool
	TriggerSwarmingTaskAndMail(ctx context.Context, swarmingClient swarming.ApiClient) error
	SendCompletionEmail(ctx context.Context, completedSuccessfully bool) error
	GetTaskName() string
	SetCompleted(success bool)
	GetDatastoreKind() ds.Kind
	GetDescription() string
	// Returns a slice of the struct type.
	Query(it *datastore.Iterator) (interface{}, error)
	// Returns the struct type.
	Get(c context.Context, key *datastore.Key) (Task, error)
	// Returns the corresponding AddTaskVars instance of this Task. The returned
	// instance is populated.
	GetPopulatedAddTaskVars() (AddTaskVars, error)
	// Returns the results link for this task if it completed successfully and if
	// the task supports results links.
	GetResultsLink() string
}

// UpdateTaskSetStarted sets the following on the task and updates it in Datastore:
// * TsStarted
// * SwarmingTaskID
// * SwarmingLogsLink
func UpdateTaskSetStarted(ctx context.Context, runID, swarmingTaskID string, task Task) error {
	task.GetCommonCols().TsStarted = ctutil.GetCurrentTsInt64()
	task.GetCommonCols().SwarmingTaskID = swarmingTaskID
	task.GetCommonCols().SwarmingLogs = fmt.Sprintf(ctutil.SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, runID)

	if _, err := ds.DS.Put(ctx, task.GetCommonCols().DatastoreKey, task); err != nil {
		return fmt.Errorf("Failed to update task %d in the datastore: %s", task.GetCommonCols().DatastoreKey.ID, err)
	}
	return nil
}

// UpdateTaskSetCompleted calls the task's SetCompleted method and updates it in Datastore.
func UpdateTaskSetCompleted(ctx context.Context, task Task, success bool) error {
	task.SetCompleted(success)
	if _, err := ds.DS.Put(ctx, task.GetCommonCols().DatastoreKey, task); err != nil {
		return fmt.Errorf("Failed to update task %d in the datastore: %s", task.GetCommonCols().DatastoreKey.ID, err)
	}
	return nil
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
	Username        string `json:"username"`
	TsAdded         string `json:"ts_added"`
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
		httputils.ReportError(w, nil, "Please login with google account to add tasks", http.StatusInternalServerError)
		return
	}
	if task.IsAdminTask() && !ctfeutil.UserHasAdminRights(r) {
		httputils.ReportError(w, nil, "Must be admin to add admin tasks; contact rmistry@", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to add %T task", task), http.StatusInternalServerError)
		return
	}
	defer skutil.Close(r.Body)

	task.GetAddTaskCommonVars().Username = login.LoggedInAs(r)
	task.GetAddTaskCommonVars().TsAdded = ctutil.GetCurrentTs()
	if len(task.GetAddTaskCommonVars().Username) > 255 {
		httputils.ReportError(w, nil, "Username is too long, limit 255 bytes", http.StatusInternalServerError)
		return
	}

	if err := AddAndTriggerTask(r.Context(), task); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to insert or trigger %T task", task), http.StatusInternalServerError)
		return
	}
}

// AddAndTriggerTask adds the task to datastore and then triggers swarming tasks.
// The swarming tasks are triggered in a separate goroutine because if it is a GCE
// task then it can take a min or 2 to autoscale the GCE instances.
func AddAndTriggerTask(ctx context.Context, task AddTaskVars) error {
	datastoreTask, err := AddTaskToDatastore(ctx, task)
	if err != nil {
		return fmt.Errorf("Failed to insert %T task: %s", task, err)
	}
	go func() {
		// Use a new context because we want the following to finish even after the HTTP
		// request is completed.
		ctx := context.Background()
		if err := TriggerTaskOnSwarming(ctx, task, datastoreTask); err != nil {
			sklog.Errorf("Failed to trigger on swarming %T task: %s", task, err)
			// Populate the started timestamp before we mark it as completed and failed.
			datastoreTask.GetCommonCols().TsStarted = ctutil.GetCurrentTsInt64()
			if err := UpdateTaskSetCompleted(ctx, datastoreTask, false); err != nil {
				sklog.Error(err)
			} else {
				skutil.LogErr(datastoreTask.SendCompletionEmail(ctx, false))
			}
		}
	}()
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
	if autoscaler != nil && datastoreTask.RunsOnGCEWorkers() {
		taskId := fmt.Sprintf("%s.%d", datastoreTask.GetTaskName(), datastoreTask.GetCommonCols().DatastoreKey.ID)
		autoscaler.RegisterGCETask(taskId)
	}
	return datastoreTask.TriggerSwarmingTaskAndMail(ctx, swarm)
}

type QueryParams struct {
	// If non-empty, limits to only tasks with the given username.
	Username string
	// Include only tasks that have completed successfully.
	SuccessfulOnly bool
	// Include only tasks that have completed after the specified timestamp.
	CompletedAfter int
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
	if params.CompletedAfter != 0 {
		q = q.Filter("TsCompleted >", params.CompletedAfter)
		q = q.Order("TsCompleted")
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

type Permissions struct {
	DeleteAllowed bool
	RedoAllowed   bool
}

type GetTasksResponse struct {
	Data        interface{}                   `json:"data"`
	Permissions []Permissions                 `json:"permissions"`
	Pagination  *httputils.ResponsePagination `json:"pagination"`
	IDs         []int64                       `json:"ids"`
}

func GetTasksHandler(prototype Task, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	params := QueryParams{}
	if ctfeutil.ParseBoolFormValue(r.FormValue("filter_by_logged_in_user")) {
		params.Username = login.LoggedInAs(r)
	}
	params.SuccessfulOnly = ctfeutil.ParseBoolFormValue(r.FormValue("successful"))
	params.PendingOnly = ctfeutil.ParseBoolFormValue(r.FormValue("not_completed"))
	params.FutureRunsOnly = ctfeutil.ParseBoolFormValue(r.FormValue("include_future_runs"))
	params.ExcludeDummyPageSets = ctfeutil.ParseBoolFormValue(r.FormValue("exclude_dummy_page_sets"))
	if params.SuccessfulOnly && params.PendingOnly {
		httputils.ReportError(w, fmt.Errorf("Inconsistent params: successful %v not_completed %v", r.FormValue("successful"), r.FormValue("not_completed")), "Inconsistent params", http.StatusInternalServerError)
		return
	}
	offset, size, err := httputils.PaginationParams(r.URL.Query(), 0, DEFAULT_PAGE_SIZE, MAX_PAGE_SIZE)
	if err == nil {
		params.Offset, params.Size = offset, size
	} else {
		httputils.ReportError(w, err, "Failed to get pagination params", http.StatusInternalServerError)
		return
	}
	params.CountQuery = false
	it := DatastoreTaskQuery(r.Context(), prototype, params)
	data, err := prototype.Query(it)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to query %s tasks", prototype.GetTaskName()), http.StatusInternalServerError)
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
			httputils.ReportError(w, err, fmt.Sprintf("Failed to query %s tasks", prototype.GetTaskName()), http.StatusInternalServerError)
			return
		}
		count++
	}

	pagination := &httputils.ResponsePagination{
		Offset: offset,
		Size:   size,
		Total:  count,
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
	// jsonResponse := map[string]interface{}{
	jsonResponse := GetTasksResponse{
		Data:        data,
		Permissions: permissions,
		Pagination:  pagination,
		IDs:         ids,
	}
	if err := json.NewEncoder(w).Encode(jsonResponse); err != nil {
		httputils.ReportError(w, err, "Failed to encode JSON", http.StatusInternalServerError)
		return
	}
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

func getClosedTasksChannel(tasks []*swarmingapi.SwarmingRpcsTaskRequestMetadata) chan *swarmingapi.SwarmingRpcsTaskRequestMetadata {
	// Create channel that contains specified tasks. This channel will be consumed by the worker
	// pool in DeleteTaskHandler.
	tasksChannel := make(chan *swarmingapi.SwarmingRpcsTaskRequestMetadata, len(tasks))

	for _, t := range tasks {
		tasksChannel <- t
	}
	close(tasksChannel)
	return tasksChannel
}

type DeleteTaskRequest struct {
	Id int64 `json:"id"`
}

func DeleteTaskHandler(prototype Task, w http.ResponseWriter, r *http.Request) {
	if !ctfeutil.UserHasEditRights(r) {
		httputils.ReportError(w, nil, "Please login with google account to delete tasks", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	var req DeleteTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to parse delete request", http.StatusInternalServerError)
		return
	}
	defer skutil.Close(r.Body)

	key := ds.NewKey(prototype.GetDatastoreKind())
	key.ID = req.Id
	task, err := prototype.Get(r.Context(), key)
	if err != nil {
		httputils.ReportError(w, err, "Failed to find requested task", http.StatusInternalServerError)
		return
	}

	// If the task is currently running then will have to cancel all of its swarming tasks as well.
	if task.GetCommonCols().TsStarted != 0 && task.GetCommonCols().TsCompleted == 0 {
		runID := GetRunID(task)
		tasks, err := swarm.ListTasks(time.Time{}, time.Time{}, []string{fmt.Sprintf("runid:%s", runID)}, "")
		if err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Could not list tasks for %s", runID), http.StatusInternalServerError)
		}
		sklog.Infof("Starting cancelation of %d tasks...", len(tasks))
		tasksChannel := getClosedTasksChannel(tasks)
		var wg sync.WaitGroup
		// Loop through workers in the worker pool.
		for i := 0; i < CANCEL_SWARMING_TASKS_WORKER_POOL_SIZE; i++ {
			// Increment the WaitGroup counter.
			wg.Add(1)
			// Create and run a goroutine closure that cancels tasks.
			go func() {
				// Decrement the WaitGroup counter when the goroutine completes.
				defer wg.Done()

				for t := range tasksChannel {
					if err := swarm.CancelTask(t.TaskId, true /* killRunning */); err != nil {
						sklog.Errorf("Could not cancel %s: %s", t.TaskId, err)
						continue
					}
					sklog.Infof("Canceled  %s", t.TaskId)
				}
			}()
		}
		// Wait for all spawned goroutines to complete
		wg.Wait()

		sklog.Infof("Cancelled %d tasks.", len(tasks))

		// Send completion email since tasks did start and there was a corresponding start email.
		skutil.LogErr(task.SendCompletionEmail(r.Context(), false))
	}
	if err := ds.DS.Delete(r.Context(), key); err != nil {
		httputils.ReportError(w, err, "Failed to delete", http.StatusInternalServerError)
		return
	}

	sklog.Infof("%s task with ID %d deleted by %s", prototype.GetTaskName(), req.Id, login.LoggedInAs(r))
}

type RedoTaskRequest struct {
	Id int64 `json:"id"`
}

func RedoTaskHandler(prototype Task, w http.ResponseWriter, r *http.Request) {
	if !ctfeutil.UserHasEditRights(r) {
		httputils.ReportError(w, nil, "Please login with google account to redo tasks", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	var req RedoTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to parse redo request", http.StatusInternalServerError)
		return
	}
	defer skutil.Close(r.Body)

	key := ds.NewKey(prototype.GetDatastoreKind())
	key.ID = req.Id
	task, err := prototype.Get(r.Context(), key)
	if err != nil {
		httputils.ReportError(w, err, "Failed to find requested task", http.StatusInternalServerError)
		return
	}

	addTaskVars, err := task.GetPopulatedAddTaskVars()
	if err != nil {
		httputils.ReportError(w, err, "Could not GetPopulatedAddTaskVars", http.StatusInternalServerError)
	}
	// Replace the username with the new requester.
	addTaskVars.GetAddTaskCommonVars().Username = login.LoggedInAs(r)
	// Do not preserve repeat_after_days for retried tasks. Carrying over
	// repeat_after_days causes the same task to be unknowingly repeated.
	addTaskVars.GetAddTaskCommonVars().RepeatAfterDays = "0"
	if err := AddAndTriggerTask(r.Context(), addTaskVars); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to insert or trigger %T task", task), http.StatusInternalServerError)
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
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
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

type CLDataResponse struct {
	CL            string `json:"cl"`
	Subject       string `json:"subject"`
	URL           string `json:"url"`
	Modified      string `json:"modified"`
	ChromiumPatch string `json:"chromium_patch"`
	SkiaPatch     string `json:"skia_patch"`
	V8Patch       string `json:"v8_patch"`
	CatapultPatch string `json:"catapult_patch"`
}

func gatherCLData(detail clDetail, patch string) (*CLDataResponse, error) {
	clData := &CLDataResponse{
		CL:      strconv.FormatInt(detail.Issue, 10),
		Subject: detail.Subject,
		URL:     detail.CodereviewURL,
	}
	modifiedTime, err := time.Parse("2006-01-02 15:04:05.999999", detail.Modified)
	if err != nil {
		sklog.Errorf("Unable to parse modified time for CL %d; input '%s', got %v", detail.Issue, detail.Modified, err)
	} else {
		clData.Modified = modifiedTime.UTC().Format(ctutil.TS_FORMAT)
	}
	switch detail.Project {
	case "chromium", "chromium/src":
		clData.ChromiumPatch = patch
	case "skia":
		clData.SkiaPatch = patch
	case "v8/v8":
		clData.V8Patch = patch
	case "catapult":
		clData.CatapultPatch = patch
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
			httputils.ReportError(w, err, "Failed to encode JSON", http.StatusInternalServerError)
		}
		return
	}
	crURL := matches[1]
	clString := matches[2]
	g, err := gerrit.NewGerrit(crURL, httpClient)
	if err != nil {
		httputils.ReportError(w, err, "Failed to talk to Gerrit", http.StatusInternalServerError)
		return
	}
	cl, err := strconv.ParseInt(clString, 10, 32)
	if err != nil {
		httputils.ReportError(w, err, "Invalid Gerrit CL number", http.StatusInternalServerError)
		return
	}
	change, err := g.GetIssueProperties(context.TODO(), cl)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get issue properties from Gerrit", http.StatusInternalServerError)
		return
	}

	// Check to see if the change has any open dependencies.
	activeDep, err := g.HasOpenDependency(context.TODO(), cl, len(change.Patchsets))
	if err != nil {
		httputils.ReportError(w, err, "Failed to get related changes from Gerrit", http.StatusInternalServerError)
		return
	}
	if activeDep {
		httputils.ReportError(w, err, fmt.Sprintf("This CL has an open dependency. Please squash your changes into a single CL."), http.StatusInternalServerError)
		return
	}

	// Check to see if the change has a binary file.
	latestPatchsetID := strconv.Itoa(len(change.Patchsets))
	isBinary, err := g.IsBinaryPatch(context.TODO(), cl, latestPatchsetID)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get list of files from Gerrit", http.StatusInternalServerError)
		return
	}
	if isBinary {
		httputils.ReportError(w, err, fmt.Sprintf("CT cannot get a full index for binary files via the Gerrit API. Details in skbug.com/7302."), http.StatusInternalServerError)
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
		httputils.ReportError(w, err, "Failed to download patch from Gerrit", http.StatusInternalServerError)
		return
	}

	clData, err := gatherCLData(detail, patch)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get CL data", http.StatusInternalServerError)
		return
	}
	if err = json.NewEncoder(w).Encode(clData); err != nil {
		httputils.ReportError(w, err, "Failed to encode JSON", http.StatusInternalServerError)
		return
	}
}

type BenchmarksPlatformsResponse struct {
	Benchmarks map[string]string `json:"benchmarks"`
	Platforms  map[string]string `json:"platforms"`
}

func benchmarksPlatformsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	data := BenchmarksPlatformsResponse{
		Benchmarks: ctutil.SupportedBenchmarksToDoc,
		Platforms:  ctutil.SupportedPlatformsToDesc,
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
		return
	}
}

type TaskPrioritiesResponse struct {
	TaskPriorities map[int]string `json:"task_priorities"`
}

func taskPrioritiesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	data := TaskPrioritiesResponse{
		TaskPriorities: ctutil.TaskPrioritiesToDesc,
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
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
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
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

func Init(ctx context.Context, local, enableAutoscaler bool, ctfeURL, serviceAccountFileFlagVal string, swarmingClient swarming.ApiClient, getGCETasksCount func(ctx context.Context) (int, error)) error {
	WebappURL = ctfeURL
	if WebappURL[len(WebappURL)-1:] != "/" {
		WebappURL = WebappURL + "/"
	}
	ServiceAccountFile = serviceAccountFileFlagVal
	swarm = swarmingClient
	ts, err := auth.NewDefaultTokenSource(local, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatal(err)
	}
	httpClient = httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	if enableAutoscaler {
		autoscaler, err = ct_autoscaler.NewCTAutoscaler(ctx, local, getGCETasksCount)
		if err != nil {
			return fmt.Errorf("Could not instantiate the CT autoscaler: %s", err)
		}
	}
	return err
}
