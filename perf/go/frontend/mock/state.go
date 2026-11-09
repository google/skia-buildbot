// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
)

// GraphConfig matches the frontend GraphConfig JSON contract.
type GraphConfig struct {
	Formulas []string `json:"formulas"`
	Queries  []string `json:"queries"`
	Keys     string   `json:"keys"`
}

// GraphsShortcut wraps a list of GraphConfigs stored under a shortcut ID.
type GraphsShortcut struct {
	Graphs []GraphConfig `json:"graphs"`
}

// NormalizeGraphConfigs ensures Formulas and Queries are non-nil slices so JSON
// serialization emits [] instead of null.
func NormalizeGraphConfigs(configs []GraphConfig) []GraphConfig {
	out := make([]GraphConfig, len(configs))
	for i, c := range configs {
		formulas := c.Formulas
		if formulas == nil {
			formulas = []string{}
		}
		queries := c.Queries
		if queries == nil {
			queries = []string{}
		}
		out[i] = GraphConfig{
			Formulas: formulas,
			Queries:  queries,
			Keys:     c.Keys,
		}
	}
	return out
}

// State holds all mutable state for the mock server behind an RWMutex so
// POST /_/mock/reset can restore the baseline between Playwright tests.
type State struct {
	mu              sync.RWMutex
	persona         Persona
	traces          []TraceDefinition
	traceByKey      map[string]TraceDefinition
	paramSet        map[string][]string
	commitHeaders   []CommitHeader
	anomalies       map[string]*MockAnomaly
	shortcuts       map[string]GraphsShortcut
	nextShortcutSeq int
	recorder        *MutationRecorder
}

func NewState() *State {
	s := &State{
		recorder: NewMutationRecorder(),
	}
	s.Reset()
	return s
}

// Reset restores the deterministic dataset, anomalies, shortcuts, persona, and
// clears the mutation recorder.
func (s *State) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.persona = PersonaChrome
	s.traces = DefaultTraces()
	s.traceByKey = make(map[string]TraceDefinition, len(s.traces))
	for _, tr := range s.traces {
		s.traceByKey[tr.Key] = tr
	}
	s.paramSet = BuildParamSet(s.traces)
	s.commitHeaders = DefaultCommitHeaders()
	s.anomalies = DefaultAnomalies()
	s.shortcuts = map[string]GraphsShortcut{
		"mock-shortcut-1": {
			Graphs: NormalizeGraphConfigs([]GraphConfig{
				{
					Queries: []string{"benchmark=jetstream3.crossbench&bot=mac-m3-pro-perf&test=Total"},
				},
			}),
		},
		"heavy-shortcut-1": {
			Graphs: NormalizeGraphConfigs([]GraphConfig{
				{Queries: []string{"benchmark=jetstream3.crossbench&bot=mac-m3-pro-perf&test=Total"}},
				{Queries: []string{"benchmark=jetstream3.crossbench&bot=linux-r350-perf&test=Total"}},
			}),
		},
		"mock-heavy-shortcut": {
			Graphs: NormalizeGraphConfigs([]GraphConfig{
				{Queries: []string{"benchmark=jetstream3.crossbench&bot=mac-m3-pro-perf&test=Total"}},
				{Queries: []string{"benchmark=jetstream3.crossbench&bot=linux-r350-perf&test=Total"}},
				{Queries: []string{"benchmark=jetstream3.crossbench&subtest_2=Air&test=Score"}},
				{Queries: []string{"benchmark=speedometer3.crossbench&test=%20TodoMVC-React-Complex-DOM"}},
				{Queries: []string{"benchmark=speedometer3.crossbench&subtest_2=Total&test=Score"}},
				{Queries: []string{"benchmark=v8&bot=MacM4&subtest_1=JetStream2&test=Total"}},
			}),
		},
	}
	s.nextShortcutSeq = 2
	s.recorder.Reset()
}

func (s *State) Recorder() *MutationRecorder {
	return s.recorder
}

func (s *State) GetPersona() Persona {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.persona
}

func (s *State) SetPersona(p Persona) PersonaConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p != PersonaV8 {
		p = PersonaChrome
	}
	s.persona = p
	return ConfigForPersona(s.persona)
}

func (s *State) GetTraces() []TraceDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TraceDefinition, len(s.traces))
	copy(out, s.traces)
	return out
}

func (s *State) GetParamSet() map[string][]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.paramSet
}

func (s *State) GetCommitHeaders() []CommitHeader {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]CommitHeader, len(s.commitHeaders))
	copy(out, s.commitHeaders)
	return out
}

// MatchTraces returns all traces matching any of the provided URL-encoded query
// strings (or all traces if queries is empty). Within a single query string,
// multiple values for the same key are OR-ed and distinct keys are AND-ed.
func (s *State) MatchTraces(queries []string, traceIDs []string) []TraceDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nonEmptyQueries := make([]string, 0, len(queries))
	for _, q := range queries {
		if strings.TrimSpace(q) != "" {
			nonEmptyQueries = append(nonEmptyQueries, q)
		}
	}

	if len(traceIDs) > 0 {
		seen := make(map[string]bool, len(traceIDs))
		var matched []TraceDefinition
		for _, id := range traceIDs {
			if tr, ok := s.traceByKey[id]; ok && !seen[id] {
				seen[id] = true
				matched = append(matched, tr)
			}
		}
		if len(matched) > 0 || len(nonEmptyQueries) == 0 {
			return matched
		}
	}

	if len(nonEmptyQueries) == 0 {
		out := make([]TraceDefinition, len(s.traces))
		copy(out, s.traces)
		return out
	}

	seen := make(map[string]bool)
	var matched []TraceDefinition
	for _, rawQuery := range nonEmptyQueries {
		parsed, err := url.ParseQuery(strings.TrimPrefix(rawQuery, "?"))
		if err != nil {
			continue
		}
		for _, tr := range s.traces {
			if seen[tr.Key] {
				continue
			}
			if traceMatchesQuery(tr.Params, parsed) {
				seen[tr.Key] = true
				matched = append(matched, tr)
			}
		}
	}
	return matched
}

func traceMatchesQuery(params map[string]string, query url.Values) bool {
	for k, allowedVals := range query {
		if len(allowedVals) == 0 {
			continue
		}
		actual, ok := params[k]
		if !ok {
			return false
		}
		matchedVal := false
		for _, allowed := range allowedVals {
			if allowed == "" || actual == allowed {
				matchedVal = true
				break
			}
		}
		if !matchedVal {
			return false
		}
	}
	return true
}

func (s *State) SaveShortcut(graphs []GraphConfig) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := fmt.Sprintf("mock-shortcut-%d", s.nextShortcutSeq)
	s.nextShortcutSeq++
	s.shortcuts[id] = GraphsShortcut{
		Graphs: NormalizeGraphConfigs(graphs),
	}
	return id
}

func (s *State) GetShortcut(id string) (GraphsShortcut, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sc, ok := s.shortcuts[id]
	return sc, ok
}

// SetAnomalyBugID updates the BugID on the requested anomaly IDs (passed as ints
// or strings) and returns the updated anomalies.
func (s *State) SetAnomalyBugID(keys []string, bugID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range keys {
		if a, ok := s.anomalies[k]; ok {
			a.BugID = bugID
			if bugID == -2 {
				a.State = "ignored"
			} else if bugID > 0 {
				a.State = "triaged"
			} else if bugID == 0 {
				if a.IsImprovement {
					a.State = "improvement"
				} else {
					a.State = "regression"
				}
			}
		}
	}
}

// OrderedAnomalies returns a copy of all anomalies in deterministic ID order.
func (s *State) OrderedAnomalies() []MockAnomaly {
	s.mu.RLock()
	defer s.mu.RUnlock()
	order := []string{"123", "456", "789", "101", "1001", "1002"}
	seen := make(map[string]bool, len(s.anomalies))
	out := make([]MockAnomaly, 0, len(s.anomalies))
	for _, id := range order {
		if a, ok := s.anomalies[id]; ok {
			seen[id] = true
			out = append(out, *a)
		}
	}
	var extra []string
	for id := range s.anomalies {
		if !seen[id] {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	for _, id := range extra {
		out = append(out, *s.anomalies[id])
	}
	return out
}

// AnomalyMapForTraces builds the trace_key -> commit_number -> Anomaly map for
// a set of traces.
func (s *State) AnomalyMapForTraces(traces []TraceDefinition) map[string]map[int]MockAnomaly {
	s.mu.RLock()
	defer s.mu.RUnlock()

	traceSet := make(map[string]bool, len(traces))
	for _, tr := range traces {
		traceSet[tr.Key] = true
	}

	out := make(map[string]map[int]MockAnomaly)
	for _, a := range s.anomalies {
		if !traceSet[a.TraceKey] {
			continue
		}
		if out[a.TraceKey] == nil {
			out[a.TraceKey] = make(map[int]MockAnomaly)
		}
		out[a.TraceKey][a.EndRevision] = *a
	}
	return out
}
