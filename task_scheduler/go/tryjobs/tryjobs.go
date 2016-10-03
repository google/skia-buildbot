package tryjobs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/context"

	buildbucket_api "github.com/luci/luci-go/common/api/buildbucket/buildbucket/v1"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/specs"
)

/*
	Integration of the Task Scheduler with Buildbucket for try jobs.
*/

const (
	// API URLs
	API_URL_PROD    = "https://cr-buildbucket.appspot.com/api/buildbucket/v1/"
	API_URL_TESTING = "http://localhost:8008/api/buildbucket/v1/"

	// Buildbucket buckets used for try jobs.
	BUCKET_PRIMARY = "skia.primary"
	BUCKET_TESTING = "skia.testing"

	// How often to send heartbeats to Buildbucket.
	HEARTBEAT_INTERVAL = 5 * time.Minute

	// We attempt to renew leases in batches. This is the batch size.
	LEASE_BATCH_SIZE = 25

	// We lease a build for this amount of time, and if we don't renew the
	// lease before the time is up, the build resets to "scheduled" status
	// and becomes available for leasing again.
	LEASE_DURATION = time.Hour

	// We use a shorter initial lease duration in case we succeed in leasing
	// a build but fail to insert the associated Job into the DB, eg.
	// because the scheduler was interrupted.
	LEASE_DURATION_INITIAL = 2 * time.Minute

	// How many pending builds to read from the bucket at a time.
	PEEK_MAX_BUILDS = 50

	// How often to poll Buildbucket for newly-scheduled builds.
	POLL_INTERVAL = time.Minute
)

// TryJobIntegrator is responsible for communicating with Buildbucket to
// trigger try jobs and report their results.
type TryJobIntegrator struct {
	bb                 *buildbucket_api.Service
	bucket             string
	db                 db.JobDB
	jCache             db.JobCache
	projectRepoMapping map[string]string
	rm                 *gitinfo.RepoMap
	taskCfgCache       *specs.TaskCfgCache
}

// NewTryJobIntegrator returns a TryJobIntegrator instance.
func NewTryJobIntegrator(apiUrl, bucket string, c *http.Client, d db.JobDB, cache db.JobCache, projectRepoMapping map[string]string, rm *gitinfo.RepoMap, taskCfgCache *specs.TaskCfgCache) (*TryJobIntegrator, error) {
	bb, err := buildbucket_api.New(c)
	if err != nil {
		return nil, err
	}
	bb.BasePath = apiUrl
	rv := &TryJobIntegrator{
		bb:                 bb,
		bucket:             bucket,
		db:                 d,
		jCache:             cache,
		projectRepoMapping: projectRepoMapping,
		rm:                 rm,
		taskCfgCache:       taskCfgCache,
	}
	return rv, nil
}

// Start initiates the TryJobIntegrator's heatbeat and polling loops. If the
// given Context is canceled, the loops stop.
func (t *TryJobIntegrator) Start(ctx context.Context) {
	go util.RepeatCtx(HEARTBEAT_INTERVAL, ctx, func() {
		if err := t.sendHeartbeats(time.Now()); err != nil {
			glog.Error(err)
		}
	})
	go util.RepeatCtx(POLL_INTERVAL, ctx, func() {
		if err := t.Poll(time.Now()); err != nil {
			glog.Errorf("Failed to poll for new try jobs: %s", err)
		}
	})
}

// sendHeartbeats sends heartbeats to Buildbucket for all of the unfinished try
// Jobs.
func (t *TryJobIntegrator) sendHeartbeats(now time.Time) error {
	glog.Infof("Sending heartbeats.")
	if err := t.jCache.Update(); err != nil {
		return err
	}
	unfinishedJobs, err := t.jCache.UnfinishedJobs()
	if err != nil {
		return fmt.Errorf("Failed to obtain unfinished jobs: %s", err)
	}
	jobs := make([]*db.Job, 0, len(unfinishedJobs))
	for _, j := range unfinishedJobs {
		if j.IsTryJob() {
			jobs = append(jobs, j)
		}
	}
	// Sort so that we get deterministic results in tests.
	sort.Sort(db.JobSlice(jobs))

	expiration := now.Add(LEASE_DURATION).Unix() * 1000000

	errs := []error{}

	// Send heartbeats for all leases.
	send := func(jobs []*db.Job) {
		heartbeats := make([]*buildbucket_api.ApiHeartbeatBatchRequestMessageOneHeartbeat, 0, len(jobs))
		for _, j := range jobs {
			heartbeats = append(heartbeats, &buildbucket_api.ApiHeartbeatBatchRequestMessageOneHeartbeat{
				BuildId:           j.BuildbucketBuildId,
				LeaseKey:          j.BuildbucketLeaseKey,
				LeaseExpirationTs: expiration,
			})
		}
		glog.Infof("Sending heartbeats for %d jobs...", len(jobs))
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
		cancelJobs := []*db.Job{}
		for i, result := range resp.Results {
			if result.Error != nil {
				// Cancel the job.
				// TODO(borenet): Should we return an error here?
				glog.Errorf("Error sending heartbeat for job; canceling %q: %s", jobs[i].Id, result.Error.Message)
				cancelJobs = append(cancelJobs, jobs[i])
			}
		}
		if len(cancelJobs) > 0 {
			glog.Errorf("Canceling %d jobs", len(cancelJobs))
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
	glog.Infof("Finished sending heartbeats.")
	if len(errs) > 0 {
		return fmt.Errorf("Got errors sending heartbeats: %v", errs)
	}
	return nil
}

func (t *TryJobIntegrator) getRepo(project string) (string, *gitinfo.GitInfo, error) {
	url, ok := t.projectRepoMapping[project]
	if !ok {
		return "", nil, fmt.Errorf("Unknown patch project %q", project)
	}
	r, err := t.rm.Repo(url)
	if err != nil {
		return "", nil, fmt.Errorf("Failed to get repo %q: %s", url, err)
	}
	return url, r, nil
}

func (t *TryJobIntegrator) getRevision(repo *gitinfo.GitInfo, revision string) (string, error) {
	if revision == "" || revision == "HEAD" || revision == "master" {
		revision = "origin/master"
	}
	return repo.FullHash(revision)
}

func (t *TryJobIntegrator) localCancelJobs(jobs []*db.Job) error {
	for _, j := range jobs {
		j.Status = db.JOB_STATUS_CANCELED
		j.Finished = time.Now()
	}
	if err := t.db.PutJobs(jobs); err != nil {
		return err
	}
	return t.jCache.Update()
}

func (t *TryJobIntegrator) remoteCancelBuild(id int64, msg string) error {
	glog.Errorf("Canceling Buildbucket build %d. Reason: %s", id, msg)
	// TODO(borenet): We want to send the cancellation reason along to
	// Buildbucket for debugging purposes.
	resp, err := t.bb.Cancel(id).Do()
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
	glog.Infof("Attempting to lease build %d", id)
	resp, err := t.bb.Lease(id, &buildbucket_api.ApiLeaseRequestBodyMessage{
		LeaseExpirationTs: expiration,
	}).Do()
	if err != nil {
		return 0, err
	}
	if resp.Error != nil {
		return 0, fmt.Errorf(resp.Error.Message)
	}
	return resp.Build.LeaseKey, nil
}

func (t *TryJobIntegrator) getJobToSchedule(b *buildbucket_api.ApiBuildMessage, now time.Time) (*db.Job, error) {
	// Parse the build parameters.
	var params buildbucket.Parameters
	if err := json.NewDecoder(strings.NewReader(b.ParametersJson)).Decode(&params); err != nil {
		return nil, t.remoteCancelBuild(b.Id, fmt.Sprintf("Invalid parameters_json: %s", err))
	}

	// Obtain and validate the RepoState.
	repoName, repo, err := t.getRepo(params.Properties.PatchProject)
	if err != nil {
		return nil, t.remoteCancelBuild(b.Id, fmt.Sprintf("Unable to find repo: %s", err))
	}
	revision, err := t.getRevision(repo, params.Properties.Revision)
	if err != nil {
		return nil, t.remoteCancelBuild(b.Id, fmt.Sprintf("Invalid revision: %s", err))
	}
	server := params.Properties.Gerrit
	issue := params.Properties.GerritIssue
	patchset := params.Properties.GerritPatchset
	if params.Properties.PatchStorage == "rietveld" {
		server = params.Properties.Rietveld
		issue = fmt.Sprintf("%d", params.Properties.RietveldIssue)
		patchset = fmt.Sprintf("%d", params.Properties.RietveldPatchset)
	} else if params.Properties.PatchStorage != "gerrit" {
		return nil, t.remoteCancelBuild(b.Id, fmt.Sprintf("Invalid patch storage: %s", params.Properties.PatchStorage))
	}
	rs := db.RepoState{
		Patch: db.Patch{
			Server:   server,
			Issue:    issue,
			Patchset: patchset,
		},
		Repo:     repoName,
		Revision: revision,
	}
	if !rs.Valid() || !rs.IsTryJob() {
		return nil, t.remoteCancelBuild(b.Id, fmt.Sprintf("Invalid RepoState: %s", rs))
	}

	// Obtain the correct Job spec.
	spec, err := t.taskCfgCache.GetJobSpec(rs, params.BuilderName)
	if err != nil {
		return nil, t.remoteCancelBuild(b.Id, fmt.Sprintf("Failed to obtain JobSpec: %s", err))
	}

	// Attempt to lease the build.
	leaseKey, err := t.tryLeaseBuild(b.Id, now)
	if err != nil {
		// TODO(borenet): Buildbot cancels the build in this case.
		// Should we do that too?
		return nil, err
	}

	// Create and return the Job.
	j := &db.Job{
		BuildbucketBuildId:  b.Id,
		BuildbucketLeaseKey: leaseKey,
		Created:             time.Unix(0, b.CreatedTs/util.MICROS_TO_NANOS).UTC(),
		Dependencies:        spec.TaskSpecs,
		Name:                params.BuilderName,
		RepoState:           rs,
	}

	return j, nil
}

func (t *TryJobIntegrator) Poll(now time.Time) error {
	// Grab all of the pending Builds from Buildbucket.
	// TODO(borenet): Buildbot maintains a maximum lease count. Should we do
	// that too?
	cursor := ""
	jobs := []*db.Job{}
	errs := []error{}
	for {
		glog.Infof("Running 'peek'")
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
			j, err := t.getJobToSchedule(b, now)
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
	if len(jobs) > 0 {
		if err := t.db.PutJobs(jobs); err != nil {
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
	cancelJobs := []*db.Job{}
	for _, j := range jobs {
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
func (t *TryJobIntegrator) jobStarted(j *db.Job) error {
	resp, err := t.bb.Start(j.BuildbucketBuildId, &buildbucket_api.ApiStartRequestBodyMessage{
		LeaseKey: j.BuildbucketLeaseKey,
		Url:      j.URL(),
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

// JobFinished notifies Buildbucket that the given Job has finished.
func (t *TryJobIntegrator) JobFinished(j *db.Job) error {
	if !j.Done() {
		return fmt.Errorf("JobFinished called for unfinished Job!")
	}
	if j.Status == db.JOB_STATUS_SUCCESS {
		resp, err := t.bb.Succeed(j.BuildbucketBuildId, &buildbucket_api.ApiSucceedRequestBodyMessage{
			LeaseKey:          j.BuildbucketLeaseKey,
			ResultDetailsJson: "{\"result\": \"TODO(borenet)\"}",
			Url:               j.URL(),
		}).Do()
		if err != nil {
			return err
		}
		if resp.Error != nil {
			return fmt.Errorf(resp.Error.Message)
		}
	} else {
		failureReason := "BUILD_FAILURE"
		if j.Status == db.JOB_STATUS_MISHAP {
			failureReason = "INFRA_FAILURE"
		}
		resp, err := t.bb.Fail(j.BuildbucketBuildId, &buildbucket_api.ApiFailRequestBodyMessage{
			FailureReason:     failureReason,
			LeaseKey:          j.BuildbucketLeaseKey,
			ResultDetailsJson: "{\"result\": \"TODO(borenet)\"}",
			Url:               j.URL(),
		}).Do()
		if err != nil {
			return err
		}
		if resp.Error != nil {
			return fmt.Errorf(resp.Error.Message)
		}
	}
	return nil
}
