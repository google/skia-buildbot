package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin"
	aloginMocks "go.skia.org/infra/go/alogin/mocks"
	"go.skia.org/infra/go/issuetracker/v1"
	"go.skia.org/infra/go/testutils"
	perf_issuetracker "go.skia.org/infra/perf/go/issuetracker"
	issueTrackerMock "go.skia.org/infra/perf/go/issuetracker/mocks"
)

// MockTriageBackend is a mock implementation of the TriageBackend interface.
type MockTriageBackend struct {
	mock.Mock
}

func (m *MockTriageBackend) FileBug(ctx context.Context, req *perf_issuetracker.FileBugRequest) (*SkiaFileBugResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*SkiaFileBugResponse), args.Error(1)
}

func (m *MockTriageBackend) EditAnomalies(ctx context.Context, req *EditAnomaliesRequest) (*EditAnomaliesResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*EditAnomaliesResponse), args.Error(1)
}

func (m *MockTriageBackend) AssociateAlerts(ctx context.Context, req *SkiaAssociateBugRequest) (*SkiaAssociateBugResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*SkiaAssociateBugResponse), args.Error(1)
}

func newTestTriageApi(login alogin.Login, backend TriageBackend) triageApi {
	return NewTriageApi(login, backend, nil)
}

func TestFileNewBug_NotLoggedIn(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail(""))

	api := NewTriageApi(login, nil, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/file_bug", nil)

	api.FileNewBug(w, r)

	require.Equal(http.StatusUnauthorized, w.Code)
	require.Contains(w.Body.String(), "You must be logged in to complete this action.")
	login.AssertExpectations(t)
}

func TestFileNewBug_InvalidJson(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail("testuser@example.com"))

	api := NewTriageApi(login, nil, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/file_bug", bytes.NewBufferString("invalid json"))

	api.FileNewBug(w, r)

	require.Equal(http.StatusInternalServerError, w.Code)
	require.Contains(w.Body.String(), "Failed to decode JSON on new bug request.")
	login.AssertExpectations(t)
}

func TestFileNewBug_BackendError(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail("testuser@example.com"))

	mockBackend := &MockTriageBackend{}
	mockBackend.On("FileBug", testutils.AnyContext, mock.AnythingOfType("*issuetracker.FileBugRequest")).Return(&SkiaFileBugResponse{}, errors.New("backend error"))

	api := NewTriageApi(login, mockBackend, nil)

	fileBugRequest := perf_issuetracker.FileBugRequest{
		Title:       "Test Bug",
		Description: "This is a test bug.",
		Component:   "Test>Component",
		Keys:        []string{"1", "2", "3"},
	}
	body, _ := json.Marshal(fileBugRequest)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/file_bug", bytes.NewBuffer(body))

	api.FileNewBug(w, r)

	require.Equal(http.StatusInternalServerError, w.Code)
	require.Contains(w.Body.String(), "File new bug request failed due to an internal server error. Please try again.")
	login.AssertExpectations(t)
	mockBackend.AssertExpectations(t)
}

func TestFileNewBug_Success(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail("testuser@example.com"))

	mockBackend := &MockTriageBackend{}
	expectedBugID := 12345
	mockBackend.On("FileBug", testutils.AnyContext, mock.AnythingOfType("*issuetracker.FileBugRequest")).Return(&SkiaFileBugResponse{BugId: expectedBugID}, nil)

	api := NewTriageApi(login, mockBackend, nil)

	fileBugRequest := perf_issuetracker.FileBugRequest{
		Title:       "Test Bug",
		Description: "This is a test bug.",
		Component:   "Test>Component",
		Keys:        []string{"1", "2", "3"},
	}
	body, _ := json.Marshal(fileBugRequest)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/file_bug", bytes.NewBuffer(body))

	api.FileNewBug(w, r)

	require.Equal(http.StatusOK, w.Code)
	var resp SkiaFileBugResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(err)
	assert.Equal(expectedBugID, resp.BugId)

	login.AssertExpectations(t)
	mockBackend.AssertExpectations(t)
}

func TestEditAnomalies_NotLoggedIn(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail(""))

	api := newTestTriageApi(login, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/edit_anomalies", nil)

	api.EditAnomalies(w, r)

	require.Equal(http.StatusUnauthorized, w.Code)
	require.Contains(w.Body.String(), "You must be logged in to complete this action.")
	login.AssertExpectations(t)
}

func TestEditAnomalies_InvalidJson(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail("testuser@example.com"))

	api := newTestTriageApi(login, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/edit_anomalies", bytes.NewBufferString("invalid json"))

	api.EditAnomalies(w, r)

	require.Equal(http.StatusInternalServerError, w.Code)
	require.Contains(w.Body.String(), "Failed to decode JSON on edit anomalies request.")
	login.AssertExpectations(t)
}

func TestEditAnomalies_InvalidRequest(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail("testuser@example.com"))

	api := newTestTriageApi(login, nil)

	testCases := []struct {
		name    string
		request EditAnomaliesRequest
		message string
	}{
		{
			name:    "Negative start revision",
			request: EditAnomaliesRequest{StartRevision: -1},
			message: "Invalid start or end revision.",
		},
		{
			name:    "End revision less than start revision",
			request: EditAnomaliesRequest{StartRevision: 10, EndRevision: 5},
			message: "End revision cannot be less than start revision.",
		},
		{
			name:    "Empty action",
			request: EditAnomaliesRequest{Action: ""},
			message: "Action must be a nonempty string.",
		},
		{
			name:    "Missing anomaly keys",
			request: EditAnomaliesRequest{Action: "ignore", Keys: []string{}},
			message: "Missing anomaly keys.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.request)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/_/triage/edit_anomalies", bytes.NewBuffer(body))

			api.EditAnomalies(w, r)

			require.Equal(http.StatusBadRequest, w.Code)
			require.Contains(w.Body.String(), tc.message)
		})
	}
	login.AssertExpectations(t)
}

func TestEditAnomalies_BackendError(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail("testuser@example.com"))

	mockBackend := &MockTriageBackend{}
	mockBackend.On("EditAnomalies", testutils.AnyContext, mock.AnythingOfType("*api.EditAnomaliesRequest")).Return(&EditAnomaliesResponse{}, errors.New("backend error"))

	api := newTestTriageApi(login, mockBackend)

	editAnomaliesRequest := EditAnomaliesRequest{
		Keys:   []string{"1", "2", "3"},
		Action: "ignore",
	}
	body, _ := json.Marshal(editAnomaliesRequest)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/edit_anomalies", bytes.NewBuffer(body))

	api.EditAnomalies(w, r)

	require.Equal(http.StatusInternalServerError, w.Code)
	require.Contains(w.Body.String(), "Chromeperf edit anomalies request failed.")
	login.AssertExpectations(t)
	mockBackend.AssertExpectations(t)
}

func TestEditAnomalies_Success(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail("testuser@example.com"))

	mockBackend := &MockTriageBackend{}
	mockBackend.On("EditAnomalies", testutils.AnyContext, mock.AnythingOfType("*api.EditAnomaliesRequest")).Return(&EditAnomaliesResponse{}, nil)

	api := newTestTriageApi(login, mockBackend)

	editAnomaliesRequest := EditAnomaliesRequest{
		Keys:   []string{"1", "2", "3"},
		Action: "ignore",
	}
	body, _ := json.Marshal(editAnomaliesRequest)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/edit_anomalies", bytes.NewBuffer(body))

	api.EditAnomalies(w, r)

	require.Equal(http.StatusOK, w.Code)
	login.AssertExpectations(t)
	mockBackend.AssertExpectations(t)
}

func TestAssociateAlerts_NotLoggedIn(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail(""))

	api := newTestTriageApi(login, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/associate_alerts", nil)

	api.AssociateAlerts(w, r)

	require.Equal(http.StatusUnauthorized, w.Code)
	require.Contains(w.Body.String(), "You must be logged in to complete this action.")
	login.AssertExpectations(t)
}

func TestAssociateAlerts_InvalidJson(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail("testuser@example.com"))

	api := newTestTriageApi(login, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/associate_alerts", bytes.NewBufferString("invalid json"))

	api.AssociateAlerts(w, r)

	require.Equal(http.StatusInternalServerError, w.Code)
	require.Contains(w.Body.String(), "Failed to decode JSON on associate bug request.")
	login.AssertExpectations(t)
}

func TestAssociateAlerts_BackendError(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail("testuser@example.com"))

	mockBackend := &MockTriageBackend{}
	mockBackend.On("AssociateAlerts", testutils.AnyContext, mock.AnythingOfType("*api.SkiaAssociateBugRequest")).Return(&SkiaAssociateBugResponse{}, errors.New("backend error"))

	api := newTestTriageApi(login, mockBackend)

	associateBugRequest := SkiaAssociateBugRequest{
		BugId: 12345,
		Keys:  []string{"1", "2", "3"},
	}
	body, _ := json.Marshal(associateBugRequest)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/associate_alerts", bytes.NewBuffer(body))

	api.AssociateAlerts(w, r)

	require.Equal(http.StatusInternalServerError, w.Code)
	require.Contains(w.Body.String(), "Chromeperf associate request failed.")
	login.AssertExpectations(t)
	mockBackend.AssertExpectations(t)
}

func TestAssociateAlerts_Success(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail("testuser@example.com"))

	mockBackend := &MockTriageBackend{}
	expectedBugID := 12345
	mockBackend.On("AssociateAlerts", testutils.AnyContext, mock.AnythingOfType("*api.SkiaAssociateBugRequest")).Return(&SkiaAssociateBugResponse{BugId: expectedBugID}, nil)

	api := newTestTriageApi(login, mockBackend)

	associateBugRequest := SkiaAssociateBugRequest{
		BugId: expectedBugID,
		Keys:  []string{"1", "2", "3"},
	}
	body, _ := json.Marshal(associateBugRequest)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/associate_alerts", bytes.NewBuffer(body))

	api.AssociateAlerts(w, r)

	require.Equal(http.StatusOK, w.Code)
	var resp SkiaAssociateBugResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(err)
	assert.Equal(expectedBugID, resp.BugId)

	login.AssertExpectations(t)
	mockBackend.AssertExpectations(t)
}

func TestListIssues_NotLoggedIn(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail(""))

	api := NewTriageApi(login, nil, &issueTrackerMock.IssueTracker{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/list_issues", nil)

	api.ListIssues(w, r)

	require.Equal(http.StatusUnauthorized, w.Code)
	require.Contains(w.Body.String(), "You must be logged in to complete this action.")
	login.AssertExpectations(t)
}

func TestListIssues_IssueTrackerNotAvailable(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail("testuser@example.com"))

	api := NewTriageApi(login, nil, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/list_issues", nil)

	api.ListIssues(w, r)

	require.Equal(http.StatusForbidden, w.Code)
	require.Contains(w.Body.String(), "IssueTracker client is not available on this instance.")
	login.AssertExpectations(t)
}

func TestListIssues_InvalidJson(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail("testuser@example.com"))

	api := NewTriageApi(login, nil, &issueTrackerMock.IssueTracker{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/list_issues", bytes.NewBufferString("invalid json"))

	api.ListIssues(w, r)

	require.Equal(http.StatusInternalServerError, w.Code)
	require.Contains(w.Body.String(), "Failed to decode JSON on bug title request.")
	login.AssertExpectations(t)
}

func TestListIssues_BackendError(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail("testuser@example.com"))

	mockIssueTracker := &issueTrackerMock.IssueTracker{}
	mockIssueTracker.On("ListIssues", testutils.AnyContext, mock.AnythingOfType("issuetracker.ListIssuesRequest")).Return(nil, errors.New("backend error"))

	api := NewTriageApi(login, nil, mockIssueTracker)

	listIssuesRequest := perf_issuetracker.ListIssuesRequest{
		IssueIds: []int{12345},
	}
	body, _ := json.Marshal(listIssuesRequest)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/list_issues", bytes.NewBuffer(body))

	api.ListIssues(w, r)

	require.Equal(http.StatusInternalServerError, w.Code)
	require.Contains(w.Body.String(), "ListIssues request failed due to an internal server error. Please try again.")
	login.AssertExpectations(t)
	mockIssueTracker.AssertExpectations(t)
}

func TestListIssues_Success(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	login := &aloginMocks.Login{}
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail("testuser@example.com"))

	expectedIssues := []*issuetracker.Issue{
		{
			IssueId: 12345,
			IssueState: &issuetracker.IssueState{
				Title: "Test Issue",
			},
		},
	}

	mockIssueTracker := &issueTrackerMock.IssueTracker{}
	mockIssueTracker.On("ListIssues", testutils.AnyContext, mock.AnythingOfType("issuetracker.ListIssuesRequest")).Return(expectedIssues, nil)

	api := NewTriageApi(login, nil, mockIssueTracker)

	listIssuesRequest := perf_issuetracker.ListIssuesRequest{
		IssueIds: []int{12345},
	}
	body, _ := json.Marshal(listIssuesRequest)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/triage/list_issues", bytes.NewBuffer(body))

	api.ListIssues(w, r)

	require.Equal(http.StatusOK, w.Code)
	var resp ListIssuesResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(err)
	assert.Equal(expectedIssues, resp.Issues)

	login.AssertExpectations(t)
	mockIssueTracker.AssertExpectations(t)
}
