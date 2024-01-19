package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/mocks"
	"go.skia.org/infra/go/gerrit"
	gerritMock "go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/git"
	gitilesMock "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/kube/go/authproxy"
	"go.skia.org/infra/tool/go/tool"
)

const (
	serializedTools      = "[]"
	emailForTest         = alogin.EMail("someone@example.org")
	testID               = "some-tool-id-for-test"
	gitHashForTest       = "0123456789"
	gerritIssueIDForTest = int64(123456)
	urlForNewCl          = "https://some-repo.example.com/issueid"
)

var (
	errForTesting = errors.New("my test error")
)

func setupForTest(t *testing.T) (context.Context, *server, chi.Router, *httptest.ResponseRecorder) {
	ctx := context.Background()

	gitilesMock := gitilesMock.NewGitilesRepo(t)
	gerritMock := gerritMock.NewGerritInterface(t)
	loginMock := mocks.NewLogin(t)

	s := &server{
		alogin:        loginMock,
		gitilesRepo:   gitilesMock,
		gerritRepo:    gerritMock,
		gerritProject: "k8s-config",
		tools:         []byte(serializedTools),
	}

	// Put a chi.Router in place so the request path gets parsed.
	router := chi.NewRouter()
	s.AddHandlers(router)
	w := httptest.NewRecorder()

	return ctx, s, router, w
}

func newAuthorizedRequest(method, target string, body io.Reader) *http.Request {
	ret := httptest.NewRequest(method, target, body)
	ret.Header.Add(authproxy.WebAuthRoleHeaderName, string(roles.Editor))
	ret.Header.Add(authproxy.WebAuthHeaderName, string(emailForTest))
	return ret
}

func TestConfigHandler_ReturnsSerializedTools(t *testing.T) {
	_, _, router, w := setupForTest(t)

	r := newAuthorizedRequest("GET", "/_/configs", nil)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, serializedTools, w.Body.String())
}

func TestCreateOrUpdateHandler_InvalidJSON_ReturnsError(t *testing.T) {
	_, _, router, w := setupForTest(t)

	b := bytes.NewBufferString("?? not valid json")
	r := newAuthorizedRequest("PUT", "/_/put", b)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, "Failed decoding JSON\n", w.Body.String())
}

func toolBody(t *testing.T) []byte {
	tt := tool.Tool{
		ID:          testID,
		Domain:      tool.Build,
		DisplayName: "Some New Tool",
	}
	b, err := json.Marshal(tt)
	require.NoError(t, err)
	return b
}

func readerWithToolBody(t *testing.T) io.Reader {
	return bytes.NewBuffer(toolBody(t))
}

func TestCreateOrUpdateHandler_FailedToFindBaseCommit_ReturnsError(t *testing.T) {
	_, s, router, w := setupForTest(t)

	s.gitilesRepo.(*gitilesMock.GitilesRepo).On("ResolveRef", testutils.AnyContext, git.MainBranch).Return("", errForTesting)

	r := newAuthorizedRequest("PUT", "/_/put", readerWithToolBody(t))

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, "Failed to find base commit.\n", w.Body.String())
}

func TestCreateOrUpdateHandler_FailedToCreateAndEditChange_ReturnsError(t *testing.T) {
	_, s, router, w := setupForTest(t)

	s.gitilesRepo.(*gitilesMock.GitilesRepo).On("ResolveRef", testutils.AnyContext, git.MainBranch).Return(gitHashForTest, nil)

	s.alogin.(*mocks.Login).On("LoggedInAs", mock.Anything).Return(emailForTest)

	s.gerritRepo.(*gerritMock.GerritInterface).On("CreateChange", testutils.AnyContext, s.gerritProject, git.MainBranch, commitMessage, gitHashForTest, "").Return(nil, errForTesting)

	r := newAuthorizedRequest("PUT", "/_/put", readerWithToolBody(t))

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, "Failed creating CL.\n", w.Body.String())
}

func TestCreateOrUpdateHandler_Success(t *testing.T) {
	_, s, router, w := setupForTest(t)

	ci := &gerrit.ChangeInfo{
		Owner: &gerrit.Person{
			Email: emailForTest.String(),
		},

		// These values are just here to make gerrit.CreateAndEditChange happy.
		Revisions: map[string]*gerrit.Revision{
			"0": nil,
			"1": nil,
		},
		Issue: gerritIssueIDForTest,
	}

	s.gitilesRepo.(*gitilesMock.GitilesRepo).On("ResolveRef", testutils.AnyContext, git.MainBranch).Return(gitHashForTest, nil)

	s.alogin.(*mocks.Login).On("LoggedInAs", mock.Anything).Return(emailForTest)

	gi := s.gerritRepo.(*gerritMock.GerritInterface)

	// These mocks are all just here to make gerrit.CreateAndEditChange
	// happy.
	gi.On("CreateChange", testutils.AnyContext, s.gerritProject, git.MainBranch, commitMessage, gitHashForTest, "").Return(ci, nil)

	gi.On("EditFile", testutils.AnyContext, ci, "configs/"+testID+".json", string(toolBody(t))).Return(nil)

	gi.On("PublishChangeEdit", testutils.AnyContext, mock.Anything).Return(nil)

	gi.On("GetIssueProperties", testutils.AnyContext, mock.Anything).Return(ci, nil)

	gi.On("Url", gerritIssueIDForTest).Return(urlForNewCl)

	r := newAuthorizedRequest("PUT", "/_/put", readerWithToolBody(t))

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "{\"url\":\"https://some-repo.example.com/issueid\"}\n", w.Body.String())
}
