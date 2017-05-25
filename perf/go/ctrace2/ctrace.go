// ctrace makes Traces into ClusterableTraces which can then be used in kmeans.
package ctrace2

import (
	"fmt"
	"math"

	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/kmeans"
)

const (
	// CENTROID_KEY is the name for the centroid when it appears as a trace in a DataFrame.
	CENTROID_KEY = "special_centroid"
)

// ClusterableTrace contains Trace data and implements kmeans.Clusterable and kmeans.Centroid.
type ClusterableTrace struct {
	Key    string
	Values []float32
}

// See kmeans.Centroid.
func (t *ClusterableTrace) Distance(c kmeans.Clusterable) float64 {
	// Data always has the same length, and NewFullTrace keeps that guarantee.
	o := c.(*ClusterableTrace)
	sum := float32(0.0)
	for i, x := range t.Values {
		sum += (x - o.Values[i]) * (x - o.Values[i])
	}
	return math.Sqrt(float64(sum))
}

// See kmeans.Centroid.
func (t *ClusterableTrace) AsClusterable() kmeans.Clusterable {
	return t
}

func (t *ClusterableTrace) String() string {
	return fmt.Sprintf("%s %#v", t.Key, t.Values[:2])
}

func (t *ClusterableTrace) Dup(newKey string) *ClusterableTrace {
	cp := &ClusterableTrace{
		Key:    newKey,
		Values: vec32.Dup(t.Values),
	}
	return cp
}

// NewFullTrace takes data you would find in a Trace and returns a
// ClusterableTrace usable for kmeans clustering.
func NewFullTrace(key string, values []float32, minStdDev float32) *ClusterableTrace {
	norm := make([]float32, len(values))
	copy(norm, values)
	vec32.Fill(norm)
	vec32.Norm(norm, minStdDev)

	return &ClusterableTrace{
		Key:    key,
		Values: norm,
	}
}

// CalculateCentroid implements kmeans.CalculateCentroid.
func CalculateCentroid(members []kmeans.Clusterable) kmeans.Centroid {
	first := members[0].(*ClusterableTrace)
	mean := make([]float32, len(first.Values))
	for _, m := range members {
		ft := m.(*ClusterableTrace)
		for i, x := range ft.Values {
			mean[i] += x
		}
	}
	numMembers := float32(len(members))
	for i := range mean {
		mean[i] = mean[i] / numMembers
	}
	return &ClusterableTrace{
		Key:    CENTROID_KEY,
		Values: mean,
	}
}
