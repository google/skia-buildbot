// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPollInterval = 5 * time.Millisecond

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
