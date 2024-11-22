package tryjobs

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	pubsub_api "cloud.google.com/go/pubsub"
	"github.com/hashicorp/go-multierror"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/cleanup"
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
	"go.skia.org/infra/task_scheduler/go/job_creation/buildbucket_taskbackend"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

/*
	Integration of the Task Scheduler with Buildbucket for try jobs.
*/

const (
	// Buildbucket buckets used for try jobs.
	BUCKET_PRIMARY  = "skia.primary"
	BUCKET_INTERNAL = "skia.internal"
	BUCKET_TESTING  = "skia.testing"

	// How often to send updates to Buildbucket.
	UPDATE_INTERVAL = 30 * time.Second

	// We attempt to renew leases in batches. This is the batch size.
	LEASE_BATCH_SIZE = 200

	// We lease a build for this amount of time, and if we don't renew the
	// lease before the time is up, the build resets to "scheduled" status
	// and becomes available for leasing again.
	LEASE_DURATION = time.Hour

	// We use a shorter initial lease duration in case we succeed in leasing
	// a build but fail to insert the associated Job into the DB, eg.
	// because the scheduler was interrupted.
	LEASE_DURATION_INITIAL = 30 * time.Minute

	// How many pending builds to read from the bucket at a time.
	PEEK_MAX_BUILDS = 50

	// How often to poll Buildbucket for newly-scheduled builds.
	POLL_INTERVAL = 10 * time.Second

	// How often to run the Buildbucket cleanup loop.
	CLEANUP_INTERVAL = 15 * time.Minute

	// We'll attempt to clean up Buildbucket builds which are older than this.
	CLEANUP_AGE_THRESHOLD = 3 * time.Hour

	// buildAlreadyStartedErr is a substring of the error message returned by
	// Buildbucket when we call StartBuild more than once for the same build.
	buildAlreadyStartedErr = "has recorded another StartBuild with request id"

	// buildAlreadyFinishedErr is a substring of the error message returned by
	// Buildbucket when we call UpdateBuild after the build has finished.
	buildAlreadyFinishedErr = "cannot update an ended build"

	metricJobQueueLength = "task_scheduler_jc_job_queue_length"
)

var (
	pubsubRegex = regexp.MustCompile(`^projects\/([a-zA-Z_-]+)\/topics\/([a-zA-Z_-]+)$`)
)

// TryJobIntegrator is responsible for communicating with Buildbucket to
// trigger try jobs and report their results.
type TryJobIntegrator struct {
	bb2                buildbucket.BuildBucketInterface
	buildbucketBucket  string
	buildbucketProject string
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
func NewTryJobIntegrator(ctx context.Context, buildbucketProject, buildbucketTarget, buildbucketBucket, host string, c *http.Client, d db.JobDB, jCache cache.JobCache, projectRepoMapping map[string]string, rm repograph.Map, taskCfgCache task_cfg_cache.TaskCfgCache, chr cacher.Cacher, gerrit gerrit.GerritInterface, pubsubClient pubsub.Client) (*TryJobIntegrator, error) {
	rv := &TryJobIntegrator{
		bb2:                buildbucket.NewClient(c),
		buildbucketBucket:  buildbucketBucket,
		buildbucketProject: buildbucketProject,
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
	lvCleanup := metrics2.NewLiveness("last_successfull_buildbucket_cleanup")
	cleanup.Repeat(CLEANUP_INTERVAL, func(_ context.Context) {
		// Explicitly ignore the passed-in context; this allows us to
		// finish leasing jobs from Buildbucket and inserting them into
		// the DB even if the context is canceled, which helps to
		// prevent inconsistencies between Buildbucket and the Task
		// Scheduler DB.
		ctx := context.Background()
		if err := t.buildbucketCleanup(ctx); err != nil {
			sklog.Errorf("Failed to clean up old Buildbucket builds: %s", err)
		} else {
			lvCleanup.Reset()
		}
	}, nil)
	go t.startJobsLoop(ctx)
}

// getActiveTryJobs returns the active (started but not yet marked as finished
// in Buildbucket) tryjobs.
func (t *TryJobIntegrator) getActiveTryJobs(ctx context.Context) ([]*types.Job, error) {
	ctx, span := trace.StartSpan(ctx, "getActiveTryJobs")
	defer span.End()
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
	ctx, span := trace.StartSpan(ctx, "updateJobs")
	defer span.End()

	// Get all Jobs associated with in-progress Buildbucket builds.
	jobs, err := t.getActiveTryJobs(ctx)
	if err != nil {
		return err
	}

	// Divide up finished and unfinished Jobs.
	finished := make([]*types.Job, 0, len(jobs))
	unfinishedV2 := make([]*types.Job, 0, len(jobs))
	for _, j := range jobs {
		if j.Done() {
			finished = append(finished, j)
		} else if isBBv2(j) {
			unfinishedV2 = append(unfinishedV2, j)
		} else {
			sklog.Errorf("Build %d (job %s) looks like a Buildbucket V1 build, which is no longer supported.", j.BuildbucketBuildId, j.Id)
		}
	}
	sklog.Infof("Have %d active try jobs; %d finished, %d unfinished (v2)", len(jobs), len(finished), len(unfinishedV2))

	// Send heartbeats for unfinished Jobs.
	var wg sync.WaitGroup
	var pubsubErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		pubsubErr = t.sendPubsubUpdates(ctx, unfinishedV2)
	}()

	// Send updates for finished Jobs, empty the lease keys to mark them
	// as inactive in the DB.
	errs := t.jobsFinished(ctx, finished)

	wg.Wait()
	if pubsubErr != nil {
		errs = append(errs, pubsubErr)
	}

	if len(errs) > 0 {
		return skerr.Fmt("Failed to update jobs; got errors: %v", errs)
	}
	sklog.Infof("Finished sending updates for jobs.")
	return nil
}

// isBBv2 returns true iff the Job was triggered using Buildbucket V2.
func isBBv2(j *types.Job) bool {
	return j.BuildbucketPubSubTopic != ""
}

// sendPubSub sends an update to Buildbucket via Pub/Sub for a single Job.
func (t *TryJobIntegrator) sendPubSub(ctx context.Context, job *types.Job) error {
	ctx, span := trace.StartSpan(ctx, "sendPubSub")
	defer span.End()

	update := &buildbucketpb.BuildTaskUpdate{
		BuildId: strconv.FormatInt(job.BuildbucketBuildId, 10),
		Task:    buildbucket_taskbackend.JobToBuildbucketTask(ctx, job, t.buildbucketTarget, t.host),
	}
	msgBinary, err := proto.Marshal(update)
	if err != nil {
		return skerr.Wrapf(err, "failed to encode BuildTaskUpdate for job %s (build %d)", job.Id, job.BuildbucketBuildId)
	}
	msgText, err := prototext.Marshal(update)
	if err != nil {
		return skerr.Wrapf(err, "failed to encode BuildTaskUpdate for job %s (build %d)", job.Id, job.BuildbucketBuildId)
	}

	// Parse the project and topic names from the fully-qualified topic.
	project := t.pubsub.Project()
	topic := job.BuildbucketPubSubTopic
	m := pubsubRegex.FindStringSubmatch(job.BuildbucketPubSubTopic)
	if len(m) == 3 {
		project = m[1]
		topic = m[2]
	}
	// Publish the message.
	sklog.Infof("Sending pubsub message for job %s (build %d): %s", job.Id, job.BuildbucketBuildId, string(msgText))
	_, err = t.pubsub.TopicInProject(topic, project).Publish(ctx, &pubsub_api.Message{
		Data: msgBinary,
	}).Get(ctx)
	return skerr.Wrapf(err, "failed to send pubsub update for job %s (build %d)", job.Id, job.BuildbucketBuildId)
}

// sendPubsubUpdates sends updates to Buildbucket via Pub/Sub for in-progress
// Jobs.
func (t *TryJobIntegrator) sendPubsubUpdates(ctx context.Context, jobs []*types.Job) error {
	ctx, span := trace.StartSpan(ctx, "sendPubsubUpdates")
	defer span.End()

	g := multierror.Group{}
	for _, job := range jobs {
		job := job // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			return t.sendPubSub(ctx, job)
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
		sklog.Warningf("Canceling job %s (build %d). Reason: %s", j.Id, j.BuildbucketBuildId, reasons[idx])
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

// findJobForBuild retrieves the Job associated with the given build. Returns
// nil, nil if no build is found.
func (t *TryJobIntegrator) findJobForBuild(ctx context.Context, id int64) (*types.Job, error) {
	end := now.Now(ctx)
	start := end.Add(-4 * 24 * time.Hour)
	foundJobs, err := t.db.SearchJobs(ctx, &db.JobSearchParams{
		BuildbucketBuildID: &id,
		TimeStart:          &start,
		TimeEnd:            &end,
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "failed searching for existing Jobs for build %d", id)
	}
	if len(foundJobs) > 0 {
		return foundJobs[0], nil
	}
	return nil, nil
}

type jobQueue struct {
	queue []*types.Job
	mtx   sync.Mutex
	m     metrics2.Int64Metric
}

func newJobQueue(rs types.RepoState) *jobQueue {
	return &jobQueue{
		queue: make([]*types.Job, 0, 50 /* approximate number of try jobs per RepoState/CL */),
		m: metrics2.GetInt64Metric(metricJobQueueLength, map[string]string{
			"repo":      rs.Repo,
			"revision":  rs.Revision,
			"issue":     rs.Issue,
			"patchset":  rs.Patchset,
			"patchrepo": rs.PatchRepo,
			"server":    rs.Server,
		}),
	}
}

func (q *jobQueue) Enqueue(job *types.Job) {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	// Prevent processing the same job multiple times if it's already queued.
	// Queue length will max out at the number of try jobs on a given CL, so
	// this should be significantly less expensive than processing the job
	// twice.
	for _, old := range q.queue {
		if old.Id == job.Id {
			sklog.Infof("Job %s (build %d) is already queued; skipping.", job.Id, job.BuildbucketBuildId)
			return
		}
	}
	q.queue = append(q.queue, job)
	q.m.Update(int64(len(q.queue)))
	sklog.Infof("Enqueued job %s (build %d) for %+v, %d others in queue", job.Id, job.BuildbucketBuildId, job.RepoState, len(q.queue))
}

func (q *jobQueue) Dequeue() *types.Job {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	rv := q.queue[0]
	q.queue = q.queue[1:]
	q.m.Update(int64(len(q.queue)))
	return rv
}

func (q *jobQueue) Len() int {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	return len(q.queue)
}

type jobQueues struct {
	queues map[types.RepoState]*jobQueue
	mtx    sync.Mutex
	workFn func(*types.Job)
}

func (q *jobQueues) Enqueue(job *types.Job) {
	sklog.Infof("Enqueue job %s (build %d): %+v", job.Id, job.BuildbucketBuildId, job.RepoState)
	q.mtx.Lock()
	defer q.mtx.Unlock()
	jobQueue, ok := q.queues[job.RepoState]
	if !ok {
		sklog.Infof("Creating new queue for job %s (build %d): %+v", job.Id, job.BuildbucketBuildId, job.RepoState)
		jobQueue = newJobQueue(job.RepoState)
		q.queues[job.RepoState] = jobQueue
	}
	jobQueue.Enqueue(job)
	if !ok {
		go func() {
			for {
				job := jobQueue.Dequeue()
				// workFn modifies the RepoState to set the actual commit hash
				// after syncing. We need to grab a copy of it pre-modification
				// so that we can delete the correct queue when finished.
				rs := job.RepoState
				sklog.Infof("Dequeue job %s (build %d): %+v", job.Id, job.BuildbucketBuildId, job.RepoState)
				q.workFn(job)

				// Lock the outer mutex before the inner one, to ensure that
				// no other thread is trying to add to this queue while we're
				// removing it. Doing so after we're finished removing the queue
				// will just add a new one, but re-adding the queue after we're
				// done is fine.
				q.mtx.Lock()
				if jobQueue.Len() == 0 {
					sklog.Infof("Deleting empty queue for job %s (build %d): %+v", job.Id, job.BuildbucketBuildId, rs)
					delete(q.queues, rs)
					util.LogErr(jobQueue.m.Delete())
					q.mtx.Unlock()
					return
				}
				q.mtx.Unlock()
			}
		}()
	}
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
	modJobsCh := t.db.ModifiedJobsCh(ctx)
	ticker := time.NewTicker(time.Minute)
	tickCh := ticker.C
	doneCh := ctx.Done()

	q := &jobQueues{
		queues: map[types.RepoState]*jobQueue{},
		workFn: func(job *types.Job) {
			if err := t.startJob(ctx, job); err != nil {
				sklog.Errorf("Failed to start job %s (build %d): %s", job.Id, job.BuildbucketBuildId, err)
			}
		},
	}
	for {
		select {
		case jobs := <-modJobsCh:
			for _, job := range jobs {
				if job.Status == types.JOB_STATUS_REQUESTED {
					sklog.Infof("Found job %s (build %d) via modified jobs channel", job.Id, job.BuildbucketBuildId)
					q.Enqueue(job)
				}
			}
		case <-tickCh:
			jobs, err := t.jCache.RequestedJobs()
			if err != nil {
				sklog.Errorf("failed retrieving Jobs: %s", err)
			} else {
				for _, job := range jobs {
					sklog.Infof("Found job %s (build %d) via periodic DB poll", job.Id, job.BuildbucketBuildId)
					q.Enqueue(job)
				}
			}
		case <-doneCh:
			ticker.Stop()
			return
		}
	}
}

func isBuildAlreadyStartedError(err error) bool {
	return err != nil && strings.Contains(err.Error(), buildAlreadyStartedErr)
}

func isBuildAlreadyFinishedError(err error) bool {
	return err != nil && strings.Contains(err.Error(), buildAlreadyFinishedErr)
}

func (t *TryJobIntegrator) startJob(ctx context.Context, job *types.Job) error {
	// We might encounter this Job via periodic polling or the query snapshot
	// iterator, or both.  We don't want to start the Job multiple times, so
	// retrieve the Job again here and ensure that we didn't already start it.
	// Note: if this is ever parallelized, we'll need to come up with an
	// alternative way to prevent double-starting jobs.
	updatedJob, err := t.db.GetJobById(ctx, job.Id)
	if err != nil {
		return skerr.Wrapf(err, "failed loading job from DB")
	}
	if updatedJob.Status != types.JOB_STATUS_REQUESTED {
		sklog.Infof("Job %s (build %d) has already started; skipping: %+v", job.Id, job.BuildbucketBuildId, job.RepoState)
		return nil
	}

	sklog.Infof("Starting job %s (build %d); lease key: %d, %+v", job.Id, job.BuildbucketBuildId, job.BuildbucketLeaseKey, job.RepoState)
	startJobHelper := func() error {
		sklog.Infof("Retrieving repo state information for job %s (build %d): %+v", job.Id, job.BuildbucketBuildId, job.RepoState)
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
		sklog.Infof("GetOrCacheRepoState for job %s (build %d): %+v", job.Id, job.BuildbucketBuildId, job.RepoState)
		if _, err := t.chr.GetOrCacheRepoState(ctx, job.RepoState); err != nil {
			return skerr.Wrapf(err, "failed to obtain JobSpec")
		}
		sklog.Infof("Reading tasks cfg for job %s (build %d): %+v", job.Id, job.BuildbucketBuildId, job.RepoState)
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
		sklog.Infof("Determining whether job %s (build %d) is a manual retry: %+v", job.Id, job.BuildbucketBuildId, job.RepoState)
		prevJobs, err := t.jCache.GetJobsByRepoState(job.Name, job.RepoState)
		if err != nil {
			return skerr.Wrap(err)
		}
		if len(prevJobs) > 0 {
			job.IsForce = true
		}
		sklog.Infof("Ready to start job %s (build %d): %+v", job.Id, job.BuildbucketBuildId, job.RepoState)
		return nil
	}

	// Run the necessary syncs, load information for the Job, etc.
	startJobErr := startJobHelper()

	// Send the StartJob notification to notify Buildbucket that the Job has
	// started. We'll check startJobErr below.
	bbToken, err := t.jobStarted(ctx, job)
	if isBuildAlreadyStartedError(err) {
		cancelReason := "StartBuild has already been called for this Job, but the Job was not correctly updated and cannot continue."
		if cancelErr := t.localCancelJobs(ctx, []*types.Job{job}, []string{cancelReason}); cancelErr != nil {
			return skerr.Wrapf(cancelErr, "failed to start job %s (build %d) with %q and failed to cancel job", job.Id, job.BuildbucketBuildId, err)
		} else {
			return skerr.Fmt("failed to start job %s (build %d) with %q", job.Id, job.BuildbucketBuildId, err)
		}
	} else if err != nil {
		return skerr.Wrapf(err, "failed to send job-started notification for job %s (build %d)", job.Id, job.BuildbucketBuildId)
	} else if bbToken != "" {
		job.BuildbucketToken = bbToken
	} else {
		sklog.Warningf("Successfully started job %s (%d) but have no Buildbucket token.", job.Id, job.BuildbucketBuildId)
	}

	// If we failed to sync, mark the Job as a mishap.
	if startJobErr != nil {
		job.Finished = now.Now(ctx)
		job.Status = types.JOB_STATUS_MISHAP
		job.StatusDetails = util.Truncate(fmt.Sprintf("Failed to start Job: %s", skerr.Unwrap(startJobErr)), 1024)
	} else {
		job.Status = types.JOB_STATUS_IN_PROGRESS
		job.Started = now.Now(ctx)
	}

	// Update the job and insert into the DB.
	if err := t.db.PutJob(ctx, job); err != nil {
		return skerr.Wrapf(err, "failed to insert Job %s (build %d) into the DB", job.Id, job.BuildbucketBuildId)
	}
	t.jCache.AddJobs([]*types.Job{job})
	if startJobErr != nil {
		sklog.Infof("Failed to start job %s (build %d) with: %s", job.Id, job.BuildbucketBuildId, startJobErr)
	} else {
		sklog.Infof("Successfully started job %s (build %d)", job.Id, job.BuildbucketBuildId)
	}
	return startJobErr
}

// jobStarted notifies Buildbucket that the given Job has started. Returns the
// Buildbucket token returned by Buildbucket, any error object returned by
// Buildbucket (eg. if the Build has been canceled), or any error which occurred
// when attempting the request.
func (t *TryJobIntegrator) jobStarted(ctx context.Context, j *types.Job) (string, error) {
	if isBBv2(j) {
		sklog.Infof("bb2.Start for job %s (build %d)", j.Id, j.BuildbucketBuildId)
		updateToken, err := t.bb2.StartBuild(ctx, j.BuildbucketBuildId, j.Id, j.BuildbucketToken)
		return updateToken, skerr.Wrap(err)
	} else {
		return "", skerr.Fmt("Build %d (job %s) looks like a Buildbucket V1 build, which is no longer supported.", j.BuildbucketBuildId, j.Id)
	}
}

func (t *TryJobIntegrator) updateBuild(ctx context.Context, j *types.Job) error {
	sklog.Infof("bb2.UpdateBuild for job %s (build %d)", j.Id, j.BuildbucketBuildId)
	if err := t.bb2.UpdateBuild(ctx, t.jobToBuildV2(ctx, j), j.BuildbucketToken); err != nil {
		return skerr.Wrapf(err, "failed to UpdateBuild %d for job %s", j.BuildbucketBuildId, j.Id)
	}
	return skerr.Wrap(t.sendPubSub(ctx, j))
}

func (t *TryJobIntegrator) cancelBuild(ctx context.Context, j *types.Job, reason string) error {
	sklog.Infof("bb2.CancelBuilds for job %s (build %d)", j.Id, j.BuildbucketBuildId)
	_, err := t.bb2.CancelBuild(ctx, j.BuildbucketBuildId, reason)
	if err != nil {
		return skerr.Wrapf(err, "failed to cancel build %d for job %s", j.BuildbucketBuildId, j.Id)
	}
	if j.BuildbucketPubSubTopic != "" {
		return skerr.Wrap(t.sendPubSub(ctx, j))
	}
	return nil
}

// jobsFinished notifies Buildbucket that the given Jobs have finished, then
// updates the jobs and inserts them into the DB and cache.  Returns any errors
// which occurred.
func (t *TryJobIntegrator) jobsFinished(ctx context.Context, finished []*types.Job) []error {
	ctx, span := trace.StartSpan(ctx, "jobFinished")
	span.AddAttributes(trace.Int64Attribute("count", int64(len(finished))))
	defer span.End()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	errs := []error{}
	insert := make([]*types.Job, 0, len(finished))
	var mtx sync.Mutex
	var wg sync.WaitGroup
	for _, j := range finished {
		j := j // Prevent bugs due to closure and loop variable overwriting.
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := t.jobFinished(ctx, j)
			mtx.Lock()
			defer mtx.Unlock()
			if err != nil {
				errs = append(errs, skerr.Wrapf(err, "failed to send jobFinished notification for job %s (build %d)", j.Id, j.BuildbucketBuildId))
			} else {
				j.BuildbucketLeaseKey = 0
				j.BuildbucketToken = ""
				insert = append(insert, j)
			}
		}()
	}
	wg.Wait()
	if err := t.db.PutJobsInChunks(ctx, insert); err != nil {
		errs = append(errs, err)
	}
	t.jCache.AddJobs(insert)
	return errs
}

// jobFinished notifies Buildbucket that the given Job has finished.
func (t *TryJobIntegrator) jobFinished(ctx context.Context, j *types.Job) error {
	ctx, span := trace.StartSpan(ctx, "jobFinished")
	defer span.End()

	if !j.Done() {
		return skerr.Fmt("JobFinished called for unfinished Job!")
	}
	if isBBv2(j) {
		if j.Status == types.JOB_STATUS_CANCELED {
			reason := j.StatusDetails
			if reason == "" {
				reason = "Underlying job was canceled."
			}
			return skerr.Wrap(t.cancelBuild(ctx, j, reason))
		} else {
			if err := t.updateBuild(ctx, j); err != nil {
				if isBuildAlreadyFinishedError(err) {
					// Either we've already updated the build successfully, or
					// someone else has updated it (likely canceled). Log a
					// warning in case this persists and we need to investigate,
					// but move on without returning an error.
					sklog.Warningf("Tried to update already-finished job %s (build %d)", j.Id, j.BuildbucketBuildId)
					return nil
				}
				return skerr.Wrap(err)
			} else {
				return nil
			}
		}
	} else {
		return skerr.Fmt("Build %d (job %s) looks like a Buildbucket V1 build, which is no longer supported.", j.BuildbucketBuildId, j.Id)
	}
}

// buildbucketCleanup looks for old Buildbucket Builds which were started but
// not properly updated and attempts to update them.
func (t *TryJobIntegrator) buildbucketCleanup(ctx context.Context) error {
	builds, err := t.bb2.Search(ctx, &buildbucketpb.BuildPredicate{
		Builder: &buildbucketpb.BuilderID{
			Project: t.buildbucketProject,
			Bucket:  t.buildbucketBucket,
		},
		Status: buildbucketpb.Status_STARTED,
		CreateTime: &buildbucketpb.TimeRange{
			EndTime: timestamppb.New(time.Now().Add(-CLEANUP_AGE_THRESHOLD)),
		},
	})
	if err != nil {
		return skerr.Wrap(err)
	}
	for _, build := range builds {
		if build.Builder.Bucket != t.buildbucketBucket {
			sklog.Infof("Cleanup: ignoring build %d; bucket %s is not %s", build.Id, build.Builder.Bucket, t.buildbucketBucket)
			continue
		}
		job, err := t.findJobForBuild(ctx, build.Id)
		if err != nil {
			return skerr.Wrap(err)
		}
		if job == nil {
			continue
		}
		if job.Done() {
			if job.BuildbucketToken == "" {
				sklog.Errorf("Cleanup: job %s for build %d no longer has an update token; canceling the build", job.Id, build.Id)
				if err := t.cancelBuild(ctx, job, "We no longer have an update token for this build"); err != nil {
					return skerr.Wrapf(err, "failed to cancel build %d (job %s)", build.Id, job.Id)
				}
			} else {
				sklog.Infof("Cleanup: attempting to update job %s for build %d", job.Id, build.Id)
				if err := t.updateBuild(ctx, job); err != nil {
					if isBuildAlreadyFinishedError(err) {
						// Ignore the error; the build shouldn't show up in the
						// next round of cleanup, but log the error anyway just
						// so that we're aware in case it does.
						sklog.Warningf("Cleanup: tried to update already-finished job %s (build %d)", job.Id, build.Id)
					} else {
						sklog.Errorf("Cleanup: failed to update job %s for build %d; canceling. Error: %s", job.Id, build.Id, err)
						if err := t.cancelBuild(ctx, job, "Failed to UpdateBuild"); err != nil {
							return skerr.Wrapf(err, "failed to cancel build %d (job %s)", build.Id, job.Id)
						}
					}
				}
			}
		}
	}
	return nil
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

// jobToBuildV2 converts a Job to a Buildbucket V2 Build to be used with
// UpdateBuild.
func (t *TryJobIntegrator) jobToBuildV2(ctx context.Context, job *types.Job) *buildbucketpb.Build {
	status := buildbucket_taskbackend.JobStatusToBuildbucketStatus(job.Status)

	// Note: There are other fields we could fill in, but I'm not sure they
	// would provide any value since we don't actually use Buildbucket builds
	// for anything.
	return &buildbucketpb.Build{
		Id: job.BuildbucketBuildId,
		Output: &buildbucketpb.Build_Output{
			Status:          status,
			SummaryMarkdown: job.StatusDetails,
		},
		Infra: &buildbucketpb.BuildInfra{
			Backend: &buildbucketpb.BuildInfra_Backend{
				Task: buildbucket_taskbackend.JobToBuildbucketTask(ctx, job, t.buildbucketTarget, t.host),
			},
		},
	}
}
