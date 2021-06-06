package continuous

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/android_ingest/go/buildapi"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestRationalize(t *testing.T) {
	unittest.SmallTest(t)

	// startTS is much later.
	builds := []buildapi.Build{
		{TS: 120},
		{TS: 123},
		{TS: 125},
	}
	builds = rationalizeTimestamps(builds, 200)
	assert.Equal(t, int64(201), builds[0].TS)
	assert.Equal(t, int64(202), builds[1].TS)
	assert.Equal(t, int64(203), builds[2].TS)

	// startTS is in middle.
	builds = []buildapi.Build{
		{TS: 120},
		{TS: 123},
		{TS: 125},
	}
	builds = rationalizeTimestamps(builds, 122)
	assert.Equal(t, int64(123), builds[0].TS)
	assert.Equal(t, int64(124), builds[1].TS)
	assert.Equal(t, int64(125), builds[2].TS)

	// startTS is way before.
	builds = []buildapi.Build{
		{TS: 120},
		{TS: 123},
		{TS: 125},
	}
	builds = rationalizeTimestamps(builds, 100)
	assert.Equal(t, int64(120), builds[0].TS)
	assert.Equal(t, int64(121), builds[1].TS)
	assert.Equal(t, int64(122), builds[2].TS)
}

func TestSort(t *testing.T) {
	unittest.SmallTest(t)
	builds := []buildapi.Build{
		{TS: 120, BuildId: 12},
		{TS: 123, BuildId: 11},
		{TS: 125, BuildId: 13},
	}
	sort.Sort(BuildSlice(builds))
	assert.Equal(t, int64(11), builds[0].BuildId)
	assert.Equal(t, int64(123), builds[0].TS)
	assert.Equal(t, int64(12), builds[1].BuildId)
	assert.Equal(t, int64(13), builds[2].BuildId)
}

const startBuildID int64 = 123
const startTS int64 = 1622980000

func TestBuildsFromStartToMostRecent_Simple_Success(t *testing.T) {
	unittest.SmallTest(t)
	builds := buildsFromStartToMostRecent(startBuildID, startTS, startBuildID+2, startTS+20)
	expected := []buildapi.Build{
		{BuildId: startBuildID + 1, TS: startTS + 19},
		{BuildId: startBuildID + 2, TS: startTS + 20},
	}
	assert.Equal(t, expected, builds)
}

func TestBuildsFromStartToMostRecent_MatchingBeginAndEndTime_ReturnsEmptySlice(t *testing.T) {
	unittest.SmallTest(t)
	builds := buildsFromStartToMostRecent(startBuildID, startTS, startBuildID, startTS)
	expected := []buildapi.Build{}
	assert.Equal(t, expected, builds)
}

func TestBuildsFromStartToMostRecent_NotEnoughSecondsToHaveOneCommitPerSecond_ReturnsOnlyMostRecentBuildsThatWillFit(t *testing.T) {
	unittest.SmallTest(t)
	builds := buildsFromStartToMostRecent(startBuildID, startTS, startBuildID+4, startTS+2)
	expected := []buildapi.Build{
		{BuildId: startBuildID + 3, TS: startTS + 1},
		{BuildId: startBuildID + 4, TS: startTS + 2},
	}
	assert.Equal(t, expected, builds)
}
