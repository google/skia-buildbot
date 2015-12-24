package kmlabel

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/kmeans"
)

func TestMeasure(t *testing.T) {
	m := NewMeasure(2)
	m.Inc(0)
	m.Inc(1)
	m.Calc()
	assert.Equal(t, math.Sqrt(2.0)/2, m.Distance(0))
	assert.Equal(t, math.Sqrt(2.0)/2, m.Distance(1))

	m.Inc(1)
	m.Calc()
	assert.Equal(t, math.Sqrt(4.0+4.0)/3, m.Distance(0))
	assert.Equal(t, math.Sqrt(1.0+1.0)/3, m.Distance(1))
}

func TestCentroid(t *testing.T) {
	c := NewCentroid([]int{2, 3}, []int{1, 1})
	tr := &Trace{
		ID:     "foo",
		Params: []int{1, 1},
	}
	assert.Equal(t, 0.0, c.Distance(tr))
	c.Clear()
	c.Add(tr)
	c.Finished()
	assert.Equal(t, 0.0, c.Distance(tr))
	tr2 := &Trace{
		ID:     "foo",
		Params: []int{1, 2},
	}
	c.Add(tr2)
	c.Finished()
	assert.Equal(t, math.Sqrt(2)/2, c.Distance(tr))

	// Now test with a centroid with no initial value.
	c = NewCentroid([]int{3, 4, 5}, nil)
	tr = &Trace{
		ID:     "foo",
		Params: []int{0, 1, 4},
	}
	c.Add(tr)
	c.Finished()
	assert.Equal(t, 0.0, c.Distance(tr))
}

func TestCentroidsAndTraces(t *testing.T) {
	paramset := map[string][]string{
		"config": []string{"8888", "565", "gpu"},
		"cpu":    []string{"intel", "arm"},
	}
	// A set of observations that should make two nice clusters, with [tr1, tr2,
	// tr3] being one cluster and [tr4] being the second cluster.
	traceparams := map[string]map[string]string{
		"tr1": map[string]string{
			"config": "8888",
			"cpu":    "arm",
		},
		"tr2": map[string]string{
			"config": "565",
			"cpu":    "arm",
		},
		"tr3": map[string]string{
			"config": "565",
			"cpu":    "intel",
		},
		"tr4": map[string]string{
			"config": "gpu",
		},
	}
	centroids, traces, f, reverse := CentroidsAndTraces(paramset, traceparams, 2)
	assert.Equal(t, 2, len(centroids))
	assert.Equal(t, 4, len(traces))
	var tr4 *Trace
	for _, tr4 = range traces {
		if tr4.ID == "tr4" {
			break
		}
	}
	assert.Equal(t, 0, tr4.Params[1], "Missing param should map to 0.")
	assert.Equal(t, 3, tr4.Params[0], "The value gpu should sort to last of the three params.")
	assert.Equal(t, traceparams["tr4"], reverse(tr4))
	kmeansCentroid := f([]kmeans.Clusterable{tr4, tr4})
	centroid := kmeansCentroid.(*Centroid)
	assert.Equal(t, 0.0, centroid.Distance(tr4))
	var tr1 *Trace
	for _, tr1 = range traces {
		if tr1.ID == "tr1" {
			break
		}
	}
	assert.Equal(t, 2.0, centroid.Distance(tr1))

	// Now run the k-means algorithm, which is deterministic for a fixed set of
	// starting centroids, so pick our centroids explicitly so we always get the
	// same answer.
	obs := make([]kmeans.Clusterable, len(traces))
	for i, tr := range traces {
		obs[i] = tr
	}
	cent := []kmeans.Centroid{
		f([]kmeans.Clusterable{tr1}),
		f([]kmeans.Clusterable{tr4}),
	}
	kmCentroids, kmClusters := kmeans.KMeans(obs, cent, 2, 10, f)
	assert.Equal(t, 2, len(kmCentroids))
	assert.Equal(t, 2, len(kmClusters))
	assert.Equal(t, 3, len(kmClusters[0]))
	assert.Equal(t, 1, len(kmClusters[1]))
	assert.Equal(t, "tr4", kmClusters[1][0].(*Trace).ID, "tr4 should be the singe member of the second cluster.")
	assert.InDelta(t, 2.7748, kmeans.TotalError(obs, kmCentroids), 0.01)

	// Run k-means again but with just one centroid and show the total error gets
	// larger.
	kmCentroids, kmClusters = kmeans.KMeans(obs, cent[:1], 2, 10, f)
	assert.Equal(t, 1, len(kmCentroids))
	assert.Equal(t, 1, len(kmClusters))
	assert.Equal(t, 4, len(kmClusters[0]))
	assert.InDelta(t, 4.42496, kmeans.TotalError(obs, kmCentroids), 0.01)
}
