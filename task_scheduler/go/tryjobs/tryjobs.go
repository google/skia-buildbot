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

	pubsub_api "cloud.google.com/go/pubsub"
	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/go-multierror"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	buildbucket_api "go.chromium.org/luci/common/api/buildbucket/buildbucket/v1"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/pubsub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/cacher"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/protobuf/encoding/prototext"
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
	buildbucketBucket  string
	buildbucketTarget  string
	chr                cacher.Cacher
	db                 db.JobDB
	gerrit             gerrit.GerritInterface
	host               string
	jCache             cache.JobCache
	projectRepoMapping map[string]string
	pubsub             pubsub.Client
	rm                 repograph.Map
	taskCfgCache       task_cfg_cache.TaskCfgCache
}

// NewTryJobIntegrator returns a TryJobIntegrator instance.
func NewTryJobIntegrator(ctx context.Context, buildbucketAPIURL, buildbucketTarget, buildbucketBucket, host string, c *http.Client, d db.JobDB, jCache cache.JobCache, projectRepoMapping map[string]string, rm repograph.Map, taskCfgCache task_cfg_cache.TaskCfgCache, chr cacher.Cacher, gerrit gerrit.GerritInterface, pubsubClient pubsub.Client) (*TryJobIntegrator, error) {
	bb, err := buildbucket_api.New(c)
	if err != nil {
		return nil, err
	}
	bb.BasePath = buildbucketAPIURL
	rv := &TryJobIntegrator{
		bb:                 bb,
		bb2:                buildbucket.NewClient(c),
		buildbucketBucket:  buildbucketBucket,
		buildbucketTarget:  buildbucketTarget,
		db:                 d,
		chr:                chr,
		gerrit:             gerrit,
		host:               host,
		jCache:             jCache,
		projectRepoMapping: projectRepoMapping,
		pubsub:             pubsubClient,
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
	go t.startJobsLoop(ctx)
}

// getActiveTryJobs returns the active (started but not yet marked as finished
// in Buildbucket) tryjobs.
func (t *TryJobIntegrator) getActiveTryJobs(ctx context.Context) ([]*types.Job, error) {
	if err := t.jCache.Update(ctx); err != nil {
		return nil, err
	}
	jobs := t.jCache.GetAllCachedJobs()
	rv := []*types.Job{}
	for _, job := range jobs {
		if (job.BuildbucketLeaseKey != 0 || job.BuildbucketToken != "") && job.Status != types.JOB_STATUS_REQUESTED {
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
	unfinishedV1 := make([]*types.Job, 0, len(jobs))
	unfinishedV2 := make([]*types.Job, 0, len(jobs))
	for _, j := range jobs {
		if j.Done() {
			finished = append(finished, j)
		} else if isBBv2(j) {
			unfinishedV2 = append(unfinishedV2, j)
		} else {
			unfinishedV1 = append(unfinishedV1, j)
		}
	}

	// Send heartbeats for unfinished Jobs.
	var heartbeatErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		heartbeatErr = t.sendHeartbeats(ctx, unfinishedV1)
	}()

	var pubsubErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		pubsubErr = t.sendPubsubUpdates(ctx, unfinishedV2)
	}()

	// Send updates for finished Jobs, empty the lease keys to mark them
	// as inactive in the DB.
	errs := []error{}
	insert := make([]*types.Job, 0, len(finished))
	for _, j := range finished {
		if err := t.jobFinished(ctx, j); err != nil {
			errs = append(errs, err)
		} else {
			j.BuildbucketLeaseKey = 0
			j.BuildbucketToken = ""
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
	if pubsubErr != nil {
		errs = append(errs, pubsubErr)
	}

	if len(errs) > 0 {
		return skerr.Fmt("Failed to update jobs; got errors: %v", errs)
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

// isBBv2 returns true iff the Job was triggered using Buildbucket V2.
func isBBv2(j *types.Job) bool {
	return j.BuildbucketPubSubTopic != ""
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
			errs = append(errs, skerr.Wrapf(err, "failed to send heartbeat request"))
			return
		}
		// Results should follow the same ordering as the jobs we sent.
		if len(resp.Results) != len(jobs) {
			errs = append(errs, skerr.Fmt("Heartbeat result has incorrect number of jobs (%d vs %d)", len(resp.Results), len(jobs)))
			return
		}
		var cancelJobs []*types.Job
		var cancelReasons []string
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
				cancelReasons = append(cancelReasons, fmt.Sprintf("Buildbucket rejected heartbeat with: %s", result.Error.Reason))
			}
		}
		if len(cancelJobs) > 0 {
			sklog.Infof("Canceling %d jobs", len(cancelJobs))
			if err := t.localCancelJobs(ctx, cancelJobs, cancelReasons); err != nil {
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
		return skerr.Fmt("got errors sending heartbeats: %v", errs)
	}
	return nil
}

// sendPubsubUpdates sends updates to Buildbucket via Pub/Sub for in-progress
// Jobs.
func (t *TryJobIntegrator) sendPubsubUpdates(ctx context.Context, jobs []*types.Job) error {
	g := multierror.Group{}
	for _, job := range jobs {
		job := job // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			update := &buildbucketpb.BuildTaskUpdate{
				BuildId: strconv.FormatInt(job.BuildbucketBuildId, 10),
				Task: &buildbucketpb.Task{
					Id: &buildbucketpb.TaskID{
						Target: t.buildbucketTarget,
						Id:     job.Id,
					},
					Link:     job.URL(t.host),
					Status:   jobStatusToBuildbucketStatus(job.Status),
					UpdateId: now.Now(ctx).UnixNano(),
				},
			}
			b, err := prototext.Marshal(update)
			if err != nil {
				return skerr.Wrapf(err, "failed to encode BuildTaskUpdate")
			}
			_, err = t.pubsub.Topic(job.BuildbucketPubSubTopic).Publish(ctx, &pubsub_api.Message{
				Data: b,
			}).Get(ctx)
			return err
		})
	}
	return g.Wait().ErrorOrNil()
}

// getRepo returns the repo information associated with the given URL.
func (t *TryJobIntegrator) getRepo(repoUrl string) (*repograph.Graph, error) {
	r, ok := t.rm[repoUrl]
	if !ok {
		return nil, skerr.Fmt("unknown repo %q", repoUrl)
	}
	return r, nil
}

// getRevision obtains the branch name from Gerrit, then retrieves and returns
// the current commit at the head of that branch.
func (t *TryJobIntegrator) getRevision(ctx context.Context, repo *repograph.Graph, issue string) (string, error) {
	issueNum, err := strconv.ParseInt(issue, 10, 64)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to parse issue number")
	}
	changeInfo, err := t.gerrit.GetIssueProperties(ctx, issueNum)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to get ChangeInfo")
	}
	c := repo.Get(changeInfo.Branch)
	if c == nil {
		return "", skerr.Fmt("Unknown branch %s", changeInfo.Branch)
	}
	return c.Hash, nil
}

func (t *TryJobIntegrator) localCancelJobs(ctx context.Context, jobs []*types.Job, reasons []string) error {
	if len(jobs) != len(reasons) {
		return skerr.Fmt("expected jobs and reasons to have the same length")
	}
	for idx, j := range jobs {
		j.BuildbucketLeaseKey = 0
		j.Status = types.JOB_STATUS_CANCELED
		j.StatusDetails = reasons[idx]
		j.Finished = now.Now(ctx)
	}
	if err := t.db.PutJobsInChunks(ctx, jobs); err != nil {
		return err
	}
	t.jCache.AddJobs(jobs)
	return nil
}

func (t *TryJobIntegrator) remoteCancelV1Build(buildId int64, msg string) error {
	sklog.Warningf("Canceling Buildbucket build %d. Reason: %s", buildId, msg)
	message := struct {
		Message string `json:"message"`
	}{
		Message: util.Truncate(msg, maxCancelReasonLen),
	}
	b, err := json.Marshal(&message)
	if err != nil {
		return err
	}
	resp, err := t.bb.Cancel(buildId, &buildbucket_api.LegacyApiCancelRequestBodyMessage{
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

func (t *TryJobIntegrator) tryLeaseV1Build(ctx context.Context, id int64) (int64, *buildbucket_api.LegacyApiErrorMessage, error) {
	expiration := now.Now(ctx).Add(LEASE_DURATION_INITIAL).Unix() * secondsToMicros
	sklog.Infof("Attempting to lease build %d", id)
	resp, err := t.bb.Lease(id, &buildbucket_api.LegacyApiLeaseRequestBodyMessage{
		LeaseExpirationTs: expiration,
	}).Do()
	if err != nil {
		return 0, nil, skerr.Wrapf(err, "failed request to lease buildbucket build %d", id)
	}
	leaseKey := int64(0)
	if resp.Build != nil {
		leaseKey = resp.Build.LeaseKey
	}
	return leaseKey, resp.Error, nil
}

func (t *TryJobIntegrator) insertNewJobV1(ctx context.Context, buildId int64) error {
	// Get the build details from the v2 API.
	build, err := t.bb2.GetBuild(ctx, buildId)
	if err != nil {
		return skerr.Wrapf(err, "failed to retrieve build %d", buildId)
	}
	if build.Status != buildbucketpb.Status_SCHEDULED {
		sklog.Warningf("Found build %d with status: %s; attempting to lease anyway, to trigger the fix in Buildbucket.", build.Id, build.Status)
		_, bbError, err := t.tryLeaseV1Build(ctx, buildId)
		if err != nil || bbError != nil {
			// This is expected.
			return nil
		}
		sklog.Warningf("Unexpectedly able to lease build %d with status %s; canceling it.", buildId, build.Status)
		if err := t.remoteCancelV1Build(buildId, fmt.Sprintf("Unexpected status %s", build.Status)); err != nil {
			sklog.Warningf("Failed to cancel errant build %d", buildId)
			return nil
		}
	}

	// Obtain and validate the RepoState.
	if build.Input.GerritChanges == nil || len(build.Input.GerritChanges) != 1 {
		return t.remoteCancelV1Build(buildId, fmt.Sprintf("Invalid Build %d: input should have exactly one GerritChanges: %+v", buildId, build.Input))
	}
	gerritChange := build.Input.GerritChanges[0]
	repoUrl, ok := t.projectRepoMapping[gerritChange.Project]
	if !ok {
		return t.remoteCancelV1Build(buildId, fmt.Sprintf("Unknown patch project %q", gerritChange.Project))
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
		Repo: repoUrl,
		// We can't fill this out without retrieving the Gerrit ChangeInfo and
		// resolving the branch to a commit hash. Defer that work until later.
		Revision: "",
	}
	requested, err := ptypes.Timestamp(build.CreateTime)
	if err != nil {
		return t.remoteCancelV1Build(buildId, fmt.Sprintf("Failed to convert timestamp for %d: %s", build.Id, err))
	}
	j := &types.Job{
		Name:               build.Builder.Builder,
		BuildbucketBuildId: buildId,
		Requested:          firestore.FixTimestamp(requested.UTC()),
		Created:            firestore.FixTimestamp(now.Now(ctx)),
		RepoState:          rs,
		Status:             types.JOB_STATUS_REQUESTED,
	}
	if !j.Requested.Before(j.Created) {
		sklog.Errorf("Try job created time %s is before requested time %s! Setting equal.", j.Created, j.Requested)
		j.Requested = j.Created.Add(-firestore.TS_RESOLUTION)
	}
	// Attempt to lease the build.
	leaseKey, bbError, err := t.tryLeaseV1Build(ctx, j.BuildbucketBuildId)
	if err != nil {
		return skerr.Wrapf(err, "failed to lease build %d", j.BuildbucketBuildId)
	} else if bbError != nil {
		// Note: we're just assuming that the only reason Buildbucket would
		// return an error is that the Build has been canceled. While this
		// is the most likely reason, others are possible, and we may gain
		// some information by reading the error and behaving accordingly.
		return t.remoteCancelV1Build(buildId, fmt.Sprintf("Buildbucket refused lease with %q", bbError.Message))
	} else if leaseKey == 0 {
		return t.remoteCancelV1Build(buildId, "Buildbucket returned zero lease key")
	}
	j.BuildbucketLeaseKey = leaseKey

	if err := t.db.PutJob(ctx, j); err != nil {
		return t.remoteCancelV1Build(j.BuildbucketBuildId, fmt.Sprintf("Failed to insert Job into the DB: %s", err))
	}
	t.jCache.AddJobs([]*types.Job{j})
	return nil
}

func (t *TryJobIntegrator) startJobsLoop(ctx context.Context) {
	// The code in startJob makes the assumption that we'll come back to the job
	// and try again if requests to Buildbucket fail for transient-looking
	// reasons. ModifiedJobsCh only changes when jobs are modified in the
	// database, so we also need a periodic poll to ensure that we retry any
	// jobs we failed to start on the first try. A 5-minute period was chosen
	// because it is short enough not to cause significant lag in handling try
	// jobs but hopefully long enough that any transient errors are resolved
	// before we try again.
	jobsCh := t.db.ModifiedJobsCh(ctx)
	ticker := time.NewTicker(5 * time.Minute)
	tickCh := ticker.C
	doneCh := ctx.Done()
	for {
		select {
		case jobs := <-jobsCh:
			for _, job := range jobs {
				if job.Status != types.JOB_STATUS_REQUESTED {
					continue
				}
				sklog.Infof("Found job %s (build %d) via modified jobs channel", job.Id, job.BuildbucketBuildId)
				if err := t.startJob(ctx, job); err != nil {
					sklog.Errorf("failed to start job: %s", err)
				}
			}
		case <-tickCh:
			jobs, err := t.jCache.RequestedJobs()
			if err != nil {
				sklog.Errorf("failed retrieving Jobs: %s", err)
			} else {
				for _, job := range jobs {
					sklog.Infof("Found job %s (build %d) via periodic DB poll", job.Id, job.BuildbucketBuildId)
					if err := t.startJob(ctx, job); err != nil {
						sklog.Errorf("failed to start job: %s", err)
					}
				}
			}
		case <-doneCh:
			ticker.Stop()
			return
		}
	}
}

func (t *TryJobIntegrator) startJob(ctx context.Context, job *types.Job) error {
	sklog.Infof("Starting job %s (build %d); lease key: %d", job.Id, job.BuildbucketBuildId, job.BuildbucketLeaseKey)
	startJobHelper := func() error {
		repoGraph, err := t.getRepo(job.Repo)
		if err != nil {
			return skerr.Wrapf(err, "unable to find repo %s", job.Repo)
		}
		if job.Revision == "" {
			// Derive the revision from the branch specified by the Gerrit CL.
			revision, err := t.getRevision(ctx, repoGraph, job.Issue)
			if err != nil {
				return skerr.Wrapf(err, "failed to find base revision for issue %s in %s", job.Issue, job.Repo)
			}
			job.Revision = revision
		} else {
			// Resolve the already-set revision (which might be a branch name)
			// to a commit hash.
			c := repoGraph.Get(job.Revision)
			if c == nil {
				return skerr.Fmt("Unknown revision %s", job.Revision)
			}
			job.Revision = c.Hash
		}
		if !job.RepoState.Valid() || !job.RepoState.IsTryJob() || skipRepoState(job.RepoState) {
			return skerr.Fmt("invalid RepoState: %s", job.RepoState)
		}

		// Create a Job.
		if _, err := t.chr.GetOrCacheRepoState(ctx, job.RepoState); err != nil {
			return skerr.Wrapf(err, "failed to obtain JobSpec")
		}
		cfg, cachedErr, err := t.taskCfgCache.Get(ctx, job.RepoState)
		if err != nil {
			return err
		}
		if cachedErr != nil {
			return cachedErr
		}
		spec, ok := cfg.Jobs[job.Name]
		if !ok {
			return skerr.Fmt("no such job: %s", job.Name)
		}
		deps, err := spec.GetTaskSpecDAG(cfg)
		if err != nil {
			return skerr.Wrap(err)
		}
		job.Dependencies = deps
		job.Tasks = map[string][]*types.TaskSummary{}

		// Determine if this is a manual retry of a previously-run try job. If
		// so, set IsForce to ensure that we don't immediately de-duplicate all
		// of its tasks.
		prevJobs, err := t.jCache.GetJobsByRepoState(job.Name, job.RepoState)
		if err != nil {
			return skerr.Wrap(err)
		}
		if len(prevJobs) > 0 {
			job.IsForce = true
		}
		return nil
	}

	if err := startJobHelper(); err != nil {
		sklog.Infof("Failed to start job %s (build %d) with: %s", job.Id, job.BuildbucketBuildId, err)
		job.Status = types.JOB_STATUS_MISHAP
		job.StatusDetails = util.Truncate(fmt.Sprintf("Failed to start Job: %s", skerr.Unwrap(err)), 1024)
	} else {
		job.Status = types.JOB_STATUS_IN_PROGRESS

		// Notify Buildbucket that the Job has started.
		bbToken, bbError, err := t.jobStarted(ctx, job)
		if err != nil {
			return skerr.Wrapf(err, "failed to send job-started notification")
		} else if bbError != nil {
			// Note: we're just assuming that the only reason Buildbucket would
			// return an error is that the Build has been canceled. While this
			// is the most likely reason, others are possible, and we may gain
			// some information by reading the error and behaving accordingly.
			cancelReason := fmt.Sprintf("Buildbucket rejected Start with: %s", bbError.Reason)
			if cancelErr := t.localCancelJobs(ctx, []*types.Job{job}, []string{cancelReason}); cancelErr != nil {
				return skerr.Wrapf(cancelErr, "failed to start build %d with %q and failed to cancel job", job.BuildbucketBuildId, bbError.Message)
			} else {
				return skerr.Fmt("failed to start build %d with %q", job.BuildbucketBuildId, bbError.Message)
			}
		} else if bbToken != "" {
			job.BuildbucketToken = bbToken
		}
	}

	// Update the job and insert into the DB.
	if err := t.db.PutJob(ctx, job); err != nil {
		return skerr.Wrapf(err, "failed to insert Job into the DB")
	}
	t.jCache.AddJobs([]*types.Job{job})
	return nil
}

func (t *TryJobIntegrator) Poll(ctx context.Context) error {
	if err := t.jCache.Update(ctx); err != nil {
		return err
	}

	// Grab all of the pending Builds from Buildbucket.
	cursor := ""
	errs := []error{}
	var mtx sync.Mutex
	for {
		sklog.Infof("Running 'peek' on %s", t.buildbucketBucket)
		resp, err := t.bb.Peek().Bucket(t.buildbucketBucket).MaxBuilds(PEEK_MAX_BUILDS).StartCursor(cursor).Do()
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
				if err := t.insertNewJobV1(ctx, b.Id); err != nil {
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
		return skerr.Fmt("got errors loading builds from Buildbucket: %v", errs)
	}

	return nil
}

// jobStarted notifies Buildbucket that the given Job has started. Returns the
// Buildbucket token returned by Buildbucket, any error object returned by
// Buildbucket (eg. if the Build has been canceled), or any error which occurred
// when attempting the request.
func (t *TryJobIntegrator) jobStarted(ctx context.Context, j *types.Job) (string, *buildbucket_api.LegacyApiErrorMessage, error) {
	if isBBv2(j) {
		sklog.Infof("bb2.Start for job %s (build %d)", j.Id, j.BuildbucketBuildId)
		updateToken, err := t.bb2.StartBuild(ctx, j.BuildbucketBuildId, j.Id, j.BuildbucketToken)
		return updateToken, nil, skerr.Wrap(err)
	} else {
		sklog.Infof("bb.Start for job %s (build %d)", j.Id, j.BuildbucketBuildId)
		resp, err := t.bb.Start(j.BuildbucketBuildId, &buildbucket_api.LegacyApiStartRequestBodyMessage{
			LeaseKey: j.BuildbucketLeaseKey,
			Url:      j.URL(t.host),
		}).Do()
		if err != nil {
			return "", nil, err
		}
		return "", resp.Error, nil
	}
}

// buildSucceededV1 sends a success notification to Buildbucket.
func (t *TryJobIntegrator) buildSucceededV1(j *types.Job) error {
	sklog.Infof("bb.Succeed for job %s (build %d)", j.Id, j.BuildbucketBuildId)
	b, err := json.Marshal(struct {
		Job *types.Job `json:"job"`
	}{
		Job: j,
	})
	if err != nil {
		return err
	}
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
	return nil
}

// buildFailed sends a failure notification to Buildbucket.
func (t *TryJobIntegrator) buildFailed(j *types.Job) error {
	b, err := json.Marshal(struct {
		Job *types.Job `json:"job"`
	}{
		Job: j,
	})
	if err != nil {
		return err
	}
	failureReason := "BUILD_FAILURE"
	if j.Status == types.JOB_STATUS_MISHAP {
		failureReason = "INFRA_FAILURE"
	}
	sklog.Infof("bb.Fail for job %s (build %d)", j.Id, j.BuildbucketBuildId)
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
	return nil
}

func (t *TryJobIntegrator) updateBuild(ctx context.Context, j *types.Job) error {
	sklog.Infof("bb2.UpdateBuild for job %s (build %d)", j.Id, j.BuildbucketBuildId)
	return t.bb2.UpdateBuild(ctx, jobToBuildV2(j), j.BuildbucketToken)
}

// jobFinished notifies Buildbucket that the given Job has finished.
func (t *TryJobIntegrator) jobFinished(ctx context.Context, j *types.Job) error {
	if !j.Done() {
		return skerr.Fmt("JobFinished called for unfinished Job!")
	}
	if isBBv2(j) {
		return t.updateBuild(ctx, j)
	} else if j.Status == types.JOB_STATUS_SUCCESS {
		return t.buildSucceededV1(j)
	} else {
		return t.buildFailed(j)
	}
}

// skipRepoState determines whether we should skip try jobs for this RepoState,
// eg. problematic CLs.
func skipRepoState(rs types.RepoState) bool {
	// Invalid hash; this causes hours of wasted sync times.
	if rs.Issue == "527502" && rs.Patchset == "1" {
		return true
	}
	return false
}

func jobStatusToBuildbucketStatus(status types.JobStatus) buildbucketpb.Status {
	switch status {
	case types.JOB_STATUS_CANCELED:
		return buildbucketpb.Status_CANCELED
	case types.JOB_STATUS_FAILURE:
		return buildbucketpb.Status_FAILURE
	case types.JOB_STATUS_IN_PROGRESS:
		return buildbucketpb.Status_STARTED
	case types.JOB_STATUS_MISHAP:
		return buildbucketpb.Status_INFRA_FAILURE
	case types.JOB_STATUS_REQUESTED:
		return buildbucketpb.Status_SCHEDULED
	case types.JOB_STATUS_SUCCESS:
		return buildbucketpb.Status_SUCCESS
	default:
		return buildbucketpb.Status_STATUS_UNSPECIFIED
	}
}

// jobToBuildV2 converts a Job to a Buildbucket V2 Build to be used with
// UpdateBuild.
func jobToBuildV2(job *types.Job) *buildbucketpb.Build {
	status := jobStatusToBuildbucketStatus(job.Status)

	// Note: There are other fields we could fill in, but I'm not sure they
	// would provide any value since we don't actually use Buildbucket builds
	// for anything.
	return &buildbucketpb.Build{
		Id: job.BuildbucketBuildId,
		Output: &buildbucketpb.Build_Output{
			Status:          status,
			SummaryMarkdown: job.StatusDetails,
		},
	}
}
