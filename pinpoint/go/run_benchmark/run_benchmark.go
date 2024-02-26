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

	spb "go.chromium.org/luci/common/api/swarming/swarming/v1"
)

// A RunBenchmarkRequest defines the request arguments of the performance test to swarming.
// Note: This is being used in workflows/internal/run_benchmark.go.
type RunBenchmarkRequest struct {
	// the Pinpoint job id
	JobID string
	// the swarming instance and cas digest hash and bytes location for the build
	Build *spb.SwarmingRpcsCASReference
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

// IsTaskStateFinished checks if a swarming task state is finished
func IsTaskStateFinished(state string) (bool, error) {
	if !slices.Contains(swarming.TASK_STATES, state) {
		return false, skerr.Fmt("Not a valid swarming task state %s", state)
	}
	return !slices.Contains(runningStates, state), nil
}

// IsTaskStateSuccess checks if a swarming task state is finished
func IsTaskStateSuccess(state string) bool {
	return state == swarming.TASK_STATE_COMPLETED
}

// Run schedules a swarming task to run the RunBenchmarkRequest.
func Run(ctx context.Context, sc backends.SwarmingClient, commit, bot, benchmark, story, storyTag string, jobID string, buildArtifact *spb.SwarmingRpcsCASReference, iter int) ([]*spb.SwarmingRpcsTaskRequestMetadata, error) {
	botConfig, err := bot_configs.GetBotConfig(bot, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create benchmark test object")
	}

	bt, err := NewBenchmarkTest(commit, botConfig.Bot, botConfig.Browser, benchmark, story, storyTag)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to prepare benchmark test for execution")
	}

	swarmingRequest := createSwarmingRequest(jobID, bt.GetCommand(), buildArtifact, botConfig.Dimensions)

	resp := make([]*spb.SwarmingRpcsTaskRequestMetadata, 0)
	for i := 0; i < iter; i++ {
		r, err := sc.TriggerTask(ctx, swarmingRequest)
		if err != nil {
			return nil, skerr.Wrapf(err, "benchmark task %d with request %v failed", i, r)
		}

		resp = append(resp, r)
	}

	return resp, nil
}
