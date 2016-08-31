// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package distribution contains distribution metrics, fixed width and geometric
// bucketers.
package distribution

// A Distribution holds a statistical summary of a collection of floating-point
// values.
type Distribution struct {
	b *Bucketer

	buckets []int64
	count   int64
	sum     float64
}

// New creates a new distribution using the given bucketer.  Passing a nil
// Bucketer will use DefaultBucketer.
func New(b *Bucketer) *Distribution {
	if b == nil {
		b = DefaultBucketer
	}
	return &Distribution{b: b}
}

// Add adds the sample to the distribution and updates the statistics.
func (d *Distribution) Add(sample float64) {
	i := d.b.Bucket(sample)
	if i >= len(d.buckets) {
		d.buckets = append(d.buckets, make([]int64, i-len(d.buckets)+1)...)
	}
	d.buckets[i]++
	d.sum += sample
	d.count++
}

// Bucketer returns the bucketer used in this distribution.
func (d *Distribution) Bucketer() *Bucketer { return d.b }

// Buckets provides access to the underlying buckets slice.  len(Buckets) will
// be <= Bucketer().NumBuckets()
func (d *Distribution) Buckets() []int64 { return d.buckets }

// Count returns the number of times Add has been called.
func (d *Distribution) Count() int64 { return d.count }

// Sum returns the sum of all samples passed to Add.
func (d *Distribution) Sum() float64 { return d.sum }
