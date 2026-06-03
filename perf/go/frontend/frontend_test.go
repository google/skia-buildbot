// Package frontend contains the Go code that servers the Perf web UI.
package frontend

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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

func TestFrontend_DefaultToManualPlotMode_Redirects(t *testing.T) {
	configFileBytes := testutils.ReadFileBytes(t, "config.json")
	err := json.Unmarshal(configFileBytes, &config.Config)
	config.Config.LandingPageRelPath = "/m/"
	config.Config.DefaultToManualPlotMode = true
	require.NoError(t, err)

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	oldMainHandler(w, r)

	require.Equal(t, http.StatusMovedPermanently, w.Result().StatusCode)
	require.Equal(t, "/m/?manual_plot_mode=true", w.Result().Header.Get("Location"))
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

	queryThatDoesNotChangeAnything := "?params_that_should_be_ignored=ignored"

	testCases := []struct {
		path                            string
		expectedStatusCode              int
		expectedRedirect                string
		compareAgainstPathWithoutParams string
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
		{
			path:                            "/m" + queryThatDoesNotChangeAnything,
			expectedStatusCode:              http.StatusOK,
			compareAgainstPathWithoutParams: "/m",
		},
		{
			// Note that the path does not change in browser.
			path:                            "/m/" + queryThatDoesNotChangeAnything,
			expectedStatusCode:              http.StatusOK,
			compareAgainstPathWithoutParams: "/m/",
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
			} else {
				expectedPath := tc.compareAgainstPathWithoutParams
				if expectedPath == "" {
					expectedPath = tc.path
				}
				require.Equal(t, expectedPath, req.URL.Path)
			}
		})
	}
}

func TestFrontend_loadAppVersion_NoFile_SetsDevVersion(t *testing.T) {
	f := &Frontend{
		flags: &config.FrontendFlags{
			VersionFile: "",
		},
	}
	f.loadAppVersion()
	require.Contains(t, f.appVersion, "dev-")
}

func TestFrontend_loadAppVersion_WithFile_SetsVersionFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "VERSION.txt")
	err := os.WriteFile(tmpFile, []byte("git-hash-123\n"), 0644)
	require.NoError(t, err)

	f := &Frontend{
		flags: &config.FrontendFlags{
			VersionFile: tmpFile,
		},
	}
	f.loadAppVersion()
	require.Equal(t, "git-hash-123", f.appVersion)
}

func TestFrontend_loadAppVersion_FileReadError_SetsEmptyVersion(t *testing.T) {
	f := &Frontend{
		flags: &config.FrontendFlags{
			VersionFile: "/non/existent/file",
		},
	}
	require.NotPanics(t, func() {
		f.loadAppVersion()
	})
	require.Equal(t, "", f.appVersion)
}

func TestFrontend_GetPageContext_InstanceName(t *testing.T) {
	f := &Frontend{
		flags: &config.FrontendFlags{},
	}
	// Save original config to restore later if needed, though tests should be isolated.
	// Better to just set it and let other tests set it if they need it.
	// Assuming config.Config is global and mutable.
	originalConfig := config.Config
	defer func() { config.Config = originalConfig }()

	config.Config = &config.InstanceConfig{
		URL: "https://perf.luci.app",
	}

	// Case 1: No instance_name
	ctx, err := f.getPageContext()
	require.NoError(t, err)
	require.Contains(t, string(ctx), "\"instance_name\": \"\"")
	require.NotContains(t, string(ctx), "\"schema_version\"")

	// Case 2: With instance_name
	config.Config.InstanceName = "chrome-perf-test"
	ctx, err = f.getPageContext()
	require.NoError(t, err)
	require.Contains(t, string(ctx), "\"instance_name\": \"chrome-perf-test\"")

	// Case 3: With long instance_name
	config.Config.InstanceName = "this-is-a-long-instance-name-that-exceeds-the-limit-of-64-chars-by-a-bit"
	ctx, err = f.getPageContext()
	require.NoError(t, err)
	// It should NOT be truncated in the JSON context, only in the UI display if needed.
	// The backend just passes it through.
	require.Contains(t, string(ctx), "\"instance_name\": \"this-is-a-long-instance-name-that-exceeds-the-limit-of-64-chars-by-a-bit\"")

	// Case 4: With non-zero schema version
	atomic.StoreInt32(&f.schemaVersion, 42)
	ctx, err = f.getPageContext()
	require.NoError(t, err)
	require.Contains(t, string(ctx), "\"schema_version\": 42")
}

func TestFrontend_devVersionHandler_ReturnsVersion(t *testing.T) {
	f := &Frontend{
		appVersion: "test-version-123",
	}

	r := httptest.NewRequest("GET", "/_/dev/version", nil)
	w := httptest.NewRecorder()

	f.devVersionHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	require.JSONEq(t, `{"version": "test-version-123"}`, w.Body.String())
}

func TestFrontend_feErrorLogHandler_Success(t *testing.T) {
	f := &Frontend{}

	// Test case 1: Successful decoding with a non-empty message.
	body := `{"message": "Test error message", "source": "test-source", "errorCode": "500", "endpoint": "test-endpoint", "method": "test-method", "url": "test-url", "stack": "test-stack-trace" }`
	r := httptest.NewRequest("POST", "/_/fe_error_log", strings.NewReader(body))
	w := httptest.NewRecorder()

	f.feErrorLogHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	// Test case 2: Successful decoding with an empty message.
	body = `{"message": "", "source": "test-source"}`
	r = httptest.NewRequest("POST", "/_/fe_error_log", strings.NewReader(body))
	w = httptest.NewRecorder()

	f.feErrorLogHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
}

func TestFrontend_feErrorLogHandler_DecodeError(t *testing.T) {
	f := &Frontend{}

	// Test case: Decoding error (bad JSON).
	body := `{"message": "Test error message", "source": "test-source"` // Missing closing brace
	r := httptest.NewRequest("POST", "/_/fe_error_log", strings.NewReader(body))
	w := httptest.NewRecorder()

	f.feErrorLogHandler(w, r)

	require.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	require.Contains(t, w.Body.String(), "Failed to decode JSON.")
}

func TestFrontend_CSPHeaders_AllowsWASM(t *testing.T) {
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

	r := f.GetHandler([]string{host})

	// Check /e2 (should allow WASM)
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/e2", nil)
	req1.Host = host
	req1.TLS = &tls.ConnectionState{}
	r.ServeHTTP(w1, req1)

	require.Equal(t, http.StatusOK, w1.Result().StatusCode)
	csp1 := w1.Result().Header.Get("Content-Security-Policy")
	require.Contains(t, csp1, "'unsafe-eval'")

	// Check /u (should allow WASM)
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("GET", "/u", nil)
	req3.Host = host
	req3.TLS = &tls.ConnectionState{}
	r.ServeHTTP(w3, req3)

	require.Equal(t, http.StatusOK, w3.Result().StatusCode)
	csp3 := w3.Result().Header.Get("Content-Security-Policy")
	require.Contains(t, csp3, "'unsafe-eval'")

	// Check /m (should NOT allow WASM)
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/m", nil)
	req2.Host = host
	req2.TLS = &tls.ConnectionState{}
	r.ServeHTTP(w2, req2)

	require.Equal(t, http.StatusOK, w2.Result().StatusCode)
	csp2 := w2.Result().Header.Get("Content-Security-Policy")
	require.NotContains(t, csp2, "'unsafe-eval'")
}
