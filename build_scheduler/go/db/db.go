package db

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"time"

	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/skia-dev/glog"
)

const (
	// Maximum number of simultaneous GetModifiedTasks users.
	MAX_MODIFIED_BUILDS_USERS = 10

	// Expiration for GetModifiedTasks users.
	MODIFIED_BUILDS_TIMEOUT = 10 * time.Minute

	// Swarming task states.
	SWARMING_STATE_BOT_DIED  = "BOT_DIED"
	SWARMING_STATE_CANCELED  = "CANCELED"
	SWARMING_STATE_COMPLETED = "COMPLETED"
	SWARMING_STATE_EXPIRED   = "EXPIRED"
	SWARMING_STATE_PENDING   = "PENDING"
	SWARMING_STATE_RUNNING   = "RUNNING"
	SWARMING_STATE_TIMED_OUT = "TIMED_OUT"

	// Swarming tags added by Build Scheduler.
	SWARMING_TAG_ID       = "scheduler_id"
	SWARMING_TAG_NAME     = "name"
	SWARMING_TAG_REPO     = "repo"
	SWARMING_TAG_REVISION = "revision"
)

var (
	ErrTooManyUsers = errors.New("Too many users")
	ErrUnknownId    = errors.New("Unknown ID")
)

func IsTooManyUsers(e error) bool {
	return e != nil && e.Error() == ErrTooManyUsers.Error()
}

func IsUnknownId(e error) bool {
	return e != nil && e.Error() == ErrUnknownId.Error()
}

type TaskStatus string

const (
	// TASK_STATUS_PENDING indicates the task has not started. It is the empty
	// string so that it is the zero value of TaskStatus.
	TASK_STATUS_PENDING TaskStatus = ""
	// TASK_STATUS_RUNNING indicates the task is in progress.
	TASK_STATUS_RUNNING = "RUNNING"
	// TASK_STATUS_SUCCESS indicates the task completed successfully.
	TASK_STATUS_SUCCESS = "SUCCESS"
	// TASK_STATUS_FAILURE indicates the task completed with failures.
	TASK_STATUS_FAILURE = "FAILURE"
	// TASK_STATUS_MISHAP indicates the task exited early with an error, died
	// while in progress, was manually canceled, expired while waiting on the
	// queue, or timed out before completing.
	TASK_STATUS_MISHAP = "MISHAP"
)

// Task describes a Swarming task generated from a TaskSpec, or a "fake" task
// that can not be executed on Swarming, but can be added to the DB and
// displayed as if it were a real TaskSpec.
type Task struct {
	// Commits are the commits which were tested in this Task. The list may
	// change due to backfilling/bisecting.
	Commits []string

	// Created is the creation timestamp.
	Created time.Time

	// Id is a generated unique identifier for this Task instance. Must be URL-safe.
	Id string

	// IsolatedOutput is the isolated hash of any outputs produced by this
	// Task. Filled in when the task is completed.
	IsolatedOutput string

	// Name is a human-friendly descriptive name for this Task. All Tasks
	// generated from the same TaskSpec have the same name.
	Name string

	// Repo is the repository of the commit at which this task ran.
	Repo string

	// Revision is the commit at which this task ran.
	Revision string

	// Status is the current task status, default TASK_STATUS_PENDING.
	Status TaskStatus

	// Swarming is information directly from Swarming, including the swarming task
	// ID. This field will not be set if the Task does not correspond to a
	// Swarming task.
	Swarming *swarming_api.SwarmingRpcsTaskRequestMetadata
}

// UpdateFromSwarming sets or initializes t from data in s.
//
// If empty, sets t.Id, t.Name, t.Repo, and t.Revision from s's tags named
// SWARMING_TAG_ID, SWARMING_TAG_NAME, SWARMING_TAG_REPO, and
// SWARMING_TAG_REVISION, and sets t.Created from s.TaskResult.CreatedTs. If
// these fields are non-empty, returns an error if they do not match.
//
// Always sets t.Status and t.IsolatedOutput based on s, and retains s as
// t.Swarming.
func (t *Task) UpdateFromSwarming(s *swarming_api.SwarmingRpcsTaskRequestMetadata) error {
	if s.TaskResult == nil {
		return fmt.Errorf("Missing TaskResult. %v", s)
	}
	tags, err := swarming.TagValues(s)
	if err != nil {
		return err
	}

	if sId, ok := tags[SWARMING_TAG_ID]; ok {
		if t.Id == "" {
			t.Id = sId
		} else if t.Id != sId {
			return fmt.Errorf("Id does not match for task %v and swarming task %v", t, s)
		}
	}

	if sName, ok := tags[SWARMING_TAG_NAME]; ok {
		if t.Name == "" {
			t.Name = sName
		} else if t.Name != sName {
			return fmt.Errorf("Name does not match for task %s. Was %s, now %s. %v", t.Id, t.Name, sName, t)
		}
	}

	if sRepo, ok := tags[SWARMING_TAG_REPO]; ok {
		if t.Repo == "" {
			t.Repo = sRepo
		} else if t.Repo != sRepo {
			return fmt.Errorf("Repo does not match for task %s. Was %s, now %s. %v", t.Id, t.Repo, sRepo, t)
		}
	}

	if sRevision, ok := tags[SWARMING_TAG_REVISION]; ok {
		if t.Revision == "" {
			t.Revision = sRevision
		} else if t.Revision != sRevision {
			return fmt.Errorf("Revision does not match for task %s. Was %s, now %s. %v", t.Id, t.Revision, sRevision, t)
		}
	}

	if sCreated, err := util.ParseTimeNs(s.TaskResult.CreatedTs); err == nil {
		if util.TimeIsZero(t.Created) {
			t.Created = sCreated
		} else if t.Created != sCreated {
			return fmt.Errorf("Creation time has changed for task %s. Was %s, now %s. %v", t.Id, t.Created, sCreated, t)
		}
	} else {
		return fmt.Errorf("Unable to parse task creation time for task %s. %v %v", t.Id, err, s)
	}

	switch s.TaskResult.State {
	case SWARMING_STATE_BOT_DIED, SWARMING_STATE_CANCELED, SWARMING_STATE_EXPIRED, SWARMING_STATE_TIMED_OUT:
		t.Status = TASK_STATUS_MISHAP
	case SWARMING_STATE_PENDING:
		t.Status = TASK_STATUS_PENDING
	case SWARMING_STATE_RUNNING:
		t.Status = TASK_STATUS_RUNNING
	case SWARMING_STATE_COMPLETED:
		if s.TaskResult.Failure {
			// TODO(benjaminwagner): Choose FAILURE or MISHAP depending on ExitCode?
			t.Status = TASK_STATUS_FAILURE
		} else {
			t.Status = TASK_STATUS_SUCCESS
		}
	default:
		return fmt.Errorf("Unknown Swarming State %v in %v", s.TaskResult.State, s)
	}

	if s.TaskResult.OutputsRef == nil {
		t.IsolatedOutput = ""
	} else {
		t.IsolatedOutput = s.TaskResult.OutputsRef.Isolated
	}

	t.Swarming = s

	return nil
}

func (t *Task) Finished() bool {
	return t.Status != TASK_STATUS_PENDING && t.Status != TASK_STATUS_RUNNING
}

func (t *Task) Success() bool {
	return t.Status == TASK_STATUS_SUCCESS
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
	// be empty or set with AssignId.
	PutTask(*Task) error

	// PutTasks inserts or updates the Tasks in the database. Each Task's Id field
	// must be empty or set with AssignId.
	PutTasks([]*Task) error

	// StartTrackingModifiedTasks initiates tracking of modified builds for
	// the current caller. Returns a unique ID which can be used by the caller
	// to retrieve builds which have been modified since the last query. The ID
	// expires after a period of inactivity.
	StartTrackingModifiedTasks() (string, error)
}
