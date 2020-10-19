package db

import (
	"context"
	"errors"
	"io"
	"regexp"
	"sort"
	"time"

	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

const (
	// Retries attempted by UpdateTasksWithRetries.
	NUM_RETRIES = 5
)

var (
	ErrAlreadyExists    = errors.New("Object already exists and modification not allowed.")
	ErrConcurrentUpdate = errors.New("Concurrent update")
	ErrNotFound         = errors.New("Task/Job with given ID does not exist")
	ErrTooManyUsers     = errors.New("Too many users")
	ErrUnknownId        = types.ErrUnknownId
)

func IsAlreadyExists(e error) bool {
	return e != nil && e.Error() == ErrAlreadyExists.Error()
}

func IsConcurrentUpdate(e error) bool {
	return e != nil && e.Error() == ErrConcurrentUpdate.Error()
}

func IsNotFound(e error) bool {
	return e != nil && e.Error() == ErrNotFound.Error()
}

func IsTooManyUsers(e error) bool {
	return e != nil && e.Error() == ErrTooManyUsers.Error()
}

func IsUnknownId(e error) bool {
	return e != nil && e.Error() == ErrUnknownId.Error()
}

// TaskReader is a read-only view of a TaskDB.
type TaskReader interface {
	// GetTaskById returns the task with the given Id field. Returns nil, nil if
	// task is not found.
	GetTaskById(string) (*types.Task, error)

	// GetTasksFromDateRange retrieves all tasks with Created in the given range.
	// The returned tasks are sorted by Created timestamp. The string field is
	// an optional repository; if provided, only return tasks associated with
	// that repo.
	GetTasksFromDateRange(time.Time, time.Time, string) ([]*types.Task, error)

	// ModifiedTasksCh returns a channel which produces Tasks as they are
	// modified in the DB. The channel is closed when the given Context is
	// canceled. The channel will immediately produce a slice of Tasks which
	// may or may not be empty.
	ModifiedTasksCh(context.Context) <-chan []*types.Task
}

// TaskDB is used by the task scheduler to store Tasks.
type TaskDB interface {
	TaskReader

	// AssignId sets the given task's Id field. Does not insert the task into the
	// database.
	AssignId(*types.Task) error

	// PutTask inserts or updates the Task in the database. Task's Id field must
	// be empty or set with AssignId. PutTask will set Task.DbModified.
	PutTask(*types.Task) error

	// PutTasks inserts or updates the Tasks in the database. Each Task's Id field
	// must be empty or set with AssignId. Each Task's DbModified field will be
	// set. All modifications are performed in a single transaction; the
	// caller must determine what to do when there are too many Tasks to be
	// inserted at once for a given TaskDB implementation. Use only when
	// consistency is important; otherwise, callers should use
	// PutTasksInChunks.
	PutTasks([]*types.Task) error

	// PutTasksInChunks is like PutTasks but inserts Tasks in multiple
	// transactions. Not appropriate for updates in which consistency is
	// important.
	PutTasksInChunks([]*types.Task) error
}

// UpdateTasksWithRetries wraps a call to db.PutTasks with retries. It calls
// db.PutTasks(f()) repeatedly until one of the following happen:
//  - f or db.PutTasks returns an error, which is then returned from
//    UpdateTasksWithRetries;
//  - PutTasks succeeds, in which case UpdateTasksWithRetries returns the updated
//    Tasks returned by f;
//  - retries are exhausted, in which case UpdateTasksWithRetries returns
//    ErrConcurrentUpdate.
//
// Within f, tasks should be refreshed from the DB, e.g. with
// db.GetModifiedTasks or db.GetTaskById.
func UpdateTasksWithRetries(db TaskDB, f func() ([]*types.Task, error)) ([]*types.Task, error) {
	var lastErr error
	for i := 0; i < NUM_RETRIES; i++ {
		t, err := f()
		if err != nil {
			return nil, err
		}
		lastErr = db.PutTasks(t)
		if lastErr == nil {
			return t, nil
		} else if !IsConcurrentUpdate(lastErr) {
			return nil, lastErr
		}
	}
	sklog.Warningf("UpdateWithRetries: %d consecutive ErrConcurrentUpdate.", NUM_RETRIES)
	return nil, lastErr
}

// UpdateTaskWithRetries reads, updates, and writes a single Task in the DB. It:
//  1. reads the task with the given id,
//  2. calls f on that task, and
//  3. calls db.PutTask() on the updated task
//  4. repeats from step 1 as long as PutTasks returns ErrConcurrentUpdate and
//     retries have not been exhausted.
// Returns the updated task if it was successfully updated in the DB.
// Immediately returns ErrNotFound if db.GetTaskById(id) returns nil.
// Immediately returns any error returned from f or from PutTasks (except
// ErrConcurrentUpdate). Returns ErrConcurrentUpdate if retries are exhausted.
func UpdateTaskWithRetries(db TaskDB, id string, f func(*types.Task) error) (*types.Task, error) {
	tasks, err := UpdateTasksWithRetries(db, func() ([]*types.Task, error) {
		t, err := db.GetTaskById(id)
		if err != nil {
			return nil, err
		}
		if t == nil {
			return nil, ErrNotFound
		}
		err = f(t)
		if err != nil {
			return nil, err
		}
		return []*types.Task{t}, nil
	})
	if err != nil {
		return nil, err
	} else {
		return tasks[0], nil
	}
}

// JobReader is a read-only view of a JobDB.
type JobReader interface {
	// GetJobById returns the job with the given Id field. Returns nil, nil if
	// job is not found.
	GetJobById(string) (*types.Job, error)

	// GetJobsFromDateRange retrieves all jobs with Created in the given
	// range. The returned jobs are sorted by Created timestamp. The string
	// field is an optional repository; if provided, only return tasks
	// associated with that repo.
	GetJobsFromDateRange(time.Time, time.Time, string) ([]*types.Job, error)

	// ModifiedJobsCh returns a channel which produces Jobs as they are
	// modified in the DB. The channel is closed when the given Context is
	// canceled. The channel will immediately produce a slice of Jobs which
	// may or may not be empty.
	ModifiedJobsCh(context.Context) <-chan []*types.Job
}

// JobDB is used by the task scheduler to store Jobs.
type JobDB interface {
	JobReader

	// PutJob inserts or updates the Job in the database. Job's Id field
	// must be empty if it is a new Job. PutJob will set Job.DbModified.
	PutJob(*types.Job) error

	// PutJobs inserts or updates the Jobs in the database. Each Jobs' Id
	// field must be empty if it is a new Job. Each Jobs' DbModified field
	// will be set. All modifications are performed in a single transaction;
	// the caller must determine what to do when there are too many Jobs to
	// be inserted at once for a given JobDB implementation. Use only when
	// consistency is important; otherwise, callers should use
	// PutJobsInChunks.
	PutJobs([]*types.Job) error

	// PutJobsInChunks is like PutJobs but inserts Jobs in multiple
	// transactions. Not appropriate for updates in which consistency is
	// important.
	PutJobsInChunks([]*types.Job) error
}

// JobSearchParams are parameters on which Jobs may be searched. All fields
// are optional; if a field is not provided, the search will return Jobs with
// any value for that field. If either of TimeStart or TimeEnd is not provided,
// the search defaults to the last 24 hours.
type JobSearchParams struct {
	BuildbucketBuildID *int64           `json:"buildbucket_build_id,string,omitempty"`
	IsForce            *bool            `json:"is_force,omitempty"`
	Issue              *string          `json:"issue,omitempty"`
	Name               *string          `json:"name"`
	Patchset           *string          `json:"patchset,omitempty"`
	Repo               *string          `json:"repo,omitempty"`
	Revision           *string          `json:"revision,omitempty"`
	Status             *types.JobStatus `json:"status"`
	TimeStart          *time.Time       `json:"time_start"`
	TimeEnd            *time.Time       `json:"time_end"`
}

// searchBoolEqual compares the two bools and returns true if the first is
// nil or equal to the second.
func searchBoolEqual(search *bool, test bool) bool {
	if search == nil {
		return true
	}
	return *search == test
}

// searchInt64Equal compares the two int64s and returns true if the first is
// either nil or equal to the second.
func searchInt64Equal(search *int64, test int64) bool {
	if search == nil {
		return true
	}
	return *search == test
}

// searchStringEqual compares the two strings and returns true if the first is
// either not provided, equal to the second, or a regular expression which
// matches the second.
func searchStringEqual(search *string, test string) bool {
	if search == nil {
		return true
	}
	if *search == test {
		return true
	}
	re, err := regexp.Compile(*search)
	if err == nil && re.MatchString(test) {
		return true
	}
	return false
}

// matchJobs returns Jobs which match the given search parameters.
func matchJobs(jobs []*types.Job, p *JobSearchParams) []*types.Job {
	rv := []*types.Job{}
	for _, j := range jobs {
		// Compare all attributes which are provided.
		if true &&
			(p.TimeStart == nil || !(*p.TimeStart).After(j.Created)) &&
			(p.TimeEnd == nil || j.Created.Before(*p.TimeEnd)) &&
			searchStringEqual(p.Issue, j.Issue) &&
			searchStringEqual(p.Name, j.Name) &&
			searchStringEqual(p.Patchset, j.Patchset) &&
			searchStringEqual(p.Repo, j.Repo) &&
			searchStringEqual(p.Revision, j.Revision) &&
			searchStringEqual((*string)(p.Status), string(j.Status)) &&
			searchBoolEqual(p.IsForce, j.IsForce) &&
			searchInt64Equal(p.BuildbucketBuildID, j.BuildbucketBuildId) {
			rv = append(rv, j)
		}
	}
	return rv
}

// SearchJobs returns Jobs in the given time range which match the given search
// parameters.
func SearchJobs(db JobReader, p *JobSearchParams) ([]*types.Job, error) {
	if p.TimeEnd == nil || util.TimeIsZero(*p.TimeEnd) {
		end := time.Now()
		p.TimeEnd = &end
	}
	if p.TimeStart == nil || util.TimeIsZero(*p.TimeStart) {
		start := (*p.TimeEnd).Add(-24 * time.Hour)
		p.TimeStart = &start
	}
	repo := ""
	if p.Repo != nil {
		repo = *p.Repo
	}
	jobs, err := db.GetJobsFromDateRange(*p.TimeStart, *p.TimeEnd, repo)
	if err != nil {
		return nil, err
	}
	return matchJobs(jobs, p), nil
}

// matchTasks returns Tasks which match the given search parameters.
func matchTasks(tasks []*types.Task, p *TaskSearchParams) []*types.Task {
	rv := []*types.Task{}
	for _, t := range tasks {
		// Compare all attributes which are provided.
		if true &&
			!p.TimeStart.After(t.Created) &&
			t.Created.Before(*p.TimeEnd) &&
			searchInt64Equal(p.Attempt, int64(t.Attempt)) &&
			searchStringEqual(p.Issue, t.Issue) &&
			searchStringEqual(p.Patchset, t.Patchset) &&
			searchStringEqual(p.Repo, t.Repo) &&
			searchStringEqual(p.Revision, t.Revision) &&
			searchStringEqual(p.Name, t.Name) &&
			searchStringEqual((*string)(p.Status), string(t.Status)) &&
			searchStringEqual(p.ForcedJobId, t.ForcedJobId) {
			rv = append(rv, t)
		}
	}
	return rv
}

// TaskSearchParams are parameters on which Tasks may be searched. All fields
// are optional; if a field is not provided, the search will return Tasks with
// any value for that field. If either of TimeStart or TimeEnd is not provided,
// the search defaults to the last 24 hours.
type TaskSearchParams struct {
	Attempt     *int64            `json:"attempt,string,omitempty"`
	Status      *types.TaskStatus `json:"status"`
	ForcedJobId *string           `json:"forcedJobId,omitempty"`
	Issue       *string           `json:"issue,omitempty"`
	Name        *string           `json:"name"`
	Patchset    *string           `json:"patchset,omitempty"`
	Repo        *string           `json:"repo,omitempty"`
	Revision    *string           `json:"revision,omitempty"`
	TimeStart   *time.Time        `json:"time_start"`
	TimeEnd     *time.Time        `json:"time_end"`
}

// SearchTasks returns Tasks in the given time range which match the given search
// parameters.
func SearchTasks(db TaskReader, p *TaskSearchParams) ([]*types.Task, error) {
	if p.TimeEnd == nil || util.TimeIsZero(*p.TimeEnd) {
		end := time.Now()
		p.TimeEnd = &end
	}
	if p.TimeStart == nil || util.TimeIsZero(*p.TimeStart) {
		start := (*p.TimeEnd).Add(-24 * time.Hour)
		p.TimeStart = &start
	}
	repo := ""
	if p.Repo != nil {
		repo = *p.Repo
	}
	tasks, err := db.GetTasksFromDateRange(*p.TimeStart, *p.TimeEnd, repo)
	if err != nil {
		return nil, err
	}
	return matchTasks(tasks, p), nil
}

// RemoteDB allows retrieving tasks and jobs and full access to comments.
type RemoteDB interface {
	TaskReader
	JobReader
	CommentDB
}

// DB implements TaskDB, JobDB, and CommentDB.
type DB interface {
	TaskDB
	JobDB
	CommentDB
}

// DBCloser is a DB that must be closed when no longer in use.
type DBCloser interface {
	io.Closer
	DB
}

// federatedDB joins an independent TaskDB, JobDB, and CommentDB into a DB.
type federatedDB struct {
	TaskDB
	JobDB
	CommentDB
}

// NewDB returns a DB that delegates to independent TaskDB, JobDB, and
// CommentDB.
func NewDB(tdb TaskDB, jdb JobDB, cdb CommentDB) DB {
	return &federatedDB{
		TaskDB:    tdb,
		JobDB:     jdb,
		CommentDB: cdb,
	}
}

// GetTasksFromWindow returns all tasks matching the given Window from the
// TaskReader.
func GetTasksFromWindow(db TaskReader, w *window.Window, now time.Time) ([]*types.Task, error) {
	defer metrics2.FuncTimer().Stop()

	startTimesByRepo := w.StartTimesByRepo()
	if len(startTimesByRepo) == 0 {
		// If the timeWindow has no associated repos, default to loading
		// tasks for all repos from the beginning of the timeWindow.
		startTimesByRepo[""] = w.EarliestStart()
	}
	tasks := make([]*types.Task, 0, 1024)
	for repo, start := range startTimesByRepo {
		sklog.Infof("Reading Tasks in %s from %s to %s.", repo, start, now)
		t0 := time.Now()
		t, err := db.GetTasksFromDateRange(start, now, repo)
		if err != nil {
			return nil, err
		}
		sklog.Infof("Read %d tasks from %s in %s", len(t), repo, time.Now().Sub(t0))
		tasks = append(tasks, t...)
	}
	sort.Sort(types.TaskSlice(tasks))
	return tasks, nil
}

// GetJobsFromWindow returns all jobs matching the given Window from the
// JobReader.
func GetJobsFromWindow(db JobReader, w *window.Window, now time.Time) ([]*types.Job, error) {
	defer metrics2.FuncTimer().Stop()

	startTimesByRepo := w.StartTimesByRepo()
	if len(startTimesByRepo) == 0 {
		// If the timeWindow has no associated repos, default to loading
		// tasks for all repos from the beginning of the timeWindow.
		startTimesByRepo[""] = w.EarliestStart()
	}
	jobs := make([]*types.Job, 0, 1024)
	for repo, start := range startTimesByRepo {
		sklog.Infof("Reading Jobs in %s from %s to %s.", repo, start, now)
		t0 := time.Now()
		j, err := db.GetJobsFromDateRange(start, now, repo)
		if err != nil {
			return nil, err
		}
		sklog.Infof("Read %d jobs from %s in %s", len(j), repo, time.Now().Sub(t0))
		jobs = append(jobs, j...)
	}
	sort.Sort(types.JobSlice(jobs))
	return jobs, nil
}

var errNotModified = errors.New("Task not modified")

// UpdateDBFromSwarmingTask updates a task in db from data in s.
func UpdateDBFromSwarmingTask(db TaskDB, s *swarming_api.SwarmingRpcsTaskResult) (bool, error) {
	id, err := swarming.GetTagValue(s, types.SWARMING_TAG_ID)
	if err != nil {
		return false, err
	}
	_, err = UpdateTaskWithRetries(db, id, func(task *types.Task) error {
		modified, err := task.UpdateFromSwarming(s)
		if err != nil {
			return err
		}
		if !modified {
			return errNotModified
		}
		return nil
	})
	if err == errNotModified {
		return false, nil
	} else if err != nil {
		return false, err
	} else {
		return true, nil
	}
}
