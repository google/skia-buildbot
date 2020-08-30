package regression

import (
	"context"
	"fmt"
	"math"

	"go.skia.org/infra/perf/go/alerts"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/stepfit"
)

// RegressionFromClusterResponse returns the commit for the regression along with
// the *Regression.
func RegressionFromClusterResponse(ctx context.Context, resp *RegressionDetectionResponse, cfg *alerts.Alert, perfGit *perfgit.Git) (perfgit.Commit, *Regression, error) {
	ret := &Regression{}
	headerLength := len(resp.Frame.DataFrame.Header)
	midPoint := headerLength / 2
	commitNumber := resp.Frame.DataFrame.Header[midPoint].Offset

	details, err := perfGit.Details(ctx, commitNumber)
	if err != nil {
		return perfgit.Commit{}, nil, fmt.Errorf("Failed to look up commit %d: %s", commitNumber, err)
	}
	lastLowRegression := float64(-1.0)
	lastHighRegression := float64(-1.0)
	for _, cl := range resp.Summary.Clusters {
		if cl.StepPoint.Offset == commitNumber {
			if cl.StepFit.Status == stepfit.LOW && len(cl.Keys) >= cfg.MinimumNum && (cfg.DirectionAsString == alerts.DOWN || cfg.DirectionAsString == alerts.BOTH) {
				if math.Abs(float64(cl.StepFit.Regression)) > lastLowRegression {
					ret.Frame = resp.Frame
					ret.Low = cl
					ret.LowStatus = TriageStatus{
						Status: Untriaged,
					}
					lastLowRegression = math.Abs(float64(cl.StepFit.Regression))
				}
			}
			if cl.StepFit.Status == stepfit.HIGH && len(cl.Keys) >= cfg.MinimumNum && (cfg.DirectionAsString == alerts.UP || cfg.DirectionAsString == alerts.BOTH) {
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
