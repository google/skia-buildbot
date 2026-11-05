// Copyright 2021 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package noop

// Float64SummaryMetric implements metrics2.Float64SummaryMetric as a no-op
type Float64SummaryMetric struct{}

func (f Float64SummaryMetric) Observe(_ float64) {}
