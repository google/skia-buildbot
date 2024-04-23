package perfresults

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_FindTaskID_ReturnsInstanceAndTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hc := setupReplay(t, "FindTaskID_ReturnsInstanceAndTask.json")
	bc, err := newBuildsClient(ctx, hc)
	require.NoError(t, err)

	verify := func(buildID int64, expectedInstance string, expectedTaskID string) {
		ti, err := bc.findBuildInfo(ctx, buildID)
		assert.NoError(t, err)
		assert.EqualValues(t, expectedInstance, ti.SwarmingInstance)
		assert.EqualValues(t, expectedTaskID, ti.TaskID)
	}

	// Chomium regular builders
	// https://ci.chromium.org/ui/p/chromium/builders/ci/linux-archive-rel/115765/infra
	verify(8750653768417566929, "chromium-swarm.appspot.com", "68f6ec30e9df0310")
	// https://ci.chromium.org/ui/p/chromium/builders/ci/mac-archive-rel/55565/infra
	verify(8750655875863273681, "chromium-swarm.appspot.com", "68f6cd84ab9c7910")
	// https://ci.chromium.org/ui/p/chromium/builders/ci/win32-official/7510/infra
	verify(8750671169141207617, "chromium-swarm.appspot.com", "68f5eef7ad72d410")
	// https://ci.chromium.org/ui/p/chromium/builders/ci/android-arm64-archive-rel/8140/infra
	verify(8750661999713621649, "chromium-swarm.appspot.com", "68f674688a5b2a10")

	// Perf CI compilers/builders
	// https://ci.chromium.org/ui/p/chrome/builders/ci/android_arm64_high_end-builder-perf-pgo/3756/infra
	verify(8750654771398709393, "chrome-swarming.appspot.com", "68f6dd95955dcd10")
	// https://ci.chromium.org/ui/p/chrome/builders/ci/mac-arm-builder-perf-pgo/5908/infra
	verify(8750664753482601809, "chrome-swarming.appspot.com", "68f64c57b665c210")

	// Perf CI testers
	// https://ci.chromium.org/ui/p/chrome/builders/ci/win-10_amd_laptop-perf/132147/infra
	verify(8750652500268361201, "chrome-swarming.appspot.com", "68f6fea3323f3510")
	// https://ci.chromium.org/ui/p/chrome/builders/ci/android-go-wembley_webview-perf/20096/infra
	verify(8750655643337951697, "chrome-swarming.appspot.com", "68f6d0e69a6d7010")
	// https://ci.chromium.org/ui/p/chrome/builders/ci/android-pixel6-perf-pgo/2015/infra
	verify(8750656867744283697, "chrome-swarming.appspot.com", "68f6bf166ca58810")
}
