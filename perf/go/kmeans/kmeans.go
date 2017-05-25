// kmeans implements a generic k-means clustering algorithm.
//
// To use this code create types that implements Clusterable, Centroid, and
// also a function that implements CalculateCentroid. In many cases the same
// type can be used as both a Clusterable and a Centroid.
//
// See the unit tests for examples.
//
package kmeans

import "math"

// Clusterable defines the interface that an object must support to do k-means
// clustering on it.
type Clusterable interface{}

// Centroid is the interface that Centroids must support to do k-means clustering.
type Centroid interface {
	// AsClusterable converts this Centroid to a Clusterable, or returns nil if
	// the conversion isn't possible.
	AsClusterable() Clusterable

	// Distance returns the distance from the given Clusterable to this Centroid.
	Distance(c Clusterable) float64
}

// CalculateCentroid calculates a new centroid from a list of Clusterables.
type CalculateCentroid func([]Clusterable) Centroid

// closestCentroid returns the index of the closest centroid to this observation.
func closestCentroid(observation Clusterable, centroids []Centroid) (int, float64) {
	var bestDistance float64 = math.MaxFloat64
	bestIndex := -1
	for j, c := range centroids {
		if dist := c.Distance(observation); dist < bestDistance {
			bestDistance = dist
			bestIndex = j
		}
	}
	return bestIndex, bestDistance
}

// Do does a single iteration of Loyd's Algorithm, taking an array of
// observations and a set of centroids along with a function to calcaulate new
// centroids for a cluster.  It returns an updated array of centroids. Note
// that the centroids array passed in gets modified so the best way to call the
// function is:
//
//  centroids = Do(observations, centroids, f)
//
func Do(observations []Clusterable, centroids []Centroid, f CalculateCentroid) []Centroid {
	k := len(centroids)

	// cluster is which cluster each observation is currently in.
	cluster := make([]int, len(observations))

	// Find the closest centroid for each observation.
	for i, o := range observations {
		cluster[i], _ = closestCentroid(o, centroids)
	}

	newCentroids := make([]Centroid, 0, len(centroids))
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

// GetClusters returns the observations categorized into the clusters they fit
// into. The return value is sorted by the number of members of the cluster.
// The very first element of each cluster is the centroid, the remainging
// members are the observations that are in the cluster.
func GetClusters(observations []Clusterable, centroids []Centroid) ([][]Clusterable, float64) {
	r := make([][]Clusterable, len(centroids))
	for i := range r {
		// The first trace is always the centroid for the cluster.
		cl := centroids[i].AsClusterable()
		if cl != nil {
			r[i] = []Clusterable{cl}
		} else {
			r[i] = []Clusterable{}
		}
	}
	totalError := 0.0
	for _, o := range observations {
		index, clusterError := closestCentroid(o, centroids)
		totalError += clusterError
		r[index] = append(r[index], o)
	}
	return r, totalError
}

// KMeans runs the k-means clustering algorithm over a set of observations and
// returns the centroids and clusters.
//
// TODO(jcgregorio) Should just iterate until total error stops changing.
func KMeans(observations []Clusterable, centroids []Centroid, k, iters int, f CalculateCentroid) ([]Centroid, [][]Clusterable) {
	for i := 0; i < iters; i++ {
		centroids = Do(observations, centroids, f)
	}
	clusters, _ := GetClusters(observations, centroids)
	return centroids, clusters
}

// TotalError calculates the total error between the centroids and the
// observations.
func TotalError(observations []Clusterable, centroids []Centroid) float64 {
	_, totalError := GetClusters(observations, centroids)

	return totalError
}
