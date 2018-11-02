package bigtable

import (
	"context"
	"time"

	"cloud.google.com/go/bigtable"
)

// unfinishedJobsTable contains IDs for all unfinished jobs.
type unfinishedJobsTable struct {
	table *bigtable.Table
}

// GetUnfinishedJobIDs returns the IDs for all unfinished jobs.
func (t *unfinishedJobsTable) GetUnfinishedJobIDs(ctx context.Context) ([]string, error) {
	var rv []string
	ctx, cancel := context.WithTimeout(ctx, QUERY_TIMEOUT)
	defer cancel()
	if err := t.table.ReadRows(ctx, nil, func(row bigtable.Row) bool {
		id := row.Key()
		if id != "" {
			rv = append(rv, id)
		}
		return true
	}, bigtable.RowFilter(bigtable.LatestNFilter(1))); err != nil {
		return nil, err
	}
	return rv, nil
}

// AddUnfinishedJobs adds Jobs to the unfinished jobs table.
func (t *unfinishedJobsTable) AddUnfinishedJobs(ctx context.Context, ids []string) error {
	mts := make([]*bigtable.Mutation, 0, len(ids))
	for _, _ = range ids {
		mt := bigtable.NewMutation()
		mt.Set(COLUMN_FAMILY_JOB, COLUMN_JOB, bigtable.Time(time.Now()), nil)
		mts = append(mts, mt)
	}
	ctx, cancel := context.WithTimeout(ctx, INSERT_TIMEOUT)
	defer cancel()
	return combineErrors(t.table.ApplyBulk(ctx, ids, mts))
}

// RemoveUnfinishedJobs removes Jobs from the unfinished jobs table.
func (t *unfinishedJobsTable) RemoveUnfinishedJobs(ctx context.Context, ids []string) error {
	mts := make([]*bigtable.Mutation, 0, len(ids))
	for _, _ = range ids {
		mt := bigtable.NewMutation()
		mt.DeleteRow()
		mts = append(mts, mt)
	}
	ctx, cancel := context.WithTimeout(ctx, INSERT_TIMEOUT)
	defer cancel()
	return combineErrors(t.table.ApplyBulk(ctx, ids, mts))
}
