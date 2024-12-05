// Package frontend contains the Go code that servers the Perf web UI.
package frontend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/mocks"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/config"
)

func TestFrontend_ShouldInitAllHandlers(t *testing.T) {
	f := &Frontend{
		loginProvider: mocks.NewLogin(t),
		flags: &config.FrontendFlags{
			NumParamSetsForQueries: 2,
		},
	}
	configFileBytes := testutils.ReadFileBytes(t, "config.json")
	err := json.Unmarshal(configFileBytes, &config.Config)
	require.NoError(t, err)

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

func TestFrontend_UnspecifiedRedirectUrl_Redirects(t *testing.T) {
	configFileBytes := testutils.ReadFileBytes(t, "config.json")
	err := json.Unmarshal(configFileBytes, &config.Config)
	require.NoError(t, err)

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	oldMainHandler(w, r)

	require.Equal(t, http.StatusMovedPermanently, w.Result().StatusCode)
	require.Equal(t, "/e/", w.Result().Header.Get("Location"))
}

func TestFrontend_SpecifiedRedirectUrl_Redirects(t *testing.T) {
	configFileBytes := testutils.ReadFileBytes(t, "config.json")
	err := json.Unmarshal(configFileBytes, &config.Config)
	config.Config.LandingPageRelPath = "/m/"
	require.NoError(t, err)

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	oldMainHandler(w, r)

	require.Equal(t, http.StatusMovedPermanently, w.Result().StatusCode)
	require.Equal(t, "/m/", w.Result().Header.Get("Location"))
}
