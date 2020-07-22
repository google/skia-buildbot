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
	"go.skia.org/infra/go/paramtools"
	"golang.org/x/time/rate"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/clstore"
	mock_clstore "go.skia.org/infra/golden/go/clstore/mocks"
	"go.skia.org/infra/golden/go/code_review"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expectations"
	mock_expectations "go.skia.org/infra/golden/go/expectations/mocks"
	"go.skia.org/infra/golden/go/ignore"
	mock_ignore "go.skia.org/infra/golden/go/ignore/mocks"
	"go.skia.org/infra/golden/go/indexer"
	mock_indexer "go.skia.org/infra/golden/go/indexer/mocks"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/paramsets"
	search_fe "go.skia.org/infra/golden/go/search/frontend"
	mock_search "go.skia.org/infra/golden/go/search/mocks"
	bug_revert "go.skia.org/infra/golden/go/testutils/data_bug_revert"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/tjstore"
	mock_tjstore "go.skia.org/infra/golden/go/tjstore/mocks"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/web/frontend"
)

func TestStubbedNow_ReplacesActualNow(t *testing.T) {
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

func TestStubbedAuthAs_OverridesLoginLogicWithHardCodedEmail(t *testing.T) {
	unittest.SmallTest(t)
	r := httptest.NewRequest(http.MethodGet, "/does/not/matter", nil)
	wh := Handlers{}
	assert.Equal(t, "", wh.loggedInAs(r))

	const fakeUser = "user@example.com"
	wh.testingAuthAs = fakeUser
	assert.Equal(t, fakeUser, wh.loggedInAs(r))
}

// TestNewHandlers_BaselineSubset_HasAllPieces_Success makes sure we can create a web.Handlers
// using the BaselineSubset of inputs.
func TestNewHandlers_BaselineSubset_HasAllPieces_Success(t *testing.T) {
	unittest.SmallTest(t)

	hc := HandlersConfig{
		GCSClient:       &mocks.GCSClient{},
		Baseliner:       &mocks.BaselineFetcher{},
		ChangeListStore: &mock_clstore.Store{},
	}
	_, err := NewHandlers(hc, BaselineSubset)
	require.NoError(t, err)
}

// TestNewHandlers_BaselineSubset_MissingPieces_Failure makes sure that if we omit values from
// HandlersConfig, NewHandlers returns an error.
func TestNewHandlers_BaselineSubset_MissingPieces_Failure(t *testing.T) {
	unittest.SmallTest(t)

	hc := HandlersConfig{}
	_, err := NewHandlers(hc, BaselineSubset)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")

	hc = HandlersConfig{
		GCSClient:       &mocks.GCSClient{},
		ChangeListStore: &mock_clstore.Store{},
	}
	_, err = NewHandlers(hc, BaselineSubset)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

// TestNewHandlers_FullFront_EndMissingPieces_Failure makes sure that if we omit values from
// HandlersConfig, NewHandlers returns an error.
// TODO(kjlubick) Add a case for FullFrontEnd with all pieces when we have mocks for all
//   remaining services.
func TestNewHandlers_FullFrontEnd_MissingPieces_Failure(t *testing.T) {
	unittest.SmallTest(t)

	hc := HandlersConfig{}
	_, err := NewHandlers(hc, FullFrontEnd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")

	hc = HandlersConfig{
		GCSClient:       &mocks.GCSClient{},
		Baseliner:       &mocks.BaselineFetcher{},
		ChangeListStore: &mock_clstore.Store{},
	}
	_, err = NewHandlers(hc, FullFrontEnd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

// TestComputeByBlame_OneUntriagedDigest_Success calculates the "byBlameEntries" for the
// entire BugRevert test data corpus, which has one seen untriaged digest. A byBlameEntry ("blame"
// or "blames" for short) points out which commits introduced untriaged digests.
func TestComputeByBlame_OneUntriagedDigest_Success(t *testing.T) {
	unittest.SmallTest(t)

	mi := &mock_indexer.IndexSource{}
	defer mi.AssertExpectations(t)

	commits := bug_revert.MakeTestCommits()
	// Go all the way to the end (bug_revert has 5 commits in it), which has cleared up all
	// untriaged digests except for FoxtrotUntriagedDigest
	fis := makeBugRevertIndex(len(commits))
	mi.On("GetIndex").Return(fis)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer: mi,
		},
	}

	output, err := wh.computeByBlame(context.Background(), "gm")
	require.NoError(t, err)

	assert.Equal(t, []frontend.ByBlameEntry{
		{
			GroupID:  bug_revert.ThirdCommitHash,
			NDigests: 1,
			NTests:   1,
			Commits:  []frontend.Commit{frontend.FromTilingCommit(commits[2])},
			AffectedTests: []frontend.TestRollup{
				{
					Test:         bug_revert.TestTwo,
					Num:          1,
					SampleDigest: bug_revert.FoxtrotUntriagedDigest,
				},
			},
		},
	}, output)
}

// TestComputeByBlame_MultipleUntriagedDigests_Success calculates the "byBlameEntries" for a
// truncated version of the bug_revert test data corpus.  This subset was chosen to have several
// untriaged digests that are easy to manually compute blames for to verify.
func TestComputeByBlame_MultipleUntriagedDigests_Success(t *testing.T) {
	unittest.SmallTest(t)

	mi := &mock_indexer.IndexSource{}
	defer mi.AssertExpectations(t)

	// We stop just before the "revert" in the fake data set, so it appears there are more untriaged
	// digests going on.
	fis := makeBugRevertIndex(bug_revert.RevertBugCommitIndex - 1)
	mi.On("GetIndex").Return(fis)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer: mi,
		},
	}

	output, err := wh.computeByBlame(context.Background(), "gm")
	require.NoError(t, err)

	commits := bug_revert.MakeTestCommits()
	assert.Equal(t, []frontend.ByBlameEntry{
		{
			GroupID:  bug_revert.SecondCommitHash,
			NDigests: 2,
			NTests:   2,
			Commits:  []frontend.Commit{frontend.FromTilingCommit(commits[1])},
			AffectedTests: []frontend.TestRollup{
				{
					Test:         bug_revert.TestOne,
					Num:          1,
					SampleDigest: bug_revert.BravoUntriagedDigest,
				},
				{
					Test:         bug_revert.TestTwo,
					Num:          1,
					SampleDigest: bug_revert.DeltaUntriagedDigest,
				},
			},
		},
		{
			GroupID:  bug_revert.ThirdCommitHash,
			NDigests: 1,
			NTests:   1,
			Commits:  []frontend.Commit{frontend.FromTilingCommit(commits[2])},
			AffectedTests: []frontend.TestRollup{
				{
					Test:         bug_revert.TestTwo,
					Num:          1,
					SampleDigest: bug_revert.FoxtrotUntriagedDigest,
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

	// Trim down the traces to end sooner (to make the data "more interesting")
	for _, trace := range tile.Traces {
		trace.Digests = trace.Digests[:endIndex]
	}
	tile.Commits = tile.Commits[:endIndex]

	cpxTile := tiling.NewComplexTile(tile)
	dc := digest_counter.New(tile)
	ps := paramsets.NewParamSummary(tile, dc)
	exp := &mock_expectations.Store{}
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
	add := make([]tiling.TracePair, 0, multiplier*len(tile.Traces))
	for i := 1; i < multiplier; i++ {
		for id, tr := range tile.Traces {
			newID := tiling.TraceID(fmt.Sprintf("%s,copy=%d", id, i))
			add = append(add, tiling.TracePair{ID: newID, Trace: tr})
		}
	}
	for _, tp := range add {
		tile.Traces[tp.ID] = tp.Trace
	}
	cpxTile := tiling.NewComplexTile(tile)

	subtile, combinedRules, err := ignore.FilterIgnored(tile, ir)
	if err != nil {
		panic(err) // this means our static data is horribly broken
	}
	cpxTile.SetIgnoreRules(subtile, combinedRules)
	dcInclude := digest_counter.New(tile)
	dcExclude := digest_counter.New(subtile)
	psInclude := paramsets.NewParamSummary(tile, dcInclude)
	psExclude := paramsets.NewParamSummary(subtile, dcExclude)
	exp := &mock_expectations.Store{}
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

// TestGetIngestedChangeLists_AllChangeLists_SunnyDay_Success tests the core functionality of
// listing all ChangeLists that have Gold results.
func TestGetIngestedChangeLists_AllChangeLists_SunnyDay_Success(t *testing.T) {
	unittest.SmallTest(t)

	mcls := &mock_clstore.Store{}
	defer mcls.AssertExpectations(t)

	mcls.On("GetChangeLists", testutils.AnyContext, clstore.SearchOptions{
		StartIdx: 0,
		Limit:    50,
	}).Return(makeCodeReviewCLs(), len(makeCodeReviewCLs()), nil)
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

// TestGetIngestedChangeLists_ActiveChangeLists_SunnyDay_Success makes sure that we properly get
// only active ChangeLists, that is, ChangeLists which are open.
func TestGetIngestedChangeLists_ActiveChangeLists_SunnyDay_Success(t *testing.T) {
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

// TestGetClSummary_SunnyDay_Success represents a case where we have a CL that has 2 patchsets with
// data, PS with order 1, ps with order 4.
func TestGetClSummary_SunnyDay_Success(t *testing.T) {
	unittest.SmallTest(t)

	const expectedCLID = "1002"

	mcls := &mock_clstore.Store{}
	mtjs := &mock_tjstore.Store{}

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
			System:      "buildbucket",
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
			SystemID:    "cirrus-7",
			System:      "cirrus",
			DisplayName: "Test-Build",
			Updated:     time.Date(2019, time.August, 27, 0, 15, 0, 0, time.UTC),
		},
		{
			SystemID:    "bb3",
			System:      "buildbucket",
			DisplayName: "Test-Code",
			Updated:     time.Date(2019, time.August, 27, 0, 20, 0, 0, time.UTC),
		},
	}
	mtjs.On("GetTryJobs", testutils.AnyContext, psID).Return(tj2, nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			CodeReviewURLTemplate: "example.com/cl/%s#templates",
			ChangeListStore:       mcls,
			TryJobStore:           mtjs,
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
						URL:         "https://cr-buildbucket.appspot.com/build/bb1",
					},
				},
			},
			{
				SystemID: "ps-4",
				Order:    4,
				TryJobs: []frontend.TryJob{
					{
						System:      "cirrus",
						SystemID:    "cirrus-7",
						DisplayName: "Test-Build",
						Updated:     time.Date(2019, time.August, 27, 0, 15, 0, 0, time.UTC),
						URL:         "https://cirrus-ci.com/task/cirrus-7",
					},
					{
						System:      "buildbucket",
						SystemID:    "bb3",
						DisplayName: "Test-Code",
						Updated:     time.Date(2019, time.August, 27, 0, 20, 0, 0, time.UTC),
						URL:         "https://cr-buildbucket.appspot.com/build/bb3",
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

// TestTriage_SingleDigestOnMaster_Success tests a common case of a developer triaging a single
// test on the master branch.
func TestTriage_SingleDigestOnMaster_Success(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mock_expectations.Store{}
	defer mes.AssertExpectations(t)

	const user = "user@example.com"

	mes.On("AddChange", testutils.AnyContext, []expectations.Delta{
		{
			Grouping: bug_revert.TestOne,
			Digest:   bug_revert.BravoUntriagedDigest,
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
		TestDigestStatus: map[types.TestName]map[types.Digest]expectations.Label{
			bug_revert.TestOne: {
				bug_revert.BravoUntriagedDigest: expectations.Negative,
			},
		},
	}

	err := wh.triage(context.Background(), user, tr)
	assert.NoError(t, err)
}

func TestTriage_SingleDigestOnMaster_ImageMatchingAlgorithmSet_UsesAlgorithmNameAsAuthor(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mock_expectations.Store{}
	defer mes.AssertExpectations(t)

	const user = "user@example.com"
	const algorithmName = "fuzzy"

	mes.On("AddChange", testutils.AnyContext, []expectations.Delta{
		{
			Grouping: bug_revert.TestOne,
			Digest:   bug_revert.BravoUntriagedDigest,
			Label:    expectations.Negative,
		},
	}, algorithmName).Return(nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			ExpectationsStore: mes,
		},
	}

	tr := frontend.TriageRequest{
		ChangeListID: "",
		TestDigestStatus: map[types.TestName]map[types.Digest]expectations.Label{
			bug_revert.TestOne: {
				bug_revert.BravoUntriagedDigest: expectations.Negative,
			},
		},
		ImageMatchingAlgorithm: algorithmName,
	}

	err := wh.triage(context.Background(), user, tr)
	assert.NoError(t, err)
}

// TestTriage_SingleDigestOnCL_Success tests a common case of a developer triaging a single test on
// a ChangeList.
func TestTriage_SingleDigestOnCL_Success(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mock_expectations.Store{}
	clExp := &mock_expectations.Store{}
	mcs := &mock_clstore.Store{}
	defer mes.AssertExpectations(t)
	defer clExp.AssertExpectations(t)
	defer mcs.AssertExpectations(t)

	const clID = "12345"
	const crs = "github"
	const user = "user@example.com"

	mes.On("ForChangeList", clID, crs).Return(clExp)

	clExp.On("AddChange", testutils.AnyContext, []expectations.Delta{
		{
			Grouping: bug_revert.TestOne,
			Digest:   bug_revert.BravoUntriagedDigest,
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
		TestDigestStatus: map[types.TestName]map[types.Digest]expectations.Label{
			bug_revert.TestOne: {
				bug_revert.BravoUntriagedDigest: expectations.Negative,
			},
		},
	}

	err := wh.triage(context.Background(), user, tr)
	assert.NoError(t, err)
}

func TestTriage_SingleDigestOnCL_ImageMatchingAlgorithmSet_UsesAlgorithmNameAsAuthor(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mock_expectations.Store{}
	clExp := &mock_expectations.Store{}
	mcs := &mock_clstore.Store{}
	defer mes.AssertExpectations(t)
	defer clExp.AssertExpectations(t)
	defer mcs.AssertExpectations(t)

	const clID = "12345"
	const crs = "github"
	const user = "user@example.com"
	const algorithmName = "fuzzy"

	mes.On("ForChangeList", clID, crs).Return(clExp)

	clExp.On("AddChange", testutils.AnyContext, []expectations.Delta{
		{
			Grouping: bug_revert.TestOne,
			Digest:   bug_revert.BravoUntriagedDigest,
			Label:    expectations.Negative,
		},
	}, algorithmName).Return(nil)

	mcs.On("System").Return(crs)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			ExpectationsStore: mes,
			ChangeListStore:   mcs,
		},
	}

	tr := frontend.TriageRequest{
		ChangeListID: clID,
		TestDigestStatus: map[types.TestName]map[types.Digest]expectations.Label{
			bug_revert.TestOne: {
				bug_revert.BravoUntriagedDigest: expectations.Negative,
			},
		},
		ImageMatchingAlgorithm: algorithmName,
	}

	err := wh.triage(context.Background(), user, tr)
	assert.NoError(t, err)
}

// TestTriage_BulkTriageOnMaster_SunnyDay_Success tests the case of a developer triaging multiple
// tests at once (via bulk triage).
func TestTriage_BulkTriageOnMaster_SunnyDay_Success(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mock_expectations.Store{}
	defer mes.AssertExpectations(t)

	const user = "user@example.com"

	matcher := mock.MatchedBy(func(delta []expectations.Delta) bool {
		assert.Contains(t, delta, expectations.Delta{
			Grouping: bug_revert.TestOne,
			Digest:   bug_revert.AlfaPositiveDigest,
			Label:    expectations.Untriaged,
		})
		assert.Contains(t, delta, expectations.Delta{
			Grouping: bug_revert.TestOne,
			Digest:   bug_revert.BravoUntriagedDigest,
			Label:    expectations.Negative,
		})
		assert.Contains(t, delta, expectations.Delta{
			Grouping: bug_revert.TestTwo,
			Digest:   bug_revert.CharliePositiveDigest,
			Label:    expectations.Positive,
		})
		assert.Contains(t, delta, expectations.Delta{
			Grouping: bug_revert.TestTwo,
			Digest:   bug_revert.DeltaUntriagedDigest,
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
		TestDigestStatus: map[types.TestName]map[types.Digest]expectations.Label{
			bug_revert.TestOne: {
				bug_revert.AlfaPositiveDigest:   expectations.Untriaged,
				bug_revert.BravoUntriagedDigest: expectations.Negative,
			},
			bug_revert.TestTwo: {
				bug_revert.CharliePositiveDigest: expectations.Positive,
				bug_revert.DeltaUntriagedDigest:  expectations.Negative,
				"digestWithNoClosestPositive":    "",
			},
		},
	}

	err := wh.triage(context.Background(), user, tr)
	assert.NoError(t, err)
}

// TestTriage_SingleLegacyDigestOnMaster_SunnyDay_Success tests a common case of a developer
// triaging a single test using the legacy code (which has "0" as key issue instead of empty string.
func TestTriage_SingleLegacyDigestOnMaster_SunnyDay_Success(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mock_expectations.Store{}
	defer mes.AssertExpectations(t)

	const user = "user@example.com"

	mes.On("AddChange", testutils.AnyContext, []expectations.Delta{
		{
			Grouping: bug_revert.TestOne,
			Digest:   bug_revert.BravoUntriagedDigest,
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
		TestDigestStatus: map[types.TestName]map[types.Digest]expectations.Label{
			bug_revert.TestOne: {
				bug_revert.BravoUntriagedDigest: expectations.Negative,
			},
		},
	}

	err := wh.triage(context.Background(), user, tr)
	assert.NoError(t, err)
}

// TestGetTriageLog_MasterBranchNoDetails_SunnyDay_Success tests getting the triage log and
// converting them to the appropriate types.
func TestGetTriageLog_MasterBranchNoDetails_SunnyDay_Success(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mock_expectations.Store{}
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

	mes.On("QueryLog", testutils.AnyContext, offset, size, false).Return([]expectations.TriageLogEntry{
		{
			ID:          "abc",
			ChangeCount: 1,
			User:        "user1@example.com",
			TS:          ts1,
			Details: []expectations.Delta{
				{
					Label:    expectations.Positive,
					Digest:   bug_revert.DeltaUntriagedDigest,
					Grouping: bug_revert.TestOne,
				},
			},
		},
		{
			ID:          "abc",
			ChangeCount: 2,
			User:        "user1@example.com",
			TS:          ts2,
			Details: []expectations.Delta{
				{
					Label:    expectations.Positive,
					Digest:   bug_revert.BravoUntriagedDigest,
					Grouping: bug_revert.TestOne,
				},
				{
					Label:    expectations.Negative,
					Digest:   bug_revert.CharliePositiveDigest,
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
					Label:    expectations.Positive,
					Digest:   bug_revert.DeltaUntriagedDigest,
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
					Label:    expectations.Positive,
					Digest:   bug_revert.BravoUntriagedDigest,
					TestName: bug_revert.TestOne,
				},
				{
					Label:    expectations.Negative,
					Digest:   bug_revert.CharliePositiveDigest,
					TestName: bug_revert.TestOne,
				},
			},
		},
	}, tle)
}

// TestGetDigestsResponse_SunnyDay_Success tests the usual case of fetching digests for a given
// test in a given corpus.
func TestGetDigestsResponse_SunnyDay_Success(t *testing.T) {
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
		Digests: []types.Digest{bug_revert.AlfaPositiveDigest, bug_revert.BravoUntriagedDigest},
	}, dlr)
}

// TestGetIgnores_NoCounts_SunnyDay_Success tests the case where we simply return the list of the
// current ignore rules, without counting any of the traces to which they apply.
func TestGetIgnores_NoCounts_SunnyDay_Success(t *testing.T) {
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

// TestGetIgnores_WithCounts_SunnyDay_Success tests the case where we get the list of current ignore
// rules and count the traces to which those rules apply.
func TestGetIgnores_WithCounts_SunnyDay_Success(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mock_expectations.Store{}
	mi := &mock_indexer.IndexSource{}
	mis := &mock_ignore.Store{}
	defer mes.AssertExpectations(t)
	defer mi.AssertExpectations(t)
	defer mis.AssertExpectations(t)

	exp := bug_revert.MakeTestExpectations()
	// Pretending EchoPositiveDigest is untriaged makes the data a bit more interesting, in the sense
	// that we can observe differences between Count/ExclusiveCount and
	// UntriagedCount/ExclusiveUntriagedCount.
	exp.Set(bug_revert.TestTwo, bug_revert.EchoPositiveDigest, expectations.Untriaged)
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

	xir, err := wh.getIgnores(context.Background(), true /* = withCounts*/)
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

// TestGetIgnores_WithCountsOnBigTile_SunnyDay_NoRaceConditions uses an artificially bigger tile to
// process to make sure the counting code has no races in it when sharded.
func TestGetIgnores_WithCountsOnBigTile_SunnyDay_NoRaceConditions(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mock_expectations.Store{}
	mi := &mock_indexer.IndexSource{}
	mis := &mock_ignore.Store{}
	defer mes.AssertExpectations(t)
	defer mi.AssertExpectations(t)
	defer mis.AssertExpectations(t)

	exp := bug_revert.MakeTestExpectations()
	// This makes the data a bit more interesting
	exp.Set(bug_revert.TestTwo, bug_revert.EchoPositiveDigest, expectations.Untriaged)
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

	xir, err := wh.getIgnores(context.Background(), true /* = withCounts*/)
	require.NoError(t, err)
	// Just check the length, other unit tests will validate the correctness.
	assert.Len(t, xir, 3)
}

// TestHandlersThatRequireLogin_NotLoggedIn_UnauthorizedError tests a list of handlers to make sure
// they return an Unauthorized status if attempted to be used without being logged in.
func TestHandlersThatRequireLogin_NotLoggedIn_UnauthorizedError(t *testing.T) {
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
	test("add", wh.AddIgnoreRule)
	test("update", wh.UpdateIgnoreRule)
	test("delete", wh.DeleteIgnoreRule)
	// TODO(kjlubick): check all handlers that need login, not just Ignores*
}

// TestHandlersWhichTakeJSON_BadInput_BadRequestError tests a list of handlers which take JSON as an
// input and make sure they all return a BadRequest response when given bad input.
func TestHandlersWhichTakeJSON_BadInput_BadRequestError(t *testing.T) {
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
	test("add", wh.AddIgnoreRule)
	test("update", wh.UpdateIgnoreRule)
	// TODO(kjlubick): check all handlers that process JSON
}

// TestAddIgnoreRule_SunnyDay_Success tests a typical case of adding an ignore rule (which ends
// up in the IgnoreStore).
func TestAddIgnoreRule_SunnyDay_Success(t *testing.T) {
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
	wh.AddIgnoreRule(w, r)

	assertJSONResponseWas(t, http.StatusOK, `{"added":"true"}`, w)
}

// TestAddIgnoreRule_StoreFailure_InternalServerError tests the exceptional case where a rule
// fails to be added to the IgnoreStore).
func TestAddIgnoreRule_StoreFailure_InternalServerError(t *testing.T) {
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
	wh.AddIgnoreRule(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// TestGetValidatedIgnoreRule_InvalidInput_Error tests several exceptional cases where an invalid
// rule is given to the handler.
func TestGetValidatedIgnoreRule_InvalidInput_Error(t *testing.T) {
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

// makeJSONWithLongFilter returns a []byte that is the encoded JSON of an otherwise valid
// IgnoreRuleBody, except it has a Filter which exceeds 10 KB.
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

// makeJSONWithLongNote returns a []byte that is the encoded JSON of an otherwise valid
// IgnoreRuleBody, except it has a Note which exceeds 1 KB.
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

// TestUpdateIgnoreRule_SunnyDay_Success tests a typical case of updating an ignore rule in
// IgnoreStore.
func TestUpdateIgnoreRule_SunnyDay_Success(t *testing.T) {
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
	wh.UpdateIgnoreRule(w, r)

	assertJSONResponseWas(t, http.StatusOK, `{"updated":"true"}`, w)
}

// TestUpdateIgnoreRule_NoID_BadRequestError tests an exceptional case of attempting to update
// an ignore rule without providing an id for that ignore rule.
func TestUpdateIgnoreRule_NoID_BadRequestError(t *testing.T) {
	unittest.SmallTest(t)

	wh := Handlers{
		testingAuthAs: "test@google.com",
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, requestURL, strings.NewReader("doesn't matter"))
	wh.UpdateIgnoreRule(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestUpdateIgnoreRule_StoreFailure_InternalServerError tests an exceptional case of attempting
// to update an ignore rule in which there is an error returned by the IgnoreStore.
func TestUpdateIgnoreRule_StoreFailure_InternalServerError(t *testing.T) {
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
	wh.UpdateIgnoreRule(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// TestDeleteIgnoreRule_RuleExists_SunnyDay_Success tests a typical case of deleting an ignore
// rule which exists in the IgnoreStore.
func TestDeleteIgnoreRule_RuleExists_SunnyDay_Success(t *testing.T) {
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
	wh.DeleteIgnoreRule(w, r)

	assertJSONResponseWas(t, http.StatusOK, `{"deleted":"true"}`, w)
}

// TestDeleteIgnoreRule_NoID_InternalServerError tests an exceptional case of attempting to
// delete an ignore rule without providing an id for that ignore rule.
func TestDeleteIgnoreRule_NoID_InternalServerError(t *testing.T) {
	unittest.SmallTest(t)

	wh := Handlers{
		testingAuthAs: "test@google.com",
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, requestURL, strings.NewReader("doesn't matter"))
	wh.DeleteIgnoreRule(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestDeleteIgnoreRule_StoreFailure_InternalServerError tests an exceptional case of attempting
// to delete an ignore rule in which there is an error returned by the IgnoreStore (note: There
// is no error returned from ignore.Store when deleting a rule which does not exist).
func TestDeleteIgnoreRule_StoreFailure_InternalServerError(t *testing.T) {
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
	wh.DeleteIgnoreRule(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// TestBaselineHandler_Success tests that the handler correctly calls the BaselineFetcher when no
// GET parameters are set.
func TestBaselineHandler_Success(t *testing.T) {
	unittest.SmallTest(t)

	mbf := &mocks.BaselineFetcher{}
	mcls := &mock_clstore.Store{}
	mcls.On("System").Return("gerrit")

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Baseliner:       mbf,
			ChangeListStore: mcls,
		},
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)

	// Prepare a fake response from the BaselineFetcher and the handler's expected JSON response.
	bl := &baseline.Baseline{ChangeListID: "", MD5: "fakehash", CodeReviewSystem: "gerrit"}
	expectedJSONResponse := `{"md5":"fakehash","master_str":null,"master":null,"crs":"gerrit"}`

	// FetchBaseline should be called as per the request parameters.
	mbf.On("FetchBaseline", testutils.AnyContext, "" /* =clID */, "gerrit", false /* =issueOnly */).Return(bl, nil)
	defer mbf.AssertExpectations(t) // Assert that the method above was called exactly as expected.

	// We'll use the counters to assert that the right route was called.
	legacyRouteCounter := metrics2.GetCounter("gold_baselinehandler_route_legacy").Get()
	newRouteCounter := metrics2.GetCounter("gold_baselinehandler_route_new").Get()

	// Call route handler under test.
	wh.BaselineHandler(w, r)
	assertJSONResponseWas(t, http.StatusOK, expectedJSONResponse, w)

	// Assert that the right route was called.
	assert.Equal(t, legacyRouteCounter, metrics2.GetCounter("gold_baselinehandler_route_legacy").Get())
	assert.Equal(t, newRouteCounter+1, metrics2.GetCounter("gold_baselinehandler_route_new").Get())
}

// TestBaselineHandler_IssueSet_Success tests that the handler correctly calls the BaselineFetcher
// when the "issue" GET parameter is set.
func TestBaselineHandler_IssueSet_Success(t *testing.T) {
	unittest.SmallTest(t)

	mbf := &mocks.BaselineFetcher{}
	mcls := &mock_clstore.Store{}
	mcls.On("System").Return("gerrit")

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Baseliner:       mbf,
			ChangeListStore: mcls,
		},
	}
	w := httptest.NewRecorder()

	// GET parameter "issue" is set.
	r := httptest.NewRequest(http.MethodGet, requestURL+"?issue=123456", nil)

	// Prepare a fake response from the BaselineFetcher and the handler's expected JSON response.
	bl := &baseline.Baseline{ChangeListID: "", MD5: "fakehash", CodeReviewSystem: "gerrit"}
	expectedJSONResponse := `{"md5":"fakehash","master_str":null,"master":null,"crs":"gerrit"}`

	// FetchBaseline should be called as per the request parameters.
	mbf.On("FetchBaseline", testutils.AnyContext, "123456" /* =clID */, "gerrit", false /* =issueOnly */).Return(bl, nil)
	defer mbf.AssertExpectations(t) // Assert that the method above was called exactly as expected.

	// We'll use the counters to assert that the right route was called.
	legacyRouteCounter := metrics2.GetCounter("gold_baselinehandler_route_legacy").Get()
	newRouteCounter := metrics2.GetCounter("gold_baselinehandler_route_new").Get()

	// Call route handler under test.
	wh.BaselineHandler(w, r)
	assertJSONResponseWas(t, http.StatusOK, expectedJSONResponse, w)

	// Assert that the right route was called.
	assert.Equal(t, legacyRouteCounter, metrics2.GetCounter("gold_baselinehandler_route_legacy").Get())
	assert.Equal(t, newRouteCounter+1, metrics2.GetCounter("gold_baselinehandler_route_new").Get())
}

// TestBaselineHandler_IssueSet_Success tests that the handler correctly calls the BaselineFetcher
// when the "issue" and "issueOnly" GET parameters are set.
func TestBaselineHandler_IssueSet_IssueOnly_Success(t *testing.T) {
	unittest.SmallTest(t)

	mbf := &mocks.BaselineFetcher{}
	mcls := &mock_clstore.Store{}
	mcls.On("System").Return("gerrit")

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Baseliner:       mbf,
			ChangeListStore: mcls,
		},
	}
	w := httptest.NewRecorder()

	// GET parameters "issue" and "issueOnly" are set.
	r := httptest.NewRequest(http.MethodGet, requestURL+"?issue=123456&issueOnly=true", nil)

	// Prepare a fake response from the BaselineFetcher and the handler's expected JSON response.
	bl := &baseline.Baseline{ChangeListID: "", MD5: "fakehash", CodeReviewSystem: "gerrit"}
	expectedJSONResponse := `{"md5":"fakehash","master_str":null,"master":null,"crs":"gerrit"}`

	// FetchBaseline should be called as per the request parameters.
	mbf.On("FetchBaseline", testutils.AnyContext, "123456" /* =clID */, "gerrit", true /* =issueOnly */).Return(bl, nil)
	defer mbf.AssertExpectations(t) // Assert that the method above was called exactly as expected.

	// We'll use the counters to assert that the right route was called.
	legacyRouteCounter := metrics2.GetCounter("gold_baselinehandler_route_legacy").Get()
	newRouteCounter := metrics2.GetCounter("gold_baselinehandler_route_new").Get()

	// Call route handler under test.
	wh.BaselineHandler(w, r)
	assertJSONResponseWas(t, http.StatusOK, expectedJSONResponse, w)

	// Assert that the right route was called.
	assert.Equal(t, legacyRouteCounter, metrics2.GetCounter("gold_baselinehandler_route_legacy").Get())
	assert.Equal(t, newRouteCounter+1, metrics2.GetCounter("gold_baselinehandler_route_new").Get())
}

// TestBaselineHandler_BaselineFetcherError_InternalServerError tests that the handler correctly
// handles BaselineFetcher errors.
func TestBaselineHandler_BaselineFetcherError_InternalServerError(t *testing.T) {
	unittest.SmallTest(t)

	mbf := &mocks.BaselineFetcher{}
	mbf.On("FetchBaseline", testutils.AnyContext, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("oops"))

	mcls := &mock_clstore.Store{}
	mcls.On("System").Return("gerrit")

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Baseliner:       mbf,
			ChangeListStore: mcls,
		},
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)

	// We'll use the counters to assert that the right route was called.
	legacyRouteCounter := metrics2.GetCounter("gold_baselinehandler_route_legacy").Get()
	newRouteCounter := metrics2.GetCounter("gold_baselinehandler_route_new").Get()

	// Call route handler under test.
	wh.BaselineHandler(w, r)
	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	// Assert that the right route was called.
	assert.Equal(t, legacyRouteCounter, metrics2.GetCounter("gold_baselinehandler_route_legacy").Get())
	assert.Equal(t, newRouteCounter+1, metrics2.GetCounter("gold_baselinehandler_route_new").Get())
}

// TestBaselineHandler_CommitHashSet_IgnoresCommitHash_Success tests that the {commit_hash} URL
// variable in the /json/expectations/commit/{commit_hash} route is ignored.
// TODO(lovisolo): Remove along with {commit_hash} and any references.
func TestBaselineHandler_CommitHashSet_IgnoresCommitHash_Success(t *testing.T) {
	unittest.SmallTest(t)

	mbf := &mocks.BaselineFetcher{}
	mcls := &mock_clstore.Store{}
	mcls.On("System").Return("gerrit")

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Baseliner:       mbf,
			ChangeListStore: mcls,
		},
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)

	// Set the {commit_hash} URL variable in /json/expectations/commit/{commit_hash}.
	r = mux.SetURLVars(r, map[string]string{"commit_hash": "09e87c3d93e2bb188a8dae01b7f8b9ffb2ebcad1"})

	// Prepare a fake response from the BaselineFetcher and the handler's expected JSON response.
	bl := &baseline.Baseline{ChangeListID: "", MD5: "fakehash", CodeReviewSystem: "gerrit"}
	expectedJSONResponse := `{"md5":"fakehash","master_str":null,"master":null,"crs":"gerrit"}`

	// Note that the {commit_hash} doesn't appear anywhere in the FetchBaseline call.
	mbf.On("FetchBaseline", testutils.AnyContext, "" /* =clID */, "gerrit", false /* =issueOnly */).Return(bl, nil)
	defer mbf.AssertExpectations(t) // Assert that the method above was called exactly as expected.

	// We'll use the counters to assert that the right route was called.
	legacyRouteCounter := metrics2.GetCounter("gold_baselinehandler_route_legacy").Get()
	newRouteCounter := metrics2.GetCounter("gold_baselinehandler_route_new").Get()

	// Call route handler under test.
	wh.BaselineHandler(w, r)
	assertJSONResponseWas(t, http.StatusOK, expectedJSONResponse, w)

	// Assert that the right route was called.
	assert.Equal(t, legacyRouteCounter+1, metrics2.GetCounter("gold_baselinehandler_route_legacy").Get())
	assert.Equal(t, newRouteCounter, metrics2.GetCounter("gold_baselinehandler_route_new").Get())
}

// TestBaselineHandler_CommitHashSet_IssueSet_IgnoresCommitHash_Success tests that the
// {commit_hash} URL variable in the /json/expectations/commit/{commit_hash} route is ignored and
// that the "issue" GET parameter is handled correctly.
// TODO(lovisolo): Remove along with {commit_hash} and any references.
func TestBaselineHandler_CommitHashSet_IssueSet_IgnoresCommitHash_Success(t *testing.T) {
	unittest.SmallTest(t)

	mbf := &mocks.BaselineFetcher{}
	mcls := &mock_clstore.Store{}
	mcls.On("System").Return("gerrit")

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Baseliner:       mbf,
			ChangeListStore: mcls,
		},
	}
	w := httptest.NewRecorder()

	// GET parameter "issue" is set.
	r := httptest.NewRequest(http.MethodGet, requestURL+"?issue=123456", nil)

	// Set the {commit_hash} URL variable in /json/expectations/commit/{commit_hash}.
	r = mux.SetURLVars(r, map[string]string{"commit_hash": "09e87c3d93e2bb188a8dae01b7f8b9ffb2ebcad1"})

	// Prepare a fake response from the BaselineFetcher and the handler's expected JSON response.
	bl := &baseline.Baseline{ChangeListID: "", MD5: "fakehash", CodeReviewSystem: "gerrit"}
	expectedJSONResponse := `{"md5":"fakehash","master_str":null,"master":null,"crs":"gerrit"}`

	// Note that the {commit_hash} doesn't appear anywhere in the FetchBaseline call.
	mbf.On("FetchBaseline", testutils.AnyContext, "123456" /* =clID */, "gerrit", false /* =issueOnly */).Return(bl, nil)
	defer mbf.AssertExpectations(t) // Assert that the method above was called exactly as expected.

	// We'll use the counters to assert that the right route was called.
	legacyRouteCounter := metrics2.GetCounter("gold_baselinehandler_route_legacy").Get()
	newRouteCounter := metrics2.GetCounter("gold_baselinehandler_route_new").Get()

	// Call route handler under test.
	wh.BaselineHandler(w, r)
	assertJSONResponseWas(t, http.StatusOK, expectedJSONResponse, w)

	// Assert that the right route was called.
	assert.Equal(t, legacyRouteCounter+1, metrics2.GetCounter("gold_baselinehandler_route_legacy").Get())
	assert.Equal(t, newRouteCounter, metrics2.GetCounter("gold_baselinehandler_route_new").Get())
}

// TestBaselineHandler_CommitHashSet_IssueSet_IssueOnly_IgnoresCommitHash_Success tests that the
// {commit_hash} URL variable in the /json/expectations/commit/{commit_hash} route is ignored and
// that the "issue" and "issueOnly" GET parameters are handled correctly.
// TODO(lovisolo): Remove along with {commit_hash} and any references.
func TestBaselineHandler_CommitHashSet_IssueSet_IssueOnly_IgnoresCommitHash_Success(t *testing.T) {
	unittest.SmallTest(t)

	mbf := &mocks.BaselineFetcher{}
	mcls := &mock_clstore.Store{}
	mcls.On("System").Return("gerrit")

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Baseliner:       mbf,
			ChangeListStore: mcls,
		},
	}
	w := httptest.NewRecorder()

	// GET parameters "issue" and "issueOnly" are set.
	r := httptest.NewRequest(http.MethodGet, requestURL+"?issue=123456&issueOnly=true", nil)

	// Set the {commit_hash} URL variable in /json/expectations/commit/{commit_hash}.
	r = mux.SetURLVars(r, map[string]string{"commit_hash": "09e87c3d93e2bb188a8dae01b7f8b9ffb2ebcad1"})

	// Prepare a fake response from the BaselineFetcher and the handler's expected JSON response.
	bl := &baseline.Baseline{ChangeListID: "", MD5: "fakehash", CodeReviewSystem: "gerrit"}
	expectedJSONResponse := `{"md5":"fakehash","master_str":null,"master":null,"crs":"gerrit"}`

	// Note that the {commit_hash} doesn't appear anywhere in the FetchBaseline call.
	mbf.On("FetchBaseline", testutils.AnyContext, "123456" /* =clID */, "gerrit", true /* =issueOnly */).Return(bl, nil)
	defer mbf.AssertExpectations(t) // Assert that the method above was called exactly as expected.

	// We'll use the counters to assert that the right route was called.
	legacyRouteCounter := metrics2.GetCounter("gold_baselinehandler_route_legacy").Get()
	newRouteCounter := metrics2.GetCounter("gold_baselinehandler_route_new").Get()

	// Call route handler under test.
	wh.BaselineHandler(w, r)
	assertJSONResponseWas(t, http.StatusOK, expectedJSONResponse, w)

	// Assert that the right route was called.
	assert.Equal(t, legacyRouteCounter+1, metrics2.GetCounter("gold_baselinehandler_route_legacy").Get())
	assert.Equal(t, newRouteCounter, metrics2.GetCounter("gold_baselinehandler_route_new").Get())
}

// TestWhoami_NotLoggedIn_Success tests that /json/whoami returns the expected empty response when
// no user is logged in.
func TestWhoami_NotLoggedIn_Success(t *testing.T) {
	unittest.SmallTest(t)
	wh := Handlers{
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	wh.Whoami(w, r)
	assertJSONResponseWas(t, http.StatusOK, `{"whoami":""}`, w)
}

// TestWhoami_LoggedIn_Success tests that /json/whoami returns the email of the user that is
// currently logged in.
func TestWhoami_LoggedIn_Success(t *testing.T) {
	unittest.SmallTest(t)
	wh := Handlers{
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
		testingAuthAs:       "test@example.com",
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	wh.Whoami(w, r)
	assertJSONResponseWas(t, http.StatusOK, `{"whoami":"test@example.com"}`, w)
}

// TestLatestPositiveDigest_Success tests that /json/latestpositivedigest/{traceId} returns the
// most recent positive digest in the expected format.
//
// Note: We don't test the cases when the tile has no positive digests, or when the trace isn't
// found, because it both cases SearchIndexer method MostRecentPositiveDigest() will just return
// types.MissingDigest and a nil error.
func TestLatestPositiveDigest_Success(t *testing.T) {
	unittest.SmallTest(t)

	mockIndexSearcher := &mock_indexer.IndexSearcher{}
	mockIndexSearcher.AssertExpectations(t)
	mockIndexSource := &mock_indexer.IndexSource{}
	mockIndexSource.AssertExpectations(t)

	const traceId = tiling.TraceID(",foo=bar,")
	const digest = types.Digest("11111111111111111111111111111111")
	const expectedJSONResponse = `{"digest":"11111111111111111111111111111111"}`

	mockIndexSource.On("GetIndex").Return(mockIndexSearcher)
	mockIndexSearcher.On("MostRecentPositiveDigest", testutils.AnyContext, traceId).Return(digest, nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer: mockIndexSource,
		},
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	r = mux.SetURLVars(r, map[string]string{"traceId": string(traceId)})

	wh.LatestPositiveDigestHandler(w, r)
	assertJSONResponseWas(t, http.StatusOK, expectedJSONResponse, w)
}

// TestLatestPositiveDigest_SearchIndexerFailure_InternalServerError tests that
// /json/latestpositivedigest/{traceId} produces an internal server error when SearchIndexer method
// MostRecentPositiveDigest() returns a non-nil error.
func TestLatestPositiveDigest_SearchIndexerFailure_InternalServerError(t *testing.T) {
	unittest.SmallTest(t)

	mockIndexSearcher := &mock_indexer.IndexSearcher{}
	mockIndexSearcher.AssertExpectations(t)
	mockIndexSource := &mock_indexer.IndexSource{}
	mockIndexSource.AssertExpectations(t)

	const traceId = tiling.TraceID(",foo=bar,")

	mockIndexSource.On("GetIndex").Return(mockIndexSearcher)
	mockIndexSearcher.On("MostRecentPositiveDigest", testutils.AnyContext, traceId).Return(tiling.MissingDigest, errors.New("kaboom"))

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer: mockIndexSource,
		},
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	r = mux.SetURLVars(r, map[string]string{"traceId": string(traceId)})

	wh.LatestPositiveDigestHandler(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
	assert.Contains(t, w.Body.String(), "Could not retrieve most recent positive digest.")
}

func TestGetPerTraceDigestsByTestName_Success(t *testing.T) {
	unittest.SmallTest(t)

	mockIndexSearcher := &mock_indexer.IndexSearcher{}
	mockIndexSource := &mock_indexer.IndexSource{}

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer: mockIndexSource,
		},
		anonymousExpensiveQuota: rate.NewLimiter(rate.Inf, 1),
	}

	mockIndexSource.On("GetIndex").Return(mockIndexSearcher)
	mockIndexSearcher.On("SlicedTraces", types.IncludeIgnoredTraces, map[string][]string{
		types.CorpusField:     {"MyCorpus"},
		types.PrimaryKeyField: {"MyTest"},
	}).Return([]*tiling.TracePair{
		{
			ID: ",name=MyTest,foo=alpha,source_type=MyCorpus,",
			Trace: tiling.NewTrace([]types.Digest{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			}, map[string]string{
				"name":        "MyTest",
				"foo":         "alpha",
				"source_type": "MyCorpus",
			}, nil),
		},
		{
			ID: ",name=MyTest,foo=beta,source_type=MyCorpus,",
			Trace: tiling.NewTrace([]types.Digest{
				"",
				"cccccccccccccccccccccccccccccccc",
			}, map[string]string{
				"name":        "MyTest",
				"foo":         "beta",
				"source_type": "MyCorpus",
			}, nil),
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	r = mux.SetURLVars(r, map[string]string{
		"corpus":   "MyCorpus",
		"testName": "MyTest",
	})

	wh.GetPerTraceDigestsByTestName(w, r)
	const expectedResponse = `{",name=MyTest,foo=alpha,source_type=MyCorpus,":["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"],",name=MyTest,foo=beta,source_type=MyCorpus,":["","cccccccccccccccccccccccccccccccc"]}`
	assertJSONResponseWas(t, http.StatusOK, expectedResponse, w)
}

func TestParamsHandler_MasterBranch_Success(t *testing.T) {
	unittest.SmallTest(t)

	mockIndexSearcher := &mock_indexer.IndexSearcher{}
	mockIndexSource := &mock_indexer.IndexSource{}

	mockIndexSource.On("GetIndex").Return(mockIndexSearcher)

	cpxTile := tiling.NewComplexTile(&tiling.Tile{
		ParamSet: paramtools.ParamSet{
			types.CorpusField:     []string{"first_corpus", "second_corpus"},
			types.PrimaryKeyField: []string{"alpha_test", "beta_test", "gamma_test"},
			"os":                  []string{"Android XYZ"},
		},
		// Other fields should be ignored.
	})

	mockIndexSearcher.On("Tile").Return(cpxTile)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer: mockIndexSource,
		},
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	wh.ParamsHandler(w, r)
	const expectedResponse = `{"name":["alpha_test","beta_test","gamma_test"],"os":["Android XYZ"],"source_type":["first_corpus","second_corpus"]}`
	assertJSONResponseWas(t, http.StatusOK, expectedResponse, w)
}

func TestParamsHandler_ChangeListIndex_Success(t *testing.T) {
	unittest.SmallTest(t)

	mockCLStore := &mock_clstore.Store{}
	mockIndexSource := &mock_indexer.IndexSource{}
	defer mockIndexSource.AssertExpectations(t) // want to make sure fallback happened

	const crs = "gerrit"
	const clID = "1234"

	clIdx := indexer.ChangeListIndex{
		ParamSet: paramtools.ParamSet{
			types.CorpusField:     []string{"first_corpus", "second_corpus"},
			types.PrimaryKeyField: []string{"alpha_test", "beta_test", "gamma_test"},
			"os":                  []string{"Android XYZ"},
		},
	}
	mockIndexSource.On("GetIndexForCL", crs, clID).Return(&clIdx)
	mockCLStore.On("System").Return(crs)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer:         mockIndexSource,
			ChangeListStore: mockCLStore,
		},
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/json/paramset?changelist_id=1234", nil)
	wh.ParamsHandler(w, r)
	const expectedResponse = `{"name":["alpha_test","beta_test","gamma_test"],"os":["Android XYZ"],"source_type":["first_corpus","second_corpus"]}`
	assertJSONResponseWas(t, http.StatusOK, expectedResponse, w)
}

func TestParamsHandler_NoChangeListIndex_FallBackToMasterBranch(t *testing.T) {
	unittest.SmallTest(t)

	mockCLStore := &mock_clstore.Store{}
	mockIndexSearcher := &mock_indexer.IndexSearcher{}
	mockIndexSource := &mock_indexer.IndexSource{}
	defer mockIndexSource.AssertExpectations(t) // want to make sure fallback happened

	const crs = "gerrit"
	const clID = "1234"

	mockIndexSource.On("GetIndex").Return(mockIndexSearcher)
	mockIndexSource.On("GetIndexForCL", crs, clID).Return(nil)

	cpxTile := tiling.NewComplexTile(&tiling.Tile{
		ParamSet: paramtools.ParamSet{
			types.CorpusField:     []string{"first_corpus", "second_corpus"},
			types.PrimaryKeyField: []string{"alpha_test", "beta_test", "gamma_test"},
			"os":                  []string{"Android XYZ"},
		},
		// Other fields should be ignored.
	})

	mockIndexSearcher.On("Tile").Return(cpxTile)

	mockCLStore.On("System").Return(crs)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer:         mockIndexSource,
			ChangeListStore: mockCLStore,
		},
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/json/paramset?changelist_id=1234", nil)
	wh.ParamsHandler(w, r)
	const expectedResponse = `{"name":["alpha_test","beta_test","gamma_test"],"os":["Android XYZ"],"source_type":["first_corpus","second_corpus"]}`
	assertJSONResponseWas(t, http.StatusOK, expectedResponse, w)
}

func TestChangeListSearchRedirect_CLHasUntriagedDigests_Success(t *testing.T) {
	unittest.SmallTest(t)

	mockSearchAPI := &mock_search.SearchAPI{}
	mockIndexSource := &mock_indexer.IndexSource{}

	const crs = "gerrit"
	const clID = "1234"

	combinedID := tjstore.CombinedPSID{
		CRS: crs,
		CL:  clID,
		PS:  "some patchset",
	}

	mockIndexSource.On("GetIndexForCL", crs, clID).Return(&indexer.ChangeListIndex{
		LatestPatchSet: combinedID,
		// Other fields should be ignored
	})

	mockSearchAPI.On("UntriagedUnignoredTryJobExclusiveDigests", testutils.AnyContext, combinedID).Return(&search_fe.UntriagedDigestList{
		Corpora: []string{"one_corpus", "two_corpus"},
	}, nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer:   mockIndexSource,
			SearchAPI: mockSearchAPI,
		},
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/cl/gerrit/1234", nil)
	r = mux.SetURLVars(r, map[string]string{
		"system": crs,
		"id":     clID,
	})
	wh.ChangeListSearchRedirect(w, r)
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	headers := w.Header()
	assert.Equal(t, []string{"/search?issue=1234&query=source_type%3Done_corpus"}, headers["Location"])
}

func TestChangeListSearchRedirect_ErrorGettingUntriagedDigests_RedirectsWithoutCorpus(t *testing.T) {
	unittest.SmallTest(t)

	mockSearchAPI := &mock_search.SearchAPI{}
	mockIndexSource := &mock_indexer.IndexSource{}
	defer mockSearchAPI.AssertExpectations(t) // make sure the error was returned

	const crs = "gerrit"
	const clID = "1234"

	combinedID := tjstore.CombinedPSID{
		CRS: crs,
		CL:  clID,
		PS:  "some patchset",
	}

	mockIndexSource.On("GetIndexForCL", crs, clID).Return(&indexer.ChangeListIndex{
		LatestPatchSet: combinedID,
		// Other fields should be ignored
	})

	mockSearchAPI.On("UntriagedUnignoredTryJobExclusiveDigests", testutils.AnyContext, combinedID).Return(nil, errors.New("this shouldn't happen"))

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer:   mockIndexSource,
			SearchAPI: mockSearchAPI,
		},
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/cl/gerrit/1234", nil)
	r = mux.SetURLVars(r, map[string]string{
		"system": crs,
		"id":     clID,
	})
	wh.ChangeListSearchRedirect(w, r)
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	headers := w.Header()
	assert.Equal(t, []string{"/search?issue=1234"}, headers["Location"])
}

func TestChangeListSearchRedirect_CLExistsNotIndexed_RedirectsWithoutCorpus(t *testing.T) {
	unittest.SmallTest(t)

	mockIndexSource := &mock_indexer.IndexSource{}
	mockCLStore := &mock_clstore.Store{}

	const crs = "gerrit"
	const clID = "1234"

	mockIndexSource.On("GetIndexForCL", crs, clID).Return(nil)

	// all this cares about is a non-error return code
	mockCLStore.On("GetChangeList", testutils.AnyContext, clID).Return(code_review.ChangeList{}, nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer:         mockIndexSource,
			ChangeListStore: mockCLStore,
		},
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/cl/gerrit/1234", nil)
	r = mux.SetURLVars(r, map[string]string{
		"system": crs,
		"id":     clID,
	})
	wh.ChangeListSearchRedirect(w, r)
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	headers := w.Header()
	assert.Equal(t, []string{"/search?issue=1234"}, headers["Location"])
}

func TestChangeListSearchRedirect_CLDoesNotExist_404Error(t *testing.T) {
	unittest.SmallTest(t)

	mockIndexSource := &mock_indexer.IndexSource{}
	mockCLStore := &mock_clstore.Store{}

	const crs = "gerrit"
	const clID = "1234"

	mockIndexSource.On("GetIndexForCL", crs, clID).Return(nil)

	// all this cares about is a non-error return code
	mockCLStore.On("GetChangeList", testutils.AnyContext, clID).Return(code_review.ChangeList{}, code_review.ErrNotFound)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer:         mockIndexSource,
			ChangeListStore: mockCLStore,
		},
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/cl/gerrit/1234", nil)
	r = mux.SetURLVars(r, map[string]string{
		"system": crs,
		"id":     clID,
	})
	wh.ChangeListSearchRedirect(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetFlakyTracesData_ThresholdZero_ReturnAllTraces(t *testing.T) {
	unittest.SmallTest(t)

	mi := &mock_indexer.IndexSource{}
	defer mi.AssertExpectations(t)

	commits := bug_revert.MakeTestCommits()
	fis := makeBugRevertIndex(len(commits))
	mi.On("GetIndex").Return(fis)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer: mi,
		},
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	r = mux.SetURLVars(r, map[string]string{
		"minUniqueDigests": "0",
	})
	wh.GetFlakyTracesData(w, r)
	const expectedRV = `{"traces":[` +
		`{"trace_id":",device=gamma,name=test_two,source_type=gm,","unique_digests_count":4},` +
		`{"trace_id":",device=alpha,name=test_one,source_type=gm,","unique_digests_count":2},` +
		`{"trace_id":",device=alpha,name=test_two,source_type=gm,","unique_digests_count":2},` +
		`{"trace_id":",device=beta,name=test_one,source_type=gm,","unique_digests_count":2},` +
		`{"trace_id":",device=delta,name=test_one,source_type=gm,","unique_digests_count":2},` +
		`{"trace_id":",device=delta,name=test_two,source_type=gm,","unique_digests_count":2},` +
		`{"trace_id":",device=gamma,name=test_one,source_type=gm,","unique_digests_count":2},` +
		`{"trace_id":",device=beta,name=test_two,source_type=gm,","unique_digests_count":1}],` +
		`"tile_size":5,"num_flaky":8,"num_traces":8}`
	assertJSONResponseWas(t, 200, expectedRV, w)
}

func TestGetFlakyTracesData_NonZeroThreshold_ReturnsFlakyTracesAboveThreshold(t *testing.T) {
	unittest.SmallTest(t)

	mi := &mock_indexer.IndexSource{}
	defer mi.AssertExpectations(t)

	commits := bug_revert.MakeTestCommits()
	fis := makeBugRevertIndex(len(commits))
	mi.On("GetIndex").Return(fis)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer: mi,
		},
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	r = mux.SetURLVars(r, map[string]string{
		"minUniqueDigests": "4",
	})
	wh.GetFlakyTracesData(w, r)
	const expectedRV = `{"traces":[{"trace_id":",device=gamma,name=test_two,source_type=gm,","unique_digests_count":4}],"tile_size":5,"num_flaky":1,"num_traces":8}`
	assertJSONResponseWas(t, 200, expectedRV, w)
}

func TestGetFlakyTracesData_NoTracesAboveThreshold_ReturnsZeroTraces(t *testing.T) {
	unittest.SmallTest(t)

	mi := &mock_indexer.IndexSource{}

	commits := bug_revert.MakeTestCommits()
	fis := makeBugRevertIndex(len(commits))
	mi.On("GetIndex").Return(fis)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Indexer: mi,
		},
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	wh.GetFlakyTracesData(w, r)
	const expectedRV = `{"traces":null,"tile_size":5,"num_flaky":0,"num_traces":8}`
	assertJSONResponseWas(t, 200, expectedRV, w)
}

func TestListTestsHandler_ValidQueries_Success(t *testing.T) {
	unittest.SmallTest(t)

	test := func(name, targetURL, expectedJSON string) {
		t.Run(name, func(t *testing.T) {
			mi := &mock_indexer.IndexSource{}

			fis := makeBugRevertIndexWithIgnores(makeIgnoreRules(), 1)
			mi.On("GetIndex").Return(fis)

			wh := Handlers{
				HandlersConfig: HandlersConfig{
					Indexer: mi,
				},
				anonymousExpensiveQuota: rate.NewLimiter(rate.Inf, 1),
			}
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, targetURL, nil)
			wh.ListTestsHandler(w, r)
			assertJSONResponseWas(t, http.StatusOK, expectedJSON, w)
		})
	}

	test("all GM tests from all traces", "/json/list?corpus=gm&include_ignored_traces=true",
		`[{"name":"test_one","positive_digests":1,"negative_digests":0,"untriaged_digests":1},`+
			`{"name":"test_two","positive_digests":2,"negative_digests":0,"untriaged_digests":2}]`)

	test("all GM tests at head from all traces", "/json/list?corpus=gm&at_head_only=true&include_ignored_traces=true",
		`[{"name":"test_one","positive_digests":1,"negative_digests":0,"untriaged_digests":0},`+
			`{"name":"test_two","positive_digests":2,"negative_digests":0,"untriaged_digests":1}]`)

	test("all GM tests for device beta from all traces", "/json/list?corpus=gm&trace_values=device%3Dbeta&include_ignored_traces=true",
		`[{"name":"test_one","positive_digests":1,"negative_digests":0,"untriaged_digests":1},`+
			`{"name":"test_two","positive_digests":1,"negative_digests":0,"untriaged_digests":0}]`)

	test("all GM tests for device delta at head from all traces", "/json/list?corpus=gm&trace_values=device%3Ddelta&at_head_only=true&include_ignored_traces=true",
		`[{"name":"test_one","positive_digests":1,"negative_digests":0,"untriaged_digests":0},`+
			`{"name":"test_two","positive_digests":0,"negative_digests":0,"untriaged_digests":1}]`)

	// Reminder that device delta and test_two match ignore rules
	test("all GM tests", "/json/list?corpus=gm",
		`[{"name":"test_one","positive_digests":1,"negative_digests":0,"untriaged_digests":1}]`)

	test("all GM tests at head", "/json/list?corpus=gm&at_head_only=true",
		`[{"name":"test_one","positive_digests":1,"negative_digests":0,"untriaged_digests":0}]`)

	test("all GM tests for device beta", "/json/list?corpus=gm&trace_values=device%3Dbeta",
		`[{"name":"test_one","positive_digests":1,"negative_digests":0,"untriaged_digests":1}]`)

	test("all GM tests for device delta at head", "/json/list?corpus=gm&trace_values=device%3Ddelta&at_head_only=true",
		"[]")

	test("non existent corpus", "/json/list?corpus=notthere", "[]")
}

func TestListTestsHandler_InvalidQueries_BadRequestError(t *testing.T) {
	unittest.SmallTest(t)

	test := func(name, targetURL string) {
		t.Run(name, func(t *testing.T) {
			mi := &mock_indexer.IndexSource{}

			fis := makeBugRevertIndexWithIgnores(makeIgnoreRules(), 1)
			mi.On("GetIndex").Return(fis)

			wh := Handlers{
				HandlersConfig: HandlersConfig{
					Indexer: mi,
				},
				anonymousExpensiveQuota: rate.NewLimiter(rate.Inf, 1),
			}
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, targetURL, nil)
			wh.ListTestsHandler(w, r)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}

	test("missing corpus", "/json/list")

	test("empty corpus", "/json/list?corpus=")

	test("invalid trace values", "/json/list?corpus=gm&trace_values=%zz")
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
