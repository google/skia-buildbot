// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package insights

import (
	"time"

	"go.skia.org/infra/go/metrics2"
)

// RecordRequest records the outcome and latency of a request. surface is "rest" or "mcp" and op
// is the operation name, e.g. "ListAnomalies".
func RecordRequest(surface, op string, code Code, d time.Duration) {
	metrics2.GetCounter("perfinsights_requests", map[string]string{
		"surface": surface,
		"op":      op,
		"code":    string(code),
	}).Inc(1)
	metrics2.GetFloat64SummaryMetric("perfinsights_request_duration_seconds", map[string]string{
		"surface": surface,
		"op":      op,
	}).Observe(d.Seconds())
}
