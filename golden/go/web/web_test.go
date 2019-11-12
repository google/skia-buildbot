package web

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/blame"
	mock_clstore "go.skia.org/infra/golden/go/clstore/mocks"
	"go.skia.org/infra/golden/go/code_review"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/indexer"
	mock_indexer "go.skia.org/infra/golden/go/indexer/mocks"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/paramsets"
	bug_revert "go.skia.org/infra/golden/go/testutils/data_bug_revert"
	"go.skia.org/infra/golden/go/tjstore"
	mock_tjstore "go.skia.org/infra/golden/go/tjstore/mocks"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
	"go.skia.org/infra/golden/go/web/frontend"
)

// TestByQuerySunnyDay is a unit test of the /byquery endpoint.
// It uses some example data based on the bug_revert corpus, which
// has some untriaged images that are easy to identify blames for.
func TestByQuerySunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	query := url.Values{
		types.CORPUS_FIELD: []string{"gm"},
	}

	mi := &mock_indexer.IndexSource{}
	defer mi.AssertExpectations(t)

	// We stop just before the "revert" in the fake data set, so it appears there are more untriaged
	// digests going on.
	fis := makeBugRevertIndex(3)
	mi.On("GetIndex").Return(fis)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer: mi,
		},
	}

	output, err := wh.computeByBlame(query)
	require.NoError(t, err)

	commits := bug_revert.MakeTestCommits()
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

// TestByQuerySunnyDaySimpler gets the ByBlame for a set of data with fewer untriaged outstanding.
func TestByQuerySunnyDaySimpler(t *testing.T) {
	unittest.SmallTest(t)

	query := url.Values{
		types.CORPUS_FIELD: []string{"gm"},
	}

	mi := &mock_indexer.IndexSource{}
	defer mi.AssertExpectations(t)

	// Go all the way to the end, which has cleared up all untriaged digests except for
	// UntriagedDigestFoxtrot
	fis := makeBugRevertIndex(5)
	mi.On("GetIndex").Return(fis)

	commits := bug_revert.MakeTestCommits()
	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer: mi,
		},
	}

	output, err := wh.computeByBlame(query)
	require.NoError(t, err)

	assert.Equal(t, []ByBlameEntry{
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

// makeBugRevertIndex returns a search index corresponding to a subset of the bug_revert_data
// (which currently has nothing ignored). We choose to use this instead of mocking
// out the SearchIndex, as per the advice in http://go/mocks#prefer-real-objects
// of "prefer to use real objects if possible". We have tests that verify these
// real objects work correctly, so we should feel safe to use them here.
func makeBugRevertIndex(endIndex int) *indexer.SearchIndex {
	tile := bug_revert.MakeTestTile()
	// Trim is [start, end)
	tile, err := tile.Trim(0, endIndex)
	if err != nil {
		panic(err) // this means our static data is horribly broken
	}

	cpxTile := types.NewComplexTile(tile)
	dc := digest_counter.New(tile)
	ps := paramsets.NewParamSummary(tile, dc)
	exp := &mocks.ExpectationsStore{}
	exp.On("Get").Return(bug_revert.MakeTestExpectations(), nil).Maybe()

	b, err := blame.New(cpxTile.GetTile(types.ExcludeIgnoredTraces), bug_revert.MakeTestExpectations())
	if err != nil {
		panic(err) // this means our static data is horribly broken
	}

	si, err := indexer.SearchIndexForTesting(cpxTile, [2]digest_counter.DigestCounter{dc, dc}, [2]paramsets.ParamSummary{ps, ps}, exp, b)
	if err != nil {
		// Something is horribly broken with our test data
		panic(err.Error())
	}
	return si
}

// TestGetChangeListsSunnyDay tests the core functionality of
// listing all ChangeLists that have Gold results.
func TestGetChangeListsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mcls := &mock_clstore.Store{}
	defer mcls.AssertExpectations(t)

	mcls.On("GetChangeLists", testutils.AnyContext, 0, 50).Return(makeCodeReviewCLs(), 3, nil)
	mcls.On("System").Return("gerrit")

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			CodeReviewURLPrefix: "example.com/cl",
			ChangeListStore:     mcls,
		},
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

func makeWebCLs() []frontend.ChangeList {
	return []frontend.ChangeList{
		{
			System:   "gerrit",
			SystemID: "1002",
			Owner:    "other@example.com",
			Status:   "Open",
			Subject:  "new feature",
			Updated:  time.Date(2019, time.August, 27, 0, 0, 0, 0, time.UTC),
			URL:      "example.com/cl/1002",
		},
		{
			System:   "gerrit",
			SystemID: "1001",
			Owner:    "test@example.com",
			Status:   "Landed",
			Subject:  "land gold",
			Updated:  time.Date(2019, time.August, 26, 0, 0, 0, 0, time.UTC),
			URL:      "example.com/cl/1001",
		},
		{
			System:   "gerrit",
			SystemID: "1000",
			Owner:    "test@example.com",
			Status:   "Abandoned",
			Subject:  "gold experiment",
			Updated:  time.Date(2019, time.August, 25, 0, 0, 0, 0, time.UTC),
			URL:      "example.com/cl/1000",
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

	mcls.On("GetChangeList", testutils.AnyContext, expectedCLID).Return(makeCodeReviewCLs()[0], nil)
	mcls.On("GetPatchSets", testutils.AnyContext, expectedCLID).Return(makeCodeReviewPSs(), nil)
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
	mtjs.On("GetTryJobs", testutils.AnyContext, psID).Return(tj1, nil)

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
	mtjs.On("GetTryJobs", testutils.AnyContext, psID).Return(tj2, nil)
	mtjs.On("System").Return("buildbucket")

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			ContinuousIntegrationURLPrefix: "example.com/tj",
			CodeReviewURLPrefix:            "example.com/cl",
			ChangeListStore:                mcls,
			TryJobStore:                    mtjs,
		},
	}

	cl, err := wh.getCLSummary(context.Background(), expectedCLID)
	assert.NoError(t, err)
	assert.Equal(t, frontend.ChangeListSummary{
		CL:                makeWebCLs()[0], // matches expectedCLID
		NumTotalPatchSets: 4,
		PatchSets: []frontend.PatchSet{
			{
				SystemID: "ps-1",
				Order:    1,
				TryJobs: []frontend.TryJob{
					{
						System:      "buildbucket",
						SystemID:    "bb1",
						DisplayName: "Test-Build",
						Updated:     time.Date(2019, time.August, 27, 1, 0, 0, 0, time.UTC),
						URL:         "example.com/tj/bb1",
					},
				},
			},
			{
				SystemID: "ps-4",
				Order:    4,
				TryJobs: []frontend.TryJob{
					{
						System:      "buildbucket",
						SystemID:    "bb2",
						DisplayName: "Test-Build",
						Updated:     time.Date(2019, time.August, 27, 0, 15, 0, 0, time.UTC),
						URL:         "example.com/tj/bb2",
					},
					{
						System:      "buildbucket",
						SystemID:    "bb3",
						DisplayName: "Test-Code",
						Updated:     time.Date(2019, time.August, 27, 0, 20, 0, 0, time.UTC),
						URL:         "example.com/tj/bb3",
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

// TestTriageMaster tests a common case of a developer triaging a single test on the
// master branch.
func TestTriageMaster(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mocks.ExpectationsStore{}
	defer mes.AssertExpectations(t)

	user := "user@example.com"

	mes.On("AddChange", testutils.AnyContext, []expstorage.Delta{
		{
			Grouping: bug_revert.TestOne,
			Digest:   bug_revert.UntriagedDigestBravo,
			Label:    expectations.Negative,
		},
	}, user).Return(nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			ExpectationsStore: mes,
		},
	}

	tr := frontend.TriageRequest{
		ChangeListID: "",
		TestDigestStatus: map[types.TestName]map[types.Digest]string{
			bug_revert.TestOne: {
				bug_revert.UntriagedDigestBravo: expectations.Negative.String(),
			},
		},
	}

	err := wh.triage(context.Background(), user, tr)
	assert.NoError(t, err)
}

// TestTriageChangeList tests a common case of a developer triaging a single test on a ChangeList.
func TestTriageChangeList(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mocks.ExpectationsStore{}
	clExp := &mocks.ExpectationsStore{}
	mcs := &mock_clstore.Store{}
	defer mes.AssertExpectations(t)
	defer clExp.AssertExpectations(t)
	defer mcs.AssertExpectations(t)

	clID := "12345"
	crs := "github"
	user := "user@example.com"

	mes.On("ForChangeList", clID, crs).Return(clExp)

	clExp.On("AddChange", testutils.AnyContext, []expstorage.Delta{
		{
			Grouping: bug_revert.TestOne,
			Digest:   bug_revert.UntriagedDigestBravo,
			Label:    expectations.Negative,
		},
	}, user).Return(nil)

	mcs.On("System").Return(crs)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			ExpectationsStore: mes,
			ChangeListStore:   mcs,
		},
	}

	tr := frontend.TriageRequest{
		ChangeListID: clID,
		TestDigestStatus: map[types.TestName]map[types.Digest]string{
			bug_revert.TestOne: {
				bug_revert.UntriagedDigestBravo: expectations.Negative.String(),
			},
		},
	}

	err := wh.triage(context.Background(), user, tr)
	assert.NoError(t, err)
}

// TestBulkTriageMaster tests the case of a developer triaging multiple tests at once
// (via bulk triage).
func TestBulkTriageMaster(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mocks.ExpectationsStore{}
	defer mes.AssertExpectations(t)

	user := "user@example.com"

	matcher := mock.MatchedBy(func(delta []expstorage.Delta) bool {
		assert.Contains(t, delta, expstorage.Delta{
			Grouping: bug_revert.TestOne,
			Digest:   bug_revert.GoodDigestAlfa,
			Label:    expectations.Untriaged,
		})
		assert.Contains(t, delta, expstorage.Delta{
			Grouping: bug_revert.TestOne,
			Digest:   bug_revert.UntriagedDigestBravo,
			Label:    expectations.Negative,
		})
		assert.Contains(t, delta, expstorage.Delta{
			Grouping: bug_revert.TestTwo,
			Digest:   bug_revert.GoodDigestCharlie,
			Label:    expectations.Positive,
		})
		assert.Contains(t, delta, expstorage.Delta{
			Grouping: bug_revert.TestTwo,
			Digest:   bug_revert.UntriagedDigestDelta,
			Label:    expectations.Negative,
		})
		return true
	})

	mes.On("AddChange", testutils.AnyContext, matcher, user).Return(nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			ExpectationsStore: mes,
		},
	}

	tr := frontend.TriageRequest{
		ChangeListID: "",
		TestDigestStatus: map[types.TestName]map[types.Digest]string{
			bug_revert.TestOne: {
				bug_revert.GoodDigestAlfa:       expectations.Untriaged.String(),
				bug_revert.UntriagedDigestBravo: expectations.Negative.String(),
			},
			bug_revert.TestTwo: {
				bug_revert.GoodDigestCharlie:    expectations.Positive.String(),
				bug_revert.UntriagedDigestDelta: expectations.Negative.String(),
			},
		},
	}

	err := wh.triage(context.Background(), user, tr)
	assert.NoError(t, err)
}

// TestTriageMasterLegacy tests a common case of a developer triaging a single test using the
// legacy code (which has "0" as key issue instead of empty string.
func TestTriageMasterLegacy(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mocks.ExpectationsStore{}
	defer mes.AssertExpectations(t)

	user := "user@example.com"

	mes.On("AddChange", testutils.AnyContext, []expstorage.Delta{
		{
			Grouping: bug_revert.TestOne,
			Digest:   bug_revert.UntriagedDigestBravo,
			Label:    expectations.Negative,
		},
	}, user).Return(nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			ExpectationsStore: mes,
		},
	}

	tr := frontend.TriageRequest{
		ChangeListID: "0",
		TestDigestStatus: map[types.TestName]map[types.Digest]string{
			bug_revert.TestOne: {
				bug_revert.UntriagedDigestBravo: expectations.Negative.String(),
			},
		},
	}

	err := wh.triage(context.Background(), user, tr)
	assert.NoError(t, err)
}

// TestNew makes sure that if we omit values from HandlersConfig, New returns an error, depending
// on which validation mode is set.
func TestNewChecksValues(t *testing.T) {
	unittest.SmallTest(t)

	hc := HandlersConfig{}
	_, err := NewHandlers(hc, BaselineSubset)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")

	hc = HandlersConfig{
		GCSClient:       &mocks.GCSClient{},
		Baseliner:       &mocks.BaselineFetcher{},
		ChangeListStore: &mock_clstore.Store{},
	}
	_, err = NewHandlers(hc, BaselineSubset)
	assert.NoError(t, err)
	_, err = NewHandlers(hc, FullFrontEnd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

// TestGetTriageLogSunnyDay tests getting the triage log and converting them to the appropriate
// types.
func TestGetTriageLogSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mocks.ExpectationsStore{}
	defer mes.AssertExpectations(t)

	masterBranch := ""

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			ExpectationsStore: mes,
		},
	}

	ts1 := time.Date(2019, time.October, 5, 4, 3, 2, 0, time.UTC)
	ts2 := time.Date(2019, time.October, 6, 7, 8, 9, 0, time.UTC)

	const offset = 10
	const size = 20

	mes.On("QueryLog", testutils.AnyContext, offset, size, false).Return([]expstorage.TriageLogEntry{
		{
			ID:          "abc",
			ChangeCount: 1,
			User:        "user1@example.com",
			TS:          ts1,
			Details: []expstorage.Delta{
				{
					Label:    expectations.Positive,
					Digest:   bug_revert.UntriagedDigestDelta,
					Grouping: bug_revert.TestOne,
				},
			},
		},
		{
			ID:          "abc",
			ChangeCount: 2,
			User:        "user1@example.com",
			TS:          ts2,
			Details: []expstorage.Delta{
				{
					Label:    expectations.Positive,
					Digest:   bug_revert.UntriagedDigestBravo,
					Grouping: bug_revert.TestOne,
				},
				{
					Label:    expectations.Negative,
					Digest:   bug_revert.GoodDigestCharlie,
					Grouping: bug_revert.TestOne,
				},
			},
		},
	}, offset+2, nil)

	tle, n, err := wh.getTriageLog(context.Background(), masterBranch, offset, size, false)
	assert.NoError(t, err)
	assert.Equal(t, offset+2, n)
	assert.Len(t, tle, 2)

	assert.Equal(t, []frontend.TriageLogEntry{
		{
			ID:          "abc",
			ChangeCount: 1,
			User:        "user1@example.com",
			TS:          ts1.Unix() * 1000,
			Details: []frontend.TriageDelta{
				{
					Label:    expectations.Positive.String(),
					Digest:   bug_revert.UntriagedDigestDelta,
					TestName: bug_revert.TestOne,
				},
			},
		},
		{
			ID:          "abc",
			ChangeCount: 2,
			User:        "user1@example.com",
			TS:          ts2.Unix() * 1000,
			Details: []frontend.TriageDelta{
				{
					Label:    expectations.Positive.String(),
					Digest:   bug_revert.UntriagedDigestBravo,
					TestName: bug_revert.TestOne,
				},
				{
					Label:    expectations.Negative.String(),
					Digest:   bug_revert.GoodDigestCharlie,
					TestName: bug_revert.TestOne,
				},
			},
		},
	}, tle)
}
