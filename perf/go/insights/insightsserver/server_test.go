// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/insights"
	"go.skia.org/infra/perf/go/insights/site"
	"go.skia.org/infra/perf/go/regression"
	subpb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

type fakeRegressions struct{}

func (fakeRegressions) GetRegressionsBySubName(context.Context, regression.GetAnomalyListRequest, int) ([]*regression.Regression, error) {
	return nil, nil
}

func (fakeRegressions) GetByIDs(context.Context, []string) ([]*regression.Regression, error) {
	return nil, nil
}

func (fakeRegressions) GetByRevision(context.Context, string) ([]*regression.Regression, error) {
	return nil, nil
}

func (fakeRegressions) GetBugIdsForRegressions(_ context.Context, regs []*regression.Regression) ([]*regression.Regression, error) {
	return regs, nil
}

// fakeSubs records whether each call had a deadline, like the Perf database pool requires.
type fakeSubs struct {
	mu        sync.Mutex
	deadlines []bool
}

func (f *fakeSubs) GetAllActiveSubscriptions(ctx context.Context) ([]*subpb.Subscription, error) {
	_, ok := ctx.Deadline()
	f.mu.Lock()
	f.deadlines = append(f.deadlines, ok)
	f.mu.Unlock()
	return []*subpb.Subscription{{Name: "Demo Sheriff"}}, nil
}

func defaultOptions() serverOptions {
	return serverOptions{RateLimitQPS: 100, RateLimitBurst: 100, RequestTimeout: 5 * time.Second}
}

// newTestServer starts a server whose allowed host and origin are its own address.
func newTestServer(t *testing.T, subs *fakeSubs, opts serverOptions) *httptest.Server {
	svc, err := insights.New(insights.Stores{Regressions: fakeRegressions{}, Subscriptions: subs}, insights.Options{InstanceName: "demo"})
	require.NoError(t, err)
	ts := httptest.NewUnstartedServer(nil)
	addr := ts.Listener.Addr().String()
	s, err := site.New(site.Options{InstanceName: "demo", Visibility: "internal", PublicURL: "http://" + addr})
	require.NoError(t, err)
	opts.AllowedHosts = append(opts.AllowedHosts, addr)
	opts.AllowedOrigins = append(opts.AllowedOrigins, "http://"+addr)
	ts.Config.Handler = newHandler(svc, s, opts)
	ts.Start()
	t.Cleanup(ts.Close)
	return ts
}

func do(t *testing.T, ts *httptest.Server, method, path string, body string, headers map[string]string) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	for k, v := range headers {
		if k == "Host" {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(b)
}

func errorCode(t *testing.T, body string) insights.Code {
	var env insights.ErrorEnvelope
	require.NoError(t, json.Unmarshal([]byte(body), &env), body)
	require.NotNil(t, env.Error, body)
	return env.Error.Code
}

func TestHandler_ServesAllSurfaces(t *testing.T) {
	ts := newTestServer(t, &fakeSubs{}, defaultOptions())

	resp, body := do(t, ts, http.MethodGet, "/healthz", "", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `{"status":"ok"}`, body)

	resp, body = do(t, ts, http.MethodGet, "/api/v1/sheriffs", "", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, "Demo Sheriff")
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))

	resp, body = do(t, ts, http.MethodGet, "/", "", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, "perfinsights")
	assert.NotEmpty(t, resp.Header.Get("Content-Security-Policy"))

	resp, _ = do(t, ts, http.MethodGet, "/llms.txt", "", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp, body = do(t, ts, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		map[string]string{"Content-Type": "application/json", "Accept": "application/json, text/event-stream"})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, "list_anomalies")

	resp, _ = do(t, ts, http.MethodGet, "/mcp", "", nil)
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)

	resp, body = do(t, ts, http.MethodGet, "/api/v1/nope", "", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, insights.CodeNotFound, errorCode(t, body))

	resp, body = do(t, ts, http.MethodDelete, "/api/v1/sheriffs", "", nil)
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	assert.Equal(t, insights.CodeInvalidArgument, errorCode(t, body))

	resp, _ = do(t, ts, http.MethodOptions, "/api/v1/sheriffs", "", map[string]string{"Access-Control-Request-Method": "GET"})
	assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"), "CORS preflights must never be approved")
}

func TestHandler_QueriesHaveDeadlines(t *testing.T) {
	subs := &fakeSubs{}
	ts := newTestServer(t, subs, defaultOptions())
	resp, _ := do(t, ts, http.MethodGet, "/healthz", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp, _ = do(t, ts, http.MethodGet, "/api/v1/sheriffs", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	subs.mu.Lock()
	defer subs.mu.Unlock()
	require.NotEmpty(t, subs.deadlines)
	for _, ok := range subs.deadlines {
		assert.True(t, ok)
	}
}

func TestHandler_RejectsOtherHostsAndOrigins(t *testing.T) {
	ts := newTestServer(t, &fakeSubs{}, defaultOptions())

	// A page that rebinds evil.example.com to 127.0.0.1 sends its own name as Host.
	resp, body := do(t, ts, http.MethodGet, "/api/v1/sheriffs", "", map[string]string{"Host": "evil.example.com:8090"})
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Equal(t, insights.CodePermissionDenied, errorCode(t, body))

	resp, body = do(t, ts, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example.com"})
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Equal(t, insights.CodePermissionDenied, errorCode(t, body))

	resp, _ = do(t, ts, http.MethodGet, "/api/v1/sheriffs", "", map[string]string{"Origin": ts.URL})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandler_RequireAuth(t *testing.T) {
	opts := defaultOptions()
	opts.RequireAuth = true
	ts := newTestServer(t, &fakeSubs{}, opts)

	resp, body := do(t, ts, http.MethodGet, "/api/v1/sheriffs", "", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, insights.CodeUnauthenticated, errorCode(t, body))

	resp, body = do(t, ts, http.MethodPost, "/mcp", `{}`, map[string]string{"Content-Type": "application/json", webAuthUserHeader: "user@google.com"})
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Equal(t, insights.CodePermissionDenied, errorCode(t, body))

	resp, _ = do(t, ts, http.MethodGet, "/api/v1/sheriffs", "", map[string]string{webAuthUserHeader: "user@google.com", webAuthRolesHeader: "viewer"})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp, _ = do(t, ts, http.MethodGet, "/healthz", "", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "probes don't send credentials")
}

func TestHandler_RateLimitsPerCaller(t *testing.T) {
	opts := defaultOptions()
	opts.RateLimitQPS = 0.01
	opts.RateLimitBurst = 2
	ts := newTestServer(t, &fakeSubs{}, opts)

	for i := 0; i < 2; i++ {
		resp, _ := do(t, ts, http.MethodGet, "/api/v1/instance", "", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	resp, body := do(t, ts, http.MethodGet, "/api/v1/instance", "", nil)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("Retry-After"))
	assert.Equal(t, insights.CodeResourceExhausted, errorCode(t, body))
}

func TestRateLimiter_KeysByUserOnlyWhenAuthIsRequired(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set(webAuthUserHeader, "User@google.com")
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	assert.Equal(t, "user:user@google.com", newRateLimiter(1, 1, true).callerKey(r))
	assert.Equal(t, "ip:10.0.0.1", newRateLimiter(1, 1, false).callerKey(r))
}

func TestRateLimiter_BoundsMemory(t *testing.T) {
	l := newRateLimiter(1000, 1000, false)
	for i := 0; i < maxLimiters+10; i++ {
		l.allow("ip:" + strconv.Itoa(i))
	}
	assert.LessOrEqual(t, len(l.limiters), maxLimiters)
}

func TestHandler_UberProxyPENHostsAndSameOrigin(t *testing.T) {
	opts := defaultOptions()
	opts.AllowedHostSuffixes = []string{uberproxyHostSuffix}
	ts := newTestServer(t, &fakeSubs{}, opts)
	pen := "b2607f8b04800100000289cb1c0a808981f90000000b690fffd5400.proxy.googlers.com"
	mcpList := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`

	resp, _ := do(t, ts, http.MethodGet, "/", "", map[string]string{"Host": pen})
	assert.Equal(t, http.StatusOK, resp.StatusCode, "UberProxy sends the PEN hostname as Host")

	resp, _ = do(t, ts, http.MethodPost, "/mcp", mcpList,
		map[string]string{"Host": pen, "Origin": "https://" + pen, "Content-Type": "application/json"})
	assert.Equal(t, http.StatusOK, resp.StatusCode, "same-origin requests from the PEN page")

	resp, body := do(t, ts, http.MethodPost, "/mcp", mcpList,
		map[string]string{"Host": pen, "Origin": "https://other.proxy.googlers.com", "Content-Type": "application/json"})
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "another PEN page is a different origin")
	assert.Equal(t, insights.CodePermissionDenied, errorCode(t, body))

	resp, _ = do(t, ts, http.MethodGet, "/api/v1/sheriffs", "", map[string]string{"Host": "evilproxy.googlers.com"})
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}
