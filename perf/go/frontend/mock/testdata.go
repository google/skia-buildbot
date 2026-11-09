// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"sort"
	"strings"
)

const (
	BaseCommitNumber    = 67120
	NumCommits          = 20
	BaseTimestamp       = int64(1584835200)
	CommitStepSeconds   = int64(3600)
	MissingDataSentinel = float32(1e32)
)

// CommitHeader matches the ColumnHeader JSON contract returned in DataFrames.
type CommitHeader struct {
	Offset    int    `json:"offset"`
	Timestamp int64  `json:"timestamp"`
	Hash      string `json:"hash"`
	Author    string `json:"author"`
	Message   string `json:"message"`
	URL       string `json:"url"`
}

// MockAnomaly matches the Anomaly JSON contract used across /m, /e, /e2, /a, and /u.
type MockAnomaly struct {
	ID                  string   `json:"id"`
	TestPath            string   `json:"test_path"`
	TraceKey            string   `json:"-"`
	BugID               int      `json:"bug_id"`
	StartRevision       int      `json:"start_revision"`
	EndRevision         int      `json:"end_revision"`
	DisplayCommitNumber int      `json:"display_commit_number"`
	StartRevisionHash   string   `json:"start_revision_hash,omitempty"`
	EndRevisionHash     string   `json:"end_revision_hash,omitempty"`
	IsImprovement       bool     `json:"is_improvement"`
	Recovered           bool     `json:"recovered"`
	State               string   `json:"state"`
	Statistic           string   `json:"statistic"`
	Units               string   `json:"units"`
	DegreesOfFreedom    float64  `json:"degrees_of_freedom"`
	MedianBeforeAnomaly float64  `json:"median_before_anomaly"`
	MedianAfterAnomaly  float64  `json:"median_after_anomaly"`
	PValue              float64  `json:"p_value"`
	SegmentSizeAfter    int      `json:"segment_size_after"`
	SegmentSizeBefore   int      `json:"segment_size_before"`
	StdDevBeforeAnomaly float64  `json:"std_dev_before_anomaly"`
	TStatistic          float64  `json:"t_statistic"`
	SubscriptionName    string   `json:"subscription_name"`
	BugComponent        string   `json:"bug_component"`
	BugLabels           []string `json:"bug_labels"`
	BugCcEmails         []string `json:"bug_cc_emails"`
	BisectIDs           []string `json:"bisect_ids"`
}

// TraceDefinition bundles a structured trace key, its parsed parameter map, and
// its 20 commit values.
type TraceDefinition struct {
	Key    string
	Params map[string]string
	Values []float32
}

// MakeStructuredKey builds a canonical ",k1=v1,k2=v2," key with sorted keys.
func MakeStructuredKey(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}
	return "," + strings.Join(parts, ",") + ","
}

// ParseStructuredKey parses a ",k1=v1,k2=v2," trace key into a map.
func ParseStructuredKey(key string) map[string]string {
	out := make(map[string]string)
	trimmed := strings.Trim(key, ",")
	if trimmed == "" {
		return out
	}
	for _, part := range strings.Split(trimmed, ",") {
		idx := strings.IndexByte(part, '=')
		if idx > 0 {
			out[part[:idx]] = part[idx+1:]
		}
	}
	return out
}

// DefaultCommitHeaders returns the 20 deterministic commits (67120..67139).
func DefaultCommitHeaders() []CommitHeader {
	headers := make([]CommitHeader, NumCommits)
	for i := 0; i < NumCommits; i++ {
		offset := BaseCommitNumber + i
		headers[i] = CommitHeader{
			Offset:    offset,
			Timestamp: BaseTimestamp + int64(i)*CommitStepSeconds,
			Hash:      fmt.Sprintf("deadbeef%02d", i),
			Author:    "mock@chromium.org",
			Message:   fmt.Sprintf("Mock commit %d", offset),
			URL:       fmt.Sprintf("https://chromium.googlesource.com/chromium/src/+/deadbeef%02d", i),
		}
	}
	return headers
}

// DefaultTraces returns the 24 deterministic traces (18 ChromiumPerf and 6 V8Perf).
func DefaultTraces() []TraceDefinition {
	type rawSpec struct {
		master    string
		bot       string
		benchmark string
		subtest1  string
		subtest2  string
		test      string
		unit      string
		gen       func(i int) float32
	}

	specs := []rawSpec{
		{
			master: "ChromiumPerf", bot: "mac-m3-pro-perf", benchmark: "jetstream3.crossbench",
			subtest1: "JetStream2", subtest2: "Total", test: "Total", unit: "score_biggerIsBetter",
			gen: func(i int) float32 {
				if i >= 15 {
					return 135.0 + float32(i-15)*0.5
				}
				return 100.0 + float32(i)*0.5
			},
		},
		{
			master: "ChromiumPerf", bot: "linux-r350-perf", benchmark: "jetstream3.crossbench",
			subtest1: "JetStream2", subtest2: "Total", test: "Total", unit: "score_biggerIsBetter",
			gen: func(i int) float32 {
				if i >= 12 {
					return 82.0 + float32(i-12)*0.5
				}
				return 95.0 + float32(i)*0.5
			},
		},
		{
			master: "ChromiumPerf", bot: "linux-r350-perf", benchmark: "jetstream3.crossbench",
			subtest1: "JetStream2", subtest2: "Air", test: "Score", unit: "score_biggerIsBetter",
			gen: func(i int) float32 {
				if i >= 14 {
					return 101.0 + float32(i-14)*0.4
				}
				return 110.0 + float32(i)*0.4
			},
		},
		{
			master: "ChromiumPerf", bot: "mac-m3-pro-perf", benchmark: "jetstream3.crossbench",
			subtest1: "JetStream2", subtest2: "Air", test: "Score", unit: "score_biggerIsBetter",
			gen: func(i int) float32 { return 112.0 + float32(i)*0.4 },
		},
		{
			master: "ChromiumPerf", bot: "linux-r350-perf", benchmark: "jetstream3.crossbench",
			subtest1: "JetStream2", subtest2: "Basic", test: "Score", unit: "score_biggerIsBetter",
			gen: func(i int) float32 { return 88.0 + float32(i)*0.3 },
		},
		{
			master: "ChromiumPerf", bot: "mac-m3-pro-perf", benchmark: "jetstream3.crossbench",
			subtest1: "JetStream2", subtest2: "Basic", test: "Score", unit: "score_biggerIsBetter",
			gen: func(i int) float32 { return 90.0 + float32(i)*0.3 },
		},
		{
			master: "ChromiumPerf", bot: "linux-r350-perf", benchmark: "jetstream3.crossbench",
			subtest1: "JetStream2", subtest2: "Total", test: "Score", unit: "score_biggerIsBetter",
			gen: func(i int) float32 { return 104.0 + float32(i)*0.2 },
		},
		{
			master: "ChromiumPerf", bot: "mac-m3-pro-perf", benchmark: "jetstream3.crossbench",
			subtest1: "JetStream2", subtest2: "Total", test: "Score", unit: "score_biggerIsBetter",
			gen: func(i int) float32 { return 106.0 + float32(i)*0.2 },
		},
		{
			master: "ChromiumPerf", bot: "linux-r350-perf", benchmark: "speedometer3.crossbench",
			subtest1: "Speedometer3", subtest2: "Total", test: " TodoMVC-React-Complex-DOM", unit: "ms_smallerIsBetter",
			gen: func(i int) float32 {
				if i == 5 || i == 12 {
					return MissingDataSentinel
				}
				return 42.0 + float32(i)*0.2
			},
		},
		{
			master: "ChromiumPerf", bot: "mac-m3-pro-perf", benchmark: "speedometer3.crossbench",
			subtest1: "Speedometer3", subtest2: "Total", test: " TodoMVC-React-Complex-DOM", unit: "ms_smallerIsBetter",
			gen: func(i int) float32 { return 38.0 + float32(i)*0.2 },
		},
		{
			master: "ChromiumPerf", bot: "linux-r350-perf", benchmark: "speedometer3.crossbench",
			subtest1: "Speedometer3", subtest2: "Total", test: "Score", unit: "score_biggerIsBetter",
			gen: func(i int) float32 {
				if i >= 14 {
					return 19.5 + float32(i-14)*0.1
				}
				return 21.0 + float32(i)*0.1
			},
		},
		{
			master: "ChromiumPerf", bot: "mac-m3-pro-perf", benchmark: "speedometer3.crossbench",
			subtest1: "Speedometer3", subtest2: "Total", test: "Score", unit: "score_biggerIsBetter",
			gen: func(i int) float32 { return 24.0 + float32(i)*0.1 },
		},
		{
			master: "ChromiumPerf", bot: "linux-r350-perf", benchmark: "speedometer3.crossbench",
			subtest1: "Speedometer3", subtest2: "Total", test: "Total", unit: "ms_smallerIsBetter",
			gen: func(i int) float32 { return 150.0 + float32(i)*0.5 },
		},
		{
			master: "ChromiumPerf", bot: "mac-m3-pro-perf", benchmark: "speedometer3.crossbench",
			subtest1: "Speedometer3", subtest2: "Total", test: "Total", unit: "ms_smallerIsBetter",
			gen: func(i int) float32 { return 140.0 + float32(i)*0.5 },
		},
		{
			master: "V8Perf", bot: "linux32", benchmark: "v8.infra",
			subtest1: "d8", subtest2: "Total", test: "binary_size", unit: "bytes_smallerIsBetter",
			gen: func(i int) float32 { return 10240.0 + float32(i)*10.0 },
		},
		{
			master: "V8Perf", bot: "linux32", benchmark: "v8.infra",
			subtest1: "chrome", subtest2: "Total", test: "binary_size", unit: "bytes_smallerIsBetter",
			gen: func(i int) float32 { return 20480.0 + float32(i)*15.0 },
		},
		{
			master: "V8Perf", bot: "MacM4", benchmark: "v8.infra",
			subtest1: "d8", subtest2: "Total", test: "binary_size", unit: "bytes_smallerIsBetter",
			gen: func(i int) float32 { return 10500.0 + float32(i)*10.0 },
		},
		{
			master: "V8Perf", bot: "MacM4", benchmark: "v8.infra",
			subtest1: "chrome", subtest2: "Total", test: "binary_size", unit: "bytes_smallerIsBetter",
			gen: func(i int) float32 { return 20800.0 + float32(i)*15.0 },
		},
		{
			master: "V8Perf", bot: "MacM4", benchmark: "v8",
			subtest1: "default", subtest2: "Total", test: "JetStream2", unit: "score_biggerIsBetter",
			gen: func(i int) float32 {
				if i >= 16 {
					return 245.0 + float32(i-16)*1.0
				}
				return 200.0 + float32(i)*1.0
			},
		},
		{
			master: "V8Perf", bot: "MacM4", benchmark: "v8",
			subtest1: "future", subtest2: "Total", test: "JetStream2", unit: "score_biggerIsBetter",
			gen: func(i int) float32 { return 180.0 + float32(i)*0.8 },
		},
		{
			master: "V8Perf", bot: "MacM4", benchmark: "v8",
			subtest1: "default", subtest2: "Total", test: "JetStream3", unit: "score_biggerIsBetter",
			gen: func(i int) float32 { return 170.0 + float32(i)*0.6 },
		},
		{
			master: "V8Perf", bot: "MacM4", benchmark: "v8",
			subtest1: "default", subtest2: "Air", test: "Octane2", unit: "score_biggerIsBetter",
			gen: func(i int) float32 { return 310.0 + float32(i)*1.2 },
		},
		{
			master: "V8Perf", bot: "linux32", benchmark: "v8",
			subtest1: "default", subtest2: "Total", test: "JetStream2", unit: "score_biggerIsBetter",
			gen: func(i int) float32 { return 195.0 + float32(i)*1.0 },
		},
		{
			master: "V8Perf", bot: "MacM4", benchmark: "v8",
			subtest1: "future", subtest2: "Total", test: "JetStream3", unit: "score_biggerIsBetter",
			gen: func(i int) float32 { return 210.0 + float32(i)*0.9 },
		},
	}

	out := make([]TraceDefinition, 0, len(specs))
	for _, s := range specs {
		params := map[string]string{
			"benchmark": s.benchmark,
			"bot":       s.bot,
			"master":    s.master,
			"stat":      "value",
			"subtest_1": s.subtest1,
			"subtest_2": s.subtest2,
			"test":      s.test,
			"unit":      s.unit,
		}
		vals := make([]float32, NumCommits)
		for i := 0; i < NumCommits; i++ {
			vals[i] = s.gen(i)
		}
		out = append(out, TraceDefinition{
			Key:    MakeStructuredKey(params),
			Params: params,
			Values: vals,
		})
	}
	return out
}

// BuildParamSet computes the sorted ParamSet from a slice of TraceDefinitions.
func BuildParamSet(traces []TraceDefinition) map[string][]string {
	seen := make(map[string]map[string]bool)
	for _, tr := range traces {
		for k, v := range tr.Params {
			if seen[k] == nil {
				seen[k] = make(map[string]bool)
			}
			seen[k][v] = true
		}
	}
	ps := make(map[string][]string, len(seen))
	for k, vals := range seen {
		list := make([]string, 0, len(vals))
		for v := range vals {
			list = append(list, v)
		}
		sort.Strings(list)
		ps[k] = list
	}
	return ps
}

// DefaultAnomalies returns the pre-seeded anomalies (123, 456, 789, 101, 1001, 1002).
// TestPath follows ChromeTraceFormatter's master/bot/benchmark/test/subtest_1/subtest_2
// layout so report-page-sk and anomalies-table-sk round-trip test_path to trace queries.
func DefaultAnomalies() map[string]*MockAnomaly {
	return map[string]*MockAnomaly{
		"123": {
			ID:                  "123",
			TestPath:            "ChromiumPerf/mac-m3-pro-perf/jetstream3.crossbench/Total/JetStream2/Total",
			TraceKey:            ",benchmark=jetstream3.crossbench,bot=mac-m3-pro-perf,master=ChromiumPerf,stat=value,subtest_1=JetStream2,subtest_2=Total,test=Total,unit=score_biggerIsBetter,",
			BugID:               0,
			StartRevision:       67134,
			EndRevision:         67135,
			DisplayCommitNumber: 67135,
			StartRevisionHash:   "deadbeef14",
			EndRevisionHash:     "deadbeef15",
			IsImprovement:       false,
			Recovered:           false,
			State:               "regression",
			Statistic:           "value",
			Units:               "score_biggerIsBetter",
			DegreesOfFreedom:    10.0,
			MedianBeforeAnomaly: 107.0,
			MedianAfterAnomaly:  135.0,
			PValue:              0.001,
			SegmentSizeBefore:   5,
			SegmentSizeAfter:    5,
			StdDevBeforeAnomaly: 1.2,
			TStatistic:          8.5,
			SubscriptionName:    "Chromium Perf Sheriff",
			BugComponent:        "Chromium>Perf",
			BugLabels:           []string{"Perf-Regression"},
			BugCcEmails:         []string{"sheriff@chromium.org"},
			BisectIDs:           []string{},
		},
		"456": {
			ID:                  "456",
			TestPath:            "ChromiumPerf/linux-r350-perf/jetstream3.crossbench/Total/JetStream2/Total",
			TraceKey:            ",benchmark=jetstream3.crossbench,bot=linux-r350-perf,master=ChromiumPerf,stat=value,subtest_1=JetStream2,subtest_2=Total,test=Total,unit=score_biggerIsBetter,",
			BugID:               0,
			StartRevision:       67131,
			EndRevision:         67132,
			DisplayCommitNumber: 67132,
			StartRevisionHash:   "deadbeef11",
			EndRevisionHash:     "deadbeef12",
			IsImprovement:       false,
			Recovered:           false,
			State:               "regression",
			Statistic:           "value",
			Units:               "score_biggerIsBetter",
			DegreesOfFreedom:    10.0,
			MedianBeforeAnomaly: 102.0,
			MedianAfterAnomaly:  82.0,
			PValue:              0.002,
			SegmentSizeBefore:   5,
			SegmentSizeAfter:    5,
			StdDevBeforeAnomaly: 1.1,
			TStatistic:          -6.4,
			SubscriptionName:    "Chromium Perf Sheriff",
			BugComponent:        "Chromium>Perf",
			BugLabels:           []string{"Perf-Regression"},
			BugCcEmails:         []string{"sheriff@chromium.org"},
			BisectIDs:           []string{},
		},
		"789": {
			ID:                  "789",
			TestPath:            "ChromiumPerf/linux-r350-perf/speedometer3.crossbench/Score/Speedometer3/Total",
			TraceKey:            ",benchmark=speedometer3.crossbench,bot=linux-r350-perf,master=ChromiumPerf,stat=value,subtest_1=Speedometer3,subtest_2=Total,test=Score,unit=score_biggerIsBetter,",
			BugID:               0,
			StartRevision:       67131,
			EndRevision:         67135,
			DisplayCommitNumber: 67134,
			StartRevisionHash:   "deadbeef11",
			EndRevisionHash:     "deadbeef15",
			IsImprovement:       false,
			Recovered:           false,
			State:               "regression",
			Statistic:           "value",
			Units:               "score_biggerIsBetter",
			DegreesOfFreedom:    10.0,
			MedianBeforeAnomaly: 21.8,
			MedianAfterAnomaly:  19.7,
			PValue:              0.005,
			SegmentSizeBefore:   5,
			SegmentSizeAfter:    5,
			StdDevBeforeAnomaly: 1.0,
			TStatistic:          -4.8,
			SubscriptionName:    "Chromium Perf Sheriff",
			BugComponent:        "Chromium>Perf",
			BugLabels:           []string{"Perf-Regression"},
			BugCcEmails:         []string{"sheriff@chromium.org"},
			BisectIDs:           []string{},
		},
		"101": {
			ID:                  "101",
			TestPath:            "V8Perf/MacM4/v8/JetStream2/default/Total",
			TraceKey:            ",benchmark=v8,bot=MacM4,master=V8Perf,stat=value,subtest_1=default,subtest_2=Total,test=JetStream2,unit=score_biggerIsBetter,",
			BugID:               0,
			StartRevision:       67136,
			EndRevision:         67137,
			DisplayCommitNumber: 67136,
			StartRevisionHash:   "deadbeef16",
			EndRevisionHash:     "deadbeef17",
			IsImprovement:       true,
			Recovered:           false,
			State:               "improvement",
			Statistic:           "value",
			Units:               "score_biggerIsBetter",
			DegreesOfFreedom:    10.0,
			MedianBeforeAnomaly: 215.0,
			MedianAfterAnomaly:  245.0,
			PValue:              0.001,
			SegmentSizeBefore:   5,
			SegmentSizeAfter:    4,
			StdDevBeforeAnomaly: 2.0,
			TStatistic:          7.2,
			SubscriptionName:    "V8 Perf Sheriff",
			BugComponent:        "V8>Perf",
			BugLabels:           []string{"Perf-Improvement"},
			BugCcEmails:         []string{"v8-sheriff@chromium.org"},
			BisectIDs:           []string{},
		},
		"1001": {
			ID:                  "1001",
			TestPath:            "V8Perf/MacM4/v8/JetStream2/future/Total",
			TraceKey:            ",benchmark=v8,bot=MacM4,master=V8Perf,stat=value,subtest_1=future,subtest_2=Total,test=JetStream2,unit=score_biggerIsBetter,",
			BugID:               0,
			StartRevision:       67136,
			EndRevision:         67138,
			DisplayCommitNumber: 67138,
			StartRevisionHash:   "deadbeef16",
			EndRevisionHash:     "deadbeef18",
			IsImprovement:       true,
			Recovered:           false,
			State:               "improvement",
			Statistic:           "value",
			Units:               "score_biggerIsBetter",
			DegreesOfFreedom:    10.0,
			MedianBeforeAnomaly: 210.0,
			MedianAfterAnomaly:  235.0,
			PValue:              0.001,
			SegmentSizeBefore:   5,
			SegmentSizeAfter:    4,
			StdDevBeforeAnomaly: 1.5,
			TStatistic:          6.8,
			SubscriptionName:    "V8 JavaScript Perf",
			BugComponent:        "V8>Perf",
			BugLabels:           []string{"Perf-Improvement"},
			BugCcEmails:         []string{"v8-sheriff@chromium.org"},
			BisectIDs:           []string{},
		},
		"1002": {
			ID:                  "1002",
			TestPath:            "V8Perf/MacM4/v8/Octane2/default/Air",
			TraceKey:            ",benchmark=v8,bot=MacM4,master=V8Perf,stat=value,subtest_1=default,subtest_2=Air,test=Octane2,unit=score_biggerIsBetter,",
			BugID:               12345,
			StartRevision:       67125,
			EndRevision:         67126,
			DisplayCommitNumber: 67126,
			StartRevisionHash:   "deadbeef05",
			EndRevisionHash:     "deadbeef06",
			IsImprovement:       false,
			Recovered:           false,
			State:               "triaged",
			Statistic:           "value",
			Units:               "score_biggerIsBetter",
			DegreesOfFreedom:    10.0,
			MedianBeforeAnomaly: 315.0,
			MedianAfterAnomaly:  280.0,
			PValue:              0.002,
			SegmentSizeBefore:   5,
			SegmentSizeAfter:    5,
			StdDevBeforeAnomaly: 1.8,
			TStatistic:          -5.5,
			SubscriptionName:    "V8 JavaScript Perf",
			BugComponent:        "V8>Perf",
			BugLabels:           []string{"Perf-Regression"},
			BugCcEmails:         []string{"v8-sheriff@chromium.org"},
			BisectIDs:           []string{},
		},
	}
}
