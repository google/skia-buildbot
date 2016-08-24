package db

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"reflect"
	"sync"
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
	TASK_STATUS_RUNNING TaskStatus = "RUNNING"
	// TASK_STATUS_SUCCESS indicates the task completed successfully.
	TASK_STATUS_SUCCESS TaskStatus = "SUCCESS"
	// TASK_STATUS_FAILURE indicates the task completed with failures.
	TASK_STATUS_FAILURE TaskStatus = "FAILURE"
	// TASK_STATUS_MISHAP indicates the task exited early with an error, died
	// while in progress, was manually canceled, expired while waiting on the
	// queue, or timed out before completing.
	TASK_STATUS_MISHAP TaskStatus = "MISHAP"
)

// Task describes a Swarming task generated from a TaskSpec, or a "fake" task
// that can not be executed on Swarming, but can be added to the DB and
// displayed as if it were a real TaskSpec.
//
// Task is stored as a GOB, so changes must maintain backwards compatibility.
// See gob package documentation for details, but generally:
//   - Ensure new fields can be initialized with their zero value.
//   - Do not change the type of any existing field.
//   - Leave removed fields commented out to ensure the field name is not
//     reused.
type Task struct {
	// Commits are the commits which were tested in this Task. The list may
	// change due to backfilling/bisecting.
	Commits []string

	// Created is the creation timestamp.
	Created time.Time

	// DbModified is the time of the last successful call to DB.PutTask/s for this
	// Task, or zero if the task is new. It is not related to the ModifiedTs time
	// of the associated Swarming task.
	DbModified time.Time

	// Finished is the time the task stopped running or expired from the queue, or
	//  zero if the task is pending or running.
	Finished time.Time

	// Id is a generated unique identifier for this Task instance. Must be
	// URL-safe.
	Id string

	// IsolatedOutput is the isolated hash of any outputs produced by this Task.
	// Filled in when the task is completed. We assume the isolate server is
	// isolate.ISOLATE_SERVER_URL and the namespace is isolate.DEFAULT_NAMESPACE.
	// This field will not be set if the Task does not correspond to a Swarming
	// task.
	IsolatedOutput string

	// Name is a human-friendly descriptive name for this Task. All Tasks
	// generated from the same TaskSpec have the same name.
	Name string

	// Repo is the repository of the commit at which this task ran.
	Repo string

	// Revision is the commit at which this task ran.
	Revision string

	// Started is the time the task started running, or zero if the task is
	// pending, or the same as Finished if the task never ran.
	Started time.Time

	// Status is the current task status, default TASK_STATUS_PENDING.
	Status TaskStatus

	// SwarmingTaskId is the Swarming task ID. This field will not be set if the
	// Task does not correspond to a Swarming task.
	SwarmingTaskId string
}

// UpdateFromSwarming sets or initializes t from data in s. If any changes were
// made to t, returns true.
//
// If empty, sets t.Id, t.Name, t.Repo, and t.Revision from s's tags named
// SWARMING_TAG_ID, SWARMING_TAG_NAME, SWARMING_TAG_REPO, and
// SWARMING_TAG_REVISION, sets t.Created from s.TaskResult.CreatedTs, and sets
// t.SwarmingTaskId from s.TaskId. If these fields are non-empty, returns an
// error if they do not match.
//
// Always sets t.Status, t.Started, t.Finished, and t.IsolatedOutput based on s.
func (orig *Task) UpdateFromSwarming(s *swarming_api.SwarmingRpcsTaskRequestMetadata) (bool, error) {
	if s.TaskResult == nil {
		return false, fmt.Errorf("Missing TaskResult. %v", s)
	}
	tags, err := swarming.TagValues(s)
	if err != nil {
		return false, err
	}

	copy := orig.Copy()
	if !reflect.DeepEqual(orig, copy) {
		glog.Fatalf("Task.Copy is broken; original and copy differ:\n%#v\n%#v", orig, copy)
	}

	// "Identity" fields stored in tags.
	checkOrSetFromTag := func(tagName string, field *string, fieldName string) error {
		if tagValue, ok := tags[tagName]; ok {
			if *field == "" {
				*field = tagValue
			} else if *field != tagValue {
				return fmt.Errorf("%s does not match for task %s. Was %s, now %s. %v %v", fieldName, orig.Id, *field, tagValue, orig, s)
			}
		}
		return nil
	}
	if err := checkOrSetFromTag(SWARMING_TAG_ID, &copy.Id, "Id"); err != nil {
		return false, err
	}
	if err := checkOrSetFromTag(SWARMING_TAG_NAME, &copy.Name, "Name"); err != nil {
		return false, err
	}
	if err := checkOrSetFromTag(SWARMING_TAG_REPO, &copy.Repo, "Repo"); err != nil {
		return false, err
	}
	if err := checkOrSetFromTag(SWARMING_TAG_REVISION, &copy.Revision, "Revision"); err != nil {
		return false, err
	}

	// CreatedTs should always be present.
	if sCreated, err := swarming.ParseTimestamp(s.TaskResult.CreatedTs); err == nil {
		if util.TimeIsZero(copy.Created) {
			copy.Created = sCreated
		} else if copy.Created != sCreated {
			return false, fmt.Errorf("Creation time has changed for task %s. Was %s, now %s. %v", orig.Id, orig.Created, sCreated, orig)
		}
	} else {
		return false, fmt.Errorf("Unable to parse task creation time for task %s. %v %v", orig.Id, err, orig)
	}

	// Swarming TaskId.
	if copy.SwarmingTaskId == "" {
		copy.SwarmingTaskId = s.TaskId
	} else if copy.SwarmingTaskId != s.TaskId {
		return false, fmt.Errorf("Swarming task ID does not match for task %s. Was %s, now %s. %v", orig.Id, orig.SwarmingTaskId, s.TaskId, orig)
	}

	// Status.
	switch s.TaskResult.State {
	case SWARMING_STATE_BOT_DIED, SWARMING_STATE_CANCELED, SWARMING_STATE_EXPIRED, SWARMING_STATE_TIMED_OUT:
		copy.Status = TASK_STATUS_MISHAP
	case SWARMING_STATE_PENDING:
		copy.Status = TASK_STATUS_PENDING
	case SWARMING_STATE_RUNNING:
		copy.Status = TASK_STATUS_RUNNING
	case SWARMING_STATE_COMPLETED:
		if s.TaskResult.Failure {
			// TODO(benjaminwagner): Choose FAILURE or MISHAP depending on ExitCode?
			copy.Status = TASK_STATUS_FAILURE
		} else {
			copy.Status = TASK_STATUS_SUCCESS
		}
	default:
		return false, fmt.Errorf("Unknown Swarming State %v in %v", s.TaskResult.State, s)
	}

	// Isolated output.
	if s.TaskResult.OutputsRef == nil {
		copy.IsolatedOutput = ""
	} else {
		copy.IsolatedOutput = s.TaskResult.OutputsRef.Isolated
	}

	// Timestamps.
	maybeUpdateTime := func(newTimeStr string, field *time.Time, name string) error {
		if newTimeStr == "" {
			return nil
		}
		newTime, err := swarming.ParseTimestamp(newTimeStr)
		if err != nil {
			return fmt.Errorf("Unable to parse %s for task %s. %v %v", name, orig.Id, err, s)
		}
		*field = newTime
		return nil
	}

	if err := maybeUpdateTime(s.TaskResult.StartedTs, &copy.Started, "StartedTs"); err != nil {
		return false, err
	}
	if err := maybeUpdateTime(s.TaskResult.CompletedTs, &copy.Finished, "CompletedTs"); err != nil {
		return false, err
	}
	if s.TaskResult.CompletedTs == "" && copy.Status == TASK_STATUS_MISHAP {
		if err := maybeUpdateTime(s.TaskResult.AbandonedTs, &copy.Finished, "AbandonedTs"); err != nil {
			return false, err
		}
	}
	if copy.Done() && util.TimeIsZero(copy.Started) {
		copy.Started = copy.Finished
	}

	// TODO(benjaminwagner): SwarmingRpcsTaskResult has a ModifiedTs field that we
	// could use to detect modifications. Unfortunately, it seems that while the
	// task is running, ModifiedTs gets updated every 30 seconds, regardless of
	// whether any other data actually changed. Maybe we could still use it for
	// pending or completed tasks.
	if !reflect.DeepEqual(orig, copy) {
		*orig = *copy
		return true, nil
	}
	return false, nil
}

var errNotModified = errors.New("Task not modified")

// UpdateDBFromSwarmingTask updates a task in db from data in s.
func UpdateDBFromSwarmingTask(db DB, s *swarming_api.SwarmingRpcsTaskRequestMetadata) error {
	id, err := swarming.GetTagValue(s, SWARMING_TAG_ID)
	if err != nil {
		return err
	}
	_, err = UpdateTaskWithRetries(db, id, func(task *Task) error {
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

func (t *Task) Done() bool {
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
type TaskDecoder struct {
	// input contains the incoming byte slices. Process() sends on this channel,
	// decode() receives from it, and Result() closes it.
	input chan []byte
	// output contains decoded Tasks. decode() sends on this channel, collect()
	// receives from it, and run() closes it when all decode() goroutines have
	// finished.
	output chan *Task
	// result contains the return value of Result(). collect() sends a single
	// value on this channel and closes it. Result() receives from it.
	result chan []*Task
	// errors contains the first error from any goroutine. It's a channel in case
	// multiple goroutines experience an error at the same time.
	errors chan error
}

const kNumDecoderGoroutines = 10

// init initializes d if it has not been initialized. May not be called concurrently.
func (d *TaskDecoder) init() {
	if d.input == nil {
		d.input = make(chan []byte, kNumDecoderGoroutines*2)
		d.output = make(chan *Task, kNumDecoderGoroutines)
		d.result = make(chan []*Task, 1)
		d.errors = make(chan error, kNumDecoderGoroutines)
		go d.run()
		go d.collect()
	}
}

// run starts the decode goroutines and closes d.output when they finish.
func (d *TaskDecoder) run() {
	// Start decoders.
	wg := sync.WaitGroup{}
	for i := 0; i < kNumDecoderGoroutines; i++ {
		wg.Add(1)
		go d.decode(&wg)
	}
	// Wait for decoders to exit.
	wg.Wait()
	// Drain d.input in the case that errors were encountered, to avoid deadlock.
	for _ = range d.input {
	}
	close(d.output)
}

// decode receives from d.input and sends to d.output until d.input is closed or
// d.errors is non-empty. Decrements wg when done.
func (d *TaskDecoder) decode(wg *sync.WaitGroup) {
	for b := range d.input {
		var t Task
		if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&t); err != nil {
			d.errors <- err
			break
		}
		d.output <- &t
		if len(d.errors) > 0 {
			break
		}
	}
	wg.Done()
}

// collect receives from d.output until it is closed, then sends on d.result.
func (d *TaskDecoder) collect() {
	result := []*Task{}
	for t := range d.output {
		result = append(result, t)
	}
	d.result <- result
	close(d.result)
}

// Process decodes the byte slice into a Task and includes it in Result() (in
// arbitrary order). Returns false if Result is certain to return an error.
// Caller must ensure b does not change until after Result() returns.
func (d *TaskDecoder) Process(b []byte) bool {
	d.init()
	d.input <- b
	return len(d.errors) == 0
}

// Result returns all decoded Tasks provided to Process (in arbitrary order), or
// any error encountered.
func (d *TaskDecoder) Result() ([]*Task, error) {
	// Allow TaskDecoder to be used without initialization.
	if d.result == nil {
		return []*Task{}, nil
	}
	close(d.input)
	select {
	case err := <-d.errors:
		return nil, err
	case result := <-d.result:
		return result, nil
	}
}
