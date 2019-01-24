package tryjobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	buildbucket_api "go.chromium.org/luci/common/api/buildbucket/buildbucket/v1"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
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
)

// TryJobIntegrator is responsible for communicating with Buildbucket to
// trigger try jobs and report their results.
type TryJobIntegrator struct {
	bb                 *buildbucket_api.Service
	bucket             string
	cacheMtx           sync.Mutex
	db                 db.JobDB
	gerrit             gerrit.GerritInterface
	host               string
	jCache             cache.JobCache
	projectRepoMapping map[string]string
	rm                 repograph.Map
	taskCfgCache       *specs.TaskCfgCache
	tjCache            *tryJobCache
}

// NewTryJobIntegrator returns a TryJobIntegrator instance.
func NewTryJobIntegrator(apiUrl, bucket, host string, c *http.Client, d db.JobDB, w *window.Window, projectRepoMapping map[string]string, rm repograph.Map, taskCfgCache *specs.TaskCfgCache, gerrit gerrit.GerritInterface) (*TryJobIntegrator, error) {
	bb, err := buildbucket_api.New(c)
	if err != nil {
		return nil, err
	}
	bb.BasePath = apiUrl
	jCache, err := cache.NewJobCache(d, w, cache.GitRepoGetRevisionTimestamp(rm))
	if err != nil {
		return nil, err
	}
	tjCache, err := newTryJobCache(d, w)
	if err != nil {
		return nil, err
	}
	gerrit.TurnOnAuthenticatedGets()
	rv := &TryJobIntegrator{
		bb:                 bb,
		bucket:             bucket,
		db:                 d,
		gerrit:             gerrit,
		host:               host,
		jCache:             jCache,
		projectRepoMapping: projectRepoMapping,
		rm:                 rm,
		taskCfgCache:       taskCfgCache,
		tjCache:            tjCache,
	}
	return rv, nil
}

// Start initiates the TryJobIntegrator's heatbeat and polling loops. If the
// given Context is canceled, the loops stop.
func (t *TryJobIntegrator) Start(ctx context.Context) {
	go util.RepeatCtx(UPDATE_INTERVAL, ctx, func() {
		if err := t.updateJobs(time.Now()); err != nil {
			sklog.Error(err)
		}
	})
	go util.RepeatCtx(POLL_INTERVAL, ctx, func() {
		if err := t.Poll(ctx, time.Now()); err != nil {
			sklog.Errorf("Failed to poll for new try jobs: %s", err)
		}
	})
}

// updateCaches updates both internal caches.
func (t *TryJobIntegrator) updateCaches() error {
	t.cacheMtx.Lock()
	defer t.cacheMtx.Unlock()
	if err := t.tjCache.Update(); err != nil {
		return err
	}
	return t.jCache.Update()
}

// updateJobs sends updates to Buildbucket for all active try Jobs.
func (t *TryJobIntegrator) updateJobs(now time.Time) error {
	// Get all Jobs associated with in-progress Buildbucket builds.
	if err := t.updateCaches(); err != nil {
		return err
	}

	jobs, err := t.tjCache.GetActiveTryJobs()
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
		heartbeatErr = t.sendHeartbeats(now, unfinished)
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
	if err := t.db.PutJobsInChunks(insert); err != nil {
		errs = append(errs, err)
	}

	wg.Wait()
	if heartbeatErr != nil {
		errs = append(errs, heartbeatErr)
	}

	if len(errs) > 0 {
		return fmt.Errorf("Failed to update jobs; got errors: %v", errs)
	}
	return nil
}

// sendHeartbeats sends heartbeats to Buildbucket for all of the unfinished try
// Jobs.
func (t *TryJobIntegrator) sendHeartbeats(now time.Time, jobs []*types.Job) error {
	defer metrics2.FuncTimer().Stop()

	expiration := now.Add(LEASE_DURATION).Unix() * 1000000

	errs := []error{}

	// Send heartbeats for all leases.
	send := func(jobs []*types.Job) {
		heartbeats := make([]*buildbucket_api.ApiHeartbeatBatchRequestMessageOneHeartbeat, 0, len(jobs))
		for _, j := range jobs {
			heartbeats = append(heartbeats, &buildbucket_api.ApiHeartbeatBatchRequestMessageOneHeartbeat{
				BuildId:           j.BuildbucketBuildId,
				LeaseKey:          j.BuildbucketLeaseKey,
				LeaseExpirationTs: expiration,
			})
		}
		sklog.Infof("Sending heartbeats for %d jobs...", len(jobs))
		resp, err := t.bb.HeartbeatBatch(&buildbucket_api.ApiHeartbeatBatchRequestMessage{
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
				// TODO(borenet): Should we return an error here?
				sklog.Errorf("Error sending heartbeat for job; canceling %q: %s", jobs[i].Id, result.Error.Message)
				cancelJobs = append(cancelJobs, jobs[i])
			}
		}
		if len(cancelJobs) > 0 {
			sklog.Errorf("Canceling %d jobs", len(cancelJobs))
			if err := t.localCancelJobs(cancelJobs); err != nil {
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

// getRepo uses the buildbucket properties to determine what top-level repo
// the try job wants us to sync, and what repo the patch is for. Returns the
// top level repo URL, its associated repograph.Graph instance, the patch repo
// URL, or an error.
func (t *TryJobIntegrator) getRepo(props *buildbucket.Properties) (string, *repograph.Graph, string, error) {
	patchRepoUrl, ok := t.projectRepoMapping[props.PatchProject]
	if !ok {
		return "", nil, "", fmt.Errorf("Unknown patch project %q", props.PatchProject)
	}
	topRepoUrl := patchRepoUrl
	if props.TryJobRepo != "" {
		topRepoUrl = props.TryJobRepo
	}
	r, ok := t.rm[topRepoUrl]
	if !ok {
		return "", nil, "", fmt.Errorf("Unknown repo %q", topRepoUrl)
	}
	return topRepoUrl, r, patchRepoUrl, nil
}

func (t *TryJobIntegrator) getRevision(repo *repograph.Graph, revision string, issue int64) (string, error) {
	revision = strings.TrimPrefix(revision, "origin/")
	if revision == "" || revision == "HEAD" {
		// Obtain the branch name from Gerrit, then use the head of that branch.
		changeInfo, err := t.gerrit.GetIssueProperties(issue)
		if err != nil {
			return "", fmt.Errorf("Failed to get ChangeInfo: %s", err)
		}
		revision = changeInfo.Branch
	}
	c := repo.Get(revision)
	if c == nil {
		return "", fmt.Errorf("Unknown revision %s", revision)
	}
	return c.Hash, nil
}

func (t *TryJobIntegrator) localCancelJobs(jobs []*types.Job) error {
	for _, j := range jobs {
		j.BuildbucketLeaseKey = 0
		j.Status = types.JOB_STATUS_CANCELED
		j.Finished = time.Now()
	}
	return t.db.PutJobsInChunks(jobs)
}

func (t *TryJobIntegrator) remoteCancelBuild(id int64, msg string) error {
	sklog.Warningf("Canceling Buildbucket build %d. Reason: %s", id, msg)
	message := struct {
		Message string `json:"message"`
	}{
		Message: msg,
	}
	b, err := json.Marshal(&message)
	if err != nil {
		return err
	}
	resp, err := t.bb.Cancel(id, &buildbucket_api.ApiCancelRequestBodyMessage{
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

func (t *TryJobIntegrator) tryLeaseBuild(id int64, now time.Time) (int64, error) {
	expiration := now.Add(LEASE_DURATION_INITIAL).Unix() * 1000000
	sklog.Infof("Attempting to lease build %d", id)
	resp, err := t.bb.Lease(id, &buildbucket_api.ApiLeaseRequestBodyMessage{
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

func (t *TryJobIntegrator) getJobToSchedule(ctx context.Context, b *buildbucket_api.ApiCommonBuildMessage, now time.Time) (*types.Job, error) {
	// Parse the build parameters.
	var params buildbucket.Parameters
	if err := json.NewDecoder(strings.NewReader(b.ParametersJson)).Decode(&params); err != nil {
		return nil, t.remoteCancelBuild(b.Id, fmt.Sprintf("Invalid parameters_json: %s;\n\n%s", err, b.ParametersJson))
	}

	// Obtain and validate the RepoState.
	topRepoUrl, topRepoGraph, patchRepoUrl, err := t.getRepo(&params.Properties)
	if err != nil {
		return nil, t.remoteCancelBuild(b.Id, fmt.Sprintf("Unable to find repo: %s", err))
	}
	revision, err := t.getRevision(topRepoGraph, params.Properties.Revision, int64(params.Properties.GerritIssue))
	if err != nil {
		return nil, t.remoteCancelBuild(b.Id, fmt.Sprintf("Invalid revision: %s", err))
	}
	server := params.Properties.Gerrit
	issue := params.Properties.GerritIssue
	patchset := params.Properties.GerritPatchset
	if params.Properties.PatchStorage == "gerrit" {
		psSplit := strings.Split(patchset, "/")
		patchset = psSplit[len(psSplit)-1]
	} else {
		return nil, t.remoteCancelBuild(b.Id, fmt.Sprintf("Invalid patch storage: %s", params.Properties.PatchStorage))
	}
	rs := types.RepoState{
		Patch: types.Patch{
			Server:    server,
			Issue:     fmt.Sprintf("%d", issue),
			PatchRepo: patchRepoUrl,
			Patchset:  patchset,
		},
		Repo:     topRepoUrl,
		Revision: revision,
	}
	if !rs.Valid() || !rs.IsTryJob() {
		return nil, t.remoteCancelBuild(b.Id, fmt.Sprintf("Invalid RepoState: %s", rs))
	}

	// Create a Job.
	j, err := t.taskCfgCache.MakeJob(ctx, rs, params.BuilderName)
	if err != nil {
		return nil, t.remoteCancelBuild(b.Id, fmt.Sprintf("Failed to obtain JobSpec: %s; \n\n%v", err, params))
	}

	// Determine if this is a manual retry of a previously-run try job. If
	// so, set IsForce to ensure that we don't immediately de-duplicate all
	// of its tasks.
	prevJobs, err := t.jCache.GetJobsByRepoState(j.Name, j.RepoState)
	if err != nil {
		return nil, err
	}
	if len(prevJobs) > 0 {
		j.IsForce = true
	}

	// Attempt to lease the build.
	leaseKey, err := t.tryLeaseBuild(b.Id, now)
	if err != nil {
		// TODO(borenet): Buildbot cancels the build in this case.
		// Should we do that too?
		return nil, err
	}

	// Update and return the Job.
	j.BuildbucketBuildId = b.Id
	j.BuildbucketLeaseKey = leaseKey

	return j, nil
}

func (t *TryJobIntegrator) Poll(ctx context.Context, now time.Time) error {
	if err := t.updateCaches(); err != nil {
		return err
	}

	// Grab all of the pending Builds from Buildbucket.
	// TODO(borenet): Buildbot maintains a maximum lease count. Should we do
	// that too?
	cursor := ""
	jobs := []*types.Job{}
	errs := []error{}
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
		for _, b := range resp.Builds {
			j, err := t.getJobToSchedule(ctx, b, now)
			if err != nil {
				errs = append(errs, err)
			} else if j != nil {
				jobs = append(jobs, j)
			}
		}
		cursor = resp.NextCursor
		if cursor == "" {
			break
		}
	}

	// Insert Jobs into the database.
	insertedJobs := make([]*types.Job, 0, len(jobs))
	if len(jobs) > 0 {
		if err := util.ChunkIter(len(jobs), firestore.MAX_TRANSACTION_DOCS, func(i, j int) error {
			if err := t.db.PutJobs(jobs[i:j]); err != nil {
				return err
			}
			insertedJobs = append(insertedJobs, jobs[i:j]...)
			return nil
		}); err != nil {
			errs = append(errs, err)
		}
	}

	// Since Jobs may consist of multiple Tasks, we consider them to be
	// "started" as soon as we've picked them up.
	// TODO(borenet): Sending "started" notifications after inserting the
	// new Jobs into the database puts us at risk of never sending the
	// notification if the process is interrupted. However, we need to
	// include the Job ID with the notification, so we have to insert the
	// Job into the DB first.
	cancelJobs := []*types.Job{}
	for _, j := range insertedJobs {
		if err := t.jobStarted(j); err != nil {
			errs = append(errs, err)
			cancelJobs = append(cancelJobs, j)
		}
	}
	if len(cancelJobs) > 0 {
		if err := t.localCancelJobs(cancelJobs); err != nil {
			errs = append(errs, err)
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
	resp, err := t.bb.Start(j.BuildbucketBuildId, &buildbucket_api.ApiStartRequestBodyMessage{
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
		resp, err := t.bb.Succeed(j.BuildbucketBuildId, &buildbucket_api.ApiSucceedRequestBodyMessage{
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
		resp, err := t.bb.Fail(j.BuildbucketBuildId, &buildbucket_api.ApiFailRequestBodyMessage{
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
