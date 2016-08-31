// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package distribution

import (
	"fmt"
	"math"
	"sort"
)

// A Bucketer maps samples into discrete buckets.
type Bucketer struct {
	width, growthFactor float64
	numFiniteBuckets    int
	lowerBounds         []float64
}

// NewBucketer creates a bucketer from custom parameters.
func NewBucketer(width, growthFactor float64, numFiniteBuckets int) *Bucketer {
	b := &Bucketer{
		width:            width,
		growthFactor:     growthFactor,
		numFiniteBuckets: numFiniteBuckets,
	}
	b.init()
	return b
}

// FixedWidthBucketer returns a Bucketer that uses numFiniteBuckets+2 buckets:
//   bucket[0] covers (-Inf...0)
//   bucket[i] covers [width*(i-1)...width*i) for i > 0 and i <= numFiniteBuckets
//   bucket[numFiniteBuckets+1] covers [width*numFiniteBuckets...+Inf)
// The width must be greater than 0, and the number of finite buckets must be
// greater than 0.
func FixedWidthBucketer(width float64, numFiniteBuckets int) *Bucketer {
	return NewBucketer(width, 0, numFiniteBuckets)
}

// GeometricBucketer returns a Bucketer that uses numFiniteBuckets+2 buckets:
//   bucket[0] covers (−Inf, 0)
//   bucket[1] covers [0, growthFactor)
//   bucket[i] covers [growthFactor^(i−2), growthFactor^(i−1)) for i > 1 and i <= numFiniteBuckets
//   bucket[numFiniteBuckets+1] covers [growthFactor^(numFiniteBuckets−1), +Inf)
//
// growthFactor must be positive, and the number of finite buckets must be
// greater than 0.
func GeometricBucketer(growthFactor float64, numFiniteBuckets int) *Bucketer {
	return NewBucketer(0, growthFactor, numFiniteBuckets)
}

// DefaultBucketer is a bucketer with sensible bucket sizes.
var DefaultBucketer = GeometricBucketer(math.Pow(10, 0.2), 100)

// NumFiniteBuckets returns the number of finite buckets.
func (b *Bucketer) NumFiniteBuckets() int { return b.numFiniteBuckets }

// NumBuckets returns the number of buckets including the underflow and overflow
// buckets.
func (b *Bucketer) NumBuckets() int { return b.numFiniteBuckets + 2 }

// Width returns the bucket width used to configure this Bucketer.
func (b *Bucketer) Width() float64 { return b.width }

// GrowthFactor returns the growth factor used to configure this Bucketer.
func (b *Bucketer) GrowthFactor() float64 { return b.growthFactor }

// UnderflowBucket returns the index of the underflow bucket.
func (b *Bucketer) UnderflowBucket() int { return 0 }

// OverflowBucket returns the index of the overflow bucket.
func (b *Bucketer) OverflowBucket() int { return b.numFiniteBuckets + 1 }

// Bucket returns the index of the bucket for sample.
// TODO(dsansome): consider reimplementing sort.Search inline to avoid overhead
// of calling a function to compare two values.
func (b *Bucketer) Bucket(sample float64) int {
	return sort.Search(b.NumBuckets(), func(i int) bool { return sample < b.lowerBounds[i] }) - 1
}

func (b *Bucketer) init() {
	if b.numFiniteBuckets < 0 {
		panic(fmt.Sprintf("numFiniteBuckets must be positive (was %d)", b.numFiniteBuckets))
	}

	b.lowerBounds = make([]float64, 2, b.NumBuckets())
	b.lowerBounds[0] = math.Inf(-1)
	b.lowerBounds[1] = 0

	var previous float64
	for i := 0; i < b.numFiniteBuckets; i++ {
		lowerBound := b.width * float64(i+1)
		if b.growthFactor != 0 {
			lowerBound += math.Pow(b.growthFactor, float64(i))
		}

		if lowerBound <= previous {
			panic("bucket boundaries must be monotonically increasing")
		}

		b.lowerBounds = append(b.lowerBounds, lowerBound)
		previous = lowerBound
	}
}
