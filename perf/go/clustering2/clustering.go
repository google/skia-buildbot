package clustering2

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ctrace2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/kmeans"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
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
	ParamSummaries []ValuePercent `json:"param_summaries2"`

	// StepFit is info on the fit of the centroid to a step function.
	StepFit *stepfit.StepFit `json:"step_fit"`

	// StepPoint is the ColumnHeader for the step point.
	StepPoint *dataframe.ColumnHeader `json:"step_point"`

	// Num is the number of observations that are in this cluster.
	Num int `json:"num"`

	// Timestamp is the timestamp when this regression was found.
	Timestamp time.Time `json:"ts"`
}

// NewClusterSummary returns a new ClusterSummary.
func NewClusterSummary(ctx context.Context) *ClusterSummary {
	return &ClusterSummary{
		Keys:           []string{},
		ParamSummaries: []ValuePercent{},
		StepFit:        &stepfit.StepFit{},
		StepPoint:      &dataframe.ColumnHeader{},
		Timestamp:      now.Now(ctx),
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

// getParamSummaries summarizes all the parameters for all observations in a
// cluster.
//
// The return value is an array of []ValueWeight's, one []ValueWeight per
// parameter. The members of each []ValueWeight are sorted by the Weight, with
// higher Weight's first.
func getParamSummaries(cluster []kmeans.Clusterable) []ValuePercent {
	keys := make([]string, 0, len(cluster))
	for _, o := range cluster {
		key := o.(*ctrace2.ClusterableTrace).Key
		if key == ctrace2.CENTROID_KEY {
			continue
		}
		keys = append(keys, key)
	}
	return GetParamSummariesForKeys(keys)
}

// GetParamSummariesForKeys summarizes all the parameters for all observations in a
// cluster.
//
// The return value is an array of []ValueWeight's, one []ValueWeight per
// parameter. The members of each []ValueWeight are sorted by the Weight, with
// higher Weight's first.
func GetParamSummariesForKeys(keys []string) []ValuePercent {
	// For each cluster member increment each parameters count.
	//        map[key]   map[value] count
	counts := map[string]int{}
	clusterSize := len(keys)
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
			counts[k+"="+v] += 1
		}
	}
	ret := []ValuePercent{}
	for key, count := range counts {
		ret = append(ret, ValuePercent{
			Value:   key,
			Percent: (100 * count) / clusterSize,
		})
	}
	SortValuePercentSlice(ret)
	return ret
}

// sortableClusterable allows for sorting kmeans.Clusterables.
type sortableClusterable struct {
	Observation kmeans.Clusterable
	Distance    float64
}

type sortableClusterableSlice []*sortableClusterable

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
func getClusterSummaries(ctx context.Context, observations []kmeans.Clusterable, centroids []kmeans.Centroid, header []*dataframe.ColumnHeader, interesting float32, stepDetection types.StepDetection, stddevThreshhold float32) *ClusterSummaries {
	ret := &ClusterSummaries{
		Clusters: make([]*ClusterSummary, len(centroids)),
	}
	allClusters, _ := kmeans.GetClusters(observations, centroids)

	for i, cluster := range allClusters {
		// cluster is just an array of the observations for a given cluster.
		// Drop the first value which is the centroid.
		cluster = cluster[1:]
		numSampleKeys := len(cluster)
		if numSampleKeys > config.MaxSampleTracesPerCluster {
			numSampleKeys = config.MaxSampleTracesPerCluster
		}
		stepFit := stepfit.GetStepFitAtMid(centroids[i].(*ctrace2.ClusterableTrace).Values, stddevThreshhold, interesting, stepDetection)
		summary := NewClusterSummary(ctx)
		summary.ParamSummaries = getParamSummaries(cluster)
		summary.StepFit = stepFit
		summary.StepPoint = header[stepFit.TurningPoint]
		summary.Num = len(cluster)

		// First, sort the traces so they are ordered with the traces closest to
		// the centroid first.
		sc := []*sortableClusterable{}
		for j := 0; j < len(cluster); j++ {
			sc = append(sc, &sortableClusterable{
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

// Progress is a function that is called periodically with the progress being
// made in clustering.
type Progress func(totalError float64)

// CalculateClusterSummaries runs k-means clustering over the trace shapes.
func CalculateClusterSummaries(ctx context.Context, df *dataframe.DataFrame, k int, stddevThreshold float32, progress Progress, interesting float32, stepDetection types.StepDetection) (*ClusterSummaries, error) {
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
	clusterSummaries := getClusterSummaries(ctx, observations, centroids, df.Header, interesting, stepDetection, stddevThreshold)
	clusterSummaries.K = k
	clusterSummaries.StdDevThreshold = stddevThreshold
	return clusterSummaries, nil
}
