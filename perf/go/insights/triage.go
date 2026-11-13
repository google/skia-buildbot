// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package insights

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/regression"
)

const (
	// DefaultTriageDays is the default period of SuggestTriage. Busy sheriffs get thousands of
	// anomalies a week, so it is short.
	DefaultTriageDays = 1

	// MaxSuggestionIDs is the maximum number of anomaly ids listed per suggestion.
	MaxSuggestionIDs = 25

	// maxGroupSuggestions and maxBugSuggestions bound the file_one_bug and file_bug suggestions.
	maxGroupSuggestions = 5
	maxBugSuggestions   = 5

	// Step quality is the change of the median in robust standard deviations of the values before
	// the step. cleanStepEffect is the minimum for a regression worth its own bug; below
	// noiseEffect, an anomaly is likely noise.
	cleanStepEffect = 4.0
	noiseEffect     = 2.0

	// An anomaly recovered if the median of the last third of its values after the step is closer
	// to the level before the step than recoveredFraction of the step.
	recoveredFraction = 0.3

	// minSegment is the minimum number of values on each side of the step to judge it.
	minSegment = 3

	// maxDraftLines is the maximum number of anomalies listed in a bug draft.
	maxDraftLines = 5

	triageNote = "Suggestions from simple heuristics on the data Perf stored with each anomaly: " +
		"overlapping commit ranges, the size of the step compared to the noise before it, and " +
		"whether the values went back to their earlier level. Review them, then apply triage in " +
		"Perf at triage_url, where your identity and permissions apply. perfinsights never changes triage."
)

// stepQuality describes how clear an anomaly's step is in the data stored with it.
type stepQuality struct {
	// Effect is the change of the median, in robust standard deviations of the values before the
	// step.
	Effect float64

	// Recovered is true if the last values went back towards the level before the step.
	Recovered bool

	// OK is false if there is too little data to judge.
	OK bool
}

// finiteValues returns the values of v that aren't missing, NaN or infinite.
func finiteValues(v []float32) []float64 {
	ret := make([]float64, 0, len(v))
	for _, x := range v {
		f := float64(x)
		if x == vec32.MissingDataSentinel || math.IsNaN(f) || math.IsInf(f, 0) {
			continue
		}
		ret = append(ret, f)
	}
	return ret
}

// medianOf returns the median of v, which must not be empty.
func medianOf(v []float64) float64 {
	c := append([]float64(nil), v...)
	sort.Float64s(c)
	mid := len(c) / 2
	if len(c)%2 == 1 {
		return c[mid]
	}
	return (c[mid-1] + c[mid]) / 2
}

// stepQualityOf computes the step quality of a regression from its stored cluster summary. For
// k-means anomalies the centroid is normalized, which doesn't change the ratio.
func stepQualityOf(reg *regression.Regression) stepQuality {
	_, cs, _ := reg.GetClusterTypeAndSummaryAndTriageStatus()
	if cs == nil || cs.StepFit == nil {
		return stepQuality{}
	}
	i := cs.StepFit.TurningPoint
	if i <= 0 || i >= len(cs.Centroid) {
		return stepQuality{}
	}
	before := finiteValues(cs.Centroid[:i])
	after := finiteValues(cs.Centroid[i:])
	if len(before) < minSegment || len(after) < minSegment {
		return stepQuality{}
	}
	mb := medianOf(before)
	ma := medianOf(after)
	deviations := make([]float64, len(before))
	for j, v := range before {
		deviations[j] = math.Abs(v - mb)
	}
	// 1.4826 * MAD estimates the standard deviation of normally distributed data. Flat data gets a
	// floor of 0.1% of its level.
	sigma := 1.4826 * medianOf(deviations)
	if floor := 1e-9 + 0.001*math.Abs(mb); sigma < floor {
		sigma = floor
	}
	step := math.Abs(ma - mb)
	q := stepQuality{Effect: step / sigma, OK: true}
	tail := after[len(after)-(len(after)+2)/3:]
	if step > 0 {
		q.Recovered = math.Abs(medianOf(tail)-mb) < recoveredFraction*step
	}
	return q
}

// triageItem is an untriaged anomaly and the quality of its step.
type triageItem struct {
	sum AnomalySummary
	q   stepQuality
}

// SuggestTriage proposes how to triage a sheriff's untriaged anomalies of the last days (default
// DefaultTriageDays). It only reads: people review the suggestions and apply them in the Perf UI.
func (s *Service) SuggestTriage(ctx context.Context, req SuggestTriageRequest) (*TriagePlan, error) {
	req.Sheriff = strings.TrimSpace(req.Sheriff)
	if err := s.checkSheriff(ctx, req.Sheriff); err != nil {
		return nil, err
	}
	days := req.Days
	if days == 0 {
		days = DefaultTriageDays
	}
	days, err := normalizeDays(days)
	if err != nil {
		return nil, err
	}
	since := s.opts.Now().Add(-time.Duration(days) * 24 * time.Hour)
	items := []triageItem{}
	_, scanned, _, hitLimit, err := s.scan(ctx, req.Sheriff, true, since, 0, MaxScanRows, func(r *regression.Regression) bool {
		sum := s.toSummary(r)
		if sum.Triage.Status == StatusUntriaged {
			items = append(items, triageItem{sum: sum, q: stepQualityOf(r)})
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return &TriagePlan{
		Scope: SummaryScope{
			Sheriff:      req.Sheriff,
			Days:         days,
			Since:        since.UTC(),
			TotalScanned: scanned,
			Truncated:    hitLimit,
		},
		Untriaged:   len(items),
		Suggestions: s.suggestions(req.Sheriff, items),
		Note:        triageNote,
	}, nil
}

// suggestions sorts untriaged anomalies into triage suggestions: groups that may share a cause,
// clean regressions worth a bug, likely noise, improvements, and the rest.
func (s *Service) suggestions(sheriff string, items []triageItem) []TriageSuggestion {
	var improvements, noise, regs []AnomalySummary
	quality := map[string]stepQuality{}
	for _, it := range items {
		quality[it.sum.ID] = it.q
		switch {
		case it.sum.Direction == DirectionImprovement:
			improvements = append(improvements, it.sum)
		case it.q.OK && (it.q.Effect < noiseEffect || it.q.Recovered):
			noise = append(noise, it.sum)
		default:
			regs = append(regs, it.sum)
		}
	}

	ret := []TriageSuggestion{}
	grouped := map[string]bool{}
	groups := groupByCommitRange(regs)
	sort.SliceStable(groups, func(i, j int) bool { return len(groups[i].members) > len(groups[j].members) })
	for _, g := range groups {
		if len(g.members) < 2 || len(ret) >= maxGroupSuggestions {
			continue
		}
		for _, m := range g.members {
			grouped[m.ID] = true
		}
		ret = append(ret, s.groupSuggestion(sheriff, g))
	}

	var clean, rest []AnomalySummary
	for _, sum := range regs {
		if grouped[sum.ID] {
			continue
		}
		if q := quality[sum.ID]; q.OK && q.Effect >= cleanStepEffect && !q.Recovered {
			clean = append(clean, sum)
		} else {
			rest = append(rest, sum)
		}
	}
	sortBySeverity(clean)
	for i, sum := range clean {
		if i >= maxBugSuggestions {
			rest = append(rest, clean[i:]...)
			break
		}
		ret = append(ret, s.bugSuggestion(sum, quality[sum.ID]))
	}

	if len(noise) > 0 {
		ret = append(ret, s.listSuggestion(ActionIgnoreLikelyNoise,
			fmt.Sprintf("%d anomalies look like noise", len(noise)),
			"Their steps are small compared to the noise before them, or their values went back to the earlier level. "+
				"Consider ignoring them after a quick look at their traces.",
			noise))
	}
	if len(improvements) > 0 {
		ret = append(ret, s.listSuggestion(ActionReviewImprovements,
			fmt.Sprintf("%d untriaged improvements", len(improvements)),
			"Perf marked them as improvements. They usually need no bug; check that none is an unexpected change.",
			improvements))
	}
	if len(rest) > 0 {
		ret = append(ret, s.listSuggestion(ActionReview,
			fmt.Sprintf("%d other regressions to review", len(rest)),
			"The stored data doesn't show them as clearly real or as noise, or has too few points to judge. Check their traces.",
			rest))
	}
	return ret
}

func commitRangeText(lo, hi int64) string {
	if lo == hi {
		return fmt.Sprintf("commit %d", lo)
	}
	return fmt.Sprintf("commits %d-%d", lo, hi)
}

func pctText(sum AnomalySummary) string {
	if sum.PctChange == nil {
		return ""
	}
	return fmt.Sprintf(" %+.1f%%", *sum.PctChange)
}

// idsOf returns the ids of sums, up to MaxSuggestionIDs.
func idsOf(sums []AnomalySummary) []string {
	ret := []string{}
	for _, sum := range sums {
		if len(ret) >= MaxSuggestionIDs {
			break
		}
		ret = append(ret, sum.ID)
	}
	return ret
}

// labelCount is how many anomalies have a label, e.g. a bot.
type labelCount struct {
	label string
	count int
}

// countLabels counts the labels of sums, most frequent first.
func countLabels(sums []AnomalySummary, label func(AnomalySummary) string) []labelCount {
	counts := map[string]int{}
	for _, sum := range sums {
		if l := label(sum); l != "" {
			counts[l]++
		}
	}
	ret := make([]labelCount, 0, len(counts))
	for l, n := range counts {
		ret = append(ret, labelCount{l, n})
	}
	sort.Slice(ret, func(i, j int) bool {
		if ret[i].count != ret[j].count {
			return ret[i].count > ret[j].count
		}
		return ret[i].label < ret[j].label
	})
	return ret
}

// topLabels returns the most frequent labels, up to maxOverlapBenchmarks.
func topLabels(counts []labelCount) []string {
	ret := []string{}
	for _, c := range counts {
		if len(ret) >= maxOverlapBenchmarks {
			break
		}
		ret = append(ret, c.label)
	}
	return ret
}

func countsText(counts []labelCount) string {
	parts := []string{}
	for _, c := range counts {
		if len(parts) >= maxOverlapBenchmarks {
			parts = append(parts, "...")
			break
		}
		parts = append(parts, fmt.Sprintf("%s (%d)", c.label, c.count))
	}
	return strings.Join(parts, ", ")
}

func benchmarkOf(sum AnomalySummary) string { return sum.Test.Benchmark }
func botOf(sum AnomalySummary) string       { return sum.Test.Bot }

// perfURL returns a link to the Perf report page (/u), where anomalies can be triaged.
func (s *Service) perfURL(params url.Values) string {
	if s.opts.InstanceURL == "" {
		return ""
	}
	return strings.TrimRight(s.opts.InstanceURL, "/") + "/u/?" + params.Encode()
}

// triageURL links to the given anomalies (the first MaxSuggestionIDs of them). A positive
// revision that all of them contain is linked instead when there are more, so the link shows
// every anomaly at that revision.
func (s *Service) triageURL(sums []AnomalySummary, revision int64) string {
	if len(sums) > MaxSuggestionIDs && revision >= 0 {
		return s.perfURL(url.Values{"rev": {strconv.FormatInt(revision, 10)}})
	}
	return s.perfURL(url.Values{"anomalyIDs": {strings.Join(idsOf(sums), ",")}})
}

// draftFooter ends every bug draft.
const draftFooter = "\nDrafted by perfinsights from Perf data. Review it before filing."

func (s *Service) groupSuggestion(sheriff string, g *rangeGroup) TriageSuggestion {
	members := append([]AnomalySummary(nil), g.members...)
	sortBySeverity(members)
	rep := members[0]
	benchmarks := countLabels(members, benchmarkOf)
	bots := countLabels(members, botOf)
	rangeText := commitRangeText(g.lo, g.hi)
	triageURL := s.triageURL(members, g.hi)

	var b strings.Builder
	fmt.Fprintf(&b, "Perf detected %d regressions for the sheriff %q whose commit ranges all contain %s, so they may share one cause.\n\n", len(members), sheriff, rangeText)
	b.WriteString("Biggest changes:\n")
	for i, m := range members {
		if i >= maxDraftLines {
			fmt.Fprintf(&b, "- ... and %d more\n", len(members)-maxDraftLines)
			break
		}
		fmt.Fprintf(&b, "- %s %s\n", m.Headline, m.Links.Report)
	}
	fmt.Fprintf(&b, "\nBenchmarks: %s\nBots: %s\n", countsText(benchmarks), countsText(bots))
	if triageURL != "" {
		fmt.Fprintf(&b, "All of them in Perf: %s\n", triageURL)
	}
	b.WriteString(draftFooter)

	return TriageSuggestion{
		Action:         ActionFileOneBug,
		Title:          fmt.Sprintf("%d regressions at %s may share one cause", len(members), rangeText),
		Reason:         fmt.Sprintf("Their commit ranges all contain %s, so one bug, or one bisection, can cover them.", rangeText),
		Count:          len(members),
		AnomalyIDs:     idsOf(members),
		Representative: &rep,
		CommitRange:    &[2]int64{g.lo, g.hi},
		Benchmarks:     topLabels(benchmarks),
		Bots:           topLabels(bots),
		TriageURL:      triageURL,
		BugDraft: &BugDraft{
			Title:       fmt.Sprintf("[%s] %d regressions in %s at %s", s.opts.InstanceName, len(members), strings.Join(topLabels(benchmarks), ", "), rangeText),
			Description: b.String(),
		},
	}
}

func (s *Service) bugSuggestion(sum AnomalySummary, q stepQuality) TriageSuggestion {
	rangeText := commitRangeText(sum.CommitRange.Start, sum.CommitRange.End)
	unit := ""
	if sum.Test.Unit != "" {
		unit = " " + sum.Test.Unit
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", sum.Headline)
	fmt.Fprintf(&b, "Median: %s -> %s%s\n", formatNumber(sum.MedianBefore), formatNumber(sum.MedianAfter), unit)
	fmt.Fprintf(&b, "Suspect commits: %s\n", rangeText)
	if sum.Links.Report != "" {
		fmt.Fprintf(&b, "Perf: %s\n", sum.Links.Report)
	}
	b.WriteString(draftFooter)
	return TriageSuggestion{
		Action:         ActionFileBug,
		Title:          fmt.Sprintf("%s%s at %s", sum.Test.TestPath, pctText(sum), rangeText),
		Reason:         fmt.Sprintf("A clean step: the median moved %.0f times the noise before it, and the values stayed at the new level.", q.Effect),
		Count:          1,
		AnomalyIDs:     []string{sum.ID},
		Representative: &sum,
		Benchmarks:     topLabels(countLabels([]AnomalySummary{sum}, benchmarkOf)),
		Bots:           topLabels(countLabels([]AnomalySummary{sum}, botOf)),
		TriageURL:      sum.Links.Report,
		BugDraft: &BugDraft{
			Title:       fmt.Sprintf("[%s] %s%s", s.opts.InstanceName, sum.Test.TestPath, pctText(sum)),
			Description: b.String(),
		},
	}
}

func (s *Service) listSuggestion(action, title, reason string, sums []AnomalySummary) TriageSuggestion {
	sorted := append([]AnomalySummary(nil), sums...)
	sortBySeverity(sorted)
	rep := sorted[0]
	return TriageSuggestion{
		Action:         action,
		Title:          title,
		Reason:         reason,
		Count:          len(sorted),
		AnomalyIDs:     idsOf(sorted),
		Representative: &rep,
		Benchmarks:     topLabels(countLabels(sorted, benchmarkOf)),
		Bots:           topLabels(countLabels(sorted, botOf)),
		TriageURL:      s.triageURL(sorted, -1),
	}
}
