package blame

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	bug_revert "go.skia.org/infra/golden/go/testutils/data_bug_revert"
	three_devices "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

func TestBlamerGetBlamesForTestThreeDevices(t *testing.T) {
	unittest.SmallTest(t)

	blamer, err := New(three_devices.MakeTestTile(), three_devices.MakeTestExpectations())
	require.NoError(t, err)

	bd := blamer.GetBlamesForTest(three_devices.AlphaTest)
	require.Len(t, bd, 1)
	b := bd[0]

	// The AlphaTest becomes untriaged in exactly the third commit for exactly
	// one trace, so blame is able to identify that author as the one
	// and only culprit.
	require.Equal(t, WeightedBlame{
		Author: three_devices.ThirdCommitAuthor,
		Prob:   1,
	}, b)

	bd = blamer.GetBlamesForTest(three_devices.BetaTest)
	require.Len(t, bd, 1)
	b = bd[0]

	// The BetaTest has an untriaged digest in the first commit and is missing
	// data after that, so this is the best that blamer can do.
	require.Equal(t, WeightedBlame{
		Author: three_devices.FirstCommitAuthor,
		Prob:   1,
	}, b)

	bd = blamer.GetBlamesForTest("test_that_does_not_exist")
	require.Len(t, bd, 0)
}

func TestBlamerGetBlameThreeDevices(t *testing.T) {
	unittest.SmallTest(t)

	blamer, err := New(three_devices.MakeTestTile(), three_devices.MakeTestExpectations())
	require.NoError(t, err)
	commits := three_devices.MakeTestCommits()

	// In the first two commits, this untriaged image doesn't show up
	// so GetBlame should return empty.
	bd := blamer.GetBlame(three_devices.AlphaTest, three_devices.AlphaUntriagedDigest, commits[0:2])
	require.Equal(t, BlameDistribution{
		Freq: []int{},
	}, bd)

	// Searching in the whole range should indicates that
	// the commit with index 2 (i.e. the third and last one)
	// has the blame
	bd = blamer.GetBlame(three_devices.AlphaTest, three_devices.AlphaUntriagedDigest, commits[0:3])
	require.Equal(t, BlameDistribution{
		Freq: []int{2},
	}, bd)

	// The BetaUntriagedDigest only shows up in the first commit (index 0)
	bd = blamer.GetBlame(three_devices.BetaTest, three_devices.BetaUntriagedDigest, commits[0:3])
	require.Equal(t, BlameDistribution{
		Freq: []int{0},
	}, bd)

	// Good digests have no blame ever
	bd = blamer.GetBlame(three_devices.BetaTest, three_devices.BetaPositiveDigest, commits[0:3])
	require.Equal(t, BlameDistribution{
		Freq: []int{},
	}, bd)

	// Negative digests have no blame ever
	bd = blamer.GetBlame(three_devices.AlphaTest, three_devices.AlphaNegativeDigest, commits[0:3])
	require.Equal(t, BlameDistribution{
		Freq: []int{},
	}, bd)
}

func TestBlamerGetBlamesForTestBugRevert(t *testing.T) {
	unittest.SmallTest(t)

	blamer, err := New(bug_revert.MakeTestTile(), bug_revert.MakeTestExpectations())
	require.NoError(t, err)

	// The data in bug_revert's TestOne are designed to have the second commit be unambiguously
	// determined to be at fault.
	bd := blamer.GetBlamesForTest(bug_revert.TestOne)
	require.Len(t, bd, 1)

	require.Equal(t, WeightedBlame{
		Author: bug_revert.BuggyAuthor,
		Prob:   1,
	}, bd[0])

	// TestTwo is a little less clear, given some flaky tests and missing data.
	bd = blamer.GetBlamesForTest(bug_revert.TestTwo)
	require.Len(t, bd, 2)

	require.Equal(t, []WeightedBlame{
		{
			Author: bug_revert.InnocentAuthor,
			Prob:   0.5,
		},
		{
			Author: bug_revert.BuggyAuthor,
			Prob:   0.5,
		},
	}, bd)
}

func TestBlamerGetBlameBugRevert(t *testing.T) {
	unittest.SmallTest(t)

	blamer, err := New(bug_revert.MakeTestTile(), bug_revert.MakeTestExpectations())
	require.NoError(t, err)
	commits := bug_revert.MakeTestCommits()

	// The data in bug_revert's TestOne are designed to have the second commit be unambiguously
	// determined to be at fault.
	bd := blamer.GetBlame(bug_revert.TestOne, bug_revert.BravoUntriagedDigest, commits)
	require.NotNil(t, bd)
	require.Equal(t, BlameDistribution{
		Freq: []int{1},
	}, bd)

	// The data in bug_revert's TestTwo are a bit more wishy-washy, with something going on
	// at either the second or third commit.
	bd = blamer.GetBlame(bug_revert.TestTwo, bug_revert.DeltaUntriagedDigest, commits)
	require.NotNil(t, bd)
	require.Equal(t, BlameDistribution{
		Freq: []int{1},
	}, bd)

	bd = blamer.GetBlame(bug_revert.TestTwo, bug_revert.FoxtrotUntriagedDigest, commits)
	require.NotNil(t, bd)
	require.Equal(t, BlameDistribution{
		// From the (incomplete and slightly misleading) data the blamer has to go on,
		// this is the best guess it can make, because it didn't see Foxtrot before
		// the third commit.
		Freq: []int{2},
	}, bd)

	// TODO(kjlubick): Is it possible to have a test case that has more than 1 index in Freq?
}

// TestBlamerCalculateBugRevert checks that the initial calculate returns the correct data.
func TestBlamerCalculateBugRevert(t *testing.T) {
	unittest.SmallTest(t)

	blamer, err := New(bug_revert.MakeTestTile(), bug_revert.MakeTestExpectations())
	require.NoError(t, err)

	require.Equal(t, bug_revert.MakeTestCommits(), blamer.commits)
	require.Equal(t, map[types.TestName]map[types.Digest]blameCounts{
		bug_revert.TestOne: {
			bug_revert.BravoUntriagedDigest: {
				4, 0, 0, 0,
			},
		},
		bug_revert.TestTwo: {
			bug_revert.DeltaUntriagedDigest: {
				2, 0, 0, 0,
			},
			bug_revert.FoxtrotUntriagedDigest: {
				1, 2, 0, 0,
			},
		},
	}, blamer.blameLists)
}

func TestBlamerCalculateBugRevertPossibleGlitch(t *testing.T) {
	unittest.SmallTest(t)

	tile := bug_revert.MakeTestTile()

	tile.Traces[",device=alpha,name=test_one,source_type=gm,"] = tiling.NewTrace(
		[]types.Digest{
			bug_revert.AlfaPositiveDigest, bug_revert.AlfaPositiveDigest, bug_revert.BravoUntriagedDigest,
			bug_revert.AlfaPositiveDigest, bug_revert.AlfaPositiveDigest,
		}, map[string]string{
			"device":              bug_revert.AlphaDevice,
			types.PrimaryKeyField: string(bug_revert.TestOne),
			types.CorpusField:     "gm",
		})

	blamer, err := New(tile, bug_revert.MakeTestExpectations())
	require.NoError(t, err)

	require.Equal(t, bug_revert.MakeTestCommits(), blamer.commits)
	require.Equal(t, map[types.TestName]map[types.Digest]blameCounts{
		bug_revert.TestOne: {
			bug_revert.BravoUntriagedDigest: {
				3, 0, 0, 0, // TODO(kjlubick): I would have expected this to be 3, 1, 0, 0
			},
		},
		bug_revert.TestTwo: {
			bug_revert.DeltaUntriagedDigest: {
				2, 0, 0, 0,
			},
			bug_revert.FoxtrotUntriagedDigest: {
				1, 2, 0, 0,
			},
		},
	}, blamer.blameLists)
}
