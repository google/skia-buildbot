package ctrace2

import (
	"math"
	"testing"

	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/kmeans"
)

const (
	e = vec32.MissingDataSentinel
)

func near(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

func TestDistance(t *testing.T) {
	a := &ClusterableTrace{Values: []float32{3, 0}}
	b := &ClusterableTrace{Values: []float32{0, 4}}
	if got, want := a.Distance(b), 5.0; !near(got, want) {
		t.Errorf("Distance mismatch: Got %f Want %f", got, want)
	}
	if got, want := a.Distance(a), 0.0; !near(got, want) {
		t.Errorf("Distance mismatch: Got %f Want %f", got, want)
	}
}

func TestNewFullTraceKey(t *testing.T) {
	ct := NewFullTrace("foo", []float32{1, -1}, config.MinStdDev)
	if got, want := ct.Key, "foo"; got != want {
		t.Errorf("Key not set: Got %s Want %s", got, want)
	}
}

func TestNewFullTrace(t *testing.T) {
	// All positive (Near=true) testcases should end up with a normalized array
	// of values with 1.0 in the first spot and a standard deviation of 1.0.
	testcases := []struct {
		Values []float32
		Near   bool
	}{
		{
			Values: []float32{1.0, -1.0},
			Near:   true,
		},
		{
			Values: []float32{e, 1.0, -1.0, -1.0},
			Near:   true,
		},
		{
			Values: []float32{e, 1.0, -1.0, e},
			Near:   true,
		},
		{
			Values: []float32{e, 2.0, -2.0, e},
			Near:   true,
		},
		{
			// There's a limit to how small of a stddev we will normalize.
			Values: []float32{e, config.MinStdDev, -config.MinStdDev, e},
			Near:   false,
		},
	}
	for _, tc := range testcases {
		ct := NewFullTrace("foo", tc.Values, config.MinStdDev)
		if got, want := float64(ct.Values[0]), 1.0; near(got, want) != tc.Near {
			t.Errorf("Normalization failed for values %#v: near(Got %f, Want %f) != %t", tc.Values, got, want, tc.Near)
		}
	}
}

func TestCalculateCentroid(t *testing.T) {
	members := []kmeans.Clusterable{
		&ClusterableTrace{Values: []float32{4, 0}},
		&ClusterableTrace{Values: []float32{0, 8}},
	}
	c := CalculateCentroid(members).(*ClusterableTrace)
	if got, want := float64(c.Values[0]), 2.0; !near(got, want) {
		t.Errorf("Failed calculating centroid: !near(Got %f, Want %f)", got, want)
	}
	if got, want := float64(c.Values[1]), 4.0; !near(got, want) {
		t.Errorf("Failed calculating centroid: !near(Got %f, Want %f)", got, want)
	}

}
