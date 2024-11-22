// Package workflow contains const and types to invoke Workflows.
package workflows

import (
	"strconv"
	"strings"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/run_benchmark"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

// Workflow name definitions.
//
// Those are used to invoke the workflows. This is meant to decouple the
// souce code dependencies such that the client doesn't need to link with
// the actual implementation.
const (
	Bisect                            = "perf.bisect"
	BuildChrome                       = "perf.build_chrome"
	CatapultBisect                    = "perf.catapult.bisect"
	ConvertToCatapultResponseWorkflow = "perf.catapult.response"
	CulpritFinderWorkflow             = "perf.culprit_finder"
	RunBenchmark                      = "perf.run_benchmark"
	RunBenchmarkPairwise              = "perf.run_benchmark.pairwise"
	SingleCommitRunner                = "perf.single_commit_runner"
	PairwiseCommitsRunner             = "perf.pairwise_commits_runner"
	PairwiseWorkflow                  = "perf.pairwise"
	BugUpdate                         = "perf.bug_update"
	TestAndExport                     = "perf.test_and_export"
	CollectAndUpload                  = "perf.collect_and_upload"
)

const defaultPairwiseAttemptCount int32 = 30

// Workflow params definitions.
//
// Each workflow defines its own struct for the params, this will ensure
// the input parameter type safety, as well as expose them in a structured way.
type BuildParams struct {
	// WorkflowID is arbitrary string that tags the build.
	// This is used to connect the downstream and know which build is used.
	// This is usually the pinpoint job ID.
	WorkflowID string
	// Commit is the chromium commit hash.
	Commit *common.CombinedCommit
	// Device is the name of the device, e.g. "linux-perf".
	Device string
	// Target is name of the build isolate target
	// e.g. "performance_test_suite".
	Target string
	// Patch is the Gerrit patch included in the build.
	Patch []*buildbucketpb.GerritChange
	// Project is the project the Build workflow is being run for.
	// For example, Chromium and V8 would be under project "chromium" and create
	// a Chrome binary. AndroidX would have project "androidx" and generate
	// Android X modules.
	Project string
}

// Build stores the build from Buildbucket.
type Build struct {
	// The parameters used to make this build.
	BuildParams
	// ID is the buildbucket ID of the Chrome build.
	// https://github.com/luci/luci-go/blob/19a07406e/buildbucket/proto/build.proto#L138
	ID int64
	// Status is the status of the build, this is needed to surface the build failures.
	Status buildbucketpb.Status
	// CAS is the CAS address of the build isolate.
	CAS *apipb.CASReference
}

// TestRun stores individual benchmark test run.
type TestRun struct {
	// TaskID is the swarming task ID.
	TaskID string
	// Status is the swarming task status.
	Status run_benchmark.State
	// CAS is the CAS address of the test output.
	CAS *apipb.CASReference
	// Values is sampled values for each benchmark story.
	Values map[string][]float64
}

// IsEmptyValues checks the TestRun if there are values at that chart
func (tr *TestRun) IsEmptyValues(chart string) bool {
	return tr == nil || tr.Values == nil || tr.Values[chart] == nil
}

// RemoveDataFromChart removes chart data from that TestRun
func (tr *TestRun) RemoveDataFromChart(chart string) {
	if tr.Values != nil {
		tr.Values[chart] = nil
	}
}

// PairwiseOrder indicates in a pairwise run, which commit ran first
type PairwiseOrder int

const (
	// LeftThenRight means Left commit ran first, then the Right commit
	LeftThenRight PairwiseOrder = 0
	// RightThenLeft means Right commit ran first, then the Left commit
	RightThenLeft PairwiseOrder = 1
)

// PairwiseTestRun stores pairwise benchmark test run.
type PairwiseTestRun struct {
	// FirstTestRun is the first benchmark test run
	FirstTestRun *TestRun
	// SecondTestRun is the second benchmark test run
	SecondTestRun *TestRun
	// First indicates which commit ran first.
	// First = Left commit or Right commit.
	Permutation PairwiseOrder
}

type BisectParams struct {
	// BisectWorkflow reuses BisectRequest message
	Request *pb.ScheduleBisectRequest
	// Available bot list.
	// This field is for internal use. Clients of BisectionWorkflow(s) are not expected to set it.
	BotIds []string
	// Production if true indicates the workflow is intended to be run on production
	// and not the dev or staging environment.
	// Used to determine whether to write to Pinpoint prod or staging.
	Production bool
	// JobID for the bisect run
	JobID string
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
	if attempt < 0 {
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

type PairwiseParams struct {
	// PairwiseWorkflow reuses SchedulePairwiseRequest proto request
	Request *pb.SchedulePairwiseRequest

	// CulpritVerify when true states the pairwise job is a culprit
	// verification job.
	CulpritVerify bool
}

// GetInitialAttempt returns the initial attempt as int32.
//
// Pairwise analysis needs to run an even number of commits to ensure an equal number
// of pairs where commit A goes first and commit B goes first.
// If the given string value is invalid or unable to parse, it returns the default 30.
func (pp *PairwiseParams) GetInitialAttempt() int32 {
	if pp.Request.InitialAttemptCount == "" {
		return defaultPairwiseAttemptCount
	}
	attempt, err := strconv.ParseInt(pp.Request.InitialAttemptCount, 10, 32)
	// TODO(sunxiaodi@): Cover invalid input error cases upstream of this function call
	if err != nil {
		return defaultPairwiseAttemptCount
	}
	if attempt < 0 {
		return defaultPairwiseAttemptCount
	}
	// use bit shifting to ensure response is always even
	return int32((attempt + 1) >> 1 << 1)
}

// GetImprovementDirection returns the improvement direction.
//
// Returns Unknown by default.
func (pp *PairwiseParams) GetImprovementDirection() compare.ImprovementDir {
	switch strings.ToLower(pp.Request.ImprovementDirection) {
	case strings.ToLower(string(compare.Up)):
		return compare.Up
	case strings.ToLower(string(compare.Down)):
		return compare.Down
	}
	return compare.UnknownDir
}

type CulpritFinderParams struct {
	// CulpritFinderParams embeds the pinpoint proto ScheduleCulpritFinderRequest
	Request *pb.ScheduleCulpritFinderRequest
	// Production if true indicates the workflow is intended to be run on production
	// and not the dev or staging environment.
	// Used to determine whether to write to Pinpoint prod or staging.
	Production bool
	// A set of parameters used for callback to culprit service if any culprit is found.
	CallbackParams *pb.CulpritProcessingCallbackParams
}
