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
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

func TestFilterRegressionsByStepFitAndDirection_EmptyResponses_ReturnsEmpty(t *testing.T) {
	cfg := &alerts.Alert{}
	var responses []*regression.RegressionDetectionResponse

	result := FilterRegressionsByStepFitAndDirection(cfg, responses)

	assert.Empty(t, result)
}

func TestFilterRegressionsByStepFitAndDirection_NilResponseOrDataFrame_Ignored(t *testing.T) {
	cfg := &alerts.Alert{}
	responses := []*regression.RegressionDetectionResponse{
		nil,
		{Frame: nil},
		{Frame: &frame.FrameResponse{DataFrame: nil}},
		{Frame: &frame.FrameResponse{DataFrame: &dataframe.DataFrame{Header: []*dataframe.ColumnHeader{}}}},
	}

	result := FilterRegressionsByStepFitAndDirection(cfg, responses)

	assert.Empty(t, result)
}

func TestFilterRegressionsByStepFitAndDirection_MatchesConditions_ReturnsFiltered(t *testing.T) {
	cfg := &alerts.Alert{
		MinimumNum:        2,
		DirectionAsString: alerts.BOTH,
	}

	commitNumber := types.CommitNumber(100)
	responses := []*regression.RegressionDetectionResponse{
		{
			Frame: &frame.FrameResponse{
				DataFrame: &dataframe.DataFrame{
					Header: []*dataframe.ColumnHeader{
						{Offset: commitNumber - 1},
						{Offset: commitNumber}, // Midpoint match
						{Offset: commitNumber + 1},
					},
				},
			},
			Summary: &clustering2.ClusterSummaries{
				Clusters: []*clustering2.ClusterSummary{
					{
						StepPoint: &dataframe.ColumnHeader{Offset: commitNumber},
						StepFit:   &stepfit.StepFit{Status: stepfit.LOW},
						Keys:      []string{"key1", "key2"}, // Meets minimum (2)
					},
					{
						StepPoint: &dataframe.ColumnHeader{Offset: commitNumber},
						StepFit:   &stepfit.StepFit{Status: stepfit.HIGH},
						Keys:      []string{"key3"}, // Below minimum (1) -> filtered out
					},
					{
						StepPoint: &dataframe.ColumnHeader{Offset: commitNumber - 1}, // Wrong offset -> filtered out
						StepFit:   &stepfit.StepFit{Status: stepfit.LOW},
						Keys:      []string{"key4", "key5"},
					},
				},
			},
			Message: "test message",
		},
	}

	result := FilterRegressionsByStepFitAndDirection(cfg, responses)

	assert.Len(t, result, 1)
	assert.Equal(t, "test message", result[0].Message)
	assert.Len(t, result[0].Summary.Clusters, 1)
	assert.Equal(t, stepfit.LOW, result[0].Summary.Clusters[0].StepFit.Status)
}

func TestFilterRegressionsByStepFitAndDirection_DirectionDown_ReturnsHigh(t *testing.T) {
	// Only accepts DOWN regressions (LOW StepFit)
	cfg := &alerts.Alert{
		MinimumNum:        1,
		DirectionAsString: alerts.DOWN,
	}

	commitNumber := types.CommitNumber(100)
	responses := []*regression.RegressionDetectionResponse{
		{
			Frame: &frame.FrameResponse{
				DataFrame: &dataframe.DataFrame{
					Header: []*dataframe.ColumnHeader{
						{Offset: commitNumber - 1},
						{Offset: commitNumber}, // Midpoint match
						{Offset: commitNumber + 1},
					},
				},
			},
			Summary: &clustering2.ClusterSummaries{
				Clusters: []*clustering2.ClusterSummary{
					{
						StepPoint: &dataframe.ColumnHeader{Offset: commitNumber},
						StepFit:   &stepfit.StepFit{Status: stepfit.HIGH},
						Keys:      []string{"key1"},
					},
				},
			},
		},
	}

	result := FilterRegressionsByStepFitAndDirection(cfg, responses)

	assert.Empty(t, result)
}

func TestProcess_ValidResponses_ReturnsConfirmedRegressions(t *testing.T) {
	refiner := NewDefaultRegressionRefiner()

	cfg := &alerts.Alert{
		MinimumNum:        1,
		DirectionAsString: alerts.UP,
	}

	commitNumber := types.CommitNumber(200)
	responses := []*regression.RegressionDetectionResponse{
		{
			Frame: &frame.FrameResponse{
				DataFrame: &dataframe.DataFrame{
					Header: []*dataframe.ColumnHeader{
						{Offset: commitNumber - 1},
						{Offset: commitNumber}, // Midpoint match
						{Offset: commitNumber + 1},
					},
				},
			},
			Summary: &clustering2.ClusterSummaries{
				Clusters: []*clustering2.ClusterSummary{
					{
						StepPoint: &dataframe.ColumnHeader{Offset: commitNumber},
						StepFit:   &stepfit.StepFit{Status: stepfit.HIGH},
						Keys:      []string{"key_a"},
					},
				},
			},
			Message: "regression detected",
		},
	}

	ctx := context.Background()
	result, err := refiner.Process(ctx, cfg, responses)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "regression detected", result[0].Message)
	assert.Len(t, result[0].Summary.Clusters, 1)
	assert.Equal(t, stepfit.HIGH, result[0].Summary.Clusters[0].StepFit.Status)
}
