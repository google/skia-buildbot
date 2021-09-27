package firestore

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
	job.Created = firestore.FixTimestamp(job.Created)
	job.DbModified = firestore.FixTimestamp(job.DbModified)
	job.Finished = firestore.FixTimestamp(job.Finished)
	job.Requested = firestore.FixTimestamp(job.Requested)
}

// jobs returns a reference to the jobs collection.
func (d *firestoreDB) jobs() *fs.CollectionRef {
	return d.client.Collection(COLLECTION_JOBS)
}

// See documentation for types.JobReader interface.
func (d *firestoreDB) GetJobById(ctx context.Context, id string) (*types.Job, error) {
	doc, err := d.client.Get(ctx, d.jobs().Doc(id), DEFAULT_ATTEMPTS, GET_SINGLE_TIMEOUT)
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
func (d *firestoreDB) GetJobsFromDateRange(ctx context.Context, start, end time.Time, repo string) ([]*types.Job, error) {
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
		if doc.Ref.ID != job.Id {
			sklog.Errorf("Job %s is stored with ID %s; GetJobById will not be able to find it!", job.Id, doc.Ref.ID)
			return nil
		}
		if repo != "" {
			if job.Repo != repo {
				sklog.Errorf("Query returned job with wrong repo; wanted %q but got %q; job: %+v", repo, job.Repo, job)
				return nil
			}
		}
		jobs[idx] = append(jobs[idx], &job)
		return nil
	}
	q := d.jobs().Query
	if repo != "" {
		q = q.Where(KEY_REPO, "==", repo)
	}
	if err := d.dateRangeHelper(ctx, "GetJobsFromDateRange", q, start, end, init, elem); err != nil {
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
	d.client.CountReadQueryAndRows(d.jobs().Path, len(docs))
	for idx, doc := range docs {
		if !doc.Exists() {
			// This is expected for new jobs.
			if !isNew[idx] {
				sklog.Errorf("Job is not new but wasn't found in the DB: %+v", jobs[idx])
				// If the job is supposed to exist but does not, then
				// we have a problem.
				return db.ErrConcurrentUpdate
			}
		} else if isNew[idx] {
			// If the job is not supposed to exist but does, then
			// we have a problem.
			var old types.Job
			if err := doc.DataTo(&old); err != nil {
				return fmt.Errorf("Job has no DbModified timestamp but already exists in the DB. Failed to decode previous job with: %s", err)
			}
			sklog.Errorf("Job has no DbModified timestamp but already exists in the DB! \"New\" job:\n%+v\nExisting job:\n%+v", jobs[idx], old)
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
				sklog.Infof("Concurrent update: Job %s in DB has DbModified %s; cached job has DbModified %s. \"New\" job:\n%+v\nExisting job:\n%+v", old.Id, old.DbModified.Format(time.RFC3339Nano), prevModified[idx].Format(time.RFC3339Nano), jobs[idx], old)
				return db.ErrConcurrentUpdate
			}
		}
	}

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

// PutJob implements types.JobDB.
func (d *firestoreDB) PutJob(ctx context.Context, job *types.Job) error {
	return d.PutJobs(ctx, []*types.Job{job})
}

// PutJobs implements types.JobDB.
func (d *firestoreDB) PutJobs(ctx context.Context, jobs []*types.Job) (rvErr error) {
	if len(jobs) == 0 {
		return nil
	}
	if len(jobs) > MAX_TRANSACTION_DOCS {
		return fmt.Errorf("Tried to insert %d jobs but Firestore maximum per transaction is %d.", len(jobs), MAX_TRANSACTION_DOCS)
	}

	// Record the previous ID and DbModified timestamp. We'll reset these
	// if we fail to insert the jobs into the DB.
	now := firestore.FixTimestamp(time.Now())
	isNew := make([]bool, len(jobs))
	prevId := make([]string, len(jobs))
	prevModified := make([]time.Time, len(jobs))
	for idx, job := range jobs {
		if util.TimeIsZero(job.Created) {
			return fmt.Errorf("Created not set. Job %s created time is %s. %v", job.Id, job.Created, job)
		}
		isNew[idx] = util.TimeIsZero(job.DbModified)
		prevId[idx] = job.Id
		prevModified[idx] = job.DbModified
	}
	defer func() {
		if rvErr != nil {
			for idx, job := range jobs {
				job.Id = prevId[idx]
				job.DbModified = prevModified[idx]
			}
		}
	}()

	// logmsg builds a log message to debug skia:9444.
	var logmsg strings.Builder
	if _, err := fmt.Fprintf(&logmsg, "Added/updated Jobs with DbModified %s:", now.Format(time.RFC3339Nano)); err != nil {
		sklog.Warningf("Error building log message: %s", err)
	}
	// Assign new IDs (where needed) and DbModified timestamps.
	for _, job := range jobs {
		logmsg.WriteRune(' ')
		if job.Id == "" {
			job.Id = firestore.AlphaNumID()
			logmsg.WriteRune('+')
		}
		logmsg.WriteString(job.Id)
		if !now.After(job.DbModified) {
			// We can't use the same DbModified timestamp for two updates,
			// or we risk losing updates. Increment the timestamp if
			// necessary.
			job.DbModified = job.DbModified.Add(firestore.TS_RESOLUTION)
			if _, err := fmt.Fprintf(&logmsg, "@%s", job.DbModified.Format(time.RFC3339Nano)); err != nil {
				sklog.Warningf("Error building log message: %s", err)
			}
		} else {
			job.DbModified = now
		}
		fixJobTimestamps(job)
	}

	// Insert the jobs into the DB.
	if err := d.client.RunTransaction(ctx, "PutJobs", fmt.Sprintf("%d jobs", len(jobs)), DEFAULT_ATTEMPTS, PUT_MULTI_TIMEOUT, func(ctx context.Context, tx *fs.Transaction) error {
		return d.putJobs(jobs, isNew, prevModified, tx)
	}); err != nil {
		return err
	}
	sklog.Debug(logmsg.String())
	return nil
}

// PutJobsInChunks implements types.JobDB.
func (d *firestoreDB) PutJobsInChunks(ctx context.Context, jobs []*types.Job) error {
	return util.ChunkIter(len(jobs), MAX_TRANSACTION_DOCS, func(i, j int) error {
		return d.PutJobs(ctx, jobs[i:j])
	})
}

// SearchJobs implements db.JobReader.
func (d *firestoreDB) SearchJobs(ctx context.Context, params *db.JobSearchParams) ([]*types.Job, error) {
	// Firestore requires all multi-column indexes to be created in advance.
	// Because we can't predict which search parameters will be given (and
	// because we don't want to create indexes for every combination of
	// parameters), we search by the parameter we think will limit the results
	// the most, then filter those results by the other parameters.
	q := d.jobs().Query
	term := "none"
	if params.BuildbucketBuildID != nil && *params.BuildbucketBuildID != 0 {
		q = q.Where("BuildbucketBuildId", "==", *params.BuildbucketBuildID)
		term = fmt.Sprintf("BuildBucketBuildId == %d", *params.BuildbucketBuildID)
	} else if params.Issue != nil && *params.Issue != "" {
		q = q.Where("Issue", "==", *params.Issue)
		term = fmt.Sprintf("Issue == %s", *params.Issue)
	} else if params.IsForce != nil && *params.IsForce {
		q = q.Where("IsForce", "==", *params.IsForce)
		term = fmt.Sprintf("IsForce == %v", *params.IsForce)
	} else if params.Revision != nil {
		q = q.Where("Revision", "==", *params.Revision)
		term = fmt.Sprintf("Revision == %s", *params.Revision)
	} else {
		term = fmt.Sprintf("Created in [%s, %s)", *params.TimeStart, params.TimeEnd)
		q = q.Where(KEY_CREATED, "<", *params.TimeEnd).Where(KEY_CREATED, ">=", *params.TimeStart)

		// Repo is compatible with TimeStart and TimeEnd because we have an
		// index for it.
		if params.Repo != nil {
			q = q.Where("Repo", "==", *params.Repo)
			term += fmt.Sprintf(" and Repo == %s", *params.Repo)
		}
	}

	// Search the DB.
	results := []*types.Job{}
	err := d.client.IterDocs(ctx, "SearchJobs", term, q, DEFAULT_ATTEMPTS, GET_MULTI_TIMEOUT, func(doc *fs.DocumentSnapshot) error {
		var job types.Job
		if err := doc.DataTo(&job); err != nil {
			return err
		}
		if db.MatchJob(&job, params) {
			results = append(results, &job)
		}
		if len(results) >= db.SearchResultLimit {
			return db.ErrDoneSearching
		}
		return nil
	})
	if err == db.ErrDoneSearching {
		err = nil
	}
	sklog.Infof("Searching jobs; terms: %q; results: %d", term, len(results))
	return results, err
}
