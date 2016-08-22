package db

import (
	"errors"
	"time"
)

const (
	// Maximum number of simultaneous GetModifiedTasks users.
	MAX_MODIFIED_BUILDS_USERS = 10

	// Expiration for GetModifiedTasks users.
	MODIFIED_BUILDS_TIMEOUT = 10 * time.Minute
)

var (
	ErrTooManyUsers     = errors.New("Too many users")
	ErrUnknownId        = errors.New("Unknown ID")
	ErrConcurrentUpdate = errors.New("Concurrent update")
)

func IsTooManyUsers(e error) bool {
	return e != nil && e.Error() == ErrTooManyUsers.Error()
}

func IsUnknownId(e error) bool {
	return e != nil && e.Error() == ErrUnknownId.Error()
}

func IsConcurrentUpdate(e error) bool {
	return e != nil && e.Error() == ErrConcurrentUpdate.Error()
}

type DB interface {
	// AssignId sets the given task's Id field. Does not insert the task into the
	// database.
	AssignId(*Task) error

	// Close the [connection to the] DB.
	Close() error

	// GetModifiedTasks returns all builds modified since the last time
	// GetModifiedTasks was run with the given id.
	GetModifiedTasks(string) ([]*Task, error)

	// GetTaskById returns the task with the given Id field. Returns nil, nil if
	// task is not found.
	GetTaskById(string) (*Task, error)

	// GetTasksFromDateRange retrieves all builds which started in the given date range.
	GetTasksFromDateRange(time.Time, time.Time) ([]*Task, error)

	// PutTask inserts or updates the Task in the database. Task's Id field must
	// be empty or set with AssignId. PutTask will set Task.DbModified.
	PutTask(*Task) error

	// PutTasks inserts or updates the Tasks in the database. Each Task's Id field
	// must be empty or set with AssignId. Each Task's DbModified field will be
	// set.
	PutTasks([]*Task) error

	// StartTrackingModifiedTasks initiates tracking of modified builds for
	// the current caller. Returns a unique ID which can be used by the caller
	// to retrieve builds which have been modified since the last query. The ID
	// expires after a period of inactivity.
	StartTrackingModifiedTasks() (string, error)
}
