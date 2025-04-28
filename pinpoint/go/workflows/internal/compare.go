package internal

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.temporal.io/sdk/workflow"
)

const (
	Functional  = "Functional"
	Performance = "Performance"
)

type CommitPairValues struct {
	Lower  CommitValues
	Higher CommitValues
}

type CommitValues struct {
	Commit      *common.CombinedCommit
	Values      []float64
	ErrorValues []float64
}

type CombinedResults struct {
	Result           *compare.CompareResults
	OtherResult      *compare.CompareResults // record the other comparison
	ResultType       string                  // either Functional or Performance
	CommitPairValues CommitPairValues
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
	funcResult, err := compare.CompareFunctional(allValues.Lower.ErrorValues, allValues.Higher.ErrorValues, errRate)
	if err != nil {
		return &CombinedResults{
			Result:           funcResult,
			ResultType:       Functional,
			CommitPairValues: allValues,
		}, skerr.Wrap(err)
	}
	// always return different verdicts
	if funcResult.Verdict == compare.Different {
		return &CombinedResults{
			Result:           funcResult,
			ResultType:       Functional,
			CommitPairValues: allValues,
		}, nil
	}

	perfResult, err := compare.ComparePerformance(allValues.Lower.Values, allValues.Higher.Values, magnitude, direction)
	if err != nil {
		return &CombinedResults{
			Result:           funcResult,
			OtherResult:      perfResult,
			ResultType:       Functional,
			CommitPairValues: allValues,
		}, skerr.Wrap(err)
	}

	// decide which verdict to return
	// occurs if all benchmark runs fail and we are relying on functional analysis results
	if perfResult.Verdict == compare.NilVerdict {
		return &CombinedResults{
			Result:           funcResult,
			OtherResult:      perfResult,
			ResultType:       Functional,
			CommitPairValues: allValues,
		}, nil
	}
	if funcResult.Verdict == compare.Unknown && perfResult.Verdict == compare.Same {
		return &CombinedResults{
			Result:           funcResult,
			OtherResult:      perfResult,
			ResultType:       Functional,
			CommitPairValues: allValues,
		}, nil
	}
	return &CombinedResults{
		Result:           perfResult,
		OtherResult:      funcResult,
		ResultType:       Performance,
		CommitPairValues: allValues,
	}, nil
}

func compareRuns(ctx workflow.Context, lRun, hRun *BisectRun, chart string, mag float64, dir compare.ImprovementDir) (*CombinedResults, error) {
	var commitPairAllValues CommitPairValues
	if err := workflow.ExecuteLocalActivity(ctx, GetAllDataForCompareLocalActivity, lRun, hRun, chart).Get(ctx, &commitPairAllValues); err != nil {
		return nil, skerr.Wrap(err)
	}

	var result *CombinedResults
	if err := workflow.ExecuteActivity(ctx, CompareActivity, commitPairAllValues, mag, compare.DefaultFunctionalErrRate, dir).Get(ctx, &result); err != nil {
		return nil, skerr.Wrap(err)
	}

	return result, nil
}

// ComparePairwiseActivity wraps compare.ComparePairwise as a temporal activity
func ComparePairwiseActivity(ctx context.Context, valuesA, valuesB []float64, dir compare.ImprovementDir) (*compare.ComparePairwiseResult, error) {
	return compare.ComparePairwise(valuesA, valuesB, dir)
}
