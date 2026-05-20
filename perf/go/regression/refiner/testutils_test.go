package refiner

import (
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

// createResponse creates a simple, skeletal RegressionDetectionResponse for testing.
// It is intended for testing grouping logic where only the anomaly offset, trace key,
// and step status matter. It hardcodes a single point of trace data and a single
// column header at the specified offset.
// Use this for tests where full trace datasets and temporal ranges are not required.
func createResponse(offset int, key string, status stepfit.StepFitStatus) *regression.RegressionDetectionResponse {
	t := types.Trace{1.0}

	var clusters []*clustering2.ClusterSummary
	if status != stepfit.UNINTERESTING {
		clusters = []*clustering2.ClusterSummary{
			{
				StepPoint: &dataframe.ColumnHeader{
					Offset: types.CommitNumber(offset),
				},
				Keys: []string{key},
				StepFit: &stepfit.StepFit{
					Regression:   0.0, // Default
					Status:       status,
					TurningPoint: len(t) / 2,
				},
				Centroid: t,
			},
		}
	}

	return &regression.RegressionDetectionResponse{
		Frame: &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				Header: []*dataframe.ColumnHeader{
					{Offset: types.CommitNumber(offset)},
				},
			},
		},
		Summary: &clustering2.ClusterSummaries{
			Clusters: clusters,
		},
		TraceName: key,
	}
}

// createResponseV2 creates a comprehensive RegressionDetectionResponse with realistic
// trace data and a fully populated DataFrame header timeline.
// It uses `makeHeader` to create a range of commit headers centered around `tpOffset`.
// Use this for tests (like `runAnomalyTest`) that need to simulate actual full data
// streams, verify peak selection algorithms, or test range expansion over time.
func createResponseV2(data types.Trace, key string, status stepfit.StepFitStatus, tpOffset int, reg float32) *regression.RegressionDetectionResponse {
	var clusters []*clustering2.ClusterSummary
	if status != stepfit.UNINTERESTING {
		clusters = []*clustering2.ClusterSummary{
			{
				StepPoint: &dataframe.ColumnHeader{
					Offset: types.CommitNumber(tpOffset),
				},
				Keys: []string{key},
				StepFit: &stepfit.StepFit{
					Regression:   reg, // Default
					Status:       status,
					TurningPoint: len(data) / 2,
					RuleEvaluations: []stepfit.AnomalyResult{
						{
							AlgoName:  string(types.AbsoluteStep),
							Value:     reg,
							IsAnomaly: status != stepfit.UNINTERESTING,
						},
					},
				},
				Centroid: []float32(data),
			},
		}
	}

	return &regression.RegressionDetectionResponse{
		Frame: &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				Header: makeHeader(tpOffset-len(data)/2, len(data)),
				// Minimal trace data to avoid nil panics if logic checks it
				TraceSet: types.TraceSet{
					key: data,
				},
			},
		},
		Summary: &clustering2.ClusterSummaries{
			Clusters: clusters,
		},
		TraceName: key,
	}
}

func makeHeader(start int, count int) []*dataframe.ColumnHeader {
	h := make([]*dataframe.ColumnHeader, count)
	for i := 0; i < count; i++ {
		h[i] = &dataframe.ColumnHeader{Offset: types.CommitNumber(start + i)}
	}
	return h
}
