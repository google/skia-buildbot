// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/gcs/mem_gcsclient"
)

func TestVerdict_MarshalJSON_MatchesSchema(t *testing.T) {
	v := newVerdict(
		"chrome-internal-autopush",
		"http://perf-fe-chrome-non-public-autopush.perf.svc.cluster.local:8000",
		"70e8b73d508ccfa242a67b29f8dd4647a67c3653",
		time.Date(2026, 12, 1, 17, 0, 0, 500_000_000, time.UTC),
		time.Date(2026, 12, 1, 17, 0, 41, 900_000_000, time.UTC),
		[]CheckResult{{Name: checkHistoricalQuery, Status: statusPass, LatencyMS: 2140}},
		&Verdict{
			Status:            statusPass,
			AppVersion:        "70e8b73d508ccfa242a67b29f8dd4647a67c3653",
			ConsecutivePasses: 5,
		},
	)
	path := verdictPath(t.TempDir(), v.Instance)
	require.NoError(t, writeVerdict(path, v))

	const wantJSON = `{
	  "instance": "chrome-internal-autopush",
	  "base_url": "http://perf-fe-chrome-non-public-autopush.perf.svc.cluster.local:8000",
	  "app_version": "70e8b73d508ccfa242a67b29f8dd4647a67c3653",
	  "status": "pass",
	  "consecutive_passes": 6,
	  "started_at": "2026-12-01T17:00:00Z",
	  "finished_at": "2026-12-01T17:00:41Z",
	  "checks": [{"name": "historical_query", "status": "pass", "latency_ms": 2140}]
	}`
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, wantJSON, string(raw))
}

func TestNewVerdict_ConsecutivePasses(t *testing.T) {
	passChecks := []CheckResult{
		{Name: checkVersion, Status: statusPass, LatencyMS: 10},
		{Name: checkHealth, Status: statusPass, LatencyMS: 5},
	}
	failChecks := []CheckResult{
		{Name: checkVersion, Status: statusPass, LatencyMS: 10},
		{Name: checkHealth, Status: statusFail, LatencyMS: 5, Error: "503"},
	}
	start := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Second)

	tests := []struct {
		name       string
		appVersion string
		checks     []CheckResult
		prev       *Verdict
		wantStatus string
		wantStreak int
	}{
		{
			name:       "no previous",
			appVersion: "hash-a",
			checks:     passChecks,
			prev:       nil,
			wantStatus: statusPass,
			wantStreak: 1,
		},
		{
			name:       "previous pass with the same version",
			appVersion: "hash-a",
			checks:     passChecks,
			prev:       &Verdict{Status: statusPass, AppVersion: "hash-a", ConsecutivePasses: 3},
			wantStatus: statusPass,
			wantStreak: 4,
		},
		{
			name:       "previous pass with another version",
			appVersion: "hash-b",
			checks:     passChecks,
			prev:       &Verdict{Status: statusPass, AppVersion: "hash-a", ConsecutivePasses: 6},
			wantStatus: statusPass,
			wantStreak: 1,
		},
		{
			name:       "previous fail",
			appVersion: "hash-a",
			checks:     passChecks,
			prev:       &Verdict{Status: statusFail, AppVersion: "hash-a", ConsecutivePasses: 0},
			wantStatus: statusPass,
			wantStreak: 1,
		},
		{
			name:       "current fail",
			appVersion: "hash-a",
			checks:     failChecks,
			prev:       &Verdict{Status: statusPass, AppVersion: "hash-a", ConsecutivePasses: 6},
			wantStatus: statusFail,
			wantStreak: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := newVerdict("chrome-internal-autopush", "http://localhost:18000", tc.appVersion, start, end, tc.checks, tc.prev)
			assert.Equal(t, tc.wantStatus, got.Status)
			assert.Equal(t, tc.wantStreak, got.ConsecutivePasses)
		})
	}
}

func TestWriteReadVerdict_RoundTrip(t *testing.T) {
	want := newVerdict(
		"v8-internal-autopush",
		"http://localhost:18001",
		"0830cc0646bf9ce301e7fac1a3c9339f5503e90b",
		time.Date(2026, 11, 4, 17, 0, 0, 123456, time.UTC),
		time.Date(2026, 11, 4, 17, 0, 12, 654321, time.UTC),
		[]CheckResult{
			{Name: checkVersion, Status: statusPass, LatencyMS: 120},
			{Name: checkHealth, Status: statusFail, LatencyMS: 45, Error: "GET /readiness returned 503"},
		},
		nil,
	)
	path := verdictPath(t.TempDir(), want.Instance)
	require.NoError(t, writeVerdict(path, want))

	got, err := readVerdict(path)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want, *got)
}

func TestReadVerdict_Missing_ReturnsNil(t *testing.T) {
	got, err := readVerdict(verdictPath(t.TempDir(), "does-not-exist"))
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestVerdictStore_GCS_RoundTripAndMissing(t *testing.T) {
	memGCS := mem_gcsclient.New("skia-perf-smoketest")
	store, err := newVerdictStore(t.Context(), "gs://skia-perf-smoketest/perfserver", false, memGCS)
	require.NoError(t, err)

	missing, err := store.read(t.Context(), "chrome-internal-autopush")
	require.NoError(t, err)
	assert.Nil(t, missing)

	want := newVerdict(
		"chrome-internal-autopush",
		"http://perf-fe-chrome-non-public-autopush.perf.svc.cluster.local:8000",
		"70e8b73d508ccfa242a67b29f8dd4647a67c3653",
		time.Date(2026, 11, 12, 17, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 12, 17, 0, 5, 0, time.UTC),
		[]CheckResult{{Name: checkVersion, Status: statusPass, LatencyMS: 50}},
		nil,
	)
	require.NoError(t, store.write(t.Context(), want))

	exists, err := memGCS.DoesFileExist(t.Context(), "perfserver/chrome-internal-autopush.json")
	require.NoError(t, err)
	assert.True(t, exists)

	got, err := store.read(t.Context(), "chrome-internal-autopush")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want, *got)
}

func TestVerdictStore_GCSHTTPClient_Read(t *testing.T) {
	existing := newVerdict(
		"chrome-internal-autopush",
		"http://perf-fe-chrome-non-public-autopush.perf.svc.cluster.local:8000",
		"70e8b73d508ccfa242a67b29f8dd4647a67c3653",
		time.Date(2026, 11, 27, 22, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 27, 22, 0, 5, 0, time.UTC),
		[]CheckResult{{Name: checkVersion, Status: statusPass, LatencyMS: 50}},
		nil,
	)
	existingBody, err := json.Marshal(existing)
	require.NoError(t, err)

	var missingHits, existingHits, forbiddenHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		switch {
		case strings.HasSuffix(r.URL.Path, "/perfserver/missing-instance.json"):
			missingHits.Add(1)
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<?xml version='1.0' encoding='UTF-8'?><Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message></Error>`))
		case strings.HasSuffix(r.URL.Path, "/perfserver/chrome-internal-autopush.json"):
			existingHits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(existingBody)
		case strings.HasSuffix(r.URL.Path, "/perfserver/forbidden-instance.json"):
			forbiddenHits.Add(1)
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`<?xml version='1.0' encoding='UTF-8'?><Error><Code>AccessDenied</Code><Message>Access denied.</Message></Error>`))
		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)

	httpClient := newGCSHTTPClient(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}))
	storageClient, err := storage.NewClient(
		t.Context(),
		option.WithEndpoint(srv.URL),
		option.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = storageClient.Close() })

	store := &verdictStore{
		gcsClient: gcsclient.New(storageClient, "test-bucket"),
		gcsPrefix: "perfserver",
	}

	missing, err := store.read(t.Context(), "missing-instance")
	require.NoError(t, err)
	assert.Nil(t, missing)
	assert.Equal(t, int32(1), missingHits.Load())

	got, err := store.read(t.Context(), "chrome-internal-autopush")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, existing, *got)
	assert.Equal(t, int32(1), existingHits.Load())

	forbidden, err := store.read(t.Context(), "forbidden-instance")
	require.Error(t, err)
	assert.Nil(t, forbidden)
	assert.Contains(t, err.Error(), "reading verdict gs://test-bucket/perfserver/forbidden-instance.json")
	assert.Equal(t, int32(1), forbiddenHits.Load())
}
