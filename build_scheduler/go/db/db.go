package db

import (
	"bytes"
	"encoding/gob"
	"errors"
	"time"

	"go.skia.org/infra/go/util"

	swarming "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/skia-dev/glog"
)

const (
	// Maximum number of simultaneous GetModifiedTasks users.
	MAX_MODIFIED_BUILDS_USERS = 10

	// Expiration for GetModifiedTasks users.
	MODIFIED_BUILDS_TIMEOUT = 10 * time.Minute

	// Swarming task states.
	TASK_STATE_BOT_DIED  = "BOT_DIED"
	TASK_STATE_CANCELED  = "CANCELED"
	TASK_STATE_COMPLETED = "COMPLETED"
	TASK_STATE_EXPIRED   = "EXPIRED"
	TASK_STATE_PENDING   = "PENDING"
	TASK_STATE_RUNNING   = "RUNNING"
	TASK_STATE_TIMED_OUT = "TIMED_OUT"
)

var (
	ErrTooManyUsers = errors.New("Too many users")
	ErrUnknownId    = errors.New("Unknown ID")

	// Valid final states for Swarming tasks.
	TASK_FINISHED_STATES = []string{
		TASK_STATE_BOT_DIED,
		TASK_STATE_CANCELED,
		TASK_STATE_COMPLETED,
		TASK_STATE_EXPIRED,
		TASK_STATE_TIMED_OUT,
	}
)

func IsTooManyUsers(e error) bool {
	return e != nil && e.Error() == ErrTooManyUsers.Error()
}

func IsUnknownId(e error) bool {
	return e != nil && e.Error() == ErrUnknownId.Error()
}

// Task is a struct which describes a Swarming task, generated from a TaskSpec.
type Task struct {
	// Task contains information directly from Swarming.
	*swarming.SwarmingRpcsTaskRequestMetadata

	// Commits are the commits which were tested in this Task. The list may
	// change due to backfilling/bisecting.
	Commits []string

	// Id is a generated unique identifier for this Task instance.
	Id string

	// IsolatedOutput is the isolated hash of any outputs produced by this
	// Task. Filled in when the task is completed.
	IsolatedOutput string

	// Name is a human-friendly descriptive name for this Task. All Tasks
	// generated from the same TaskSpec have the same name.
	Name string

	// Revision is the commit at which this task ran.
	Revision string
}

func (t *Task) Created() (time.Time, error) {
	return util.ParseTimeNs(t.TaskResult.CreatedTs)
}

func (t *Task) Finished() bool {
	return util.In(t.TaskResult.State, TASK_FINISHED_STATES)
}

func (t *Task) Success() bool {
	return t.TaskResult.State == TASK_STATE_COMPLETED && !t.TaskResult.Failure
}

func (t *Task) Copy() *Task {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(t); err != nil {
		glog.Fatal(err)
	}
	var rv Task
	if err := gob.NewDecoder(&buf).Decode(&rv); err != nil {
		glog.Fatal(err)
	}
	return &rv
}

type DB interface {
	// Close the [connection to the] DB.
	Close() error

	// GetTasksFromDateRange retrieves all builds which started in the given date range.
	GetTasksFromDateRange(time.Time, time.Time) ([]*Task, error)

	// GetModifiedTasks returns all builds modified since the last time
	// GetModifiedTasks was run with the given id.
	GetModifiedTasks(string) ([]*Task, error)

	// PutTask inserts or updates the Task in the database.
	PutTask(*Task) error

	// PutTasks inserts or updates the Tasks in the database.
	PutTasks([]*Task) error

	// StartTrackingModifiedTasks initiates tracking of modified builds for
	// the current caller. Returns a unique ID which can be used by the caller
	// to retrieve builds which have been modified since the last query. The ID
	// expires after a period of inactivity.
	StartTrackingModifiedTasks() (string, error)
}
