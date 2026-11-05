// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package refiner

import (
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
)

// findRegressionAreas identifies contiguous areas of interesting regressions.
//
// The purpose of this function is to split a list of RegressionDetectionResponses into regression areas suitable for the next processing steps.
//
// What do we mean by interesting and uninteresting regressions?
//
// Interesting regression: A regression that exceeds the threshold (status LOW or HIGH)
// AND for which the alert's direction is appropriate (e.g., an UP alert requires a HIGH regression,
// while a DOWN alert requires a LOW regression). These are the regressions we want to analyze.
//
// Note regarding current implementation of uninteresting regressions:
// In the current state of the system, we detect uninteresting regressions by checking if the cluster size is zero (len == 0).
// This is because we don't save clustering info for uninteresting regressions. We also check for the UNINTERESTING status
// because it is logically correct. However, for now, the code does not send this status because this information is kept
// in the cluster, and for uninteresting regressions, the cluster is empty.
//
// A group of regressions is only considered valid if it is bounded by "uninteresting" regressions
// (regressions where no anomaly was detected, or they are filtered out by direction)
// or regressions of a different direction. This ensures we process distinct, bounded
// anomaly segments. If we don't have regression areas which are bounded by such regressions,
// we will ignore them until we reach a valid boundary.
func findRegressionAreas(resps []*regression.RegressionDetectionResponse, cfg *alerts.Alert) [][]*regression.RegressionDetectionResponse {
	var areas [][]*regression.RegressionDetectionResponse

	n := len(resps)
	i := 0
	for i < n {
		if !isInteresting(resps[i], cfg) {
			i++
			continue
		}

		// Found a new group: contiguous block of same-status responses
		start := i
		status := getStatus(resps[i])

		// Advance until status changes or we hit uninteresting/end
		for i < n && isInteresting(resps[i], cfg) && getStatus(resps[i]) == status {
			i++
		}
		end := i // Exclusive end index of the group

		// Check boundaries.
		// A group is valid ONLY if it has an element on both sides (UNINTERESTING or Different Status).
		// Since we skip UNINTERESTING at the start of loop, if start > 0, resps[start-1] is UNINTERESTING or Different Status.
		// If end < n, resps[end] is UNINTERESTING or Different Status.
		hasLeftBoundary := start > 0
		hasRightBoundary := end < n

		if hasLeftBoundary && hasRightBoundary {
			// Append the slice of the group
			areas = append(areas, resps[start:end])
		}
	}

	// Post-process areas: Split very long areas.
	// "Very long" defined as length > 2 * Radius.
	// Split into chunks of at most 2 * Radius length.
	var splitAreas [][]*regression.RegressionDetectionResponse
	for _, area := range areas {
		if len(area) > 2*cfg.Radius {
			splitAreas = append(splitAreas, splitBigRegressionArea(area, cfg.Radius)...)
		} else {
			splitAreas = append(splitAreas, area)
		}
	}

	// Check if all regressions are interesting.
	// If so, return each regression as a separate group of size 1.
	if len(splitAreas) == 0 && n > 0 {
		if areas := handleAllInterestingCase(resps, cfg); areas != nil {
			return areas
		}
	}

	return splitAreas
}

func splitBigRegressionArea(area []*regression.RegressionDetectionResponse, radius int) [][]*regression.RegressionDetectionResponse {
	var chunks [][]*regression.RegressionDetectionResponse
	chunkSize := 2 * radius
	for i := 0; i < len(area); i += chunkSize {
		end := i + chunkSize
		if end > len(area) {
			end = len(area)
		}
		chunks = append(chunks, area[i:end])
	}
	return chunks
}

// If the regression area covers the whole range (i.e. every regression is interesting), for safety we process
// each regression independently and return groups of size one. This matches the behavior of the default
// regression refiner. This edge case is considered rare and is monitored.
func handleAllInterestingCase(resps []*regression.RegressionDetectionResponse, cfg *alerts.Alert) [][]*regression.RegressionDetectionResponse {
	for _, r := range resps {
		if !isInteresting(r, cfg) {
			return nil
		}
	}

	logAllInterestingCase(resps, cfg)

	areas := make([][]*regression.RegressionDetectionResponse, len(resps))
	for i, r := range resps {
		areas[i] = []*regression.RegressionDetectionResponse{r}
	}
	return areas
}

func logAllInterestingCase(resps []*regression.RegressionDetectionResponse, cfg *alerts.Alert) {
	// Skip logging for Const step as it can produce contiguous regressions.
	if cfg.Step == types.Const {
		return
	}

	// Try to get trace name and commit range for logging
	traceName := resps[0].Summary.Clusters[0].Keys[0]
	startCommit := resps[0].Summary.Clusters[0].StepPoint.Offset
	endCommit := resps[len(resps)-1].Summary.Clusters[0].StepPoint.Offset

	sklog.Warningf("All regressions in the range are interesting. Returning each as a separate group. Trace: %s, Start: %d, End: %d", traceName, startCommit, endCommit)
	metrics2.GetCounter("anomaly_bounds_refiner_warnings", map[string]string{
		"cause": "regression_areas.all_interesting_case",
	}).Inc(1)
}

// isInteresting determines if a regression is considered interesting based on the alert configuration.
//
// An interesting regression is one that exceeds the threshold (status LOW or HIGH)
// AND for which the alert's direction is appropriate (e.g., an UP alert requires a HIGH regression,
// while a DOWN alert requires a LOW regression).
//
// Uninteresting regressions are those that do not meet these criteria (e.g., cluster size 0
// or a LOW regression for an UP alert) and serve as boundaries for interesting areas.
func isInteresting(r *regression.RegressionDetectionResponse, cfg *alerts.Alert) bool {

	if len(r.Summary.Clusters) == 0 {
		return false
	}

	s := r.Summary.Clusters[0].StepFit.Status

	if s == stepfit.UNINTERESTING {
		return false
	}

	// Filter by direction
	switch cfg.DirectionAsString {
	case alerts.UP:
		if s == stepfit.LOW {
			return false
		}
	case alerts.DOWN:
		if s == stepfit.HIGH {
			return false
		}
	}
	return true
}

func getStatus(r *regression.RegressionDetectionResponse) stepfit.StepFitStatus {
	return r.Summary.Clusters[0].StepFit.Status
}
