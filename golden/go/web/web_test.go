package web

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/blame"
	mock_clstore "go.skia.org/infra/golden/go/clstore/mocks"
	"go.skia.org/infra/golden/go/code_review"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/indexer/mocks"
	"go.skia.org/infra/golden/go/summary"
	bug_revert "go.skia.org/infra/golden/go/testutils/data_bug_revert"
	"go.skia.org/infra/golden/go/tjstore"
	mock_tjstore "go.skia.org/infra/golden/go/tjstore/mocks"
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

var anyctx = mock.AnythingOfType("*context.emptyCtx")

// TestGetChangeListsSunnyDay tests the core functionality of
// listing all ChangeLists that have Gold results.
func TestGetChangeListsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mcls := &mock_clstore.Store{}
	defer mcls.AssertExpectations(t)

	mcls.On("GetChangeLists", anyctx, 0, 50).Return(makeCodeReviewCLs(), 3, nil)
	mcls.On("System").Return("gerrit")

	wh := WebHandlers{
		ChangeListStore: mcls,
	}

	cls, pagination, err := wh.getIngestedChangeLists(context.Background(), 0, 50)
	assert.NoError(t, err)
	assert.Len(t, cls, 3)
	assert.NotNil(t, pagination)

	assert.Equal(t, &httputils.ResponsePagination{
		Offset: 0,
		Size:   50,
		Total:  3,
	}, pagination)

	expected := makeWebCLs()
	assert.Equal(t, expected, cls)
}

func makeCodeReviewCLs() []code_review.ChangeList {
	return []code_review.ChangeList{
		{
			SystemID: "1002",
			Owner:    "other@example.com",
			Status:   code_review.Open,
			Subject:  "new feature",
			Updated:  time.Date(2019, time.August, 27, 0, 0, 0, 0, time.UTC),
		},
		{
			SystemID: "1001",
			Owner:    "test@example.com",
			Status:   code_review.Landed,
			Subject:  "land gold",
			Updated:  time.Date(2019, time.August, 26, 0, 0, 0, 0, time.UTC),
		},
		{
			SystemID: "1000",
			Owner:    "test@example.com",
			Status:   code_review.Abandoned,
			Subject:  "gold experiment",
			Updated:  time.Date(2019, time.August, 25, 0, 0, 0, 0, time.UTC),
		},
	}
}

func makeWebCLs() []changeList {
	return []changeList{
		{
			System:   "gerrit",
			SystemID: "1002",
			Owner:    "other@example.com",
			Status:   "Open",
			Subject:  "new feature",
			Updated:  time.Date(2019, time.August, 27, 0, 0, 0, 0, time.UTC),
		},
		{
			System:   "gerrit",
			SystemID: "1001",
			Owner:    "test@example.com",
			Status:   "Landed",
			Subject:  "land gold",
			Updated:  time.Date(2019, time.August, 26, 0, 0, 0, 0, time.UTC),
		},
		{
			System:   "gerrit",
			SystemID: "1000",
			Owner:    "test@example.com",
			Status:   "Abandoned",
			Subject:  "gold experiment",
			Updated:  time.Date(2019, time.August, 25, 0, 0, 0, 0, time.UTC),
		},
	}
}

// TestGetCLSummarySunnyDay represents a case where we have a CL that
// has 2 patchsets with data, PS with order 1, ps with order 4
func TestGetCLSummarySunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	expectedCLID := "1002"

	mcls := &mock_clstore.Store{}
	mtjs := &mock_tjstore.Store{}
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)

	mcls.On("GetChangeList", anyctx, expectedCLID).Return(makeCodeReviewCLs()[0], nil)
	mcls.On("GetPatchSets", anyctx, expectedCLID).Return(makeCodeReviewPSs(), nil)
	mcls.On("System").Return("gerrit")

	psID := tjstore.CombinedPSID{
		CL:  expectedCLID,
		CRS: "gerrit",
		PS:  "ps-1",
	}
	tj1 := []ci.TryJob{
		{
			SystemID:    "bb1",
			DisplayName: "Test-Build",
			Updated:     time.Date(2019, time.August, 27, 1, 0, 0, 0, time.UTC),
		},
	}
	mtjs.On("GetTryJobs", anyctx, psID).Return(tj1, nil)

	psID = tjstore.CombinedPSID{
		CL:  expectedCLID,
		CRS: "gerrit",
		PS:  "ps-4",
	}
	tj2 := []ci.TryJob{
		{
			SystemID:    "bb2",
			DisplayName: "Test-Build",
			Updated:     time.Date(2019, time.August, 27, 0, 15, 0, 0, time.UTC),
		},
		{
			SystemID:    "bb3",
			DisplayName: "Test-Code",
			Updated:     time.Date(2019, time.August, 27, 0, 20, 0, 0, time.UTC),
		},
	}
	mtjs.On("GetTryJobs", anyctx, psID).Return(tj2, nil)
	mtjs.On("System").Return("buildbucket")

	wh := WebHandlers{
		ChangeListStore: mcls,
		TryJobStore:     mtjs,
	}

	cl, err := wh.getCLSummary(context.Background(), expectedCLID)
	assert.NoError(t, err)
	assert.Equal(t, changeListSummary{
		CL:                makeWebCLs()[0], // matches expectedCLID
		NumTotalPatchSets: 4,
		PatchSets: []patchSet{
			{
				SystemID: "ps-1",
				Order:    1,
				TryJobs: []tryJob{
					{
						System:      "buildbucket",
						SystemID:    "bb1",
						DisplayName: "Test-Build",
						Updated:     time.Date(2019, time.August, 27, 1, 0, 0, 0, time.UTC),
					},
				},
			},
			{
				SystemID: "ps-4",
				Order:    4,
				TryJobs: []tryJob{
					{
						System:      "buildbucket",
						SystemID:    "bb2",
						DisplayName: "Test-Build",
						Updated:     time.Date(2019, time.August, 27, 0, 15, 0, 0, time.UTC),
					},
					{
						System:      "buildbucket",
						SystemID:    "bb3",
						DisplayName: "Test-Code",
						Updated:     time.Date(2019, time.August, 27, 0, 20, 0, 0, time.UTC),
					},
				},
			},
		},
	}, cl)
}

func makeCodeReviewPSs() []code_review.PatchSet {
	// This data is arbitrary
	return []code_review.PatchSet{
		{
			SystemID:     "ps-1",
			ChangeListID: "1002",
			Order:        1,
			GitHash:      "d6ac82ac4ee426b5ce2061f78cc02f9fe1db587e",
		},
		{
			SystemID:     "ps-4",
			ChangeListID: "1002",
			Order:        4,
			GitHash:      "45247158d641ece6318f2598fefecfce86a61ae0",
		},
	}
}
