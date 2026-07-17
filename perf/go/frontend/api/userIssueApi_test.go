package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/issuetracker"
	issuetrackerMocks "go.skia.org/infra/perf/go/issuetracker/mocks"
	"go.skia.org/infra/perf/go/userissue"
	userissueMocks "go.skia.org/infra/perf/go/userissue/mocks"
)

func TestFrontendUserIssuesHandler_Success(t *testing.T) {
	w := httptest.NewRecorder()
	req := GetUserIssuesForTraceKeysRequest{
		TraceKeys:           []string{",a=1,b=1,c=1,", ",a=1,b=1,c=1,"},
		BeginCommitPosition: 1,
		EndCommitPosition:   10,
	}
	uiBody, _ := json.Marshal(req)
	body := bytes.NewReader(uiBody)
	r := httptest.NewRequest("POST", "/_/userissues/", body)

	fakeUserIssues := []userissue.UserIssue{
		{
			UserId:         "a@b.com",
			TraceKey:       ",a=1,b=1,",
			CommitPosition: 1,
			IssueId:        12,
		},
		{
			UserId:         "b@c.com",
			TraceKey:       ",a=2,b=2,",
			CommitPosition: 7,
			IssueId:        89,
		},
	}

	uiMocks := userissueMocks.NewStore(t)
	uiMocks.On("GetUserIssuesForTraceKeys", testutils.AnyContext, mock.Anything, mock.Anything, mock.Anything).Return(fakeUserIssues, nil)

	login := mocks.NewLogin(t)
	itMocks := issuetrackerMocks.NewIssueTracker(t)

	ui := NewUserIssueApi(login, uiMocks, itMocks)

	ui.userIssuesHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
}

func TestFrontendSaveUserIssueHandler_Success(t *testing.T) {
	w := httptest.NewRecorder()
	saveReq := SaveUserIssueRequest{
		TraceKey:       ",a=1,b=1,c=1,",
		CommitPosition: 1,
		IssueId:        12345,
	}
	uiBody, _ := json.Marshal(saveReq)
	body := bytes.NewReader(uiBody)
	r := httptest.NewRequest("POST", "/_/userissue/save", body)

	uiMocks := userissueMocks.NewStore(t)
	uiMocks.On("Save", testutils.AnyContext, mock.Anything).Return(nil)

	login := mocks.NewLogin(t)
	login.On("LoggedInAs", r).Return(alogin.EMail("nobody@example.org"))
	itMocks := issuetrackerMocks.NewIssueTracker(t)

	ui := NewUserIssueApi(login, uiMocks, itMocks)

	ui.saveUserIssueHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
}

func TestFrontendDeleteIssueHandler_Success(t *testing.T) {
	w := httptest.NewRecorder()
	deleteReq := DeleteUserIssueRequest{
		TraceKey:       ",a=1,b=1,c=1,",
		CommitPosition: 1,
	}
	uiBody, _ := json.Marshal(deleteReq)
	body := bytes.NewReader(uiBody)
	r := httptest.NewRequest("POST", "/_/userissue/delete", body)

	uiMocks := userissueMocks.NewStore(t)
	uiMocks.On("Delete", testutils.AnyContext, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	login := mocks.NewLogin(t)
	login.On("LoggedInAs", r).Return(alogin.EMail("nobody@example.org"))
	itMocks := issuetrackerMocks.NewIssueTracker(t)

	ui := NewUserIssueApi(login, uiMocks, itMocks)

	ui.deleteUserIssueHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
}

func TestFrontendCreateUserIssueHandler_Success(t *testing.T) {
	w := httptest.NewRecorder()
	createReq := issuetracker.CreateUserIssueRequest{
		TraceKey:       ",a=1,b=1,c=1,",
		CommitPosition: 123,
	}
	uiBody, _ := json.Marshal(createReq)
	body := bytes.NewReader(uiBody)
	r := httptest.NewRequest("POST", "/_/user_issue/create", body)

	uiMocks := userissueMocks.NewStore(t)
	uiMocks.On("GetUserIssuesForTraceKeys", testutils.AnyContext, mock.Anything, mock.Anything, mock.Anything).Return([]userissue.UserIssue{}, nil)
	uiMocks.On("Save", testutils.AnyContext, mock.Anything).Return(nil)

	login := mocks.NewLogin(t)
	login.On("LoggedInAs", r).Return(alogin.EMail("nobody@example.org"))

	itMocks := issuetrackerMocks.NewIssueTracker(t)
	itMocks.On("FileUserIssue", testutils.AnyContext, mock.Anything).Return(12345, nil)

	ui := NewUserIssueApi(login, uiMocks, itMocks)

	ui.createUserIssueHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	var resp CreateUserIssueResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	require.Equal(t, int64(12345), resp.BugId)
}

func TestFrontendCreateUserIssueHandler_UnAuthorized(t *testing.T) {
	w := httptest.NewRecorder()
	createReq := issuetracker.CreateUserIssueRequest{
		TraceKey:       ",a=1,b=1,c=1,",
		CommitPosition: 123,
	}
	uiBody, _ := json.Marshal(createReq)
	body := bytes.NewReader(uiBody)
	r := httptest.NewRequest("POST", "/_/user_issue/create", body)

	uiMocks := userissueMocks.NewStore(t)
	login := mocks.NewLogin(t)
	login.On("LoggedInAs", r).Return(alogin.EMail(""))

	itMocks := issuetrackerMocks.NewIssueTracker(t)

	ui := NewUserIssueApi(login, uiMocks, itMocks)

	ui.createUserIssueHandler(w, r)

	require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
}

func TestFrontendCreateUserIssueHandler_Conflict(t *testing.T) {
	w := httptest.NewRecorder()
	createReq := issuetracker.CreateUserIssueRequest{
		TraceKey:       ",a=1,b=1,c=1,",
		CommitPosition: 123,
	}
	uiBody, _ := json.Marshal(createReq)
	body := bytes.NewReader(uiBody)
	r := httptest.NewRequest("POST", "/_/user_issue/create", body)

	// Mock returning an existing issue to trigger conflict
	fakeUserIssues := []userissue.UserIssue{
		{
			UserId:         "nobody@example.org",
			TraceKey:       ",a=1,b=1,c=1,",
			CommitPosition: 123,
			IssueId:        12345,
		},
	}
	uiMocks := userissueMocks.NewStore(t)
	uiMocks.On("GetUserIssuesForTraceKeys", testutils.AnyContext, mock.Anything, mock.Anything, mock.Anything).Return(fakeUserIssues, nil)

	login := mocks.NewLogin(t)
	login.On("LoggedInAs", r).Return(alogin.EMail("nobody@example.org"))

	itMocks := issuetrackerMocks.NewIssueTracker(t)

	ui := NewUserIssueApi(login, uiMocks, itMocks)

	ui.createUserIssueHandler(w, r)

	require.Equal(t, http.StatusConflict, w.Result().StatusCode)
}
