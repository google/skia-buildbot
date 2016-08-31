// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package types

import (
	"time"

	"github.com/luci/luci-go/common/tsmon/distribution"
)

// Metric is the low-level interface provided by all metrics.
// Concrete types are defined in the "metrics" package.
type Metric interface {
	Info() MetricInfo
	Metadata() MetricMetadata

	// SetFixedResetTime overrides the reset time for this metric.  Usually cells
	// take the current time when they're first assigned a value, but it can be
	// useful to override the reset time when tracking an external counter.
	SetFixedResetTime(t time.Time)
}

// DistributionMetric is the low-level interface provided by all distribution
// metrics.  It has a Bucketer which is responsible for assigning buckets to
// samples.  Concrete types are defined in the "metrics" package.
type DistributionMetric interface {
	Metric

	Bucketer() *distribution.Bucketer
}
