package trybots

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	buildbucket_api "github.com/luci/luci-go/common/api/buildbucket/buildbucket/v1"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/specs"
)

/*
	Integration of the Task Scheduler with Buildbucket for try jobs.
*/

const (
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
	bb           *buildbucket_api.Service
	bucket       string
	db           db.JobDB
	jCache       db.JobCache
	rm           *gitinfo.RepoMap
	taskCfgCache *specs.TaskCfgCache
}

// NewTrybotIntegrator returns a TryJobIntegrator instance.
func NewTrybotIntegrator(bucket string, c *http.Client, d db.JobDB, cache db.JobCache, rm *gitinfo.RepoMap, taskCfgCache *specs.TaskCfgCache) (*TryJobIntegrator, error) {
	bb, err := buildbucket_api.New(c)
	if err != nil {
		return nil, err
	}
	// TODO(borenet): if !testing {
	bb.BasePath = "https://cr-buildbucket.appspot.com/_ah/api/buildbucket/v1/"
	rv := &TryJobIntegrator{
		bb:           bb,
		bucket:       bucket,
		db:           d,
		jCache:       cache,
		rm:           rm,
		taskCfgCache: taskCfgCache,
	}
	return rv, nil
}

// Start initiates the TryJobIntegrator's heatbeat and polling loops.
func (t *TryJobIntegrator) Start() {
	go func() {
		for _ = range time.Tick(HEARTBEAT_INTERVAL) {
			if err := t.sendHeartbeats(); err != nil {
				glog.Errorf("Failed to send hearbeats: %s", err)
			}
		}
	}()
	go func() {
		for _ = range time.Tick(POLL_INTERVAL) {
			if err := t.poll(); err != nil {
				glog.Errorf("Failed to poll for new try jobs: %s", err)
			}
		}
	}()
}

// sendHeartbeats sends heartbeats to Buildbucket for all of the unfinished try
// Jobs.
func (t *TryJobIntegrator) sendHeartbeats() error {
	glog.Infof("Sending heartbeats.")
	unfinishedJobs, err := t.jCache.UnfinishedJobs()
	if err != nil {
		return err
	}
	jobs := make([]*db.Job, 0, len(unfinishedJobs))
	for _, j := range unfinishedJobs {
		if j.IsTryJob() {
			jobs = append(jobs, j)
		}
	}

	expiration := time.Now().Add(LEASE_DURATION).Unix() * 1000000

	// Send heartbeats for all leases.
	send := func(jobs []*db.Job) error {
		heartbeats := make([]*buildbucket_api.ApiHeartbeatBatchRequestMessageOneHeartbeat, 0, len(jobs))
		byBuildId := make(map[int64]*db.Job, len(jobs))
		for _, j := range jobs {
			heartbeats = append(heartbeats, &buildbucket_api.ApiHeartbeatBatchRequestMessageOneHeartbeat{
				BuildId:           j.BuildbucketBuildId,
				LeaseKey:          j.BuildbucketLeaseKey,
				LeaseExpirationTs: expiration,
			})
			byBuildId[j.BuildbucketBuildId] = j
		}
		resp, err := t.bb.HeartbeatBatch(&buildbucket_api.ApiHeartbeatBatchRequestMessage{
			Heartbeats: heartbeats,
		}).Do()
		if err != nil {
			return err
		}
		cancelJobs := []*db.Job{}
		for _, result := range resp.Results {
			j := byBuildId[result.BuildId]
			if result.Error != nil {
				if j == nil {
					glog.Errorf("Error sending heartbeat for job; no valid ID returned from buildbucket so can't cancel the job.")
				} else {
					// Cancel the job.
					glog.Errorf("Error sending heartbeat for job; canceling %q: %s", j.Id, result.Error.Message)
					j.Status = db.JOB_STATUS_CANCELED
					cancelJobs = append(cancelJobs, j)
				}
			}
		}
		if len(cancelJobs) > 0 {
			return t.db.PutJobs(cancelJobs)
		}
		return nil
	}

	// Send heartbeats in batches.
	for i := 0; i < len(jobs); i += LEASE_BATCH_SIZE {
		j := i + LEASE_BATCH_SIZE
		if j > len(jobs) {
			j = len(jobs)
		}
		if j > i {
			if err := send(jobs[i:j]); err != nil {
				return err
			}
		} else {
			break
		}
	}
	glog.Infof("Finished sending heartbeats.")
	return nil
}

func (t *TryJobIntegrator) getRepo(project string) (string, *gitinfo.GitInfo, error) {
	// TODO(borenet): It'd be nice not to hard-code these.
	url := ""
	if project == "skia" {
		url = common.REPO_SKIA
	} else if project == "buildbot" {
		url = common.REPO_SKIA_INFRA
	} else {
		return "", nil, fmt.Errorf("Unknown patch project: %s", project)
	}
	r, err := t.rm.Repo(url)
	if err != nil {
		return "", nil, fmt.Errorf("Failed to get repo %s: %s", url, err)
	}
	return url, r, nil
}

func (t *TryJobIntegrator) getRevision(repo *gitinfo.GitInfo, revision string) (string, error) {
	if revision == "" || revision == "HEAD" {
		revision = "origin/master"
	}
	return repo.FullHash(revision)
}

func (t *TryJobIntegrator) cancelBuild(id int64, msg string) error {
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

func (t *TryJobIntegrator) tryLeaseBuild(id int64) (int64, error) {
	expiration := time.Now().Add(LEASE_DURATION_INITIAL).Unix() * 1000000
	glog.Infof("Attempting to lease build, ts=%d", expiration)
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

func (t *TryJobIntegrator) getJobToSchedule(b *buildbucket_api.ApiBuildMessage) (*db.Job, error) {
	glog.Infof("Found build: %v", b)
	// Attempt to lease the build.
	leaseKey, err := t.tryLeaseBuild(b.Id)
	if err != nil {
		// TODO(borenet): Buildbot cancels the build in this case.
		// Should we do that too?
		return nil, err
	}

	// Parse the build parameters.
	var params buildbucket.BuildBucketParameters
	if err := json.NewDecoder(strings.NewReader(b.ParametersJson)).Decode(&params); err != nil {
		return nil, t.cancelBuild(b.Id, fmt.Sprintf("Invalid parameters_json: %s", err))
	}

	// Obtain and validate the RepoState.
	repoName, repo, err := t.getRepo(params.Properties.PatchProject)
	if err != nil {
		return nil, t.cancelBuild(b.Id, fmt.Sprintf("Unable to find repo: %s", err))
	}
	glog.Infof("Properties: %v", params.Properties)
	revision, err := t.getRevision(repo, params.Properties.Revision)
	if err != nil {
		return nil, t.cancelBuild(b.Id, fmt.Sprintf("Invalid revision: %s", err))
	}
	server := params.Properties.Gerrit
	issue := params.Properties.GerritIssue
	patchset := params.Properties.GerritPatchset
	if params.Properties.PatchStorage == "rietveld" {
		server = params.Properties.Rietveld
		issue = fmt.Sprintf("%d", params.Properties.RietveldIssue)
		patchset = fmt.Sprintf("%d", params.Properties.RietveldPatchset)
	} else if params.Properties.PatchStorage != "gerrit" {
		return nil, t.cancelBuild(b.Id, fmt.Sprintf("Invalid patch storage: %s", params.Properties.PatchStorage))
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
		return nil, t.cancelBuild(b.Id, fmt.Sprintf("Invalid RepoState: %s", rs))
	}

	// Obtain the correct Job spec.
	spec, err := t.taskCfgCache.GetJobSpec(rs, params.BuilderName)
	if err != nil {
		return nil, t.cancelBuild(b.Id, fmt.Sprintf("Failed to obtain JobSpec: %s", err))
	}

	j := &db.Job{
		BuildbucketBuildId:  b.Id,
		BuildbucketLeaseKey: leaseKey,
		Created:             time.Unix(0, b.CreatedTs).UTC(),
		Dependencies:        spec.TaskSpecs,
		Name:                params.BuilderName,
		RepoState:           rs,
	}

	// Since Jobs may consist of multiple Tasks, we consider them to be
	// "started" as soon as we've picked them up.
	if err := t.onJobStarted(j); err != nil {
		return nil, err
	}

	// Create and return the Job.
	return j, nil
}

func (t *TryJobIntegrator) poll() error {
	// Grab all of the pending Builds from Buildbucket.
	// TODO(borenet): Buildbot maintains a maximum lease count. Should we do
	// that too?
	cursor := ""
	jobs := []*db.Job{}
	errs := []error{}
	for {
		glog.Infof("Running 'peek'")
		resp, err := t.bb.Peek().Bucket(t.bucket).MaxBuilds(PEEK_MAX_BUILDS).StartCursor(cursor).Do()
		glog.Infof("Done peeking.")
		if err != nil {
			errs = append(errs, err)
			break
		}
		if resp.Error != nil {
			errs = append(errs, fmt.Errorf(resp.Error.Message))
			break
		}
		for _, b := range resp.Builds {
			j, err := t.getJobToSchedule(b)
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

	// Report any errors.
	if len(errs) > 0 {
		return fmt.Errorf("Got errors loading builds from Buildbucket: %v", errs)
	}

	return nil
}

// onJobStarted notifies Buildbucket that the given Job has started.
func (t *TryJobIntegrator) onJobStarted(j *db.Job) error {
	resp, err := t.bb.Start(j.BuildbucketBuildId, &buildbucket_api.ApiStartRequestBodyMessage{
		LeaseKey: j.BuildbucketLeaseKey,
		Url:      j.Url(),
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
		return fmt.Errorf("onJobFinished called for unfinished Job!")
	}
	if j.Status == db.JOB_STATUS_SUCCESS {
		resp, err := t.bb.Succeed(j.BuildbucketBuildId, &buildbucket_api.ApiSucceedRequestBodyMessage{
			LeaseKey:          j.BuildbucketLeaseKey,
			ResultDetailsJson: "{\"result\": \"TODO(borenet)\"}",
			Url:               j.Url(),
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
			Url:               j.Url(),
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
