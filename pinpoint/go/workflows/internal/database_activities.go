package internal

import (
	"context"
	"errors"

	"go.skia.org/infra/go/skerr"
	jobstore "go.skia.org/infra/pinpoint/go/sql/jobs_store"
	pinpointpb "go.skia.org/infra/pinpoint/proto/v1"
)

const (
	AddInitialJob   = "add-initial-job-activity"
	UpdateJobStatus = "update-job-status-activity"
	SetErrors       = "set-errors-activity"
	AddResults      = "add-results-activity"
	AddCommitRuns   = "add-commit-runs-activity"
)

// JobStoreActivities holds dependencies for job store activities.
// The activities in this struct must be registered with a Temporal worker.
type JobStoreActivities struct {
	js jobstore.JobStore
}

// Store intial job parameters to database.
func (a *JobStoreActivities) AddInitialJob(ctx context.Context, request *pinpointpb.SchedulePairwiseRequest, id string) error {
	return skerr.Wrap(a.js.AddInitialJob(ctx, request, id))
}

// UpdateJobStatus updates the status of a job.
func (a *JobStoreActivities) UpdateJobStatus(ctx context.Context, jobID string, status string, duration int64) error {
	return skerr.Wrap(a.js.UpdateJobStatus(ctx, jobID, status, duration))
}

// Store error received from pairwise execution.
func (a *JobStoreActivities) SetErrors(ctx context.Context, jobID string, errMsg string) error {
	var err error
	if errMsg != "" {
		err = errors.New(errMsg)
	}
	return skerr.Wrap(a.js.SetErrors(ctx, jobID, err))
}

// Store final statistics from pairwise job.
func (a *JobStoreActivities) AddResults(ctx context.Context, jobID string, results map[string]*pinpointpb.PairwiseExecution_WilcoxonResult) error {
	return skerr.Wrap(a.js.AddResults(ctx, jobID, results))
}

// Store results from pairiwse commit runner including CAS references for builds and tests.
func (a *JobStoreActivities) AddCommitRuns(ctx context.Context, jobID string, left, right *jobstore.CommitRunData) error {
	return skerr.Wrap(a.js.AddCommitRuns(ctx, jobID, left, right))
}

// NewJobStoreActivities creates a new JobStoreActivities.
func NewJobStoreActivities(js jobstore.JobStore) *JobStoreActivities {
	return &JobStoreActivities{js: js}
}
