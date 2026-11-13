// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPollInterval = 5 * time.Millisecond

var (
	testBegin = time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	testEnd   = testBegin.Add(24 * time.Hour)
)

func newTestTarget(t *testing.T, h http.Handler) *target {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &target{name: "test", baseURL: srv.URL, client: newHTTPClient()}
}

func runCheck(t *testing.T, c Check, tgt *target) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.Run(ctx, tgt)
}

// pageHTML mirrors how perfserver templates inject the page context.
func pageHTML(appVersion string) string {
	return `<!DOCTYPE html><html><head><script nonce="n">
      window.perf = {"instance_name": "test", "app_version": "` + appVersion + `"};
    </script></head><body></body></html>`
}

// versionHandler serves /m/ with one app_version per request and repeats the
// last one.
func versionHandler(calls *atomic.Int32, versions ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/m/" {
			http.NotFound(w, r)
			return
		}
		n := int(calls.Add(1))
		_, _ = w.Write([]byte(pageHTML(versions[min(n, len(versions))-1])))
	})
}

// frameHandler answers POST /_/frame/start with a Running progress, sends the
// decoded request body to bodies, and serves one progress body per poll,
// repeating the last one.
func frameHandler(bodies chan<- map[string]any, polls *atomic.Int32, progress ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/_/frame/start":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			bodies <- body
			_, _ = w.Write([]byte(`{"status":"Running","messages":[],"url":"/_/status/abc"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/_/status/abc":
			n := int(polls.Add(1))
			_, _ = w.Write([]byte(progress[min(n, len(progress))-1]))
		default:
			http.NotFound(w, r)
		}
	})
}

const runningProgress = `{"status":"Running","messages":[],"url":"/_/status/abc"}`

func TestExtractWindowPerf_SpacedAndCompactForms_ReturnsContext(t *testing.T) {
	for name, page := range map[string]string{
		"spaced":  `<script>window.perf = {"app_version": "abc"};</script>`,
		"compact": `<script>window.perf={"app_version":"abc"};</script>`,
		"newline": "<script>\n  window.perf =\n  {\"app_version\": \"abc\",\n  \"keys\": [1, 2]};\n</script>",
	} {
		t.Run(name, func(t *testing.T) {
			perf, err := extractWindowPerf([]byte(page))
			require.NoError(t, err)
			assert.Equal(t, "abc", perf["app_version"])
		})
	}
}

func TestExtractWindowPerf_MissingOrMalformed_ReturnsError(t *testing.T) {
	for name, page := range map[string]string{
		"missing":   `<script>window.other = {};</script>`,
		"malformed": `<script>window.perf = {"app_version": };</script>`,
		"null":      `<script>window.perf = null;</script>`,
		"string":    `<script>window.perf = "abc";</script>`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := extractWindowPerf([]byte(page))
			require.Error(t, err)
		})
	}
}

func TestVersionCheck_Match_Passes(t *testing.T) {
	var calls atomic.Int32
	tgt := newTestTarget(t, versionHandler(&calls, "abc"))

	require.NoError(t, runCheck(t, versionCheck("abc", 0, testPollInterval), tgt))
	assert.Equal(t, "abc", tgt.appVersion)
}

func TestVersionCheck_MatchAfterRetry_Passes(t *testing.T) {
	var calls atomic.Int32
	tgt := newTestTarget(t, versionHandler(&calls, "old", "old", "abc"))

	require.NoError(t, runCheck(t, versionCheck("abc", time.Second, testPollInterval), tgt))
	assert.Equal(t, int32(3), calls.Load())
}

func TestVersionCheck_MismatchUntilTimeout_FailsWithObservedVersion(t *testing.T) {
	var calls atomic.Int32
	tgt := newTestTarget(t, versionHandler(&calls, "old"))

	err := runCheck(t, versionCheck("abc", 50*time.Millisecond, testPollInterval), tgt)
	require.Error(t, err)
	assert.Contains(t, errorMessage(err), `app_version is "old", want "abc"`)
	assert.Greater(t, calls.Load(), int32(1))
	assert.Equal(t, "old", tgt.appVersion)
}

func TestHealthCheck_Non200_Fails(t *testing.T) {
	tgt := newTestTarget(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readiness" {
			http.Error(w, "Database schema needs migration.", http.StatusServiceUnavailable)
		}
	}))

	require.NoError(t, runCheck(t, healthCheck("/healthz"), tgt))
	err := runCheck(t, healthCheck("/readiness"), tgt)
	require.Error(t, err)
	assert.Contains(t, errorMessage(err), "GET /readiness returned 503")
	assert.Contains(t, errorMessage(err), "Database schema needs migration.")
}

func TestTemplatesCheck_PageWithoutWindowPerf_FailsNamingPage(t *testing.T) {
	tgt := newTestTarget(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/e/" {
			_, _ = w.Write([]byte("<html><body>no context</body></html>"))
			return
		}
		_, _ = w.Write([]byte(pageHTML("abc")))
	}))

	err := runCheck(t, templatesCheck([]string{"/m/", "/e/", "/a/"}), tgt)
	require.Error(t, err)
	assert.Contains(t, errorMessage(err), "page /e/")
}

func TestTemplatesCheck_Redirect_FailsWithLocation(t *testing.T) {
	// The redirect target serves a valid page, so following the redirect
	// would hide the failure.
	tgt := newTestTarget(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/m/" {
			http.Redirect(w, r, "/login?continue=/m/", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(pageHTML("abc")))
	}))

	err := runCheck(t, templatesCheck([]string{"/m/"}), tgt)
	require.Error(t, err)
	assert.Contains(t, errorMessage(err), "GET /m/ returned 302")
	assert.Contains(t, errorMessage(err), "Location /login?continue=/m/")
}

func TestParamsetCheck_RequiredKeys(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"all keys", http.StatusOK, `{"dataframe":{"paramset":{"benchmark":["b"],"bot":["m"],"test":["t"],"extra":["x"]}}}`, ""},
		{"missing key", http.StatusOK, `{"dataframe":{"paramset":{"benchmark":["b"],"bot":["m"]}}}`, "paramset has no values for test"},
		{"empty values", http.StatusOK, `{"dataframe":{"paramset":{"benchmark":[],"bot":["m"],"test":["t"]}}}`, "paramset has no values for benchmark"},
		{"no dataframe", http.StatusOK, `{"dataframe":null}`, "paramset has no values for benchmark, bot, test"},
		{"not found", http.StatusNotFound, "404 page not found", "GET /_/initpage/ returned 404"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tgt := newTestTarget(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/_/initpage/" {
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))

			err := runCheck(t, paramsetCheck([]string{"benchmark", "bot", "test"}), tgt)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, errorMessage(err), tc.wantErr)
		})
	}
}

func TestHistoricalQueryCheck_FollowsProgressURL_Passes(t *testing.T) {
	bodies := make(chan map[string]any, 1)
	var polls atomic.Int32
	tgt := newTestTarget(t, frameHandler(bodies, &polls,
		runningProgress,
		`{"status":"Finished","messages":[],"results":{"dataframe":{"traceset":{",arch=x86_64,os=Ubuntu,":[1,2]}}},"url":"/_/status/abc"}`))

	require.NoError(t, runCheck(t, historicalQueryCheck("arch=x86_64&os=Ubuntu", testBegin, testEnd, testPollInterval), tgt))
	assert.Equal(t, int32(2), polls.Load())
	assert.Equal(t, map[string]any{
		"begin":   float64(testBegin.Unix()),
		"end":     float64(testEnd.Unix()),
		"queries": []any{"arch=x86_64&os=Ubuntu"},
		"tz":      "UTC",
	}, <-bodies)
}

func TestHistoricalQueryCheck_PerTargetQueryOverridesDefault(t *testing.T) {
	bodies := make(chan map[string]any, 1)
	var polls atomic.Int32
	tgt := newTestTarget(t, frameHandler(bodies, &polls,
		`{"status":"Finished","messages":[],"results":{"dataframe":{"traceset":{",benchmark=v8,":[1]}}},"url":"/_/status/abc"}`))
	tgt.frameQuery = "benchmark=v8&bot=linux-perf"

	require.NoError(t, runCheck(t, historicalQueryCheck("benchmark=default", testBegin, testEnd, testPollInterval), tgt))
	assert.Equal(t, map[string]any{
		"begin":   float64(testBegin.Unix()),
		"end":     float64(testEnd.Unix()),
		"queries": []any{"benchmark=v8&bot=linux-perf"},
		"tz":      "UTC",
	}, <-bodies)
}

func TestHistoricalQueryCheck_EmptyTraceSet_Fails(t *testing.T) {
	var polls atomic.Int32
	tgt := newTestTarget(t, frameHandler(make(chan map[string]any, 1), &polls,
		`{"status":"Finished","messages":[],"results":{"dataframe":{"traceset":{}}},"url":"/_/status/abc"}`))

	err := runCheck(t, historicalQueryCheck("arch=x86_64", testBegin, testEnd, testPollInterval), tgt)
	require.Error(t, err)
	assert.Contains(t, errorMessage(err), "empty traceset")
}

func TestHistoricalQueryCheck_ErrorStatus_FailsWithMessages(t *testing.T) {
	var polls atomic.Int32
	tgt := newTestTarget(t, frameHandler(make(chan map[string]any, 1), &polls,
		`{"status":"Error","messages":[{"key":"Error","value":"Failed to load data"}],"url":"/_/status/abc"}`))

	err := runCheck(t, historicalQueryCheck("arch=x86_64", testBegin, testEnd, testPollInterval), tgt)
	require.Error(t, err)
	assert.Contains(t, errorMessage(err), "frame request failed: Error: Failed to load data")
}

func TestHistoricalQueryCheck_NeverFinishes_TimesOut(t *testing.T) {
	var polls atomic.Int32
	tgt := newTestTarget(t, frameHandler(make(chan map[string]any, 1), &polls, runningProgress))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := historicalQueryCheck("arch=x86_64", testBegin, testEnd, testPollInterval).Run(ctx, tgt)
	require.Error(t, err)
	assert.Contains(t, errorMessage(err), "deadline")
	assert.Greater(t, polls.Load(), int32(1))
}

func TestRegCountCheck_Responses(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{"count", http.StatusOK, `{"Count":3}`, false},
		{"zero count", http.StatusOK, `{"Count":0}`, false},
		{"no Count field", http.StatusOK, `{}`, true},
		{"array", http.StatusOK, `[]`, true},
		{"not json", http.StatusOK, `<html></html>`, true},
		{"server error", http.StatusInternalServerError, `{"Count":1}`, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tgt := newTestTarget(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/_/reg/count" {
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))

			err := runCheck(t, regCountCheck(), tgt)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSheriffListCheck_Unauthorized_Fails(t *testing.T) {
	tgt := newTestTarget(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "User is not logged in or is not authorized.", http.StatusUnauthorized)
	}))

	err := runCheck(t, sheriffListCheck(), tgt)
	require.Error(t, err)
	assert.Contains(t, errorMessage(err), "GET /_/anomalies/sheriff_list returned 401")
}

func TestTarget_SendsHostAndForwardedProto(t *testing.T) {
	type seen struct{ host, proto string }
	got := make(chan seen, 1)
	tgt := newTestTarget(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got <- seen{host: r.Host, proto: r.Header.Get("X-Forwarded-Proto")}
	}))
	serverHost := tgt.baseURL[len("http://"):]

	require.NoError(t, runCheck(t, healthCheck("/readiness"), tgt))
	assert.Equal(t, seen{host: serverHost}, <-got)

	tgt.hostHeader = "chrome-perf-autopush.corp.goog"
	tgt.forwardedHTTPS = true
	require.NoError(t, runCheck(t, healthCheck("/readiness"), tgt))
	assert.Equal(t, seen{host: "chrome-perf-autopush.corp.goog", proto: "https"}, <-got)
}

func TestRunChecks_VersionFailure_SkipsRemaining(t *testing.T) {
	var healthCalls atomic.Int32
	tgt := newTestTarget(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			healthCalls.Add(1)
			return
		}
		_, _ = w.Write([]byte(pageHTML("old")))
	}))
	checks := []Check{versionCheck("abc", 0, testPollInterval), healthCheck("/healthz"), templatesCheck([]string{"/m/"})}
	for i := range checks {
		checks[i].Timeout = time.Second
	}

	results := runChecks(context.Background(), tgt, checks)
	require.Len(t, results, 3)
	assert.Equal(t, statusFail, results[0].Status)
	assert.Equal(t, `app_version is "old", want "abc"`, results[0].Error)
	assert.Equal(t, CheckResult{Name: checkHealth, Status: statusSkipped}, results[1])
	assert.Equal(t, CheckResult{Name: checkTemplates, Status: statusSkipped}, results[2])
	assert.Zero(t, healthCalls.Load())
}
