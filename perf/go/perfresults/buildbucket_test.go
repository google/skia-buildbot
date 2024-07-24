package perfresults

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_BuildInfo_ReturnsValidPosition(t *testing.T) {
	validate := func(cp, expected string) {
		assert.EqualValues(t, expected, BuildInfo{CommitPosisition: cp}.GetPosition())
	}

	// Full qualified position string
	validate("refs/heads/main@{#1294264}", "CP:1294264")

	// Only contains numbers
	validate("main@{#1294264}", "CP:1294264")
}

func Test_BuildInfo_ReturnsRevision(t *testing.T) {
	validate := func(cp, revision, expected string) {
		assert.EqualValues(t, expected, BuildInfo{
			Revision:         revision,
			CommitPosisition: cp,
		}.GetPosition())
	}

	const aRevision = "f6db5c95c4099889f96cdad9ca1a067dbcb5fbaa"

	// Empty CP returns the revision instead
	validate("", aRevision, aRevision)

	// Missing numbers return git revision
	validate("refs/heads/main@{#}", aRevision, aRevision)
}

func Test_FindTaskID_ReturnsInstanceAndTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hc := setupReplay(t, "FindTaskID_ReturnsInstanceAndTask.json")
	bc, err := newBuildsClient(ctx, hc)
	require.NoError(t, err)

	verify := func(buildID int64, expectedBuilder, expectedMachineGroup, expectedInstance, expectedTaskID string) {
		ti, err := bc.findBuildInfo(ctx, buildID)
		assert.NoError(t, err)
		assert.EqualValues(t, expectedBuilder, ti.BuilderName)
		assert.EqualValues(t, expectedMachineGroup, ti.MachineGroup)
		assert.EqualValues(t, expectedInstance, ti.SwarmingInstance)
		assert.EqualValues(t, expectedTaskID, ti.TaskID)
	}

	// Chomium regular builders
	// https://ci.chromium.org/ui/p/chromium/builders/ci/linux-archive-rel/115765/infra
	verify(8750653768417566929, "linux-archive-rel", "", "chromium-swarm.appspot.com", "68f6ec30e9df0310")
	// https://ci.chromium.org/ui/p/chromium/builders/ci/mac-archive-rel/55565/infra
	verify(8750655875863273681, "mac-archive-rel", "", "chromium-swarm.appspot.com", "68f6cd84ab9c7910")
	// https://ci.chromium.org/ui/p/chromium/builders/ci/win32-official/7510/infra
	verify(8750671169141207617, "win32-official", "", "chromium-swarm.appspot.com", "68f5eef7ad72d410")
	// https://ci.chromium.org/ui/p/chromium/builders/ci/android-arm64-archive-rel/8140/infra
	verify(8750661999713621649, "android-arm64-archive-rel", "", "chromium-swarm.appspot.com", "68f674688a5b2a10")

	// Perf CI compilers/builders
	// https://ci.chromium.org/ui/p/chrome/builders/ci/android_arm64_high_end-builder-perf-pgo/3756/infra
	verify(8750654771398709393, "android_arm64_high_end-builder-perf-pgo", "ChromiumPerfPGO", "chrome-swarming.appspot.com", "68f6dd95955dcd10")
	// https://ci.chromium.org/ui/p/chrome/builders/ci/mac-arm-builder-perf-pgo/5908/infra
	verify(8750664753482601809, "mac-arm-builder-perf-pgo", "ChromiumPerfPGO", "chrome-swarming.appspot.com", "68f64c57b665c210")

	// Perf CI testers
	// https://ci.chromium.org/ui/p/chrome/builders/ci/win-10_amd_laptop-perf/132147/infra
	verify(8750652500268361201, "win-10_amd_laptop-perf", "ChromiumPerf", "chrome-swarming.appspot.com", "68f6fea3323f3510")
	// https://ci.chromium.org/ui/p/chrome/builders/ci/android-go-wembley_webview-perf/20096/infra
	verify(8750655643337951697, "android-go-wembley_webview-perf", "ChromiumPerf", "chrome-swarming.appspot.com", "68f6d0e69a6d7010")
	// https://ci.chromium.org/ui/p/chrome/builders/ci/android-pixel6-perf-pgo/2015/infra
	verify(8750656867744283697, "android-pixel6-perf-pgo", "ChromiumPerfPGO", "chrome-swarming.appspot.com", "68f6bf166ca58810")
}
