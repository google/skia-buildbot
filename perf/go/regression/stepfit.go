package regression

import (
	"context"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
)

// StepFit finds regressions by looking at each trace individually and seeing if that looks like a regression.
func StepFit(ctx context.Context, df *dataframe.DataFrame, k int, stddevThreshold float32, progress clustering2.Progress, interesting float32, stepDetection types.StepDetection) (*clustering2.ClusterSummaries, error) {
	low := clustering2.NewClusterSummary(ctx)
	high := clustering2.NewClusterSummary(ctx)
	// Normalize each trace and then run through stepfit. If interesting then
	// add to appropriate cluster.
	count := 0
	for key, trace := range df.TraceSet {
		count++
		if count%10000 == 0 {
			sklog.Infof("stepfit count: %d", count)
		}
		var sf *stepfit.StepFit
		sf = stepfit.GetStepFitAtMid(trace, stddevThreshold, interesting, stepDetection)

		isLow := sf.Status == stepfit.LOW
		isHigh := sf.Status == stepfit.HIGH

		// If stepfit is at the middle and if it is a step up or down.
		if isLow {
			if low.StepFit.Status == "" {
				low.StepFit = sf
				low.StepFit.Status = stepfit.LOW
				low.StepPoint = df.Header[sf.TurningPoint]
				low.Centroid = vec32.Dup(trace)
			}
			low.Num++
			if low.Num < config.MaxSampleTracesPerCluster {
				low.Keys = append(low.Keys, key)
			}
		} else if isHigh {
			if high.StepFit.Status == "" {
				high.StepFit = sf
				high.StepFit.Status = stepfit.HIGH
				high.StepPoint = df.Header[sf.TurningPoint]
				high.Centroid = vec32.Dup(trace)
			}
			high.Num++
			if high.Num < config.MaxSampleTracesPerCluster {
				high.Keys = append(high.Keys, key)
			}
		}
	}
	sklog.Infof("Found LOW: %d HIGH: %d", low.Num, high.Num)
	ret := &clustering2.ClusterSummaries{
		Clusters:        []*clustering2.ClusterSummary{},
		K:               k,
		StdDevThreshold: stddevThreshold,
	}
	if low.Num > 0 {
		low.ParamSummaries = clustering2.GetParamSummariesForKeys(low.Keys)
		ret.Clusters = append(ret.Clusters, low)
	}
	if high.Num > 0 {
		high.ParamSummaries = clustering2.GetParamSummariesForKeys(high.Keys)
		ret.Clusters = append(ret.Clusters, high)
	}
	return ret, nil
}
