package alerting

import (
	"testing"

	"skia.googlesource.com/buildbot.git/perf/go/types"
)

func newCluster(keys []string, regression float64) *types.ClusterSummary {
	c := types.NewClusterSummary(len(keys), 0)
	for i, k := range keys {
		c.Keys[i] = k
	}
	c.StepFit.Regression = regression
	return c
}

func TestCombineClusters(t *testing.T) {
	// Let's say we have three existing clusters with the following trace ids:
	//
	//    C[1], C[2], C[3,4], C[7]
	//
	// And we run clustering and get the following four new clusters:
	//
	//    N[1], N[3], N[4], N[5], N[7]
	//
	// In the end we should end up with the following clusters:
	//
	//  N[1] from C[1] or N[1]
	//  C[2] from C[2]
	//  N[4] from C[3,4] or N[3] or N[4]
	//  N[5] from N[5]
	//  C[7] from C[7] or N[7]
	//
	// and CombineClusters should return the clusters that need to be written:
	//
	//  N[1]
	//  N[3]
	//  N[4]
	//  N[5]
	//
	// Given the Regression values for each cluster given below:
	//
	// Note that N[4] and C[3,4] is tricky since N[3] should initially replace
	// C[3,4] and then N[4] should not match N[3] and so becomes a new cluster
	// itself. I.e. the C[3,4] cluster gets replaced with two better clusters.

	C := []*types.ClusterSummary{
		newCluster([]string{"1"}, 200),
		newCluster([]string{"2"}, 300),
		newCluster([]string{"3", "4"}, 400),
		newCluster([]string{"7"}, 400),
	}
	N := []*types.ClusterSummary{
		newCluster([]string{"1"}, 250),
		newCluster([]string{"3"}, 450),
		newCluster([]string{"4"}, 500),
		newCluster([]string{"5"}, 150),
		newCluster([]string{"7"}, 300),
	}
	R := CombineClusters(N, C)

	expected := []struct {
		Key        string
		Regression float64
	}{
		{
			Key:        "1",
			Regression: 250,
		},
		{
			Key:        "3",
			Regression: 450,
		},
		{
			Key:        "4",
			Regression: 500,
		},
		{
			Key:        "5",
			Regression: 150,
		},
	}
	if got, want := len(R), len(expected); got != want {
		t.Fatalf("Wrong number of results: Got %v Want %v", got, want)
	}
	for i, r := range R {
		if got, want := r.Keys[0], expected[i].Key; got != want {
			t.Errorf("Wrong ID: Got %v Want %v", got, want)
		}
		if got, want := r.StepFit.Regression, expected[i].Regression; got != want {
			t.Errorf("Regression not copied over: Got %v Want %v", got, want)
		}
	}
}
