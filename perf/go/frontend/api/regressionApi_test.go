package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/mocks"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/regression"
	regressionMocks "go.skia.org/infra/perf/go/regression/mocks"
)

func setupForTest(t *testing.T, userIsEditor bool) (*httptest.ResponseRecorder, *http.Request, regressionsApi) {
	login := mocks.NewLogin(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/not-used", nil)
	login.On("LoggedInAs", r).Return(alogin.EMail("nobody@example.org"))
	login.On("HasRole", r, roles.Editor).Return(userIsEditor)
	rApi := NewRegressionsApi(login, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	return w, r, rApi
}

func TestFrontendRegressionsHandler_Success(t *testing.T) {
	regMock := regressionMocks.NewStore(t)
	req := regression.GetAnomalyListRequest{
		SubName:             "test",
		PaginationOffset:    10,
		IncludeTriaged:      false,
		IncludeImprovements: false,
	}
	regMock.On("GetRegressionsBySubName", testutils.AnyContext, req, 10).Return(
		[]*regression.Regression{
			{
				Id:      "r1",
				AlertId: 1,
			},
			{
				Id:      "r2",
				AlertId: 1,
			},
			{
				Id:      "r3",
				AlertId: 2,
			},
		}, nil)
	f := NewRegressionsApi(nil, nil, nil, regMock, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	w := httptest.NewRecorder()

	r := httptest.NewRequest("GET", "/_/regressions?sub_name=test&limit=10&offset=10", nil)
	f.regressionsHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	require.Contains(t, w.Body.String(), "r1")
	require.Contains(t, w.Body.String(), "r2")
	require.Contains(t, w.Body.String(), "r3")
}

func TestFrontendRegressionsHandler_ShowTriagedAndImprovements(t *testing.T) {
	regMock := regressionMocks.NewStore(t)
	req := regression.GetAnomalyListRequest{
		SubName:             "test",
		PaginationOffset:    10,
		IncludeTriaged:      true,
		IncludeImprovements: true,
	}
	// TODO(b/477238168) determine what "triaged" really means.
	regMock.On("GetRegressionsBySubName", testutils.AnyContext, req, 10).Return(
		[]*regression.Regression{
			{
				Id:            "r1",
				AlertId:       1,
				IsImprovement: true,
			},
			{
				Id:            "r2",
				AlertId:       1,
				IsImprovement: true,
			},
			{
				Id:            "r3",
				AlertId:       2,
				IsImprovement: true,
			},
		}, nil)
	f := NewRegressionsApi(nil, nil, nil, regMock, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	w := httptest.NewRecorder()

	r := httptest.NewRequest("GET", "/_/regressions?sub_name=test&limit=10&offset=10&triaged=true&improvements=true", nil)
	f.regressionsHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	require.Contains(t, w.Body.String(), "r1")
	require.Contains(t, w.Body.String(), "r2")
	require.Contains(t, w.Body.String(), "r3")
}

func TestFrontendIsEditor_UserIsEditor_ReportsStatusOK(t *testing.T) {
	w, r, f := setupForTest(t, true)
	f.isEditor(w, r, "my-test-action", nil)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)
}

func TestFrontendIsEditor_UserIsOnlyViewer_ReportsError(t *testing.T) {
	w, r, f := setupForTest(t, false)
	f.isEditor(w, r, "my-test-action", nil)
	require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
}
