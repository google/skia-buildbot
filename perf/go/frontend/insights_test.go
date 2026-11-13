// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin"
	loginmocks "go.skia.org/infra/go/alogin/mocks"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/insights"
	"go.skia.org/infra/perf/go/insights/site"
	regmocks "go.skia.org/infra/perf/go/regression/mocks"
	submocks "go.skia.org/infra/perf/go/subscription/mocks"
	subpb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

const insightsTestPublicURL = "https://perf.example.com/insights"

// newInsightsLogin returns a login provider for a user with the given email and roles.
func newInsightsLogin(t *testing.T, email string, userRoles roles.Roles) *loginmocks.Login {
	login := loginmocks.NewLogin(t)
	login.On("LoggedInAs", mock.Anything).Return(alogin.EMail(email)).Maybe()
	login.On("Roles", mock.Anything).Return(userRoles).Maybe()
	return login
}

func newInsightsSubscriptions(t *testing.T) *submocks.Store {
	subs := submocks.NewStore(t)
	subs.On("GetAllActiveSubscriptions", mock.Anything).Return([]*subpb.Subscription{{Name: "V8 JavaScript Internal"}}, nil).Maybe()
	return subs
}

func newInsightsTestServer(t *testing.T, login alogin.Login) *httptest.Server {
	svc, err := insights.New(insights.Stores{
		Regressions:   regmocks.NewStore(t),
		Subscriptions: newInsightsSubscriptions(t),
	}, insights.Options{InstanceName: "V8 (Autopush)", InstanceURL: "https://perf.example.com"})
	require.NoError(t, err)
	r := chi.NewRouter()
	require.NoError(t, mountInsightsService(r, svc, login, insightsTestPublicURL))
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts
}

func insightsDo(t *testing.T, ts *httptest.Server, method, path, body string) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(b)
}

func TestMountInsights_ServesThePagesRESTAndMCPUnderInsights(t *testing.T) {
	ts := newInsightsTestServer(t, newInsightsLogin(t, "user@google.com", roles.Roles{roles.Viewer}))

	resp, body := insightsDo(t, ts, http.MethodGet, "/insights/", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, site.ContentSecurityPolicy, resp.Header.Get("Content-Security-Policy"))
	assert.Contains(t, body, `data-base="/insights"`)
	assert.Contains(t, body, `src="/insights/static/site.js"`)
	assert.Contains(t, body, insightsTestPublicURL+"/mcp")

	resp, body = insightsDo(t, ts, http.MethodGet, "/insights/trace?id=68bf6dad-916e-43f4-b59e-4d72d19f5ee3", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, `data-page="trace"`)

	resp, _ = insightsDo(t, ts, http.MethodGet, "/insights/static/site.js", "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp, body = insightsDo(t, ts, http.MethodGet, "/insights/api/v1/sheriffs", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, "V8 JavaScript Internal")

	resp, body = insightsDo(t, ts, http.MethodGet, "/insights/openapi.json", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &doc))
	assert.Equal(t, []interface{}{map[string]interface{}{"url": "/insights"}}, doc["servers"])

	resp, body = insightsDo(t, ts, http.MethodPost, "/insights/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, "summarize_anomalies")

	resp, body = insightsDo(t, ts, http.MethodGet, "/insights/healthz", "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, `"ok"`)

	resp, body = insightsDo(t, ts, http.MethodGet, "/insights/nope", "")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Contains(t, body, `"NOT_FOUND"`)

	resp, _ = insightsDo(t, ts, http.MethodDelete, "/insights/api/v1/sheriffs", "")
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestMountInsights_RequiresASignedInUserWithARole(t *testing.T) {
	mcpList := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`

	anonymous := newInsightsTestServer(t, newInsightsLogin(t, "", nil))
	for _, path := range []string{"/insights/", "/insights/api/v1/sheriffs", "/insights/healthz"} {
		resp, body := insightsDo(t, anonymous, http.MethodGet, path, "")
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, path)
		assert.Contains(t, body, `"UNAUTHENTICATED"`, path)
	}
	resp, _ := insightsDo(t, anonymous, http.MethodPost, "/insights/mcp", mcpList)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	noRole := newInsightsTestServer(t, newInsightsLogin(t, "user@google.com", roles.Roles{}))
	resp, body := insightsDo(t, noRole, http.MethodGet, "/insights/api/v1/sheriffs", "")
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Contains(t, body, `"PERMISSION_DENIED"`)
	resp, _ = insightsDo(t, noRole, http.MethodPost, "/insights/mcp", mcpList)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	editor := newInsightsTestServer(t, newInsightsLogin(t, "user@google.com", roles.Roles{roles.Editor}))
	resp, _ = insightsDo(t, editor, http.MethodGet, "/insights/api/v1/sheriffs", "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestLimitConcurrency_RejectsRequestsOverTheLimit(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	h := limitConcurrency(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
	}))
	done := make(chan int)
	go func() {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		done <- w.Code
	}()
	<-entered

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), `"RESOURCE_EXHAUSTED"`)
	assert.NotEmpty(t, w.Header().Get("Retry-After"))

	close(release)
	assert.Equal(t, http.StatusOK, <-done)

	// The slot is free again.
	go func() { <-entered }()
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetHandler_ServesInsightsOnlyWhenEnabled(t *testing.T) {
	const host = "localhost:8001"
	configFileBytes := testutils.ReadFileBytes(t, "config.json")
	require.NoError(t, json.Unmarshal(configFileBytes, &config.Config))
	t.Cleanup(func() { config.Config.EnableInsights = false })

	newHandler := func(enabled bool) http.Handler {
		config.Config.EnableInsights = enabled
		f := &Frontend{
			loginProvider: newInsightsLogin(t, "user@google.com", roles.Roles{roles.Viewer}),
			regStore:      regmocks.NewStore(t),
			subStore:      newInsightsSubscriptions(t),
			flags: &config.FrontendFlags{
				ConfigFilename:         "./testdata/config.json",
				NumParamSetsForQueries: 2,
			},
		}
		mapFS := fstest.MapFS{}
		for _, filename := range templateFilenames {
			mapFS[filename] = &fstest.MapFile{Data: []byte("")}
		}
		f.distFileSystem = http.FS(mapFS)
		return f.GetHandler([]string{host})
	}
	get := func(h http.Handler, path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Host = host
		req.TLS = &tls.ConnectionState{}
		h.ServeHTTP(w, req)
		return w
	}

	h := newHandler(true)
	for _, path := range []string{"/insights", "/insights/", "/insights/trace", "/insights/api/v1/sheriffs"} {
		w := get(h, path)
		assert.Equal(t, http.StatusOK, w.Code, path)
	}
	// The pages keep their own, stricter CSP rather than the one of the Perf UI.
	assert.Equal(t, site.ContentSecurityPolicy, get(h, "/insights/").Header().Get("Content-Security-Policy"))

	h = newHandler(false)
	assert.Equal(t, http.StatusNotFound, get(h, "/insights/api/v1/sheriffs").Code)
}
