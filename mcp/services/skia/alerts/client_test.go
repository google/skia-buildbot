// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package alerts

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/am/go/incident"
	incident_mocks "go.skia.org/infra/am/go/incident/mocks"
	"go.skia.org/infra/am/go/note"
	"go.skia.org/infra/am/go/silence"
	silence_mocks "go.skia.org/infra/am/go/silence/mocks"
	"go.skia.org/infra/go/paramtools"
)

func TestActiveAlertsList_String(t *testing.T) {
	start := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC).Unix()
	list := ActiveAlertsList{
		{
			Incident: incident.Incident{
				ID:     "inc-1",
				Key:    "key-1",
				Active: true,
				Start:  start,
				Params: paramtools.Params{
					incident.ALERT_NAME: "JobDown",
					incident.ABBR:       "perf-fe",
					incident.SEVERITY:   "critical",
					incident.CATEGORY:   "infra",
					incident.OWNER:      "alice@google.com",
					"app":               "k8s-checker",
					"exported_app":      "perf-fe",
					"cluster":           "skia-infra-public",
					"exported_cluster":  "skia-infra-public",
					"job":               "cluster-pod-monitoring",
					"__state__":         "active",
					"link_to_source":    "https://prom2.skia.org/graph",
					"pod_template_hash": "12345",
					"description":       "perf-fe is down\ncheck logs | pods",
				},
				Notes: []note.Note{
					{Author: "alice@google.com", Text: "Looking", TS: start + 60},
				},
			},
			Silenced:               false,
			RecentlyExpiredSilence: true,
		},
		{
			Incident: incident.Incident{
				ID:     "inc-2",
				Key:    "key-2",
				Active: true,
				Start:  start,
				Params: paramtools.Params{
					incident.ALERT_NAME: "BotMissing",
					incident.ABBR:       "lin-1-e505",
					incident.SEVERITY:   "critical",
					incident.CATEGORY:   "skia",
					"bot":               "lin-1-e505",
					"os":                "Android",
					"gpu":               "none",
					"description":       "bot is missing",
				},
			},
		},
	}

	expected := `| ID | AlertName | Abbr | Severity | Category | Started | Owner | AssignedTo | Flags | cluster | exported_app |
|----|-----------|------|----------|----------|---------|-------|------------|-------|---------|--------------|
| inc-1 | JobDown | perf-fe | critical | infra | 2026-10-23T12:00:00Z | alice@google.com |  | RECENTLY_EXPIRED_SILENCE, NOTES(1) | skia-infra-public | perf-fe |

| ID | AlertName | Abbr | Severity | Category | Started | Owner | AssignedTo | Flags | bot |
|----|-----------|------|----------|----------|---------|-------|------------|-------|-----|
| inc-2 | BotMissing | lin-1-e505 | critical | skia | 2026-10-23T12:00:00Z |  |  |  | lin-1-e505 |
`
	require.Equal(t, expected, list.String())
	require.Equal(t, "No matching active alerts.", ActiveAlertsList{}.String())
}

func TestGetActiveAlertsHandler_FilteringAndSorting(t *testing.T) {
	start := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC).Unix()
	incidentStore := incident_mocks.NewStore(t)
	incidentStore.On("GetAll").Return([]incident.Incident{
		{
			ID:     "warn-assigned",
			Key:    "k1",
			Active: true,
			Start:  start,
			Params: paramtools.Params{
				incident.ALERT_NAME:  "TooManyGoRoutines",
				incident.ABBR:        "status",
				incident.SEVERITY:    "warning",
				incident.CATEGORY:    "infra",
				incident.OWNER:       "alice@google.com",
				incident.ASSIGNED_TO: "bob@google.com",
			},
		},
		{
			ID:     "crit-unassigned",
			Key:    "k2",
			Active: true,
			Start:  start,
			Params: paramtools.Params{
				incident.ALERT_NAME: "JobDown",
				incident.ABBR:       "autoroll",
				incident.SEVERITY:   "critical",
				incident.CATEGORY:   "infra",
			},
		},
		{
			ID:     "silenced-crit",
			Key:    "k3",
			Active: true,
			Start:  start,
			Params: paramtools.Params{
				incident.ALERT_NAME: "BotMissing",
				incident.ABBR:       "skia-e-linux-001",
				incident.SEVERITY:   "critical",
				incident.CATEGORY:   "skia",
				"bot":               "skia-e-linux-001",
			},
		},
		{
			ID:     "warn-unassigned",
			Key:    "k4",
			Active: true,
			Start:  start,
			Params: paramtools.Params{
				incident.ALERT_NAME: "StaleK8sImage",
				incident.ABBR:       "gold",
				incident.SEVERITY:   "warning",
				incident.CATEGORY:   "infra",
				incident.OWNER:      "alice@google.com",
			},
		},
	}, nil)

	silenceStore := silence_mocks.NewStore(t)
	silenceStore.On("GetAll").Return([]silence.Silence{
		{
			Key:    "sil-1",
			Active: true,
			ParamSet: paramtools.ParamSet{
				incident.ALERT_NAME: []string{"BotMissing"},
				"bot":               []string{"skia-e-linux-.*"},
			},
		},
	}, nil)
	silenceStore.On("GetRecentlyArchived", silence.RECENTLY_EXPIRED_DURATION).Return(nil, nil)

	client := &AlertsClient{
		incidentStore: incidentStore,
		silenceStore:  silenceStore,
	}
	ctx := context.Background()

	t.Run("default excludes silenced and sorts by severity then assignment", func(t *testing.T) {
		res, err := client.GetActiveAlertsHandler(ctx, mcp.CallToolRequest{})
		require.NoError(t, err)
		list, ok := res.(ActiveAlertsList)
		require.True(t, ok)
		require.Len(t, list, 3)
		require.Equal(t, "crit-unassigned", list[0].Incident.ID)
		require.Equal(t, "warn-unassigned", list[1].Incident.ID)
		require.Equal(t, "warn-assigned", list[2].Incident.ID)
	})

	t.Run("include silenced places silenced alerts at the end", func(t *testing.T) {
		res, err := client.GetActiveAlertsHandler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					argIncludeSilenced: true,
				},
			},
		})
		require.NoError(t, err)
		list := res.(ActiveAlertsList)
		require.Len(t, list, 4)
		require.Equal(t, "silenced-crit", list[3].Incident.ID)
		require.True(t, list[3].Silenced)
	})

	t.Run("exclude assigned and filter by severity", func(t *testing.T) {
		res, err := client.GetActiveAlertsHandler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					argIncludeAssigned: false,
					argSeverity:        "warning",
				},
			},
		})
		require.NoError(t, err)
		list := res.(ActiveAlertsList)
		require.Len(t, list, 1)
		require.Equal(t, "warn-unassigned", list[0].Incident.ID)
	})

	t.Run("filter by assigned_to matches explicitly assigned", func(t *testing.T) {
		res, err := client.GetActiveAlertsHandler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					argAssignedTo: "bob@google.com",
				},
			},
		})
		require.NoError(t, err)
		list := res.(ActiveAlertsList)
		require.Len(t, list, 1)
		require.Equal(t, "warn-assigned", list[0].Incident.ID)
	})

	t.Run("filter by assigned_to does not match owner", func(t *testing.T) {
		res, err := client.GetActiveAlertsHandler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					argAssignedTo: "alice@google.com",
				},
			},
		})
		require.NoError(t, err)
		list := res.(ActiveAlertsList)
		require.NotNil(t, list)
		require.Empty(t, list)
	})

	t.Run("filter by owner matches all alerts with owner", func(t *testing.T) {
		res, err := client.GetActiveAlertsHandler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					argOwner: "alice@google.com",
				},
			},
		})
		require.NoError(t, err)
		list := res.(ActiveAlertsList)
		require.Len(t, list, 2)
		require.Equal(t, "warn-unassigned", list[0].Incident.ID)
		require.Equal(t, "warn-assigned", list[1].Incident.ID)
	})
}

func TestGetAlertDetailsHandler(t *testing.T) {
	baseTS := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC).Unix()

	// Create 10 short-lived historical occurrences so it is classified as flaky.
	var resolved []incident.Incident
	for i := 0; i < 10; i++ {
		s := baseTS - int64((i+1)*3600)
		resolved = append(resolved, incident.Incident{
			ID:       "alert-123",
			Key:      fmt.Sprintf("hist-%d", i),
			Active:   false,
			Start:    s,
			LastSeen: s + 120, // 2m duration (< 600s threshold)
		})
	}

	t.Run("active alert with history_limit < 20", func(t *testing.T) {
		incidentStore := incident_mocks.NewStore(t)
		incidentStore.On("GetAll").Return([]incident.Incident{
			{
				ID:       "alert-123",
				Key:      "active-key",
				Active:   true,
				Start:    baseTS,
				LastSeen: baseTS + 300,
				Params: paramtools.Params{
					incident.ALERT_NAME: "JobDown",
					incident.ABBR:       "perf",
					incident.SEVERITY:   "critical",
				},
				Notes: []note.Note{
					{Author: "dev@google.com", Text: "Investigating", TS: baseTS + 60},
				},
			},
		}, nil)
		incidentStore.On("GetRecentlyResolvedForID", "alert-123", "active-key", incident.FlakyNumThreshold).Return(resolved, nil)

		silenceStore := silence_mocks.NewStore(t)
		silenceStore.On("GetAll").Return(nil, nil)
		silenceStore.On("GetRecentlyArchived", silence.RECENTLY_EXPIRED_DURATION).Return([]silence.Silence{
			{
				Key:      "arch-sil",
				Active:   false,
				User:     "gardener@google.com",
				Created:  baseTS - 7200,
				Duration: "2h",
				ParamSet: paramtools.ParamSet{
					incident.ALERT_NAME: []string{"JobDown"},
				},
				Notes: []note.Note{
					{Author: "gardener@google.com", Text: "Silenced for maintenance", TS: baseTS - 7200},
				},
			},
		}, nil)

		client := &AlertsClient{
			incidentStore: incidentStore,
			silenceStore:  silenceStore,
		}
		res, err := client.GetAlertDetailsHandler(context.Background(), mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					argAlertID:      "alert-123",
					argHistoryLimit: 2,
				},
			},
		})
		require.NoError(t, err)
		details, ok := res.(*AlertDetails)
		require.True(t, ok)
		require.Equal(t, "alert-123", details.Incident.ID)
		require.True(t, details.Incident.Active)
		require.False(t, details.Silenced)
		require.True(t, details.RecentlyExpiredSilence)
		require.True(t, details.Flaky)
		require.Len(t, details.MatchingSilences, 1)
		require.Len(t, details.History, 2)

		md := details.String()
		require.Contains(t, md, "**ID:** alert-123")
		require.Contains(t, md, "**Flaky:** true")
		require.Contains(t, md, "**Recently Expired Silence:** true")
		require.Contains(t, md, "Silenced for maintenance")
		require.Contains(t, md, "### Recent History (2 occurrences)")
	})

	t.Run("resolved alert with 10 occurrences and history_limit > 20", func(t *testing.T) {
		incidentStore := incident_mocks.NewStore(t)
		incidentStore.On("GetAll").Return(nil, nil)
		incidentStore.On("GetRecentlyResolvedForID", "alert-123", "", 50).Return(resolved, nil)

		silenceStore := silence_mocks.NewStore(t)
		silenceStore.On("GetAll").Return(nil, nil)
		silenceStore.On("GetRecentlyArchived", silence.RECENTLY_EXPIRED_DURATION).Return(nil, nil)

		client := &AlertsClient{
			incidentStore: incidentStore,
			silenceStore:  silenceStore,
		}
		res, err := client.GetAlertDetailsHandler(context.Background(), mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					argAlertID:      "alert-123",
					argHistoryLimit: 50,
				},
			},
		})
		require.NoError(t, err)
		details, ok := res.(*AlertDetails)
		require.True(t, ok)
		require.Equal(t, "hist-0", details.Incident.Key)
		require.False(t, details.Incident.Active)
		require.True(t, details.Flaky)
		require.Len(t, details.History, 9)
	})
}
