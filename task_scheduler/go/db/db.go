package db

import (
	"errors"
	"io"
	"time"

	"github.com/skia-dev/glog"
)

const (
	// Maximum number of simultaneous GetModifiedTasks users.
	MAX_MODIFIED_DATA_USERS = 10

	// Expiration for GetModifiedTasks users.
	MODIFIED_DATA_TIMEOUT = 10 * time.Minute

	// Retries attempted by UpdateWithRetries and UpdateTaskWithRetries.
	NUM_RETRIES = 5
)

var (
	ErrAlreadyExists    = errors.New("Object already exists and modification not allowed.")
	ErrConcurrentUpdate = errors.New("Concurrent update")
	ErrNotFound         = errors.New("Task/Job with given ID does not exist")
	ErrTooManyUsers     = errors.New("Too many users")
	ErrUnknownId        = errors.New("Unknown ID")
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
	io.Closer

	// GetModifiedTasks returns all tasks modified since the last time
	// GetModifiedTasks was run with the given id. The returned tasks are sorted
	// by Created timestamp.
	GetModifiedTasks(string) ([]*Task, error)

	// GetTaskById returns the task with the given Id field. Returns nil, nil if
	// task is not found.
	GetTaskById(string) (*Task, error)

	// GetTasksFromDateRange retrieves all tasks with Created in the given range.
	// The returned tasks are sorted by Created timestamp.
	GetTasksFromDateRange(time.Time, time.Time) ([]*Task, error)

	// StartTrackingModifiedTasks initiates tracking of modified tasks for
	// the current caller. Returns a unique ID which can be used by the caller
	// to retrieve tasks which have been modified since the last query. The ID
	// expires after a period of inactivity.
	StartTrackingModifiedTasks() (string, error)

	// StopTrackingModifiedTasks cancels tracking of modified tasks for the
	// provided ID.
	StopTrackingModifiedTasks(string)
}

// TaskDB is used by the task scheduler to store Tasks.
type TaskDB interface {
	TaskReader

	// AssignId sets the given task's Id field. Does not insert the task into the
	// database.
	AssignId(*Task) error

	// PutTask inserts or updates the Task in the database. Task's Id field must
	// be empty or set with AssignId. PutTask will set Task.DbModified.
	PutTask(*Task) error

	// PutTasks inserts or updates the Tasks in the database. Each Task's Id field
	// must be empty or set with AssignId. Each Task's DbModified field will be
	// set.
	PutTasks([]*Task) error
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
func UpdateTasksWithRetries(db TaskDB, f func() ([]*Task, error)) ([]*Task, error) {
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
	glog.Warningf("UpdateWithRetries: %d consecutive ErrConcurrentUpdate.", NUM_RETRIES)
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
func UpdateTaskWithRetries(db TaskDB, id string, f func(*Task) error) (*Task, error) {
	tasks, err := UpdateTasksWithRetries(db, func() ([]*Task, error) {
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
		return []*Task{t}, nil
	})
	if err != nil {
		return nil, err
	} else {
		return tasks[0], nil
	}
}

// JobReader is a read-only view of a JobDB.
type JobReader interface {
	io.Closer

	// GetModifiedJobs returns all jobs modified since the last time
	// GetModifiedJobs was run with the given id. The returned jobs are sorted by
	// Created timestamp.
	GetModifiedJobs(string) ([]*Job, error)

	// GetJobById returns the job with the given Id field. Returns nil, nil if
	// job is not found.
	GetJobById(string) (*Job, error)

	// GetJobsFromDateRange retrieves all jobs with Created in the given range.
	// The returned jobs are sorted by Created timestamp.
	GetJobsFromDateRange(time.Time, time.Time) ([]*Job, error)

	// StartTrackingModifiedJobs initiates tracking of modified jobs for
	// the current caller. Returns a unique ID which can be used by the caller
	// to retrieve jobs which have been modified since the last query. The ID
	// expires after a period of inactivity.
	StartTrackingModifiedJobs() (string, error)

	// StopTrackingModifiedJobs cancels tracking of modified jobs for the
	// provided ID.
	StopTrackingModifiedJobs(string)
}

// JobDB is used by the task scheduler to store Jobs.
type JobDB interface {
	JobReader

	// PutJob inserts or updates the Job in the database. Job's Id field
	// must be empty if it is a new Job. PutJob will set Job.DbModified.
	PutJob(*Job) error

	// PutJobs inserts or updates the Jobs in the database. Each Jobs' Id
	// field must be empty if it is a new Job. Each Jobs' DbModified field
	// will be set.
	PutJobs([]*Job) error
}

// UpdateJobsWithRetries wraps a call to db.PutJobs with retries. It calls
// db.PutJobs(f()) repeatedly until one of the following happen:
//  - f or db.PutJobs returns an error, which is then returned from
//    UpdateJobsWithRetries;
//  - PutJobs succeeds, in which case UpdateJobsWithRetries returns the updated
//    Jobs returned by f;
//  - retries are exhausted, in which case UpdateJobsWithRetries returns
//    ErrConcurrentUpdate.
//
// Within f, jobs should be refreshed from the DB, e.g. with
// db.GetModifiedJobs or db.GetJobById.
// TODO(borenet): We probably don't need this; consider removing.
func UpdateJobsWithRetries(db JobDB, f func() ([]*Job, error)) ([]*Job, error) {
	var lastErr error
	for i := 0; i < NUM_RETRIES; i++ {
		t, err := f()
		if err != nil {
			return nil, err
		}
		lastErr = db.PutJobs(t)
		if lastErr == nil {
			return t, nil
		} else if !IsConcurrentUpdate(lastErr) {
			return nil, lastErr
		}
	}
	glog.Warningf("UpdateWithRetries: %d consecutive ErrConcurrentUpdate.", NUM_RETRIES)
	return nil, lastErr
}

// UpdateJobWithRetries reads, updates, and writes a single Job in the DB. It:
//  1. reads the job with the given id,
//  2. calls f on that job, and
//  3. calls db.PutJob() on the updated job
//  4. repeats from step 1 as long as PutJobs returns ErrConcurrentUpdate and
//     retries have not been exhausted.
// Returns the updated job if it was successfully updated in the DB.
// Immediately returns ErrNotFound if db.GetJobById(id) returns nil.
// Immediately returns any error returned from f or from PutJobs (except
// ErrConcurrentUpdate). Returns ErrConcurrentUpdate if retries are exhausted.
// TODO(borenet): We probably don't need this; consider removing.
func UpdateJobWithRetries(db JobDB, id string, f func(*Job) error) (*Job, error) {
	jobs, err := UpdateJobsWithRetries(db, func() ([]*Job, error) {
		t, err := db.GetJobById(id)
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
		return []*Job{t}, nil
	})
	if err != nil {
		return nil, err
	} else {
		return jobs[0], nil
	}
}

// RemoteDB allows retrieving tasks and jobs and full access to comments.
type RemoteDB interface {
	//JobReader
	TaskReader
	CommentDB
}

// TaskAndCommentDB implements both TaskDB and CommentDB.
type TaskAndCommentDB interface {
	TaskDB
	CommentDB
}
