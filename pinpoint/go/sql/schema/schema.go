package schema

import (
	"time"

	"go.skia.org/infra/pinpoint/go/workflows"
	pinpointpb "go.skia.org/infra/pinpoint/proto/v1"
)

// CommitRunData contains the build and test run data for a commit.
type CommitRunData struct {
	Build *workflows.Build     `json:"Build"`
	Runs  []*workflows.TestRun `json:"Runs"`
}

// CommitRuns contains the data for left and right commits.
type CommitRuns struct {
	Left  *CommitRunData `json:"left"`
	Right *CommitRunData `json:"right"`
}

// AdditionalRequestParametersSchema contains all additional parameters needed for a Job.
type AdditionalRequestParametersSchema struct {
	StartCommitGithash  string      `json:"start_commit_githash,omitempty"`
	EndCommitGithash    string      `json:"end_commit_githash,omitempty"`
	Story               string      `json:"story,omitempty"`
	StoryTags           string      `json:"story_tags,omitempty"`
	InitialAttemptCount string      `json:"initial_attempt_count,omitempty"`
	AggregationMethod   string      `json:"aggregation_method,omitempty"`
	Target              string      `json:"target,omitempty"`
	Project             string      `json:"project,omitempty"`
	BugId               string      `json:"bug_id,omitempty"`
	Chart               string      `json:"chart,omitempty"`
	Duration            string      `json:"duration,omitempty"`
	CommitRuns          *CommitRuns `json:"commit_runs,omitempty"`
}

// JobSchema represents the SQL schema of the Jobs table in Cloud Spanner.
type JobSchema struct {
	// JobID is a numerical identifier for a specific job.
	// It's the primary key for the table
	JobID string `sql:"job_id UUID PRIMARY KEY"`

	// JobName is a custom user-defined name for the job.
	JobName string `sql:"job_name STRING"`

	// JobStatus holds the current status of the job (e.g., "Pending", "Running", "Completed", "Failed").
	JobStatus string `sql:"job_status STRING"`

	// JobType specifies the type of job started (e.g., "Try", "Performance").
	JobType string `sql:"job_type STRING"`

	// SubmittedBy is the user identifier (e.g., an email account) who submitted the job.
	SubmittedBy string `sql:"submitted_by STRING"`

	// Benchmark is the name of the benchmark used in the request parameters.
	// Story and Story Tags will be stored in AdditionalRequestParameters
	Benchmark string `sql:"benchmark STRING"`

	// BotName identifies the specific machine used for the job
	// (Also known as bot_configuration or bot_configuration in legacy terminology).
	BotName string `sql:"bot_name STRING"`

	// AdditionalRequestParameters contains all additonal parameters needed for the Try Job,
	// stored as a JSONB object.
	AdditionalRequestParameters *AdditionalRequestParametersSchema `sql:"additional_request_parameters JSONB"`

	// MetricSummary holds the charts and results of a Try job, stored as a JSONB object.
	MetricSummary map[string]*pinpointpb.PairwiseExecution_WilcoxonResult `sql:"metric_summary JSONB"`

	// Error explaining why workflow failed
	ErrorMessage string `sql:"error_message STRING"`

	// The time in which the Pinpoint workflow began
	CreatedDate time.Time `sql:"createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP"`
}
