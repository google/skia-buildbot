package internal

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.temporal.io/sdk/workflow"
)

const (
	functional  = "Functional"
	performance = "Performance"
)

type CommitPairValues struct {
	Lower  CommitValues
	Higher CommitValues
}

type CommitValues struct {
	Commit      *midpoint.CombinedCommit
	Values      []float64
	ErrorValues []float64
}

type CombinedResults struct {
	Result      *compare.CompareResults
	OtherResult *compare.CompareResults // record the other comparison
	ResultType  string                  // either Functional or Performance
}

// TODO(sunxiaodi@): Change GetAllDataForCompareLocalActivity to a regular function
func GetAllDataForCompareLocalActivity(ctx context.Context, lbr *BisectRun, hbr *BisectRun, chart string) (*CommitPairValues, error) {
	return &CommitPairValues{
		Lower:  CommitValues{lbr.Build.Commit, lbr.AllValues(chart), lbr.AllErrorValues(chart)},
		Higher: CommitValues{hbr.Build.Commit, hbr.AllValues(chart), hbr.AllErrorValues(chart)},
	}, nil
}

// CompareActivity wraps compare.ComparePerformance and compare.CompareFunctional as activity
//
// commitA and commitB are passed in to make it easier to see on the Temporal
// UI what two commits are being tested. Errors are recorded in the activity but
// the ErrorVerdict is not passed back to the main workflow.
func CompareActivity(ctx context.Context, allValues CommitPairValues, magnitude, errRate float64, direction compare.ImprovementDir) (*CombinedResults, error) {
	// TODO(sunxiaodi@): skip functional analysis if there are no errors
	funcResult, err := compare.CompareFunctional(allValues.Lower.ErrorValues, allValues.Higher.ErrorValues, errRate)
	if err != nil {
		return &CombinedResults{Result: funcResult, ResultType: functional}, skerr.Wrap(err)
	}
	// always return different verdicts
	if funcResult.Verdict == compare.Different {
		return &CombinedResults{
			Result:     funcResult,
			ResultType: functional,
		}, nil
	}

	perfResult, err := compare.ComparePerformance(allValues.Lower.Values, allValues.Higher.Values, magnitude, direction)
	if err != nil {
		return &CombinedResults{
			Result:      funcResult,
			OtherResult: perfResult,
			ResultType:  functional,
		}, skerr.Wrap(err)
	}

	// decide which verdict to return
	// occurs if all benchmark runs fail and we are relying on functional analysis results
	if perfResult.Verdict == compare.NilVerdict {
		return &CombinedResults{
			Result:      funcResult,
			OtherResult: perfResult,
			ResultType:  functional,
		}, nil
	}
	if funcResult.Verdict == compare.Unknown && perfResult.Verdict == compare.Same {
		return &CombinedResults{
			Result:      funcResult,
			OtherResult: perfResult,
			ResultType:  functional,
		}, nil
	}
	return &CombinedResults{
		Result:      perfResult,
		OtherResult: funcResult,
		ResultType:  performance,
	}, nil
}

func compareRuns(ctx workflow.Context, lRun, hRun *BisectRun, chart string, mag float64, dir compare.ImprovementDir) (*compare.CompareResults, error) {
	var commitPairAllValues CommitPairValues
	if err := workflow.ExecuteLocalActivity(ctx, GetAllDataForCompareLocalActivity, lRun, hRun, chart).Get(ctx, &commitPairAllValues); err != nil {
		return nil, skerr.Wrap(err)
	}

	var result *CombinedResults
	if err := workflow.ExecuteActivity(ctx, CompareActivity, commitPairAllValues, mag, compare.DefaultFunctionalErrRate, dir).Get(ctx, &result); err != nil {
		return nil, skerr.Wrap(err)
	}

	return result.Result, nil
}
