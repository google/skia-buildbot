// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package insights

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/stepfit"
)

var (
	quietBefore = []float32{10, 10.1, 9.9, 10, 10.05, 9.95}
	stepAfter   = []float32{20, 20.1, 19.9, 20, 20.05, 19.95}
)

// withWindow stores the values around the step the way StepFit does: the centroid holds the
// trace's own values, and the turning point is the index of the first value after the step.
func withWindow(r *regression.Regression, before, after []float32) *regression.Regression {
	r.High.Centroid = append(append([]float32{}, before...), after...)
	r.High.StepFit = &stepfit.StepFit{TurningPoint: len(before)}
	return r
}

func TestStepQualityOf(t *testing.T) {
	clean := stepQualityOf(withWindow(newReg(regOpts{n: 1}), quietBefore, stepAfter))
	assert.True(t, clean.OK)
	assert.Greater(t, clean.Effect, 100.0)
	assert.False(t, clean.Recovered)

	noisy := stepQualityOf(withWindow(newReg(regOpts{n: 2}), []float32{10, 12, 8, 11, 9, 10}, []float32{10.5, 11.5, 9.5, 10.4, 10.6, 10}))
	assert.True(t, noisy.OK)
	assert.Less(t, noisy.Effect, noiseEffect)

	spike := stepQualityOf(withWindow(newReg(regOpts{n: 3}), quietBefore, []float32{20, 20, 20, 20, 10, 10}))
	assert.True(t, spike.OK)
	assert.True(t, spike.Recovered, "the last values went back to the earlier level")

	assert.False(t, stepQualityOf(withWindow(newReg(regOpts{n: 4}), []float32{1}, []float32{2})).OK, "too few points")
	assert.False(t, stepQualityOf(newReg(regOpts{n: 5})).OK, "no step fit")
}

func TestSuggestTriage_SortsUntriagedAnomaliesIntoSuggestions(t *testing.T) {
	regs := []*regression.Regression{
		// Three clean regressions whose ranges all contain commit 104.
		withWindow(newReg(regOpts{n: 1, prev: 99, curr: 105, before: 10, after: 20}), quietBefore, stepAfter),
		withWindow(newReg(regOpts{n: 2, prev: 102, curr: 108, before: 10, after: 15}), quietBefore, stepAfter),
		withWindow(newReg(regOpts{n: 3, prev: 103, curr: 104, before: 10, after: 12}), quietBefore, stepAfter),
		// A clean regression on its own.
		withWindow(newReg(regOpts{n: 4, prev: 199, curr: 201, before: 10, after: 30}), quietBefore, stepAfter),
		// Noise, and a spike that recovered.
		withWindow(newReg(regOpts{n: 5, prev: 299, curr: 300, before: 10, after: 10.5}), []float32{10, 12, 8, 11, 9, 10}, []float32{10.5, 11.5, 9.5, 10.4, 10.6, 10}),
		withWindow(newReg(regOpts{n: 6, prev: 399, curr: 400, before: 10, after: 20}), quietBefore, []float32{20, 20, 20, 20, 10, 10}),
		// An improvement.
		withWindow(newReg(regOpts{n: 7, prev: 499, curr: 500, before: 20, after: 10, improvement: true}), quietBefore, stepAfter),
		// Left out: already triaged, and older than the default 1 day.
		withWindow(newReg(regOpts{n: 8, prev: 99, curr: 105, before: 10, after: 20, status: regression.Triaged}), quietBefore, stepAfter),
		withWindow(newReg(regOpts{n: 9, age: 48 * time.Hour, prev: 99, curr: 105, before: 10, after: 20}), quietBefore, stepAfter),
	}
	svc, _ := newService(t, regs...)

	plan, err := svc.SuggestTriage(context.Background(), SuggestTriageRequest{Sheriff: sheriff})
	require.NoError(t, err)
	assert.Equal(t, DefaultTriageDays, plan.Scope.Days)
	assert.Equal(t, 7, plan.Untriaged)
	assert.Contains(t, plan.Note, "never changes triage")

	actions := []string{}
	for _, s := range plan.Suggestions {
		actions = append(actions, s.Action)
	}
	require.Equal(t, []string{ActionFileOneBug, ActionFileBug, ActionIgnoreLikelyNoise, ActionReviewImprovements}, actions)

	group := plan.Suggestions[0]
	assert.Equal(t, 3, group.Count)
	assert.Equal(t, &[2]int64{104, 104}, group.CommitRange)
	assert.Equal(t, id(1), group.Representative.ID, "the biggest change represents the group")
	assert.Equal(t, []string{id(1), id(2), id(3)}, group.AnomalyIDs)
	assert.Equal(t, "https://perf.example.com/u/?anomalyIDs="+id(1)+"%2C"+id(2)+"%2C"+id(3), group.TriageURL)
	require.NotNil(t, group.BugDraft)
	assert.Equal(t, "[demo] 3 regressions in 8888 at commit 104", group.BugDraft.Title)
	assert.Contains(t, group.BugDraft.Description, "Review it before filing")
	assert.Equal(t, []string{"x86"}, group.Bots)

	bug := plan.Suggestions[1]
	assert.Equal(t, []string{id(4)}, bug.AnomalyIDs)
	assert.Equal(t, "https://perf.example.com/u/?anomalyIDs="+id(4), bug.TriageURL)
	require.NotNil(t, bug.BugDraft)
	assert.True(t, strings.HasPrefix(bug.BugDraft.Title, "[demo] demo/x86/8888/encode +200.0%"), bug.BugDraft.Title)

	noise := plan.Suggestions[2]
	assert.Equal(t, 2, noise.Count)
	assert.ElementsMatch(t, []string{id(5), id(6)}, noise.AnomalyIDs)
	assert.Nil(t, noise.BugDraft)

	assert.Equal(t, []string{id(7)}, plan.Suggestions[3].AnomalyIDs)

	plan, err = svc.SuggestTriage(context.Background(), SuggestTriageRequest{Sheriff: sheriff, Days: 3})
	require.NoError(t, err)
	assert.Equal(t, 8, plan.Untriaged, "the 2-day-old anomaly is included")
}

func TestSuggestTriage_BigGroupsLinkToTheRevision(t *testing.T) {
	regs := []*regression.Regression{}
	for i := 1; i <= MaxSuggestionIDs+5; i++ {
		regs = append(regs, withWindow(newReg(regOpts{n: i, prev: 499, curr: 500, before: 10, after: float32(10 + i)}), quietBefore, stepAfter))
	}
	svc, _ := newService(t, regs...)
	plan, err := svc.SuggestTriage(context.Background(), SuggestTriageRequest{Sheriff: sheriff})
	require.NoError(t, err)
	require.Len(t, plan.Suggestions, 1)
	group := plan.Suggestions[0]
	assert.Equal(t, MaxSuggestionIDs+5, group.Count)
	assert.Len(t, group.AnomalyIDs, MaxSuggestionIDs)
	assert.Equal(t, "https://perf.example.com/u/?rev=500", group.TriageURL)
	assert.Contains(t, group.BugDraft.Description, fmt.Sprintf("and %d more", MaxSuggestionIDs+5-maxDraftLines))
}

func TestSuggestTriage_InvalidInput(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.SuggestTriage(context.Background(), SuggestTriageRequest{Sheriff: "nope"})
	assert.Equal(t, CodeInvalidArgument, publicCode(t, err))
	_, err = svc.SuggestTriage(context.Background(), SuggestTriageRequest{Sheriff: sheriff, Days: MaxDays + 1})
	assert.Equal(t, CodeInvalidArgument, publicCode(t, err))
}
