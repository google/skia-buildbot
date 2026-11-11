// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dfiter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

// twoTraceDataFrame returns three commits of data for two traces.
func twoTraceDataFrame() *dataframe.DataFrame {
	df := dataframe.NewEmpty()
	df.Header = []*dataframe.ColumnHeader{{Offset: 1}, {Offset: 2}, {Offset: 3}}
	df.TraceSet = types.TraceSet{
		",config=8888,name=foo,": types.Trace{1, 2, 3},
		",config=565,name=bar,":  types.Trace{4, 5, 6},
	}
	return df
}

// firstWindowTraceCount returns how many traces the first window holds.
func firstWindowTraceCount(t *testing.T, it DataFrameIterator) int {
	require.True(t, it.Next())
	df, err := it.Value(context.Background())
	require.NoError(t, err)
	return len(df.TraceSet)
}

func TestNewSlicer_StepFitByDefault_OneTracePerWindow(t *testing.T) {
	alert := &alerts.Alert{Algo: types.StepFitGrouping}

	it := newSlicer(twoTraceDataFrame(), 3, alert, config.AnomalyConfig{}, "")

	assert.Equal(t, 1, firstWindowTraceCount(t, it))
}

func TestNewSlicer_StepFitWithGroupStepFitTraces_AllTracesInOneWindow(t *testing.T) {
	alert := &alerts.Alert{Algo: types.StepFitGrouping}

	it := newSlicer(twoTraceDataFrame(), 3, alert, config.AnomalyConfig{GroupStepFitTraces: true}, "")

	assert.Equal(t, 2, firstWindowTraceCount(t, it))
}

func TestNewSlicer_GroupStepFitTracesWithAnomalyBoundsRefiner_OneTracePerWindow(t *testing.T) {
	alert := &alerts.Alert{Algo: types.StepFitGrouping}

	it := newSlicer(twoTraceDataFrame(), 3, alert, config.AnomalyConfig{GroupStepFitTraces: true}, "anomaly_bounds")

	assert.Equal(t, 1, firstWindowTraceCount(t, it))
}

func TestNewSlicer_GroupStepFitTracesWithImprovedRefiner_OneTracePerWindow(t *testing.T) {
	alert := &alerts.Alert{Algo: types.StepFitGrouping}

	it := newSlicer(twoTraceDataFrame(), 3, alert, config.AnomalyConfig{GroupStepFitTraces: true}, "improved")

	assert.Equal(t, 1, firstWindowTraceCount(t, it))
}

func TestNewSlicer_GroupStepFitTracesWithRecursiveLoader_OneTracePerWindow(t *testing.T) {
	alert := &alerts.Alert{Algo: types.StepFitGrouping}

	it := newSlicer(twoTraceDataFrame(), 3, alert, config.AnomalyConfig{GroupStepFitTraces: true, UseRecursiveLoader: true}, "")

	assert.Equal(t, 1, firstWindowTraceCount(t, it))
}

func TestNewSlicer_KMeans_AllTracesInOneWindow(t *testing.T) {
	alert := &alerts.Alert{Algo: types.KMeansGrouping}

	it := newSlicer(twoTraceDataFrame(), 3, alert, config.AnomalyConfig{}, "anomaly_bounds")

	assert.Equal(t, 2, firstWindowTraceCount(t, it))
}
