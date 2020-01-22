package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/clstore"
	mock_clstore "go.skia.org/infra/golden/go/clstore/mocks"
	"go.skia.org/infra/golden/go/code_review"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	mock_ignore "go.skia.org/infra/golden/go/ignore/mocks"
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

func TestStubbedNow(t *testing.T) {
	unittest.SmallTest(t)
	fakeNow := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)
	wh := Handlers{}
	assert.NotEqual(t, fakeNow, wh.now())

	wh.testingNow = fakeNow
	// Now, it's always the same
	assert.Equal(t, fakeNow, wh.now())
	assert.Equal(t, fakeNow, wh.now())
	assert.Equal(t, fakeNow, wh.now())
}

func TestStubbedAuthAs(t *testing.T) {
	unittest.SmallTest(t)
	r := httptest.NewRequest(http.MethodGet, "/does/not/matter", nil)
	wh := Handlers{}
	assert.Equal(t, "", wh.loggedInAs(r))

	const fakeUser = "user@example.com"
	wh.testingAuthAs = fakeUser
	assert.Equal(t, fakeUser, wh.loggedInAs(r))
}

// TestByQuerySunnyDay is a unit test of the /byquery endpoint.
// It uses some example data based on the bug_revert corpus, which
// has some untriaged images that are easy to identify blames for.
func TestByQuerySunnyDay(t *testing.T) {
	unittest.SmallTest(t)

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

	output, err := wh.computeByBlame(context.Background(), "gm")
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

	output, err := wh.computeByBlame(context.Background(), "gm")
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
	exp.On("Get", testutils.AnyContext).Return(bug_revert.MakeTestExpectations(), nil).Maybe()

	b, err := blame.New(cpxTile.GetTile(types.ExcludeIgnoredTraces), bug_revert.MakeTestExpectations())
	if err != nil {
		panic(err) // this means our static data is horribly broken
	}

	si, err := indexer.SearchIndexForTesting(cpxTile, [2]digest_counter.DigestCounter{dc, dc}, [2]paramsets.ParamSummary{ps, ps}, exp, b)
	if err != nil {
		panic(err) // this means our static data is horribly broken
	}
	return si
}

// makeBugRevertIndex returns a search index corresponding to the bug_revert_data
// with the given ignores. Like makeBugRevertIndex, we return a real SearchIndex.
// If multiplier is > 1, duplicate traces will be added to the tile to make it artificially
// bigger.
func makeBugRevertIndexWithIgnores(ir []ignore.Rule, multiplier int) *indexer.SearchIndex {
	tile := bug_revert.MakeTestTile()
	add := make([]types.TracePair, 0, multiplier*len(tile.Traces))
	for i := 1; i < multiplier; i++ {
		for id, tr := range tile.Traces {
			newID := tiling.TraceID(fmt.Sprintf("%s,copy=%d", id, i))
			add = append(add, types.TracePair{ID: newID, Trace: tr.(*types.GoldenTrace)})
		}
	}
	for _, tp := range add {
		tile.Traces[tp.ID] = tp.Trace
	}
	cpxTile := types.NewComplexTile(tile)

	subtile, combinedRules, err := ignore.FilterIgnored(tile, ir)
	if err != nil {
		panic(err) // this means our static data is horribly broken
	}
	cpxTile.SetIgnoreRules(subtile, combinedRules)
	dcInclude := digest_counter.New(tile)
	dcExclude := digest_counter.New(subtile)
	psInclude := paramsets.NewParamSummary(tile, dcInclude)
	psExclude := paramsets.NewParamSummary(subtile, dcExclude)
	exp := &mocks.ExpectationsStore{}
	exp.On("Get", testutils.AnyContext).Return(bug_revert.MakeTestExpectations(), nil).Maybe()

	b, err := blame.New(cpxTile.GetTile(types.ExcludeIgnoredTraces), bug_revert.MakeTestExpectations())
	if err != nil {
		panic(err) // this means our static data is horribly broken
	}

	si, err := indexer.SearchIndexForTesting(cpxTile,
		[2]digest_counter.DigestCounter{dcExclude, dcInclude},
		[2]paramsets.ParamSummary{psExclude, psInclude}, exp, b)
	if err != nil {
		panic(err) // this means our static data is horribly broken
	}
	return si
}

// TestGetChangeListsSunnyDay tests the core functionality of
// listing all ChangeLists that have Gold results.
func TestGetChangeListsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mcls := &mock_clstore.Store{}
	defer mcls.AssertExpectations(t)

	mcls.On("GetChangeLists", testutils.AnyContext, clstore.SearchOptions{
		StartIdx: 0,
		Limit:    50,
	}).Return(makeCodeReviewCLs(), 3, nil)
	mcls.On("System").Return("gerrit")

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			CodeReviewURLTemplate: "example.com/cl/%s#templates",
			ChangeListStore:       mcls,
		},
	}

	cls, pagination, err := wh.getIngestedChangeLists(context.Background(), 0, 50, false)
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

func TestGetActiveChangeListsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mcls := &mock_clstore.Store{}
	defer mcls.AssertExpectations(t)

	mcls.On("GetChangeLists", testutils.AnyContext, clstore.SearchOptions{
		StartIdx:    20,
		Limit:       30,
		OpenCLsOnly: true,
	}).Return(makeCodeReviewCLs(), 3, nil)
	mcls.On("System").Return("gerrit")

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			CodeReviewURLTemplate: "example.com/cl/%s#templates",
			ChangeListStore:       mcls,
		},
	}

	cls, pagination, err := wh.getIngestedChangeLists(context.Background(), 20, 30, true)
	assert.NoError(t, err)
	assert.Len(t, cls, 3)
	assert.NotNil(t, pagination)

	assert.Equal(t, &httputils.ResponsePagination{
		Offset: 20,
		Size:   30,
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
			URL:      "example.com/cl/1002#templates",
		},
		{
			System:   "gerrit",
			SystemID: "1001",
			Owner:    "test@example.com",
			Status:   "Landed",
			Subject:  "land gold",
			Updated:  time.Date(2019, time.August, 26, 0, 0, 0, 0, time.UTC),
			URL:      "example.com/cl/1001#templates",
		},
		{
			System:   "gerrit",
			SystemID: "1000",
			Owner:    "test@example.com",
			Status:   "Abandoned",
			Subject:  "gold experiment",
			Updated:  time.Date(2019, time.August, 25, 0, 0, 0, 0, time.UTC),
			URL:      "example.com/cl/1000#templates",
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
			ContinuousIntegrationURLTemplate: "example.com/tj/%s#wow",
			CodeReviewURLTemplate:            "example.com/cl/%s#templates",
			ChangeListStore:                  mcls,
			TryJobStore:                      mtjs,
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
						URL:         "example.com/tj/bb1#wow",
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
						URL:         "example.com/tj/bb2#wow",
					},
					{
						System:      "buildbucket",
						SystemID:    "bb3",
						DisplayName: "Test-Code",
						Updated:     time.Date(2019, time.August, 27, 0, 20, 0, 0, time.UTC),
						URL:         "example.com/tj/bb3#wow",
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

// TestDigestListHandlerSunnyDay tests the usual case of fetching digests for a given test.
func TestDigestListHandlerSunnyDay(t *testing.T) {
	unittest.SmallTest(t)
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

	dlr := wh.getDigestsResponse(string(bug_revert.TestOne), "todo")

	assert.Equal(t, frontend.DigestListResponse{
		Digests: []types.Digest{bug_revert.GoodDigestAlfa, bug_revert.UntriagedDigestBravo},
	}, dlr)
}

func TestListIgnoresNoCounts(t *testing.T) {
	unittest.SmallTest(t)

	mis := &mock_ignore.Store{}
	defer mis.AssertExpectations(t)

	mis.On("List", testutils.AnyContext).Return(makeIgnoreRules(), nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			IgnoreStore: mis,
		},
	}

	xir, err := wh.getIgnores(context.Background(), false)
	require.NoError(t, err)
	clearParsedQueries(xir)
	assert.Equal(t, []*frontend.IgnoreRule{
		{
			ID:        "1234",
			CreatedBy: "user@example.com",
			UpdatedBy: "user2@example.com",
			Expires:   firstRuleExpire,
			Query:     "device=delta",
			Note:      "Flaky driver",
		},
		{
			ID:        "5678",
			CreatedBy: "user2@example.com",
			UpdatedBy: "user@example.com",
			Expires:   secondRuleExpire,
			Query:     "name=test_two&source_type=gm",
			Note:      "Not ready yet",
		},
		{
			ID:        "-1",
			CreatedBy: "user3@example.com",
			UpdatedBy: "user3@example.com",
			Expires:   thirdRuleExpire,
			Query:     "matches=nothing",
			Note:      "Oops, this matches nothing",
		},
	}, xir)
}

func TestListIgnoresCountsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mocks.ExpectationsStore{}
	mi := &mock_indexer.IndexSource{}
	mis := &mock_ignore.Store{}
	defer mes.AssertExpectations(t)
	defer mi.AssertExpectations(t)
	defer mis.AssertExpectations(t)

	exp := bug_revert.MakeTestExpectations()
	// This makes the data a bit more interesting
	exp.Set(bug_revert.TestTwo, bug_revert.GoodDigestEcho, expectations.Untriaged)
	mes.On("Get", testutils.AnyContext).Return(exp, nil)

	fis := makeBugRevertIndexWithIgnores(makeIgnoreRules(), 1)
	mi.On("GetIndex").Return(fis)

	mis.On("List", testutils.AnyContext).Return(makeIgnoreRules(), nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			ExpectationsStore: mes,
			IgnoreStore:       mis,
			Indexer:           mi,
		},
	}

	xir, err := wh.getIgnores(context.Background(), true)
	require.NoError(t, err)
	clearParsedQueries(xir)
	assert.Equal(t, []*frontend.IgnoreRule{
		{
			ID:                      "1234",
			CreatedBy:               "user@example.com",
			UpdatedBy:               "user2@example.com",
			Expires:                 firstRuleExpire,
			Query:                   "device=delta",
			Note:                    "Flaky driver",
			Count:                   2,
			ExclusiveCount:          1,
			UntriagedCount:          1,
			ExclusiveUntriagedCount: 0,
		},
		{
			ID:                      "5678",
			CreatedBy:               "user2@example.com",
			UpdatedBy:               "user@example.com",
			Expires:                 secondRuleExpire,
			Query:                   "name=test_two&source_type=gm",
			Note:                    "Not ready yet",
			Count:                   4,
			ExclusiveCount:          3,
			UntriagedCount:          2,
			ExclusiveUntriagedCount: 1,
		},
		{
			ID:                      "-1",
			CreatedBy:               "user3@example.com",
			UpdatedBy:               "user3@example.com",
			Expires:                 thirdRuleExpire,
			Query:                   "matches=nothing",
			Note:                    "Oops, this matches nothing",
			Count:                   0,
			ExclusiveCount:          0,
			UntriagedCount:          0,
			ExclusiveUntriagedCount: 0,
		},
	}, xir)
}

// TestListIgnoresCountsBigTile uses an artificially bigger tile to process to make sure
// the counting code has no races in it when sharded up.
func TestListIgnoresCountsBigTile(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mocks.ExpectationsStore{}
	mi := &mock_indexer.IndexSource{}
	mis := &mock_ignore.Store{}
	defer mes.AssertExpectations(t)
	defer mi.AssertExpectations(t)
	defer mis.AssertExpectations(t)

	exp := bug_revert.MakeTestExpectations()
	// This makes the data a bit more interesting
	exp.Set(bug_revert.TestTwo, bug_revert.GoodDigestEcho, expectations.Untriaged)
	mes.On("Get", testutils.AnyContext).Return(exp, nil)

	fis := makeBugRevertIndexWithIgnores(makeIgnoreRules(), 50)
	mi.On("GetIndex").Return(fis)

	mis.On("List", testutils.AnyContext).Return(makeIgnoreRules(), nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			ExpectationsStore: mes,
			IgnoreStore:       mis,
			Indexer:           mi,
		},
	}

	xir, err := wh.getIgnores(context.Background(), true)
	require.NoError(t, err)
	// Just check the length, other unit tests will validate the correctness.
	assert.Len(t, xir, 3)
}

func TestHandlersThatRequireLogin(t *testing.T) {
	unittest.SmallTest(t)

	wh := Handlers{}

	test := func(name string, endpoint http.HandlerFunc) {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, requestURL, strings.NewReader("does not matter"))
			endpoint(w, r)

			resp := w.Result()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
	test("add", wh.IgnoresAddHandler)
	test("update", wh.IgnoresUpdateHandler)
	test("delete", wh.IgnoresDeleteHandler)
	// TODO(kjlubick): check all handlers that need login, not just Ignores*
}

func TestHandlersThatTakeJSON(t *testing.T) {
	unittest.SmallTest(t)

	wh := Handlers{
		testingAuthAs: "test@google.com",
	}

	test := func(name string, endpoint http.HandlerFunc) {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, requestURL, strings.NewReader("invalid JSON"))
			endpoint(w, r)

			resp := w.Result()
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
	test("add", wh.IgnoresAddHandler)
	test("update", wh.IgnoresUpdateHandler)
	// TODO(kjlubick): check all handlers that process JSON
}

func TestAddIgnoreRule_SunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	const user = "test@example.com"
	var fakeNow = time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)
	var oneWeekFromNow = time.Date(2020, time.January, 9, 3, 4, 5, 0, time.UTC)

	mis := &mock_ignore.Store{}
	defer mis.AssertExpectations(t)

	expectedRule := ignore.Rule{
		ID:        "",
		CreatedBy: user,
		UpdatedBy: user,
		Expires:   oneWeekFromNow,
		Query:     "a=b&c=d",
		Note:      "skbug:9744",
	}
	mis.On("Create", testutils.AnyContext, expectedRule).Return(nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			IgnoreStore: mis,
		},
		testingAuthAs: user,
		testingNow:    fakeNow,
	}
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"duration": "1w", "filter": "a=b&c=d", "note": "skbug:9744"}`)
	r := httptest.NewRequest(http.MethodPost, requestURL, body)
	wh.IgnoresAddHandler(w, r)

	assertJSONResponseWas(t, http.StatusOK, `{"added":"true"}`, w)
}

func TestAddIgnoreRule_StoreFailure(t *testing.T) {
	unittest.SmallTest(t)

	mis := &mock_ignore.Store{}
	defer mis.AssertExpectations(t)

	mis.On("Create", testutils.AnyContext, mock.Anything).Return(errors.New("firestore broke"))
	wh := Handlers{
		HandlersConfig: HandlersConfig{
			IgnoreStore: mis,
		},
		testingAuthAs: "test@google.com",
	}
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"duration": "1w", "filter": "a=b&c=d", "note": "skbug:9744"}`)
	r := httptest.NewRequest(http.MethodPost, requestURL, body)
	r = mux.SetURLVars(r, map[string]string{"id": "12345"})
	wh.IgnoresAddHandler(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestGetValidatedIgnoreRule_InvalidInput(t *testing.T) {
	unittest.SmallTest(t)

	test := func(name, errorFragment, jsonInput string) {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, requestURL, strings.NewReader(jsonInput))
			_, _, err := getValidatedIgnoreRule(r)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), errorFragment)
		})
	}

	test("invalid JSON", "request JSON", "This should not be valid JSON")
	// There's an instagram joke here... #nofilter
	test("no filter", "supply a filter", `{"duration": "1w", "filter": "", "note": "skbug:9744"}`)
	test("no duration", "invalid duration", `{"duration": "", "filter": "a=b", "note": "skbug:9744"}`)
	test("invalid duration", "invalid duration", `{"duration": "bad", "filter": "a=b", "note": "skbug:9744"}`)
	test("filter too long", "Filter must be", string(makeJSONWithLongFilter(t)))
	test("note too long", "Note must be", string(makeJSONWithLongNote(t)))
}

func makeJSONWithLongFilter(t *testing.T) []byte {
	superLongFilter := frontend.IgnoreRuleBody{
		Duration: "1w",
		Filter:   strings.Repeat("a=b&", 10000),
		Note:     "really long filter",
	}
	superLongFilterBytes, err := json.Marshal(superLongFilter)
	require.NoError(t, err)
	return superLongFilterBytes
}

func makeJSONWithLongNote(t *testing.T) []byte {
	superLongFilter := frontend.IgnoreRuleBody{
		Duration: "1w",
		Filter:   "a=b",
		Note:     strings.Repeat("really long note ", 1000),
	}
	superLongFilterBytes, err := json.Marshal(superLongFilter)
	require.NoError(t, err)
	return superLongFilterBytes
}

func TestUpdateIgnoreRule_SunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	const id = "12345"
	const user = "test@example.com"
	var fakeNow = time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)
	var oneWeekFromNow = time.Date(2020, time.January, 9, 3, 4, 5, 0, time.UTC)

	mis := &mock_ignore.Store{}
	defer mis.AssertExpectations(t)

	expectedRule := ignore.Rule{
		ID:        id,
		CreatedBy: user,
		UpdatedBy: user,
		Expires:   oneWeekFromNow,
		Query:     "a=b&c=d",
		Note:      "skbug:9744",
	}
	mis.On("Update", testutils.AnyContext, expectedRule).Return(nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			IgnoreStore: mis,
		},
		testingAuthAs: user,
		testingNow:    fakeNow,
	}
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"duration": "1w", "filter": "a=b&c=d", "note": "skbug:9744"}`)
	r := httptest.NewRequest(http.MethodPost, requestURL, body)
	r = setID(r, id)
	wh.IgnoresUpdateHandler(w, r)

	assertJSONResponseWas(t, http.StatusOK, `{"updated":"true"}`, w)
}

func TestUpdateIgnoreRule_NoID(t *testing.T) {
	unittest.SmallTest(t)

	wh := Handlers{
		testingAuthAs: "test@google.com",
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, requestURL, strings.NewReader("doesn't matter"))
	wh.IgnoresUpdateHandler(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdateIgnoreRule_StoreFailure(t *testing.T) {
	unittest.SmallTest(t)
	mis := &mock_ignore.Store{}
	defer mis.AssertExpectations(t)

	mis.On("Update", testutils.AnyContext, mock.Anything).Return(errors.New("firestore broke"))
	wh := Handlers{
		HandlersConfig: HandlersConfig{
			IgnoreStore: mis,
		},
		testingAuthAs: "test@google.com",
	}
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"duration": "1w", "filter": "a=b&c=d", "note": "skbug:9744"}`)
	r := httptest.NewRequest(http.MethodPost, requestURL, body)
	r = mux.SetURLVars(r, map[string]string{"id": "12345"})
	wh.IgnoresUpdateHandler(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestDeleteIgnoreRule_SunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	const id = "12345"

	mis := &mock_ignore.Store{}
	defer mis.AssertExpectations(t)

	mis.On("Delete", testutils.AnyContext, id).Return(nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			IgnoreStore: mis,
		},
		testingAuthAs: "test@example.com",
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, requestURL, nil)
	r = setID(r, id)
	wh.IgnoresDeleteHandler(w, r)

	assertJSONResponseWas(t, http.StatusOK, `{"deleted":"true"}`, w)
}

func TestDeleteIgnoreRule_NoID(t *testing.T) {
	unittest.SmallTest(t)

	wh := Handlers{
		testingAuthAs: "test@google.com",
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, requestURL, strings.NewReader("doesn't matter"))
	wh.IgnoresDeleteHandler(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestDeleteIgnoreRule_StoreError(t *testing.T) {
	unittest.SmallTest(t)

	const id = "12345"

	mis := &mock_ignore.Store{}
	defer mis.AssertExpectations(t)

	mis.On("Delete", testutils.AnyContext, id).Return(errors.New("firestore broke"))

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			IgnoreStore: mis,
		},
		testingAuthAs: "test@example.com",
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, requestURL, nil)
	r = setID(r, id)
	wh.IgnoresDeleteHandler(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// Because we are calling our handlers directly, the target URL doesn't matter. The target URL
// would only matter if we were calling into the router, so it knew which handler to call.
const requestURL = "/does/not/matter"

var (
	// These dates are arbitrary and don't matter. The logic for determining if an alert has
	// "expired" is handled on the frontend.
	firstRuleExpire  = time.Date(2019, time.November, 30, 3, 4, 5, 0, time.UTC)
	secondRuleExpire = time.Date(2020, time.November, 30, 3, 4, 5, 0, time.UTC)
	thirdRuleExpire  = time.Date(2020, time.November, 27, 3, 4, 5, 0, time.UTC)
)

func makeIgnoreRules() []ignore.Rule {
	return []ignore.Rule{
		{
			ID:        "1234",
			CreatedBy: "user@example.com",
			UpdatedBy: "user2@example.com",
			Expires:   firstRuleExpire,
			Query:     "device=delta",
			Note:      "Flaky driver",
		},
		{
			ID:        "5678",
			CreatedBy: "user2@example.com",
			UpdatedBy: "user@example.com",
			Expires:   secondRuleExpire,
			Query:     "name=test_two&source_type=gm",
			Note:      "Not ready yet",
		},
		{
			ID:        "-1",
			CreatedBy: "user3@example.com",
			UpdatedBy: "user3@example.com",
			Expires:   thirdRuleExpire,
			Query:     "matches=nothing",
			Note:      "Oops, this matches nothing",
		},
	}
}

// clearParsedQueries removes the implementation detail parts of the IgnoreRule that don't make
// sense to assert against.
func clearParsedQueries(xir []*frontend.IgnoreRule) {
	for _, ir := range xir {
		ir.ParsedQuery = nil
	}
}

// assertJSONResponseWasOK asserts that the given ResponseRecorder was given the appropriate JSON
// headers and saw a status OK (200) response.
func assertJSONResponseWas(t *testing.T, status int, body string, w *httptest.ResponseRecorder) {
	resp := w.Result()
	assert.Equal(t, status, resp.StatusCode)
	assert.Equal(t, jsonContentType, resp.Header.Get(contentTypeHeader))
	assert.Equal(t, allowAllOrigins, resp.Header.Get(accessControlHeader))
	assert.Equal(t, noSniffContent, resp.Header.Get(contentTypeOptionsHeader))
	respBody, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	// The JSON encoder includes a newline "\n" at the end of the body, which is awkward to include
	// in the literals passed in above, so we add that here
	assert.Equal(t, body+"\n", string(respBody))
}

// setID applies the ID mux.Var to a copy of the given request. In a normal server setting, mux will
// parse the given url with a string that indicates how to extract variables (e.g.
// '/json/ignores/save/{id}' and store those to the request's context. However, since we just call
// the handler directly, we need to set those variables ourselves.
func setID(r *http.Request, id string) *http.Request {
	return mux.SetURLVars(r, map[string]string{"id": id})
}
