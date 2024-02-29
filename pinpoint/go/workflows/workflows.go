// Package workflow contains const and types to invoke Workflows.
package workflows

import (
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/pinpoint/go/midpoint"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

// Workflow name definitions.
//
// Those are used to invoke the workflows. This is meant to decouple the
// souce code dependencies such that the client doesn't need to link with
// the actual implementation.
// TODO(b/326352379): introduce a specific type to encapsulate these workflow names
const (
	Bisect             = "perf.bisect"
	BuildChrome        = "perf.build_chrome"
	RunBenchmark       = "perf.run_benchmark"
	SingleCommitRunner = "perf.single_commit_runner"
)

// Workflow params definitions.
//
// Each workflow defines its own struct for the params, this will ensure
// the input parameter type safety, as well as expose them in a structured way.
type BuildChromeParams struct {
	// PinpointJobID is the Job ID to associate with the build.
	PinpointJobID string
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
	Status string
	// CAS is the CAS address of the test output.
	CAS *swarmingV1.SwarmingRpcsCASReference
	// Values is sampled values for each benchmark story.
	Values map[string][]float64
}

type BisectParams struct {
	// BisectWorkflow reuses BisectRequest message
	Request *pb.ScheduleBisectRequest
}
