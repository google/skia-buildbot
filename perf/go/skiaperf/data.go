// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package main

import (
	"math"
	"math/rand"

	"sort"
)

import "github.com/golang/glog"

import (
	"skia.googlesource.com/buildbot.git/perf/go/ctrace"
	"skia.googlesource.com/buildbot.git/perf/go/kmeans"
	"skia.googlesource.com/buildbot.git/perf/go/types"
)

const (
	NUM_SAMPLE_TRACES_PER_CLUSTER = 10

	// K is the k in k-means.
	K = 100

	KMEANS_ITERATIONS = 10
)

// ValueWeight is a weight proportional to the number of times the parameter
// Value appears in a cluster. Used in ClusterSummary.
type ValueWeight struct {
	Value  string
	Weight int
}

// StepFit stores information on the best Step Function fit on a trace.
// Deviation is the Least Absolute Deviation divided by the step size.
// TurningPoint is the point index from where the Step Function changes value.
type StepFit struct {
	Deviation float64
	StepSize  float64
}

// ClusterSummary is a summary of a single cluster of traces.
type ClusterSummary struct {
	// Traces contains at most NUM_SAMPLE_TRACES_PER_CLUSTER sample traces, the first is the centroid.
	Traces [][][]float64

	// Keys of all the members of the Cluster.
	Keys []string

	// ParamSummaries is a summary of all the parameters in the cluster.
	ParamSummaries [][]ValueWeight

	// StepFit is info on the best Step Function fit of the centroid.
	StepFit StepFit
}

// ClusterSummaries is one summary for each cluster that the k-means clustering
// found.
type ClusterSummaries struct {
	Clusters         []*ClusterSummary
	StdDevThreshhold float64
	K                int
}

func NewClusterSummaries() *ClusterSummaries {
	return &ClusterSummaries{
		Clusters:         []*ClusterSummary{},
		StdDevThreshhold: ctrace.MIN_STDDEV,
		K:                K,
	}
}

// chooseK chooses a random sample of k observations. Used as the starting
// point for the k-means clustering.
func chooseK(observations []kmeans.Clusterable, k int) []kmeans.Clusterable {
	popN := len(observations)
	centroids := make([]kmeans.Clusterable, k)
	for i := 0; i < k; i++ {
		o := observations[rand.Intn(popN)].(*ctrace.ClusterableTrace)
		cp := &ctrace.ClusterableTrace{
			Key:    "I'm a centroid",
			Values: make([]float64, len(o.Values)),
		}
		copy(cp.Values, o.Values)
		centroids[i] = cp
	}
	return centroids
}

// traceToFlot converts the data into a format acceptable to the Flot plotting
// library.
//
// Flot expects data formatted as an array of [x, y] pairs.
func traceToFlot(t *ctrace.ClusterableTrace) [][]float64 {
	ret := make([][]float64, len(t.Values))
	for i, x := range t.Values {
		ret[i] = []float64{float64(i), x}
	}
	return ret
}

// ValueWeightSortable is a utility class for sorting the ValueWeight's by Weight.
type ValueWeightSortable []ValueWeight

func (p ValueWeightSortable) Len() int           { return len(p) }
func (p ValueWeightSortable) Less(i, j int) bool { return p[i].Weight > p[j].Weight } // Descending.
func (p ValueWeightSortable) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// getParamSummaries summaries all the parameters for all observations in a cluster.
//
// The return value is an array of []ValueWeight's, one []ValueWeight per
// parameter. The members of each []ValueWeight are sorted by the Weight, with
// higher Weight's first.
func getParamSummaries(cluster []kmeans.Clusterable) [][]ValueWeight {
	// For each cluster member increment each parameters count.
	type ValueMap map[string]int
	counts := map[string]ValueMap{}
	clusterSize := float64(len(cluster))
	// First figure out what parameters and values appear in the cluster.
	for _, o := range cluster {
		for k, v := range o.(*ctrace.ClusterableTrace).Params {
			if v == "" {
				continue
			}
			if _, ok := counts[k]; !ok {
				counts[k] = ValueMap{}
				counts[k][v] = 0
			}
			counts[k][v] += 1
		}
	}
	// Now calculate the weights for each parameter value.  The weight of each
	// value is proportional to the number of times it appears on an observation
	// versus all other values for the same parameter.
	ret := make([][]ValueWeight, 0)
	for _, count := range counts {
		weights := []ValueWeight{}
		for value, weight := range count {
			weights = append(weights, ValueWeight{
				Value:  value,
				Weight: int(14*float64(weight)/clusterSize) + 12,
			})
		}
		sort.Sort(ValueWeightSortable(weights))
		ret = append(ret, weights)
	}

	return ret
}

// average calculates and returns the average value of the given []float64.
func average(xs []float64) float64 {
	total := 0.0
	for _, v := range xs {
		total += v
	}
	return total / float64(len(xs))
}

// sse calculates and returns the sum squared error from the given base of []float64.
func sse(xs []float64, base float64) float64 {
	total := 0.0
	for _, v := range xs {
		total += math.Pow(v-base, 2)
	}
	return total
}

// getStepFit takes one []float64 trace and calculates and returns its StepFit.
func getStepFit(trace []float64) StepFit {
	deviation := math.MaxFloat64
	stepSize := -1.0
	for i := range trace {
		if i == 0 {
			continue
		}
		y0 := average(trace[:i])
		y1 := average(trace[i:])
		if y0 == y1 {
			continue
		}
		d := math.Sqrt(sse(trace[:i], y0)+sse(trace[i:], y1)) / float64(len(trace))
		if d < deviation {
			deviation = d
			stepSize = math.Abs(y0 - y1)
		}
	}
	return StepFit{deviation, stepSize}
}

type SortableClusterable struct {
	Cluster  kmeans.Clusterable
	Distance float64
}

type SortableClusterableSlice []*SortableClusterable

func (p SortableClusterableSlice) Len() int           { return len(p) }
func (p SortableClusterableSlice) Less(i, j int) bool { return p[i].Distance < p[j].Distance }
func (p SortableClusterableSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// GetClusterSummaries returns a summaries for each cluster.
func GetClusterSummaries(observations, centroids []kmeans.Clusterable) *ClusterSummaries {
	ret := &ClusterSummaries{
		Clusters: make([]*ClusterSummary, len(centroids)),
	}
	allClusters, _ := kmeans.GetClusters(observations, centroids)
	for i, cluster := range allClusters {
		// cluster is just an array of the observations for a given cluster.
		numSampleTraces := len(cluster)
		if numSampleTraces > NUM_SAMPLE_TRACES_PER_CLUSTER {
			numSampleTraces = NUM_SAMPLE_TRACES_PER_CLUSTER
		}
		summary := &ClusterSummary{
			Keys:           make([]string, len(cluster)),
			Traces:         make([][][]float64, numSampleTraces),
			ParamSummaries: getParamSummaries(cluster),
			// Try fit on the centroid.
			StepFit: getStepFit(cluster[0].(*ctrace.ClusterableTrace).Values),
		}
		for j, o := range cluster {
			summary.Keys[j] = o.(*ctrace.ClusterableTrace).Key
		}
		// First, sort the traces so they are order with the traces closest to the
		// centroid first.
		sc := []*SortableClusterable{}
		for j := 0; j < numSampleTraces; j++ {
			sc = append(sc, &SortableClusterable{Cluster: cluster[j], Distance: cluster[j].Distance(cluster[0])})
		}
		// Sort, but leave the centroid, the 0th element, unmoved.
		sort.Sort(SortableClusterableSlice(sc[1:]))

		for j := 0; j < numSampleTraces; j++ {
			summary.Traces[j] = traceToFlot(sc[j].Cluster.(*ctrace.ClusterableTrace))
		}
		ret.Clusters[i] = summary
	}

	return ret
}

// calculateClusterSummaries runs k-means clustering over the trace shapes.
func calculateClusterSummaries(tile *types.Tile, k int, stddevThreshhold float64) *ClusterSummaries {
	lastCommitIndex := 0
	for i, c := range tile.Commits {
		if c.CommitTime != 0 {
			lastCommitIndex = i
		}
	}
	observations := make([]kmeans.Clusterable, 0, len(tile.Traces))
	for key, trace := range tile.Traces {
		observations = append(observations, ctrace.NewFullTrace(string(key), trace.Values[:lastCommitIndex], trace.Params, stddevThreshhold))
	}

	// Create K starting centroids.
	centroids := chooseK(observations, k)
	// TODO(jcgregorio) Keep iterating until the total error stops changing.
	for i := 0; i < KMEANS_ITERATIONS; i++ {
		centroids = kmeans.Do(observations, centroids, ctrace.CalculateCentroid)
		glog.Infof("Total Error: %f\n", kmeans.TotalError(observations, centroids))
	}
	clusterSummaries := GetClusterSummaries(observations, centroids)
	clusterSummaries.K = k
	clusterSummaries.StdDevThreshhold = stddevThreshhold
	return clusterSummaries
}
