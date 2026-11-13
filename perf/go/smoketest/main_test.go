// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/mem_gcsclient"
)

func TestParseTargets(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		targets, err := parseTargets(
			[]string{
				"chrome-internal-autopush=http://localhost:18000/",
				"v8-internal-autopush=https://v8-perf-autopush.corp.goog",
			},
			"chrome-perf-autopush.corp.goog",
			true,
		)
		require.NoError(t, err)
		require.Len(t, targets, 2)
		assert.Equal(t, "chrome-internal-autopush", targets[0].name)
		assert.Equal(t, "http://localhost:18000", targets[0].baseURL)
		assert.Equal(t, "chrome-perf-autopush.corp.goog", targets[0].hostHeader)
		assert.True(t, targets[0].forwardedHTTPS)
		assert.Equal(t, "v8-internal-autopush", targets[1].name)
		assert.Equal(t, "https://v8-perf-autopush.corp.goog", targets[1].baseURL)
	})

	errorCases := []struct {
		name    string
		specs   []string
		wantErr string
	}{
		{
			name:    "missing =",
			specs:   []string{"http://localhost:18000"},
			wantErr: "name=url[,host=...][,query=...]",
		},
		{
			name:    "bad name with slash",
			specs:   []string{"../escape=http://localhost:18000"},
			wantErr: "must match ^[a-z0-9-]+$",
		},
		{
			name:    "bad URL",
			specs:   []string{"chrome=://bad-url"},
			wantErr: "invalid target url",
		},
		{
			name:    "duplicate",
			specs:   []string{"chrome=http://localhost:18000", "chrome=http://localhost:18001"},
			wantErr: `duplicate target name "chrome"`,
		},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseTargets(tc.specs, "", false)
			require.Error(t, err)
			assert.Contains(t, errorMessage(err), tc.wantErr)
		})
	}
}

func TestParseTargets_WithPerTargetQuery(t *testing.T) {
	targets, err := parseTargets(
		[]string{
			"chrome-internal-autopush=http://localhost:18000,host=chrome-perf-autopush.corp.goog,query=benchmark=speedometer3&bot=linux-perf",
			"v8-internal-autopush=http://localhost:18001,query=benchmark=v8,host=v8-perf-autopush.corp.goog",
			"skia-public-autopush=http://localhost:18002",
		},
		"default-host.corp.goog",
		false,
	)
	require.NoError(t, err)
	require.Len(t, targets, 3)
	assert.Equal(t, "http://localhost:18000", targets[0].baseURL)
	assert.Equal(t, "chrome-perf-autopush.corp.goog", targets[0].hostHeader)
	assert.Equal(t, "benchmark=speedometer3&bot=linux-perf", targets[0].frameQuery)
	assert.Equal(t, "http://localhost:18001", targets[1].baseURL)
	assert.Equal(t, "v8-perf-autopush.corp.goog", targets[1].hostHeader)
	assert.Equal(t, "benchmark=v8", targets[1].frameQuery)
	assert.Equal(t, "http://localhost:18002", targets[2].baseURL)
	assert.Equal(t, "default-host.corp.goog", targets[2].hostHeader)
	assert.Empty(t, targets[2].frameQuery)
}

func TestParseFrameWindow(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		begin, end, err := parseFrameWindow("2026-10-27T00:00:00Z", "2026-10-28T00:00:00Z")
		require.NoError(t, err)
		assert.Equal(t, 24*time.Hour, end.Sub(begin))
	})

	errorCases := []struct {
		name    string
		begin   string
		end     string
		wantErr string
	}{
		{
			name:    "end not after begin",
			begin:   "2026-10-28T00:00:00Z",
			end:     "2026-10-28T00:00:00Z",
			wantErr: "must be after",
		},
		{
			name:    "over 24h",
			begin:   "2026-10-27T00:00:00Z",
			end:     "2026-10-28T00:00:01Z",
			wantErr: "exceeds 24h0m0s maximum",
		},
		{
			name:    "bad RFC 3339",
			begin:   "2026-10-27",
			end:     "2026-10-28T00:00:00Z",
			wantErr: "parsing --frame_begin",
		},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseFrameWindow(tc.begin, tc.end)
			require.Error(t, err)
			assert.Contains(t, errorMessage(err), tc.wantErr)
		})
	}
}

func TestBuildChecks(t *testing.T) {
	stagesPath := writeStagesFixture(t, `{
	  "images": {
	    "gcr.io/skia-public/perfserver": {
	      "stages": {
	        "latest": {"git_hash": "0830cc0646bf9ce301e7fac1a3c9339f5503e90b", "digest": "sha256:aaa"}
	      }
	    }
	  }
	}`)

	t.Run("valid default checks", func(t *testing.T) {
		checks, err := buildChecks(runConfig{
			checkNames:     defaultChecks,
			stagesFile:     stagesPath,
			pages:          defaultPages,
			requiredParams: defaultRequiredParams,
			healthPath:     "/readiness",
			frameQuery:     "benchmark=v8&bot=MacM4",
			frameBegin:     "2026-10-27T00:00:00Z",
			frameEnd:       "2026-10-28T00:00:00Z",
			timeout:        60 * time.Second,
		})
		require.NoError(t, err)
		require.Len(t, checks, 6)
	})

	errorCases := []struct {
		name    string
		cfg     runConfig
		wantErr string
	}{
		{
			name: "unknown name",
			cfg: runConfig{
				checkNames: []string{"nonexistent"},
			},
			wantErr: `unknown check "nonexistent"`,
		},
		{
			name: "historical_query without frame flags",
			cfg: runConfig{
				checkNames: []string{checkHistoricalQuery},
			},
			wantErr: "--frame_query is required",
		},
		{
			name: "version without a hash source",
			cfg: runConfig{
				checkNames: []string{checkVersion},
			},
			wantErr: "requires exactly one of --expected_git_hash or --stages_file",
		},
		{
			name: "both hash sources",
			cfg: runConfig{
				checkNames:      []string{checkVersion},
				expectedGitHash: "abc",
				stagesFile:      stagesPath,
			},
			wantErr: "requires exactly one of --expected_git_hash or --stages_file",
		},
		{
			name: "negative rollout_timeout",
			cfg: runConfig{
				checkNames:      []string{checkVersion},
				expectedGitHash: "abc",
				rolloutTimeout:  -time.Second,
			},
			wantErr: "--rollout_timeout must be >= 0",
		},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildChecks(tc.cfg)
			require.Error(t, err)
			assert.Contains(t, errorMessage(err), tc.wantErr)
		})
	}
}

func TestRunCmd(t *testing.T) {
	const expectedHash = "70e8b73d508ccfa242a67b29f8dd4647a67c3653"

	t.Run("pass_writes_verdict_and_table", func(t *testing.T) {
		feSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/healthz":
				_, _ = w.Write([]byte("ok"))
			default:
				_, _ = w.Write([]byte(pageHTML(expectedHash)))
			}
		}))
		t.Cleanup(feSrv.Close)

		memGCS := mem_gcsclient.New("skia-perf-smoketest")
		var out, errOut bytes.Buffer
		code := runCLIWithDeps(t.Context(), []string{
			"run",
			"--expected_git_hash=" + expectedHash,
			"--checks=version,health",
			"--verdict_dir=gs://skia-perf-smoketest/perfserver",
			"--target=chrome-internal-autopush=" + feSrv.URL,
		}, &out, &errOut, memGCS)
		require.Equal(t, 0, code, "stderr: %s", errOut.String())
		assert.Contains(t, out.String(), "INSTANCE")
		assert.Contains(t, out.String(), "VERDICT: instance=chrome-internal-autopush status=pass app_version=70e8b73d50 consecutive_passes=1")

		store, err := newVerdictStore(t.Context(), "gs://skia-perf-smoketest/perfserver", true, memGCS)
		require.NoError(t, err)
		got, err := store.read(t.Context(), "chrome-internal-autopush")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, statusPass, got.Status)
		assert.Equal(t, expectedHash, got.AppVersion)
		assert.Equal(t, 1, got.ConsecutivePasses)
	})

	t.Run("check_failure_returns_1_and_writes_fail_verdict", func(t *testing.T) {
		feSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" {
				http.Error(w, "unhealthy", http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(pageHTML(expectedHash)))
		}))
		t.Cleanup(feSrv.Close)

		vDir := t.TempDir()
		var out, errOut bytes.Buffer
		code := runCLI(t.Context(), []string{
			"run",
			"--expected_git_hash=" + expectedHash,
			"--checks=version,health",
			"--verdict_dir=" + vDir,
			"--target=chrome-internal-autopush=" + feSrv.URL,
		}, &out, &errOut)
		assert.Equal(t, 1, code)
		assert.Contains(t, out.String(), "VERDICT: instance=chrome-internal-autopush status=fail app_version=70e8b73d50 consecutive_passes=0")

		got, err := readVerdict(verdictPath(vDir, "chrome-internal-autopush"))
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, statusFail, got.Status)
		assert.Equal(t, 0, got.ConsecutivePasses)
	})

	t.Run("cli_and_setup_errors_return_2", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			args    []string
			wantErr string
		}{
			{
				name:    "no_args",
				args:    nil,
				wantErr: "usage: perf-smoketest",
			},
			{
				name:    "unknown_command",
				args:    []string{"bogus"},
				wantErr: `unknown command "bogus"`,
			},
			{
				name:    "missing_verdict_dir",
				args:    []string{"run", "--target=chrome=http://localhost:18000"},
				wantErr: "--verdict_dir is required",
			},
			{
				name:    "missing_target",
				args:    []string{"run", "--verdict_dir=" + t.TempDir()},
				wantErr: "--target is required",
			},
			{
				name: "negative_rollout_timeout",
				args: []string{
					"run",
					"--verdict_dir=" + t.TempDir(),
					"--target=chrome=http://localhost:18000",
					"--expected_git_hash=" + expectedHash,
					"--checks=version",
					"--rollout_timeout=-1s",
				},
				wantErr: "--rollout_timeout must be >= 0",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var out, errOut bytes.Buffer
				code := runCLI(t.Context(), tc.args, &out, &errOut)
				assert.Equal(t, 2, code)
				assert.Contains(t, errOut.String(), tc.wantErr)
			})
		}
	})
}

func TestGateCmd_DefaultThresholds(t *testing.T) {
	const latest = "0830cc0646bf9ce301e7fac1a3c9339f5503e90b"
	stagesPath := writeStagesFixture(t, `{
	  "images": {
	    "gcr.io/skia-public/perfserver": {
	      "stages": {
	        "latest": {"git_hash": "`+latest+`", "digest": "sha256:aaa"}
	      }
	    }
	  }
	}`)

	t.Run("streak_3_and_age_5m_passes", func(t *testing.T) {
		vDir := t.TempDir()
		require.NoError(t, writeVerdict(verdictPath(vDir, "chrome-internal-autopush"), Verdict{
			Instance:          "chrome-internal-autopush",
			AppVersion:        latest,
			Status:            statusPass,
			ConsecutivePasses: 3,
			FinishedAt:        time.Now().UTC().Add(-5 * time.Minute),
		}))
		var out, errOut bytes.Buffer
		code := runCLI(t.Context(), []string{
			"gate",
			"--stages_file=" + stagesPath,
			"--verdict_dir=" + vDir,
			"--instances=chrome-internal-autopush",
		}, &out, &errOut)
		assert.Equal(t, 0, code)
		assert.Contains(t, out.String(), "OVERALL: PASS")
	})

	t.Run("age_20m_blocks_under_default_15m_max_age", func(t *testing.T) {
		vDir := t.TempDir()
		require.NoError(t, writeVerdict(verdictPath(vDir, "chrome-internal-autopush"), Verdict{
			Instance:          "chrome-internal-autopush",
			AppVersion:        latest,
			Status:            statusPass,
			ConsecutivePasses: 5,
			FinishedAt:        time.Now().UTC().Add(-20 * time.Minute),
		}))
		var out, errOut bytes.Buffer
		code := runCLI(t.Context(), []string{
			"gate",
			"--stages_file=" + stagesPath,
			"--verdict_dir=" + vDir,
			"--instances=chrome-internal-autopush",
		}, &out, &errOut)
		assert.Equal(t, 1, code)
		assert.Contains(t, out.String(), "max_age 15m0s")
		assert.Contains(t, out.String(), "OVERALL: BLOCK")
	})
}
