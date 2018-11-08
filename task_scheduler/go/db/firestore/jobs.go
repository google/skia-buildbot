package firestore

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/pborman/uuid"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	COLLECTION_JOBS = "jobs"
)

// Fix all timestamps for the given job.
func fixJobTimestamps(job *db.Job) {
	job.Created = fixTimestamp(job.Created)
	job.DbModified = fixTimestamp(job.DbModified)
	job.Finished = fixTimestamp(job.Finished)
}

// jobs returns a reference to the jobs collection.
func (d *firestoreDB) jobs() *firestore.CollectionRef {
	return d.collection(COLLECTION_JOBS)
}

// See documentation for db.JobReader interface.
func (d *firestoreDB) GetJobById(id string) (*db.Job, error) {
	doc, err := d.jobs().Doc(id).Get(context.Background())
	if grpc.Code(err) == codes.NotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var rv db.Job
	if err := doc.DataTo(&rv); err != nil {
		return nil, err
	}
	return &rv, nil
}

// See documentation for db.JobReader interface.
func (d *firestoreDB) GetJobsFromDateRange(start, end time.Time) ([]*db.Job, error) {
	// Adjust start and end times for Firestore resolution.
	start = fixTimestamp(start)
	end = fixTimestamp(end)
	// Adjust for time skew.
	min := start.Add(-local_db.MAX_CREATED_TIME_SKEW)
	max := end.Add(local_db.MAX_CREATED_TIME_SKEW)
	q := d.jobs().Where("Created", ">=", min).Where("Created", "<", max).OrderBy("Created", firestore.Asc)
	iter := q.Documents(context.Background())
	defer iter.Stop()
	rv := []*db.Job{}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		}
		var job db.Job
		if err := doc.DataTo(&job); err != nil {
			return nil, err
		}
		if !job.Created.Before(start) && end.After(job.Created) {
			// TODO(borenet): It's possible that we should require
			// another field when searching by timestamp; indexing a
			// timestamp causes the whole collection to cap out at a
			// maximum of 500 writes per second.
			rv = append(rv, &job)
		}
	}
	return rv, nil
}

// putJobs sets the contents of the given jobs in Firestore, as part of the
// given transaction. It is used by PutJob and PutJobs.
func (d *firestoreDB) putJobs(jobs []*db.Job, tx *firestore.Transaction) (rvErr error) {
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
	for idx, job := range jobs {
		ref := d.jobs().Doc(job.Id)
		doc, err := tx.Get(ref)
		if grpc.Code(err) == codes.NotFound {
			// This is expected for new jobs.
			if !isNew[idx] {
				sklog.Errorf("Job is not new but wasn't found in the DB.")
				// If the job is supposed to exist but does not, then
				// we have a problem.
				return db.ErrConcurrentUpdate
			}
		} else if err != nil {
			sklog.Errorf("isNew: %v", isNew)
			sklog.Errorf("Got error: %s", err)
			if grpc.Code(err) == codes.NotFound {
				sklog.Errorf("not found")
			}
			return err
		} else if isNew[idx] {
			// If the job is not supposed to exist but does, then
			// we have a problem.
			return db.ErrConcurrentUpdate
		}
		// If the job already exists, check the DbModified timestamp
		// to ensure that someone else didn't update it.
		if !isNew[idx] {
			var old db.Job
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

// See documentation for db.JobDB interface.
func (d *firestoreDB) PutJob(job *db.Job) error {
	return d.PutJobs([]*db.Job{job})
}

// See documentation for db.JobDB interface.
func (d *firestoreDB) PutJobs(jobs []*db.Job) error {
	for _, job := range jobs {
		if job.Id == "" {
			// TODO(borenet): Use firestore-assigned IDs.
			job.Id = uuid.New()
		}
		fixJobTimestamps(job)
	}

	ctx := context.Background()
	if err := d.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		return d.putJobs(jobs, tx)
	}); err != nil {
		return err
	}
	for _, job := range jobs {
		d.TrackModifiedJob(job)
	}
	return nil
}
