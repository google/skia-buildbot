package commenter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metrics_utils "go.skia.org/infra/go/metrics2/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/clstore"
	mock_clstore "go.skia.org/infra/golden/go/clstore/mocks"
	"go.skia.org/infra/golden/go/code_review"
	mock_codereview "go.skia.org/infra/golden/go/code_review/mocks"
	"go.skia.org/infra/golden/go/search/frontend"
	mock_search "go.skia.org/infra/golden/go/search/mocks"
	"go.skia.org/infra/golden/go/types"
)

// TestUpdateNotOpenBotsSunnyDay tests a typical case where two of the known open CLs are no longer
// open and must be updated in the clstore.
func TestUpdateNotOpenBotsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}
	defer mcs.AssertExpectations(t)

	optionsMatcher := mock.MatchedBy(func(options clstore.SearchOptions) bool {
		if options.StartIdx != 0 && options.StartIdx != 8 {
			assert.Fail(t, "Unexpected value for StartIdx: %d", options.StartIdx)
		}
		assert.True(t, options.Limit > 10)
		assert.True(t, options.OpenCLsOnly)
		assert.NotZero(t, options.After)
		return true
	})
	mcs.On("GetChangeLists", testutils.AnyContext, optionsMatcher).Return(makeChangeLists(10), 10, nil).Once()
	mcs.On("GetChangeLists", testutils.AnyContext, optionsMatcher).Return(nil, 10, nil).Once()
	mcs.On("GetPatchSets", testutils.AnyContext, mock.Anything).Return(makePatchSets(2, false), nil)

	xcl := makeChangeLists(10)
	mcr.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(func(ctx context.Context, id string) code_review.ChangeList {
		i, err := strconv.Atoi(id)
		assert.NoError(t, err)
		cl := xcl[i]
		cl.Subject = "Updated"
		if i == 3 {
			cl.Status = code_review.Abandoned
		}
		if i == 5 {
			cl.Status = code_review.Landed
		}
		xcl[i] = cl
		return cl
	}, nil)

	oldTime := xcl[0].Updated
	// This matcher checks that only the abandoned CL gets updated in the DB.
	putClMatcher := mock.MatchedBy(func(cl code_review.ChangeList) bool {
		assert.True(t, cl.Updated.After(oldTime))
		if cl.SystemID == "0003" {
			assert.Equal(t, code_review.Abandoned, cl.Status)
			return true
		}
		return false
	})
	mcs.On("PutChangeList", testutils.AnyContext, putClMatcher).Return(nil).Once()

	c := newTestCommenter(mcr, mcs, nil)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "8", metrics_utils.GetRecordedMetric(t, numRecentOpenCLsMetric, nil))
}

// TestUpdateNotOpenBotsNotFound tests a case where a single CL was not found by the CRS. In this
// case, we ignore the error and the CL for purposes of counting and commenting.
func TestUpdateNotOpenBotsNotFound(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}

	optionsMatcher := mock.MatchedBy(func(options clstore.SearchOptions) bool {
		if options.StartIdx != 0 && options.StartIdx != 4 {
			assert.Fail(t, "Unexpected value for StartIdx", options.StartIdx)
		}
		assert.True(t, options.Limit > 10)
		assert.True(t, options.OpenCLsOnly)
		assert.NotZero(t, options.After)
		return true
	})
	mcs.On("GetChangeLists", testutils.AnyContext, optionsMatcher).Return(makeChangeLists(5), 5, nil).Once()
	mcs.On("GetChangeLists", testutils.AnyContext, optionsMatcher).Return(nil, 5, nil).Once()
	mcs.On("GetPatchSets", testutils.AnyContext, mock.Anything).Return(makePatchSets(2, false), nil)

	xcl := makeChangeLists(5)
	mcr.On("GetChangeList", testutils.AnyContext, "0002").Return(code_review.ChangeList{}, code_review.ErrNotFound)
	mcr.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(func(ctx context.Context, id string) code_review.ChangeList {
		i, err := strconv.Atoi(id)
		assert.NoError(t, err)
		return xcl[i]
	}, nil)

	c := newTestCommenter(mcr, mcs, nil)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.NoError(t, err)
	// The one CL that was not found should not be counted as an open CL.
	assert.Equal(t, "4", metrics_utils.GetRecordedMetric(t, numRecentOpenCLsMetric, nil))
}

// TestUpdateBorkedCL tests a case where the only CLs returned by the clstore were deleted by
// the crs (and thus no longer exist). We want to make sure we don't hang, constantly querying over
// and over again.
func TestUpdateBorkedCL(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}

	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(makeChangeLists(5), 5, nil)
	mcr.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(code_review.ChangeList{}, code_review.ErrNotFound)

	c := newTestCommenter(mcr, mcs, nil)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.NoError(t, err)
	// This shouldn't hang and we'll see 0 open CLs
	assert.Equal(t, "0", metrics_utils.GetRecordedMetric(t, numRecentOpenCLsMetric, nil))
}

// TestUpdateNotOpenBotsCRSError checks that we bail out on a more-serious CRS error.
func TestUpdateNotOpenBotsCRSError(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}

	optionsMatcher := mock.MatchedBy(func(options clstore.SearchOptions) bool {
		assert.Equal(t, 0, options.StartIdx)
		assert.True(t, options.Limit > 10)
		assert.True(t, options.OpenCLsOnly)
		assert.NotZero(t, options.After)
		return true
	})
	mcs.On("GetChangeLists", testutils.AnyContext, optionsMatcher).Return(makeChangeLists(5), 5, nil)

	mcr.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(code_review.ChangeList{}, errors.New("GitHub down"))
	mcr.On("System").Return("github")

	c := newTestCommenter(mcr, mcs, nil)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	assertErrorWasCanceledOrContains(t, err, "down", "github")
}

// TestUpdateNotOpenBotsCLStoreError tests that we bail out if writing to the clstore fails.
func TestUpdateNotOpenBotsCLStoreError(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}
	defer mcs.AssertExpectations(t)

	optionsMatcher := mock.MatchedBy(func(options clstore.SearchOptions) bool {
		assert.Equal(t, 0, options.StartIdx)
		assert.True(t, options.Limit > 10)
		assert.True(t, options.OpenCLsOnly)
		assert.NotZero(t, options.After)
		return true
	})
	mcs.On("GetChangeLists", testutils.AnyContext, optionsMatcher).Return(makeChangeLists(5), 5, nil)

	mcr.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(code_review.ChangeList{
		Status: code_review.Abandoned,
	}, nil)

	mcs.On("PutChangeList", testutils.AnyContext, mock.Anything).Return(errors.New("firestore broke"))

	c := newTestCommenter(mcr, mcs, nil)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	assertErrorWasCanceledOrContains(t, err, "firestore broke")
}

// TestCommentOnCLsSunnyDay tests a typical case where two of the open ChangeLists have patchsets
// that have Untriaged images, and no comment yet.
func TestCommentOnCLsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}
	msa := &mock_search.SearchAPI{}
	defer mcr.AssertExpectations(t)
	defer mcs.AssertExpectations(t)

	var indexTime = time.Date(2020, time.May, 1, 2, 3, 4, 0, time.UTC)

	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(makeChangeLists(10), 10, nil).Once()
	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(nil, 10, nil).Once()
	// Mark two of the CLs (with id 0003 and 0007) as having untriaged digests in all n patchsets.
	mcs.On("GetPatchSets", testutils.AnyContext, "0003").Return(makePatchSets(4, true), nil)
	mcs.On("GetPatchSets", testutils.AnyContext, "0007").Return(makePatchSets(9, true), nil)
	mcs.On("GetPatchSets", testutils.AnyContext, mock.Anything).Return(makePatchSets(1, false), nil)

	// We should see two PatchSets with their CommentedOnCL and LastCheckedIfCommentNecessary field
	// written back to Firestore.
	patchSetsWereMarkedCommentedOn := mock.MatchedBy(func(ps code_review.PatchSet) bool {
		assert.True(t, ps.CommentedOnCL)
		assert.Equal(t, indexTime, ps.LastCheckedIfCommentNecessary)
		return true
	})
	mcs.On("PutPatchSet", testutils.AnyContext, patchSetsWereMarkedCommentedOn).Return(nil).Twice()

	xcl := makeChangeLists(10)
	mcr.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(func(ctx context.Context, id string) code_review.ChangeList {
		i, err := strconv.Atoi(id)
		assert.NoError(t, err)
		return xcl[i]
	}, nil)
	mcr.On("CommentOn", testutils.AnyContext, mock.Anything, mock.Anything).Return(func(ctx context.Context, clID, msg string) error {
		i, err := strconv.Atoi(clID)
		assert.NoError(t, err)
		if i == 3 {
			// On CL 0003, the most recent patchset with untriaged digests has order 4.
			assert.Contains(t, msg, "patchset 4")
			assert.Contains(t, msg, "gold.skia.org/search?issue=0003")
		} else if i == 7 {
			// On CL 0007, the most recent patchset with untriaged digests has order 9.
			assert.Contains(t, msg, "patchset 9")
			assert.Contains(t, msg, "gold.skia.org/search?issue=0007")
		} else {
			assert.Fail(t, "unexpected call")
		}
		assert.Contains(t, msg, "2 untriaged digest(s)")
		return nil
	}, nil)
	mcr.On("System").Return("github")

	// Pretend all CLs queried have 2 untriaged digests.
	msa.On("UntriagedUnignoredTryJobExclusiveDigests", testutils.AnyContext, mock.Anything).Return(&frontend.UntriagedDigestList{
		Digests: []types.Digest{"doesn't", "matter"},
		TS:      indexTime,
	}, nil)

	c := newTestCommenter(mcr, mcs, msa)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.NoError(t, err)
}

// CommentOnChangeListsWithUntriagedDigests_NoUntriagedDigests_Success tests a typical case where
// two of the open ChangeLists have patchsets have no Untriaged images, and no comment yet. We
// should the LastCheckedIfCommentNecessary data get updated but no comments made.
func TestCommentOnChangeListsWithUntriagedDigests_NoUntriagedDigests_Success(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}
	msa := &mock_search.SearchAPI{}
	defer mcs.AssertExpectations(t)

	var indexTime = time.Date(2020, time.May, 1, 2, 3, 4, 0, time.UTC)

	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(makeChangeLists(10), 10, nil).Once()
	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(nil, 10, nil).Once()
	// Mark two of the CLs (with id 0003 and 0007) as having untriaged digests in all n patchsets.
	mcs.On("GetPatchSets", testutils.AnyContext, "0003").Return(makePatchSets(4, true), nil)
	mcs.On("GetPatchSets", testutils.AnyContext, "0007").Return(makePatchSets(9, true), nil)
	mcs.On("GetPatchSets", testutils.AnyContext, mock.Anything).Return(makePatchSets(1, false), nil)

	// We should see two PatchSets with their LastCheckedIfCommentNecessary field updated.
	patchSetsWereMarkedCommentedOn := mock.MatchedBy(func(ps code_review.PatchSet) bool {
		assert.False(t, ps.CommentedOnCL)
		assert.Equal(t, indexTime, ps.LastCheckedIfCommentNecessary)
		return true
	})
	mcs.On("PutPatchSet", testutils.AnyContext, patchSetsWereMarkedCommentedOn).Return(nil).Twice()

	xcl := makeChangeLists(10)
	mcr.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(func(ctx context.Context, id string) code_review.ChangeList {
		i, err := strconv.Atoi(id)
		assert.NoError(t, err)
		return xcl[i]
	}, nil)
	mcr.On("System").Return("github")

	// Pretend all CLs queried have 2 untriaged digests.
	msa.On("UntriagedUnignoredTryJobExclusiveDigests", testutils.AnyContext, mock.Anything).Return(&frontend.UntriagedDigestList{
		Digests: nil,
		TS:      indexTime,
	}, nil)

	c := newTestCommenter(mcr, mcs, msa)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.NoError(t, err)
}

func TestCommentOnChangeListsWithUntriagedDigests_SearchAPIError_LogsErrorAndSuccess(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}
	msa := &mock_search.SearchAPI{}
	defer mcs.AssertExpectations(t)

	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(makeChangeLists(10), 10, nil).Once()
	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(nil, 10, nil).Once()
	// Mark two of the CLs (with id 0003 and 0007) as having untriaged digests in all n patchsets.
	mcs.On("GetPatchSets", testutils.AnyContext, "0003").Return(makePatchSets(4, true), nil)
	mcs.On("GetPatchSets", testutils.AnyContext, "0007").Return(makePatchSets(9, true), nil)
	mcs.On("GetPatchSets", testutils.AnyContext, mock.Anything).Return(makePatchSets(1, false), nil)

	// We should see two PatchSets with their LastCheckedIfCommentNecessary field updated.
	patchSetsWereMarkedCommentedOn := mock.MatchedBy(func(ps code_review.PatchSet) bool {
		assert.False(t, ps.CommentedOnCL)
		assert.Equal(t, fakeNow, ps.LastCheckedIfCommentNecessary)
		return true
	})
	mcs.On("PutPatchSet", testutils.AnyContext, patchSetsWereMarkedCommentedOn).Return(nil).Twice()

	xcl := makeChangeLists(10)
	mcr.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(func(ctx context.Context, id string) code_review.ChangeList {
		i, err := strconv.Atoi(id)
		assert.NoError(t, err)
		return xcl[i]
	}, nil)
	mcr.On("System").Return("github")

	// Simulate an error working with the
	msa.On("UntriagedUnignoredTryJobExclusiveDigests", testutils.AnyContext, mock.Anything).Return(nil, errors.New("boom"))

	c := newTestCommenter(mcr, mcs, msa)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.NoError(t, err)
}

// TestCommentOnCLsLogCommentsOnly tests that if we specify logCommentsOnly mode, we don't actually
// call CommentOn.
func TestCommentOnCLsLogCommentsOnly(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}
	msa := &mock_search.SearchAPI{}
	defer mcs.AssertExpectations(t)

	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(makeChangeLists(10), 10, nil).Once()
	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(nil, 10, nil).Once()
	// Mark two of the CLs (with id 0003 and 0007) as having untriaged digests in all n patchsets.
	mcs.On("GetPatchSets", testutils.AnyContext, "0003").Return(makePatchSets(4, true), nil)
	mcs.On("GetPatchSets", testutils.AnyContext, "0007").Return(makePatchSets(9, true), nil)
	mcs.On("GetPatchSets", testutils.AnyContext, mock.Anything).Return(makePatchSets(1, false), nil)

	// We should see two PatchSets with their CommentedOnCL bit set written back to Firestore.
	// Even though we are logging the comments, we want to update Firestore that we "commented".
	patchSetsWereMarkedCommentedOn := mock.MatchedBy(func(ps code_review.PatchSet) bool {
		assert.True(t, ps.CommentedOnCL)
		return true
	})
	mcs.On("PutPatchSet", testutils.AnyContext, patchSetsWereMarkedCommentedOn).Return(nil).Twice()

	xcl := makeChangeLists(10)
	mcr.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(func(ctx context.Context, id string) code_review.ChangeList {
		i, err := strconv.Atoi(id)
		assert.NoError(t, err)
		return xcl[i]
	}, nil)
	mcr.On("System").Return("github")

	// Pretend all CLs queried have 2 untriaged digests.
	msa.On("UntriagedUnignoredTryJobExclusiveDigests", testutils.AnyContext, mock.Anything).Return(&frontend.UntriagedDigestList{
		Digests: []types.Digest{"doesn't", "matter"},
	}, nil)

	c := newTestCommenter(mcr, mcs, msa)
	c.logCommentsOnly = true
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.NoError(t, err)
}

// TestCommentOnCLsOnlyCommentOnce tests the case where all open CLs have already been commented on
// and therefore we do not comment again.
func TestCommentOnCLsOnlyCommentOnce(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}

	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(makeChangeLists(10), 10, nil).Once()
	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(nil, 10, nil).Once()
	xps := makePatchSets(1, true)
	xps[0].CommentedOnCL = true
	mcs.On("GetPatchSets", testutils.AnyContext, mock.Anything).Return(xps, nil)

	xcl := makeChangeLists(10)
	mcr.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(func(ctx context.Context, id string) code_review.ChangeList {
		i, err := strconv.Atoi(id)
		assert.NoError(t, err)
		return xcl[i]
	}, nil)
	// no calls to CommentOn expected because the CL has already been commented on

	c := newTestCommenter(mcr, mcs, nil)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.NoError(t, err)
}

// TestCommentOnCLsPatchSetsRetrievalError tests the case where fetching PatchSets from clstore
// fails. The whole process should fail.
func TestCommentOnCLsPatchSetsRetrievalError(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}

	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(makeChangeLists(10), 10, nil).Once()
	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(nil, 10, nil).Once()
	mcs.On("GetPatchSets", testutils.AnyContext, mock.Anything).Return(nil, errors.New("firestore kaput"))

	xcl := makeChangeLists(10)
	mcr.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(func(ctx context.Context, id string) code_review.ChangeList {
		i, err := strconv.Atoi(id)
		assert.NoError(t, err)
		return xcl[i]
	}, nil)

	c := newTestCommenter(mcr, mcs, nil)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "firestore kaput")
}

// TestCommentOnCLsCommentError tests the case where leaving a comment fails. The whole function
// should fail then.
func TestCommentOnCLsCommentError(t *testing.T) {
	unittest.SmallTest(t)

	msa := &mock_search.SearchAPI{}
	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}
	defer mcr.AssertExpectations(t)

	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(makeChangeLists(10), 10, nil).Once()
	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(nil, 10, nil).Once()
	mcs.On("GetPatchSets", testutils.AnyContext, mock.Anything).Return(makePatchSets(1, true), nil)

	xcl := makeChangeLists(10)
	mcr.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(func(ctx context.Context, id string) code_review.ChangeList {
		i, err := strconv.Atoi(id)
		assert.NoError(t, err)
		return xcl[i]
	}, nil)
	mcr.On("CommentOn", testutils.AnyContext, mock.Anything, mock.Anything).Return(errors.New("internet down"))
	mcr.On("System").Return("gerritHub")

	// Pretend all CLs queried have 2 untriaged digests.
	msa.On("UntriagedUnignoredTryJobExclusiveDigests", testutils.AnyContext, mock.Anything).Return(&frontend.UntriagedDigestList{
		Digests: []types.Digest{"doesn't", "matter"},
	}, nil)

	c := newTestCommenter(mcr, mcs, msa)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	assertErrorWasCanceledOrContains(t, err, "internet down")
}

func makeChangeLists(n int) []code_review.ChangeList {
	var xcl []code_review.ChangeList
	for i := 0; i < n; i++ {
		xcl = append(xcl, code_review.ChangeList{
			SystemID: fmt.Sprintf("%04d", i),
			Owner:    "user@example.com",
			Status:   0,
			Subject:  "blarg",
			Updated:  oneHourAgo,
		})
	}
	return xcl
}

func makePatchSets(n int, needsComment bool) []code_review.PatchSet {
	var xps []code_review.PatchSet
	for i := 0; i < n; i++ {
		ps := code_review.PatchSet{
			SystemID:      fmt.Sprintf("%04d", i),
			ChangeListID:  "ignored",
			Order:         i + 1,
			GitHash:       "ignored",
			CommentedOnCL: false,
		}
		if needsComment {
			ps.LastCheckedIfCommentNecessary = ninetyMinutesAgo
		} else {
			ps.LastCheckedIfCommentNecessary = tenMinutesAgo
		}
		xps = append(xps, ps)
	}
	return xps
}

// assertErrorWasCanceledOrContains helps with the cases where the error that is returned is
// non-deterministic, for example, when using an errgroup. It checks that the error message matches
// a context being canceled or contains the given submessages.
func assertErrorWasCanceledOrContains(t *testing.T, err error, submessages ...string) {
	require.Error(t, err)
	e := err.Error()
	if strings.Contains(e, "canceled") {
		return
	}
	for _, m := range submessages {
		assert.Contains(t, err.Error(), m)
	}
}

func newTestCommenter(mcr *mock_codereview.Client, mcs *mock_clstore.Store, msa *mock_search.SearchAPI) *Impl {
	c := New(mcr, mcs, msa, basicTemplate, instanceURL, false)
	c.now = func() time.Time {
		return fakeNow
	}
	return c
}

const (
	instanceURL   = "gold.skia.org"
	basicTemplate = `Gold has detected about %d untriaged digest(s) on patchset %d.
Please triage them at %s/search?issue=%s.`
)

var (
	fakeNow = time.Date(2020, time.May, 1, 10, 0, 0, 0, time.UTC)

	ninetyMinutesAgo = fakeNow.Add(-time.Minute * 90)

	oneHourAgo = fakeNow.Add(-time.Hour)

	tenMinutesAgo = fakeNow.Add(-time.Minute * 10)
)
