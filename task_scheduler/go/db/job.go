package db

import "time"

const (
	// JOB_STATUS_IN_PROGRESS indicates that one or more of the Job's
	// Task dependencies has not yet been satisfied.
	JOB_STATUS_IN_PROGRESS = ""

	// JOB_STATUS_SUCCESS indicates that all of the Job's Task dependencies
	// completed successfully.
	JOB_STATUS_SUCCESS = "SUCCESS"

	// JOB_STATUS_FAILURE indicates that one or more of the Job's Task
	// dependencies failed.
	JOB_STATUS_FAILURE = "FAILURE"

	// JOB_STATUS_MISHAP indicates that one or more of the Job's Task
	// dependencies exited early with an error, died while in progress, was
	// manually canceled, expired while waiting on the queue, or timed out
	// before completing.
	JOB_STATUS_MISHAP = "MISHAP"
)

// JobStatus represents the current status of a Job. A JobStatus other than
// JOB_STATUS_IN_PROGRESS is final; we do not retry Jobs, only their component
// Tasks.
type JobStatus string

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
	// Created is the creation timestamp.
	Created time.Time

	// DbModified is the time of the last successful call to JobDB.PutJob/s
	// for this Job, or zero if the job is new.
	DbModified time.Time

	// Dependencies are the names of the TaskSpecs on which this Job
	// depends.
	Dependencies []string

	// Finished is the time at which all of the Job's dependencies finished,
	// successfully or not.
	Finished time.Time

	// Id is a unique identifier for the Job.
	Id string

	// Name is a human-friendly descriptive name for the Job. All Jobs
	// generated from the same JobSpec have the same name.
	Name string

	// Repo is the repository of the commit at which this Job ran.
	Repo string

	// Revision is the commit at which this Job ran.
	Revision string

	// Status is the current Job status, default JOB_STATUS_IN_PROGRESS.
	Status JobStatus
}

// Copy returns a copy of the Job.
func (j *Job) Copy() *Job {
	var deps []string
	if j.Dependencies != nil {
		deps = make([]string, len(j.Dependencies))
		copy(deps, j.Dependencies)
	}
	return &Job{
		Created:      j.Created,
		DbModified:   j.DbModified,
		Dependencies: deps,
		Finished:     j.Finished,
		Id:           j.Id,
		Name:         j.Name,
		Repo:         j.Repo,
		Revision:     j.Revision,
		Status:       j.Status,
	}
}
