// Copyright 2025 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Server implements the stateful mock API handlers backed by a resettable State.
type Server struct {
	state *State
}

func NewServer() *Server {
	return &Server{
		state: NewState(),
	}
}

func NewServerWithState(state *State) *Server {
	return &Server{
		state: state,
	}
}

func (s *Server) State() *State {
	return s.state
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) recordMutation(r *http.Request) []byte {
	var raw []byte
	if r.Body != nil {
		raw, _ = io.ReadAll(r.Body)
	}
	s.state.Recorder().Record(r.URL.Path, r.Method, raw)
	return raw
}

// ResetHandler handles POST /_/mock/reset and POST /_/test/reset to restore
// baseline state between tests and optionally switch the active persona.
func (s *Server) ResetHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Persona  string `json:"persona"`
		Instance string `json:"instance"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	s.state.Reset()
	chosen := strings.TrimSpace(req.Persona)
	if chosen == "" {
		chosen = strings.TrimSpace(req.Instance)
	}
	if chosen != "" {
		s.state.SetPersona(Persona(strings.ToLower(chosen)))
	}
	writeJSON(w, map[string]interface{}{
		"status":  "ok",
		"persona": s.state.GetPersona(),
	})
}

// SetPersonaHandler handles POST /_/mock/set_persona to switch window.perf between
// Chrome and V8 configurations.
func (s *Server) SetPersonaHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Persona string `json:"persona"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	cfg := s.state.SetPersona(Persona(strings.ToLower(strings.TrimSpace(req.Persona))))
	writeJSON(w, map[string]interface{}{
		"persona": s.state.GetPersona(),
		"config":  cfg,
	})
}

// MutationsHandler handles GET /_/mock/mutations returning all recorded write calls.
func (s *Server) MutationsHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]interface{}{
		"mutations": s.state.Recorder().List(),
	})
}

// TestRecordedHandler handles GET /_/test/recorded (with optional ?path= filter)
// for MockServerClient.recorded() and MockServerClient.allRecorded().
func (s *Server) TestRecordedHandler(w http.ResponseWriter, r *http.Request) {
	list := s.state.Recorder().List()
	pathFilter := strings.TrimRight(strings.TrimSpace(r.URL.Query().Get("path")), "/")
	if pathFilter != "" {
		matched := make([]map[string]interface{}, 0)
		for _, m := range list {
			if strings.TrimRight(m.Endpoint, "/") == pathFilter {
				matched = append(matched, map[string]interface{}{
					"method": m.Method,
					"path":   m.Endpoint,
					"body":   m.Body,
				})
			}
		}
		writeJSON(w, matched)
		return
	}
	byPath := make(map[string][]map[string]interface{})
	for _, m := range list {
		byPath[m.Endpoint] = append(byPath[m.Endpoint], map[string]interface{}{
			"method": m.Method,
			"path":   m.Endpoint,
			"body":   m.Body,
		})
	}
	writeJSON(w, byPath)
}

func (s *Server) LoginStatusHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]interface{}{
		"email": "test@google.com",
		"roles": []string{"editor", "admin"},
	})
}

func (s *Server) InitPageHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]interface{}{
		"dataframe": map[string]interface{}{
			"traceset":      map[string][]float32{},
			"header":        s.state.GetCommitHeaders(),
			"paramset":      s.state.GetParamSet(),
			"skip":          0,
			"traceMetadata": []interface{}{},
		},
		"ticks":     []interface{}{},
		"skps":      []int{},
		"msg":       "",
		"anomalies": map[string]interface{}{},
	})
}

func (s *Server) DefaultsHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]interface{}{
		"include_params": []string{"benchmark", "bot", "test", "subtest_1", "subtest_2"},
		"default_param_selections": map[string][]string{
			"stat": {"value"},
		},
		"default_url_values": map[string]string{
			"plotSummary":   "true",
			"useTestPicker": "true",
		},
		"default_range": 604800,
	})
}

var paramHierarchy = []string{"benchmark", "bot", "test", "subtest_1", "subtest_2"}

func (s *Server) NextParamListHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Q string `json:"q"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	selectedParams, _ := url.ParseQuery(req.Q)
	nextParam := ""
	for _, key := range paramHierarchy {
		if vals, ok := selectedParams[key]; !ok || len(vals) == 0 || (len(vals) == 1 && vals[0] == "") {
			nextParam = key
			break
		}
	}

	matched := s.state.MatchTraces([]string{req.Q}, nil)
	fullPS := BuildParamSet(matched)
	respParamSet := make(map[string][]string)
	if nextParam != "" {
		if vals, ok := fullPS[nextParam]; ok && len(vals) > 0 {
			respParamSet[nextParam] = vals
		}
	}
	writeJSON(w, map[string]interface{}{
		"count":    len(matched),
		"paramset": respParamSet,
	})
}

func (s *Server) CountHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Q string `json:"q"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	matched := s.state.MatchTraces([]string{req.Q}, nil)
	writeJSON(w, map[string]interface{}{
		"count":    len(matched),
		"paramset": BuildParamSet(matched),
	})
}

func extractQueriesFromFormulas(formulas []string) []string {
	var out []string
	for _, f := range formulas {
		idx := strings.Index(f, `filter("`)
		if idx == -1 {
			continue
		}
		rest := f[idx+len(`filter("`):]
		endIdx := strings.Index(rest, `")`)
		if endIdx != -1 {
			out = append(out, rest[:endIdx])
		}
	}
	return out
}

func (s *Server) FrameStartHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Queries  []string `json:"queries"`
		Formulas []string `json:"formulas"`
		Keys     string   `json:"keys"`
		TraceIDs []string `json:"trace_ids"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	queries := append([]string{}, req.Queries...)
	if req.Keys != "" {
		if sc, ok := s.state.GetShortcut(req.Keys); ok {
			for _, g := range sc.Graphs {
				queries = append(queries, g.Queries...)
			}
		}
	}
	if len(queries) == 0 && len(req.TraceIDs) == 0 && len(req.Formulas) > 0 {
		queries = extractQueriesFromFormulas(req.Formulas)
	}

	matched := s.state.MatchTraces(queries, req.TraceIDs)
	traceset := make(map[string][]float32, len(matched)+len(req.Formulas))
	if len(req.Queries) > 0 || len(req.TraceIDs) > 0 || req.Keys != "" {
		for _, tr := range matched {
			vals := make([]float32, len(tr.Values))
			copy(vals, tr.Values)
			traceset[tr.Key] = vals
		}
	}

	for _, formula := range req.Formulas {
		trimmed := strings.TrimSpace(formula)
		if trimmed == "" {
			continue
		}
		formulaVals := make([]float32, NumCommits)
		for i := 0; i < NumCommits; i++ {
			var sum float32
			var count int
			for _, tr := range matched {
				if i < len(tr.Values) && tr.Values[i] < MissingDataSentinel*0.99 {
					sum += tr.Values[i]
					count++
				}
			}
			if count > 0 {
				formulaVals[i] = sum / float32(count)
			} else {
				formulaVals[i] = 100.0 + float32(i)*0.5
			}
		}
		traceset[trimmed] = formulaVals
	}

	writeJSON(w, map[string]interface{}{
		"status":   "Finished",
		"messages": []map[string]string{{"key": "Loading", "value": "Finished"}},
		"results": map[string]interface{}{
			"dataframe": map[string]interface{}{
				"traceset":      traceset,
				"header":        s.state.GetCommitHeaders(),
				"paramset":      BuildParamSet(matched),
				"skip":          0,
				"traceMetadata": []interface{}{},
			},
			"anomalymap":   s.state.AnomalyMapForTraces(matched),
			"display_mode": "display_plot",
			"msg":          "",
		},
	})
}

func (s *Server) CidRangeHandler(w http.ResponseWriter, _ *http.Request) {
	headers := s.state.GetCommitHeaders()
	slice := make([]map[string]interface{}, len(headers))
	for i, h := range headers {
		slice[i] = map[string]interface{}{
			"offset":  h.Offset,
			"hash":    h.Hash,
			"ts":      h.Timestamp,
			"author":  h.Author,
			"message": h.Message,
			"url":     h.URL,
			"body":    fmt.Sprintf("Commit body for %d", h.Offset),
		}
	}
	writeJSON(w, slice)
}

func (s *Server) CidHandler(w http.ResponseWriter, r *http.Request) {
	var requested []int
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&requested)
	}
	headers := s.state.GetCommitHeaders()
	want := make(map[int]bool, len(requested))
	for _, cid := range requested {
		want[cid] = true
	}

	var slice []map[string]interface{}
	for _, h := range headers {
		if len(want) > 0 && !want[h.Offset] {
			continue
		}
		slice = append(slice, map[string]interface{}{
			"offset":  h.Offset,
			"hash":    h.Hash,
			"ts":      h.Timestamp,
			"author":  h.Author,
			"message": h.Message,
			"url":     h.URL,
			"body":    fmt.Sprintf("Commit body for %d", h.Offset),
		})
	}
	if len(slice) == 0 && len(headers) > 0 {
		h := headers[0]
		slice = []map[string]interface{}{
			{
				"offset":  h.Offset,
				"hash":    h.Hash,
				"ts":      h.Timestamp,
				"author":  h.Author,
				"message": h.Message,
				"url":     h.URL,
				"body":    fmt.Sprintf("Commit body for %d", h.Offset),
			},
		}
	}

	writeJSON(w, map[string]interface{}{
		"commitSlice": slice,
		"logEntry":    "commit deadbeef\nAuthor: mock@chromium.org\n\nMock commit message",
	})
}

func (s *Server) DetailsHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cid     int    `json:"cid"`
		TraceID string `json:"traceid"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	cid := req.Cid
	if cid == 0 {
		cid = 67135
	}
	writeJSON(w, map[string]interface{}{
		"version":  1,
		"git_hash": fmt.Sprintf("deadbeef%02d", cid-BaseCommitNumber),
		"hash":     fmt.Sprintf("deadbeef%02d", cid-BaseCommitNumber),
		"author":   "mock@chromium.org",
		"message":  fmt.Sprintf("Mock commit details for %d", cid),
		"url":      fmt.Sprintf("https://chromium.googlesource.com/chromium/src/+/deadbeef%02d", cid-BaseCommitNumber),
		"ts":       BaseTimestamp,
		"links": map[string]string{
			"Build Page": fmt.Sprintf("https://ci.chromium.org/b/8700000000%d", cid),
		},
	})
}

func (s *Server) ShiftHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Begin int `json:"begin"`
		End   int `json:"end"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	headers := s.state.GetCommitHeaders()
	var beginTS, endTS int64
	if len(headers) > 0 {
		beginTS = headers[0].Timestamp
		endTS = headers[len(headers)-1].Timestamp
	}
	for _, h := range headers {
		if h.Offset == req.Begin {
			beginTS = h.Timestamp
		}
		if h.Offset == req.End {
			endTS = h.Timestamp
		}
	}
	writeJSON(w, map[string]interface{}{
		"begin": beginTS,
		"end":   endTS,
	})
}

func (s *Server) AlertsHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]interface{}{
		"alerts": 3,
	})
}

func (s *Server) AlertListHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, []map[string]interface{}{
		{
			"id":           "1",
			"display_name": "Demo Alert",
			"query":        "benchmark=jetstream3.crossbench",
			"alert":        "sheriff@chromium.org",
		},
	})
}

func (s *Server) AlertNewHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]interface{}{
		"display_name": "New Alert",
		"step":         "0.5",
	})
}

func (s *Server) AlertUpdateHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]int{"id": 123})
}

func (s *Server) AlertDeleteHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) AlertBugTryHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"url": "http://buganizer/new?template=123"})
}

func (s *Server) AlertNotifyTryHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"status": "sent"})
}

func (s *Server) SubscriptionsHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, []map[string]interface{}{
		{
			"name":          "Chromium Perf Sheriff",
			"contact_email": "sheriff@chromium.org",
		},
	})
}

func (s *Server) DryrunStartHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"id": "dryrun-123"})
}

func (s *Server) ClusterStartHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"id": "cluster-job-1"})
}

func (s *Server) ShortcutUpdateHandler(w http.ResponseWriter, r *http.Request) {
	raw := s.recordMutation(r)
	var req GraphsShortcut
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req)
	}
	id := s.state.SaveShortcut(req.Graphs)
	writeJSON(w, map[string]interface{}{
		"id": id,
	})
}

func (s *Server) GetShortcutHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID      string `json:"ID"`
		LowerID string `json:"id"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	id := req.ID
	if id == "" {
		id = req.LowerID
	}
	if id == "" {
		id = r.URL.Query().Get("id")
	}
	if sc, ok := s.state.GetShortcut(id); ok {
		writeJSON(w, sc)
		return
	}
	writeJSON(w, GraphsShortcut{
		Graphs: NormalizeGraphConfigs([]GraphConfig{
			{Queries: []string{"benchmark=jetstream3.crossbench&bot=mac-m3-pro-perf&test=Total"}},
		}),
	})
}

func (s *Server) GetSheriffListHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]interface{}{
		"sheriff_list": []string{"Chromium Perf Sheriff", "V8 JavaScript Perf", "V8 Perf Sheriff"},
		"error":        "",
	})
}

func (s *Server) GetAnomaliesHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sheriff := q.Get("sheriff")
	showImprovements := q.Get("improvements") == "true"
	includeTriaged := q.Get("triaged") == "true"

	all := s.state.OrderedAnomalies()
	filtered := make([]MockAnomaly, 0, len(all))
	for _, a := range all {
		if !showImprovements && a.IsImprovement {
			continue
		}
		isTriaged := a.BugID != 0
		if !includeTriaged && isTriaged {
			continue
		}
		if sheriff == "V8 JavaScript Perf" {
			if a.ID != "123" && a.ID != "789" && a.ID != "101" && a.ID != "1001" && a.ID != "1002" {
				continue
			}
		} else if sheriff == "V8 Perf Sheriff" {
			if a.SubscriptionName != "V8 Perf Sheriff" && a.SubscriptionName != "V8 JavaScript Perf" {
				continue
			}
		} else {
			// Default / Chromium Perf Sheriff
			if a.SubscriptionName != "Chromium Perf Sheriff" && !(showImprovements && a.ID == "101") {
				continue
			}
		}
		filtered = append(filtered, a)
	}

	writeJSON(w, map[string]interface{}{
		"subscription": map[string]interface{}{
			"name":          sheriff,
			"bug_component": "Chromium>Perf",
		},
		"alerts":         []interface{}{},
		"anomaly_list":   filtered,
		"anomaly_cursor": "",
		"error":          "",
	})
}

func (s *Server) GroupReportHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Rev            string `json:"rev"`
		AnomalyIDs     string `json:"anomalyIDs"`
		BugID          string `json:"bugID"`
		AnomalyGroupID string `json:"anomalyGroupID"`
		Sid            string `json:"sid"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	all := s.state.OrderedAnomalies()
	requestedIDs := make(map[string]bool)
	var selectedKeys []string
	if strings.TrimSpace(req.AnomalyIDs) != "" {
		for _, part := range strings.Split(req.AnomalyIDs, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				requestedIDs[trimmed] = true
				selectedKeys = append(selectedKeys, trimmed)
			}
		}
	}

	var matched []MockAnomaly
	for _, a := range all {
		if len(requestedIDs) > 0 {
			if requestedIDs[a.ID] {
				matched = append(matched, a)
			} else if (requestedIDs["123"] || requestedIDs["456"]) && a.ID == "789" {
				// Include overlapping untriaged anomaly 789 so report-page-sk renders
				// the "Other groups, related to requested ones" section.
				matched = append(matched, a)
			}
			continue
		}
		if req.BugID != "" {
			if strconv.Itoa(a.BugID) == req.BugID {
				matched = append(matched, a)
				selectedKeys = append(selectedKeys, a.ID)
			}
			continue
		}
		if req.Rev != "" {
			revNum, err := strconv.Atoi(req.Rev)
			if err == nil && revNum >= a.StartRevision && revNum <= a.EndRevision {
				matched = append(matched, a)
				selectedKeys = append(selectedKeys, a.ID)
			}
			continue
		}
		if !a.IsImprovement && a.SubscriptionName == "Chromium Perf Sheriff" {
			matched = append(matched, a)
		}
	}

	if len(matched) == 0 {
		for _, a := range all {
			if !a.IsImprovement && a.SubscriptionName == "Chromium Perf Sheriff" {
				matched = append(matched, a)
			}
		}
	}
	if len(selectedKeys) == 0 && len(matched) > 0 {
		selectedKeys = []string{matched[0].ID}
	}

	headers := s.state.GetCommitHeaders()
	var minTS, maxTS int64
	if len(headers) > 0 {
		minTS = headers[0].Timestamp
		maxTS = headers[len(headers)-1].Timestamp
	}
	timerangeMap := make(map[string]map[string]int64, len(matched))
	for _, a := range matched {
		timerangeMap[a.ID] = map[string]int64{
			"begin": minTS,
			"end":   maxTS,
		}
	}

	writeJSON(w, map[string]interface{}{
		"anomaly_list":           matched,
		"timerange_map":          timerangeMap,
		"selected_keys":          selectedKeys,
		"is_commit_number_based": true,
		"sid":                    req.Sid,
		"error":                  "",
	})
}

func (s *Server) TriageEditAnomaliesHandler(w http.ResponseWriter, r *http.Request) {
	raw := s.recordMutation(r)
	var req struct {
		Keys   []json.Number `json:"keys"`
		Action string        `json:"action"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req)
	}
	keyStrings := make([]string, 0, len(req.Keys))
	for _, k := range req.Keys {
		keyStrings = append(keyStrings, k.String())
	}
	action := strings.ToUpper(strings.TrimSpace(req.Action))
	switch action {
	case "IGNORE":
		s.state.SetAnomalyBugID(keyStrings, -2)
	case "RESET":
		s.state.SetAnomalyBugID(keyStrings, 0)
	case "NUDGE":
		// Nudge shifts revision bounds in place; keep bug state intact.
	}
	writeJSON(w, map[string]interface{}{
		"error": "",
	})
}

func (s *Server) TriageAssociateAlertsHandler(w http.ResponseWriter, r *http.Request) {
	raw := s.recordMutation(r)
	var req struct {
		BugID int           `json:"bug_id"`
		Keys  []json.Number `json:"keys"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req)
	}
	bugID := req.BugID
	if bugID == 0 {
		bugID = 12345
	}
	keyStrings := make([]string, 0, len(req.Keys))
	for _, k := range req.Keys {
		keyStrings = append(keyStrings, k.String())
	}
	s.state.SetAnomalyBugID(keyStrings, bugID)
	writeJSON(w, map[string]interface{}{
		"bug_id": bugID,
	})
}

func (s *Server) TriageFileBugHandler(w http.ResponseWriter, r *http.Request) {
	raw := s.recordMutation(r)
	var req struct {
		Keys []json.Number `json:"keys"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req)
	}
	bugID := 12345
	keyStrings := make([]string, 0, len(req.Keys))
	for _, k := range req.Keys {
		keyStrings = append(keyStrings, k.String())
	}
	s.state.SetAnomalyBugID(keyStrings, bugID)
	writeJSON(w, map[string]interface{}{
		"bug_id": bugID,
	})
}

func (s *Server) TriageListIssuesHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]interface{}{
		"issues": []map[string]interface{}{
			{
				"issueId": "12345",
				"issueState": map[string]interface{}{
					"title":  "Mock Issue 12345",
					"status": "NEW",
				},
			},
		},
	})
}

func (s *Server) UserIssuesHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]interface{}{
		"UserIssues": []interface{}{},
	})
}

func (s *Server) RevisionHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, []map[string]interface{}{
		{
			"master":         "ChromiumPerf",
			"bot":            "mac-m3-pro-perf",
			"benchmark":      "jetstream3.crossbench",
			"start_revision": 67134,
			"end_revision":   67135,
			"start_time":     1700050400,
			"end_time":       1700054000,
			"test":           "JetStream2/Total",
			"is_improvement": false,
			"bug_id":         "12345",
			"explore_url":    "/e",
			"query":          "benchmark=jetstream3.crossbench&bot=mac-m3-pro-perf&test=Total",
			"anomaly_ids":    []string{"123"},
		},
	})
}

func (s *Server) FavoritesHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]interface{}{
		"sections": []map[string]interface{}{
			{
				"name": "Mock Favorites",
				"links": []map[string]interface{}{
					{
						"id":          "fav1",
						"text":        "JetStream3 Mac M3",
						"href":        "/m?shortcut=mock-shortcut-1",
						"description": "Mock favorite link for testing",
					},
				},
			},
		},
	})
}

func (s *Server) RegressionsHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]interface{}{
		"header":     []interface{}{},
		"table":      []interface{}{},
		"categories": []string{},
	})
}

func (s *Server) SubrepoLinksBatchHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CommitNumbers []int    `json:"commit_numbers"`
		TraceIDs      []string `json:"trace_ids"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	res := make(map[string]map[string]map[string]string)
	for _, cid := range req.CommitNumbers {
		cidStr := strconv.Itoa(cid)
		res[cidStr] = make(map[string]map[string]string)
		for _, tid := range req.TraceIDs {
			res[cidStr][tid] = map[string]string{
				"V8":     fmt.Sprintf("https://chromium.googlesource.com/v8/v8/+/v8hash%d", cid),
				"WebRTC": fmt.Sprintf("https://chromium.googlesource.com/external/webrtc/+/webrtchash%d", cid),
			}
		}
	}
	writeJSON(w, res)
}

func (s *Server) CalculateRegrShortcutHandler(w http.ResponseWriter, r *http.Request) {
	raw := s.recordMutation(r)
	var req struct {
		AnomalyIDs string `json:"anomalyIDs"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req)
	}
	sid := s.state.SaveShortcut([]GraphConfig{
		{Queries: []string{"benchmark=jetstream3.crossbench&bot=mac-m3-pro-perf&bot=linux-r350-perf&test=Total"}},
	})
	writeJSON(w, map[string]interface{}{
		"sid": sid,
	})
}

func (s *Server) BisectCreateHandler(w http.ResponseWriter, r *http.Request) {
	s.recordMutation(r)
	jobID := "pinpoint-mock-job-001"
	jobURL := "https://pinpoint-dot-chromeperf.appspot.com/job/pinpoint-mock-job-001"
	writeJSON(w, map[string]interface{}{
		"jobId":   jobID,
		"jobUrl":  jobURL,
		"job_id":  jobID,
		"job_url": jobURL,
	})
}

func (s *Server) TryJobCreateHandler(w http.ResponseWriter, r *http.Request) {
	s.recordMutation(r)
	jobID := "try-mock-job-001"
	jobURL := "https://pinpoint-dot-chromeperf.appspot.com/job/try-mock-job-001"
	writeJSON(w, map[string]interface{}{
		"jobId":   jobID,
		"jobUrl":  jobURL,
		"job_id":  jobID,
		"job_url": jobURL,
	})
}
