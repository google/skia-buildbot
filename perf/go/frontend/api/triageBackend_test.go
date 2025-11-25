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
)

// mockTriageBackend is a TriageBackend implementation for testing that allows overriding
// the AssociateAlerts method.
type mockTriageBackend struct {
	triageBackend
	associateAlertsFn func(ctx context.Context, req *SkiaAssociateBugRequest) (*SkiaAssociateBugResponse, error)
}

// AssociateAlerts implements the TriageBackend interface, allowing a test-specific function to be called.
func (m *mockTriageBackend) AssociateAlerts(ctx context.Context, req *SkiaAssociateBugRequest) (*SkiaAssociateBugResponse, error) {
	if m.associateAlertsFn != nil {
		return m.associateAlertsFn(ctx, req)
	}
	// Fallback to the original implementation or a default test behavior if no mock is provided.
	return m.triageBackend.AssociateAlerts(ctx, req)
}

func TestFileBug_Success(t *testing.T) {
	mockIssueTracker := &mocks.IssueTracker{}
	expectedBugID := 12345

	mockIssueTracker.On("FileBug", mock.Anything, mock.AnythingOfType("*issuetracker.FileBugRequest")).Return(expectedBugID, nil)

	backend := &mockTriageBackend{
		triageBackend: triageBackend{
			issueTracker: mockIssueTracker,
		},
		associateAlertsFn: func(ctx context.Context, req *SkiaAssociateBugRequest) (*SkiaAssociateBugResponse, error) {
			assert.Equal(t, expectedBugID, req.BugId)
			assert.Equal(t, []int{1, 2, 3}, req.Keys)
			assert.Equal(t, []string{"trace1", "trace2"}, req.TraceNames)
			return &SkiaAssociateBugResponse{BugId: req.BugId}, nil
		},
	}

	req := &perf_issuetracker.FileBugRequest{
		Title:      "Test Bug",
		Keys:       []int{1, 2, 3},
		TraceNames: []string{"trace1", "trace2"},
	}

	resp, err := backend.FileBug(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, expectedBugID, resp.BugId)

	mockIssueTracker.AssertExpectations(t)
}

func TestFileBug_FileBugError(t *testing.T) {
	mockIssueTracker := &mocks.IssueTracker{}
	expectedErr := errors.New("file bug error")

	mockIssueTracker.On("FileBug", mock.Anything, mock.AnythingOfType("*issuetracker.FileBugRequest")).Return(0, expectedErr)

	backend := NewTriageBackend(mockIssueTracker)

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
	expectedBugID := 12345
	expectedErr := errors.New("unimplemented call to associate alerts")

	mockIssueTracker.On("FileBug", mock.Anything, mock.AnythingOfType("*issuetracker.FileBugRequest")).Return(expectedBugID, nil)

	backend := &mockTriageBackend{
		triageBackend: triageBackend{
			issueTracker: mockIssueTracker,
		},
		associateAlertsFn: func(ctx context.Context, req *SkiaAssociateBugRequest) (*SkiaAssociateBugResponse, error) {
			return nil, expectedErr
		},
	}

	req := &perf_issuetracker.FileBugRequest{
		Title:      "Test Bug",
		Keys:       []int{1, 2, 3},
		TraceNames: []string{"trace1", "trace2"},
	}

	resp, err := backend.FileBug(context.Background(), req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedErr.Error())
	assert.Equal(t, expectedBugID, resp.BugId)

	mockIssueTracker.AssertExpectations(t)
}
