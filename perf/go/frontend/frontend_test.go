// Package frontend contains the Go code that servers the Perf web UI.
package frontend

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/mocks"
	"go.skia.org/infra/go/roles"
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
