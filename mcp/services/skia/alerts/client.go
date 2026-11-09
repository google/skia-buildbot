// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package alerts

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/am/go/incident"
	"go.skia.org/infra/am/go/note"
	"go.skia.org/infra/am/go/silence"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/timer"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// AlertsClient provides MCP handlers for querying Alert Manager state.
type AlertsClient struct {
	incidentStore incident.Store
	silenceStore  silence.Store
}

// NewClient returns a new AlertsClient backed by Cloud Datastore.
func NewClient(ctx context.Context) (*AlertsClient, error) {
	ts, err := google.DefaultTokenSource(ctx, datastore.ScopeDatastore)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := ds.InitWithOpt(ds.ALERT_MANAGER_PROJECT, ds.ALERT_MANAGER_NS, option.WithTokenSource(ts)); err != nil {
		return nil, skerr.Wrap(err)
	}
	return &AlertsClient{
		incidentStore: incident.NewStore(ds.DS, nil),
		silenceStore:  silence.NewStore(ds.DS),
	}, nil
}

// GetActiveAlertsHandler handles the get_active_alerts MCP tool call.
func (c *AlertsClient) GetActiveAlertsHandler(ctx context.Context, req mcp.CallToolRequest) (fmt.Stringer, error) {
	defer timer.New("GetActiveAlertsHandler").Stop()

	category := req.GetString(argCategory, "")
	severity := req.GetString(argSeverity, "")
	alertName := req.GetString(argAlertName, "")
	assignedToFilter := req.GetString(argAssignedTo, "")
	ownerFilter := req.GetString(argOwner, "")
	includeSilenced := req.GetBool(argIncludeSilenced, false)
	includeAssigned := req.GetBool(argIncludeAssigned, true)

	incidents, err := c.incidentStore.GetAll()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	activeSilences, err := c.silenceStore.GetAll()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	archivedSilences, err := c.silenceStore.GetRecentlyArchived(silence.RECENTLY_EXPIRED_DURATION)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	summaries := make([]AlertSummary, 0, len(incidents))
	for _, in := range incidents {
		if !in.Active {
			continue
		}
		if category != "" && !strings.EqualFold(in.Params[incident.CATEGORY], category) {
			continue
		}
		if severity != "" && !strings.EqualFold(in.Params[incident.SEVERITY], severity) {
			continue
		}
		if alertName != "" && !strings.EqualFold(in.Params[incident.ALERT_NAME], alertName) {
			continue
		}
		if ownerFilter != "" && !strings.EqualFold(in.Params[incident.OWNER], ownerFilter) {
			continue
		}

		silenced := in.IsSilenced(activeSilences, true)
		if silenced && !includeSilenced {
			continue
		}

		assignedTo := in.Params[incident.ASSIGNED_TO]
		if assignedToFilter != "" {
			if !strings.EqualFold(assignedTo, assignedToFilter) {
				continue
			}
		} else if assignedTo != "" && !includeAssigned {
			continue
		}

		recentlyExpired := false
		if len(archivedSilences) > 0 {
			recentlyExpired = in.IsSilenced(archivedSilences, false)
		}

		summaries = append(summaries, AlertSummary{
			Incident:               in,
			Silenced:               silenced,
			RecentlyExpiredSilence: recentlyExpired,
		})
	}

	sortAlertSummaries(summaries)
	return ActiveAlertsList(summaries), nil
}

func severityRank(sev string) int {
	switch strings.ToLower(sev) {
	case "critical":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

func sortAlertSummaries(summaries []AlertSummary) {
	sort.Slice(summaries, func(i, j int) bool {
		a, b := summaries[i], summaries[j]
		if a.Silenced != b.Silenced {
			return !a.Silenced
		}
		rankA, rankB := severityRank(a.Incident.Params[incident.SEVERITY]), severityRank(b.Incident.Params[incident.SEVERITY])
		if rankA != rankB {
			return rankA < rankB
		}
		if a.RecentlyExpiredSilence != b.RecentlyExpiredSilence {
			return a.RecentlyExpiredSilence
		}
		assignedA := a.Incident.Params[incident.ASSIGNED_TO] != ""
		assignedB := b.Incident.Params[incident.ASSIGNED_TO] != ""
		if assignedA != assignedB {
			return !assignedA
		}
		for _, key := range []string{incident.ALERT_NAME, incident.ABBR, incident.ID} {
			valA := a.Incident.Params[key]
			valB := b.Incident.Params[key]
			if valA != valB {
				return valA < valB
			}
		}
		return a.Incident.ID < b.Incident.ID
	})
}

// GetAlertDetailsHandler handles the get_alert_details MCP tool call.
func (c *AlertsClient) GetAlertDetailsHandler(ctx context.Context, req mcp.CallToolRequest) (fmt.Stringer, error) {
	defer timer.New("GetAlertDetailsHandler").Stop()

	alertID, err := req.RequireString(argAlertID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	historyLimit := req.GetInt(argHistoryLimit, incident.NUM_RECENTLY_RESOLVED_FOR_ID)
	if historyLimit < 0 {
		return nil, skerr.Fmt("history_limit must be non-negative")
	}

	activeIncidents, err := c.incidentStore.GetAll()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var activeIn *incident.Incident
	for i := range activeIncidents {
		if activeIncidents[i].ID == alertID && activeIncidents[i].Active {
			activeIn = &activeIncidents[i]
			break
		}
	}

	excludeKey := ""
	if activeIn != nil {
		excludeKey = activeIn.Key
	}

	queryLimit := historyLimit
	if queryLimit < incident.FlakyNumThreshold {
		queryLimit = incident.FlakyNumThreshold
	}
	resolved, err := c.incidentStore.GetRecentlyResolvedForID(alertID, excludeKey, queryLimit)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	if activeIn == nil && len(resolved) == 0 {
		return nil, skerr.Fmt("No alert found with ID %q", alertID)
	}

	flaky := incident.AreIncidentsFlaky(resolved, incident.FlakyNumThreshold, incident.FlakyDurationThreshold, incident.FlakyDurationPercentage)

	var target incident.Incident
	if activeIn != nil {
		target = *activeIn
	} else {
		target = resolved[0]
		resolved = resolved[1:]
	}

	if len(resolved) > historyLimit {
		resolved = resolved[:historyLimit]
	}

	activeSilences, err := c.silenceStore.GetAll()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	archivedSilences, err := c.silenceStore.GetRecentlyArchived(silence.RECENTLY_EXPIRED_DURATION)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	silenced := target.IsSilenced(activeSilences, true)
	recentlyExpired := false
	if len(archivedSilences) > 0 {
		recentlyExpired = target.IsSilenced(archivedSilences, false)
	}

	var matchingSilences []silence.Silence
	for _, s := range activeSilences {
		if target.IsSilenced([]silence.Silence{s}, true) {
			matchingSilences = append(matchingSilences, s)
		}
	}
	for _, s := range archivedSilences {
		if target.IsSilenced([]silence.Silence{s}, false) {
			matchingSilences = append(matchingSilences, s)
		}
	}

	return &AlertDetails{
		Incident:               target,
		Silenced:               silenced,
		RecentlyExpiredSilence: recentlyExpired,
		Flaky:                  flaky,
		MatchingSilences:       matchingSilences,
		History:                resolved,
	}, nil
}

// AlertSummary represents a single active alert in the list output.
type AlertSummary struct {
	Incident               incident.Incident `json:"incident"`
	Silenced               bool              `json:"silenced"`
	RecentlyExpiredSilence bool              `json:"recently_expired_silence"`
}

// ActiveAlertsList is a slice of AlertSummary that formats as Markdown.
type ActiveAlertsList []AlertSummary

func sanitizeTableCell(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.TrimSpace(s)
}

var ignoredParamKeys = map[string]bool{
	// Keys already rendered in dedicated columns:
	incident.ABBR:        true,
	incident.ALERT_NAME:  true,
	incident.ASSIGNED_TO: true,
	incident.CATEGORY:    true,
	incident.ID:          true,
	incident.OWNER:       true,
	incident.SEVERITY:    true,

	// Internal, ephemeral, or low-signal keys:
	incident.ABBR_OWNER_REGEX:        true,
	incident.K8S_POD_NAME:            true,
	"__state__":                      true,
	"absent_alerts_only_in_clusters": true,
	"alertConstraint":                true,
	"dashboard":                      true,
	"description":                    true,
	"device_os":                      true,
	"device_state":                   true,
	"device_type":                    true,
	"exported_cluster":               true,
	"gpu":                            true,
	"instance":                       true,
	"job":                            true,
	"level":                          true,
	"link_to_source":                 true,
	"location":                       true,
	"os":                             true,
	"pod_template_hash":              true,
	"project_id":                     true,
	"skia_location":                  true,
	"type":                           true,
	"uid":                            true,
}

func (l ActiveAlertsList) String() string {
	if len(l) == 0 {
		return "No matching active alerts."
	}

	type alertGroup struct {
		paramKeys []string
		alerts    []AlertSummary
	}
	var groups []*alertGroup
	groupByKey := map[string]*alertGroup{}

	for _, a := range l {
		var paramKeys []string
		for k, v := range a.Incident.Params {
			if v == "" || ignoredParamKeys[k] {
				continue
			}
			if exportedKey := "exported_" + k; a.Incident.Params[exportedKey] != "" && !ignoredParamKeys[exportedKey] {
				continue
			}
			paramKeys = append(paramKeys, k)
		}
		sort.Strings(paramKeys)
		sig := strings.Join(paramKeys, "\x00")
		g, ok := groupByKey[sig]
		if !ok {
			g = &alertGroup{paramKeys: paramKeys}
			groupByKey[sig] = g
			groups = append(groups, g)
		}
		g.alerts = append(g.alerts, a)
	}

	var sb strings.Builder
	for i, g := range groups {
		if i > 0 {
			sb.WriteString("\n")
		}
		headers := []string{"ID", "AlertName", "Abbr", "Severity", "Category", "Started", "Owner", "AssignedTo", "Flags"}
		headers = append(headers, g.paramKeys...)

		seps := make([]string, len(headers))
		for j, h := range headers {
			n := len(h) + 2
			if n < 4 {
				n = 4
			}
			seps[j] = strings.Repeat("-", n)
		}

		_, _ = fmt.Fprintf(&sb, "| %s |\n", strings.Join(headers, " | "))
		_, _ = fmt.Fprintf(&sb, "|%s|\n", strings.Join(seps, "|"))

		for _, a := range g.alerts {
			in := a.Incident
			var flags []string
			if a.Silenced {
				flags = append(flags, "SILENCED")
			}
			if a.RecentlyExpiredSilence {
				flags = append(flags, "RECENTLY_EXPIRED_SILENCE")
			}
			if len(in.Notes) > 0 {
				flags = append(flags, fmt.Sprintf("NOTES(%d)", len(in.Notes)))
			}
			started := time.Unix(in.Start, 0).UTC().Format(time.RFC3339)
			cells := []string{
				sanitizeTableCell(in.ID),
				sanitizeTableCell(in.Params[incident.ALERT_NAME]),
				sanitizeTableCell(in.Params[incident.ABBR]),
				sanitizeTableCell(in.Params[incident.SEVERITY]),
				sanitizeTableCell(in.Params[incident.CATEGORY]),
				started,
				sanitizeTableCell(in.Params[incident.OWNER]),
				sanitizeTableCell(in.Params[incident.ASSIGNED_TO]),
				strings.Join(flags, ", "),
			}
			for _, k := range g.paramKeys {
				cells = append(cells, sanitizeTableCell(in.Params[k]))
			}
			_, _ = fmt.Fprintf(&sb, "| %s |\n", strings.Join(cells, " | "))
		}
	}
	return sb.String()
}

// AlertDetails contains detailed information about an alert, its silences, and its history.
type AlertDetails struct {
	Incident               incident.Incident   `json:"incident"`
	Silenced               bool                `json:"silenced"`
	RecentlyExpiredSilence bool                `json:"recently_expired_silence"`
	Flaky                  bool                `json:"flaky"`
	MatchingSilences       []silence.Silence   `json:"matching_silences,omitempty"`
	History                []incident.Incident `json:"history,omitempty"`
}

func formatNotes(sb *strings.Builder, notes []note.Note, indent string) {
	for _, n := range notes {
		ts := time.Unix(n.TS, 0).UTC().Format(time.RFC3339)
		_, _ = fmt.Fprintf(sb, "%s- [%s] **%s:** %s\n", indent, ts, n.Author, strings.TrimSpace(n.Text))
	}
}

func (d *AlertDetails) String() string {
	var sb strings.Builder
	in := d.Incident
	start := time.Unix(in.Start, 0).UTC()
	lastSeen := time.Unix(in.LastSeen, 0).UTC()

	_, _ = fmt.Fprintf(&sb, "**ID:** %s\n", in.ID)
	_, _ = fmt.Fprintf(&sb, "**Key:** %s\n", in.Key)
	_, _ = fmt.Fprintf(&sb, "**AlertName:** %s\n", in.Params[incident.ALERT_NAME])
	if abbr := in.Params[incident.ABBR]; abbr != "" {
		_, _ = fmt.Fprintf(&sb, "**Abbr:** %s\n", abbr)
	}
	_, _ = fmt.Fprintf(&sb, "**Active:** %t\n", in.Active)
	_, _ = fmt.Fprintf(&sb, "**Silenced:** %t\n", d.Silenced)
	_, _ = fmt.Fprintf(&sb, "**Recently Expired Silence:** %t\n", d.RecentlyExpiredSilence)
	_, _ = fmt.Fprintf(&sb, "**Flaky:** %t\n", d.Flaky)
	_, _ = fmt.Fprintf(&sb, "**Started:** %s\n", start.Format(time.RFC3339))
	_, _ = fmt.Fprintf(&sb, "**Last Seen:** %s\n", lastSeen.Format(time.RFC3339))
	_, _ = fmt.Fprintf(&sb, "**Duration:** %s\n", lastSeen.Sub(start).String())

	_, _ = fmt.Fprintf(&sb, "\n### Parameters\n\n")
	keys := make([]string, 0, len(in.Params))
	for k := range in.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		_, _ = fmt.Fprintf(&sb, "- **%s:** %s\n", k, in.Params[k])
	}

	if len(in.Notes) > 0 {
		_, _ = fmt.Fprintf(&sb, "\n### Notes\n\n")
		formatNotes(&sb, in.Notes, "")
	}

	if len(d.MatchingSilences) > 0 {
		_, _ = fmt.Fprintf(&sb, "\n### Matching Silences\n\n")
		for _, s := range d.MatchingSilences {
			created := time.Unix(s.Created, 0).UTC().Format(time.RFC3339)
			pKeys := make([]string, 0, len(s.ParamSet))
			for k := range s.ParamSet {
				pKeys = append(pKeys, k)
			}
			sort.Strings(pKeys)
			var pParts []string
			for _, k := range pKeys {
				pParts = append(pParts, fmt.Sprintf("%s=%s", k, strings.Join(s.ParamSet[k], ",")))
			}
			_, _ = fmt.Fprintf(&sb, "- **Active:** %t | **User:** %s | **Created:** %s | **Duration:** %s | **Match:** `%s`\n", s.Active, s.User, created, s.Duration, strings.Join(pParts, " "))
			formatNotes(&sb, s.Notes, "  ")
		}
	}

	if len(d.History) > 0 {
		_, _ = fmt.Fprintf(&sb, "\n### Recent History (%d occurrences)\n\n", len(d.History))
		_, _ = fmt.Fprintf(&sb, "| Started | Last Seen | Duration | Notes |\n")
		_, _ = fmt.Fprintf(&sb, "|---------|-----------|----------|-------|\n")
		for _, h := range d.History {
			hStart := time.Unix(h.Start, 0).UTC()
			hLastSeen := time.Unix(h.LastSeen, 0).UTC()
			var noteTexts []string
			for _, n := range h.Notes {
				noteTexts = append(noteTexts, fmt.Sprintf("%s: %s", n.Author, n.Text))
			}
			_, _ = fmt.Fprintf(
				&sb,
				"| %s | %s | %s | %s |\n",
				hStart.Format(time.RFC3339),
				hLastSeen.Format(time.RFC3339),
				hLastSeen.Sub(hStart).String(),
				sanitizeTableCell(strings.Join(noteTexts, "; ")),
			)
		}
	}

	return sb.String()
}
