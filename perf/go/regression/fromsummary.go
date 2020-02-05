package regression

import (
	"context"
	"fmt"
	"math"

	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/stepfit"
)

// RegressionFromClusterResponse returns the commit for the regression along with
// the *Regression.
func RegressionFromClusterResponse(ctx context.Context, resp *ClusterResponse, cfg *alerts.Alert, cidl *cid.CommitIDLookup) (*cid.CommitDetail, *Regression, error) {
	ret := &Regression{}
	headerLength := len(resp.Frame.DataFrame.Header)
	midPoint := headerLength / 2

	midOffset := resp.Frame.DataFrame.Header[midPoint].Offset

	id := &cid.CommitID{
		Offset: int(midOffset),
	}

	details, err := cidl.Lookup(ctx, []*cid.CommitID{id})
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to look up commit %v: %s", *id, err)
	}
	lastLowRegression := float64(-1.0)
	lastHighRegression := float64(-1.0)
	for _, cl := range resp.Summary.Clusters {
		if cl.StepPoint.Offset == midOffset {
			if cl.StepFit.Status == stepfit.LOW && len(cl.Keys) >= cfg.MinimumNum && (cfg.Direction == alerts.DOWN || cfg.Direction == alerts.BOTH) {
				if math.Abs(float64(cl.StepFit.Regression)) > lastLowRegression {
					ret.Frame = resp.Frame
					ret.Low = cl
					ret.LowStatus = TriageStatus{
						Status: UNTRIAGED,
					}
					lastLowRegression = math.Abs(float64(cl.StepFit.Regression))
				}
			}
			if cl.StepFit.Status == stepfit.HIGH && len(cl.Keys) >= cfg.MinimumNum && (cfg.Direction == alerts.UP || cfg.Direction == alerts.BOTH) {
				if math.Abs(float64(cl.StepFit.Regression)) > lastHighRegression {
					ret.Frame = resp.Frame
					ret.High = cl
					ret.HighStatus = TriageStatus{
						Status: UNTRIAGED,
					}
					lastHighRegression = math.Abs(float64(cl.StepFit.Regression))
				}
			}
		}
	}
	return details[0], ret, nil
}
