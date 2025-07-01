package schema

import (
	"time"

	pinpointpb "go.skia.org/infra/pinpoint/proto/v1"
)

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
	// Info stored: Interation Count, Aggregation Method, Swarming Statuses, CAS References (Build & Tests), Story,
	// Story Tags, Commit Hashes (Base & End), Workflow Duration
	AdditionalRequestParameters map[string]string `sql:"additional_request_parameters JSONB"`

	// MetricSummary holds the charts and results of a Try job, stored as a JSONB object.
	MetricSummary map[string]*pinpointpb.PairwiseExecution_WilcoxonResult `sql:"metric_summary JSONB"`

	// Error explaining why workflow failed
	ErrorMessage string `sql:"error_message STRING"`

	// The time in which the Pinpoint workflow began
	CreatedDate time.Time `sql:"createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP"`
}
