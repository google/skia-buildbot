package tryjobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	buildbucket_api "go.chromium.org/luci/common/api/buildbucket/buildbucket/v1"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/cacher"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/types"
)

/*
	Integration of the Task Scheduler with Buildbucket for try jobs.
*/

const (
	// API URLs
	API_URL_PROD    = "https://cr-buildbucket.appspot.com/api/buildbucket/v1/"
	API_URL_TESTING = "http://localhost:8008/api/buildbucket/v1/"

	// Buildbucket buckets used for try jobs.
	BUCKET_PRIMARY  = "skia.primary"
	BUCKET_INTERNAL = "skia.internal"
	BUCKET_TESTING  = "skia.testing"

	// How often to send updates to Buildbucket.
	UPDATE_INTERVAL = 30 * time.Second

	// We attempt to renew leases in batches. This is the batch size.
	LEASE_BATCH_SIZE = 25

	// We lease a build for this amount of time, and if we don't renew the
	// lease before the time is up, the build resets to "scheduled" status
	// and becomes available for leasing again.
	LEASE_DURATION = time.Hour

	// We use a shorter initial lease duration in case we succeed in leasing
	// a build but fail to insert the associated Job into the DB, eg.
	// because the scheduler was interrupted.
	LEASE_DURATION_INITIAL = 5 * time.Minute

	// How many pending builds to read from the bucket at a time.
	PEEK_MAX_BUILDS = 50

	// How often to poll Buildbucket for newly-scheduled builds.
	POLL_INTERVAL = 10 * time.Second

	// This error reason indicates that we already marked the build as
	// finished.
	BUILDBUCKET_API_ERROR_REASON_COMPLETED = "BUILD_IS_COMPLETED"

	secondsToMicros = 1000000
	microsToNanos   = 1000

	// In case the error is very verbose (e.g. bot_update output), only send a
	// truncated cancel reason to Buildbucket to avoid exceeding limits in
	// Buildbucket's DB.
	maxCancelReasonLen = 1024
)

// TryJobIntegrator is responsible for communicating with Buildbucket to
// trigger try jobs and report their results.
type TryJobIntegrator struct {
	bb                 *buildbucket_api.Service
	bb2                buildbucket.BuildBucketInterface
	bucket             string
	chr                *cacher.Cacher
	db                 db.JobDB
	gerrit             gerrit.GerritInterface
	host               string
	jCache             cache.JobCache
	projectRepoMapping map[string]string
	rm                 repograph.Map
	taskCfgCache       *task_cfg_cache.TaskCfgCache
}

// NewTryJobIntegrator returns a TryJobIntegrator instance.
func NewTryJobIntegrator(apiUrl, bucket, host string, c *http.Client, d db.JobDB, jCache cache.JobCache, projectRepoMapping map[string]string, rm repograph.Map, taskCfgCache *task_cfg_cache.TaskCfgCache, chr *cacher.Cacher, gerrit gerrit.GerritInterface) (*TryJobIntegrator, error) {
	bb, err := buildbucket_api.New(c)
	if err != nil {
		return nil, err
	}
	bb.BasePath = apiUrl
	rv := &TryJobIntegrator{
		bb:                 bb,
		bb2:                buildbucket.NewClient(c),
		bucket:             bucket,
		db:                 d,
		chr:                chr,
		gerrit:             gerrit,
		host:               host,
		jCache:             jCache,
		projectRepoMapping: projectRepoMapping,
		rm:                 rm,
		taskCfgCache:       taskCfgCache,
	}
	return rv, nil
}

// Start initiates the TryJobIntegrator's heatbeat and polling loops. If the
// given Context is canceled, the loops stop.
func (t *TryJobIntegrator) Start(ctx context.Context) {
	lvUpdate := metrics2.NewLiveness("last_successful_update_buildbucket_tryjob_state")
	cleanup.Repeat(UPDATE_INTERVAL, func(_ context.Context) {
		// Explicitly ignore the passed-in context; this allows us to
		// finish sending heartbeats and updating finished jobs in the
		// DB even if the context is canceled, which helps to prevent
		// inconsistencies between Buildbucket and the Task Scheduler
		// DB.
		if err := t.updateJobs(ctx); err != nil {
			sklog.Error(err)
		} else {
			lvUpdate.Reset()
		}
	}, nil)
	lvPoll := metrics2.NewLiveness("last_successful_poll_buildbucket_for_new_tryjobs")
	cleanup.Repeat(POLL_INTERVAL, func(_ context.Context) {
		// Explicitly ignore the passed-in context; this allows us to
		// finish leasing jobs from Buildbucket and inserting them into
		// the DB even if the context is canceled, which helps to
		// prevent inconsistencies between Buildbucket and the Task
		// Scheduler DB.
		ctx := context.Background()
		if err := t.Poll(ctx); err != nil {
			sklog.Errorf("Failed to poll for new try jobs: %s", err)
		} else {
			lvPoll.Reset()
		}
	}, nil)
}

// getActiveTryJobs returns the active (not yet marked as finished in
// Buildbucket) tryjobs.
func (t *TryJobIntegrator) getActiveTryJobs(ctx context.Context) ([]*types.Job, error) {
	if err := t.jCache.Update(ctx); err != nil {
		return nil, err
	}
	jobs := t.jCache.GetAllCachedJobs()
	rv := []*types.Job{}
	for _, job := range jobs {
		if job.BuildbucketLeaseKey != 0 {
			rv = append(rv, job)
		}
	}
	return rv, nil
}

// updateJobs sends updates to Buildbucket for all active try Jobs.
func (t *TryJobIntegrator) updateJobs(ctx context.Context) error {
	// Get all Jobs associated with in-progress Buildbucket builds.
	jobs, err := t.getActiveTryJobs(ctx)
	if err != nil {
		return err
	}

	// Divide up finished and unfinished Jobs.
	finished := make([]*types.Job, 0, len(jobs))
	unfinished := make([]*types.Job, 0, len(jobs))
	for _, j := range jobs {
		if j.Done() {
			finished = append(finished, j)
		} else {
			unfinished = append(unfinished, j)
		}
	}

	// Send heartbeats for unfinished Jobs.
	var heartbeatErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		heartbeatErr = t.sendHeartbeats(ctx, unfinished)
	}()

	// Send updates for finished Jobs, empty the lease keys to mark them
	// as inactive in the DB.
	errs := []error{}
	insert := make([]*types.Job, 0, len(finished))
	for _, j := range finished {
		if err := t.jobFinished(j); err != nil {
			errs = append(errs, err)
		} else {
			j.BuildbucketLeaseKey = 0
			insert = append(insert, j)
		}
	}
	if err := t.db.PutJobsInChunks(ctx, insert); err != nil {
		errs = append(errs, err)
	}
	t.jCache.AddJobs(insert)

	wg.Wait()
	if heartbeatErr != nil {
		errs = append(errs, heartbeatErr)
	}

	if len(errs) > 0 {
		return fmt.Errorf("Failed to update jobs; got errors: %v", errs)
	}
	return nil
}

// heartbeatJobSlice implements sort.Interface to sort Jobs by BuildbucketBuildId.
type heartbeatJobSlice []*types.Job

func (s heartbeatJobSlice) Len() int { return len(s) }

func (s heartbeatJobSlice) Less(i, j int) bool {
	return s[i].BuildbucketBuildId < s[j].BuildbucketBuildId
}

func (s heartbeatJobSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// sendHeartbeats sends heartbeats to Buildbucket for all of the unfinished try
// Jobs.
func (t *TryJobIntegrator) sendHeartbeats(ctx context.Context, jobs []*types.Job) error {
	defer metrics2.FuncTimer().Stop()

	// Sort the jobs by BuildbucketBuildId for consistency in testing.
	sort.Sort(heartbeatJobSlice(jobs))

	expiration := now.Now(ctx).Add(LEASE_DURATION).Unix() * secondsToMicros

	errs := []error{}

	// Send heartbeats for all leases.
	send := func(jobs []*types.Job) {
		heartbeats := make([]*buildbucket_api.LegacyApiHeartbeatBatchRequestMessageOneHeartbeat, 0, len(jobs))
		for _, j := range jobs {
			heartbeats = append(heartbeats, &buildbucket_api.LegacyApiHeartbeatBatchRequestMessageOneHeartbeat{
				BuildId:           j.BuildbucketBuildId,
				LeaseKey:          j.BuildbucketLeaseKey,
				LeaseExpirationTs: expiration,
			})
		}
		sklog.Infof("Sending heartbeats for %d jobs...", len(jobs))
		resp, err := t.bb.HeartbeatBatch(&buildbucket_api.LegacyApiHeartbeatBatchRequestMessage{
			Heartbeats: heartbeats,
		}).Do()
		if err != nil {
			errs = append(errs, fmt.Errorf("Failed to send heartbeat request: %s", err))
			return
		}
		// Results should follow the same ordering as the jobs we sent.
		if len(resp.Results) != len(jobs) {
			errs = append(errs, fmt.Errorf("Heartbeat result has incorrect number of jobs (%d vs %d)", len(resp.Results), len(jobs)))
			return
		}
		cancelJobs := []*types.Job{}
		for i, result := range resp.Results {
			if result.Error != nil {
				// Cancel the job.
				if result.Error.Reason == BUILDBUCKET_API_ERROR_REASON_COMPLETED {
					// This indicates that the build was canceled, eg. because
					// a newer patchset was uploaded. This isn't an error, so we
					// cancel the job but don't log an error.
				} else {
					sklog.Errorf("Error sending heartbeat for job; canceling %q: %s", jobs[i].Id, result.Error.Message)
				}
				cancelJobs = append(cancelJobs, jobs[i])
			}
		}
		if len(cancelJobs) > 0 {
			sklog.Infof("Canceling %d jobs", len(cancelJobs))
			if err := t.localCancelJobs(ctx, cancelJobs); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Send heartbeats in batches.
	for len(jobs) > 0 {
		j := LEASE_BATCH_SIZE
		if j > len(jobs) {
			j = len(jobs)
		}
		send(jobs[:j])
		jobs = jobs[j:]
	}
	sklog.Infof("Finished sending heartbeats.")
	if len(errs) > 0 {
		return fmt.Errorf("Got errors sending heartbeats: %v", errs)
	}
	return nil
}

// getRepo returns the repo information associated with the given project.
// Returns the repo URL and its associated repograph.Graph instance or an error.
func (t *TryJobIntegrator) getRepo(project string) (string, *repograph.Graph, error) {
	repoUrl, ok := t.projectRepoMapping[project]
	if !ok {
		return "", nil, fmt.Errorf("Unknown patch project %q", project)
	}
	r, ok := t.rm[repoUrl]
	if !ok {
		return "", nil, fmt.Errorf("Unknown repo %q", repoUrl)
	}
	return repoUrl, r, nil
}

func (t *TryJobIntegrator) getRevision(ctx context.Context, repo *repograph.Graph, issue int64) (string, error) {
	// Obtain the branch name from Gerrit, then use the head of that branch.
	changeInfo, err := t.gerrit.GetIssueProperties(ctx, issue)
	if err != nil {
		return "", fmt.Errorf("Failed to get ChangeInfo: %s", err)
	}
	c := repo.Get(changeInfo.Branch)
	if c == nil {
		return "", fmt.Errorf("Unknown branch %s", changeInfo.Branch)
	}
	return c.Hash, nil
}

func (t *TryJobIntegrator) localCancelJobs(ctx context.Context, jobs []*types.Job) error {
	for _, j := range jobs {
		j.BuildbucketLeaseKey = 0
		j.Status = types.JOB_STATUS_CANCELED
		j.Finished = now.Now(ctx)
	}
	if err := t.db.PutJobsInChunks(ctx, jobs); err != nil {
		return err
	}
	t.jCache.AddJobs(jobs)
	return nil
}

func (t *TryJobIntegrator) remoteCancelBuild(id int64, msg string) error {
	sklog.Warningf("Canceling Buildbucket build %d. Reason: %s", id, msg)
	message := struct {
		Message string `json:"message"`
	}{
		Message: util.Truncate(msg, maxCancelReasonLen),
	}
	b, err := json.Marshal(&message)
	if err != nil {
		return err
	}
	resp, err := t.bb.Cancel(id, &buildbucket_api.LegacyApiCancelRequestBodyMessage{
		ResultDetailsJson: string(b),
	}).Do()
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf(resp.Error.Message)
	}
	return nil
}

func (t *TryJobIntegrator) tryLeaseBuild(ctx context.Context, id int64) (int64, error) {
	expiration := now.Now(ctx).Add(LEASE_DURATION_INITIAL).Unix() * secondsToMicros
	sklog.Infof("Attempting to lease build %d", id)
	resp, err := t.bb.Lease(id, &buildbucket_api.LegacyApiLeaseRequestBodyMessage{
		LeaseExpirationTs: expiration,
	}).Do()
	if err != nil {
		return 0, fmt.Errorf("Failed request to lease buildbucket build %d: %s", id, err)
	}
	if resp.Error != nil {
		return 0, fmt.Errorf("Error response for leasing buildbucket build %d: %s", id, resp.Error.Message)
	}
	return resp.Build.LeaseKey, nil
}

func (t *TryJobIntegrator) insertNewJob(ctx context.Context, buildId int64) error {
	// Get the build details from the v2 API.
	build, err := t.bb2.GetBuild(ctx, buildId)
	if err != nil {
		return t.remoteCancelBuild(buildId, fmt.Sprintf("Failed to retrieve build %q: %s", buildId, err))
	}
	if build.Status != buildbucketpb.Status_SCHEDULED {
		sklog.Warningf("Found build %d with status: %s; attempting to lease anyway, to trigger the fix in Buildbucket.", build.Id, build.Status)
		_, err := t.tryLeaseBuild(ctx, buildId)
		if err != nil {
			// This is expected.
			return nil
		}
		sklog.Warningf("Unexpectedly able to lease build %d with status %s; canceling it.", buildId, build.Status)
		if err := t.remoteCancelBuild(buildId, fmt.Sprintf("Unexpected status %s", build.Status)); err != nil {
			sklog.Warningf("Failed to cancel errant build %d", buildId)
			return nil
		}
	}

	// Obtain and validate the RepoState.
	if build.Input.GerritChanges == nil || len(build.Input.GerritChanges) != 1 {
		return t.remoteCancelBuild(buildId, fmt.Sprintf("Invalid Build %d: input should have exactly one GerritChanges: %+v", buildId, build.Input))
	}
	gerritChange := build.Input.GerritChanges[0]
	repoUrl, repoGraph, err := t.getRepo(gerritChange.Project)
	if err != nil {
		return t.remoteCancelBuild(buildId, fmt.Sprintf("Unable to find repo: %s", err))
	}
	revision, err := t.getRevision(ctx, repoGraph, gerritChange.Change)
	if err != nil {
		return t.remoteCancelBuild(buildId, fmt.Sprintf("Invalid revision: %s", err))
	}
	server := gerritChange.Host
	if !strings.Contains(server, "://") {
		server = fmt.Sprintf("https://%s", server)
	}
	rs := types.RepoState{
		Patch: types.Patch{
			Server:    server,
			Issue:     strconv.FormatInt(gerritChange.Change, 10),
			PatchRepo: repoUrl,
			Patchset:  strconv.FormatInt(gerritChange.Patchset, 10),
		},
		Repo:     repoUrl,
		Revision: revision,
	}
	if !rs.Valid() || !rs.IsTryJob() {
		return t.remoteCancelBuild(buildId, fmt.Sprintf("Invalid RepoState: %s", rs))
	}

	// Create a Job.
	if _, err := t.chr.GetOrCacheRepoState(ctx, rs); err != nil {
		return t.remoteCancelBuild(buildId, fmt.Sprintf("Failed to obtain JobSpec: %s; \n\n%v", err, rs))
	}
	j, err := t.taskCfgCache.MakeJob(ctx, rs, build.Builder.Builder)
	if err != nil {
		return t.remoteCancelBuild(buildId, fmt.Sprintf("Failed to create Job from JobSpec: %s @ %+v: %s", build.Builder.Builder, rs, err))
	}
	requested, err := ptypes.Timestamp(build.CreateTime)
	if err != nil {
		return t.remoteCancelBuild(buildId, fmt.Sprintf("Failed to convert timestamp for %d: %s", build.Id, err))
	}
	j.Requested = firestore.FixTimestamp(requested.UTC())
	j.Created = firestore.FixTimestamp(j.Created)
	if !j.Requested.Before(j.Created) {
		sklog.Errorf("Try job created time %s is before requested time %s! Setting equal.", j.Created, j.Requested)
		j.Requested = j.Created.Add(-firestore.TS_RESOLUTION)
	}

	// Determine if this is a manual retry of a previously-run try job. If
	// so, set IsForce to ensure that we don't immediately de-duplicate all
	// of its tasks.
	prevJobs, err := t.jCache.GetJobsByRepoState(j.Name, j.RepoState)
	if err != nil {
		return err
	}
	if len(prevJobs) > 0 {
		j.IsForce = true
	}

	// Attempt to lease the build.
	leaseKey, err := t.tryLeaseBuild(ctx, buildId)
	if err != nil {
		// TODO(borenet): Buildbot cancels the build in this case.
		// Should we do that too?
		return err
	}

	// Update the job and insert into the DB.
	j.BuildbucketBuildId = buildId
	j.BuildbucketLeaseKey = leaseKey
	if err := t.db.PutJob(ctx, j); err != nil {
		return t.remoteCancelBuild(buildId, fmt.Sprintf("Failed to insert Job into the DB: %s", err))
	}
	t.jCache.AddJobs([]*types.Job{j})

	// Since Jobs may consist of multiple Tasks, we consider them to be
	// "started" as soon as we've picked them up.
	// TODO(borenet): Sending "started" notifications after inserting the
	// new Jobs into the database puts us at risk of never sending the
	// notification if the process is interrupted. However, we need to
	// include the Job ID with the notification, so we have to insert the
	// Job into the DB first.
	if err := t.jobStarted(j); err != nil {
		if cancelErr := t.localCancelJobs(ctx, []*types.Job{j}); cancelErr != nil {
			return fmt.Errorf("Failed to send job-started notification with: %s\nAnd failed to cancel the job with: %s", err, cancelErr)
		}
		return fmt.Errorf("Failed to send job-started notification with: %s", err)
	}
	return nil
}

func (t *TryJobIntegrator) Poll(ctx context.Context) error {
	if err := t.jCache.Update(ctx); err != nil {
		return err
	}

	// Grab all of the pending Builds from Buildbucket.
	// TODO(borenet): Buildbot maintains a maximum lease count. Should we do
	// that too?
	cursor := ""
	errs := []error{}
	var mtx sync.Mutex
	for {
		sklog.Infof("Running 'peek' on %s", t.bucket)
		resp, err := t.bb.Peek().Bucket(t.bucket).MaxBuilds(PEEK_MAX_BUILDS).StartCursor(cursor).Do()
		if err != nil {
			errs = append(errs, err)
			break
		}
		if resp.Error != nil {
			errs = append(errs, fmt.Errorf(resp.Error.Message))
			break
		}
		var wg sync.WaitGroup
		for _, b := range resp.Builds {
			wg.Add(1)
			go func(b *buildbucket_api.LegacyApiCommonBuildMessage) {
				defer wg.Done()
				if err := t.insertNewJob(ctx, b.Id); err != nil {
					mtx.Lock()
					errs = append(errs, err)
					mtx.Unlock()
				}
			}(b)
		}
		wg.Wait()
		cursor = resp.NextCursor
		if cursor == "" {
			break
		}
	}

	// Report any errors.
	if len(errs) > 0 {
		return fmt.Errorf("Got errors loading builds from Buildbucket: %v", errs)
	}

	return nil
}

// jobStarted notifies Buildbucket that the given Job has started.
func (t *TryJobIntegrator) jobStarted(j *types.Job) error {
	resp, err := t.bb.Start(j.BuildbucketBuildId, &buildbucket_api.LegacyApiStartRequestBodyMessage{
		LeaseKey: j.BuildbucketLeaseKey,
		Url:      j.URL(t.host),
	}).Do()
	if err != nil {
		return err
	}
	if resp.Error != nil {
		// TODO(borenet): Buildbot cancels builds in this case. Should
		// we do that too?
		return fmt.Errorf(resp.Error.Message)
	}
	return nil
}

// jobFinished notifies Buildbucket that the given Job has finished.
func (t *TryJobIntegrator) jobFinished(j *types.Job) error {
	if !j.Done() {
		return fmt.Errorf("JobFinished called for unfinished Job!")
	}
	b, err := json.Marshal(struct {
		Job *types.Job `json:"job"`
	}{
		Job: j,
	})
	if err != nil {
		return err
	}
	if j.Status == types.JOB_STATUS_SUCCESS {
		resp, err := t.bb.Succeed(j.BuildbucketBuildId, &buildbucket_api.LegacyApiSucceedRequestBodyMessage{
			LeaseKey:          j.BuildbucketLeaseKey,
			ResultDetailsJson: string(b),
			Url:               j.URL(t.host),
		}).Do()
		if err != nil {
			return err
		}
		if resp.Error != nil {
			if resp.Error.Reason == BUILDBUCKET_API_ERROR_REASON_COMPLETED {
				sklog.Warningf("Sent success status for build %d after completion.", j.BuildbucketBuildId)
			} else {
				return fmt.Errorf(resp.Error.Message)
			}
		}
	} else {
		failureReason := "BUILD_FAILURE"
		if j.Status == types.JOB_STATUS_MISHAP {
			failureReason = "INFRA_FAILURE"
		}
		resp, err := t.bb.Fail(j.BuildbucketBuildId, &buildbucket_api.LegacyApiFailRequestBodyMessage{
			FailureReason:     failureReason,
			LeaseKey:          j.BuildbucketLeaseKey,
			ResultDetailsJson: string(b),
			Url:               j.URL(t.host),
		}).Do()
		if err != nil {
			return err
		}
		if resp.Error != nil {
			if resp.Error.Reason == BUILDBUCKET_API_ERROR_REASON_COMPLETED {
				sklog.Warningf("Sent failure status for build %d after completion.", j.BuildbucketBuildId)
			} else {
				return fmt.Errorf(resp.Error.Message)
			}
		}
	}
	return nil
}
