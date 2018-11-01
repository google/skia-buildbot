package bigtable

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/task_scheduler/go/db"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

/*
	Schema:
	- Jobs table:
		- Row key is commit hash + short repo name + sequence number.
		- Each row contains one job.
		- Job status is a column.
		- Job data is stored as a gob-encoded blob of bytes.
	- Unfinished jobs table
		- Row keys is job ID.
		- Rows are deleted as jobs finish.
		- All queries are full table scans.
	- Tasks table:
		- Row key is commit hash + short repo name + sequence number.
		- The first column family contains the task data at the time of
		  creation.
		- Subsequent columns contain all updates to the task, indicating
		  deltas to apply.
	- Task comments table:
		- Row key is task ID (ie. commit hash + short repo name +
		  sequence number) plus sequence number or timestamp.
	- Task spec comments table:
		- Row key is task spec name + sequence number or timestamp.
		- All queries are full table scans.
		- We need to periodically clean up comments for no-longer-used
		  task specs.
	- Commit comments table:
		- Row key is commit hash + short repo name + sequence number.
	- ID table:
		- Contains a single row for each of: jobs, tasks, task comments,
		  task spec comments, and commit comments.
		- Each row contains a monotonically increasing sequence number
		  which is used in creating IDs.
*/

const (
	// Timeouts for BigTable operations.
	INSERT_TIMEOUT = 30 * time.Second
	QUERY_TIMEOUT  = 5 * time.Second

	// BigTable tables.
	TABLE_JOBS               = "jobs"
	TABLE_UNFINISHED_JOBS    = "unfinished-jobs"
	TABLE_TASKS              = "tasks"
	TABLE_TASK_COMMENTS      = "task-comments"
	TABLE_TASK_SPEC_COMMENTS = "task-spec-comments"
	TABLE_COMMIT_COMMENTS    = "commit-comments"
	TABLE_IDS                = "ids"
)

func shortCommit(hash string) string {
	return hash[:7]
}

// combineErrors combines errors from BigTable bulk mutations.
func combineErrors(errs []error, err error) error {
	if err != nil {
		return fmt.Errorf("Failed to apply bulk mutation: %s", err)
	}
	if len(errs) > 0 {
		rv := "Individual mutation(s) failed:"
		for _, err := range errs {
			rv += "\n" + err.Error()
		}
		return errors.New(rv)
	}
	return nil
}

// bigtableDB is an interface to BigTable which performs all of the required
// operations for tasks, jobs, and comments.
type bigtableDB struct {
	client *bigtable.Client
	*jobsTable
	*unfinishedJobsTable
	*tasksTable
	// *taskCommentsTable
	// *taskSpecCommentsTable
	// *commitCommentsTable
	*idTable
}

// NewBigTableDB returns a bigtableDB instance.
func NewBigTableDB(ctx context.Context, project, instance string, ts oauth2.TokenSource) (*bigtableDB, error) {
	client, err := bigtable.NewClient(ctx, project, instance, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("Failed to create BigTable client: %s", err)
	}
	return &bigtableDB{
		jobsTable:           &jobsTable{client.Open(TABLE_JOBS)},
		unfinishedJobsTable: &unfinishedJobsTable{client.Open(TABLE_UNFINISHED_JOBS)},
		tasksTable:          &tasksTable{client.Open(TABLE_TASKS)},
		// taskCommentsTable: &taskCommentsTable{ client.Open(TABLE_TASK_COMMENTS) },
		// taskSpecCommentsTable: &taskSpecCommentsTable{ client.Open(TABLE_TASK_SPEC_COMMENTS) },
		// commitCommentsTable: &commitCommentsTable{ client.Open(TABLE_COMMIT_COMMENTS) },
		idTable: &idTable{client.Open(TABLE_IDS)},
	}, nil
}

// Close the DB.
func (d *bigtableDB) Close() error {
	return d.client.Close()
}

// GetUnfinishedJobs returns the set of all not-yet-finished jobs.
func (d *bigtableDB) GetUnfinishedJobs(ctx context.Context) ([]*db.Job, error) {
	ids, err := d.unfinishedJobsTable.GetUnfinishedJobIDs(ctx)
	if err != nil {
		return nil, err
	}
	return d.jobsTable.GetJobsWithPrefixes(ctx, ids)
}
