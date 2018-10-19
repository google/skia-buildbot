package clustering2

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ctrace2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/kmeans"
	"go.skia.org/infra/perf/go/shortcut2"
	"go.skia.org/infra/perf/go/stepfit"
)

const (

	// K is the k in k-means.
	K = 50

	// MAX_KMEANS_ITERATIONS is the maximum number of k-means iterations to run.
	MAX_KMEANS_ITERATIONS = 100

	// KMEAN_EPSILON is the smallest change in the k-means total error we will
	// accept per iteration.  If the change in error falls below KMEAN_EPSILON
	// the iteration will terminate.
	KMEAN_EPSILON = 1.0
)

// ValueWeight is a weight proportional to the number of times the parameter
// Value appears in a cluster. Used in ClusterSummary.
type ValueWeight struct {
	Value  string `json:"value"`
	Weight int    `json:"weight"`
}

// ClusterSummary is a summary of a single cluster of traces.
type ClusterSummary struct {
	// Centroid is the calculated centroid of the cluster.
	Centroid []float32 `json:"centroid"`

	// Keys of all the members of the Cluster.
	//
	// The keys are sorted so that the ones at the beginning of the list are
	// closest to the centroid.
	//
	// Note: This value is not serialized to JSON.
	Keys []string `json:"-"`

	// Shortcut is the id of a shortcut for the above Keys.
	Shortcut string `json:"shortcut"`

	// ParamSummaries is a summary of all the parameters in the cluster.
	ParamSummaries map[string][]ValueWeight `json:"param_summaries"`

	// StepFit is info on the fit of the centroid to a step function.
	StepFit *stepfit.StepFit `json:"step_fit"`

	// StepPoint is the ColumnHeader for the step point.
	StepPoint *dataframe.ColumnHeader `json:"step_point"`

	// Num is the number of observations that are in this cluster.
	Num int `json:"num"`
}

// newClusterSummary returns a new ClusterSummary.
func newClusterSummary() *ClusterSummary {
	return &ClusterSummary{
		Keys:           []string{},
		ParamSummaries: map[string][]ValueWeight{},
		StepFit:        &stepfit.StepFit{},
		StepPoint:      &dataframe.ColumnHeader{},
	}
}

// ClusterSummaries is one summary for each cluster that the k-means clustering
// found.
type ClusterSummaries struct {
	Clusters        []*ClusterSummary
	StdDevThreshold float32
	K               int
}

// chooseK chooses a random sample of k observations. Used as the starting
// point for the k-means clustering.
func chooseK(observations []kmeans.Clusterable, k int) []kmeans.Centroid {
	popN := len(observations)
	centroids := make([]kmeans.Centroid, k)
	for i := 0; i < k; i++ {
		centroids[i] = observations[rand.Intn(popN)].(*ctrace2.ClusterableTrace).Dup(ctrace2.CENTROID_KEY)
	}
	return centroids
}

// ValueWeightSortable is a utility class for sorting the ValueWeight's by Weight.
type ValueWeightSortable []ValueWeight

func (p ValueWeightSortable) Len() int           { return len(p) }
func (p ValueWeightSortable) Less(i, j int) bool { return p[i].Weight > p[j].Weight } // Descending.
func (p ValueWeightSortable) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// getParamSummaries summarizes all the parameters for all observations in a
// cluster.
//
// The return value is an array of []ValueWeight's, one []ValueWeight per
// parameter. The members of each []ValueWeight are sorted by the Weight, with
// higher Weight's first.
func getParamSummaries(cluster []kmeans.Clusterable) map[string][]ValueWeight {
	keys := make([]string, 0, len(cluster))
	for _, o := range cluster {
		key := o.(*ctrace2.ClusterableTrace).Key
		if key == ctrace2.CENTROID_KEY {
			continue
		}
		keys = append(keys, key)
	}
	return getParamSummariesForKeys(keys)
}

// getParamSummariesForKeys summarizes all the parameters for all observations in a
// cluster.
//
// The return value is an array of []ValueWeight's, one []ValueWeight per
// parameter. The members of each []ValueWeight are sorted by the Weight, with
// higher Weight's first.
func getParamSummariesForKeys(keys []string) map[string][]ValueWeight {
	// For each cluster member increment each parameters count.
	//        map[key]   map[value] count
	counts := map[string]map[string]int{}
	clusterSize := float64(len(keys))
	// First figure out what parameters and values appear in the cluster.
	for _, key := range keys {
		params, err := query.ParseKey(key)
		if err != nil {
			sklog.Errorf("Invalid key found in Cluster: %s", err)
			continue
		}
		for k, v := range params {
			if v == "" {
				continue
			}
			if _, ok := counts[k]; !ok {
				counts[k] = map[string]int{}
			}
			counts[k][v] += 1
		}
	}
	// Now calculate the weights for each parameter value.  The weight of each
	// value is proportional to the number of times it appears on an observation
	// versus all other values for the same parameter.
	ret := map[string][]ValueWeight{}
	for key, count := range counts {
		weights := []ValueWeight{}
		for value, weight := range count {
			weights = append(weights, ValueWeight{
				Value:  value,
				Weight: int(14*float64(weight)/clusterSize) + 12,
			})
		}
		sort.Sort(ValueWeightSortable(weights))
		ret[key] = weights
	}

	return ret
}

// SortableClusterable allows for sorting kmeans.Clusterables.
type SortableClusterable struct {
	Observation kmeans.Clusterable
	Distance    float64
}

type sortableClusterableSlice []*SortableClusterable

func (p sortableClusterableSlice) Len() int           { return len(p) }
func (p sortableClusterableSlice) Less(i, j int) bool { return p[i].Distance < p[j].Distance }
func (p sortableClusterableSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type sortableClusterSummarySlice []*ClusterSummary

func (p sortableClusterSummarySlice) Len() int { return len(p) }
func (p sortableClusterSummarySlice) Less(i, j int) bool {
	return p[i].StepFit.Regression < p[j].StepFit.Regression
}
func (p sortableClusterSummarySlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// getClusterSummaries returns a summary for each cluster.
func getClusterSummaries(observations []kmeans.Clusterable, centroids []kmeans.Centroid, header []*dataframe.ColumnHeader, interesting float32) *ClusterSummaries {
	ret := &ClusterSummaries{
		Clusters: make([]*ClusterSummary, len(centroids)),
	}
	allClusters, _ := kmeans.GetClusters(observations, centroids)

	for i, cluster := range allClusters {
		// cluster is just an array of the observations for a given cluster.
		// Drop the first value which is the centroid.
		cluster = cluster[1:]
		numSampleKeys := len(cluster)
		if numSampleKeys > config.MAX_SAMPLE_TRACES_PER_CLUSTER {
			numSampleKeys = config.MAX_SAMPLE_TRACES_PER_CLUSTER
		}
		stepFit := stepfit.GetStepFitAtMid(centroids[i].(*ctrace2.ClusterableTrace).Values, interesting)
		summary := newClusterSummary()
		summary.ParamSummaries = getParamSummaries(cluster)
		summary.StepFit = stepFit
		summary.StepPoint = header[stepFit.TurningPoint]
		summary.Num = len(cluster)

		// First, sort the traces so they are ordered with the traces closest to
		// the centroid first.
		sc := []*SortableClusterable{}
		for j := 0; j < len(cluster); j++ {
			sc = append(sc, &SortableClusterable{
				Observation: cluster[j],
				Distance:    centroids[i].Distance(cluster[j]),
			})
		}
		sort.Sort(sortableClusterableSlice(sc))

		for _, o := range sc[:numSampleKeys] {
			summary.Keys = append(summary.Keys, o.Observation.(*ctrace2.ClusterableTrace).Key)
		}

		summary.Centroid = centroids[i].(*ctrace2.ClusterableTrace).Values

		ret.Clusters[i] = summary
	}
	sort.Sort(sortableClusterSummarySlice(ret.Clusters))

	return ret
}

type Progress func(totalError float64)

// Please see go/calmbench-trace-tail for how we filter the tail
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

// shortcutFromKeys stores a new shortcut for each cluster based on its Keys.
func shortcutFromKeys(summary *ClusterSummaries) error {
	var err error
	for _, cs := range summary.Clusters {
		if cs.Shortcut, err = shortcut2.InsertShortcut(&shortcut2.Shortcut{Keys: cs.Keys}); err != nil {
			return err
		}
	}
	return nil
}

// CalculateClusterSummaries runs k-means clustering over the trace shapes.
func CalculateClusterSummaries(df *dataframe.DataFrame, k int, stddevThreshold float32, progress Progress, interesting float32, algo ClusterAlgo) (*ClusterSummaries, error) {
	if algo == KMEANS_ALGO {
		// Convert the DataFrame to a slice of kmeans.Clusterable.
		observations := make([]kmeans.Clusterable, 0, len(df.TraceSet))
		for key, trace := range df.TraceSet {
			observations = append(observations, ctrace2.NewFullTrace(key, trace, stddevThreshold))
		}
		if len(observations) == 0 {
			return nil, fmt.Errorf("Zero traces in the DataFrame.")
		}

		// Create K starting centroids.
		centroids := chooseK(observations, k)
		lastTotalError := 0.0
		for i := 0; i < MAX_KMEANS_ITERATIONS; i++ {
			centroids = kmeans.Do(observations, centroids, ctrace2.CalculateCentroid)
			totalError := kmeans.TotalError(observations, centroids)
			if progress != nil {
				progress(totalError)
			}
			if math.Abs(totalError-lastTotalError) < KMEAN_EPSILON {
				break
			}
			lastTotalError = totalError
		}
		clusterSummaries := getClusterSummaries(observations, centroids, df.Header, interesting)
		clusterSummaries.K = k
		clusterSummaries.StdDevThreshold = stddevThreshold
		if err := shortcutFromKeys(clusterSummaries); err != nil {
			return nil, fmt.Errorf("Failed to write shortcut for keys: %s", err)
		}
		return clusterSummaries, nil
	} else if algo == STEPFIT_ALGO || algo == TAIL_ALGO {

		low := newClusterSummary()
		high := newClusterSummary()
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
			if algo == TAIL_ALGO {
				quantile := 1 / float64(interesting)
				slack := float64(k) * 0.01
				const MULTIPLIER = 2 // TODO(liyuqian): Make this configurable

				tail := FilterTail(trace, quantile, MULTIPLIER, slack)
				isLow = tail < 0
				isHigh = tail > 0
				sf.TurningPoint = len(trace) - 1
			}

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
		ret := &ClusterSummaries{
			Clusters:        []*ClusterSummary{},
			K:               k,
			StdDevThreshold: stddevThreshold,
		}
		if low.Num > 0 {
			low.ParamSummaries = getParamSummariesForKeys(low.Keys)
			ret.Clusters = append(ret.Clusters, low)
		}
		if high.Num > 0 {
			high.ParamSummaries = getParamSummariesForKeys(high.Keys)
			ret.Clusters = append(ret.Clusters, high)
		}
		if err := shortcutFromKeys(ret); err != nil {
			return nil, fmt.Errorf("Failed to write shortcut for keys: %s", err)
		}
		return ret, nil
	} else {
		return nil, fmt.Errorf("Unknown clustering algorithm: %s", algo)
	}
}
