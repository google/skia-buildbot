package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	issuetracker "go.skia.org/infra/go/issuetracker/v1"
	perf_issuetracker "go.skia.org/infra/perf/go/issuetracker"
	"go.skia.org/infra/perf/go/issuetracker/mocks"
	regmocks "go.skia.org/infra/perf/go/regression/mocks"
	"go.skia.org/infra/perf/go/types"
)

func TestFileBug_Success(t *testing.T) {
	mockIssueTracker := &mocks.IssueTracker{}
	mockRegStore := regmocks.NewStore(t)
	expectedBugID := 12345

	mockIssueTracker.On("FileBug", mock.Anything, mock.AnythingOfType("*issuetracker.FileBugRequest")).Return(expectedBugID, nil)

	backend := NewTriageBackend(mockIssueTracker, mockRegStore)

	req := &perf_issuetracker.FileBugRequest{
		Title:      "Test Bug",
		Keys:       []string{"1", "2", "3"},
		TraceNames: []string{"trace1", "trace2"},
	}
	mockRegStore.On("SetBugID", context.Background(), []string{"1", "2", "3"}, expectedBugID).Return(nil)

	resp, err := backend.FileBug(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, expectedBugID, resp.BugId)

	mockIssueTracker.AssertExpectations(t)
	mockRegStore.AssertExpectations(t)
}

func TestFileBug_FileBugError(t *testing.T) {
	mockIssueTracker := &mocks.IssueTracker{}
	mockStore := regmocks.NewStore(t)

	expectedErr := errors.New("file bug error")

	mockIssueTracker.On("FileBug", mock.Anything, mock.AnythingOfType("*issuetracker.FileBugRequest")).Return(0, expectedErr)

	backend := NewTriageBackend(mockIssueTracker, mockStore)

	req := &perf_issuetracker.FileBugRequest{
		Title: "Test Bug",
	}

	_, err := backend.FileBug(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)

	mockIssueTracker.AssertExpectations(t)
}

func TestFileBug_DBError(t *testing.T) {
	mockIssueTracker := &mocks.IssueTracker{}
	mockRegStore := regmocks.NewStore(t)
	expectedBugID := 12345
	expectedErr := errors.New("regression store error")

	mockIssueTracker.On("FileBug", mock.Anything, mock.AnythingOfType("*issuetracker.FileBugRequest")).Return(expectedBugID, nil)
	mockRegStore.On("SetBugID", context.Background(), []string{"1", "2", "3"}, expectedBugID).Return(expectedErr)

	backend := NewTriageBackend(mockIssueTracker, mockRegStore)

	req := &perf_issuetracker.FileBugRequest{
		Title:      "Test Bug",
		Keys:       []string{"1", "2", "3"},
		TraceNames: []string{"trace1", "trace2"},
	}

	_, err := backend.FileBug(context.Background(), req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedErr.Error())

	mockIssueTracker.AssertExpectations(t)
}

func TestTriageBackend_AssociateAlerts_Success(t *testing.T) {
	mockIssueTracker := &mocks.IssueTracker{}

	mockStore := regmocks.NewStore(t)
	backend := NewTriageBackend(mockIssueTracker, mockStore)
	bugId := 12345

	req := &SkiaAssociateBugRequest{
		BugId: bugId,
		Keys:  []string{"alert1", "alert2"},
	}

	// We need to mock the call to ListIssues since it is called inside AssociateAlerts.
	mockIssueTracker.On("ListIssues", mock.Anything, perf_issuetracker.ListIssuesRequest{
		IssueIds: []int{bugId}}).Return([]*issuetracker.Issue{{IssueId: int64(bugId)}}, nil)
	mockStore.On("SetBugID", context.Background(), req.Keys, req.BugId).Return(nil)

	res, err := backend.AssociateAlerts(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, bugId, res.BugId)
	mockStore.AssertExpectations(t)
}

func TestTriageBackend_AssociateAlerts_MissingBugId(t *testing.T) {
	mockIssueTracker := &mocks.IssueTracker{}
	mockStore := regmocks.NewStore(t)
	backend := NewTriageBackend(mockIssueTracker, mockStore)

	req := &SkiaAssociateBugRequest{
		Keys: []string{"alert1", "alert2"},
	}

	_, err := backend.AssociateAlerts(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BugId must be a positive integer")
	mockStore.AssertExpectations(t)
}

func TestTriageBackend_AssociateAlerts_MissingKeys(t *testing.T) {
	mockIssueTracker := &mocks.IssueTracker{}
	mockStore := regmocks.NewStore(t)

	backend := NewTriageBackend(mockIssueTracker, mockStore)

	req := &SkiaAssociateBugRequest{
		BugId: 12345,
	}

	_, err := backend.AssociateAlerts(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Keys are required")
	mockStore.AssertExpectations(t)
}

func TestEditAnomalies_Ignore(t *testing.T) {
	mockIssueTracker := &mocks.IssueTracker{}
	mockStore := regmocks.NewStore(t)
	backend := NewTriageBackend(mockIssueTracker, mockStore)

	req := &EditAnomaliesRequest{
		Keys:   []string{"a1", "a2"},
		Action: "IGNORE",
	}

	mockStore.On("IgnoreAnomalies", context.Background(), req.Keys).Return(nil)

	_, err := backend.EditAnomalies(context.Background(), req)
	require.NoError(t, err)
	mockStore.AssertExpectations(t)
}

func TestEditAnomalies_Reset(t *testing.T) {
	mockIssueTracker := &mocks.IssueTracker{}
	mockStore := regmocks.NewStore(t)
	backend := NewTriageBackend(mockIssueTracker, mockStore)

	req := &EditAnomaliesRequest{
		Keys:   []string{"a1", "a2"},
		Action: "RESET",
	}

	mockStore.On("ResetAnomalies", context.Background(), req.Keys).Return(nil)

	_, err := backend.EditAnomalies(context.Background(), req)
	require.NoError(t, err)
	mockStore.AssertExpectations(t)
}

func TestEditAnomalies_Nudge(t *testing.T) {
	mockIssueTracker := &mocks.IssueTracker{}
	mockStore := regmocks.NewStore(t)
	backend := NewTriageBackend(mockIssueTracker, mockStore)

	req := &EditAnomaliesRequest{
		Keys:          []string{"a1", "a2"},
		Action:        "NUDGE",
		StartRevision: 100,
		EndRevision:   200,
	}

	mockStore.On("NudgeAndResetAnomalies", mock.Anything, []string{"a1", "a2"}, types.CommitNumber(200), types.CommitNumber(100)).Return(nil)

	_, err := backend.EditAnomalies(context.Background(), req)
	require.NoError(t, err)
	mockStore.AssertExpectations(t)
}

func TestTriageBackend_AssociateAlerts_EmptyIssueList(t *testing.T) {
	mockIssueTracker := &mocks.IssueTracker{}
	mockRegStore := regmocks.NewStore(t)

	backend := NewTriageBackend(mockIssueTracker, mockRegStore)

	req := &SkiaAssociateBugRequest{
		BugId: 12345,
		Keys:  []string{"alert1", "alert2"},
	}

	// Configure mockIssueTracker.ListIssues to return an empty slice of issues.
	mockIssueTracker.On("ListIssues", mock.Anything, perf_issuetracker.ListIssuesRequest{IssueIds: []int{req.BugId}}).Return(nil, nil)

	// Configure mockRegStore.SetBugID to *not* be called.
	mockRegStore.AssertNotCalled(t, "SetBugID", mock.Anything, mock.Anything, mock.Anything)

	_, err := backend.AssociateAlerts(context.Background(), req)

	// Assert that an error is returned and its content matches the expected error message.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Issue with bug_id = 12345 does not exist.")

	mockIssueTracker.AssertExpectations(t)
	mockRegStore.AssertExpectations(t)
}
