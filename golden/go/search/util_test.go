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
	"go.skia.org/infra/golden/go/types"
)

// TestJoinedHistories_GetTriageHistory_WithChangelist_Success tests the 4 cases of triage history
// existing or not existing on the changelist and master branch.
func TestJoinedHistories_GetTriageHistory_WithChangelist_Success(t *testing.T) {
	unittest.SmallTest(t)
	const crs = "github"
	const clID = "whatever"

	const grouping = types.TestName("some_test")
	const noHistoryOnMasterOrChangelist = types.Digest("digestHasNoHistory")
	const historyOnMasterOnly = types.Digest("digestHasHistoryOnMasterOnly")
	const historyOnChangelistOnly = types.Digest("digestHasHistoryOnChangelistOnly")
	const historyOnBoth = types.Digest("digestHasHistoryOnBoth")

	const masterBranchUser = "masterBranch@"
	const changelistUser = "clUser@"

	var masterBranchTriageTime = time.Date(2020, time.May, 18, 17, 16, 15, 0, time.UTC)
	var changelistTriageTime = time.Date(2020, time.May, 19, 18, 17, 16, 0, time.UTC)

	masterBranchHistory := &mock_expectations.Store{}
	changelistHistory := &mock_expectations.Store{}
	masterBranchHistory.On("ForChangelist", clID, crs).Return(changelistHistory)

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
	masterBranchHistory.On("GetTriageHistory", testutils.AnyContext, grouping, historyOnChangelistOnly).Return(nil, nil)
	masterBranchHistory.On("GetTriageHistory", testutils.AnyContext, grouping, noHistoryOnMasterOrChangelist).Return(nil, nil)

	changelistHistory.On("GetTriageHistory", testutils.AnyContext, grouping, historyOnChangelistOnly).Return([]expectations.TriageHistory{
		{
			User: changelistUser,
			TS:   changelistTriageTime,
		},
	}, nil)
	changelistHistory.On("GetTriageHistory", testutils.AnyContext, grouping, historyOnBoth).Return([]expectations.TriageHistory{
		{
			User: changelistUser,
			TS:   changelistTriageTime,
		},
	}, nil)
	changelistHistory.On("GetTriageHistory", testutils.AnyContext, grouping, historyOnMasterOnly).Return(nil, nil)
	changelistHistory.On("GetTriageHistory", testutils.AnyContext, grouping, noHistoryOnMasterOrChangelist).Return(nil, nil)

	s := SearchImpl{expectationsStore: masterBranchHistory}
	joined := s.makeTriageHistoryGetter(crs, clID)
	ctx := context.Background()

	th, err := joined.GetTriageHistory(ctx, grouping, noHistoryOnMasterOrChangelist)
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

	th, err = joined.GetTriageHistory(ctx, grouping, historyOnChangelistOnly)
	require.NoError(t, err)
	assert.Equal(t, []expectations.TriageHistory{
		{
			User: changelistUser,
			TS:   changelistTriageTime,
		},
	}, th)

	th, err = joined.GetTriageHistory(ctx, grouping, historyOnBoth)
	require.NoError(t, err)
	assert.Equal(t, []expectations.TriageHistory{
		{
			User: changelistUser,
			TS:   changelistTriageTime,
		},
		{
			User: masterBranchUser,
			TS:   masterBranchTriageTime,
		},
	}, th)
}

func TestJoinedHistories_GetTriageHistory_NoChangelist_Success(t *testing.T) {
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

func TestJoinedHistories_GetTriageHistory_ChangelistBackendErrorCausesError(t *testing.T) {
	unittest.SmallTest(t)
	const crs = "github"
	const clID = "whatever"

	masterBranchHistory := &mock_expectations.Store{}
	changelistHistory := &mock_expectations.Store{}
	masterBranchHistory.On("ForChangelist", clID, crs).Return(changelistHistory)
	changelistHistory.On("GetTriageHistory", testutils.AnyContext, mock.Anything, mock.Anything).Return(nil, errors.New("pow"))

	s := SearchImpl{expectationsStore: masterBranchHistory}
	joined := s.makeTriageHistoryGetter(crs, clID)
	ctx := context.Background()

	_, err := joined.GetTriageHistory(ctx, "whatever", "whatever")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pow")
}
