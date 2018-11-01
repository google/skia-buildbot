package bigtable

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/common"
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
	return fmt.Sprintf("%s-%s-%s", shortCommit(job.Revision), common.REPO_PROJECT_MAPPING[job.Repo], uuid)
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

// mutationForJob returns the row key and mutation for the given job.
func mutationForJob(job *db.Job, ts time.Time) (string, *bigtable.Mutation, error) {
	// Encode the Job.
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(job); err != nil {
		return "", nil, fmt.Errorf("Failed to gob-encode job: %s", err)
	}
	// Create the mutation.
	mt := bigtable.NewMutation()
	mt.Set(COLUMN_FAMILY_JOB, COLUMN_JOB, bigtable.Time(ts), buf.Bytes())
	rk := job.Id
	if rk == "" {
		return "", nil, fmt.Errorf("Job has no ID")
	}
	return rk, mt, nil
}

// PutJob inserts or updates the given job in BigTable.
func (t *jobsTable) PutJob(ctx context.Context, job *db.Job, ts time.Time) (rvErr error) {
	prevModified := job.DbModified
	job.DbModified = ts
	defer func() {
		if rvErr != nil {
			job.DbModified = prevModified
		}
	}()

	rk, mt, err := mutationForJob(job, ts)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, INSERT_TIMEOUT)
	defer cancel()
	return t.table.Apply(ctx, rk, mt)
}

// PutJobs inserts or updates the given jobs in BigTable.
func (t *jobsTable) PutJobs(ctx context.Context, jobs []*db.Job, ts time.Time) (rvErr error) {
	prevModified := make([]time.Time, 0, len(jobs))
	for _, job := range jobs {
		prevModified = append(prevModified, job.DbModified)
		job.DbModified = ts
	}
	defer func() {
		// TODO(borenet): It's possible that some of the jobs were
		// successfully updated. We should look at the individual errors
		// returned by ApplyBulk.
		if rvErr != nil {
			for idx, job := range jobs {
				job.DbModified = prevModified[idx]
			}
		}
	}()

	rks := make([]string, 0, len(jobs))
	mts := make([]*bigtable.Mutation, 0, len(jobs))
	for _, job := range jobs {
		rk, mt, err := mutationForJob(job, ts)
		if err != nil {
			return err
		}
		rks = append(rks, rk)
		mts = append(mts, mt)
	}
	ctx, cancel := context.WithTimeout(ctx, INSERT_TIMEOUT)
	defer cancel()
	return combineErrors(t.table.ApplyBulk(ctx, rks, mts))
}
