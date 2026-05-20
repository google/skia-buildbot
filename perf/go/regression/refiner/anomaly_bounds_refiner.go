package refiner

import (
	"context"
	"fmt"

	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
)

// AnomalyBoundsRefiner implements regression.RegressionRefiner.
// It groups adjacent anomalies into a single "Wide Range Anomaly".
// It uses peaks, regression areas, and anomaly checkers to identify the best
// representative peak and determine the true start and end commits of the anomaly.
// See go/skia_perf_anomaly_data_point_localization
type AnomalyBoundsRefiner struct {
	stdDevThreshold float32
}

func NewAnomalyBoundsRefiner(stdDevThreshold float32) regression.RegressionRefiner {
	return &AnomalyBoundsRefiner{
		stdDevThreshold: stdDevThreshold,
	}
}

// Process implements the regression.RegressionRefiner interface.
func (r *AnomalyBoundsRefiner) Process(ctx context.Context, cfg *alerts.Alert, responses []*regression.RegressionDetectionResponse) ([]*regression.ConfirmedRegression, error) {
	if err := r.validateInput(cfg, responses); err != nil {
		return nil, err
	}

	var confirmed []*regression.ConfirmedRegression

	groups := groupResponsesByTraceName(responses)

	for _, group := range groups {
		areas := findRegressionAreas(group, cfg)
		for _, area := range areas {
			confirmedReg := r.mergeArea(cfg, area)
			confirmed = append(confirmed, confirmedReg)
		}
	}

	return confirmed, nil
}

func groupResponsesByTraceName(responses []*regression.RegressionDetectionResponse) map[string][]*regression.RegressionDetectionResponse {
	// Create a map to instantly group responses by TraceName
	groupedMap := make(map[string][]*regression.RegressionDetectionResponse)
	for _, resp := range responses {
		groupedMap[resp.TraceName] = append(groupedMap[resp.TraceName], resp)
	}

	return groupedMap
}

func (r *AnomalyBoundsRefiner) validateInput(cfg *alerts.Alert, responses []*regression.RegressionDetectionResponse) error {
	if cfg.Algo != types.StepFitGrouping {
		return fmt.Errorf("AnomalyBoundsRefiner expects StepFitGrouping clustering, got %s instead", cfg.Algo)
	}

	switch cfg.Step {
	case types.AbsoluteStep, types.Const, types.PercentStep, types.CohenStep, types.MannWhitneyU, types.OriginalStep:
		// Valid
	default:
		// Please don't forget about anomaly_checker.go when adding new step detection algorithms.
		return fmt.Errorf("AnomalyBoundsRefiner expects valid step detection algorithm, got unsupported algorithm %q, please adopt the code to support it", cfg.Step)
	}

	for _, resp := range responses {
		if resp.Summary == nil {
			return fmt.Errorf("regression detection response summary is nil")
		}
		if resp.Frame == nil {
			return fmt.Errorf("regression detection response frame is nil")
		}
		if resp.Frame.DataFrame == nil {
			return fmt.Errorf("regression detection response dataframe is nil")
		}
		if len(resp.Summary.Clusters) > 1 {
			return fmt.Errorf("StepFit expects at most 1 cluster per response, got %d", len(resp.Summary.Clusters))
		}
		if len(resp.Summary.Clusters) == 1 && len(resp.Summary.Clusters[0].Keys) != 1 {
			return fmt.Errorf("AnomalyBoundsRefiner expects exactly 1 key per cluster, got %d", len(resp.Summary.Clusters[0].Keys))
		}
		if len(resp.Summary.Clusters) == 1 && resp.Summary.Clusters[0].StepPoint == nil {
			return fmt.Errorf("AnomalyBoundsRefiner expects StepPoint to be not nil")
		}
		if len(resp.Summary.Clusters) == 1 && resp.Summary.Clusters[0].StepFit == nil {
			return fmt.Errorf("AnomalyBoundsRefiner expects StepFit to be not nil")
		}
	}
	if err := r.validateKeys(responses); err != nil {
		return err
	}
	return nil
}

// validateKeys ensures that all cluster keys match the response's TraceName.
func (r *AnomalyBoundsRefiner) validateKeys(responses []*regression.RegressionDetectionResponse) error {
	for _, resp := range responses {
		for _, cluster := range resp.Summary.Clusters {
			for _, key := range cluster.Keys {
				if key != resp.TraceName {
					return fmt.Errorf("Inconsistency: key %q does not match trace name %q", key, resp.TraceName)
				}
			}
		}
	}
	return nil
}

// mergeArea takes a contiguous area of anomalies, identifies the most representative peaks within it,
// and delegates to refineGroup to determine the true boundaries of the anomaly. It returns a single
// ConfirmedRegression that encompasses the refined anomalous range.
func (r *AnomalyBoundsRefiner) mergeArea(cfg *alerts.Alert, area []*regression.RegressionDetectionResponse) *regression.ConfirmedRegression {
	peakIdx := findPeaks(area)
	return r.refineGroup(area, peakIdx, cfg)
}

// refineGroup takes a group of anomalies and their peak indexes, and refines the actual
// start and end commits of the anomaly. It does this by expanding the range leftwards and rightwards
// from the leftmost and rightmost peaks, comparing against the pre-anomaly and post-anomaly baselines, respectively, until
// the data is no longer considered anomalous. It returns a ConfirmedRegression representing these refined bounds.
func (r *AnomalyBoundsRefiner) refineGroup(group []*regression.RegressionDetectionResponse, peakIdx PeakIndexes, cfg *alerts.Alert) *regression.ConfirmedRegression {
	peak := group[peakIdx.MaxIndex]
	groupLen := len(group)
	regressionStartIndex := peakIdx.LeftIndex
	regressionEndIndex := peakIdx.RightIndex

	if peakIdx.LeftIndex > 0 {
		// For the left baseline we use the left half of the leftmost regression in the regression area.
		// As this data actually shows the data that we have before the anomaly happened.
		tpIndex := group[0].Summary.Clusters[0].StepFit.TurningPoint
		leftBaseline := group[0].Summary.Clusters[0].Centroid[:tpIndex]
		regressionStartIndex = r.expandRangeToLeft(leftBaseline, peakIdx.LeftIndex-1, group, cfg)
	}

	if peakIdx.RightIndex < groupLen-1 {
		// For the right baseline we use the right half of the rightmost regression in the regression area.
		// As this data actually shows the data that we have after the anomaly happened.
		tpIndex := group[groupLen-1].Summary.Clusters[0].StepFit.TurningPoint
		rightBaseline := group[groupLen-1].Summary.Clusters[0].Centroid[tpIndex:]
		regressionEndIndex = r.expandRangeToRight(rightBaseline, peakIdx.RightIndex+1, group, cfg)
	}

	leftTp := group[regressionStartIndex].Summary.Clusters[0].StepFit.TurningPoint

	cr := &regression.ConfirmedRegression{
		Summary:             peak.Summary,
		RightMostSummary:    group[regressionEndIndex].Summary,
		Frame:               peak.Frame,
		RightMostFrame:      group[regressionEndIndex].Frame,
		Message:             peak.Message,
		PrevCommitNumber:    group[regressionStartIndex].Frame.DataFrame.Header[leftTp-1].Offset,
		CommitNumber:        group[regressionEndIndex].Summary.Clusters[0].StepPoint.Offset,
		DisplayCommitNumber: peak.Summary.Clusters[0].StepPoint.Offset,
	}

	return cr
}

// expandRangeToLeft iterates backwards from startIndex (leftmost local peak) to find the continuous sequence of anomalous points.
// It compares each point's regression candidate against the provided left baseline to determine
// if it is still part of the anomaly. The expansion stops when a non-anomalous point is found.
// Returns the index of the leftmost point in the expanded range.
func (r *AnomalyBoundsRefiner) expandRangeToLeft(baseline []float32, startIndex int, group []*regression.RegressionDetectionResponse, cfg *alerts.Alert) int {
	// Default to startIndex + 1. If the point at startIndex is not an anomaly,
	// the loop breaks immediately and we return startIndex + 1, effectively
	// excluding startIndex and keeping the regression start to its right.
	regressionStart := startIndex + 1

	for i := startIndex; i >= 0; i-- {
		tpIndex := group[i].Summary.Clusters[0].StepFit.TurningPoint
		regressionCandidate := group[i].Summary.Clusters[0].Centroid[tpIndex]

		rule := cfg.DetectionRule
		if rule == nil {
			rule = stepfit.NewSimpleRule(cfg.Step, cfg.Interesting)
		}

		isInteresting := evaluateRuleForRefinement(regressionCandidate, baseline, rule, r.stdDevThreshold)

		if isInteresting {
			regressionStart = i
		} else {
			break
		}
	}
	return regressionStart
}

// expandRangeToRight iterates forwards from startIndex (rightmost local peak) to find the continuous sequence of anomalous points.
// It compares each point's regression candidate against the provided right baseline to determine
// if it is still part of the anomaly. The expansion stops when a non-anomalous point is found.
// Returns the index of the rightmost point in the expanded range.
func (r *AnomalyBoundsRefiner) expandRangeToRight(baseline []float32, startIndex int, group []*regression.RegressionDetectionResponse, cfg *alerts.Alert) int {
	// Default to startIndex - 1. If the point at startIndex is not an anomaly,
	// the loop breaks immediately and we return startIndex - 1, effectively
	// excluding startIndex and keeping the regression start to its left.
	regressionStart := startIndex - 1

	for i := startIndex; i < len(group); i++ {
		tpIndex := group[i-1].Summary.Clusters[0].StepFit.TurningPoint
		// Note the difference from expandRangeToLeft: we use i-1 instead of i.
		// In the right range, we want to find the first point where the statistics
		// return to baseline (the clean area after the anomaly).
		// We make use of i-1 to evaluate the preceding point. When group[i-1] stops
		// being an anomaly, it means we have found the change point, and the current
		// point represents where the regression area ends.
		regressionCandidate := group[i-1].Summary.Clusters[0].Centroid[tpIndex]

		rule := cfg.DetectionRule
		if rule == nil {
			rule = stepfit.NewSimpleRule(cfg.Step, cfg.Interesting)
		}

		isInteresting := evaluateRuleForRefinement(regressionCandidate, baseline, rule, r.stdDevThreshold)

		if isInteresting {
			regressionStart = i
		} else {
			break
		}
	}
	return regressionStart
}
