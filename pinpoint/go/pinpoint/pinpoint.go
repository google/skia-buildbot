package pinpoint

import (
	"context"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"

	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/bot_configs"
	"go.skia.org/infra/pinpoint/go/build_chrome"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/read_values"
	"go.skia.org/infra/pinpoint/go/run_benchmark"

	"golang.org/x/oauth2/google"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	bpb "go.chromium.org/luci/buildbucket/proto"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	ppb "go.skia.org/infra/pinpoint/proto/v1"
)

const (
	missingRequiredParamTemplate = "Missing required param %s"
	chromiumSrcGit               = "https://chromium.googlesource.com/chromium/src.git"
	maxSampleSize                = 20
	interval                     = 10
	swarmingServiceAddress       = "chrome-swarming.appspot.com:443"
)

// PinpointHandler is an interface to run Pinpoint jobs
type PinpointHandler interface {
	// Run triggers a local run of a Pinpoint job. So far this job will
	// build Chrome at the StartCommit and EndCommit and retrieve the CAS
	// of any successful builds
	// jobID is an optional argument for local testing. Setting the same
	// jobID can reuse swarming results which can be helpful to triage
	// the workflow and not wait on tasks to finish.
	// TODO(sunxiaodi@): implement Run
	Run(ctx context.Context, req *ppb.ScheduleBisectRequest, jobID string) (*PinpointRunResponse, error)
}

type PinpointRunResponse struct {
	// JobID is the unique job ID.
	JobID string
	// Commits is for tracking all of the commits run in the
	// job. Commits is useful for triaging.
	Commits []*commitData
	// Culprits is a list of culprits found in a bisection run.
	Culprits []string
}

// pinpointJobImpl implements the PinpointJob interface.
type pinpointHandlerImpl struct {
	sc backends.SwarmingClient
	bc build_chrome.BuildChromeClient
	mc *midpoint.MidpointHandler
}

// buildMetadata tracks relevant build Chrome metadata
type buildMetadata struct {
	// buildID is the buildbucket ID of the Chrome build
	buildID int64
	// buildStatus is the status of the build
	buildStatus bpb.Status
	// buildCAS is the CAS address of the build isolate
	buildCAS *swarmingV1.SwarmingRpcsCASReference
}

// testMetadata tracks relevant run benchmark metadata
type testMetadata struct {
	req        *run_benchmark.RunBenchmarkRequest
	tasks      []string
	states     []string
	casOutputs []*swarmingV1.SwarmingRpcsCASReference
	isRunning  bool
}

// commitData stores relevant metadata pertaining to the specific commit
type commitData struct {
	commit *midpoint.Commit
	build  *buildMetadata
	tests  *testMetadata
	values []float64
}

// commitDataList tracks all of the commits in the Pinpoint job
// commitDataList also ensures the order of the commits in order
// of when they landed
type commitDataList struct {
	commits []*commitData
}

func New(ctx context.Context) (*pinpointHandlerImpl, error) {
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrapf(err, "Problem setting up default token source")
	}
	c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).With2xxOnly().Client()

	sc, err := backends.NewSwarmingClient(ctx, swarmingServiceAddress)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not create swarming client")
	}

	return &pinpointHandlerImpl{
		sc: sc,
		bc: build_chrome.NewWithClient(c),
		mc: midpoint.New(ctx, c),
	}, nil
}

// Run implements the pinpointJobImpl interface
func (pp *pinpointHandlerImpl) Run(ctx context.Context, req *ppb.ScheduleBisectRequest, jobID string) (*PinpointRunResponse, error) {
	if jobID == "" {
		jobID = uuid.New().String()
	}
	err := pp.validateRunRequest(req)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not validate request inputs")
	}

	resp := &PinpointRunResponse{
		JobID:    jobID,
		Culprits: []string{},
	}

	target, err := bot_configs.GetIsolateTarget(req.Configuration, req.Benchmark)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not get isolate target with builder name %s and benchmark %s", req.Configuration, req.Benchmark)
	}

	cdl := commitDataList{
		commits: []*commitData{
			{
				commit: &midpoint.Commit{
					GitHash:       req.StartGitHash,
					RepositoryUrl: chromiumSrcGit,
				},
			},
			{
				commit: &midpoint.Commit{
					GitHash:       req.EndGitHash,
					RepositoryUrl: chromiumSrcGit,
				},
			},
		},
	}

	// execute Pinpoint job
	for cdl.shouldContinue() {
		cdl.statusCheck()
		// start builds that have not been scheduled
		for _, c := range cdl.commits {
			if c.build == nil {
				buildID, err := pp.bc.SearchOrBuild(ctx, jobID, c.commit.GitHash, req.Configuration, nil, nil)
				if err != nil {
					return resp, skerr.Wrapf(err, "could not kick off build for commit %s", c.commit.GitHash)
				}
				c.build = &buildMetadata{
					buildID: buildID,
				}
			}
		}
		// TODO(sunxiaodi@) deprecate polling with pubsub
		c, err := cdl.pollBuild(ctx, pp.bc)
		if err != nil {
			return resp, err
		}
		// retrieve CAS of successful builds and schedule new benchmark runs
		if c != nil && c.build.buildStatus == bpb.Status_SUCCESS {
			cas, err := pp.bc.RetrieveCAS(ctx, c.build.buildID, target)
			if err != nil {
				return resp, skerr.Wrapf(err, "Could not retrieve CAS info")
			}
			c.build.buildCAS = cas
			c.tests = &testMetadata{
				req: c.createRunBenchmarkRequest(jobID, req.Configuration, target, req),
			}
			tasks, err := c.scheduleRunBenchmark(ctx, pp.sc, req)
			if err != nil {
				return resp, err
			}
			if len(tasks) > 0 {
				c.tests.tasks = tasks
				c.tests.isRunning = true
			}
		}
		// TODO(sunxiaodi@) deprecate polling with pubsub
		i, c, err := cdl.pollTests(ctx, pp.sc)
		if err != nil {
			return resp, err
		}
		if c != nil {
			cas, err := c.getTestCAS(ctx, pp.sc)
			if err != nil {
				return resp, err
			}
			// TODO(sunxiaodi@): handle all tests failed
			if len(cas) == 0 {
				return resp, skerr.Fmt("all tests failed for commit %s", c.commit.GitHash)
			}
			c.tests.casOutputs = cas
			// we don't know the cas instance ahead of time so we dial it here
			rc, err := read_values.DialRBECAS(ctx, c.tests.casOutputs[0].CasInstance)
			if err != nil {
				return resp, skerr.Wrapf(err, "failed to dial rbe client")
			}
			values, err := c.getValues(ctx, rc, req)
			if err != nil {
				return resp, err
			}
			c.values = values
			// must compare right before left
			// If left compare happens first and midpoint is queued, then
			// i would need to shift one. Doing it this way avoids any index shifting
			left, right := i, i+1
			sklog.Debugf("compare right [%d] vs [%d]", left, right)
			magnitude, err := strconv.ParseFloat(req.ComparisonMagnitude, 64)
			if err != nil {
				return resp, err
			}
			res, err := cdl.compareNeighbor(left, right, magnitude)
			if err != nil {
				return resp, skerr.Wrapf(err, "could not compare [%d] against right neighbor", left)
			}
			if res != nil {
				spew.Dump(res)
				culprit, err := cdl.updateCommitsByResult(ctx, pp.sc, pp.mc, res, left, right, req)
				if err != nil {
					return resp, skerr.Wrapf(err, "could not update commitDataList after compare")
				}
				if culprit != nil {
					resp.Culprits = append(resp.Culprits, culprit.GitHash)
				}
			}
			// compare left
			left, right = i-1, i
			sklog.Debugf("compare left [%d] vs [%d]", left, right)
			res, err = cdl.compareNeighbor(left, right, magnitude)
			if err != nil {
				return resp, skerr.Wrapf(err, "could not compare [%d] against left neighbor", right)
			}
			if res != nil {
				spew.Dump(res)
				culprit, err := cdl.updateCommitsByResult(ctx, pp.sc, pp.mc, res, left, right, req)
				if err != nil {
					return resp, skerr.Wrapf(err, "could not update commitDataList after compare")
				}
				if culprit != nil {
					resp.Culprits = append(resp.Culprits, culprit.GitHash)
				}
			}
			sklog.Debugf("after compares - length of cdl now is %d", len(cdl.commits))
		}
		resp.Commits = cdl.commits
		sklog.Debugf("current culprit list %v", resp.Culprits)
		time.Sleep(10 * time.Second)
	}
	return resp, nil
}

// validateRunRequest validates the request args and returns an error if there request is invalid
func (pp *pinpointHandlerImpl) validateRunRequest(req *ppb.ScheduleBisectRequest) error {
	if req.StartGitHash == "" {
		return skerr.Fmt(missingRequiredParamTemplate, "base git hash a")
	}
	if req.EndGitHash == "" {
		return skerr.Fmt(missingRequiredParamTemplate, "base git hash b")
	}
	_, err := bot_configs.GetBotConfig(req.Configuration, false)
	if err != nil {
		return skerr.Wrapf(err, "Builder name %s not allowed in bot configurations", req.Configuration)
	}
	if req.Benchmark == "" {
		return skerr.Fmt(missingRequiredParamTemplate, "benchmark")
	}
	if req.Story == "" {
		return skerr.Fmt(missingRequiredParamTemplate, "story")
	}
	if req.Chart == "" {
		return skerr.Fmt(missingRequiredParamTemplate, "chart")
	}
	return nil
}

// shouldContinue returns true if the pinpoint job should continue or not
func (cdl commitDataList) shouldContinue() bool {
	for _, c := range cdl.commits {
		if c.build == nil || c.build.buildStatus < bpb.Status_ENDED_MASK {
			return true
		}
		if c.tests != nil && c.tests.isRunning {
			return true
		}
	}
	return false
}

// statusCheck runs through every commit in the list and checks on their ongoing status
func (cdl commitDataList) statusCheck() {
	for i, c := range cdl.commits {
		if c.build == nil || c.build.buildStatus < bpb.Status_ENDED_MASK {
			sklog.Debugf("commit [%d] %s is building", i, c.commit.GitHash[:7])
		} else if c.tests != nil && c.tests.isRunning {
			sklog.Debugf("commit [%d] %s is testing", i, c.commit.GitHash[:7])
		} else {
			sklog.Debugf("commit [%d] %s is done", i, c.commit.GitHash[:7])
		}
	}
}

// pollBuild checks the build status of every commit in the commitQ
// returns upon finding the first build that was running and finishes
func (cdl commitDataList) pollBuild(ctx context.Context, bc build_chrome.BuildChromeClient) (
	*commitData, error) {
	for _, c := range cdl.commits {
		if c.build == nil || c.build.buildID == 0 {
			return nil, skerr.Fmt("Cannot poll build of non-existent build")
		}
		// build already finished, then don't poll
		if c.tests != nil {
			continue
		}
		status, err := bc.GetStatus(ctx, c.build.buildID)
		sklog.Debugf("build %d has status %s", c.build.buildID, status)
		if err != nil {
			return nil, skerr.Wrapf(err, "Could not get build status %d", c.build.buildID)
		}
		// check ongoing build
		if c.build.buildStatus < bpb.Status_ENDED_MASK {
			// update the build status
			c.build.buildStatus = status
			if status > bpb.Status_ENDED_MASK {
				return c, nil
			}
		}
	}
	return nil, nil
}

// createRunBenchmarkRequest converts job run request information to a run_benchmark
// swarming request
func (c *commitData) createRunBenchmarkRequest(jobID string, cfg string, target string, req *ppb.ScheduleBisectRequest) *run_benchmark.RunBenchmarkRequest {
	return &run_benchmark.RunBenchmarkRequest{
		JobID:     jobID,
		Build:     c.build.buildCAS,
		Commit:    c.commit.GitHash,
		BotConfig: cfg,
		Benchmark: req.Benchmark,
		Story:     req.Story,
		Target:    target,
	}
}

// scheduleRunBenchmark schedules run benchmark tests to swarming and returns the task IDs
func (c *commitData) scheduleRunBenchmark(ctx context.Context, sc backends.SwarmingClient, req *ppb.ScheduleBisectRequest) ([]string, error) {
	if c.tests == nil || c.tests.req == nil {
		return nil, skerr.Fmt("Cannot schedule benchmark runs without request")
	}
	// Fetching Pinpoint tasks here can skip scheduling new tasks for faster testing
	tasks, err := sc.ListPinpointTasks(ctx, c.tests.req.JobID, c.tests.req.Build)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not list tasks prior to run benchmark for request %v", *c.tests.req)
	}
	if len(tasks) < maxSampleSize {
		for i := 0; i < interval; i++ {
			task, err := run_benchmark.Run(ctx, sc, c.tests.req.Commit, req.Configuration, req.Benchmark, req.Story, req.StoryTags, c.tests.req.JobID, c.tests.req.Build, 1)
			if err != nil {
				return nil, skerr.Wrapf(err, "Could not start run benchmark task for request %v", c.tests.req)
			}
			tasks = append(tasks, task[0].TaskId)
		}
	}
	return tasks, nil
}

// pollTests checks the test status of every commit in the commitQ
// returns upon finding the first commit with running tasks that all finished
// returns the index of the commit so it is easier to compare left and right neighbors
func (cdl commitDataList) pollTests(ctx context.Context, sc backends.SwarmingClient) (int, *commitData, error) {
	for i, c := range cdl.commits {
		if c.tests == nil {
			continue
		}
		if c.tests.isRunning {
			c.tests.isRunning = false
			states, err := sc.GetStates(ctx, c.tests.tasks)
			if err != nil {
				return -1, nil, skerr.Wrapf(err, "failed to retrieve swarming tasks %v", c.tests.tasks)
			}
			c.tests.states = states
			for j, s := range states {
				if s == swarming.TASK_STATE_PENDING || s == swarming.TASK_STATE_RUNNING {
					sklog.Debugf("[%d] commit %s: task %s with state %s", j, c.commit.GitHash[:7], c.tests.tasks[j], s)
					c.tests.isRunning = true
				}
			}
			if !c.tests.isRunning {
				return i, c, nil
			}
		}
	}
	return -1, nil, nil
}

// getTestCAS returns the CAS output addresses from a set of swarming tests
func (c *commitData) getTestCAS(ctx context.Context, sc backends.SwarmingClient) (
	[]*swarmingV1.SwarmingRpcsCASReference, error) {
	casOutputs := []*swarmingV1.SwarmingRpcsCASReference{}
	if c.tests == nil {
		return nil, skerr.Fmt("cannot get cas output of non-existent swarming tasks")
	}
	if len(c.tests.states) != len(c.tests.tasks) {
		return nil, skerr.Fmt("mismatching number of swarming states (%d) and task IDs (%d)",
			len(c.tests.states), len(c.tests.tasks))
	}
	for i, s := range c.tests.states {
		if s == "COMPLETED" {
			cas, err := sc.GetCASOutput(ctx, c.tests.tasks[i])
			if err != nil {
				return nil, skerr.Wrapf(err, "error retrieving cas outputs")
			}
			casOutputs = append(casOutputs, cas)
		}
	}
	return casOutputs, nil
}

// getValues will return the values from a set of swarming test cas outputs
func (c *commitData) getValues(ctx context.Context, rc *rbeclient.Client, req *ppb.ScheduleBisectRequest) ([]float64, error) {
	if c.tests == nil {
		return nil, skerr.Fmt("cannot retrieve values with no swarming tests")
	}
	if c.tests.casOutputs == nil {
		return nil, skerr.Fmt("cannot retrieve values with no swarming test cas outputs")
	}
	return read_values.ReadValuesByChart(ctx, rc, req.Benchmark,
		req.Chart, c.tests.casOutputs, req.AggregationMethod)
}

// compareNeighbor takes two commits (left and right) and compares
// their values against each other to see if they are statistically
// significantly different.
func (cdl commitDataList) compareNeighbor(left, right int, rawDiff float64) (*compare.CompareResults, error) {
	if left >= right {
		return nil, skerr.Fmt("left index %d is >= right index %d", left, right)
	}
	if right < 0 {
		return nil, skerr.Fmt("right index %d is out of bounds", right)
	}
	if left >= len(cdl.commits) {
		return nil, skerr.Fmt("left index %d is out of bounds", left)
	}
	// it is possible for left or right to be out of bounds
	// i.e. left = -1, right = 0 because commits will compare against
	// left and right neighbors. In such cases, return nil
	if left < 0 || right >= len(cdl.commits) {
		return nil, nil
	}
	if cdl.commits[left].notComparable() || cdl.commits[right].notComparable() {
		return nil, nil
	}

	return compare.ComparePerformance(cdl.commits[left].values, cdl.commits[right].values, rawDiff)
}

// notComparable checks if a commit is still waiting on something to finish
// TODO(sunxiaodi@) capture case where commit A has already compared commit B
func (c *commitData) notComparable() bool {
	// build may not be queued up
	if c.build == nil {
		return true
	}
	// build or tests are still running
	// more tests can be scheduled even after the
	// commit generates values
	// tests may never generate if build failed
	if c.tests == nil || c.tests.isRunning {
		return true
	}
	// it is possible for all tests to finish but
	// generate no values if all tests failed
	if c.values == nil {
		return true
	}

	return false
}

// updateCommitsByResult takes the compare results and determines the next
// steps in the workflow. Changes are made to CommitDataList depending
// on what the compare verdict is.
func (cdl *commitDataList) updateCommitsByResult(ctx context.Context, sc backends.SwarmingClient, mh *midpoint.MidpointHandler, res *compare.CompareResults, left, right int, req *ppb.ScheduleBisectRequest) (*midpoint.Commit, error) {
	if left < 0 || right >= len(cdl.commits) {
		return nil, skerr.Fmt("cannot update commitDataList with left %d and right %d index out of bounds", left, right)
	}
	if left >= right {
		return nil, skerr.Fmt("cannot update commitDataList with left %d index >= right %d", left, right)
	}
	if res.Verdict == compare.Unknown {
		return nil, cdl.runMoreTestsIfNeeded(ctx, sc, left, right, req)
	} else if res.Verdict == compare.Different {
		return cdl.findMidpointOrCulprit(ctx, mh, left, right)
	}
	return nil, nil
}

// findMidpointOrCulprit updates the commitDataList with either a new midpoint
// or returns a culprit if the midpoint is the same as the left commit
// TODO(sunxiaodi@) create mock for MidpointHandler and create unit tests
func (cdl *commitDataList) findMidpointOrCulprit(ctx context.Context, mc *midpoint.MidpointHandler, left, right int) (
	*midpoint.Commit, error) {
	lcommit := cdl.commits[left].commit
	rcommit := cdl.commits[right].commit
	sklog.Debugf("commit left %s vs commit right %s", lcommit.GitHash[:7], rcommit.GitHash[:7])
	m, err := mc.FindMidCommit(ctx, lcommit, rcommit)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not get midpoint between [%d] %s and [%d] %s",
			left, lcommit.GitHash, right, rcommit.GitHash)
	}
	// culprit found
	if m.GitHash == lcommit.GitHash {
		return rcommit, nil
	}
	// append mid commit in between left and right
	cdl.commits = append(cdl.commits[:right], cdl.commits[left:]...)
	cdl.commits[right] = &commitData{
		commit: m,
	}
	return nil, nil
}

// runMoreTestsIfNeeded adds more run_benchmark tasks to the left and right commit
func (cdl *commitDataList) runMoreTestsIfNeeded(ctx context.Context, sc backends.SwarmingClient, left, right int, req *ppb.ScheduleBisectRequest) error {
	c := cdl.commits[left]
	tasks, err := c.scheduleRunBenchmark(ctx, sc, req)
	if err != nil {
		return skerr.Wrapf(err, "could not schedule more tasks for left commit [%d] %s", left, c.commit.GitHash[:7])
	}
	if len(tasks) > 0 {
		c.tests.tasks = tasks
		c.tests.isRunning = true
	}
	c = cdl.commits[right]
	tasks, err = c.scheduleRunBenchmark(ctx, sc, req)
	if err != nil {
		return skerr.Wrapf(err, "could not schedule more tasks for right commit [%d] %s", right, c.commit.GitHash[:7])
	}
	if len(tasks) > 0 {
		c.tests.tasks = tasks
		c.tests.isRunning = true
	}
	return nil
}
