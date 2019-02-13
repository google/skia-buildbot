package firestore

import (
	"context"
	"fmt"
	"sort"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	COLLECTION_JOBS = "jobs"
)

// Fix all timestamps for the given job.
func fixJobTimestamps(job *types.Job) {
	job.Created = fixTimestamp(job.Created)
	job.DbModified = fixTimestamp(job.DbModified)
	job.Finished = fixTimestamp(job.Finished)
}

// jobs returns a reference to the jobs collection.
func (d *firestoreDB) jobs() *fs.CollectionRef {
	return d.client.Collection(COLLECTION_JOBS)
}

// See documentation for types.JobReader interface.
func (d *firestoreDB) GetJobById(id string) (*types.Job, error) {
	doc, err := d.client.Get(d.jobs().Doc(id), DEFAULT_ATTEMPTS, GET_SINGLE_TIMEOUT)
	if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var rv types.Job
	if err := doc.DataTo(&rv); err != nil {
		return nil, err
	}
	return &rv, nil
}

// See documentation for types.JobReader interface.
func (d *firestoreDB) GetJobsFromDateRange(start, end time.Time) ([]*types.Job, error) {
	var jobs [][]*types.Job
	init := func(numGoroutines int) {
		jobs = make([][]*types.Job, numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			estResults := estResultSize(end.Sub(start) / time.Duration(numGoroutines))
			jobs[i] = make([]*types.Job, 0, estResults)
		}
	}
	elem := func(idx int, doc *fs.DocumentSnapshot) error {
		var job types.Job
		if err := doc.DataTo(&job); err != nil {
			return err
		}
		jobs[idx] = append(jobs[idx], &job)
		return nil
	}
	if err := d.dateRangeHelper("GetJobsFromDateRange", d.jobs(), start, end, init, elem); err != nil {
		return nil, err
	}
	totalResults := 0
	for _, jobList := range jobs {
		totalResults += len(jobList)
	}
	rv := make([]*types.Job, 0, totalResults)
	for _, jobList := range jobs {
		rv = append(rv, jobList...)
	}
	sort.Sort(types.JobSlice(rv))
	return rv, nil
}

// putJobs sets the contents of the given jobs in Firestore, as part of the
// given transaction. It is used by PutJob and PutJobs.
func (d *firestoreDB) putJobs(jobs []*types.Job, isNew []bool, prevModified []time.Time, tx *fs.Transaction) (rvErr error) {
	// Set the new contents of the jobs.
	d.client.CountWriteQueryAndRows(d.jobs().Path, len(jobs))
	for _, job := range jobs {
		ref := d.jobs().Doc(job.Id)
		if err := tx.Set(ref, job); err != nil {
			return err
		}
	}
	return nil
}

// See documentation for types.JobDB interface.
func (d *firestoreDB) PutJob(job *types.Job) error {
	return d.PutJobs([]*types.Job{job})
}

// See documentation for types.JobDB interface.
func (d *firestoreDB) PutJobs(jobs []*types.Job) (rvErr error) {
	if len(jobs) > MAX_TRANSACTION_DOCS {
		return fmt.Errorf("Tried to insert %d jobs but Firestore maximum per transaction is %d.", len(jobs), MAX_TRANSACTION_DOCS)
	}

	for _, job := range jobs {
		fixJobTimestamps(job)
	}

	// Insert the jobs into the DB.
	if err := d.client.RunTransaction("PutJobs", fmt.Sprintf("%d jobs", len(jobs)), DEFAULT_ATTEMPTS, PUT_MULTI_TIMEOUT, func(ctx context.Context, tx *fs.Transaction) error {
		return d.putJobs(jobs, nil, nil, tx)
	}); err != nil {
		return err
	}
	for _, job := range jobs {
		d.TrackModifiedJob(job)
	}
	return nil
}

// See documentation for types.JobDB interface.
func (d *firestoreDB) PutJobsInChunks(jobs []*types.Job) error {
	return util.ChunkIter(len(jobs), MAX_TRANSACTION_DOCS, func(i, j int) error {
		return d.PutJobs(jobs[i:j])
	})
}
