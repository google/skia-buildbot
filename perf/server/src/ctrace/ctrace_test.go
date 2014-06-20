// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.
package ctrace

import (
	"kmeans"
	"math"
	"testing"
)

func near(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

func TestDistance(t *testing.T) {
	a := &ClusterableTrace{Values: []float64{3, 0}}
	b := &ClusterableTrace{Values: []float64{0, 4}}
	if got, want := a.Distance(b), 5.0; !near(got, want) {
		t.Errorf("Distance mismatch: Got %f Want %f", got, want)
	}
	if got, want := a.Distance(a), 0.0; !near(got, want) {
		t.Errorf("Distance mismatch: Got %f Want %f", got, want)
	}
}

func TestNewFullTraceKey(t *testing.T) {
	ct := NewFullTrace("foo", []float64{1, -1})
	if got, want := ct.Key, "foo"; got != want {
		t.Errorf("Key not set: Got %s Want %s", got, want)
	}
}

func TestNewFullTrace(t *testing.T) {
	// All positive (Near=true) testcases should end up with a normalized array
	// of values with 1.0 in the first spot and a standard deviation of 1.0.
	testcases := []struct {
		Values []float64
		Near   bool
	}{
		{
			Values: []float64{1.0, -1.0},
			Near:   true,
		},
		{
			Values: []float64{1e100, 1.0, -1.0, -1.0},
			Near:   true,
		},
		{
			Values: []float64{1e100, 1.0, -1.0, 1e100},
			Near:   true,
		},
		{
			Values: []float64{1e100, 2.0, -2.0, 1e100},
			Near:   true,
		},
		{
			// There's a limit to how small of a stddev we will normalize.
			Values: []float64{1e100, MIN_STDDEV, -MIN_STDDEV, 1e100},
			Near:   false,
		},
	}
	for _, tc := range testcases {
		ct := NewFullTrace("foo", tc.Values)
		if got, want := ct.Values[0], 1.0; near(got, want) != tc.Near {
			t.Errorf("Normalization failed for values %#v: near(Got %f, Want %f) != %t", tc.Values, got, want, tc.Near)
		}
	}
}

func TestCalculateCentroid(t *testing.T) {
	members := []kmeans.Clusterable{
		&ClusterableTrace{Values: []float64{4, 0}},
		&ClusterableTrace{Values: []float64{0, 8}},
	}
	c := CalculateCentroid(members).(*ClusterableTrace)
	if got, want := c.Values[0], 2.0; !near(got, want) {
		t.Errorf("Failed calculating centroid: !near(Got %f, Want %f)", got, want)
	}
	if got, want := c.Values[1], 4.0; !near(got, want) {
		t.Errorf("Failed calculating centroid: !near(Got %f, Want %f)", got, want)
	}

}
