package regression

import (
	"context"
	"math"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/alerts"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/stepfit"
)

// ConfirmedRegressionFromClusterResponse returns the commit for the regression along with
// the *Regression.
func ConfirmedRegressionFromClusterResponse(ctx context.Context, resp *ConfirmedRegression, cfg *alerts.Alert, perfGit perfgit.Git) (provider.Commit, *Regression, error) {
	ret := &Regression{}
	commitNumber := resp.DisplayCommitNumber
	details, err := perfGit.CommitFromCommitNumber(ctx, commitNumber)
	if err != nil {
		return perfgit.BadCommit, nil, skerr.Wrapf(err, "Failed to look up commit %d", commitNumber)
	}
	lastLowRegression := float64(-1.0)
	lastHighRegression := float64(-1.0)
	for _, cl := range resp.Summary.Clusters {
		if cl.StepPoint.Offset == resp.DisplayCommitNumber {
			if cl.StepFit.Status == stepfit.LOW {
				if math.Abs(float64(cl.StepFit.Regression)) > lastLowRegression {
					ret.Frame = resp.Frame
					ret.Low = cl
					ret.LowStatus = TriageStatus{
						Status: Untriaged,
					}
					lastLowRegression = math.Abs(float64(cl.StepFit.Regression))
				}
			}
			if cl.StepFit.Status == stepfit.HIGH {
				if math.Abs(float64(cl.StepFit.Regression)) > lastHighRegression {
					ret.Frame = resp.Frame
					ret.High = cl
					ret.HighStatus = TriageStatus{
						Status: Untriaged,
					}
					lastHighRegression = math.Abs(float64(cl.StepFit.Regression))
				}
			}
		}
	}
	return details, ret, nil
}
