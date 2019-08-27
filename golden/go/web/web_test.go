package web

import (
	"net/url"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/indexer/mocks"
	"go.skia.org/infra/golden/go/summary"
	bug_revert "go.skia.org/infra/golden/go/testutils/data_bug_revert"
	"go.skia.org/infra/golden/go/types"
)

// TestByQuerySunnyDay is a unit test of the /byquery endpoint.
// It uses some example data based on the bug_revert corpus, which
// has some untriaged images that are easy to identify blames for.
func TestByQuerySunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	query := url.Values{
		types.CORPUS_FIELD: []string{"dm"},
	}

	mim := &mocks.IndexSource{}
	mis := &mocks.IndexSearcher{}
	defer mim.AssertExpectations(t)
	defer mis.AssertExpectations(t)

	mim.On("GetIndex").Return(mis)

	mis.On("CalcSummaries", types.TestNameSet(nil), query, types.ExcludeIgnoredTraces, true).
		Return(makeBugRevertSummaryMap(), nil)
	cpxTile := types.NewComplexTile(bug_revert.MakeTestTile())
	mis.On("Tile").Return(cpxTile)

	commits := bug_revert.MakeTestCommits()
	mis.On("GetBlame", bug_revert.TestOne, bug_revert.UntriagedDigestBravo, commits).
		Return(makeBugRevertBravoBlame(), nil)
	mis.On("GetBlame", bug_revert.TestTwo, bug_revert.UntriagedDigestDelta, commits).
		Return(makeBugRevertDeltaBlame(), nil)
	mis.On("GetBlame", bug_revert.TestTwo, bug_revert.UntriagedDigestFoxtrot, commits).
		Return(makeBugRevertFoxtrotBlame(), nil)

	wh := WebHandlers{
		Indexer: mim,
	}

	output, err := wh.computeByBlame(query)
	assert.NoError(t, err)

	assert.Equal(t, []ByBlameEntry{
		{
			GroupID:  bug_revert.SecondCommitHash,
			NDigests: 2,
			NTests:   2,
			Commits:  []*tiling.Commit{commits[1]},
			AffectedTests: []TestRollup{
				{
					Test:         bug_revert.TestOne,
					Num:          1,
					SampleDigest: bug_revert.UntriagedDigestBravo,
				},
				{
					Test:         bug_revert.TestTwo,
					Num:          1,
					SampleDigest: bug_revert.UntriagedDigestDelta,
				},
			},
		},
		{
			GroupID:  bug_revert.ThirdCommitHash,
			NDigests: 1,
			NTests:   1,
			Commits:  []*tiling.Commit{commits[2]},
			AffectedTests: []TestRollup{
				{
					Test:         bug_revert.TestTwo,
					Num:          1,
					SampleDigest: bug_revert.UntriagedDigestFoxtrot,
				},
			},
		},
	}, output)
}

// makeBugRevertSummaryMap returns the SummaryMap for the whole tile.
// TODO(kjlubick): This was copied from summary_test. It would be
// nice to have a clean way to share this hard_coded data, but also
// avoid awkward dependency cycles.
// We return the summary for the whole tile, not just HEAD, because it's a bit more interesting
// and can exercise more pathways.
func makeBugRevertSummaryMap() summary.SummaryMap {
	return summary.SummaryMap{
		bug_revert.TestOne: {
			Name:      bug_revert.TestOne,
			Pos:       1,
			Untriaged: 1,
			UntHashes: types.DigestSlice{bug_revert.UntriagedDigestBravo},
			Num:       2,
			Corpus:    "gm",
			Blame: []blame.WeightedBlame{
				{
					Author: bug_revert.BuggyAuthor,
					Prob:   1,
				},
			},
		},
		bug_revert.TestTwo: {
			Name:      bug_revert.TestTwo,
			Pos:       2,
			Untriaged: 2,
			UntHashes: types.DigestSlice{bug_revert.UntriagedDigestDelta, bug_revert.UntriagedDigestFoxtrot},
			Num:       4,
			Corpus:    "gm",
			Blame: []blame.WeightedBlame{
				{
					Author: bug_revert.InnocentAuthor,
					Prob:   0.5,
				},
				{
					Author: bug_revert.BuggyAuthor,
					Prob:   0.5,
				},
			},
		},
	}
}

// The following functions have their data pulled from blame_test
func makeBugRevertBravoBlame() blame.BlameDistribution {
	return blame.BlameDistribution{
		Freq: []int{1},
	}
}

func makeBugRevertDeltaBlame() blame.BlameDistribution {
	return blame.BlameDistribution{
		Freq: []int{1},
	}
}

func makeBugRevertFoxtrotBlame() blame.BlameDistribution {
	return blame.BlameDistribution{
		Freq: []int{2},
	}
}
