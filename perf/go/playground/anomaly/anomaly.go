package anomaly

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"

	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/types"
)

const (
	// PlaygroundTraceName is the trace name used for the playground.
	PlaygroundTraceName = ",name=playground,"
)

// DetectRequest is the structure of the JSON request to the /_/playground/anomaly/v1/detect endpoint.
type DetectRequest struct {
	Trace          []float32           `json:"trace"`
	Radius         int                 `json:"radius"`
	Threshold      float32             `json:"threshold"`
	Algorithm      types.StepDetection `json:"algorithm"`
	GroupAnomalies bool                `json:"group_anomalies"`
}

// Anomaly matches the frontend interface.
type Anomaly struct {
	ID                  string   `json:"id"`
	TestPath            string   `json:"test_path"`
	BugID               int      `json:"bug_id"`
	StartRevision       int      `json:"start_revision"`
	EndRevision         int      `json:"end_revision"`
	IsImprovement       bool     `json:"is_improvement"`
	Recovered           bool     `json:"recovered"`
	State               string   `json:"state"`
	Statistic           string   `json:"statistic"`
	Units               string   `json:"units"`
	DegreesOfFreedom    float64  `json:"degrees_of_freedom"`
	MedianBeforeAnomaly float32  `json:"median_before_anomaly"`
	MedianAfterAnomaly  float32  `json:"median_after_anomaly"`
	PValue              float64  `json:"p_value"`
	SegmentSizeAfter    int      `json:"segment_size_after"`
	SegmentSizeBefore   int      `json:"segment_size_before"`
	StdDevBeforeAnomaly float32  `json:"std_dev_before_anomaly"`
	TStatistic          float64  `json:"t_statistic"`
	SubscriptionName    string   `json:"subscription_name"`
	BugComponent        string   `json:"bug_component"`
	BugLabels           []string `json:"bug_labels"`
	BugCCEmails         []string `json:"bug_cc_emails"`
	BisectIDs           []string `json:"bisect_ids"`
}

// DetectResponse is the structure of the JSON response from the /_/playground/anomaly/v1/detect endpoint.
type DetectResponse struct {
	Anomalies []Anomaly `json:"anomalies"`
}

// DetectedAnomaly holds the index and the regression score of a detected anomaly.
type DetectedAnomaly struct {
	Index      int
	Regression float32
}

// slidingWindowStepFit runs stepfit over a sliding window of the trace.
func slidingWindowStepFit(ctx context.Context, trace []float32, radius int, interesting float32, stepDetection types.StepDetection) []DetectedAnomaly {
	anomalies := []DetectedAnomaly{}
	// Dummy headers needed for StepFit to avoid panic.
	// We need headers for the max window size (2*radius + 1).
	// But since we create a new dataframe for each window, we just need enough for that window.
	// The window size is 2*radius + 1.
	windowSize := 2*radius + 1
	dummyHeader := make([]*dataframe.ColumnHeader, windowSize)
	for i := 0; i < windowSize; i++ {
		dummyHeader[i] = &dataframe.ColumnHeader{
			Offset: types.CommitNumber(i), // Relative offset within window
		}
	}

	for i := radius; i < len(trace)-radius; i++ {
		window := trace[i-radius : i+radius+1]

		df := dataframe.NewEmpty()
		df.TraceSet[PlaygroundTraceName] = window
		df.Header = dummyHeader

		// k=0 since this is not "k-mean", but "individual" computation
		summaries, err := regression.StepFit(ctx, df, 0, config.MinStdDev, nil, interesting, stepDetection)
		if err != nil {
			continue
		}

		if len(summaries.Clusters) > 0 {
			// Found an anomaly (or more? StepFit returns High/Low clusters).
			// We check if any cluster has our key.
			for _, cluster := range summaries.Clusters {
				if cluster.StepFit.Status == "High" || cluster.StepFit.Status == "Low" {
					anomalies = append(anomalies, DetectedAnomaly{
						Index:      i,
						Regression: cluster.StepFit.Regression,
					})
					// Break after finding one, as we only process one trace here.
					break
				}
			}
		}
	}
	return anomalies
}

// Handler handles the /_/playground/anomaly/v1/detect endpoint.
func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req DetectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	anomalies := []Anomaly{}
	if len(req.Trace) > 2*req.Radius {
		candidates := slidingWindowStepFit(r.Context(), req.Trace, req.Radius, req.Threshold, req.Algorithm)

		// Helper to add an anomaly to the list
		addAnomaly := func(cand DetectedAnomaly) {
			// Calculate stats
			startIdx := cand.Index - req.Radius
			if startIdx < 0 {
				startIdx = 0
			}
			endIdx := cand.Index + req.Radius
			if endIdx > len(req.Trace) {
				endIdx = len(req.Trace)
			}

			before := req.Trace[startIdx:cand.Index]
			after := req.Trace[cand.Index:endIdx]

			medianBefore := median(before)
			medianAfter := median(after)

			anomalies = append(anomalies, Anomaly{
				ID:                  strconv.Itoa(len(anomalies)),
				BugID:               -1,
				StartRevision:       cand.Index,
				EndRevision:         cand.Index,
				IsImprovement:       cand.Regression > 0,
				MedianBeforeAnomaly: medianBefore,
				MedianAfterAnomaly:  medianAfter,
				State:               "untriaged",
			})
		}

		if len(candidates) > 0 {
			if req.GroupAnomalies {
				// Non-maximum suppression:
				// Group consecutive candidates and pick the best one from each group.

				// Initialize the current group with the first candidate.
				currentGroupBest := candidates[0]
				lastIndex := candidates[0].Index

				for i := 1; i < len(candidates); i++ {
					cand := candidates[i]
					// If this candidate is consecutive to the last one (or within a small window,
					// here strictly consecutive as indices are sorted), it belongs to the same group.
					if cand.Index == lastIndex+1 {
						if abs(cand.Regression) > abs(currentGroupBest.Regression) {
							currentGroupBest = cand
						}
					} else {
						// Gap detected, close the current group and start a new one.
						addAnomaly(currentGroupBest)
						currentGroupBest = cand
					}
					lastIndex = cand.Index
				}
				// Append the last group's best candidate.
				addAnomaly(currentGroupBest)
			} else {
				// No grouping, add all candidates.
				for _, cand := range candidates {
					addAnomaly(cand)
				}
			}
		}
	}

	resp := DetectResponse{
		Anomalies: anomalies,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

func median(data []float32) float32 {
	vals := vec32.RemoveMissingDataSentinel(data)
	if len(vals) == 0 {
		return 0
	}
	slices.Sort(vals)
	n := len(vals)
	if n%2 == 1 {
		return vals[n/2]
	}
	return (vals[n/2-1] + vals[n/2]) / 2
}
