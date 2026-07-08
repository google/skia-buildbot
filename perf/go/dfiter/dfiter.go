// Package dfiter efficiently creates dataframes used in regression detection.
package dfiter

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"time"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
)

// ErrInsufficientData is returned by the DataFrameIterator if all the queries
// compeleted successfully, but there wasn't enough data to continue.
var ErrInsufficientData = errors.New("insufficient data")

const (
	// minRadiusRatioForRefinement defines the minimum ratio of data points to radius
	// required when anomaly bounds refiners are enabled.
	minRadiusRatioForRefinement = 3.5
)

// DataFrameIterator is an iterator that produces DataFrames.
//
//	for it.Next() {
//	  df, err := it.Value(ctx)
//	  // Do something with df.
//	}
type DataFrameIterator interface {
	Next() bool
	Value(ctx context.Context) (*dataframe.DataFrame, error)
}

// NewDataFrameIterator returns a DataFrameIterator that produces a set of
// dataframes for the given query, domain, and alert.
//
// If domain.Offset is non-zero then we want the iterator to return a single
// dataframe of alert.Radius around the specified commit. Otherwise it returns a
// series of dataframes of size 2*alert.Radius+1 sliced from a single dataframe
// of size domain.N.
func NewDataFrameIterator(
	ctx context.Context,
	progress progress.Progress,
	dfBuilder dataframe.DataFrameBuilder,
	perfGit perfgit.Git,
	regressionStateCallback types.ProgressCallback,
	queryAsString string,
	domain types.Domain,
	alert *alerts.Alert,
	anomalyConfig config.AnomalyConfig,
	dfProvider *DfProvider,
) (DataFrameIterator, error) {
	ctx, span := trace.StartSpan(ctx, "dfiter.NewDataFrameIterator")
	defer span.End()

	// Because of GroupBy the Alert query isn't the one we use, instead a
	// sub-query is passed in.
	u, err := url.ParseQuery(queryAsString)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	q, err := query.New(u)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	windowSize := stepfit.GetWindowSize(alert.Radius, alert.Step, alert.DetectionRule)
	minPoints := windowSize
	var df *dataframe.DataFrame
	if !domain.IsSingleCommitMode() {
		if anomalyConfig.SettlingTime != 0 {
			currentTime := now.Now(ctx)
			latestAllowedPoints := currentTime.Add(-1 * time.Duration(anomalyConfig.SettlingTime))
			if latestAllowedPoints.Before(domain.End) {
				domain.End = latestAllowedPoints
			}
		}

		if dfProvider != nil {
			df, err = dfProvider.GetDataFrame(ctx, dfBuilder, q, domain.End, domain.N, progress)
			if err != nil {
				// Log the error and fall back to the usual NewNFromQuery.
				sklog.Errorf("Failed to get DataFrame from cache: %v", err)
			}
		}
		if df == nil {
			df, err = dfBuilder.NewNFromQuery(ctx, domain.End, q, domain.N, progress)
		}

		if err != nil {
			if regressionStateCallback != nil {
				regressionStateCallback("Failed querying the data due to an internal error.")
			}
			return nil, skerr.Wrapf(err, "Failed to build dataframe iterator source dataframe")
		}
	} else {
		// We can get an iterator that returns just a single dataframe by making
		// sure that the size of the origin dataframe is the same size as the
		// slicer size, so we set them both to windowSize (2*Radius+1 for OriginalStep, 2*Radius for others).
		// Need to find an End time, which is the commit time of the commit at:
		// Offset+Radius for OriginalStep (window [Offset-Radius .. Offset+Radius], size 2R+1)
		// Offset+Radius-1 for non-OriginalStep (window [Offset-Radius .. Offset+Radius-1], size 2R)
		endOffset := alert.Radius
		if !stepfit.UsesOriginalStep(alert.Step, alert.DetectionRule) {
			endOffset = alert.Radius - 1
		}
		endCommit := types.CommitNumber(int(domain.Offset) + endOffset)
		commit, err := perfGit.CommitFromCommitNumber(ctx, endCommit)
		if err != nil {
			if regressionStateCallback != nil {
				regressionStateCallback(fmt.Sprintf("Not a valid commit number %d. Make sure you choose a commit old enough to have %d commits before it and %d commits after it.", endCommit, alert.Radius, endOffset))
			}

			return nil, skerr.Wrapf(err, "Failed to look up CommitNumber of a single cluster request.")
		}
		df, err = dfBuilder.NewNFromQuery(ctx, time.Unix(commit.Timestamp, 0), q, int32(windowSize), progress)
		if err != nil {
			if regressionStateCallback != nil {
				regressionStateCallback("Failed querying the data due to an internal error.")
			}
			return nil, skerr.Wrapf(err, "Failed to build dataframe iterator source dataframe.")
		}
	}
	// For single-commit validation (domain.IsSingleCommitMode() is true), we only query a window of size
	// windowSize, so minPoints must stay at windowSize. We only increase the required
	// minPoints for refinement/localization when performing continuous detection (!domain.IsSingleCommitMode()).
	if !domain.IsSingleCommitMode() && (anomalyConfig.UseAnomalyLocalization || anomalyConfig.UseImprovedAnomalyBoundsRefiner) {
		refinerMinPoints := int(math.Ceil(minRadiusRatioForRefinement * float64(alert.Radius)))
		if refinerMinPoints > minPoints {
			minPoints = refinerMinPoints
		}
	}

	if len(df.Header) < minPoints {
		if regressionStateCallback != nil {
			regressionStateCallback(fmt.Sprintf("Query didn't return enough data points: Got %d. Want %d.", len(df.Header), minPoints))
		}
		sklog.Warningf("Query didn't return enough data points: Got %d. Want %d.", len(df.Header), minPoints)
		return nil, ErrInsufficientData
	}
	// Record the total number of floating point values that were just queried
	// fron the database. Since we know the size of a float we can use this to
	// roughly estimate the MB/s of regression detection.
	metrics2.GetCounter("perf_regression_detection_floats").Inc(int64(len(df.Header) * len(df.TraceSet)))

	// The DfTraceSlicer will only work for stepfit (i.e individual and not kmeans)
	if alert.Algo == types.StepFitGrouping {
		return NewStepFitDfTraceSlicer(df, windowSize), nil
	} else {
		return NewKmeansDataframeSlicer(df, windowSize), nil
	}
}
