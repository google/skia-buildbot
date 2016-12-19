package clustering2

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ctrace2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/kmeans"
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
	INTERESTING_THRESHHOLD = 50.0

	LOW           = "Low"
	HIGH          = "High"
	UNINTERESTING = "Uninteresting"
)

// ValueWeight is a weight proportional to the number of times the parameter
// Value appears in a cluster. Used in ClusterSummary.
type ValueWeight struct {
	Value  string `json:"value"`
	Weight int    `json:"weight"`
}

// StepFit stores information on the best Step Function fit on a trace.
//
// Used in ClusterSummary.
type StepFit struct {
	// LeastSquares is the Least Squares error for a step function curve fit to the trace.
	LeastSquares float32 `json:"least_squares"`

	// TurningPoint is the index where the Step Function changes value.
	TurningPoint int `json:"turning_point"`

	// StepSize is the size of the step in the step function. Negative values
	// indicate a step up, i.e. they look like a performance regression in the
	// trace, as opposed to positive values which look like performance
	// improvements.
	StepSize float32 `json:"step_size"`

	// The "Regression" value is calculated as Step Size / Least Squares Error.
	//
	// The better the fit the larger the number returned, because LSE
	// gets smaller with a better fit. The higher the Step Size the
	// larger the number returned.
	Regression float32 `json:"regression"`

	// Status of the cluster.
	//
	// Values can be "High", "Low", and "Uninteresting"
	Status string `json:"status"`
}

// ClusterSummary is a summary of a single cluster of traces.
type ClusterSummary struct {
	// Centroid is the calculated centroid of the cluster.
	Centroid []float32 `json:"centroid"`

	// Keys of all the members of the Cluster.
	//
	// The keys are sorted so that the ones at the beginning of the list are
	// closest to the centroid.
	Keys []string `json:"keys"`

	// ParamSummaries is a summary of all the parameters in the cluster.
	ParamSummaries map[string][]ValueWeight `json:"param_summaries"`

	// StepFit is info on the fit of the centroid to a step function.
	StepFit *StepFit `json:"step_fit"`

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
		StepFit:        &StepFit{},
		StepPoint:      &dataframe.ColumnHeader{},
	}
}

// ClusterSummaries is one summary for each cluster that the k-means clustering
// found.
type ClusterSummaries struct {
	Clusters         []*ClusterSummary
	StdDevThreshhold float32
	K                int
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
	// For each cluster member increment each parameters count.
	//        map[key]   map[value] count
	counts := map[string]map[string]int{}
	clusterSize := float64(len(cluster))
	// First figure out what parameters and values appear in the cluster.
	for _, o := range cluster {
		key := o.(*ctrace2.ClusterableTrace).Key
		if key == ctrace2.CENTROID_KEY {
			continue
		}
		params, err := query.ParseKey(key)
		if err != nil {
			glog.Errorf("Invalid key found in Cluster: %s", err)
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

// getStepFit takes one []float32 trace and calculates and returns a StepFit.
//
// See StepFit for a description of the values being calculated.
func getStepFit(trace []float32) *StepFit {
	lse := float32(math.MaxFloat32)
	stepSize := float32(-1.0)
	turn := 0

	for i := 1; i < len(trace); i++ {
		y0 := vec32.Mean(trace[:i])
		y1 := vec32.Mean(trace[i:])
		if y0 == y1 {
			continue
		}
		d := vec32.SSE(trace[:i], y0) + vec32.SSE(trace[i:], y1)
		if d < lse {
			lse = d
			stepSize = (y0 - y1)
			turn = i
		}
	}
	lse = float32(math.Sqrt(float64(lse))) / float32(len(trace))
	regression := stepSize / lse
	status := UNINTERESTING
	if regression > INTERESTING_THRESHHOLD {
		status = LOW
	} else if regression < -INTERESTING_THRESHHOLD {
		status = HIGH
	}
	return &StepFit{
		LeastSquares: lse,
		StepSize:     stepSize,
		TurningPoint: turn,
		Regression:   regression,
		Status:       status,
	}
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
func getClusterSummaries(observations []kmeans.Clusterable, centroids []kmeans.Centroid, header []*dataframe.ColumnHeader) *ClusterSummaries {
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
		stepFit := getStepFit(centroids[i].(*ctrace2.ClusterableTrace).Values)
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

// CalculateClusterSummaries runs k-means clustering over the trace shapes.
func CalculateClusterSummaries(df *dataframe.DataFrame, k int, stddevThreshhold float32, progress Progress) (*ClusterSummaries, error) {
	// Convert the DataFrame to a slice of kmeans.Clusterable.
	observations := make([]kmeans.Clusterable, 0, len(df.TraceSet))
	for key, trace := range df.TraceSet {
		observations = append(observations, ctrace2.NewFullTrace(key, trace, stddevThreshhold))
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
		glog.Infof("Total Error: %f\n", totalError)
		if progress != nil {
			progress(totalError)
		}
		if math.Abs(totalError-lastTotalError) < KMEAN_EPSILON {
			break
		}
		lastTotalError = totalError
	}
	clusterSummaries := getClusterSummaries(observations, centroids, df.Header)
	clusterSummaries.K = k
	clusterSummaries.StdDevThreshhold = stddevThreshhold
	return clusterSummaries, nil
}
