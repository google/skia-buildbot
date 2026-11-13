// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeStagesFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stages.json")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

func TestLatestPerfserverHash_ReadsLatestStage(t *testing.T) {
	path := writeStagesFixture(t, `{
	  "images": {
	    "gcr.io/skia-public/perfserver": {
	      "stages": {
	        "latest": {"git_hash": "latest-hash-1111111111111111111111111111", "digest": "sha256:aaa"},
	        "prod": {"git_hash": "prod-hash-222222222222222222222222222222", "digest": "sha256:bbb"}
	      }
	    }
	  }
	}`)

	got, err := latestPerfserverHash(path)
	require.NoError(t, err)
	assert.Equal(t, "latest-hash-1111111111111111111111111111", got)
}

func TestLatestPerfserverHash_MissingImage_Errors(t *testing.T) {
	path := writeStagesFixture(t, `{"images": {"gcr.io/skia-public/other": {"stages": {}}}}`)

	_, err := latestPerfserverHash(path)
	require.Error(t, err)
	assert.Contains(t, errorMessage(err), perfserverImage)
}

func TestEvaluateGate(t *testing.T) {
	const latest = "0830cc0646bf9ce301e7fac1a3c9339f5503e90b"
	now := time.Date(2026, 11, 4, 18, 0, 0, 0, time.UTC)
	fresh := now.Add(-5 * time.Minute)
	instances := []string{"chrome-internal-autopush", "v8-internal-autopush"}

	healthyVerdict := func(inst string) *Verdict {
		return &Verdict{
			Instance:          inst,
			AppVersion:        latest,
			Status:            statusPass,
			ConsecutivePasses: 3,
			FinishedAt:        fresh,
		}
	}

	tests := []struct {
		name           string
		mutate         func(map[string]*Verdict)
		wantPass       bool
		wantBlockedIdx int
		wantReasonSubs []string
	}{
		{
			name:     "pass",
			mutate:   func(map[string]*Verdict) {},
			wantPass: true,
		},
		{
			name: "red",
			mutate: func(m map[string]*Verdict) {
				m["chrome-internal-autopush"].Status = statusFail
				m["chrome-internal-autopush"].ConsecutivePasses = 0
			},
			wantPass:       false,
			wantBlockedIdx: 0,
			wantReasonSubs: []string{`status is "fail", want "pass"`},
		},
		{
			name: "stale",
			mutate: func(m map[string]*Verdict) {
				m["chrome-internal-autopush"].FinishedAt = now.Add(-20 * time.Minute)
			},
			wantPass:       false,
			wantBlockedIdx: 0,
			wantReasonSubs: []string{"verdict age 20m0s > max_age 15m0s"},
		},
		{
			name: "mismatch",
			mutate: func(m map[string]*Verdict) {
				m["chrome-internal-autopush"].AppVersion = "0000000000000000000000000000000000000000"
			},
			wantPass:       false,
			wantBlockedIdx: 0,
			wantReasonSubs: []string{"app_version 0000000000 != latest 0830cc0646"},
		},
		{
			name: "missing",
			mutate: func(m map[string]*Verdict) {
				delete(m, "v8-internal-autopush")
			},
			wantPass:       false,
			wantBlockedIdx: 1,
			wantReasonSubs: []string{"missing verdict"},
		},
		{
			name: "short_streak",
			mutate: func(m map[string]*Verdict) {
				m["chrome-internal-autopush"].ConsecutivePasses = 2
			},
			wantPass:       false,
			wantBlockedIdx: 0,
			wantReasonSubs: []string{"consecutive_passes 2 < 3"},
		},
		{
			name: "multiple_reasons",
			mutate: func(m map[string]*Verdict) {
				v := m["chrome-internal-autopush"]
				v.Status = statusFail
				v.AppVersion = "0000000000000000000000000000000000000000"
				v.ConsecutivePasses = 0
				v.FinishedAt = now.Add(-20 * time.Minute)
			},
			wantPass:       false,
			wantBlockedIdx: 0,
			wantReasonSubs: []string{
				`status is "fail", want "pass"`,
				"app_version 0000000000 != latest 0830cc0646",
				"consecutive_passes 0 < 3",
				"verdict age 20m0s > max_age 15m0s",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			verdicts := map[string]*Verdict{
				"chrome-internal-autopush": healthyVerdict("chrome-internal-autopush"),
				"v8-internal-autopush":     healthyVerdict("v8-internal-autopush"),
			}
			tc.mutate(verdicts)

			decisions, overallPass := evaluateGate(latest, instances, verdicts, 3, 15*time.Minute, now)
			require.Len(t, decisions, 2)
			assert.Equal(t, tc.wantPass, overallPass)

			var out bytes.Buffer
			printGateTable(&out, latest, decisions, overallPass)
			if tc.wantPass {
				assert.Equal(t, gatePass, decisions[0].Decision)
				assert.Equal(t, gatePass, decisions[1].Decision)
				assert.Contains(t, out.String(), "OVERALL: PASS")
				return
			}
			blocked := decisions[tc.wantBlockedIdx]
			assert.Equal(t, gateBlock, blocked.Decision)
			for _, sub := range tc.wantReasonSubs {
				assert.Contains(t, out.String(), sub)
			}
			assert.Contains(t, out.String(), "OVERALL: BLOCK")
		})
	}
}
