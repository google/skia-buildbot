// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package insights

import (
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/types"
)

// subtestKeys are the trace params that make up the subtest part of a test path, in order.
var subtestKeys = []string{"subtest_1", "subtest_2", "subtest_3", "subtest_4", "subtest_5", "subtest_6", "subtest_7"}

// maxTriageMessageLength bounds the triage message included in responses.
const maxTriageMessageLength = 200

// traceKeys returns the sorted trace keys stored in the regression's frame. The detector keeps
// the keys of the traces in the cluster, but not their values.
func traceKeys(reg *regression.Regression) []string {
	if reg.Frame == nil || reg.Frame.DataFrame == nil {
		return nil
	}
	keys := make([]string, 0, len(reg.Frame.DataFrame.TraceSet))
	for k := range reg.Frame.DataFrame.TraceSet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// parseKey parses a structured trace key into its params. Keys that fail strict validation
// (e.g. unsorted params or unusual characters) are parsed leniently.
func parseKey(key string) (map[string]string, error) {
	params, err := query.ParseKey(key)
	if err != nil {
		return query.ParseKeyFast(key)
	}
	return params, nil
}

// parseTestInfo parses a structured trace key such as ",benchmark=b,bot=x,master=m,test=t,".
func parseTestInfo(key string) TestInfo {
	info := TestInfo{TraceKey: key}
	if key == "" {
		info.TestPath = "unknown"
		return info
	}
	params, err := parseKey(key)
	if err != nil {
		info.TestPath = key
		return info
	}
	info.Master = params["master"]
	info.Bot = params["bot"]
	info.Benchmark = params["benchmark"]
	info.Test = params["test"]
	for _, k := range subtestKeys {
		if v := params[k]; v != "" {
			info.Subtests = append(info.Subtests, v)
		}
	}
	info.Unit = params["unit"]
	if info.Unit == "" {
		info.Unit = params["units"]
	}
	parts := []string{}
	for _, p := range append([]string{info.Master, info.Bot, info.Benchmark, info.Test}, info.Subtests...) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		info.TestPath = key
	} else {
		info.TestPath = strings.Join(parts, "/")
	}
	return info
}

// triageOf returns the triage state of a regression.
func triageOf(reg *regression.Regression) Triage {
	_, _, ts := reg.GetClusterTypeAndSummaryAndTriageStatus()
	status := string(ts.Status)
	if status == "" {
		status = "none"
	}
	bugs := []string{}
	seen := map[string]bool{}
	for _, b := range reg.Bugs {
		if b.BugId == "" || b.BugId == "0" || seen[b.BugId] {
			continue
		}
		seen[b.BugId] = true
		bugs = append(bugs, b.BugId)
	}
	return Triage{Status: status, Message: truncate(ts.Message, maxTriageMessageLength), BugIDs: bugs}
}

// finite returns v, or 0 if v is NaN or infinite (which JSON cannot represent).
func finite(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// roundSig rounds v to the given number of significant digits, so float32 values from the
// database show up as 52.9022 rather than 52.90219497680664.
func roundSig(v float64, digits int) float64 {
	v = finite(v)
	if v == 0 {
		return 0
	}
	f, err := strconv.ParseFloat(strconv.FormatFloat(v, 'g', digits, 64), 64)
	if err != nil {
		return v
	}
	return f
}

// pctChange returns the percent change from before to after, rounded to 2 decimals, or nil if
// before is 0.
func pctChange(before, after float64) *float64 {
	if before == 0 {
		return nil
	}
	p := math.Round((after-before)/math.Abs(before)*10000) / 100
	p = finite(p)
	return &p
}

// formatNumber formats a measurement for headlines.
func formatNumber(v float64) string {
	return strconv.FormatFloat(v, 'g', 5, 64)
}

// headline returns a deterministic one-line description of an anomaly.
func headline(s AnomalySummary) string {
	verb := "regressed"
	if s.Direction == DirectionImprovement {
		verb = "improved"
	}
	change := ""
	if s.PctChange != nil {
		change = fmt.Sprintf(" %+.1f%%", *s.PctChange)
	}
	unit := ""
	if s.Test.Unit != "" {
		unit = " " + s.Test.Unit
	}
	more := ""
	if s.NumTraces > 1 {
		more = fmt.Sprintf(" (+%d more traces)", s.NumTraces-1)
	}
	triage := s.Triage.Status
	if len(s.Triage.BugIDs) > 0 {
		triage += ", bug " + s.Triage.BugIDs[0]
	}
	return fmt.Sprintf("%s%s %s%s (%s -> %s%s) in commits %d-%d; %s.",
		s.Test.TestPath, more, verb, change,
		formatNumber(s.MedianBefore), formatNumber(s.MedianAfter), unit,
		s.CommitRange.Start, s.CommitRange.End, triage)
}

// reportURL returns the link to the anomaly in the Perf report page (/u). It mirrors
// urlprovider.GroupReport.
func reportURL(instanceURL, id string) string {
	if instanceURL == "" || id == "" {
		return ""
	}
	return strings.TrimRight(instanceURL, "/") + "/u/?" + url.Values{"anomalyIDs": []string{id}}.Encode()
}

// centroidMedian returns the median of the valid values of v, and false if there are none.
func centroidMedian(v []float32) (float64, bool) {
	values := make([]float64, 0, len(v))
	for _, x := range v {
		f := float64(x)
		if x == vec32.MissingDataSentinel || math.IsNaN(f) || math.IsInf(f, 0) {
			continue
		}
		values = append(values, f)
	}
	if len(values) == 0 {
		return 0, false
	}
	sort.Float64s(values)
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return values[mid], true
	}
	return (values[mid-1] + values[mid]) / 2, true
}

// mediansOf returns the medians of the anomaly's values before and after the step.
//
// Perf computes them with vec32.TwoSidedStdDev, which needs at least 4 points on each side, so
// alerts with a small radius store 0 for both. In that case the medians of the stored centroid on
// each side of the step are returned instead, which is what Perf computes when it has enough
// points.
func mediansOf(reg *regression.Regression) (float64, float64) {
	before, after := float64(reg.MedianBefore), float64(reg.MedianAfter)
	if before != 0 || after != 0 {
		return before, after
	}
	_, cs, _ := reg.GetClusterTypeAndSummaryAndTriageStatus()
	if cs == nil || cs.StepFit == nil {
		return before, after
	}
	i := cs.StepFit.TurningPoint
	if i <= 0 || i >= len(cs.Centroid) {
		return before, after
	}
	b, okBefore := centroidMedian(cs.Centroid[:i])
	a, okAfter := centroidMedian(cs.Centroid[i:])
	if !okBefore || !okAfter {
		return before, after
	}
	return b, a
}

// toSummary converts a regression into an AnomalySummary.
func (s *Service) toSummary(reg *regression.Regression) AnomalySummary {
	keys := traceKeys(reg)
	test := parseTestInfo("")
	if len(keys) > 0 {
		test = parseTestInfo(keys[0])
	}
	medianBefore, medianAfter := mediansOf(reg)
	before := roundSig(medianBefore, 6)
	after := roundSig(medianAfter, 6)
	direction := DirectionRegression
	if reg.IsImprovement {
		direction = DirectionImprovement
	}
	display := int64(reg.DisplayCommitNumber)
	if display == 0 {
		display = int64(reg.CommitNumber)
	}
	sum := AnomalySummary{
		ID:           reg.Id,
		Sheriff:      reg.SubscriptionName,
		Test:         test,
		NumTraces:    len(keys),
		Direction:    direction,
		MedianBefore: before,
		MedianAfter:  after,
		PctChange:    pctChange(before, after),
		AbsDelta:     roundSig(math.Abs(after-before), 6),
		CommitRange: CommitRange{
			Start:   int64(reg.PrevCommitNumber) + 1,
			End:     int64(reg.CommitNumber),
			Display: display,
		},
		DetectedAt: reg.CreationTime.UTC(),
		Triage:     triageOf(reg),
		AlertID:    reg.AlertId,
		Links:      Links{Report: reportURL(s.opts.InstanceURL, reg.Id)},
	}
	sum.Headline = headline(sum)
	return sum
}

// windowOf returns the data around the anomaly from the regression's stored cluster summary and
// frame headers, without reading the trace store. For StepFit alerts the centroid holds the
// trace's own values; for k-means alerts it is a normalized centroid.
func windowOf(reg *regression.Regression, normalized bool) *ValueWindow {
	_, cs, _ := reg.GetClusterTypeAndSummaryAndTriageStatus()
	if cs == nil || len(cs.Centroid) == 0 || reg.Frame == nil || reg.Frame.DataFrame == nil {
		return nil
	}
	header := reg.Frame.DataFrame.Header
	n := len(cs.Centroid)
	if len(header) < n {
		n = len(header)
	}
	var stepAt int64
	if cs.StepPoint != nil {
		stepAt = int64(cs.StepPoint.Offset)
	}
	stepIdx := -1
	points := make([]WindowPoint, 0, n)
	for i := 0; i < n; i++ {
		h := header[i]
		v := cs.Centroid[i]
		if h == nil || v == vec32.MissingDataSentinel {
			continue
		}
		f := float64(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			continue
		}
		if stepAt != 0 && int64(h.Offset) == stepAt {
			stepIdx = len(points)
		}
		points = append(points, WindowPoint{Commit: int64(h.Offset), Timestamp: int64(h.Timestamp), Value: roundSig(f, 6)})
	}
	if len(points) == 0 {
		return nil
	}
	w := &ValueWindow{Normalized: normalized, StepAt: stepAt, Points: points}
	if len(points) > MaxWindowPoints {
		// Keep the points centered on the step.
		center := stepIdx
		if center < 0 {
			center = len(points) / 2
		}
		lo := center - MaxWindowPoints/2
		if lo < 0 {
			lo = 0
		}
		hi := lo + MaxWindowPoints
		if hi > len(points) {
			hi = len(points)
			lo = hi - MaxWindowPoints
		}
		w.Points = points[lo:hi]
		w.Truncated = true
	}
	return w
}

// alertInfoOf converts an alert config into an AlertInfo.
func alertInfoOf(id int64, a *alerts.Alert) *AlertInfo {
	algo := string(a.Algo)
	if algo == "" {
		algo = string(types.KMeansGrouping)
	}
	step := string(a.Step)
	if step == "" {
		step = "original"
	}
	return &AlertInfo{
		ID:               id,
		DisplayName:      a.DisplayName,
		Query:            a.Query,
		Algo:             algo,
		Step:             step,
		Radius:           a.Radius,
		Threshold:        a.Interesting,
		Direction:        string(a.DirectionAsString),
		GroupBy:          a.GroupBy,
		MinimumNum:       a.MinimumNum,
		Action:           string(a.Action),
		HasDetectionRule: a.DetectionRule != nil,
	}
}

// commitInfoOf converts a commit into a CommitInfo.
func commitInfoOf(c provider.Commit) CommitInfo {
	return CommitInfo{
		Number:    int64(c.CommitNumber),
		Hash:      c.GitHash,
		Author:    c.Author,
		Subject:   truncate(c.Subject, 200),
		URL:       c.URL,
		Timestamp: c.Timestamp,
	}
}

// absPct returns |pct_change|, or -1 if it is unknown, for sorting.
func absPct(s AnomalySummary) float64 {
	if s.PctChange == nil {
		return -1
	}
	return math.Abs(*s.PctChange)
}

// sortBySeverity sorts summaries by |pct_change| descending, then by ID for stability.
func sortBySeverity(sums []AnomalySummary) {
	sort.SliceStable(sums, func(i, j int) bool {
		a, b := absPct(sums[i]), absPct(sums[j])
		if a != b {
			return a > b
		}
		return sums[i].ID < sums[j].ID
	})
}
