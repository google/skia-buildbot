package search

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/expectations"
	mock_expectations "go.skia.org/infra/golden/go/expectations/mocks"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

func TestTraceViewFn(t *testing.T) {
	unittest.SmallTest(t)

	type testCase struct {
		name string
		// inputs
		startHash string
		endHash   string

		// outputs
		trimmedStartIndex int
		trimmedEndIndex   int
	}

	testCases := []testCase{
		{
			name:      "whole tile",
			startHash: data.FirstCommitHash,
			endHash:   data.ThirdCommitHash,

			trimmedEndIndex:   2,
			trimmedStartIndex: 0,
		},
		{
			name:      "empty means whole tile",
			startHash: "",
			endHash:   "",

			trimmedEndIndex:   2,
			trimmedStartIndex: 0,
		},
		{
			name:      "invalid means whole tile",
			startHash: "not found",
			endHash:   "not found",

			trimmedEndIndex:   2,
			trimmedStartIndex: 0,
		},
		{
			name:      "last two",
			startHash: data.SecondCommitHash,
			endHash:   data.ThirdCommitHash,

			trimmedEndIndex:   2,
			trimmedStartIndex: 1,
		},
		{
			name:      "first only",
			startHash: data.FirstCommitHash,
			endHash:   data.FirstCommitHash,

			trimmedEndIndex:   0,
			trimmedStartIndex: 0,
		},
		{
			name:      "first two",
			startHash: data.FirstCommitHash,
			endHash:   data.SecondCommitHash,

			trimmedEndIndex:   1,
			trimmedStartIndex: 0,
		},
		{
			name:      "invalid start means beginning",
			startHash: "not found",
			endHash:   data.SecondCommitHash,

			trimmedEndIndex:   1,
			trimmedStartIndex: 0,
		},
		{
			name:      "invalid end means last",
			startHash: data.SecondCommitHash,
			endHash:   "not found",

			trimmedEndIndex:   2,
			trimmedStartIndex: 1,
		},
	}

	for _, tc := range testCases {
		fn, err := getTraceViewFn(data.MakeTestCommits(), tc.startHash, tc.endHash)
		require.NoError(t, err, tc.name)
		assert.NotNil(t, fn, tc.name)
		// Run through all the traces and make sure they are properly trimmed
		for _, trace := range data.MakeTestTile().Traces {
			reducedTr := fn(trace)
			assert.Equal(t, trace.Digests[tc.trimmedStartIndex:tc.trimmedEndIndex+1], reducedTr.Digests, "test case %s with trace %v", tc.name, trace.Keys())
		}
	}
}

func TestTraceViewFnErr(t *testing.T) {
	unittest.SmallTest(t)

	// It's an error to swap the order of the hashes
	_, err := getTraceViewFn(data.MakeTestCommits(), data.ThirdCommitHash, data.SecondCommitHash)
	require.Error(t, err)
	require.Contains(t, err.Error(), "later than end")
}

// TestJoinedHistories_GetTriageHistory_WithChangeList_Success tests the 4 cases of triage history
// existing or not existing on the changelist and master branch.
func TestJoinedHistories_GetTriageHistory_WithChangeList_Success(t *testing.T) {
	unittest.SmallTest(t)
	const crs = "github"
	const clID = "whatever"

	const grouping = types.TestName("some_test")
	const noHistoryOnMasterOrChangeList = types.Digest("digestHasNoHistory")
	const historyOnMasterOnly = types.Digest("digestHasHistoryOnMasterOnly")
	const historyOnChangeListOnly = types.Digest("digestHasHistoryOnChangeListOnly")
	const historyOnBoth = types.Digest("digestHasHistoryOnBoth")

	const masterBranchUser = "masterBranch@"
	const changeListUser = "clUser@"

	var masterBranchTriageTime = time.Date(2020, time.May, 18, 17, 16, 15, 0, time.UTC)
	var changeListTriageTime = time.Date(2020, time.May, 19, 18, 17, 16, 0, time.UTC)

	masterBranchHistory := &mock_expectations.Store{}
	changeListHistory := &mock_expectations.Store{}
	masterBranchHistory.On("ForChangeList", clID, crs).Return(changeListHistory)

	masterBranchHistory.On("GetTriageHistory", testutils.AnyContext, grouping, historyOnMasterOnly).Return([]expectations.TriageHistory{
		{
			User: masterBranchUser,
			TS:   masterBranchTriageTime,
		},
	}, nil)
	masterBranchHistory.On("GetTriageHistory", testutils.AnyContext, grouping, historyOnBoth).Return([]expectations.TriageHistory{
		{
			User: masterBranchUser,
			TS:   masterBranchTriageTime,
		},
	}, nil)
	masterBranchHistory.On("GetTriageHistory", testutils.AnyContext, grouping, historyOnChangeListOnly).Return(nil, nil)
	masterBranchHistory.On("GetTriageHistory", testutils.AnyContext, grouping, noHistoryOnMasterOrChangeList).Return(nil, nil)

	changeListHistory.On("GetTriageHistory", testutils.AnyContext, grouping, historyOnChangeListOnly).Return([]expectations.TriageHistory{
		{
			User: changeListUser,
			TS:   changeListTriageTime,
		},
	}, nil)
	changeListHistory.On("GetTriageHistory", testutils.AnyContext, grouping, historyOnBoth).Return([]expectations.TriageHistory{
		{
			User: changeListUser,
			TS:   changeListTriageTime,
		},
	}, nil)
	changeListHistory.On("GetTriageHistory", testutils.AnyContext, grouping, historyOnMasterOnly).Return(nil, nil)
	changeListHistory.On("GetTriageHistory", testutils.AnyContext, grouping, noHistoryOnMasterOrChangeList).Return(nil, nil)

	s := SearchImpl{expectationsStore: masterBranchHistory}
	joined := s.makeTriageHistoryGetter(crs, clID)
	ctx := context.Background()

	th, err := joined.GetTriageHistory(ctx, grouping, noHistoryOnMasterOrChangeList)
	require.NoError(t, err)
	assert.Empty(t, th)

	th, err = joined.GetTriageHistory(ctx, grouping, historyOnMasterOnly)
	require.NoError(t, err)
	assert.Equal(t, []expectations.TriageHistory{
		{
			User: masterBranchUser,
			TS:   masterBranchTriageTime,
		},
	}, th)

	th, err = joined.GetTriageHistory(ctx, grouping, historyOnChangeListOnly)
	require.NoError(t, err)
	assert.Equal(t, []expectations.TriageHistory{
		{
			User: changeListUser,
			TS:   changeListTriageTime,
		},
	}, th)

	th, err = joined.GetTriageHistory(ctx, grouping, historyOnBoth)
	require.NoError(t, err)
	assert.Equal(t, []expectations.TriageHistory{
		{
			User: changeListUser,
			TS:   changeListTriageTime,
		},
		{
			User: masterBranchUser,
			TS:   masterBranchTriageTime,
		},
	}, th)
}

func TestJoinedHistories_GetTriageHistory_NoChangeList_Success(t *testing.T) {
	unittest.SmallTest(t)
	const grouping = types.TestName("some_test")
	const noHistoryOnMaster = types.Digest("digestHasNoHistory")
	const historyOnMaster = types.Digest("digestHasHistoryOnMaster")

	const masterBranchUser = "masterBranch@"

	var masterBranchTriageTime = time.Date(2020, time.May, 18, 17, 16, 15, 0, time.UTC)

	masterBranchHistory := &mock_expectations.Store{}

	masterBranchHistory.On("GetTriageHistory", testutils.AnyContext, grouping, historyOnMaster).Return([]expectations.TriageHistory{
		{
			User: masterBranchUser,
			TS:   masterBranchTriageTime,
		},
	}, nil)
	masterBranchHistory.On("GetTriageHistory", testutils.AnyContext, grouping, noHistoryOnMaster).Return(nil, nil)

	s := SearchImpl{expectationsStore: masterBranchHistory}
	joined := s.makeTriageHistoryGetter("", "")
	ctx := context.Background()

	th, err := joined.GetTriageHistory(ctx, grouping, noHistoryOnMaster)
	require.NoError(t, err)
	assert.Empty(t, th)

	th, err = joined.GetTriageHistory(ctx, grouping, historyOnMaster)
	require.NoError(t, err)
	assert.Equal(t, []expectations.TriageHistory{
		{
			User: masterBranchUser,
			TS:   masterBranchTriageTime,
		},
	}, th)
}

func TestJoinedHistories_GetTriageHistory_BackendErrorCausesError(t *testing.T) {
	unittest.SmallTest(t)
	masterBranchHistory := &mock_expectations.Store{}
	masterBranchHistory.On("GetTriageHistory", testutils.AnyContext, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))

	s := SearchImpl{expectationsStore: masterBranchHistory}
	joined := s.makeTriageHistoryGetter("", "")
	ctx := context.Background()

	_, err := joined.GetTriageHistory(ctx, "whatever", "whatever")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestJoinedHistories_GetTriageHistory_ChangeListBackendErrorCausesError(t *testing.T) {
	unittest.SmallTest(t)
	const crs = "github"
	const clID = "whatever"

	masterBranchHistory := &mock_expectations.Store{}
	changeListHistory := &mock_expectations.Store{}
	masterBranchHistory.On("ForChangeList", clID, crs).Return(changeListHistory)
	changeListHistory.On("GetTriageHistory", testutils.AnyContext, mock.Anything, mock.Anything).Return(nil, errors.New("pow"))

	s := SearchImpl{expectationsStore: masterBranchHistory}
	joined := s.makeTriageHistoryGetter(crs, clID)
	ctx := context.Background()

	_, err := joined.GetTriageHistory(ctx, "whatever", "whatever")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pow")
}
