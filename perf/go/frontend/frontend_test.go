// Package frontend contains the Go code that servers the Perf web UI.
package frontend

import (
	"bytes"
	"encoding/json"
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
	subscriptionMocks "go.skia.org/infra/perf/go/subscription/mocks"
	subscriptionProtoV1 "go.skia.org/infra/perf/go/subscription/proto/v1"
)

func setupForTest(t *testing.T, userIsEditor bool) (*httptest.ResponseRecorder, *http.Request, *Frontend) {
	login := mocks.NewLogin(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/not-used", nil)
	login.On("LoggedInAs", r).Return(alogin.EMail("nobody@example.org"))
	login.On("HasRole", r, roles.Editor).Return(userIsEditor)
	f := &Frontend{
		loginProvider: login,
	}
	return w, r, f
}

func TestFrontend_ShouldInitAllHandlers(t *testing.T) {
	f := &Frontend{
		loginProvider: mocks.NewLogin(t),
	}
	// Check if there is a conflict or misuse in the http/chi handler API
	require.NotPanics(t, func() {
		f.GetHandler([]string{})
	})
}

func TestFrontend_RoleEnforced_ReportsError(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	login := mocks.NewLogin(t)
	login.On("Status", r).Return(alogin.Status{
		EMail: "nobody@example.org",
	})
	login.On("HasRole", r, roles.Admin).Return(false)

	f := &Frontend{
		loginProvider: login,
	}
	h := f.RoleEnforcedHandler(roles.Admin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusForbidden, w.Result().StatusCode)
	require.Contains(t, w.Body.String(), "not authenticated")
}

func TestFrontend_RoleEnforced_ReportsOK(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	login := mocks.NewLogin(t)
	login.On("Status", r).Return(alogin.Status{
		EMail: "nobody@example.org",
	})
	login.On("HasRole", r, roles.Admin).Return(true)
	const expected_body = "hello"

	f := &Frontend{
		loginProvider: login,
	}
	h := f.RoleEnforcedHandler(roles.Admin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(expected_body))
	}))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	require.Equal(t, expected_body, w.Body.String())
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

func TestFrontendDetailsHandler_InvalidTraceID_ReturnsErrorMessage(t *testing.T) {
	f := &Frontend{}
	w := httptest.NewRecorder()

	req := CommitDetailsRequest{
		CommitNumber: 0,
		TraceID:      `calc("this is not a trace id, but a calculation")`,
	}
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(req)
	require.NoError(t, err)

	r := httptest.NewRequest("POST", "/_/details", &b)
	f.detailsHandler(w, r)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	require.Contains(t, w.Body.String(), "version\":0")
}

func TestFrontendUniqSubscriptionHandler_Success(t *testing.T) {
	subMock := subscriptionMocks.NewStore(t)
	subMock.On("GetAllSubscriptions", testutils.AnyContext).Return(
		[]*subscriptionProtoV1.Subscription{
			{
				Name:         "Test Subscription 1",
				Revision:     "abcd",
				BugLabels:    []string{"A", "B"},
				Hotlists:     []string{"C", "D"},
				BugComponent: "Component1>Subcomponent1",
				BugPriority:  1,
				BugSeverity:  2,
				BugCcEmails: []string{
					"abcd@efg.com",
					"1234@567.com",
				},
				ContactEmail: "test@owner.com",
			},
			{
				Name:         "Test Subscription 2",
				Revision:     "bcde",
				BugLabels:    []string{"A", "B"},
				Hotlists:     []string{"C", "D"},
				BugComponent: "Component1>Subcomponent1",
				BugPriority:  1,
				BugSeverity:  2,
				BugCcEmails: []string{
					"abcd@efg.com",
					"1234@567.com",
				},
				ContactEmail: "test@owner.com",
			},
		}, nil)
	f := &Frontend{
		subStore: subMock,
	}
	w := httptest.NewRecorder()

	r := httptest.NewRequest("GET", "/_/allsubscriptions", nil)
	f.subscriptionsHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	require.Contains(t, w.Body.String(), "Test Subscription 1")
	require.Contains(t, w.Body.String(), "Test Subscription 2")
}

func TestFrontendRegressionsHandler_Success(t *testing.T) {
	regMock := regressionMocks.NewStore(t)
	regMock.On("GetRegressionsBySubName", testutils.AnyContext, "test", 10, 10).Return(
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
	f := &Frontend{
		regStore: regMock,
	}
	w := httptest.NewRecorder()

	r := httptest.NewRequest("GET", "/_/regressions?sub_name=test&limit=10&offset=10", nil)
	f.regressionsHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	require.Contains(t, w.Body.String(), "r1")
	require.Contains(t, w.Body.String(), "r2")
	require.Contains(t, w.Body.String(), "r3")
}
