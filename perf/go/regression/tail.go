package regression

import (
	"math"
	"sort"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/stepfit"
)

// FilterTail - Please see go/calmbench-trace-tail for how we filter the tail.
func FilterTail(trace []float32, quantile float64, multiplier float64, slack float64) float32 {
	if len(trace) == 0 {
		return 0
	}
	tail := trace[len(trace)-1]
	if tail == vec32.MISSING_DATA_SENTINEL {
		return 0
	}

	sortedTrace := make([]float64, 0, len(trace))
	for i := 0; i < len(trace)-1; i++ {
		if trace[i] != vec32.MISSING_DATA_SENTINEL {
			sortedTrace = append(sortedTrace, float64(trace[i]))
		}
	}
	sort.Float64s(sortedTrace)

	n := len(sortedTrace)
	p := int(math.Floor(float64(n-1) * quantile))
	lowerBound := math.Min(0, sortedTrace[p])
	upperBound := math.Max(0, sortedTrace[n-1-p])

	tail64 := float64(tail)
	if tail64 > upperBound*multiplier+slack || tail64 < lowerBound*multiplier-slack {
		return tail
	} else {
		return 0
	}
}

// Tail finds regressions in calmbench data.
func Tail(df *dataframe.DataFrame, k int, stddevThreshold float32, progress clustering2.Progress, interesting float32) (*clustering2.ClusterSummaries, error) {
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

		quantile := 1 / float64(interesting)
		slack := float64(k) * 0.01
		const MULTIPLIER = 2 // TODO(liyuqian): Make this configurable

		tail := FilterTail(trace, quantile, MULTIPLIER, slack)
		isLow = tail < 0
		isHigh = tail > 0
		sf.TurningPoint = len(trace) - 1

		// If stepfit is at the middle and if it is a step up or down.
		if isLow {
			if low.StepFit.Status == "" {
				low.StepFit = sf
				low.StepFit.Status = stepfit.LOW // for TAIL_ALGO
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
				high.StepFit.Status = stepfit.HIGH // for TAIL_ALGO
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
	return ret, nil
}
