package db

import (
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
	// Maximum number of simultaneous GetModifiedTasks users.
	MAX_MODIFIED_DATA_USERS = 20

	// Expiration for GetModifiedTasks users.
	MODIFIED_DATA_TIMEOUT = 30 * time.Minute

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

// ModifiedTasksReader tracks which tasks have been modified and returns results
// to subscribers based on what has changed since the last call to
// GetModifiedTasks.
type ModifiedTasksReader interface {
	// GetModifiedTasks returns all tasks modified since the last time
	// GetModifiedTasks was run with the given id. The returned tasks are sorted
	// by Created timestamp. If GetModifiedTasks returns an error, the caller
	// should call StopTrackingModifiedTasks and StartTrackingModifiedTasks
	// again, and load all data from scratch to be sure that no tasks were
	// missed.
	GetModifiedTasks(string) ([]*types.Task, error)

	// GetModifiedTasksGOB returns the GOB-encoded results of GetModifiedTasks,
	// keyed by Task.Id. Callers should not modify the returned byte slices.
	GetModifiedTasksGOB(string) (map[string][]byte, error)

	// StartTrackingModifiedTasks initiates tracking of modified tasks for
	// the current caller. Returns a unique ID which can be used by the caller
	// to retrieve tasks which have been modified since the last query. The ID
	// expires after a period of inactivity.
	StartTrackingModifiedTasks() (string, error)

	// StopTrackingModifiedTasks cancels tracking of modified tasks for the
	// provided ID.
	StopTrackingModifiedTasks(string)
}

// ModifiedTasks tracks which tasks have been modified and returns results to
// subscribers based on what has changed since the last call to
// GetModifiedTasks.
type ModifiedTasks interface {
	ModifiedTasksReader

	// TrackModifiedTask indicates the given Task should be returned from the next
	// call to GetModifiedTasks from each subscriber.
	TrackModifiedTask(*types.Task)

	// TrackModifiedTasksGOB is a batch, GOB version of TrackModifiedTask. Given a
	// map from Task.Id to GOB-encoded task, it is equivalent to GOB-decoding each
	// value of gobs as a Task and calling TrackModifiedTask on each one. Values of
	// gobs must not be modified after this call. The time parameter is the
	// DbModified timestamp of the tasks.
	TrackModifiedTasksGOB(time.Time, map[string][]byte)
}

// TaskReader is a read-only view of a TaskDB.
type TaskReader interface {
	ModifiedTasksReader

	// GetTaskById returns the task with the given Id field. Returns nil, nil if
	// task is not found.
	GetTaskById(string) (*types.Task, error)

	// GetTasksFromDateRange retrieves all tasks with Created in the given range.
	// The returned tasks are sorted by Created timestamp. The string field is
	// an optional repository; if provided, only return tasks associated with
	// that repo.
	GetTasksFromDateRange(time.Time, time.Time, string) ([]*types.Task, error)
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

// ModifiedJobsReader tracks which tasks have been modified and returns results
// to subscribers based on what has changed since the last call to
// GetModifiedJobs.
type ModifiedJobsReader interface {
	// GetModifiedJobs returns all jobs modified since the last time
	// GetModifiedJobs was run with the given id. The returned jobs are sorted by
	// Created timestamp. If GetModifiedJobs returns an error, the caller
	// should call StopTrackingModifiedJobs and StartTrackingModifiedJobs
	// again, and load all data from scratch to be sure that no jobs were
	// missed.
	GetModifiedJobs(string) ([]*types.Job, error)

	// GetModifiedJobsGOB returns the GOB-encoded results of GetModifiedJobs,
	// keyed by Job.Id. Callers should not modify the returned byte slices.
	GetModifiedJobsGOB(string) (map[string][]byte, error)

	// StartTrackingModifiedJobs initiates tracking of modified jobs for
	// the current caller. Returns a unique ID which can be used by the caller
	// to retrieve jobs which have been modified since the last query. The ID
	// expires after a period of inactivity.
	StartTrackingModifiedJobs() (string, error)

	// StopTrackingModifiedJobs cancels tracking of modified jobs for the
	// provided ID.
	StopTrackingModifiedJobs(string)
}

// ModifiedJobs tracks which tasks have been modified and returns results to
// subscribers based on what has changed since the last call to
// GetModifiedJobs.
type ModifiedJobs interface {
	ModifiedJobsReader

	// TrackModifiedJob indicates the given Job should be returned from the next
	// call to GetModifiedJobs from each subscriber.
	TrackModifiedJob(*types.Job)

	// TrackModifiedJobsGOB is a batch, GOB version of TrackModifiedJob. Given a
	// map from Job.Id to GOB-encoded task, it is equivalent to GOB-decoding each
	// value of gobs as a Job and calling TrackModifiedJob on each one. Values of
	// gobs must not be modified after this call. The time parameter is the
	// DbModified timestamp of the jobs.
	TrackModifiedJobsGOB(time.Time, map[string][]byte)
}

// JobReader is a read-only view of a JobDB.
type JobReader interface {
	ModifiedJobsReader

	// GetJobById returns the job with the given Id field. Returns nil, nil if
	// job is not found.
	GetJobById(string) (*types.Job, error)

	// GetJobsFromDateRange retrieves all jobs with Created in the given range.
	// The returned jobs are sorted by Created timestamp.
	GetJobsFromDateRange(time.Time, time.Time) ([]*types.Job, error)
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

// ModifiedData combines ModifiedTasks, ModifiedJobs, and ModifiedComments.
type ModifiedData interface {
	ModifiedTasks
	ModifiedJobs
	ModifiedComments
}

// modifiedData implements ModifiedData.
type modifiedData struct {
	ModifiedTasks
	ModifiedJobs
	ModifiedComments
}

// NewModifiedData returns a ModifiedData which combines the given
// ModifiedTasks, ModifiedJobs, and ModifiedComments.
func NewModifiedData(t ModifiedTasks, j ModifiedJobs, c ModifiedComments) ModifiedData {
	return &modifiedData{
		ModifiedTasks:    t,
		ModifiedJobs:     j,
		ModifiedComments: c,
	}
}

// JobSearchParams are parameters on which Jobs may be searched. All fields
// are optional; if a field is not provided, the search will return Jobs with
// any value for that field. If either of TimeStart or TimeEnd is not provided,
// the search defaults to the last 24 hours.
type JobSearchParams struct {
	types.RepoState
	BuildbucketBuildId *int64          `json:"buildbucket_build_id,string,omitempty"`
	IsForce            *bool           `json:"is_force,omitempty"`
	Name               string          `json:"name"`
	Status             types.JobStatus `json:"status"`
	TimeStart          time.Time       `json:"time_start"`
	TimeEnd            time.Time       `json:"time_end"`
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
// either not provided or equal to the second.
func searchStringEqual(search, test string) bool {
	if search == "" {
		return true
	}
	return search == test
}

// matchJobs returns Jobs which match the given search parameters.
func matchJobs(jobs []*types.Job, p *JobSearchParams) ([]*types.Job, error) {
	// We accept a regex for the job name.
	nameRe, err := regexp.Compile(p.Name)
	if err != nil {
		return nil, err
	}

	rv := []*types.Job{}
	for _, j := range jobs {
		// Compare all attributes which are provided.
		if true &&
			!p.TimeStart.After(j.Created) &&
			j.Created.Before(p.TimeEnd) &&
			searchStringEqual(p.Issue, j.Issue) &&
			searchStringEqual(p.Patchset, j.Patchset) &&
			searchStringEqual(p.Server, j.Server) &&
			searchStringEqual(p.Repo, j.Repo) &&
			searchStringEqual(p.Revision, j.Revision) &&
			nameRe.MatchString(j.Name) &&
			searchStringEqual(string(p.Status), string(j.Status)) &&
			searchBoolEqual(p.IsForce, j.IsForce) &&
			searchInt64Equal(p.BuildbucketBuildId, j.BuildbucketBuildId) {
			rv = append(rv, j)
		}
	}
	return rv, nil
}

// SearchJobs returns Jobs in the given time range which match the given search
// parameters.
func SearchJobs(db JobReader, p *JobSearchParams) ([]*types.Job, error) {
	if util.TimeIsZero(p.TimeStart) || util.TimeIsZero(p.TimeEnd) {
		p.TimeEnd = time.Now()
		p.TimeStart = p.TimeEnd.Add(-24 * time.Hour)
	}
	jobs, err := db.GetJobsFromDateRange(p.TimeStart, p.TimeEnd)
	if err != nil {
		return nil, err
	}
	return matchJobs(jobs, p)
}

// matchTasks returns Tasks which match the given search parameters.
func matchTasks(tasks []*types.Task, p *TaskSearchParams) []*types.Task {
	rv := []*types.Task{}
	for _, t := range tasks {
		// Compare all attributes which are provided.
		if true &&
			!p.TimeStart.After(t.Created) &&
			t.Created.Before(p.TimeEnd) &&
			searchInt64Equal(p.Attempt, int64(t.Attempt)) &&
			searchStringEqual(p.Issue, t.Issue) &&
			searchStringEqual(p.Patchset, t.Patchset) &&
			searchStringEqual(p.Server, t.Server) &&
			searchStringEqual(p.Repo, t.Repo) &&
			searchStringEqual(p.Revision, t.Revision) &&
			searchStringEqual(p.Name, t.Name) &&
			searchStringEqual(string(p.Status), string(t.Status)) &&
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
	Attempt *int64           `json:"attempt,string,omitempty"`
	Status  types.TaskStatus `json:"status"`
	types.TaskKey
	TimeStart time.Time `json:"time_start"`
	TimeEnd   time.Time `json:"time_end"`
}

// SearchTasks returns Tasks in the given time range which match the given search
// parameters.
func SearchTasks(db TaskReader, p *TaskSearchParams) ([]*types.Task, error) {
	if util.TimeIsZero(p.TimeStart) || util.TimeIsZero(p.TimeEnd) {
		p.TimeEnd = time.Now()
		p.TimeStart = p.TimeEnd.Add(-24 * time.Hour)
	}
	tasks, err := db.GetTasksFromDateRange(p.TimeStart, p.TimeEnd, p.Repo)
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

// BackupDBCloser is a DBCloser that provides backups.
type BackupDBCloser interface {
	DBCloser

	// WriteBackup writes a backup of the DB to the given io.Writer.
	WriteBackup(io.Writer) error

	// SetIncrementalBackupTime marks the given time as a checkpoint for
	// incremental backups.
	SetIncrementalBackupTime(time.Time) error
	// GetIncrementalBackupTime returns the most recent time provided to
	// SetIncrementalBackupTime. Any incremental backups taken after the returned
	// time should be reapplied to the DB.
	GetIncrementalBackupTime() (time.Time, error)
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
		sklog.Infof("Reading Tasks from %s to %s.", start, now)
		t0 := time.Now()
		t, err := db.GetTasksFromDateRange(start, now, repo)
		if err != nil {
			return nil, err
		}
		sklog.Infof("Read %d tasks in %s", len(t), time.Now().Sub(t0))
		tasks = append(tasks, t...)
	}
	sort.Sort(types.TaskSlice(tasks))
	return tasks, nil
}

var errNotModified = errors.New("Task not modified")

// UpdateDBFromSwarmingTask updates a task in db from data in s.
func UpdateDBFromSwarmingTask(db TaskDB, s *swarming_api.SwarmingRpcsTaskResult) error {
	id, err := swarming.GetTagValue(s, types.SWARMING_TAG_ID)
	if err != nil {
		return err
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
		return nil
	} else {
		return err
	}
}
