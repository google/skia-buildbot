package pinpoint

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/bot_configs"
	"go.skia.org/infra/pinpoint/go/build_chrome"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/read_values"
	"golang.org/x/oauth2/google"

	bpb "go.chromium.org/luci/buildbucket/proto"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
)

const (
	missingRequiredParamTemplate = "Missing required param %s"
	chromiumSrcGit               = "https://chromium.googlesource.com/chromium/src.git"
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

// commitData stores relevant metadata pertaining to the specific commit
type commitData struct {
	commit *midpoint.Commit
	build  *buildMetadata
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

	bc, err := build_chrome.New(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not create buildbucket client")
	}

	resp := &PinpointRunResponse{
		JobID:    jobID,
		Culprits: nil,
	}

	// TODO(sunxiaodi@): replace with isolate target mapping
	target := "performance_test_suite"

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
		// poll ongoing builds
		// TODO(sunxiaodi@) deprecate polling with pubsub
		i, err := cdl.pollBuild(ctx, bc)
		if err != nil {
			return resp, err
		}
		// retrieve CAS of successful builds
		if i != nil && cdl[*i].build.buildStatus == bpb.Status_SUCCESS {
			cas, err := bc.RetrieveCAS(ctx, cdl[*i].build.buildID, target)
			if err != nil {
				return resp, skerr.Wrapf(err, "Could not retrieve CAS info")
			}
			cdl[*i].build.buildCAS = cas
		}

		time.Sleep(1 * time.Second)
	}
	resp.Commits = cdl
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
	}
	return false
}

// pollBuild checks the build status of every commit in the commitQ
// returns upon finding the first build that was running and finishes
// returns the index of the commit and the build's status
func (cdl commitDataList) pollBuild(ctx context.Context, bc build_chrome.BuildChromeClient) (
	*int, error) {
	for i, c := range cdl {
		if c.build == nil || c.build.buildID == 0 {
			return nil, skerr.Fmt("Cannot poll build of non-existent build")
		}
		status, err := bc.GetStatus(ctx, c.build.buildID)
		if err != nil {
			return nil, skerr.Wrapf(err, "Could not get build status %d", c.build.buildID)
		}
		// check ongoing build
		if c.build.buildStatus < bpb.Status_ENDED_MASK {
			// update the build status
			cdl[i].build.buildStatus = status
			if status > bpb.Status_ENDED_MASK {
				return &i, nil
			}
		}
	}
	return nil, nil
}
