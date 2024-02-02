package pinpoint

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/pinpoint/go/bot_configs"
	"go.skia.org/infra/pinpoint/go/build_chrome"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/read_values"
	"go.skia.org/infra/pinpoint/go/run_benchmark"
	"golang.org/x/oauth2/google"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	bpb "go.chromium.org/luci/buildbucket/proto"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
)

const (
	missingRequiredParamTemplate = "Missing required param %s"
	chromiumSrcGit               = "https://chromium.googlesource.com/chromium/src.git"
	maxSampleSize                = 4
	interval                     = 2
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
	Run(ctx context.Context, req PinpointRunRequest, jobID string) (*PinpointRunResponse, error)
}

// PinpointRunRequest is the request arguments to run a Pinpoint job.
type PinpointRunRequest struct {
	// Device is the device to test Chrome on i.e. linux-perf
	Device string
	// Benchmark is the benchmark to test
	Benchmark string
	// Story is the benchmark's story to test
	Story string
	// Chart is the story's subtest to measure. Only used in bisections.
	Chart string
	// Magnitude is the expected absolute difference of a potential regression.
	// Only used in bisections. Default is 1.0.
	Magnitude float64
	// AggregationMethod is the method to aggregate the measurements after a single
	// benchmark runs. Some benchmarks will output multiple values in one
	// run. Aggregation is needed to be consistent with perf measurements.
	// Only used in bisection.
	AggregationMethod read_values.AggDataMethodEnum
	// StartCommit is the base or start commit hash to run
	StartCommit string
	// EndCommit is the experimental or end commit hash to run
	EndCommit string
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
	client *http.Client
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
type commitDataList []*commitData

func New(ctx context.Context) (*pinpointHandlerImpl, error) {
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrapf(err, "Problem setting up default token source")
	}
	c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).With2xxOnly().Client()

	return &pinpointHandlerImpl{
		client: c,
	}, nil
}

// Run implements the pinpointJobImpl interface
func (pp *pinpointHandlerImpl) Run(ctx context.Context, req PinpointRunRequest, jobID string) (
	*PinpointRunResponse, error) {
	if jobID == "" {
		jobID = uuid.New().String()
	}
	err := pp.validateRunRequest(req)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not validate request inputs")
	}
	cfg, err := bot_configs.GetBotConfig(req.Device, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "Device %s not allowed in bot configurations", req.Device)
	}

	bc, err := build_chrome.New(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not create buildbucket client")
	}

	sc, err := run_benchmark.DialSwarming(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not create swarming client")
	}

	resp := &PinpointRunResponse{
		JobID:    jobID,
		Culprits: nil,
	}

	target, err := bot_configs.GetIsolateTarget(req.Device, req.Benchmark)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not get isolate target with device %s and benchmark %s", req.Device, req.Benchmark)
	}

	cdl := commitDataList{
		{
			commit: &midpoint.Commit{
				GitHash:       req.StartCommit,
				RepositoryUrl: chromiumSrcGit,
			},
		},
		{
			commit: &midpoint.Commit{
				GitHash:       req.EndCommit,
				RepositoryUrl: chromiumSrcGit,
			},
		},
	}

	// execute Pinpoint job
	for cdl.shouldContinue() {
		// start builds that have not been scheduled
		for _, c := range cdl {
			if c.build == nil {
				buildID, err := bc.SearchOrBuild(ctx, jobID, c.commit.GitHash, req.Device, nil, nil)
				if err != nil {
					return resp, skerr.Wrapf(err, "could not kick off build for commit %s", c.commit.GitHash)
				}
				c.build = &buildMetadata{
					buildID: buildID,
				}
			}
		}
		// TODO(sunxiaodi@) deprecate polling with pubsub
		c, err := cdl.pollBuild(ctx, bc)
		if err != nil {
			return resp, err
		}
		// retrieve CAS of successful builds and schedule new benchmark runs
		if c != nil && c.build.buildStatus == bpb.Status_SUCCESS {
			cas, err := bc.RetrieveCAS(ctx, c.build.buildID, target)
			if err != nil {
				return resp, skerr.Wrapf(err, "Could not retrieve CAS info")
			}
			c.build.buildCAS = cas
			c.tests = &testMetadata{
				req: c.createRunBenchmarkRequest(jobID, cfg, target, req),
			}
			tasks, err := c.scheduleRunBenchmark(ctx, sc)
			if err != nil {
				return resp, err
			}
			if len(tasks) > 0 {
				c.tests.tasks = tasks
				c.tests.isRunning = true
			}
		}
		// TODO(sunxiaodi@) deprecate polling with pubsub
		_, c, err = cdl.pollTests(ctx, sc)
		if err != nil {
			return resp, err
		}
		if c != nil {
			cas, err := c.getTestCAS(ctx, sc)
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
		}
		resp.Commits = cdl
		time.Sleep(5 * time.Second)
	}
	return resp, nil
}

// validateRunRequest validates the request args and returns an error if there request is invalid
func (pp *pinpointHandlerImpl) validateRunRequest(req PinpointRunRequest) error {
	if req.StartCommit == "" {
		return skerr.Fmt(missingRequiredParamTemplate, "start commit")
	}
	if req.EndCommit == "" {
		return skerr.Fmt(missingRequiredParamTemplate, "end commit")
	}
	_, err := bot_configs.GetBotConfig(req.Device, false)
	if err != nil {
		return skerr.Wrapf(err, "Device %s not allowed in bot configurations", req.Device)
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

func (cdl commitDataList) shouldContinue() bool {
	for _, c := range cdl {
		if c.build == nil || c.build.buildStatus < bpb.Status_ENDED_MASK {
			return true
		}
		if c.tests != nil && c.tests.isRunning {
			return true
		}
	}
	return false
}

// pollBuild checks the build status of every commit in the commitQ
// returns upon finding the first build that was running and finishes
func (cdl commitDataList) pollBuild(ctx context.Context, bc build_chrome.BuildChromeClient) (
	*commitData, error) {
	for _, c := range cdl {
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
func (c *commitData) createRunBenchmarkRequest(jobID string, cfg bot_configs.BotConfig, target string, req PinpointRunRequest) *run_benchmark.RunBenchmarkRequest {
	return &run_benchmark.RunBenchmarkRequest{
		JobID:     jobID,
		Build:     c.build.buildCAS,
		Commit:    c.commit.GitHash,
		Config:    cfg,
		Benchmark: req.Benchmark,
		Story:     req.Story,
		Target:    target,
	}
}

// scheduleRunBenchmark schedules run benchmark tests to swarming and returns the task IDs
func (c *commitData) scheduleRunBenchmark(ctx context.Context, sc swarming.ApiClient) ([]string, error) {
	if c.tests == nil || c.tests.req == nil {
		return nil, skerr.Fmt("Cannot schedule benchmark runs without request")
	}
	// Fetching Pinpoint tasks here can skip scheduling new tasks for faster testing
	tasks, err := run_benchmark.ListPinpointTasks(ctx, sc, *c.tests.req)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not list tasks prior to run benchmark for request %v", *c.tests.req)
	}
	if len(tasks) < maxSampleSize {
		for i := 0; i < interval; i++ {
			task, err := run_benchmark.Run(ctx, sc, *c.tests.req)
			if err != nil {
				return nil, skerr.Wrapf(err, "Could not start run benchmark task for request %v", c.tests.req)
			}
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

// pollTests checks the test status of every commit in the commitQ
// returns upon finding the first commit with running tasks that all finished
// returns the index of the commit so it is easier to compare left and right neighbors
func (cdl commitDataList) pollTests(ctx context.Context, sc swarming.ApiClient) (
	int, *commitData, error) {
	for i, c := range cdl {
		if c.tests == nil {
			continue
		}
		if c.tests.isRunning {
			c.tests.isRunning = false
			states, err := run_benchmark.GetStates(ctx, sc, c.tests.tasks)
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
func (c *commitData) getTestCAS(ctx context.Context, sc swarming.ApiClient) (
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
			cas, err := run_benchmark.GetCASOutput(ctx, sc, c.tests.tasks[i])
			if err != nil {
				return nil, skerr.Wrapf(err, "error retrieving cas outputs")
			}
			casOutputs = append(casOutputs, cas)
		}
	}
	return casOutputs, nil
}

// getValues will return the values from a set of swarming test cas outputs
func (c *commitData) getValues(ctx context.Context, rc *rbeclient.Client, req PinpointRunRequest) (
	[]float64, error) {
	if c.tests == nil {
		return nil, skerr.Fmt("cannot retrieve values with no swarming tests")
	}
	if c.tests.casOutputs == nil {
		return nil, skerr.Fmt("cannot retrieve values with no swarming test cas outputs")
	}
	return read_values.ReadValuesByChart(ctx, rc, req.Benchmark,
		req.Chart, c.tests.casOutputs, req.AggregationMethod)
}
