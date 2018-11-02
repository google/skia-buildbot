package bigtable

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/bigtable"
	multierror "github.com/hashicorp/go-multierror"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
)

const (
	COLUMN_FAMILY_JOB = "JOB"
	COLUMN_JOB        = "JOB"
)

var (
	// Fully-qualified BigTable column name.
	COLUMN_JOB_FULL = fmt.Sprintf("%s:%s", COLUMN_FAMILY_JOB, COLUMN_JOB)
)

// makeRowKeyJob returns a row key for the given Job. Assumes that the Job has
// a valid commit hash and repo name.
func makeRowKeyJob(job *db.Job, uuid string) string {
	return fmt.Sprintf("%s-%s-%s", ShortCommit(job.Revision), common.REPO_PROJECT_MAPPING[job.Repo], uuid)
}

// jobsTable interacts with the BigTable table for jobs.
type jobsTable struct {
	table *bigtable.Table
}

// GetJobById returns the job with the given ID.
func (t *jobsTable) GetJobById(ctx context.Context, id string) (*db.Job, error) {
	jobs, err := t.GetJobsWithPrefixes(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, db.ErrNotFound
	}
	if len(jobs) != 1 {
		return nil, fmt.Errorf("Expected exactly one job with ID %s but got %d", id, len(jobs))
	}
	return jobs[0], nil
}

// GetJobsWithPrefixes returns all jobs with row keys having the given prefixes.
func (t *jobsTable) GetJobsWithPrefixes(ctx context.Context, prefixes []string) ([]*db.Job, error) {
	rs := make([]bigtable.RowRange, 0, len(prefixes))
	for _, prefix := range prefixes {
		rs = append(rs, bigtable.PrefixRange(prefix))
	}
	var decodeErr error
	ctx, cancel := context.WithTimeout(ctx, QUERY_TIMEOUT)
	defer cancel()
	rv := make([]*db.Job, 0, len(prefixes))
	if err := t.table.ReadRows(ctx, bigtable.RowRangeList(rs), func(row bigtable.Row) bool {
		for _, ri := range row[COLUMN_FAMILY_JOB] {
			if ri.Column == COLUMN_JOB_FULL {
				var job *db.Job
				decodeErr = gob.NewDecoder(bytes.NewReader(ri.Value)).Decode(&job)
				if decodeErr != nil {
					return false
				}
				rv = append(rv, job)
				// We only store one job per row, so return here.
				return true
			}
		}
		return true
	}, bigtable.RowFilter(bigtable.LatestNFilter(1))); err != nil {
		return nil, fmt.Errorf("Failed to retrieve data from BigTable: %s", err)
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("Failed to gob-decode job: %s", decodeErr)
	}
	return rv, nil
}

// putJob inserts or updates a job in BigTable.
func (t *jobsTable) putJob(ctx context.Context, job *db.Job, isNew bool) (rvErr error) {
	// Validation.
	if job.Id == "" {
		return errors.New("Job.Id is required.")
	}
	if isNew && !util.TimeIsZero(job.DbModified) {
		return errors.New("InsertJob must only be called for new jobs, but this one has a DbModified timestamp.")
	} else if !isNew && util.TimeIsZero(job.DbModified) {
		// TODO(borenet): We should error out if we have a non-new job
		// without a DbModified timestamp, but since db.DB doesn't
		// distinguish between Insert and Update, we have to assume that
		// this is actually a new job and AssignId was called outside
		// of the adapter package.
		isNew = true
	}

	// Set the modification timestamp.
	nowTs := bigtable.Now()
	prevModified := job.DbModified
	if prevModified.After(nowTs.Time()) {
		// Ensure that the modification time increases even if we update
		// faster than the timestamp resolution.
		nowTs = bigtable.Time(prevModified.Add(time.Millisecond)).TruncateToMilliseconds()
	}
	job.DbModified = nowTs.Time()
	defer func() {
		if rvErr != nil {
			job.DbModified = prevModified
		}
	}()

	// Encode the Job.
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(job); err != nil {
		return fmt.Errorf("Failed to gob-encode job: %s", err)
	}

	// Create the mutation.
	mt := bigtable.NewMutation()
	mt.Set(COLUMN_FAMILY_JOB, COLUMN_JOB, nowTs, buf.Bytes())
	mt.Set(COLUMN_FAMILY_DB_MODIFIED, COLUMN_DB_MODIFIED, nowTs, []byte(job.DbModified.Format(DB_MODIFIED_FORMAT)))
	if isNew {
		// Only insert the job if there's no existing value.
		f := bigtable.ColumnFilter(COLUMN_DB_MODIFIED)
		mt = bigtable.NewCondMutation(f, nil, mt)
	} else {
		// Only insert the job if the existing value has the expected
		// timestamp; if it doesn't, someone else modified the job.
		f := bigtable.ChainFilters(
			bigtable.ColumnFilter(COLUMN_DB_MODIFIED),
			bigtable.ValueFilter(prevModified.Format(DB_MODIFIED_FORMAT)),
		)
		mt = bigtable.NewCondMutation(f, mt, nil)
	}

	// Apply the mutation.
	ctx, cancel := context.WithTimeout(ctx, INSERT_TIMEOUT)
	defer cancel()
	var matched bool
	if err := t.table.Apply(ctx, job.Id, mt, bigtable.GetCondMutationResult(&matched)); err != nil {
		return err
	}
	// We expect no match for new jobs, but do expect one for existing jobs.
	if matched == isNew {
		return db.ErrConcurrentUpdate
	}
	return nil
}

// InsertJob inserts a new job into BigTable.
func (t *jobsTable) InsertJob(ctx context.Context, job *db.Job) error {
	return t.putJob(ctx, job, true)
}

// UpdateJob updates a job in BigTable.
func (t *jobsTable) UpdateJob(ctx context.Context, job *db.Job) error {
	return t.putJob(ctx, job, false)
}

// putJobs inserts or updates the given jobs into BigTable.
func (t *jobsTable) putJobs(ctx context.Context, jobs []*db.Job, fn func(context.Context, *db.Job) error) error {
	var mtx sync.Mutex
	var errs error
	var wg sync.WaitGroup
	for _, job := range jobs {
		go func(job *db.Job) {
			if err := fn(ctx, job); err != nil {
				mtx.Lock()
				defer mtx.Unlock()
				errs = multierror.Append(errs, err)
			}
		}(job)
	}
	wg.Wait()
	return errs
}

// InsertJobs inserts the given jobs into BigTable.
func (t *jobsTable) InsertJobs(ctx context.Context, jobs []*db.Job) error {
	return t.putJobs(ctx, jobs, t.InsertJob)
}

// UpdateJobs updates the given jobs in BigTable.
func (t *jobsTable) UpdateJobs(ctx context.Context, jobs []*db.Job) error {
	return t.putJobs(ctx, jobs, t.UpdateJob)
}
