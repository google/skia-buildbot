package alerting

import (
	"testing"

	"skia.googlesource.com/buildbot.git/perf/go/types"
)

func newCluster(keys []string, regression float64, hash string) *types.ClusterSummary {
	c := types.NewClusterSummary(len(keys), 0)
	for i, k := range keys {
		c.Keys[i] = k
	}
	c.StepFit.Regression = regression
	c.Hash = hash
	return c
}

func TestCombineClusters(t *testing.T) {
	// Let's say we have three existing clusters with the following trace ids:
	//
	//    C[1], C[2], C[3], C[8,9] C[10,11]
	//
	// And we run clustering and get the following four new clusters:
	//
	//    N[1], N[2], N[3,4], N[5], N[8], N[10, 11]
	//
	// In the end we should end up with the following clusters:
	//
	//  C[1]      from C[1] or N[1]
	//  N[2]      from C[2] or N[2]
	//  N[3, 4]   from C[3] or N[3, 4]
	//  N[5]      from N[5]
	//  C[8]      from N[8]
	//  N[10, 11] from C[10, 11] or N[10, 11]
	//
	// and CombineClusters should return the clusters that need to be written:
	//
	//  N[2]
	//  N[3, 4]
	//  N[5]
	//  N[10, 11]
	//
	// Similarly, if we have clusters with the same hashes and Regression direction
	// then they should be merged:
	//
	// C[20, 21] and N[22] -> N[20, 21, 22]
	//
	// And clusters with the same hashes and opposite Regression direction are
	// treated as new clusters:
	//
	// C[30, 31] and N[33, 34] -> N[33, 34]
	//
	// And finally, clusters with the same traces, same hash, and opposite Regression
	// are ignored.
	//
	// C[40] and N[40] -> nil
	//
	// Given the Regression values for each cluster given below:
	C := []*types.ClusterSummary{
		newCluster([]string{"1"}, 250, "aaa"),
		newCluster([]string{"2"}, 300, "bbb"),
		newCluster([]string{"3"}, 400, "ccc"),
		newCluster([]string{"8", "9"}, 400, "ddd"),
		newCluster([]string{"10", "11"}, 400, "eee"),
		newCluster([]string{"20", "21"}, 700, "match"),
		newCluster([]string{"30", "31"}, 700, "opposite"),
		newCluster([]string{"40"}, 700, "nonsense"),
	}
	N := []*types.ClusterSummary{
		newCluster([]string{"1"}, 200, "fff"),
		newCluster([]string{"2"}, 350, "ggg"),
		newCluster([]string{"3", "4"}, 300, "hhh"),
		newCluster([]string{"5"}, 150, "iii"),
		newCluster([]string{"8"}, 450, "jjj"),
		newCluster([]string{"10", "11"}, 450, "kkk"),
		newCluster([]string{"22"}, 750, "match"),
		newCluster([]string{"33", "34"}, -700, "opposite"),
		newCluster([]string{"40"}, -700, "nonsense"),
	}
	R := CombineClusters(N, C)

	expected := []struct {
		Key        string
		Regression float64
	}{
		{
			Key:        "20",
			Regression: 700,
		},
		{
			Key:        "2",
			Regression: 350,
		},
		{
			Key:        "3",
			Regression: 300,
		},
		{
			Key:        "5",
			Regression: 150,
		},
		{
			Key:        "10",
			Regression: 450,
		},
		{
			Key:        "33",
			Regression: -700,
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
	if got, want := len(R[0].Keys), 3; got != want {
		t.Errorf("Incorrect merge: Got %v Want %v", got, want)
	}
}
