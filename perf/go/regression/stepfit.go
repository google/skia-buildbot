package regression

import (
	"fmt"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/stepfit"
)

func StepFit(df *dataframe.DataFrame, k int, stddevThreshold float32, progress clustering2.Progress, interesting float32) (*clustering2.ClusterSummaries, error) {
	low := clustering2.NewClusterSummary()
	high := clustering2.NewClusterSummary()
	// Normalize each trace and then run through stepfit. If interesting then
	// add to appropriate cluster.
	count := 0
	for key, trace := range df.TraceSet {
		count++
		if count%10000 == 0 {
			sklog.Infof("stepfit count: %d", count)
		}
		t := vec32.Dup(trace)
		vec32.Norm(t, stddevThreshold)
		sf := stepfit.GetStepFitAtMid(t, interesting)

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
			if low.Num < config.MAX_SAMPLE_TRACES_PER_CLUSTER {
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
			if high.Num < config.MAX_SAMPLE_TRACES_PER_CLUSTER {
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
	if err := clustering2.ShortcutFromKeys(ret); err != nil {
		return nil, fmt.Errorf("Failed to write shortcut for keys: %s", err)
	}
	return ret, nil
}
