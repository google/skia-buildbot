package internal

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.temporal.io/sdk/workflow"
)

const (
	functional  = "Functional"
	performance = "Performance"
)

type Result struct {
	Result *compare.CompareResults
	Type   string // either Functional or Performance
}

// ComparePerformanceActivity wraps compare.ComparePerformance as activity
func ComparePerformanceActivity(ctx context.Context, valuesA, valuesB []float64, magnitude float64, direction compare.ImprovementDir) (*compare.CompareResults, error) {
	return compare.ComparePerformance(valuesA, valuesB, magnitude, direction)
}

// CompareFunctionalActivity wraps compare.CompareFunctional as activity
func CompareFunctionalActivity(ctx context.Context, valuesA, valuesB []float64, errRate float64) (*compare.CompareResults, error) {
	return compare.CompareFunctional(valuesA, valuesB, errRate)
}

// TODO(sunxiaodi@): consolidate compareRuns to minimize noise generated in temporal
// workflow event history
func compareRuns(ctx workflow.Context, lRun, hRun *BisectRun, chart string, mag float64, dir compare.ImprovementDir) (*Result, error) {
	// conduct functional analysis
	var lValues, hValues *CommitValues
	if err := workflow.ExecuteLocalActivity(ctx, GetErrorValuesLocalActivity, lRun, chart).Get(ctx, &lValues); err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := workflow.ExecuteLocalActivity(ctx, GetErrorValuesLocalActivity, hRun, chart).Get(ctx, &hValues); err != nil {
		return nil, skerr.Wrap(err)
	}

	// TODO(sunxiaodi@): skip functional analysis if there are no errors
	var funcResult *compare.CompareResults
	if err := workflow.ExecuteActivity(ctx, CompareFunctionalActivity, lValues.Values, hValues.Values, compare.DefaultFunctionalErrRate).Get(ctx, &funcResult); err != nil {
		return nil, skerr.Wrap(err)
	}
	// always return different verdicts
	if funcResult.Verdict == compare.Different {
		return &Result{
			Result: funcResult,
			Type:   functional,
		}, nil
	}

	// conduct performance analysis
	if err := workflow.ExecuteLocalActivity(ctx, GetAllValuesLocalActivity, lRun, chart).Get(ctx, &lValues); err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := workflow.ExecuteLocalActivity(ctx, GetAllValuesLocalActivity, hRun, chart).Get(ctx, &hValues); err != nil {
		return nil, skerr.Wrap(err)
	}

	var result *compare.CompareResults
	if err := workflow.ExecuteActivity(ctx, ComparePerformanceActivity, lValues.Values, hValues.Values, mag, dir).Get(ctx, &result); err != nil {
		return nil, skerr.Wrap(err)
	}

	// decide which verdict to return
	if funcResult.Verdict == compare.Unknown && result.Verdict == compare.Same {
		return &Result{
			Result: funcResult,
			Type:   functional,
		}, nil
	}
	return &Result{
		Result: result,
		Type:   performance,
	}, nil
}
