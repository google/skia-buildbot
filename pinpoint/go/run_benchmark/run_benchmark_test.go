package run_benchmark

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/swarming/mocks"
	"go.skia.org/infra/pinpoint/go/backends"

	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	ppb "go.skia.org/infra/pinpoint/proto/v1"
)

var req = RunBenchmarkRequest{
	JobID:     "id",
	Benchmark: "benchmark",
	Story:     "story",
	Build: &swarmingV1.SwarmingRpcsCASReference{
		CasInstance: "instance",
		Digest: &swarmingV1.SwarmingRpcsDigest{
			Hash:      "hash",
			SizeBytes: 0,
		},
	},
	Commit: "64893ca6294946163615dcf23b614afe0419bfa3",
}
var expectedErr = skerr.Fmt("some error")

func TestRun_TelemetryTest_ValidExecution(t *testing.T) {
	ctx := context.Background()
	mockClient := mocks.NewApiClient(t)
	sc := &backends.SwarmingClientImpl{
		ApiClient: mockClient,
	}

	bisectReq := &ppb.ScheduleBisectRequest{
		Story:         "story",
		StoryTags:     "all",
		Configuration: "android-pixel2_webview-perf",
		Benchmark:     "performance_browser_tests",
	}

	buildArtifact := &swarmingV1.SwarmingRpcsCASReference{
		CasInstance: "instance",
		Digest: &swarmingV1.SwarmingRpcsDigest{
			Hash:      "hash",
			SizeBytes: 0,
		},
	}

	commit := "64893ca6294946163615dcf23b614afe0419bfa3"

	mockClient.On("TriggerTask", ctx, mock.Anything).
		Return(&swarmingV1.SwarmingRpcsTaskRequestMetadata{
			TaskId: "123",
		}, nil).Once()
	taskIds, err := Run(ctx, sc, bisectReq, commit, "id", buildArtifact, 1)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(taskIds))
	assert.Equal(t, "123", taskIds[0].TaskId)
}

func TestIsTaskStateFinished_GivenCompleteStates_ReturnsTrue(t *testing.T) {
	states := []string{
		swarming.TASK_STATE_COMPLETED,
		swarming.TASK_STATE_BOT_DIED,
		swarming.TASK_STATE_TIMED_OUT,
	}
	for _, s := range states {
		out, err := IsTaskStateFinished(s)
		assert.True(t, out)
		assert.NoError(t, err)
	}
}

func TestIsTaskStateFinished_GivenRunningStates_ReturnsFalse(t *testing.T) {
	states := []string{
		swarming.TASK_STATE_PENDING,
		swarming.TASK_STATE_RUNNING,
	}
	for _, s := range states {
		out, err := IsTaskStateFinished(s)
		assert.False(t, out)
		assert.NoError(t, err)
	}
}

func TestIsTaskStateFinished_GivenBadStates_ReturnsError(t *testing.T) {
	states := []string{
		"fake_state",
		"another_fake_state",
	}
	for _, s := range states {
		out, err := IsTaskStateFinished(s)
		assert.False(t, out)
		assert.Error(t, err)
	}
}

func TestIsTaskStateSuccess_GivenCompleted_ReturnsTrue(t *testing.T) {
	states := []string{
		swarming.TASK_STATE_COMPLETED,
	}
	for _, s := range states {
		out := IsTaskStateSuccess(s)
		assert.True(t, out)
	}
}

func TestIsTaskStateSuccess_GivenNonCompleted_ReturnsFalse(t *testing.T) {
	states := []string{
		swarming.TASK_STATE_PENDING,
		swarming.TASK_STATE_RUNNING,
		swarming.TASK_STATE_BOT_DIED,
		swarming.TASK_STATE_CANCELED,
		swarming.TASK_STATE_TIMED_OUT,
	}
	for _, s := range states {
		out := IsTaskStateSuccess(s)
		assert.False(t, out)
	}
}
