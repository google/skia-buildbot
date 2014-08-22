package clustering

import (
	"fmt"
	"math"
	"math/rand"

	"sort"
)

import "github.com/golang/glog"

import (
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/ctrace"
	"skia.googlesource.com/buildbot.git/perf/go/kmeans"
	"skia.googlesource.com/buildbot.git/perf/go/types"
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

	// INTERESTING_THRESHHOLD is the threshhold value beyond which
	// StepFit.Regression values become interesting, i.e. they may indicate real
	// regressions or improvements.
	INTERESTING_THRESHHOLD = 150.0
)

// ClusterSummaries is one summary for each cluster that the k-means clustering
// found.
type ClusterSummaries struct {
	Clusters         []*types.ClusterSummary
	StdDevThreshhold float64
	K                int
}

func NewClusterSummaries() *ClusterSummaries {
	return &ClusterSummaries{
		Clusters:         []*types.ClusterSummary{},
		StdDevThreshhold: ctrace.MIN_STDDEV,
		K:                K,
	}
}

// chooseK chooses a random sample of k observations. Used as the starting
// point for the k-means clustering.
func chooseK(observations []kmeans.Clusterable, k int) []kmeans.Clusterable {
	popN := len(observations)
	centroids := make([]kmeans.Clusterable, k)
	for i := 0; i < k; i++ {
		o := observations[rand.Intn(popN)].(*ctrace.ClusterableTrace)
		cp := &ctrace.ClusterableTrace{
			Key:    "I'm a centroid",
			Values: make([]float64, len(o.Values)),
		}
		copy(cp.Values, o.Values)
		centroids[i] = cp
	}
	return centroids
}

// traceToFlot converts the data into a format acceptable to the Flot plotting
// library.
//
// Flot expects data formatted as an array of [x, y] pairs.
func traceToFlot(t *ctrace.ClusterableTrace) [][]float64 {
	ret := make([][]float64, len(t.Values))
	for i, x := range t.Values {
		ret[i] = []float64{float64(i), x}
	}
	return ret
}

// ValueWeightSortable is a utility class for sorting the types.ValueWeight's by Weight.
type ValueWeightSortable []types.ValueWeight

func (p ValueWeightSortable) Len() int           { return len(p) }
func (p ValueWeightSortable) Less(i, j int) bool { return p[i].Weight > p[j].Weight } // Descending.
func (p ValueWeightSortable) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// getParamSummaries summaries all the parameters for all observations in a cluster.
//
// The return value is an array of []types.ValueWeight's, one []types.ValueWeight per
// parameter. The members of each []types.ValueWeight are sorted by the Weight, with
// higher Weight's first.
func getParamSummaries(cluster []kmeans.Clusterable) [][]types.ValueWeight {
	// For each cluster member increment each parameters count.
	type ValueMap map[string]int
	counts := map[string]ValueMap{}
	clusterSize := float64(len(cluster))
	// First figure out what parameters and values appear in the cluster.
	for _, o := range cluster {
		for k, v := range o.(*ctrace.ClusterableTrace).Params {
			if v == "" {
				continue
			}
			if _, ok := counts[k]; !ok {
				counts[k] = ValueMap{}
				counts[k][v] = 0
			}
			counts[k][v] += 1
		}
	}
	// Now calculate the weights for each parameter value.  The weight of each
	// value is proportional to the number of times it appears on an observation
	// versus all other values for the same parameter.
	ret := make([][]types.ValueWeight, 0)
	for _, count := range counts {
		weights := []types.ValueWeight{}
		for value, weight := range count {
			weights = append(weights, types.ValueWeight{
				Value:  value,
				Weight: int(14*float64(weight)/clusterSize) + 12,
			})
		}
		sort.Sort(ValueWeightSortable(weights))
		ret = append(ret, weights)
	}

	return ret
}

// average calculates and returns the average value of the given []float64.
func average(xs []float64) float64 {
	total := 0.0
	for _, v := range xs {
		total += v
	}
	return total / float64(len(xs))
}

// sse calculates and returns the sum squared error from the given base of []float64.
func sse(xs []float64, base float64) float64 {
	total := 0.0
	for _, v := range xs {
		total += math.Pow(v-base, 2)
	}
	return total
}

// getStepFit takes one []float64 trace and calculates and returns a types.StepFit.
//
// See types.StepFit for a description of the values being calculated.
func getStepFit(trace []float64) *types.StepFit {
	lse := math.MaxFloat64
	stepSize := -1.0
	turn := 0
	for i, _ := range trace {
		if i == 0 {
			continue
		}
		y0 := average(trace[:i])
		y1 := average(trace[i:])
		if y0 == y1 {
			continue
		}
		d := math.Sqrt(sse(trace[:i], y0)+sse(trace[i:], y1)) / float64(len(trace))
		if d < lse {
			lse = d
			stepSize = (y0 - y1)
			turn = i
		}
	}
	regression := stepSize / lse
	status := "Uninteresting"
	if regression > INTERESTING_THRESHHOLD {
		status = "High"
	} else if regression < -INTERESTING_THRESHHOLD {
		status = "Low"
	}
	return &types.StepFit{
		LeastSquares: lse,
		StepSize:     stepSize,
		TurningPoint: turn,
		Regression:   regression,
		Status:       status,
	}
}

type SortableClusterable struct {
	Cluster  kmeans.Clusterable
	Distance float64
}

type SortableClusterableSlice []*SortableClusterable

func (p SortableClusterableSlice) Len() int           { return len(p) }
func (p SortableClusterableSlice) Less(i, j int) bool { return p[i].Distance < p[j].Distance }
func (p SortableClusterableSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type SortableClusterSummarySlice []*types.ClusterSummary

func (p SortableClusterSummarySlice) Len() int { return len(p) }
func (p SortableClusterSummarySlice) Less(i, j int) bool {
	return p[i].StepFit.Regression < p[j].StepFit.Regression
}
func (p SortableClusterSummarySlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// GetClusterSummaries returns a summaries for each cluster.
func GetClusterSummaries(observations, centroids []kmeans.Clusterable, commits []*types.Commit) *ClusterSummaries {
	ret := &ClusterSummaries{
		Clusters: make([]*types.ClusterSummary, len(centroids)),
	}
	allClusters, _ := kmeans.GetClusters(observations, centroids)

	for i, cluster := range allClusters {
		// cluster is just an array of the observations for a given cluster.
		numSampleTraces := len(cluster)
		if numSampleTraces > config.MAX_SAMPLE_TRACES_PER_CLUSTER {
			numSampleTraces = config.MAX_SAMPLE_TRACES_PER_CLUSTER
		}
		stepFit := getStepFit(cluster[0].(*ctrace.ClusterableTrace).Values)
		summary := types.NewClusterSummary(len(cluster)-1, numSampleTraces)
		summary.ParamSummaries = getParamSummaries(cluster)
		summary.StepFit = stepFit
		summary.Hash = commits[stepFit.TurningPoint].Hash
		summary.Timestamp = commits[stepFit.TurningPoint].CommitTime

		for j, o := range cluster {
			if j != 0 {
				summary.Keys[j-1] = o.(*ctrace.ClusterableTrace).Key
			}
		}
		// First, sort the traces so they are order with the traces closest to the
		// centroid first.
		sc := []*SortableClusterable{}
		for j := 0; j < len(cluster); j++ {
			sc = append(sc, &SortableClusterable{Cluster: cluster[j], Distance: cluster[j].Distance(cluster[0])})
		}
		// Sort, but leave the centroid, the 0th element, unmoved.
		sort.Sort(SortableClusterableSlice(sc[1:]))

		for j, clusterable := range sc {
			if j == numSampleTraces {
				break
			}
			summary.Traces[j] = traceToFlot(clusterable.Cluster.(*ctrace.ClusterableTrace))
		}

		ret.Clusters[i] = summary
	}
	sort.Sort(SortableClusterSummarySlice(ret.Clusters))

	return ret
}

// Filter returns true if a trace should be included in clustering.
type Filter func(tr *types.Trace) bool

// CalculateClusterSummaries runs k-means clustering over the trace shapes.
func CalculateClusterSummaries(tile *types.Tile, k int, stddevThreshhold float64, filter Filter) (*ClusterSummaries, error) {
	lastCommitIndex := 0
	for i, c := range tile.Commits {
		if c.CommitTime != 0 {
			lastCommitIndex = i
		}
	}
	observations := make([]kmeans.Clusterable, 0, len(tile.Traces))
	for key, trace := range tile.Traces {
		if filter(trace) {
			observations = append(observations, ctrace.NewFullTrace(string(key), trace.Values[:lastCommitIndex], trace.Params, stddevThreshhold))
		}
	}
	if len(observations) == 0 {
		return nil, fmt.Errorf("Zero traces matched.")
	}

	// Create K starting centroids.
	centroids := chooseK(observations, k)
	// TODO(jcgregorio) Keep iterating until the total error stops changing.
	lastTotalError := 0.0
	for i := 0; i < MAX_KMEANS_ITERATIONS; i++ {
		centroids = kmeans.Do(observations, centroids, ctrace.CalculateCentroid)
		totalError := kmeans.TotalError(observations, centroids)
		glog.Infof("Total Error: %f\n", totalError)
		if math.Abs(totalError-lastTotalError) < KMEAN_EPSILON {
			break
		}
		lastTotalError = totalError
	}
	clusterSummaries := GetClusterSummaries(observations, centroids, tile.Commits)
	clusterSummaries.K = k
	clusterSummaries.StdDevThreshhold = stddevThreshhold
	return clusterSummaries, nil
}
