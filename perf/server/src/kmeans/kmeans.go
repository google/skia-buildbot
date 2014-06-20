// kmeans implements a generic k-means clustering algorithm.
//
// To use this code create a type that implements Clusterable and also
// a function that implements CalculateCentroid.
//
// See the unit tests for examples.
//
package kmeans

import (
	"math"
	"math/rand"
	"sort"
)

// Clusterable defines the interface that an object must support to do k-means
// clustering on it.
type Clusterable interface {
	Distance(other Clusterable) float64
}

// CalculateCentroid calculates a new centroid from a list of Clusterables.
type CalculateCentroid func([]Clusterable) Clusterable

// closestCentroid returns the index of the closest centroid to this observation.
func closestCentroid(observation Clusterable, centroids []Clusterable) int {
	var bestDistance float64 = math.MaxFloat64
	bestIndex := -1
	for j, c := range centroids {
		if dist := observation.Distance(c); dist < bestDistance {
			bestDistance = dist
			bestIndex = j
		}
	}
	return bestIndex
}

// Do does a single iteration of Loyd's Algorithm, taking an array of observations and
// a set of centroids along with a function to calcaulate new centroids for a cluster.
// It returns an updated array of centroids. Note that the centroids array passed in gets modified
// so the best way to call the function is:
//
//  centroids = Do(observations, centroids, f)
//
func Do(observations, centroids []Clusterable, f CalculateCentroid) []Clusterable {
	k := len(centroids)

	// cluster is which cluster each observation is currently in.
	cluster := make([]int, len(observations))

	// Find the closest centroid for each observation.
	for i, o := range observations {
		cluster[i] = closestCentroid(o, centroids)
	}

	newCentroids := make([]Clusterable, 0, len(centroids))
	// Calculate new centroids based on each the new cluster members.
	for i := 0; i < k; i++ {
		c := make([]Clusterable, 0)
		for j, o := range observations {
			if cluster[j] == i {
				c = append(c, o)
			}
		}
		if len(c) != 0 {
			newCentroids = append(newCentroids, f(c))
		}
	}
	return newCentroids
}

// SortableClusterSlice is a utility type for sorting.
type SortableClusterSlice [][]Clusterable

func (p SortableClusterSlice) Len() int           { return len(p) }
func (p SortableClusterSlice) Less(i, j int) bool { return len(p[i]) > len(p[j]) } // Sort from largest to smallest.
func (p SortableClusterSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// GetClusters returns the observations categorized into the clusters they fit
// into.  The return value is sorted by the number of members of the cluster.
func GetClusters(observations, centroids []Clusterable) [][]Clusterable {
	r := make([][]Clusterable, len(centroids), len(centroids))
	for i, _ := range r {
		r[i] = make([]Clusterable, 0)
	}
	for _, o := range observations {
		index := closestCentroid(o, centroids)
		r[index] = append(r[index], o)
	}
	sort.Sort(SortableClusterSlice(r))
	return r
}

// ChooseK will choose a starting set of centroids at random from a set of observations.
//
// TODO(jcgregorio) Choosing w/o replacement would be better.
func ChooseK(observations []Clusterable, k int) []Clusterable {
	popN := len(observations)
	centroids := make([]Clusterable, k, k)
	for i := 0; i < k; i++ {
		centroids[i] = observations[rand.Intn(popN)]
	}
	return centroids
}

// KMeans runs the k-means clustering algorithm over a set of observations and
// returns the centroids and clusters.
func KMeans(observations []Clusterable, k, iters int, f CalculateCentroid) ([]Clusterable, [][]Clusterable) {
	centroids := ChooseK(observations, k)
	for i := 0; i < iters; i++ {
		centroids = Do(observations, centroids, f)
	}
	return centroids, GetClusters(observations, centroids)
}
