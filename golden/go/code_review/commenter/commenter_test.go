package commenter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
)

// TestUpdateNotOpenBotsSunnyDay tests a typical case where two of the known open CLs are no longer
// open and must be updated in the clstore.
func TestUpdateNotOpenBotsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}
	defer mcr.AssertExpectations(t)
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
	// This matcher checks that the two no-longer-open CLs get stored to the DB
	putClMatcher := mock.MatchedBy(func(cl code_review.ChangeList) bool {
		assert.True(t, cl.Updated.After(oldTime))
		if cl.SystemID == "0003" {
			assert.Equal(t, code_review.Abandoned, cl.Status)
			return true
		} else if cl.SystemID == "0005" {
			assert.Equal(t, code_review.Landed, cl.Status)
			return true
		}
		return false
	})
	mcs.On("PutChangeList", testutils.AnyContext, putClMatcher).Return(nil)

	c := New(mcr, mcs, instanceURL, false)
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
	defer mcr.AssertExpectations(t)
	defer mcs.AssertExpectations(t)

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

	c := New(mcr, mcs, instanceURL, false)
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
	defer mcr.AssertExpectations(t)
	defer mcs.AssertExpectations(t)

	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(makeChangeLists(5), 5, nil)
	mcr.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(code_review.ChangeList{}, code_review.ErrNotFound)

	c := New(mcr, mcs, instanceURL, false)
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
	defer mcr.AssertExpectations(t)
	defer mcs.AssertExpectations(t)

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

	c := New(mcr, mcs, instanceURL, false)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "down")
	assert.Contains(t, err.Error(), "github")
}

// TestUpdateNotOpenBotsCLStoreError tests that we bail out if writing to the clstore fails.
func TestUpdateNotOpenBotsCLStoreError(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}
	defer mcr.AssertExpectations(t)
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
		Status: code_review.Landed,
	}, nil)

	mcs.On("PutChangeList", testutils.AnyContext, mock.Anything).Return(errors.New("firestore broke"))

	c := New(mcr, mcs, instanceURL, false)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "firestore broke")
}

// TestCommentOnCLsSunnyDay tests a typical case where two of the open ChangeLists have patchsets
// that have Untriaged images, and no comment yet.
func TestCommentOnCLsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}
	defer mcr.AssertExpectations(t)
	defer mcs.AssertExpectations(t)

	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(makeChangeLists(10), 10, nil).Once()
	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(nil, 10, nil).Once()
	// Mark two of the CLs (with id 0003 and 0007) as having untriaged digests in all n patchsets.
	mcs.On("GetPatchSets", testutils.AnyContext, "0003").Return(makePatchSets(4, true), nil)
	mcs.On("GetPatchSets", testutils.AnyContext, "0007").Return(makePatchSets(9, true), nil)
	mcs.On("GetPatchSets", testutils.AnyContext, mock.Anything).Return(makePatchSets(1, false), nil)

	// We should see two PatchSets with their CommentedOnCL bit set written back to Firestore.
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
		return nil
	}, nil)

	c := New(mcr, mcs, instanceURL, false)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.NoError(t, err)
}

// TestCommentOnCLsLogCommentsOnly tests that if we specify logCommentsOnly mode, we don't actually call
// CommentOn.
func TestCommentOnCLsLogCommentsOnly(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}
	defer mcr.AssertExpectations(t)
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

	c := New(mcr, mcs, instanceURL, true)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.NoError(t, err)
}

// TestCommentOnCLsOnlyCommentOnce tests the case where all open CLs have already been commented on
// and therefore we do not comment again.
func TestCommentOnCLsOnlyCommentOnce(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}
	defer mcr.AssertExpectations(t)
	defer mcs.AssertExpectations(t)

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

	c := New(mcr, mcs, instanceURL, false)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.NoError(t, err)
}

// TestCommentOnCLsPatchSetsRetrievalError tests the case where fetching PatchSets from clstore
// fails. The whole process should fail.
func TestCommentOnCLsPatchSetsRetrievalError(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}
	defer mcr.AssertExpectations(t)
	defer mcs.AssertExpectations(t)

	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(makeChangeLists(10), 10, nil).Once()
	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return(nil, 10, nil).Once()
	mcs.On("GetPatchSets", testutils.AnyContext, mock.Anything).Return(nil, errors.New("firestore kaput"))

	xcl := makeChangeLists(10)
	mcr.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(func(ctx context.Context, id string) code_review.ChangeList {
		i, err := strconv.Atoi(id)
		assert.NoError(t, err)
		return xcl[i]
	}, nil)

	c := New(mcr, mcs, instanceURL, false)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "firestore kaput")
}

// TestCommentOnCLsCommentError tests the case where leaving a comment fails. The whole function
// should fail then.
func TestCommentOnCLsCommentError(t *testing.T) {
	unittest.SmallTest(t)

	mcr := &mock_codereview.Client{}
	mcs := &mock_clstore.Store{}
	defer mcr.AssertExpectations(t)
	defer mcs.AssertExpectations(t)

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

	c := New(mcr, mcs, instanceURL, false)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internet down")
}

func makeChangeLists(n int) []code_review.ChangeList {
	var xcl []code_review.ChangeList
	now := time.Now()
	for i := 0; i < n; i++ {
		xcl = append(xcl, code_review.ChangeList{
			SystemID: fmt.Sprintf("%04d", i),
			Owner:    "user@example.com",
			Status:   0,
			Subject:  "blarg",
			Updated:  now.Add(-time.Duration(i) * time.Second).Add(-time.Minute),
		})
	}
	return xcl
}

func makePatchSets(n int, needsComment bool) []code_review.PatchSet {
	var xps []code_review.PatchSet
	for i := 0; i < n; i++ {
		ps := code_review.PatchSet{
			SystemID:     fmt.Sprintf("%04d", i),
			ChangeListID: "ignored",
			Order:        i + 1,
			GitHash:      "ignored",
		}
		if needsComment {
			ps.HasUntriagedDigests = true
			ps.CommentedOnCL = false
		}
		xps = append(xps, ps)
	}
	return xps
}

const instanceURL = "gold.skia.org"
