// Package frontend contains the Go code that servers the Perf web UI.
package frontend

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"testing/fstest"

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
	require.Equal(t, "/e", w.Result().Header.Get("Location"))
}

func TestFrontend_SpecifiedRedirectUrl_Redirects(t *testing.T) {
	configFileBytes := testutils.ReadFileBytes(t, "config.json")
	err := json.Unmarshal(configFileBytes, &config.Config)
	config.Config.LandingPageRelPath = "/m"
	require.NoError(t, err)

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	oldMainHandler(w, r)

	require.Equal(t, http.StatusMovedPermanently, w.Result().StatusCode)
	require.Equal(t, "/m", w.Result().Header.Get("Location"))
}

func TestFrontend_StripSlashes(t *testing.T) {
	host := "localhost:8001"

	f := &Frontend{
		loginProvider: mocks.NewLogin(t),
		flags: &config.FrontendFlags{
			ConfigFilename:         "./testdata/config.json",
			NumParamSetsForQueries: 2,
		},
	}

	mapFS := fstest.MapFS{}
	for _, filename := range templateFilenames {
		mapFS[filename] = &fstest.MapFile{
			Data: []byte(""),
		}
	}
	f.distFileSystem = http.FS(mapFS)

	configFileBytes := testutils.ReadFileBytes(t, "config.json")
	err := json.Unmarshal(configFileBytes, &config.Config)
	require.NoError(t, err)
	u, err := url.Parse(config.Config.URL)
	require.NoError(t, err)
	f.host = u.Host
	// Setup redirect for the root path "/"
	config.Config.LandingPageRelPath = "/e"
	require.NoError(t, err)

	r := f.GetHandler([]string{host})

	testCases := []struct {
		path               string
		expectedStatusCode int
		expectedRedirect   string
	}{
		{
			path:               "/c/",
			expectedStatusCode: http.StatusOK,
		},
		{
			path:               "/c",
			expectedStatusCode: http.StatusOK,
		},
		{
			path:               "/a/",
			expectedStatusCode: http.StatusOK,
		},
		{
			path:               "/a",
			expectedStatusCode: http.StatusOK,
		},
		{
			path:               "/m/",
			expectedStatusCode: http.StatusOK,
		},
		{
			path:               "/m",
			expectedStatusCode: http.StatusOK,
		},
		{
			path:               "/e",
			expectedStatusCode: http.StatusOK,
		},
		{
			path:               "/e/",
			expectedStatusCode: http.StatusOK,
		},
		{
			path:               "/",
			expectedStatusCode: http.StatusMovedPermanently,
			expectedRedirect:   config.Config.LandingPageRelPath,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", tc.path, nil)
			req.Host = host
			req.TLS = &tls.ConnectionState{}
			r.ServeHTTP(w, req)

			require.Equal(t, tc.expectedStatusCode, w.Result().StatusCode)
			if tc.expectedRedirect != "" {
				require.Equal(t, tc.expectedRedirect, w.Header().Get("Location"))
			}
		})
	}
}
