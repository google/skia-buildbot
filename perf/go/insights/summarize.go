// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package insights

import (
	"math"
	"sort"
)

const (
	topRegressionsCount  = 5
	maxSummaryRows       = 10
	maxOverlapClusters   = 5
	maxOverlapIDs        = 10
	maxOverlapBenchmarks = 5
	maxRepeatOffenders   = 5
	unknownLabel         = "(unknown)"
)

// summarize aggregates anomalies into an InsightSummary. The scope is set by the caller.
func summarize(sums []AnomalySummary) *InsightSummary {
	ret := &InsightSummary{
		TopRegressions:    []AnomalySummary{},
		ByBenchmark:       []BenchmarkCount{},
		ByBot:             []BotCount{},
		OverlappingRanges: []OverlapCluster{},
		RepeatOffenders:   []RepeatOffender{},
	}
	regressions := []AnomalySummary{}
	for _, s := range sums {
		if s.Direction == DirectionImprovement {
			ret.Totals.Improvements++
		} else {
			ret.Totals.Regressions++
			regressions = append(regressions, s)
		}
		switch s.Triage.Status {
		case StatusUntriaged:
			ret.Totals.Untriaged++
		case StatusTriaged:
			ret.Totals.Triaged++
		case StatusIgnored:
			ret.Totals.Ignored++
		case StatusPending:
			ret.Totals.Pending++
		default:
			ret.Totals.Other++
		}
	}

	top := append([]AnomalySummary(nil), regressions...)
	sortBySeverity(top)
	if len(top) > topRegressionsCount {
		top = top[:topRegressionsCount]
	}
	ret.TopRegressions = top

	benchCounts := map[string]int{}
	benchPcts := map[string][]float64{}
	botCounts := map[string]int{}
	testCounts := map[string]int{}
	for _, r := range regressions {
		b := r.Test.Benchmark
		if b == "" {
			b = unknownLabel
		}
		benchCounts[b]++
		if r.PctChange != nil {
			benchPcts[b] = append(benchPcts[b], *r.PctChange)
		}
		bot := r.Test.Bot
		if bot == "" {
			bot = unknownLabel
		}
		botCounts[bot]++
		testCounts[r.Test.TestPath]++
	}

	for b, n := range benchCounts {
		bc := BenchmarkCount{Benchmark: b, Regressions: n}
		if m, ok := median(benchPcts[b]); ok {
			bc.MedianPctChange = &m
		}
		ret.ByBenchmark = append(ret.ByBenchmark, bc)
	}
	sort.Slice(ret.ByBenchmark, func(i, j int) bool {
		if ret.ByBenchmark[i].Regressions != ret.ByBenchmark[j].Regressions {
			return ret.ByBenchmark[i].Regressions > ret.ByBenchmark[j].Regressions
		}
		return ret.ByBenchmark[i].Benchmark < ret.ByBenchmark[j].Benchmark
	})
	if len(ret.ByBenchmark) > maxSummaryRows {
		ret.ByBenchmark = ret.ByBenchmark[:maxSummaryRows]
	}

	for b, n := range botCounts {
		ret.ByBot = append(ret.ByBot, BotCount{Bot: b, Regressions: n})
	}
	sort.Slice(ret.ByBot, func(i, j int) bool {
		if ret.ByBot[i].Regressions != ret.ByBot[j].Regressions {
			return ret.ByBot[i].Regressions > ret.ByBot[j].Regressions
		}
		return ret.ByBot[i].Bot < ret.ByBot[j].Bot
	})
	if len(ret.ByBot) > maxSummaryRows {
		ret.ByBot = ret.ByBot[:maxSummaryRows]
	}

	for t, n := range testCounts {
		if n >= 2 {
			ret.RepeatOffenders = append(ret.RepeatOffenders, RepeatOffender{TestPath: t, Anomalies: n})
		}
	}
	sort.Slice(ret.RepeatOffenders, func(i, j int) bool {
		if ret.RepeatOffenders[i].Anomalies != ret.RepeatOffenders[j].Anomalies {
			return ret.RepeatOffenders[i].Anomalies > ret.RepeatOffenders[j].Anomalies
		}
		return ret.RepeatOffenders[i].TestPath < ret.RepeatOffenders[j].TestPath
	})
	if len(ret.RepeatOffenders) > maxRepeatOffenders {
		ret.RepeatOffenders = ret.RepeatOffenders[:maxRepeatOffenders]
	}

	ret.OverlappingRanges = overlapClusters(regressions)
	return ret
}

// median returns the median of v rounded to 2 decimals, and false if v is empty.
func median(v []float64) (float64, bool) {
	if len(v) == 0 {
		return 0, false
	}
	c := append([]float64(nil), v...)
	sort.Float64s(c)
	m := c[len(c)/2]
	if len(c)%2 == 0 {
		m = (c[len(c)/2-1] + c[len(c)/2]) / 2
	}
	return math.Round(m*100) / 100, true
}

// rangeGroup is a set of anomalies whose commit ranges all contain [lo, hi].
type rangeGroup struct {
	lo, hi  int64
	members []AnomalySummary
}

// groupByCommitRange greedily groups anomalies whose commit ranges intersect, like anomaly
// grouping does: an anomaly joins the current group if its range overlaps the group's common
// range, which then narrows to the intersection. Anomalies in one group may share a culprit.
func groupByCommitRange(regs []AnomalySummary) []*rangeGroup {
	sorted := append([]AnomalySummary(nil), regs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].CommitRange.Start != sorted[j].CommitRange.Start {
			return sorted[i].CommitRange.Start < sorted[j].CommitRange.Start
		}
		return sorted[i].CommitRange.End < sorted[j].CommitRange.End
	})
	var groups []*rangeGroup
	var cur *rangeGroup
	for _, r := range sorted {
		start, end := r.CommitRange.Start, r.CommitRange.End
		if end < start {
			start, end = end, start
		}
		if cur != nil && start <= cur.hi && end >= cur.lo {
			if start > cur.lo {
				cur.lo = start
			}
			if end < cur.hi {
				cur.hi = end
			}
			cur.members = append(cur.members, r)
			continue
		}
		cur = &rangeGroup{lo: start, hi: end, members: []AnomalySummary{r}}
		groups = append(groups, cur)
	}
	return groups
}

// overlapClusters returns the groups of 2+ regressions whose commit ranges intersect, largest
// first. See groupByCommitRange.
func overlapClusters(regs []AnomalySummary) []OverlapCluster {
	ret := []OverlapCluster{}
	for _, c := range groupByCommitRange(regs) {
		if len(c.members) < 2 {
			continue
		}
		sortBySeverity(c.members)
		oc := OverlapCluster{CommitRange: [2]int64{c.lo, c.hi}, Count: len(c.members), Benchmarks: []string{}, AnomalyIDs: []string{}}
		seen := map[string]bool{}
		for _, m := range c.members {
			if b := m.Test.Benchmark; b != "" && !seen[b] && len(oc.Benchmarks) < maxOverlapBenchmarks {
				seen[b] = true
				oc.Benchmarks = append(oc.Benchmarks, b)
			}
			if len(oc.AnomalyIDs) < maxOverlapIDs {
				oc.AnomalyIDs = append(oc.AnomalyIDs, m.ID)
			}
		}
		ret = append(ret, oc)
	}
	sort.SliceStable(ret, func(i, j int) bool { return ret[i].Count > ret[j].Count })
	if len(ret) > maxOverlapClusters {
		ret = ret[:maxOverlapClusters]
	}
	return ret
}
