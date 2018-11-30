package types

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
)

const (
	// JOB_STATUS_IN_PROGRESS indicates that one or more of the Job's
	// Task dependencies has not yet been satisfied.
	JOB_STATUS_IN_PROGRESS JobStatus = ""

	// JOB_STATUS_SUCCESS indicates that all of the Job's Task dependencies
	// completed successfully.
	JOB_STATUS_SUCCESS JobStatus = "SUCCESS"

	// JOB_STATUS_FAILURE indicates that one or more of the Job's Task
	// dependencies failed.
	JOB_STATUS_FAILURE JobStatus = "FAILURE"

	// JOB_STATUS_MISHAP indicates that one or more of the Job's Task
	// dependencies exited early with an error, died while in progress, was
	// manually canceled, expired while waiting on the queue, or timed out
	// before completing.
	JOB_STATUS_MISHAP JobStatus = "MISHAP"

	// JOB_STATUS_CANCELED indicates that the Job has been canceled.
	JOB_STATUS_CANCELED JobStatus = "CANCELED"

	// JOB_URL_TMPL is a template for Job URLs.
	JOB_URL_TMPL = "%s/job/%s"

	// DEFAULT_MAX_TASK_ATTEMPTS is the maximum number of attempts we'll
	// make of each TaskSpec in a Job.
	DEFAULT_MAX_TASK_ATTEMPTS = 2
)

var (
	JOB_STATUS_BADNESS = map[JobStatus]int{
		JOB_STATUS_SUCCESS:     0,
		JOB_STATUS_IN_PROGRESS: 1,
		JOB_STATUS_CANCELED:    2,
		JOB_STATUS_FAILURE:     3,
		JOB_STATUS_MISHAP:      4,
	}
	VALID_JOB_STATUSES = []JobStatus{
		JOB_STATUS_IN_PROGRESS,
		JOB_STATUS_SUCCESS,
		JOB_STATUS_FAILURE,
		JOB_STATUS_MISHAP,
		JOB_STATUS_CANCELED,
	}
)

// JobStatus represents the current status of a Job. A JobStatus other than
// JOB_STATUS_IN_PROGRESS is final; we do not retry Jobs, only their component
// Tasks.
type JobStatus string

// WorseThan returns true iff this JobStatus is worse than the given JobStatus.
func (s JobStatus) WorseThan(other JobStatus) bool {
	return JOB_STATUS_BADNESS[s] > JOB_STATUS_BADNESS[other]
}

// WorseJobStatus returns the worse of the two JobStatus.
func WorseJobStatus(a, b JobStatus) JobStatus {
	if a.WorseThan(b) {
		return a
	}
	return b
}

// JobStatusFromTaskStatus returns a JobStatus based on a TaskStatus.
func JobStatusFromTaskStatus(s TaskStatus) JobStatus {
	switch s {
	case TASK_STATUS_SUCCESS:
		return JOB_STATUS_SUCCESS
	case TASK_STATUS_FAILURE:
		return JOB_STATUS_FAILURE
	case TASK_STATUS_MISHAP:
		return JOB_STATUS_MISHAP
	}
	return JOB_STATUS_IN_PROGRESS
}

// Job represents a set of Tasks which are executed as part of a larger effort.
//
// Job is stored as a GOB, so changes must maintain backwards compatibility.
// See gob package documentation for details, but generally:
//   - Ensure new fields can be initialized with their zero value.
//   - Do not change the type of any existing field.
//   - Leave removed fields commented out to ensure the field name is not
//     reused.
//   - Add any new fields to the Copy() method.
type Job struct {
	// BuildbucketBuildId is the ID of the Buildbucket build with which this
	// Job is associated, if one exists.
	BuildbucketBuildId int64 `json:"buildbucketBuildId"`

	// BuildbucketLeaseKey is the lease key for running a Buildbucket build.
	// TODO(borenet): Maybe this doesn't belong in the DB.
	BuildbucketLeaseKey int64 `json:"buildbucketLeaseKey"`

	// Created is the creation timestamp. This property should never change
	// for a given Job instance.
	Created time.Time `json:"created"`

	// DbModified is the time of the last successful call to JobDB.PutJob/s
	// for this Job, or zero if the job is new.
	DbModified time.Time `json:"dbModified"`

	// Dependencies maps out the DAG of TaskSpec names upon which this Job
	// depends. Keys are TaskSpec names and values are slices of TaskSpec
	// names indicating which TaskSpecs that TaskSpec depends on. This
	// property should never change for a given Job instance.
	Dependencies map[string][]string `json:"dependencies"`

	// Finished is the time at which all of the Job's dependencies finished,
	// successfully or not.
	Finished time.Time `json:"finished"`

	// Id is a unique identifier for the Job. This property should never
	// change for a given Job instance, after its initial insertion into the
	// DB.
	Id string `json:"id"`

	// IsForce indicates whether this is a manually-triggered Job, as
	// opposed to a normally scheduled one, or a try job.
	IsForce bool `json:"isForce"`

	// Name is a human-friendly descriptive name for the Job. All Jobs
	// generated from the same JobSpec have the same name. This property
	// should never change for a given Job instance.
	Name string `json:"name"`

	// Priority is an indicator of the relative priority of this Job.
	Priority float64 `json:"priority"`

	// RepoState is the current state of the repository for this Job.
	RepoState

	// Status is the current Job status, default JOB_STATUS_IN_PROGRESS.
	Status JobStatus `json:"status"`

	// Tasks are the Task instances which satisfied the dependencies of
	// the Job. Keys are TaskSpec names and values are slices of TaskSummary
	// instances describing the Tasks.
	Tasks map[string][]*TaskSummary `json:"tasks"`
}

// Copy returns a copy of the Job.
func (j *Job) Copy() *Job {
	var deps map[string][]string
	if j.Dependencies != nil {
		deps = make(map[string][]string, len(j.Dependencies))
		for k, v := range j.Dependencies {
			cpy := make([]string, len(v))
			copy(cpy, v)
			deps[k] = cpy
		}
	}
	var tasks map[string][]*TaskSummary
	if j.Tasks != nil {
		tasks = make(map[string][]*TaskSummary, len(j.Tasks))
		for k, v := range j.Tasks {
			cpy := make([]*TaskSummary, 0, len(v))
			for _, t := range v {
				cpy = append(cpy, t.Copy())
			}
			tasks[k] = cpy
		}
	}
	return &Job{
		BuildbucketBuildId:  j.BuildbucketBuildId,
		BuildbucketLeaseKey: j.BuildbucketLeaseKey,
		Created:             j.Created,
		DbModified:          j.DbModified,
		Dependencies:        deps,
		Finished:            j.Finished,
		Id:                  j.Id,
		IsForce:             j.IsForce,
		Name:                j.Name,
		Priority:            j.Priority,
		RepoState:           j.RepoState.Copy(),
		Status:              j.Status,
		Tasks:               tasks,
	}
}

func (j *Job) Done() bool {
	return j.Status != JOB_STATUS_IN_PROGRESS
}

// MakeTaskKey returns a TaskKey for the given Task name.
func (j *Job) MakeTaskKey(taskName string) TaskKey {
	rv := TaskKey{
		RepoState: j.RepoState.Copy(),
		Name:      taskName,
	}
	if j.IsForce {
		rv.ForcedJobId = j.Id
	}
	return rv
}

// URL returns a URL for the Job.
func (j *Job) URL(taskSchedulerHost string) string {
	return fmt.Sprintf(JOB_URL_TMPL, taskSchedulerHost, j.Id)
}

// TraverseDependencies traces the dependency graph of the Job, calling the
// given function for each dependency. Only calls the function on task specs
// for whose dependencies the function has already been called. If the passed-in
// function returns an error, iteration stops and TraverseDependencies returns
// the same error.
func (j *Job) TraverseDependencies(fn func(string) error) error {
	done := make(map[string]bool, len(j.Dependencies))
	var visit func(string) error
	visit = func(name string) error {
		for _, d := range j.Dependencies[name] {
			if !done[d] {
				if err := visit(d); err != nil {
					return err
				}
			}
		}
		done[name] = true
		return fn(name)
	}
	for d := range j.Dependencies {
		if !done[d] {
			if err := visit(d); err != nil {
				return err
			}
		}
	}
	return nil
}

// DeriveStatus derives a JobStatus based on the TaskStatuses in the Job's
// dependency tree.
func (j *Job) DeriveStatus() JobStatus {
	if len(j.Tasks) == 0 {
		return JOB_STATUS_IN_PROGRESS
	}
	worstStatus := JOB_STATUS_SUCCESS
	if err := j.TraverseDependencies(func(name string) error {
		tasks, ok := j.Tasks[name]
		if !ok || len(tasks) == 0 {
			worstStatus = WorseJobStatus(worstStatus, JOB_STATUS_IN_PROGRESS)
			return nil
		}

		// We may have more than one Task for this spec, due to
		// retrying of failed Tasks. We should not return a "failed"
		// result if we still have retry attempts remaining or if we've
		// already retried and succeeded.
		maxAttempts := tasks[0].MaxAttempts
		if maxAttempts == 0 {
			maxAttempts = DEFAULT_MAX_TASK_ATTEMPTS
		}
		canRetry := len(tasks) < maxAttempts
		bestStatus := JOB_STATUS_MISHAP
		for _, t := range tasks {
			status := JobStatusFromTaskStatus(t.Status)
			if bestStatus.WorseThan(status) {
				bestStatus = status
			}
		}
		if bestStatus == JOB_STATUS_SUCCESS || bestStatus == JOB_STATUS_IN_PROGRESS {
			worstStatus = WorseJobStatus(worstStatus, bestStatus)
		} else if canRetry {
			worstStatus = WorseJobStatus(worstStatus, JOB_STATUS_IN_PROGRESS)
		} else {
			worstStatus = WorseJobStatus(worstStatus, bestStatus)
		}
		return nil
	}); err != nil {
		// Our inner function doesn't return errors, and
		// TraverseDependencies doesn't return errors of its own, so
		// this should be safe.
		sklog.Errorf("Got error traversing Job dependencies: %s", err)
		return JOB_STATUS_IN_PROGRESS
	}
	return worstStatus
}

// JobSlice implements sort.Interface. To sort jobs []*Job, use
// sort.Sort(JobSlice(jobs)).
type JobSlice []*Job

func (s JobSlice) Len() int { return len(s) }

func (s JobSlice) Less(i, j int) bool {
	return s[i].Created.Before(s[j].Created)
}

func (s JobSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// JobEncoder encodes Jobs into bytes via GOB encoding. Not safe for
// concurrent use.
// TODO(benjaminwagner): Encode in parallel.
type JobEncoder struct {
	err    error
	jobs   []*Job
	result [][]byte
}

// Process encodes the Job into a byte slice that will be returned from Next()
// (in arbitrary order). Returns false if Next is certain to return an error.
// Caller must ensure j does not change until after the first call to Next().
// May not be called after calling Next().
func (e *JobEncoder) Process(j *Job) bool {
	if e.err != nil {
		return false
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(j); err != nil {
		e.err = err
		e.jobs = nil
		e.result = nil
		return false
	}
	e.jobs = append(e.jobs, j)
	e.result = append(e.result, buf.Bytes())
	return true
}

// Next returns one of the Jobs provided to Process (in arbitrary order) and
// its serialized bytes. If any jobs remain, returns the job, the serialized
// bytes, nil. If all jobs have been returned, returns nil, nil, nil. If an
// error is encountered, returns nil, nil, error.
func (e *JobEncoder) Next() (*Job, []byte, error) {
	if e.err != nil {
		return nil, nil, e.err
	}
	if len(e.jobs) == 0 {
		return nil, nil, nil
	}
	j := e.jobs[0]
	e.jobs = e.jobs[1:]
	serialized := e.result[0]
	e.result = e.result[1:]
	return j, serialized, nil
}

// JobDecoder decodes bytes into Jobs via GOB decoding. Not safe for
// concurrent use.
type JobDecoder struct {
	// input contains the incoming byte slices. Process() sends on this channel,
	// decode() receives from it, and Result() closes it.
	input chan []byte
	// output contains decoded Jobs. decode() sends on this channel, collect()
	// receives from it, and run() closes it when all decode() goroutines have
	// finished.
	output chan *Job
	// result contains the return value of Result(). collect() sends a single
	// value on this channel and closes it. Result() receives from it.
	result chan []*Job
	// errors contains the first error from any goroutine. It's a channel in case
	// multiple goroutines experience an error at the same time.
	errors chan error
}

// init initializes d if it has not been initialized. May not be called concurrently.
func (d *JobDecoder) init() {
	if d.input == nil {
		d.input = make(chan []byte, kNumDecoderGoroutines*2)
		d.output = make(chan *Job, kNumDecoderGoroutines)
		d.result = make(chan []*Job, 1)
		d.errors = make(chan error, kNumDecoderGoroutines)
		go d.run()
		go d.collect()
	}
}

// run starts the decode goroutines and closes d.output when they finish.
func (d *JobDecoder) run() {
	// Start decoders.
	wg := sync.WaitGroup{}
	for i := 0; i < kNumDecoderGoroutines; i++ {
		wg.Add(1)
		go d.decode(&wg)
	}
	// Wait for decoders to exit.
	wg.Wait()
	// Drain d.input in the case that errors were encountered, to avoid deadlock.
	for range d.input {
	}
	close(d.output)
}

// decode receives from d.input and sends to d.output until d.input is closed or
// d.errors is non-empty. Decrements wg when done.
func (d *JobDecoder) decode(wg *sync.WaitGroup) {
	for b := range d.input {
		var j Job
		if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&j); err != nil {
			d.errors <- err
			break
		}
		d.output <- &j
		if len(d.errors) > 0 {
			break
		}
	}
	wg.Done()
}

// collect receives from d.output until it is closed, then sends on d.result.
func (d *JobDecoder) collect() {
	result := []*Job{}
	for j := range d.output {
		result = append(result, j)
	}
	d.result <- result
	close(d.result)
}

// Process decodes the byte slice into a Job and includes it in Result() (in
// arbitrary order). Returns false if Result is certain to return an error.
// Caller must ensure b does not change until after Result() returns.
func (d *JobDecoder) Process(b []byte) bool {
	d.init()
	d.input <- b
	return len(d.errors) == 0
}

// Result returns all decoded Jobs provided to Process (in arbitrary order), or
// any error encountered.
func (d *JobDecoder) Result() ([]*Job, error) {
	// Allow JobDecoder to be used without initialization.
	if d.result == nil {
		return []*Job{}, nil
	}
	close(d.input)
	select {
	case err := <-d.errors:
		return nil, err
	case result := <-d.result:
		return result, nil
	}
}
