package jobstore

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"strconv"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/pinpoint/go/sql/schema"
	"go.skia.org/infra/pinpoint/go/workflows"
	pinpointpb "go.skia.org/infra/pinpoint/proto/v1"
)

const (
	JobStatusIntial = "Pending"
	JobType         = "Pairwise"
)

type JobStore interface {
	// Store intial job parameters to database
	AddInitialJob(ctx context.Context, request *pinpointpb.SchedulePairwiseRequest, id string) error

	// UpdateJobStatus updates the status of a job.
	UpdateJobStatus(ctx context.Context, jobID string, status string, duration int64) error

	// Return all elements of a Job
	GetJob(ctx context.Context, jobID string) (*schema.JobSchema, error)

	// Store final statistics from pairwise job
	AddResults(ctx context.Context, jobID string, results map[string]*pinpointpb.PairwiseExecution_WilcoxonResult) error

	// Store error received from pairwise execution
	SetErrors(ctx context.Context, jobID string, err error) error

	// Store results from pairiwse commit runner including CAS references for builds and tests
	AddCommitRuns(ctx context.Context, jobID string, left, right *CommitRunData) error
}

// JobStore provides methods to access job data from the database.
type jobStoreImpl struct {
	db pool.Pool
}

// NewJobStore creates a new JobStore with the given database connection.
func NewJobStore(db pool.Pool) JobStore {
	return &jobStoreImpl{
		db: db,
	}
}

// CommitRunData contains the build and test run data for a commit.
type CommitRunData struct {
	// Build contains the build data for a commit.
	Build *workflows.Build
	// Runs contains the test run data for a commit.
	Runs []*workflows.TestRun
}

func (js *jobStoreImpl) AddInitialJob(ctx context.Context, request *pinpointpb.SchedulePairwiseRequest, id string) error {
	if request == nil {
		return skerr.Fmt("SchedulePairwiseRequest cannot be nil")
	}
	additionalParams := make(map[string]string)

	if request.StartCommit != nil && request.StartCommit.Main != nil && request.StartCommit.Main.GitHash != "" {
		additionalParams["start_commit_githash"] = request.StartCommit.Main.GitHash
	}
	if request.EndCommit != nil && request.EndCommit.Main != nil && request.EndCommit.Main.GitHash != "" {
		additionalParams["end_commit_githash"] = request.EndCommit.Main.GitHash
	}

	params := []struct {
		key   string
		value string
	}{
		{"story", request.Story},
		{"story_tags", request.StoryTags},
		{"initial_attempt_count", request.InitialAttemptCount},
		{"aggregation_method", request.AggregationMethod},
		{"target", request.Target},
		{"project", request.Project},
		{"bug_id", request.BugId},
		{"chart", request.Chart},
	}

	for _, p := range params {
		if p.value != "" {
			additionalParams[p.key] = p.value
		}
	}

	jobName := "default"
	submittedBy := "default"
	jobError := ""
	query := `
       INSERT INTO jobs (
           job_id,
           job_name,
           job_status,
           job_type,
           submitted_by,
           benchmark,
           bot_name,
           additional_request_parameters,
           error_message
       ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
       `
	var err error
	_, err = js.db.Exec(
		ctx,
		query,
		id,
		jobName,
		JobStatusIntial,
		JobType,
		submittedBy,
		request.Benchmark,
		request.Configuration,
		additionalParams,
		jobError,
	)

	if err != nil {
		return skerr.Fmt("failed to add job: %w", err)
	}

	return nil
}

func (js *jobStoreImpl) GetJob(ctx context.Context, jobID string) (*schema.JobSchema, error) {
	var job schema.JobSchema

	// Construct the SQL SELECT query to retrieve all columns for a given job_id.
	query := `SELECT
       job_id,
       job_name,
       job_status,
       job_type,
       createdat,
       submitted_by,
       benchmark,
       bot_name,
       additional_request_parameters,
       metric_summary,
       error_message
   FROM jobs
   WHERE job_id = $1`

	err := js.db.QueryRow(
		ctx,
		query,
		jobID,
	).Scan(
		&job.JobID,
		&job.JobName,
		&job.JobStatus,
		&job.JobType,
		&job.CreatedDate,
		&job.SubmittedBy,
		&job.Benchmark,
		&job.BotName,
		&job.AdditionalRequestParameters,
		&job.MetricSummary,
		&job.ErrorMessage,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, skerr.Fmt("job with ID %s not found", jobID)
		}
		return nil, skerr.Fmt("failed to query or scan job with ID %s: %w", jobID, err)
	}

	return &job, nil
}

func (js *jobStoreImpl) UpdateJobStatus(ctx context.Context, jobID string, status string, workflowDuration int64) error {

	// If not positive, then the job is not complete
	if workflowDuration <= 0 {
		query := `UPDATE jobs SET job_status = $2 WHERE job_id = $1`
		_, err := js.db.Exec(ctx, query, jobID, status)
		if err != nil {
			return skerr.Fmt("failed to update job status for job_id %s: %w", jobID, err)
		}
		return nil
	}

	// Update duration parameter
	tx, err := js.db.Begin(ctx)
	if err != nil {
		return skerr.Fmt("failed to begin transaction: %w", err)
	}

	params, err := js.getAdditionalParams(ctx, jobID, tx)
	if err != nil {
		return err
	}
	params["duration"] = strconv.FormatInt(workflowDuration, 10)

	query := `
       UPDATE jobs SET
       job_status = $2,
       additional_request_parameters = $3
       WHERE job_id = $1
   `
	if _, err = tx.Exec(ctx, query, jobID, status, params); err != nil {
		return skerr.Fmt("failed to update job status and duration for job_id %s: %w", jobID, err)
	}

	return tx.Commit(ctx)
}

func (js *jobStoreImpl) AddResults(ctx context.Context, jobID string, results map[string]*pinpointpb.PairwiseExecution_WilcoxonResult) error {
	query := `UPDATE jobs SET metric_summary = $2 WHERE job_id = $1`
	_, err := js.db.Exec(ctx, query, jobID, results)
	if err != nil {
		return skerr.Fmt("failed to add results for job_id %s: %w", jobID, err)
	}
	return nil
}

func (js *jobStoreImpl) SetErrors(ctx context.Context, jobID string, err error) error {
	query := `UPDATE jobs SET error_message = $2 WHERE job_id = $1`
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}
	_, dbErr := js.db.Exec(ctx, query, jobID, errMsg)
	if dbErr != nil {
		return skerr.Fmt("failed to add error for job_id %s: %w", jobID, dbErr)
	}
	return nil
}

func (js *jobStoreImpl) AddCommitRuns(ctx context.Context, jobID string, left, right *CommitRunData) error {
	// We want to pull additional_request_parameters, combine, then update
	tx, err := js.db.Begin(ctx)
	if err != nil {
		return skerr.Fmt("failed to begin transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	params, err := js.getAdditionalParams(ctx, jobID, tx)
	if err != nil {
		return err
	}
	commitRunsData := map[string]any{
		"left":  left,
		"right": right,
	}

	commitRunsJSON, err := json.Marshal(commitRunsData)
	if err != nil {
		return skerr.Fmt("failed to marshal commit runs to JSON for job_id %s: %w", jobID, err)
	}

	params["commit_runs"] = string(commitRunsJSON)

	query := `
       UPDATE jobs SET
       additional_request_parameters = $2
       WHERE job_id = $1
   `
	if _, err = tx.Exec(ctx, query, jobID, params); err != nil {
		return skerr.Fmt("failed to update commit runs for job_id %s: %w", jobID, err)
	}

	// Commit the transaction
	return tx.Commit(ctx)

}

// Helper function that retrieves the additional parameters for a given job ID.
func (js *jobStoreImpl) getAdditionalParams(ctx context.Context, jobID string, tx pgx.Tx,
) (map[string]string, error) {

	var existingParams []byte
	err := tx.QueryRow(ctx,
		`SELECT additional_request_parameters FROM jobs WHERE job_id = $1`,
		jobID).Scan(&existingParams)
	if err != nil {
		return nil, skerr.Fmt("failed to query existing params for job %s: %w", jobID, err)
	}

	var params map[string]string
	if err := json.NewDecoder(bytes.NewReader(existingParams)).Decode(&params); err != nil {
		return nil, skerr.Fmt("failed to unmarshal existing params for job %s: %w", jobID, err)
	}

	return params, nil
}
