// Package run_benchmark runs the benchmark, story, metric as a
// on a build of chrome via swarming tasks.
//
// Package run_benchmark also supports various utility functions
// that make it easy to get the performance measuring tasks of
// a Pinpoint job and check their statuses.
package run_benchmark

import (
	"context"
	"slices"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/bot_configs"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
)

// A RunBenchmarkRequest defines the request arguments of the performance test to swarming.
// Note: This is being used in workflows/internal/run_benchmark.go.
type RunBenchmarkRequest struct {
	// the Pinpoint job id
	JobID string
	// the swarming instance and cas digest hash and bytes location for the build
	Build *apipb.CASReference
	// commit hash
	Commit string
	// device configuration
	BotConfig string
	// benchmark to test
	Benchmark string
	// story to test
	Story string
	// story tags for the test
	StoryTags string
	// test target of the job
	Target string
}

var runningStates = []string{
	swarming.TASK_STATE_PENDING,
	swarming.TASK_STATE_RUNNING,
}

type State string

// IsNoResource checks if a swarming task state has state NO_RESOURCE
func (s State) IsNoResource() bool {
	return string(s) == swarming.TASK_STATE_NO_RESOURCE
}

// IsTaskPending checks if a swarming task state is still pending
func (s State) IsTaskPending() bool {
	return string(s) == swarming.TASK_STATE_PENDING
}

// IsTaskFinished checks if a swarming task state is finished
func (s State) IsTaskFinished() bool {
	return !slices.Contains(runningStates, string(s))
}

// IsTaskTerminalFailure checks if a swarming task state is a
// terminal failure - swarming task did not complete benchmark
// execution due to a failure
func (s State) IsTaskTerminalFailure() bool {
	return s.IsTaskFinished() && !s.IsTaskBenchmarkFailure() && !s.IsTaskTimedOut() && !s.IsTaskSuccessful()
}

// IsTaskBenchmarkFailure checks if a swarming task state
// is a completed run benchmark failure
func (s State) IsTaskBenchmarkFailure() bool {
	return string(s) == backends.RunBenchmarkFailure
}

// IsTaskTimedOut checks if a swarming task state is timed out
func (s State) IsTaskTimedOut() bool {
	return string(s) == swarming.TASK_STATE_TIMED_OUT
}

// IsTaskSuccessful checks if a swarming task state is successful
func (s State) IsTaskSuccessful() bool {
	return string(s) == swarming.TASK_STATE_COMPLETED
}

// ConvertToProto
func (s State) ConvertToProto() pinpoint_proto.SwarmingStatus {
	if s.IsTaskSuccessful() {
		return pinpoint_proto.SwarmingStatus_COMPLETED
	} else if s.IsTaskBenchmarkFailure() {
		return pinpoint_proto.SwarmingStatus_BENCHMARK_FAILURE
	} else if !s.IsTaskFinished() {
		return pinpoint_proto.SwarmingStatus_RUNNING
	}
	return pinpoint_proto.SwarmingStatus_FAILURE
}

// Run schedules a swarming task to run the RunBenchmarkRequest.
func Run(ctx context.Context, sc backends.SwarmingClient, commit, bot, benchmark, story, storyTag string, extraArgs []string, jobID string, buildArtifact *apipb.CASReference, iter int, botID map[string]string) ([]*apipb.TaskRequestMetadataResponse, error) {
	botConfig, err := bot_configs.GetBotConfig(bot, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create benchmark test object")
	}

	bt, err := NewBenchmarkTest(commit, botConfig.Bot, botConfig.Browser, benchmark, story, storyTag, extraArgs)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to prepare benchmark test for execution")
	}

	dims := botConfig.Dimensions
	// botID can be nil upon retry or testing smaller workflows
	if botID != nil {
		dims = append(dims, botID)
	}
	swarmingRequest := createSwarmingRequest(jobID, bt.GetCommand(), buildArtifact, dims)

	resp := make([]*apipb.TaskRequestMetadataResponse, 0)
	for i := 0; i < iter; i++ {
		r, err := sc.TriggerTask(ctx, swarmingRequest)
		if err != nil {
			return nil, skerr.Wrapf(err, "benchmark task %d with request %v failed", i, r)
		}

		resp = append(resp, r)
	}

	return resp, nil
}

// Cancel cancels a swarming task.
func Cancel(ctx context.Context, sc backends.SwarmingClient, taskID string) error {
	err := sc.CancelTasks(ctx, []string{taskID})
	if err != nil {
		return skerr.Wrapf(err, "benchmark task %v cancellation failed", taskID)
	}

	return nil
}
