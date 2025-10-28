package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mocks "go.skia.org/infra/perf/go/chromeperf/mock"
)

func TestChromeperfFileBug_Success(t *testing.T) {
	mockClient := &mocks.ChromePerfClient{}
	backend := NewChromeperfTriageBackend(mockClient)

	req := &FileBugRequest{
		Title: "Test Bug",
	}

	mockClient.On("SendPostRequest",
		mock.Anything,
		"file_bug_skia",
		"",
		req,
		mock.AnythingOfType("*api.ChromeperfFileBugResponse"),
		[]int{200, 400, 401, 500},
	).Run(func(args mock.Arguments) {
		resp := args.Get(4).(*ChromeperfFileBugResponse)
		resp.BugId = 12345
	}).Return(nil)

	resp, err := backend.FileBug(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, 12345, resp.BugId)

	mockClient.AssertExpectations(t)
}

func TestChromeperfFileBug_ApiError(t *testing.T) {
	mockClient := &mocks.ChromePerfClient{}
	backend := NewChromeperfTriageBackend(mockClient)

	req := &FileBugRequest{
		Title: "Test Bug",
	}

	mockClient.On("SendPostRequest",
		mock.Anything,
		"file_bug_skia",
		"",
		req,
		mock.AnythingOfType("*api.ChromeperfFileBugResponse"),
		[]int{200, 400, 401, 500},
	).Run(func(args mock.Arguments) {
		resp := args.Get(4).(*ChromeperfFileBugResponse)
		resp.Error = "api error"
	}).Return(nil)

	_, err := backend.FileBug(context.Background(), req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")

	mockClient.AssertExpectations(t)
}

func TestChromeperfFileBug_RequestError(t *testing.T) {
	mockClient := &mocks.ChromePerfClient{}
	backend := NewChromeperfTriageBackend(mockClient)

	req := &FileBugRequest{
		Title: "Test Bug",
	}

	mockClient.On("SendPostRequest",
		mock.Anything,
		"file_bug_skia",
		"",
		req,
		mock.AnythingOfType("*api.ChromeperfFileBugResponse"),
		[]int{200, 400, 401, 500},
	).Return(errors.New("request error"))

	_, err := backend.FileBug(context.Background(), req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "request error")

	mockClient.AssertExpectations(t)
}

func TestChromeperfEditAnomalies_Success(t *testing.T) {
	mockClient := &mocks.ChromePerfClient{}
	backend := NewChromeperfTriageBackend(mockClient)

	req := &EditAnomaliesRequest{
		Action: "ignore",
	}

	mockClient.On("SendPostRequest",
		mock.Anything,
		"edit_anomalies_skia",
		"",
		req,
		mock.AnythingOfType("*api.EditAnomaliesResponse"),
		[]int{200, 400, 401, 500},
	).Return(nil)

	resp, err := backend.EditAnomalies(context.Background(), req)

	require.NoError(t, err)
	assert.Empty(t, resp.Error)

	mockClient.AssertExpectations(t)
}

func TestChromeperfEditAnomalies_ApiError(t *testing.T) {
	mockClient := &mocks.ChromePerfClient{}
	backend := NewChromeperfTriageBackend(mockClient)

	req := &EditAnomaliesRequest{
		Action: "ignore",
	}

	mockClient.On("SendPostRequest",
		mock.Anything,
		"edit_anomalies_skia",
		"",
		req,
		mock.AnythingOfType("*api.EditAnomaliesResponse"),
		[]int{200, 400, 401, 500},
	).Run(func(args mock.Arguments) {
		resp := args.Get(4).(*EditAnomaliesResponse)
		resp.Error = "api error"
	}).Return(nil)

	_, err := backend.EditAnomalies(context.Background(), req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")

	mockClient.AssertExpectations(t)
}

func TestChromeperfAssociateAlerts_Success(t *testing.T) {
	mockClient := &mocks.ChromePerfClient{}
	backend := NewChromeperfTriageBackend(mockClient)

	req := &SkiaAssociateBugRequest{
		BugId: 12345,
	}

	mockClient.On("SendPostRequest",
		mock.Anything,
		"associate_alerts_skia",
		"",
		req,
		mock.AnythingOfType("*api.ChromeperfAssociateBugResponse"),
		[]int{200, 400, 401, 500},
	).Return(nil)

	resp, err := backend.AssociateAlerts(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, 12345, resp.BugId)

	mockClient.AssertExpectations(t)
}
