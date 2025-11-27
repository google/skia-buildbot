package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	perf_issuetracker "go.skia.org/infra/perf/go/issuetracker"
	"go.skia.org/infra/perf/go/issuetracker/mocks"
	regmocks "go.skia.org/infra/perf/go/regression/mocks"
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

func TestFileBug_AssociateAlertsError(t *testing.T) {
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

	resp, err := backend.FileBug(context.Background(), req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedErr.Error())
	assert.Equal(t, expectedBugID, resp.BugId)

	mockIssueTracker.AssertExpectations(t)
}

func TestTriageBackend_AssociateAlerts_Success(t *testing.T) {
	mockIssueTracker := &mocks.IssueTracker{}

	mockStore := regmocks.NewStore(t)
	backend := NewTriageBackend(mockIssueTracker, mockStore)

	req := &SkiaAssociateBugRequest{
		BugId: 12345,
		Keys:  []string{"alert1", "alert2"},
	}

	mockStore.On("SetBugID", context.Background(), req.Keys, req.BugId).Return(nil)

	_, err := backend.AssociateAlerts(context.Background(), req)
	require.NoError(t, err)
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
