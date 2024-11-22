package internal

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.temporal.io/sdk/workflow"
)

const (
	Functional  = "Functional"
	Performance = "Performance"
	nudgeFactor = float64(1e-10)
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
	valuesB = handlePairwiseEdgeCase(valuesA, valuesB)
	return compare.ComparePairwise(valuesA, valuesB, dir)
}

// if every value in valuesA is identicial and every value in valuesB is identical,
// pairwise comparison will fail to return a confidence interval because the
// wilcoxon_signed_rank.go runs into an error:
// "cannot compute confidence interval when all observations are zero or tied"
// This edge case can happen for some very consistent, near-deterministic benchmark runs.
// However, this does not mean that the data collected is not useful. Only one data point
// needs to be nudged to return a confidence interval. Here we manipulate the data
// on a significant figure that should not have any adverse affect on the overall sample
func handlePairwiseEdgeCase(valuesA, valuesB []float64) []float64 {
	allSameA := allSameValues(valuesA)
	allSameB := allSameValues(valuesB)
	if allSameA && allSameB {
		sklog.Warningf("all values in A are identical and all values in B are identical. Nudging one element by %v in valuesB. ValuesA: %v; ValuesB: %v", valuesB[0]*nudgeFactor, valuesA, valuesB)
		valuesB[0] += valuesB[0] * nudgeFactor
	}
	return valuesB
}

func allSameValues(values []float64) bool {
	for i := 1; i < len(values); i++ {
		if values[i] != values[0] {
			return false
		}
	}
	return true
}
