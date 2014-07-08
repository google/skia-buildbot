// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.
//
// ctrace makes Traces into ClusterableTraces which can then be used in kmeans.
package ctrace

import (
	"fmt"
	"math"
)

import (
	"config"
	"kmeans"
)

const (
	// MIN_STDDEV is the smallest standard deviation we will normalize, smaller
	// than this and we presume it's a standard deviation of zero.
	MIN_STDDEV = 0.01
)

// ClusterableTrace contains Trace data and implements kmeans.Clusterable.
type ClusterableTrace struct {
	Key    string
	Values []float64
	Params map[string]string
}

func (t *ClusterableTrace) Distance(other kmeans.Clusterable) float64 {
	// Data is always loaded from BigQuery so that every Trace has the same length,
	// and NewFullTrace keeps that guarantee.
	o := other.(*ClusterableTrace)
	sum := 0.0
	for i, x := range t.Values {
		sum += (x - o.Values[i]) * (x - o.Values[i])
	}
	return math.Sqrt(sum)
}

func (t *ClusterableTrace) String() string {
	return fmt.Sprintf("%s %#v", t.Key, t.Values[:2])
}

// NewFullTrace takes data you would find in a Trace and returns a
// ClusterableTrace usable for kmeans clustering.
func NewFullTrace(key string, values []float64, params map[string]string, minStdDev float64) *ClusterableTrace {
	norm := make([]float64, len(values))

	// Find the first non-sentinel data point.
	last := 0.0
	for _, x := range values {
		if x != config.MISSING_DATA_SENTINEL {
			last = x
			break
		}
	}
	// Copy over the data from values, backfilling in sentinels with
	// older points, except for the beginning of the array where
	// we can't do that, so we fill those points in using the first
	// non sentinel.
	// So
	//    [1e100, 1e100, 2, 3, 1e100, 5]
	// becomes
	//    [2    , 2    , 2, 3, 3    , 5]
	//
	sum := 0.0
	sum2 := 0.0
	for i, x := range values {
		if x == config.MISSING_DATA_SENTINEL {
			norm[i] = last
		} else {
			norm[i] = x
			last = x
		}
		sum += norm[i]
		sum2 += norm[i] * norm[i]
	}

	mean := sum / float64(len(norm))
	stddev := math.Sqrt(sum2/float64(len(norm)) - mean*mean)

	// Normalize the data to a mean of 0 and standard deviation of 1.0.
	for i, _ := range norm {
		norm[i] -= mean
		if stddev > MIN_STDDEV {
			norm[i] = norm[i] / stddev
		}
	}

	return &ClusterableTrace{
		Key:    key,
		Values: norm,
		Params: params,
	}
}

// CalculateCentroid implements kmeans.CalculateCentroid.
func CalculateCentroid(members []kmeans.Clusterable) kmeans.Clusterable {
	first := members[0].(*ClusterableTrace)
	mean := make([]float64, len(first.Values))
	for _, m := range members {
		ft := m.(*ClusterableTrace)
		for i, x := range ft.Values {
			mean[i] += x
		}
	}
	numMembers := float64(len(members))
	for i, _ := range mean {
		mean[i] = mean[i] / numMembers
	}
	return &ClusterableTrace{
		Key:    "I'm a centroid!",
		Values: mean,
	}
}
