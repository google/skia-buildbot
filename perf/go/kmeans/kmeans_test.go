package kmeans

import (
	"math"
	"sort"
	"testing"
)

// myObservation implements Clusterable and Centroid.
type myObservation struct {
	x float64
	y float64
}

func (m myObservation) Distance(c Clusterable) float64 {
	o := c.(myObservation)
	return math.Sqrt((m.x-o.x)*(m.x-o.x) + (m.y-o.y)*(m.y-o.y))
}

func (m myObservation) AsClusterable() Clusterable {
	return m
}

// calculateCentroid implements CalculateCentroid.
func calculateCentroid(members []Clusterable) Centroid {
	var sumX = 0.0
	var sumY = 0.0
	length := float64(len(members))

	for _, m := range members {
		sumX += m.(myObservation).x
		sumY += m.(myObservation).y
	}
	return myObservation{x: sumX / length, y: sumY / length}
}

func near(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

func almostEqual(t *testing.T, a, b Clusterable) {
	if got, want := a.(myObservation).x, b.(myObservation).x; !near(got, want) {
		t.Errorf("Not near enough on the x: Got %f Want %f", got, want)
	}
	if got, want := a.(myObservation).y, b.(myObservation).y; !near(got, want) {
		t.Errorf("Not near enough on the x: Got %f Want %f", got, want)
	}
}

func TestBasicIteration(t *testing.T) {
	observations := []Clusterable{
		myObservation{0.0, 0.0},
		myObservation{3.0, 0.0},
		myObservation{3.0, 1.0},
	}
	centroids := []Centroid{
		myObservation{0.0, 0.0},
		myObservation{3.0, 0.0},
	}
	centroids = Do(observations, centroids, calculateCentroid)
	almostEqual(t, centroids[0], myObservation{0.0, 0.0})
	almostEqual(t, centroids[1], myObservation{3.0, 0.5})
}

func TestEmptyCentroids(t *testing.T) {
	observations := []Clusterable{
		myObservation{0.0, 0.0},
		myObservation{3.0, 0.0},
		myObservation{3.0, 1.0},
	}
	centroids := []Centroid{}
	centroids = Do(observations, centroids, calculateCentroid)
	if got, want := len(centroids), 0; got != want {
		t.Errorf("Wrong length of centroids returned: Got %d, Want %d", got, want)
	}
}

func TestEmptyEverything(t *testing.T) {
	observations := []Clusterable{}
	centroids := []Centroid{}
	centroids = Do(observations, centroids, calculateCentroid)
	if got, want := len(centroids), 0; got != want {
		t.Errorf("Wrong length of centroids returned: Got %d, Want %d", got, want)
	}
}

func TestLosingCentroids(t *testing.T) {
	observations := []Clusterable{
		myObservation{0.0, 0.0},
		myObservation{3.0, 0.0},
	}
	centroids := []Centroid{
		myObservation{0.0, 0.0},
		myObservation{3.0, 0.0},
		myObservation{3.0, 1.0},
	}

	centroids = Do(observations, centroids, calculateCentroid)
	if got, want := len(centroids), 2; got != want {
		t.Errorf("Wrong length of centroids returned: Got %d, Want %d", got, want)
	}
}

// SortableClusterSlice is a utility type for sorting.
type SortableClusterSlice [][]Clusterable

func (p SortableClusterSlice) Len() int           { return len(p) }
func (p SortableClusterSlice) Less(i, j int) bool { return len(p[i]) > len(p[j]) } // Sort from largest to smallest.
func (p SortableClusterSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func TestFullKmeans(t *testing.T) {
	observations := []Clusterable{
		myObservation{0.0, 0.0},
		myObservation{3.0, 0.0},
		myObservation{3.0, 0.5},
		myObservation{6.0, 6.0},
		myObservation{6.0, 6.1},
		myObservation{6.0, 6.2},
	}
	centroids := []Centroid{
		myObservation{0.0, 0.0},
		myObservation{3.0, 0.0},
		myObservation{6.0, 6.0},
	}
	centroids = Do(observations, centroids, calculateCentroid)
	centroids = Do(observations, centroids, calculateCentroid)
	centroids = Do(observations, centroids, calculateCentroid)
	clusters, _ := GetClusters(observations, centroids)
	sort.Sort(SortableClusterSlice(clusters))
	if got, want := len(centroids), 3; got != want {
		t.Errorf("Wrong length of centroids: Got %d, Want %d", got, want)
	}
	if got, want := len(clusters[0]), 3+1; got != want {
		t.Errorf("Wrong length of clusters[0]: Got %d, Want %d", got, want)
	}
	if got, want := len(clusters[1]), 2+1; got != want {
		t.Errorf("Wrong length of clusters[1]: Got %d, Want %d", got, want)
	}
	if got, want := len(clusters[2]), 1+1; got != want {
		t.Errorf("Wrong length of clusters[2]: Got %d, Want %d", got, want)
	}
}
