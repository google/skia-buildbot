package replaybackends

import (
	"context"
	"path/filepath"
	"testing"

	"go.skia.org/infra/bazel/go/bazel"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromZipFile(t *testing.T) {
	path := filepath.Join(
		bazel.RunfilesDir(),
		"external/cabe_replay_data",
		// https://pinpoint-dot-chromeperf.appspot.com/job/16f46f1c260000
		"pinpoint_16f46f1c260000.zip")
	benchmarkName := "fake benchmark name"
	replayers := FromZipFile(
		path,
		benchmarkName,
	)
	require.NotNil(t, replayers, "replayers was nil when it should not be")

	assert.NotNil(t, replayers.CASResultReader, "CASResultReader was nil when it should not be")
	assert.NotNil(t, replayers.SwarmingTaskReader, "SwarmingTaskReader was nil when it should not be")

	ctx := context.Background()
	instance := "projects/chrome-swarming/instances/default_instance"
	digest := "587d9372661b9506c3df2ef384532a1215901beeda01a3002772be4ead97d480/178"

	casRes, err := replayers.CASResultReader(ctx, instance, digest)
	assert.NoError(t, err)
	pr := casRes[benchmarkName]
	require.NotNil(t, pr)
	assert.Equal(t, 42, len(pr.Histograms))
	assert.Equal(t, "blink_decode_time_gpu_rasterization", pr.Histograms[0].Name)

	swarmingRes, err := replayers.SwarmingTaskReader(ctx, "16f46f1c260000")
	assert.NoError(t, err)
	require.NotNil(t, swarmingRes)
	assert.Equal(t, 130, len(swarmingRes))
}
