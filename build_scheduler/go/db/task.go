package db

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"

	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/skia-dev/glog"
)

const (
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

	// Id is a generated unique identifier for this Task instance. Must be
	// URL-safe.
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
			// TODO(benjaminwagner): Choose FAILURE or MISHAP depending on
			// ExitCode?
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

// TaskSlice implements sort.Interface. To sort tasks []*Task, use
// sort.Sort(TaskSlice(tasks)).
type TaskSlice []*Task

func (s TaskSlice) Len() int { return len(s) }

func (s TaskSlice) Less(i, j int) bool {
	return s[i].Created.Before(s[j].Created)
}

func (s TaskSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// TaskEncoder encodes Tasks into bytes via GOB encoding. Not safe for
// concurrent use.
// TODO(benjaminwagner): Encode in parallel.
type TaskEncoder struct {
	err    error
	tasks  []*Task
	result [][]byte
}

// Process encodes the Task into a byte slice that will be returned from Next()
// (in arbitrary order). Returns false if Next is certain to return an error.
// Caller must ensure t does not change until after the first call to Next().
// May not be called after calling Next().
func (e *TaskEncoder) Process(t *Task) bool {
	if e.err != nil {
		return false
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(t); err != nil {
		e.err = err
		e.tasks = nil
		e.result = nil
		return false
	}
	e.tasks = append(e.tasks, t)
	e.result = append(e.result, buf.Bytes())
	return true
}

// Next returns one of the Tasks provided to Process (in arbitrary order) and
// its serialized bytes. If any tasks remain, returns the task, the serialized
// bytes, nil. If all tasks have been returned, returns nil, nil, nil. If an
// error is encountered, returns nil, nil, error.
func (e *TaskEncoder) Next() (*Task, []byte, error) {
	if e.err != nil {
		return nil, nil, e.err
	}
	if len(e.tasks) == 0 {
		return nil, nil, nil
	}
	t := e.tasks[0]
	e.tasks = e.tasks[1:]
	serialized := e.result[0]
	e.result = e.result[1:]
	return t, serialized, nil
}

// TaskDecoder decodes bytes into Tasks via GOB decoding. Not safe for
// concurrent use.
// TODO(benjaminwagner): Decode in parallel.
type TaskDecoder struct {
	err    error
	result []*Task
}

// Process decodes the byte slice into a Task and includes it in Result() (in
// arbitrary order). Returns false if Result is certain to return an error.
// Caller must ensure b does not change until after Result() returns.
func (d *TaskDecoder) Process(b []byte) bool {
	if d.err != nil {
		return false
	}
	var t Task
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&t); err != nil {
		d.err = err
		d.result = nil
		return false
	}
	d.result = append(d.result, &t)
	return true
}

// Result returns all decoded Tasks provided to Process (in arbitrary order), or
// any error encountered.
func (d *TaskDecoder) Result() ([]*Task, error) {
	// Allow TaskDecoder to be used without initialization.
	if d.err == nil && d.result == nil {
		return []*Task{}, nil
	}
	return d.result, d.err
}
