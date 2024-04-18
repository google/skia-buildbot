// Package workflow contains const and types to invoke Workflows.
package workflows

import (
	"strconv"
	"strings"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/run_benchmark"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

// Workflow name definitions.
//
// Those are used to invoke the workflows. This is meant to decouple the
// souce code dependencies such that the client doesn't need to link with
// the actual implementation.
// TODO(b/326352379): introduce a specific type to encapsulate these workflow names
const (
	Bisect                            = "perf.bisect"
	BuildChrome                       = "perf.build_chrome"
	CatapultBisect                    = "perf.catapult.bisect"
	ConvertToCatapultResponseWorkflow = "perf.catapult.response"
	RunBenchmark                      = "perf.run_benchmark"
	SingleCommitRunner                = "perf.single_commit_runner"
	PairwiseCommitsRunner             = "perf.pairwise_commits_runner"
	BugUpdate                         = "perf.bug_update"
)

// Workflow params definitions.
//
// Each workflow defines its own struct for the params, this will ensure
// the input parameter type safety, as well as expose them in a structured way.
type BuildChromeParams struct {
	// WorkflowID is arbitrary string that tags the build.
	// This is used to connect the downstream and know which build is used.
	// This is usually the pinpoint job ID.
	WorkflowID string
	// Commit is the chromium commit hash.
	Commit *midpoint.CombinedCommit
	// Device is the name of the device, e.g. "linux-perf".
	Device string
	// Target is name of the build isolate target
	// e.g. "performance_test_suite".
	Target string
	// Patch is the Gerrit patch included in the build.
	Patch []*buildbucketpb.GerritChange
}

// Build stores the build from Buildbucket.
type Build struct {
	// The parameters used to make this build.
	BuildChromeParams
	// ID is the buildbucket ID of the Chrome build.
	// https://github.com/luci/luci-go/blob/19a07406e/buildbucket/proto/build.proto#L138
	ID int64
	// Status is the status of the build, this is needed to surface the build failures.
	Status buildbucketpb.Status
	// CAS is the CAS address of the build isolate.
	CAS *swarmingV1.SwarmingRpcsCASReference
}

// TestRun stores individual benchmark test run.
type TestRun struct {
	// TaskID is the swarming task ID.
	TaskID string
	// Status is the swarming task status.
	Status run_benchmark.State
	// CAS is the CAS address of the test output.
	CAS *swarmingV1.SwarmingRpcsCASReference
	// Values is sampled values for each benchmark story.
	Values map[string][]float64
}

type BisectParams struct {
	// BisectWorkflow reuses BisectRequest message
	Request *pb.ScheduleBisectRequest
}

// GetMagnitude returns the magnitude as float64.
//
// If the given string value is invalid or unable to parse, it returns the default 1.0.
func (bp *BisectParams) GetMagnitude() float64 {
	if bp.Request.ComparisonMagnitude == "" {
		return 1.0
	}

	magnitude, err := strconv.ParseFloat(bp.Request.ComparisonMagnitude, 64)
	if err != nil {
		return 1.0
	}
	return magnitude
}

// GetInitialAttempt returns the initial attempt as int32.
//
// If the given string value is invalid or unable to parse, it returns the default 0.
func (bp *BisectParams) GetInitialAttempt() int32 {
	if bp.Request.InitialAttemptCount == "" {
		return 0
	}
	attempt, err := strconv.ParseInt(bp.Request.InitialAttemptCount, 10, 32)
	if err != nil {
		return 0
	}
	return int32(attempt)
}

// GetImprovementDirection returns the improvement direction.
//
// Returns Unknown by default regardless of input.
func (bp *BisectParams) GetImprovementDirection() compare.ImprovementDir {
	switch strings.ToLower(bp.Request.ImprovementDirection) {
	case strings.ToLower(string(compare.Up)):
		return compare.Up
	case strings.ToLower(string(compare.Down)):
		return compare.Down
	}
	return compare.UnknownDir
}
