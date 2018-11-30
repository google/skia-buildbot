package firestore

import (
	"context"
	"fmt"
	"sort"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
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
	doc, err := firestore.Get(d.jobs().Doc(id), DEFAULT_ATTEMPTS, GET_SINGLE_TIMEOUT)
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
	if err := d.dateRangeHelper(d.jobs(), start, end, init, elem); err != nil {
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
func (d *firestoreDB) putJobs(jobs []*types.Job, tx *fs.Transaction) (rvErr error) {
	// Set the modification time of the jobs.
	now := fixTimestamp(time.Now())
	isNew := make([]bool, len(jobs))
	prevModified := make([]time.Time, len(jobs))
	for idx, job := range jobs {
		if util.TimeIsZero(job.Created) {
			return fmt.Errorf("Created not set. Job %s created time is %s. %v", job.Id, job.Created, job)
		}
		isNew[idx] = util.TimeIsZero(job.DbModified)
		prevModified[idx] = job.DbModified
		if !now.After(job.DbModified) {
			return fmt.Errorf("Job modification time is in the future: %s (current time is %s)", job.DbModified, now)
		}
		job.DbModified = now
	}
	defer func() {
		if rvErr != nil {
			for idx, job := range jobs {
				job.DbModified = prevModified[idx]
			}
		}
	}()

	// Find the previous versions of the jobs. Ensure that they weren't
	// updated concurrently.
	refs := make([]*fs.DocumentRef, 0, len(jobs))
	for _, job := range jobs {
		refs = append(refs, d.jobs().Doc(job.Id))
	}
	docs, err := tx.GetAll(refs)
	if err != nil {
		return err
	}
	for idx, doc := range docs {
		if !doc.Exists() {
			// This is expected for new jobs.
			if !isNew[idx] {
				sklog.Errorf("Job is not new but wasn't found in the DB.")
				// If the job is supposed to exist but does not, then
				// we have a problem.
				return db.ErrConcurrentUpdate
			}
		} else if isNew[idx] {
			// If the job is not supposed to exist but does, then
			// we have a problem.
			return db.ErrConcurrentUpdate
		}
		// If the job already exists, check the DbModified timestamp
		// to ensure that someone else didn't update it.
		if !isNew[idx] {
			var old types.Job
			if err := doc.DataTo(&old); err != nil {
				return err
			}
			if old.DbModified != prevModified[idx] {
				return db.ErrConcurrentUpdate
			}
		}
	}

	// Set the new contents of the jobs.
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
func (d *firestoreDB) PutJobs(jobs []*types.Job) error {
	if len(jobs) > MAX_TRANSACTION_DOCS/2 {
		sklog.Warningf("Inserting %d jobs; Firestore maximum per transaction is %d", len(jobs), MAX_TRANSACTION_DOCS)
	}
	for _, job := range jobs {
		if job.Id == "" {
			job.Id = d.jobs().NewDoc().ID
		}
		fixJobTimestamps(job)
	}

	if err := firestore.RunTransaction(d.client, DEFAULT_ATTEMPTS, PUT_MULTI_TIMEOUT, func(ctx context.Context, tx *fs.Transaction) error {
		return d.putJobs(jobs, tx)
	}); err != nil {
		return err
	}
	for _, job := range jobs {
		d.TrackModifiedJob(job)
	}
	return nil
}
