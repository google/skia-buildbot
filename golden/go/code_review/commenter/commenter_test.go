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

	c := New(mcr, mcs)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "8", metrics_utils.GetRecordedMetric(t, numOpenCLsMetric, nil))
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

	xcl := makeChangeLists(5)
	mcr.On("GetChangeList", testutils.AnyContext, "0002").Return(code_review.ChangeList{}, code_review.ErrNotFound)
	mcr.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(func(ctx context.Context, id string) code_review.ChangeList {
		i, err := strconv.Atoi(id)
		assert.NoError(t, err)
		return xcl[i]
	}, nil)

	c := New(mcr, mcs)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.NoError(t, err)
	// The one CL that was not found should not be counted as an open CL.
	assert.Equal(t, "4", metrics_utils.GetRecordedMetric(t, numOpenCLsMetric, nil))
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

	c := New(mcr, mcs)
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

	c := New(mcr, mcs)
	err := c.CommentOnChangeListsWithUntriagedDigests(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "firestore broke")
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
