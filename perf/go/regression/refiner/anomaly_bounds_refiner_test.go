// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package refiner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
)

const (
	defaultStdDevThreshold = 0.001
)

func TestAnomalyBoundsRefiner_ValidResponses_ReturnsConfirmedRegressions(t *testing.T) {
	// Simple case: 0, 10, 0
	fullTrace := []float32{0, 10, 0, 0}
	inputs := []responseDef{
		{offset: 99, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: 0},
		{offset: 100, status: stepfit.LOW, regression: 10.0, useTraceIdx: 1},
		{offset: 101, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: 2},
	}
	runAnomalyTest(t, fullTrace, 2, inputs, []anomalyExpectation{{100, 99, 100}})
}

func TestAnomalyBoundsRefiner_EmptyInput_ReturnsEmpty(t *testing.T) {
	r := NewAnomalyBoundsRefiner(defaultStdDevThreshold)
	res, err := r.Process(context.Background(), &alerts.Alert{Algo: types.StepFitGrouping}, []*regression.RegressionDetectionResponse{})
	assert.NoError(t, err)
	assert.Empty(t, res)
}
func TestAnomalyBoundsRefiner_GroupsAdjacent(t *testing.T) {

	// Simple trace values: 0 and 10 to simulate low/high
	// It's mock data they mostly not required if you don't plan to run step left and right processing
	fullTrace := []float32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	inputs := []responseDef{
		// Group 1
		{offset: 99, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: 0},
		{offset: 100, status: stepfit.LOW, regression: 10.0, useTraceIdx: 1},
		{offset: 101, status: stepfit.LOW, regression: 10.0, useTraceIdx: 2},
		{offset: 102, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: 3},

		// Gap (103 skipped)

		// Group 2
		{offset: 104, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: 4},
		{offset: 105, status: stepfit.LOW, regression: 10.0, useTraceIdx: 5},
		{offset: 106, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: 6},
	}

	expectations := []anomalyExpectation{
		{peakOffset: 101, prevCommitNumber: 99, commitNumber: 101},
		{peakOffset: 105, prevCommitNumber: 104, commitNumber: 105},
	}

	runAnomalyTest(t, fullTrace, 2, inputs, expectations)

}

func TestAnomalyBoundsRefiner_RefinesGroup_PicksPeak(t *testing.T) {
	fullTrace := []float32{10, 11, 12, 11, 15, 20, 21, 20, 21, 21}
	inputs := []responseDef{
		{offset: 103, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: -1},
		{offset: 104, status: stepfit.LOW, regression: 2.5, useTraceIdx: 0},
		{offset: 105, status: stepfit.LOW, regression: 5.0, useTraceIdx: 1},
		{offset: 106, status: stepfit.LOW, regression: 2.5, useTraceIdx: 2},
		{offset: 107, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: -1},
	}
	runAnomalyTest(t, fullTrace, 8, inputs, []anomalyExpectation{{105, 103, 105}})
}

// Trace Data:
//
// 22 |                                     *
// 21 |                         *       *
// 20 |                               *
// 17 |                     *
// 15 |                 *
// 12 |         *
// 11 |     *       *
// 10 | *
//
//	+---------------------------------------
func TestAnomalyBoundsRefiner_RefinesGroup_PicksPeakV2(t *testing.T) {
	fullTrace := []float32{10, 11, 12, 11, 15, 17, 21, 20, 21, 21}
	inputs := []responseDef{
		{offset: 103, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: -1},
		{offset: 104, status: stepfit.LOW, regression: 2.5, useTraceIdx: 0},
		{offset: 105, status: stepfit.LOW, regression: 5.0, useTraceIdx: 1},
		{offset: 106, status: stepfit.LOW, regression: 2.5, useTraceIdx: 2},
		{offset: 107, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: -1},
	}
	runAnomalyTest(t, fullTrace, 8, inputs, []anomalyExpectation{{105, 103, 106}})
}

// Trace Data:
//
// 21 |                     *       *   *
// 20 |                                   *
// 17 |                 *
// 15 |             *
// 12 |     *
// 11 |   *   *
// 10 | *
//
//	+---------------------------------------
func TestAnomalyBoundsRefiner_RefinesGroup_PicksPeakV3(t *testing.T) {
	fullTrace := []float32{10, 11, 12, 11, 15, 17, 21, 20, 21, 21, 20}
	inputs := []responseDef{
		{offset: 103, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: -1},
		{offset: 104, status: stepfit.LOW, regression: 2.5, useTraceIdx: 0},
		{offset: 105, status: stepfit.LOW, regression: 5.0, useTraceIdx: 1},
		{offset: 106, status: stepfit.LOW, regression: 2.6, useTraceIdx: 2},
		{offset: 107, status: stepfit.LOW, regression: 2.5, useTraceIdx: 3},
		{offset: 108, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: -1},
	}
	runAnomalyTest(t, fullTrace, 8, inputs, []anomalyExpectation{{105, 103, 106}})
}

// Trace Data:
//
// 21 |                     *       *   *       *
// 20 |                                   *   *
// 17 |                 *
// 15 |             *
// 12 |     *
// 11 |   *   *
// 10 | *
//
//	+-------------------------------------------
func TestAnomalyBoundsRefiner_RefinesGroup_PicksPeakV4(t *testing.T) {
	fullTrace := []float32{10, 11, 12, 11, 15, 17, 21, 20, 21, 21, 20, 20, 21}
	inputs := []responseDef{
		{offset: 103, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: -1},
		{offset: 104, status: stepfit.LOW, regression: 2.5, useTraceIdx: 0},
		{offset: 105, status: stepfit.LOW, regression: 5.0, useTraceIdx: 1},
		{offset: 106, status: stepfit.LOW, regression: 2.6, useTraceIdx: 2},
		{offset: 107, status: stepfit.LOW, regression: 2.7, useTraceIdx: 3},
		{offset: 108, status: stepfit.LOW, regression: 2.7, useTraceIdx: 4},
		{offset: 109, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: -1},
	}
	runAnomalyTest(t, fullTrace, 8, inputs, []anomalyExpectation{{105, 103, 108}})
}

// Trace Data:
//
// 21 |                     *       *   *       *
// 20 |                                   *   *
// 17 |                 *
// 15 |             *
// 12 |     *
// 11 |   *   *
// 10 | *
//
//	+-------------------------------------------
func TestAnomalyBoundsRefiner_RefinesGroup_PicksPeakV5(t *testing.T) {
	fullTrace := []float32{10, 11, 12, 11, 15, 17, 21, 20, 21, 21, 20, 20, 21}
	inputs := []responseDef{
		{offset: 103, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: -1},
		{offset: 104, status: stepfit.LOW, regression: 2.5, useTraceIdx: 0},
		{offset: 105, status: stepfit.LOW, regression: 5.0, useTraceIdx: 1},
		{offset: 106, status: stepfit.LOW, regression: 2.6, useTraceIdx: 2},
		{offset: 107, status: stepfit.LOW, regression: 2.7, useTraceIdx: 3},
		{offset: 108, status: stepfit.LOW, regression: 2.6, useTraceIdx: 4},
		{offset: 109, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: -1},
	}
	runAnomalyTest(t, fullTrace, 8, inputs, []anomalyExpectation{{105, 103, 107}})
}

func TestAnomalyBoundsRefiner_RefinesGroup_PicksPeakV6(t *testing.T) {
	fullTrace := []float32{10, 11, 12, 11, 10, 20, 21, 20, 21, 21}
	inputs := []responseDef{
		{offset: 103, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: -1},
		{offset: 104, status: stepfit.LOW, regression: 2.5, useTraceIdx: 0},
		{offset: 105, status: stepfit.LOW, regression: 5.0, useTraceIdx: 1},
		{offset: 106, status: stepfit.LOW, regression: 2.5, useTraceIdx: 2},
		{offset: 107, status: stepfit.UNINTERESTING, regression: 0, useTraceIdx: -1},
	}
	runAnomalyTest(t, fullTrace, 8, inputs, []anomalyExpectation{{105, 104, 105}})
}

func TestAnomalyBoundsRefiner_KMeansAlgo_ReturnsError(t *testing.T) {
	r := NewAnomalyBoundsRefiner(defaultStdDevThreshold)

	cfg := &alerts.Alert{
		Algo: types.KMeansGrouping,
	}

	_, err := r.Process(context.Background(), cfg, []*regression.RegressionDetectionResponse{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kmeans")
}

func TestAnomalyBoundsRefinerValidateInput_ReturnsErrors(t *testing.T) {
	r := &AnomalyBoundsRefiner{}

	// Case 1: KMeans
	cfg := &alerts.Alert{Algo: types.KMeansGrouping}
	err := r.validateInput(cfg, []*regression.RegressionDetectionResponse{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kmeans")

	// Case 2: Nil Summary
	respNilSummary := createResponse(100, "t1", stepfit.LOW)
	respNilSummary.Summary = nil
	err = r.validateInput(&alerts.Alert{Algo: types.StepFitGrouping}, []*regression.RegressionDetectionResponse{respNilSummary})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "summary is nil")

	// Case 3: Multiple Clusters
	respMultiClusters := createResponse(100, "t1", stepfit.LOW)
	respMultiClusters.Summary.Clusters = append(respMultiClusters.Summary.Clusters, &clustering2.ClusterSummary{})
	err = r.validateInput(&alerts.Alert{Algo: types.StepFitGrouping}, []*regression.RegressionDetectionResponse{respMultiClusters})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at most 1 cluster")

	// Case 4: Multiple Keys
	respMultiKeys := createResponse(100, "t1", stepfit.LOW)
	respMultiKeys.Summary.Clusters[0].Keys = []string{"k1", "k2"}
	err = r.validateInput(&alerts.Alert{Algo: types.StepFitGrouping}, []*regression.RegressionDetectionResponse{respMultiKeys})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expects exactly 1 key")

	// Case 5: Nil StepPoint
	respNilStepPoint := createResponse(100, "t1", stepfit.LOW)
	respNilStepPoint.Summary.Clusters[0].StepPoint = nil
	err = r.validateInput(&alerts.Alert{Algo: types.StepFitGrouping}, []*regression.RegressionDetectionResponse{respNilStepPoint})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "StepPoint to be not nil")

	// Case 6: Nil StepFit
	respNilStepFit := createResponse(100, "t1", stepfit.LOW)
	respNilStepFit.Summary.Clusters[0].StepFit = nil
	err = r.validateInput(&alerts.Alert{Algo: types.StepFitGrouping}, []*regression.RegressionDetectionResponse{respNilStepFit})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "StepFit to be not nil")

}

func generateTracesFromStream(fullData []float32, windowSize int) [][]float32 {
	var traces [][]float32
	for i := 0; i <= len(fullData)-windowSize; i++ {
		trace := make([]float32, windowSize)
		copy(trace, fullData[i:i+windowSize])
		traces = append(traces, trace)
	}
	return traces
}

type responseDef struct {
	offset      int
	status      stepfit.StepFitStatus
	regression  float32
	useTraceIdx int // -1 for empty/uninteresting, >=0 for index in generated traces
}

type anomalyExpectation struct {
	peakOffset       int
	prevCommitNumber int
	commitNumber     int
}

func runAnomalyTest(t *testing.T, fullTrace []float32, windowSize int, inputDefs []responseDef, expectations []anomalyExpectation) {
	r := NewAnomalyBoundsRefiner(defaultStdDevThreshold)
	traces := generateTracesFromStream(fullTrace, windowSize)
	notImportantTrace := types.Trace{0}

	var inputs []*regression.RegressionDetectionResponse
	for _, def := range inputDefs {
		var trace types.Trace
		if def.useTraceIdx >= 0 {
			trace = traces[def.useTraceIdx]
		} else {
			trace = notImportantTrace
		}
		inputs = append(inputs, createResponseV2(trace, "t1", def.status, def.offset, def.regression))
	}

	alert := &alerts.Alert{
		Algo:        types.StepFitGrouping,
		Step:        types.CohenStep,
		Interesting: 2.5,
		Radius:      4,
	}

	res, err := r.Process(context.Background(), alert, inputs)
	assert.NoError(t, err)
	assert.Len(t, res, len(expectations))

	for i, expectation := range expectations {
		// Verify Peak Selection
		assert.Equal(t, types.CommitNumber(expectation.peakOffset), res[i].Summary.Clusters[0].StepPoint.Offset, "Result %d: Should pick peak at %d", i, expectation.peakOffset)

		// Determine expected trace from inputs
		// Find the input definition corresponding to the expected peak offset
		var expectedTrace []float32
		for _, def := range inputDefs {
			if def.offset == expectation.peakOffset {
				if def.useTraceIdx >= 0 {
					expectedTrace = traces[def.useTraceIdx]
				} else {
					expectedTrace = notImportantTrace
				}
				break
			}
		}
		assert.NotEmpty(t, expectedTrace, "Result %d: Could not find expected trace for offset %d", i, expectation.peakOffset)
		assert.Equal(t, expectedTrace, res[i].Summary.Clusters[0].Centroid, "Result %d: Trace mismatch", i)
		assert.Equal(t, types.CommitNumber(expectation.prevCommitNumber), res[i].PrevCommitNumber, "Result %d: PrevCommitNumber mismatch", i)
		assert.Equal(t, types.CommitNumber(expectation.commitNumber), res[i].CommitNumber, "Result %d: CommitNumber mismatch", i)
		assert.Equal(t, types.CommitNumber(expectation.peakOffset), res[i].DisplayCommitNumber, "Result %d: DisplayCommitNumber mismatch", i)
	}
}

func TestAnomalyBoundsRefinerExpandRange(t *testing.T) {
	r := NewAnomalyBoundsRefiner(defaultStdDevThreshold).(*AnomalyBoundsRefiner)
	cfg := &alerts.Alert{Algo: types.StepFitGrouping}

	createResp := func(traceValues []float32) ([]*regression.RegressionDetectionResponse, error) {
		group := make([]*regression.RegressionDetectionResponse, len(traceValues))
		for i, v := range traceValues {
			// Each point in time has a regression response.
			// The response contains a trace. For simplicity, we can say trace is [v].
			// And TurningPoint is 0.
			resp := createResponse(100+i, "key", stepfit.LOW)
			resp.Summary.Clusters[0].Centroid = types.Trace{v}
			resp.Summary.Clusters[0].StepFit.TurningPoint = 0
			group[i] = resp
		}
		return group, nil
	}

	t.Run("ExpandRangeToLeft", func(t *testing.T) {
		// Timeline: [Start, Start, Start, Anomaly, Anomaly]
		// Values:   [10,    10,    0,     0,       0]
		// Indices:  0      1      2      3        4
		// Target:   We start at index 3 (value 0). Baseline is {10, 10, 10}.
		// It should expand left to include 2 (value 0).
		// Should stop at 1 (value 10).

		vals := []float32{10, 10, 0, 0, 0}
		group, _ := createResp(vals)

		baseline := []float32{10, 10, 10}

		// Start expanding from index 3 to the left
		startIndex := 3
		foundIndex := r.expandRangeToLeft(baseline, startIndex, group, cfg)

		assert.Equal(t, 2, foundIndex, "Should expand until index 2")
	})

	t.Run("ExpandRangeToRight", func(t *testing.T) {
		// Timeline: [Anomaly, Anomaly, Normal, Normal]
		// Values:   [0,       0,       10,     10]
		// Indices:  0        1        2       3
		// Target:   We start at index 1 (value 0). Baseline is {10, 10, 10}.
		// It should expand right.
		// Index 2, it is a change point (val 10).
		// Returns 1.

		vals := []float32{0, 0, 10, 10}
		group, _ := createResp(vals)

		baseline := []float32{10, 10, 10}

		// Start expanding from index 1 to the right
		startIndex := 1
		foundIndex := r.expandRangeToRight(baseline, startIndex, group, cfg)

		assert.Equal(t, 2, foundIndex, "Should stop at index 2")

		// Case 2: Expand more
		// Values: [0, 0, 0, 10]
		// Indices: 0, 1, 2, 3
		// Start at 1.
		// i=1 (0). Anomaly.
		// i=2 (0). Anomaly.
		// i=3 (10). Change Point.
		// Returns 3.
		vals2 := []float32{0, 0, 0, 10}
		group2, _ := createResp(vals2)

		foundIndex2 := r.expandRangeToRight(baseline, 1, group2, cfg)
		assert.Equal(t, 3, foundIndex2, "Should expand to index 3")
	})
}

func TestAnomalyBoundsRefiner_MultipleTraces_GroupsSeparately(t *testing.T) {
	r := NewAnomalyBoundsRefiner(defaultStdDevThreshold)

	alert := &alerts.Alert{
		Algo:        types.StepFitGrouping,
		Step:        types.CohenStep,
		Interesting: 2.5,
		Radius:      4,
	}

	// Two traces with isolated peaks
	// We want offset to be 100
	traceData := []float32{10, 10, 10, 10, 20, 20, 20, 20}

	inputs := []*regression.RegressionDetectionResponse{
		createResponseV2(traceData, "trace-A", stepfit.UNINTERESTING, 99, 0),
		createResponseV2(traceData, "trace-B", stepfit.UNINTERESTING, 99, 0),
		createResponseV2(traceData, "trace-A", stepfit.LOW, 100, 10),
		createResponseV2(traceData, "trace-B", stepfit.LOW, 100, 10),
		createResponseV2(traceData, "trace-A", stepfit.UNINTERESTING, 101, 0),
		createResponseV2(traceData, "trace-B", stepfit.UNINTERESTING, 101, 0),
	}

	res, err := r.Process(context.Background(), alert, inputs)
	assert.NoError(t, err)
	// Without grouping, the refiner would conflate trace-A and trace-B offsets and only emit 1 confirmed regression peak.
	// With grouping by TraceName, it treats them entirely independently and finds 2 peaks.
	assert.Len(t, res, 2, "Expected exactly 2 regressions, one for each trace")
}
