package perfresults

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
)

func makeTestLoader(t *testing.T, replayName string) loader {
	return loader{
		client: setupReplay(t, replayName+".json"),
		rbeProvider: func(ctx context.Context, casInstance string) (*RBEPerfLoader, error) {
			return newRBEReplay(t, ctx, casInstance, replayName), nil
		},
	}
}

func Test_GetNonNilCAS_RemovesNilCAS(t *testing.T) {
	// Sanity check to make sure getNonNilCAS can remove all the nil values as we don't find the actual
	// examples that have partial success runs.
	assert.Len(t, getNonNilCAS(nil, nil), 0)
	assert.Len(t, getNonNilCAS(&apipb.CASReference{}, nil), 1)
	assert.Len(t, getNonNilCAS(&apipb.CASReference{}, &apipb.CASReference{}, &apipb.CASReference{}), 3)
	assert.Len(t, getNonNilCAS(&apipb.CASReference{}, nil, &apipb.CASReference{}), 2)
	assert.Len(t, getNonNilCAS(nil, nil, &apipb.CASReference{}), 1)
}

func Test_LoadPerfResults_InvalidBuildID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l := makeTestLoader(t, "LoadPerfResults_InvalidBuildID")

	// https://ci.chromium.org/ui/p/chrome/builders/ci/android-pixel2-processor-perf/29185/overview
	_, pf, err := l.LoadPerfResults(ctx, 874985279402)
	assert.ErrorContains(t, err, "unable to get build info")
	assert.Empty(t, pf)
}

func Test_LoadPerfResults_NoChildRuns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l := makeTestLoader(t, "LoadPerfResults_NoChildRuns")

	// https://ci.chromium.org/ui/p/chrome/builders/ci/android-pixel2-processor-perf/29185/overview
	bi, pf, err := l.LoadPerfResults(ctx, 8749852794028752801)
	assert.NoError(t, err)
	assert.Empty(t, pf)
	assert.EqualValues(t, bi, BuildInfo{
		SwarmingInstance: "chrome-swarming.appspot.com",
		BuilderName:      "android-pixel2-processor-perf",
		MachineGroup:     "ChromiumPerf",
		TaskID:           "692473e4c9b8d410",
		Revision:         "c845ea1817639802e52815b0a401391ea5d72282",
		CommitPosisition: "refs/heads/main@{#1291128}",
	})
}

func Test_LoadPerfResults_ValidFullResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l := makeTestLoader(t, "LoadPerfResults_ValidFullResults")

	// https://ci.chromium.org/ui/p/chrome/builders/ci/mac-m1_mini_2020-perf-pgo/5691/infra
	bi, pf, err := l.LoadPerfResults(ctx, 8749893364195553889)
	require.NoError(t, err)
	require.Contains(t, pf, "speedometer3")
	assert.Len(t, pf["speedometer3"].Histograms, 21)
	assert.Len(t, pf["speedometer3"].GetSampleValues("TodoMVC-Vue"), 40)
	assert.EqualValues(t, bi, BuildInfo{
		SwarmingInstance: "chrome-swarming.appspot.com",
		BuilderName:      "mac-m1_mini_2020-perf-pgo",
		MachineGroup:     "ChromiumPerfPGO",
		TaskID:           "6922258402e54910",
		Revision:         "958d24898d18c3fcd3cefdb7c994b39672261813",
		CommitPosisition: "refs/heads/main@{#1291040}",
	})
}
