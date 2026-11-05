// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package refiner

import (
	"context"

	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/stepfit"
)

// DefaultRegressionRefiner is an implementation of regression.RegressionRefiner that filters regressions based on the alert config.
type DefaultRegressionRefiner struct{}

// Process implements the regression.RegressionRefiner interface.
func (p *DefaultRegressionRefiner) Process(ctx context.Context, cfg *alerts.Alert, responses []*regression.RegressionDetectionResponse) ([]*regression.ConfirmedRegression, error) {
	confirmedRegressions := FilterRegressionsByStepFitAndDirection(cfg, responses)
	var ret []*regression.ConfirmedRegression
	for _, resp := range confirmedRegressions {
		headerLength := len(resp.Frame.DataFrame.Header)
		midPoint := headerLength / 2
		commitNumber := resp.Frame.DataFrame.Header[midPoint].Offset

		ret = append(ret, &regression.ConfirmedRegression{
			Summary:             resp.Summary,
			RightMostSummary:    resp.Summary,
			Frame:               resp.Frame,
			RightMostFrame:      resp.Frame,
			Message:             resp.Message,
			PrevCommitNumber:    resp.Frame.DataFrame.Header[midPoint-1].Offset,
			CommitNumber:        commitNumber,
			DisplayCommitNumber: commitNumber,
		})
	}
	return ret, nil
}

// FilterRegressionsByThreshold is a reusable utility function that filters a slice of responses
// StepFit High or Low it's an indicator that we have a regression
func FilterRegressionsByStepFitAndDirection(cfg *alerts.Alert, responses []*regression.RegressionDetectionResponse) []*regression.RegressionDetectionResponse {
	var ret []*regression.RegressionDetectionResponse
	for _, resp := range responses {
		if resp == nil || resp.Frame == nil || resp.Frame.DataFrame == nil || len(resp.Frame.DataFrame.Header) == 0 {
			continue
		}

		// Create a new summary that only contains clusters that meet the threshold.
		filteredSummary := &clustering2.ClusterSummaries{
			Clusters: []*clustering2.ClusterSummary{},
		}

		headerLength := len(resp.Frame.DataFrame.Header)
		midPoint := headerLength / 2
		commitNumber := resp.Frame.DataFrame.Header[midPoint].Offset

		for _, cl := range resp.Summary.Clusters {
			if cl.StepPoint.Offset == commitNumber && len(cl.Keys) >= cfg.MinimumNum {
				isLow := cl.StepFit.Status == stepfit.LOW && (cfg.DirectionAsString == alerts.DOWN || cfg.DirectionAsString == alerts.BOTH)
				isHigh := cl.StepFit.Status == stepfit.HIGH && (cfg.DirectionAsString == alerts.UP || cfg.DirectionAsString == alerts.BOTH)
				if isLow || isHigh {
					filteredSummary.Clusters = append(filteredSummary.Clusters, cl)
				}
			}
		}

		// If we have any confirmed clusters, create a new response for them.
		if len(filteredSummary.Clusters) > 0 {
			// Create a new response with the filtered summary.
			confirmedResp := &regression.RegressionDetectionResponse{
				Summary: filteredSummary,
				Frame:   resp.Frame,
				Message: resp.Message,
			}
			ret = append(ret, confirmedResp)
		}
	}
	return ret
}

// NewDefaultRegressionRefiner returns a new instance of DefaultRegressionRefiner.
func NewDefaultRegressionRefiner() regression.RegressionRefiner {
	return &DefaultRegressionRefiner{}
}
