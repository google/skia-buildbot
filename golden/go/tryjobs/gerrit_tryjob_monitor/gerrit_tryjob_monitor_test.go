package gerrit_tryjob_monitor

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	mock_eventbus "go.skia.org/infra/go/eventbus/mocks"
	"go.skia.org/infra/go/gerrit"
	gerrit_mocks "go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/tryjobstore"
)

// TestWriteCommentSunnyDay tests the sunny day case that the
// issue exists on Gerrit and in the tryjobstore
func TestWriteCommentSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	isAuthoritative := true
	siteURL := "https://gold.skia.org"

	expectedComment := `Gold results for tryjobs are being ingested.
See image differences at: https://gold.skia.org/search?issue=12345`

	meb := mockEventBus()
	mg := mockGerrit()
	mtjs := &mocks.TryjobStore{}
	defer meb.AssertExpectations(t)
	defer mg.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)

	changeInfo := makeTestChangeInfo()
	mg.On("GetIssueProperties", mockIssueID).Return(changeInfo, nil)
	mg.On("AddComment", changeInfo, expectedComment).Return(nil)

	storeIssue := makeTestIssue()
	mtjs.On("GetIssue", mockIssueID, false).Return(storeIssue, nil)
	mtjs.On("UpdateIssue", storeIssue, mock.Anything).Run(func(args mock.Arguments) {
		// Execute the callback and assert that it updates the entry
		callback, ok := args.Get(1).(tryjobstore.NewValueFn)
		assert.True(t, ok, "Wrong callback function")
		assert.False(t, storeIssue.CommentAdded)
		_ = callback(storeIssue)
		assert.True(t, storeIssue.CommentAdded)
	}).Return(nil)

	tryjobMonitor := New(mtjs, nil, nil, mg, siteURL, meb, isAuthoritative)

	assert.NoError(t, tryjobMonitor.WriteGoldLinkAsComment(mockIssueID))
}

// TestWriteCommentNoIssue tests that if the issue doesn't exist,
// we fail gracefully
func TestWriteCommentNoIssue(t *testing.T) {
	unittest.SmallTest(t)

	isAuthoritative := true
	siteURL := "https://gold.skia.org"

	meb := mockEventBus()
	mg := mockGerrit()
	mtjs := &mocks.TryjobStore{}
	defer meb.AssertExpectations(t)
	defer mg.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)

	mtjs.On("GetIssue", mockIssueID, false).Return(nil, nil)

	tryjobMonitor := New(mtjs, nil, nil, mg, siteURL, meb, isAuthoritative)

	err := tryjobMonitor.WriteGoldLinkAsComment(mockIssueID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func mockGerrit() *gerrit_mocks.GerritInterface {
	return &gerrit_mocks.GerritInterface{}
}

func mockEventBus() *mock_eventbus.EventBus {
	meb := &mock_eventbus.EventBus{}
	meb.On("SubscribeAsync", tryjobstore.EV_TRYJOB_UPDATED, mock.Anything)
	return meb
}

const mockIssueID = int64(12345)

func makeTestIssue() *tryjobstore.Issue {
	return &tryjobstore.Issue{
		ID:      mockIssueID,
		Subject: "Test issue",
		Owner:   "jdoe@example.com",
		// arbitrary time
		Updated: time.Date(2019, time.May, 11, 15, 0, 3, 7, time.UTC),
		Status:  "",
	}
}

func makeTestChangeInfo() *gerrit.ChangeInfo {
	return &gerrit.ChangeInfo{
		Id:      strconv.FormatInt(mockIssueID, 10),
		Subject: "Test issue",
		Created: time.Date(2019, time.May, 10, 14, 0, 2, 6, time.UTC),
		Updated: time.Date(2019, time.May, 11, 15, 0, 3, 7, time.UTC),
		// None of this is used by tryjob_monitor
		// It's just passed back to the Gerrit API.
	}
}
