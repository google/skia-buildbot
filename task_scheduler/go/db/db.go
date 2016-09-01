package db

import (
	"errors"
	"io"
	"time"

	"github.com/skia-dev/glog"
)

const (
	// Maximum number of simultaneous GetModifiedTasks users.
	MAX_MODIFIED_TASKS_USERS = 10

	// Expiration for GetModifiedTasks users.
	MODIFIED_TASKS_TIMEOUT = 10 * time.Minute

	// Retries attempted by UpdateWithRetries and UpdateTaskWithRetries.
	NUM_RETRIES = 5
)

var (
	ErrAlreadyExists    = errors.New("Object already exists and modification not allowed.")
	ErrConcurrentUpdate = errors.New("Concurrent update")
	ErrNotFound         = errors.New("Task with given ID does not exist")
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

// TaskReader is a read-only view of a DB.
type TaskReader interface {
	io.Closer

	// GetModifiedTasks returns all tasks modified since the last time
	// GetModifiedTasks was run with the given id.
	GetModifiedTasks(string) ([]*Task, error)

	// GetTaskById returns the task with the given Id field. Returns nil, nil if
	// task is not found.
	GetTaskById(string) (*Task, error)

	// GetTasksFromDateRange retrieves all tasks which started in the given date range.
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

// DB is used by the task scheduler to store Tasks.
type DB interface {
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

// UpdateWithRetries wraps a call to db.PutTasks with retries. It calls
// db.PutTasks(f()) repeatedly until one of the following happen:
//  - f or db.PutTasks returns an error, which is then returned from
//    UpdateWithRetries;
//  - PutTasks succeeds, in which case UpdateWithRetries returns the updated
//    Tasks returned by f;
//  - retries are exhausted, in which case UpdateWithRetries returns
//    ErrConcurrentUpdate.
//
// Within f, tasks should be refreshed from the DB, e.g. with
// db.GetModifiedTasks or db.GetTaskById.
func UpdateWithRetries(db DB, f func() ([]*Task, error)) ([]*Task, error) {
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
func UpdateTaskWithRetries(db DB, id string, f func(*Task) error) (*Task, error) {
	tasks, err := UpdateWithRetries(db, func() ([]*Task, error) {
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

// RemoteDB allows retrieving tasks and full access to comments.
type RemoteDB interface {
	TaskReader
	CommentDB
}

// TaskAndCommentDB implements both DB and CommentDB.
type TaskAndCommentDB interface {
	DB
	CommentDB
}
